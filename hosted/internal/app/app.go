package app

import (
	"context"
	"crypto/rand"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"net/http"
	"net/url"
	"strings"
	"sync"
	"time"

	"github.com/jackc/pgx/v5/pgxpool"
	"github.com/sirerun/gist/hosted/internal/discovery"
	"github.com/sirerun/gist/hosted/internal/events"
	"github.com/sirerun/gist/hosted/internal/identity"
	"github.com/sirerun/gist/hosted/internal/ports"
	"github.com/sirerun/gist/hosted/internal/remotemcp"
	"github.com/sirerun/gist/hosted/internal/resolution"
	"github.com/sirerun/gist/hosted/internal/rest"
	"github.com/sirerun/gist/hosted/internal/storage"
)

type App struct {
	cfg       Config
	pool      *pgxpool.Pool
	objects   *storage.ObjectStore
	server    *http.Server
	closeOnce sync.Once
}

// New opens the real pgx pool and filesystem-backed object store. It pings
// PostgreSQL before returning, so a missing integration fixture is an actual
// failure rather than a test skip.
func New(ctx context.Context, cfg Config) (*App, error) {
	if cfg.ResourceAudience == "" {
		cfg.ResourceAudience = cfg.PublicOrigin
	}
	if err := cfg.Validate(); err != nil {
		return nil, err
	}
	pool, err := pgxpool.New(ctx, cfg.DatabaseURL)
	if err != nil {
		return nil, fmt.Errorf("app: create postgres pool: %w", err)
	}
	if err := pool.Ping(ctx); err != nil {
		pool.Close()
		return nil, fmt.Errorf("app: ping postgres: %w", err)
	}
	objects, err := storage.NewObjectStore(cfg.ObjectStoreRoot)
	if err != nil {
		pool.Close()
		return nil, err
	}
	return newWithStores(cfg, pool, objects)
}

func newWithStores(cfg Config, pool *pgxpool.Pool, objects *storage.ObjectStore) (*App, error) {
	if pool == nil || objects == nil {
		return nil, errors.New("app: backing stores are required")
	}
	clock := identityClock{}
	key, err := identity.GenerateSigningKey("startup", clock.Now())
	if err != nil {
		pool.Close()
		return nil, err
	}
	keys, err := identity.NewKeySet(clock, key)
	if err != nil {
		pool.Close()
		return nil, err
	}
	policy := &postgresPolicy{pool: pool}
	lease, err := identity.NewRevocationLease(30 * time.Second)
	if err != nil {
		pool.Close()
		return nil, err
	}
	issuer, err := identity.NewWorkloadIssuer(identity.Config{Issuer: cfg.PublicOrigin, Audience: cfg.ResourceAudience, Clock: clock, Keys: keys, Policy: policy, Revocations: lease})
	if err != nil {
		pool.Close()
		return nil, err
	}
	identityStore := verifiedIdentity{issuer: issuer}
	// storage.Postgres is the catalog, identity and policy-adjacent DB adapter.
	catalog, err := storage.NewPostgres(pool)
	if err != nil {
		pool.Close()
		return nil, err
	}
	identityStore.catalog = catalog
	search, err := discovery.New(catalog, policy)
	if err != nil {
		pool.Close()
		return nil, err
	}
	resolutionStore := &postgresResolutionStore{pool: pool}
	resolver, err := resolution.NewResolver(catalog, policy, resolutionStore, clock, nil)
	if err != nil {
		pool.Close()
		return nil, err
	}
	feed := events.NewStore(clock)
	broker := newBroker(cfg.BrokerURL, cfg.RequestTimeout)
	var connections ports.ConnectionInitiator
	if broker != nil {
		connections = broker
	}
	services := rest.Services{Identity: identityStore, Authorizer: policy, Catalog: catalog, Search: lexicalAdapter{service: search}, Artifacts: objects, Resolutions: resolutionStore, Connections: connections, Events: feed, Publisher: publisher{pool: pool, objects: objects, limits: cfg}, Resolver: resolverAdapter{resolver: resolver, maxBytes: cfg.MaxResponseBytes}, Limits: cfg.RESTLimits(), Audience: cfg.ResourceAudience}
	rh, err := rest.New(services)
	if err != nil {
		pool.Close()
		return nil, err
	}
	mcp, err := remotemcp.New(remotemcp.Config{Services: services, AllowedOrigins: map[string]bool{cfg.PublicOrigin: true}, MaxBodyBytes: cfg.MaxRequestBytes, MaxCalls: cfg.MaxConcurrentRequests})
	if err != nil {
		pool.Close()
		return nil, err
	}
	return &App{cfg: cfg, pool: pool, objects: objects, server: &http.Server{Addr: cfg.ListenAddress, Handler: requestContext{rest: rh, mcp: mcp, ready: func(ctx context.Context) error { return pool.Ping(ctx) }}, ReadHeaderTimeout: cfg.RequestTimeout}}, nil
}

func (a *App) Handler() http.Handler { return a.server.Handler }
func (a *App) ListenAndServe() error {
	if a.server == nil {
		return errors.New("app: not initialized")
	}
	err := a.server.ListenAndServe()
	if errors.Is(err, http.ErrServerClosed) {
		return nil
	}
	return err
}
func (a *App) Shutdown(ctx context.Context) error {
	var err error
	a.closeOnce.Do(func() { err = a.server.Shutdown(ctx); a.pool.Close() })
	return err
}

type requestContext struct {
	rest, mcp http.Handler
	ready     func(context.Context) error
}

func (h requestContext) ServeHTTP(w http.ResponseWriter, r *http.Request) {
	if r.Header.Get("X-Request-ID") == "" {
		var b [12]byte
		if _, err := rand.Read(b[:]); err == nil {
			r.Header.Set("X-Request-ID", fmt.Sprintf("req_%x", b))
		}
	}
	if r.URL.Path == "/mcp" {
		h.mcp.ServeHTTP(w, r)
		return
	}
	if r.URL.Path == "/healthz" {
		w.WriteHeader(http.StatusOK)
		_, _ = io.WriteString(w, `{"status":"ok"}`)
		return
	}
	if r.URL.Path == "/readyz" {
		if h.ready != nil {
			if err := h.ready(r.Context()); err != nil {
				w.Header().Set("Content-Type", "application/json")
				w.WriteHeader(http.StatusServiceUnavailable)
				_, _ = io.WriteString(w, `{"status":"store_unavailable"}`)
				return
			}
		}
		w.Header().Set("Content-Type", "application/json")
		w.WriteHeader(http.StatusOK)
		_, _ = io.WriteString(w, `{"status":"ready","policy":"online"}`)
		return
	}
	h.rest.ServeHTTP(w, r)
}

type identityClock struct{}

func (identityClock) Now() time.Time { return time.Now().UTC() }

type verifiedIdentity struct {
	issuer  *identity.WorkloadIssuer
	catalog ports.IdentityStore
}

func (v verifiedIdentity) Lookup(ctx context.Context, token, audience string) (ports.IdentityRecord, error) {
	got, err := v.issuer.Verify(ctx, token)
	if err != nil || got.Principal.Audience != audience {
		return ports.IdentityRecord{}, identity.ErrUnauthorized
	}
	return ports.IdentityRecord{Issuer: got.Principal.Issuer, Subject: got.Principal.Subject, WorkspaceID: got.Principal.WorkspaceID, SubjectType: got.Principal.SubjectType, Scopes: got.Principal.Scopes, PolicyGeneration: got.Principal.PolicyGeneration, ExpiresAt: got.ExpiresAt.Unix()}, nil
}
func (v verifiedIdentity) Revoke(ctx context.Context, issuer, subject string) error {
	return v.catalog.Revoke(ctx, issuer, subject)
}

type lexicalAdapter struct{ service *discovery.Service }

func (s lexicalAdapter) Search(ctx context.Context, q ports.SearchQuery) (ports.SearchPage, error) {
	got, err := s.service.Search(ctx, discovery.Request{Principal: q.Principal, Query: q.Text, Kinds: q.Kinds, Tags: q.Tags, Cursor: q.Cursor, Limit: q.Limit, MaxBytes: q.MaxBytes})
	if err != nil {
		return ports.SearchPage{}, err
	}
	out := ports.SearchPage{}
	for _, c := range got.Candidates {
		metadata, _ := json.Marshal(c)
		out.Records = append(out.Records, ports.CatalogRecord{Ref: c.Ref, State: "published", Metadata: metadata})
	}
	if got.NextCursor != nil {
		out.Next = *got.NextCursor
	}
	return out, nil
}

type resolverAdapter struct {
	resolver *resolution.Resolver
	maxBytes int
}

func (r resolverAdapter) Resolve(ctx context.Context, p ports.Principal, raw []byte) ([]byte, error) {
	var in struct {
		Skill            ports.ArtifactRef `json:"skill"`
		RuntimeID        string            `json:"runtime_id"`
		LocalExecution   bool              `json:"local_execution"`
		OwnedConnections bool              `json:"owned_connections"`
		SelectedBindings map[string]string `json:"selected_bindings"`
		MaxBytes         int               `json:"max_bytes"`
	}
	if err := json.Unmarshal(raw, &in); err != nil {
		return nil, fmt.Errorf("decode resolution request: %w", err)
	}
	if in.Skill.WorkspaceID == "" {
		in.Skill.WorkspaceID = p.WorkspaceID
	}
	if in.MaxBytes <= 0 {
		in.MaxBytes = r.maxBytes
	}
	got, err := r.resolver.Resolve(ctx, resolution.Request{Principal: p, Skill: in.Skill, RuntimeID: in.RuntimeID, LocalExecution: in.LocalExecution, OwnedConnections: in.OwnedConnections, SelectedBindings: in.SelectedBindings, MaxBytes: in.MaxBytes})
	if err != nil {
		return nil, err
	}
	return json.Marshal(got)
}

type broker struct {
	endpoint string
	client   *http.Client
}

func newBroker(endpoint string, timeout time.Duration) *broker {
	if strings.TrimSpace(endpoint) == "" {
		return nil
	}
	u, err := url.Parse(endpoint)
	if err != nil || u.Scheme != "https" || u.Host == "" {
		return nil
	}
	return &broker{endpoint: strings.TrimRight(endpoint, "/"), client: &http.Client{Timeout: timeout}}
}
func (b *broker) Begin(ctx context.Context, p ports.Principal, ref ports.ArtifactRef) (ports.Connection, error) {
	return b.call(ctx, http.MethodPost, "", p, ref)
}
func (b *broker) Get(ctx context.Context, p ports.Principal, id string) (ports.Connection, error) {
	return b.call(ctx, http.MethodGet, "/"+url.PathEscape(id), p, ports.ArtifactRef{})
}
func (b *broker) call(ctx context.Context, method, suffix string, p ports.Principal, ref ports.ArtifactRef) (ports.Connection, error) {
	var body io.Reader
	if method == http.MethodPost {
		raw, _ := json.Marshal(map[string]any{"workspace_id": p.WorkspaceID, "subject": p.Subject, "capability": ref})
		body = strings.NewReader(string(raw))
	}
	req, err := http.NewRequestWithContext(ctx, method, b.endpoint+"/connections"+suffix, body)
	if err != nil {
		return ports.Connection{}, err
	}
	req.Header.Set("Content-Type", "application/json")
	resp, err := b.client.Do(req)
	if err != nil {
		return ports.Connection{}, fmt.Errorf("connection broker: %w", err)
	}
	defer resp.Body.Close()
	if resp.StatusCode < 200 || resp.StatusCode >= 300 {
		return ports.Connection{}, fmt.Errorf("connection broker returned %s", resp.Status)
	}
	var c ports.Connection
	if err := json.NewDecoder(resp.Body).Decode(&c); err != nil {
		return ports.Connection{}, err
	}
	return c, nil
}

// Compile-time checks keep the app wiring honest when a frozen port changes.
var _ http.Handler = requestContext{}
var _ ports.ConnectionInitiator = (*broker)(nil)

package app

import (
	"context"
	"crypto/rand"
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"net/http"
	"net/url"
	"strings"
	"sync"
	"time"

	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgxpool"
	"github.com/sirerun/gist/hosted/internal/discovery"
	"github.com/sirerun/gist/hosted/internal/events"
	"github.com/sirerun/gist/hosted/internal/identity"
	"github.com/sirerun/gist/hosted/internal/oauth"
	"github.com/sirerun/gist/hosted/internal/ports"
	"github.com/sirerun/gist/hosted/internal/remotemcp"
	"github.com/sirerun/gist/hosted/internal/resolution"
	"github.com/sirerun/gist/hosted/internal/rest"
	"github.com/sirerun/gist/hosted/internal/storage"
)

type App struct {
	cfg     Config
	pool    *pgxpool.Pool
	objects *storage.ObjectStore
	issuer  *identity.WorkloadIssuer
	// identities records the stored identity behind every minted token.
	identities issuedIdentityRecorder
	// sessions issues reference authorization-server session cookies.
	sessions  *oauth.CookieSessions
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
	objects.UseCatalog(catalog)
	identityStore.catalog = catalog
	search, err := discovery.New(catalog, policy)
	if err != nil {
		pool.Close()
		return nil, err
	}
	resolutionStore := &postgresResolutionStore{pool: pool}
	resolver, err := resolution.NewResolver(unrevokedCatalog{CatalogStore: catalog}, policy, resolutionStore, clock, nil)
	if err != nil {
		pool.Close()
		return nil, err
	}
	feed := events.NewStore(clock)
	broker := newBroker(cfg.BrokerURL, cfg.RequestTimeout, cfg.BrokerClient)
	var connections ports.ConnectionInitiator
	if broker != nil {
		connections = broker
	}
	services := rest.Services{Identity: identityStore, Authorizer: policy, Catalog: catalog, Search: lexicalAdapter{service: search}, Versions: catalog, Artifacts: objects, Resolutions: resolutionStore, Connections: connections, Events: eventStoreAdapter{store: feed}, Publisher: publisher{pool: pool, objects: objects, limits: cfg}, Revocations: artifactRevoker{catalog: catalog, feed: eventStoreAdapter{store: feed}}, Resolver: resolverAdapter{resolver: resolver, maxBytes: cfg.MaxResponseBytes}, Limits: cfg.RESTLimits(), Audience: cfg.ResourceAudience}
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
	// The reference authorization server issues access tokens with the same
	// workload issuer (and so the same key set, issuer and audience) that
	// verifies REST and MCP requests, and records each issued identity so the
	// stored-identity check in verifiedIdentity accepts it.
	sessionSecret := make([]byte, 32)
	if _, err := rand.Read(sessionSecret); err != nil {
		pool.Close()
		return nil, fmt.Errorf("app: generate oauth session secret: %w", err)
	}
	sessions, err := oauth.NewCookieSessions(sessionSecret, clock)
	if err != nil {
		pool.Close()
		return nil, err
	}
	minter := oauth.IssuerMinter{Issuers: map[string]*identity.WorkloadIssuer{cfg.ResourceAudience: issuer}, Record: recordIssued(catalog)}
	as, err := oauth.New(oauth.Config{Issuer: cfg.PublicOrigin, Resources: []string{cfg.ResourceAudience}, ScopesSupported: []string{"catalog:read", "catalog:publish"}, Store: catalog.OAuth(), Minter: minter, Policy: policy, Sessions: sessions, Keys: keys, Clock: clock, LoginURL: cfg.OAuthLoginURL})
	if err != nil {
		pool.Close()
		return nil, err
	}
	prm := as.ProtectedResourceMetadataURL()
	handler := requestContext{rest: oauth.WithChallenge(rh, prm), mcp: oauth.WithChallenge(mcp, prm), oauth: as, ready: func(ctx context.Context) error { return pool.Ping(ctx) }}
	return &App{cfg: cfg, pool: pool, objects: objects, issuer: issuer, identities: catalog, sessions: sessions, server: &http.Server{Addr: cfg.ListenAddress, Handler: handler, ReadHeaderTimeout: cfg.RequestTimeout}}, nil
}

// recordIssued adapts the identity store to the OAuth minter. A revoked
// stored identity refuses issuance, so its token is discarded.
func recordIssued(store issuedIdentityRecorder) oauth.IdentityRecorder {
	return func(ctx context.Context, r ports.IdentityRecord) error {
		if store == nil {
			return errors.New("app: identity store is unavailable")
		}
		if err := store.RecordIssued(ctx, r); err != nil {
			if errors.Is(err, storage.ErrIdentityRevoked) {
				return identity.ErrUnauthorized
			}
			return fmt.Errorf("app: record workload identity: %w", err)
		}
		return nil
	}
}

// IssueOAuthSession returns a reference authorization-server session cookie
// for a person the deployment's login flow has already authenticated. It is
// the seam acceptance fixtures use in place of an interactive login; the
// consent step still checks live workspace membership.
func (a *App) IssueOAuthSession(subject string, workspaces []string) (*http.Cookie, error) {
	if a == nil || a.sessions == nil {
		return nil, errors.New("app: oauth sessions are unavailable")
	}
	return a.sessions.Issue(subject, workspaces, time.Hour)
}

func (a *App) Handler() http.Handler { return a.server.Handler }

// MintWorkloadToken is the local workload-issuer seam used by acceptance
// fixtures. Production callers obtain tokens from the identity service; the
// composed app keeps the issuer private while allowing an in-process fixture
// to mint tokens signed by the exact key set used for verification.
func (a *App) MintWorkloadToken(ctx context.Context, req identity.WorkloadRequest) (string, error) {
	if a == nil || a.issuer == nil {
		return "", errors.New("app: workload issuer is unavailable")
	}
	// Token verification requires an unrevoked stored identity, so issuance
	// records one from the claims read back out of the signed token. The
	// token is returned only after the row is written; a revoked identity
	// gets no token. The OAuth server mints through the same path.
	minted, err := oauth.IssuerMinter{Issuers: map[string]*identity.WorkloadIssuer{a.cfg.ResourceAudience: a.issuer}, Record: recordIssued(a.identities)}.MintAccess(ctx, a.cfg.ResourceAudience, req)
	if err != nil {
		return "", err
	}
	return minted.Token, nil
}

// SeedAcceptanceArtifact installs a complete artifact into the same
// filesystem/catalog stores used by the composed handler. It is intentionally
// narrow and exists for the integration fixture; normal publication uses the
// REST publisher path.
func (a *App) SeedAcceptanceArtifact(ctx context.Context, p ports.Principal, ref ports.ArtifactRef, raw, metadata []byte) error {
	if a == nil || a.objects == nil || a.pool == nil {
		return errors.New("app: acceptance stores are unavailable")
	}
	sum := sha256.Sum256(raw)
	digest := ports.Digest{Algorithm: "sha256", Value: hex.EncodeToString(sum[:])}
	if err := a.objects.Put(ctx, digest, bytesReader(raw), int64(len(raw))); err != nil {
		return err
	}
	if err := a.objects.Bind(ref, digest); err != nil {
		return err
	}
	return storage.WithTenantPrincipal(ctx, a.pool, storage.Tenant{Issuer: p.Issuer, Subject: p.Subject, Audience: p.Audience, WorkspaceID: p.WorkspaceID, Scopes: p.Scopes, PolicyGeneration: p.PolicyGeneration}, func(ctx context.Context, tx pgx.Tx) error {
		_, err := tx.Exec(ctx, `INSERT INTO catalog_versions(workspace_id,kind,artifact_id,version,state,digest_algorithm,digest_value,manifest_digest_algorithm,manifest_digest_value,metadata,package_key,owner_id) VALUES($1,$2,$3,$4,'published','sha256',$5,'sha256',$5,$6,$5,$7) ON CONFLICT DO NOTHING`, ref.WorkspaceID, ref.Kind, ref.ID, ref.Version, digest.Value, metadata, p.Subject)
		return err
	})
}
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
	rest, mcp, oauth http.Handler
	ready            func(context.Context) error
}

func (h requestContext) ServeHTTP(w http.ResponseWriter, r *http.Request) {
	if r.Header.Get("X-Request-ID") == "" {
		var b [12]byte
		if _, err := rand.Read(b[:]); err == nil {
			r.Header.Set("X-Request-ID", fmt.Sprintf("req_%x", b))
		}
	}
	if h.oauth != nil && oauth.Handles(r.URL.Path) {
		h.oauth.ServeHTTP(w, r)
		return
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

type eventStoreAdapter struct{ store *events.Store }

func (s eventStoreAdapter) Append(ctx context.Context, e ports.Event) error {
	return s.store.Append(ctx, e)
}
func (s eventStoreAdapter) Read(ctx context.Context, c ports.Cursor) (ports.EventPage, error) {
	if c.ID == "" {
		return ports.EventPage{}, nil
	}
	return s.store.Read(ctx, c)
}
func (s eventStoreAdapter) NewCursor(p ports.Principal, ttl time.Duration) (ports.Cursor, error) {
	return s.store.NewCursor(p, ttl)
}

var _ rest.EventCursorOpener = eventStoreAdapter{}

// identityCatalog is the workspace-scoped identity table. Reads and writes
// carry the workspace so storage can run them under row-level security.
type identityCatalog interface {
	Lookup(ctx context.Context, workspaceID, issuer, subject string) (ports.IdentityRecord, error)
	Revoke(ctx context.Context, workspaceID, issuer, subject string) error
}

// issuedIdentityRecorder persists the stored identity behind a minted token.
// It must refuse, with storage.ErrIdentityRevoked, to revive a revoked row.
type issuedIdentityRecorder interface {
	RecordIssued(ctx context.Context, r ports.IdentityRecord) error
}

// tokenVerifier verifies a workload token's signature, audience binding and
// expiry. *identity.WorkloadIssuer is the production implementation.
type tokenVerifier interface {
	Verify(ctx context.Context, rawToken string) (identity.VerifiedToken, error)
}

type verifiedIdentity struct {
	issuer  tokenVerifier
	catalog identityCatalog
}

func (v verifiedIdentity) Lookup(ctx context.Context, token, audience string) (ports.IdentityRecord, error) {
	_, rec, err := v.AuthenticateContext(ctx, token, audience)
	return rec, err
}

// AuthenticateContext verifies the token like Lookup and also returns a
// context carrying the verified identity.AuthContext, so workspace-bound
// operations such as Revoke take the workspace from the verified token.
func (v verifiedIdentity) AuthenticateContext(ctx context.Context, token, audience string) (context.Context, ports.IdentityRecord, error) {
	if v.issuer == nil {
		return ctx, ports.IdentityRecord{}, identity.ErrUnauthorized
	}
	authCtx, auth, err := identity.VerifyContext(ctx, v.issuer, token)
	if err != nil || auth.Principal().Audience != audience {
		return ctx, ports.IdentityRecord{}, identity.ErrUnauthorized
	}
	// A valid signature is not enough: the stored identity must still exist
	// and be unrevoked. Storage filters revoked rows, so a revoked identity
	// surfaces as not-found. Any storage failure fails closed.
	if v.catalog == nil {
		return ctx, ports.IdentityRecord{}, identity.ErrUnauthorized
	}
	p := auth.Principal()
	stored, err := v.catalog.Lookup(ctx, p.WorkspaceID, p.Issuer, p.Subject)
	if err != nil || stored.WorkspaceID != p.WorkspaceID || stored.Issuer != p.Issuer || stored.Subject != p.Subject {
		return ctx, ports.IdentityRecord{}, identity.ErrUnauthorized
	}
	return authCtx, ports.IdentityRecord{Issuer: p.Issuer, Subject: p.Subject, WorkspaceID: p.WorkspaceID, SubjectType: p.SubjectType, Scopes: p.Scopes, PolicyGeneration: p.PolicyGeneration, ExpiresAt: auth.ExpiresAt().Unix()}, nil
}

// Revoke revokes a workload identity in the caller's own workspace. The
// workspace comes from the authenticated principal on ctx, never from input.
func (v verifiedIdentity) Revoke(ctx context.Context, issuer, subject string) error {
	auth, ok := identity.FromContext(ctx)
	if !ok || auth.Principal().WorkspaceID == "" {
		return identity.ErrUnauthorized
	}
	if v.catalog == nil {
		return identity.ErrUnauthorized
	}
	err := v.catalog.Revoke(ctx, auth.Principal().WorkspaceID, issuer, subject)
	if errors.Is(err, storage.ErrNotFound) {
		// Missing and foreign identities are indistinguishable (ADR 005).
		return ports.ErrIdentityNotFound
	}
	return err
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
	if errors.Is(err, storage.ErrRevoked) {
		return nil, rest.ErrArtifactRevoked
	}
	if err != nil {
		return nil, err
	}
	return json.Marshal(got)
}

type broker struct {
	endpoint string
	client   *http.Client
}

func newBroker(endpoint string, timeout time.Duration, client *http.Client) *broker {
	if strings.TrimSpace(endpoint) == "" {
		return nil
	}
	u, err := url.Parse(endpoint)
	if err != nil || u.Scheme != "https" || u.Host == "" {
		return nil
	}
	if client == nil {
		client = &http.Client{Timeout: timeout}
	}
	return &broker{endpoint: strings.TrimRight(endpoint, "/"), client: client}
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

var _ rest.ContextAuthenticator = verifiedIdentity{}

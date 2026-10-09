package app

import (
	"context"
	"crypto/hmac"
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
	sessions                  *oauth.CookieSessions
	server                    *http.Server
	closeOnce                 sync.Once
	shutdownDone              chan struct{}
	shutdownErr               error
	janitorCancel             context.CancelFunc
	janitorDone               chan struct{}
	publicationJanitorCancel  context.CancelFunc
	publicationJanitorDone    chan struct{}
	maintenanceMu             sync.Mutex
	maintenanceErr            error
	publicationMaintenanceErr error
}

// New opens the real pgx pool and the object store GIST_OBJECT_STORE_ROOT
// names (a directory, or s3://bucket/prefix). It pings
// PostgreSQL before returning, so a missing integration fixture is an actual
// failure rather than a test skip.
func New(ctx context.Context, cfg Config) (constructed *App, retErr error) {
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
	objects, err := storage.OpenObjectStore(ctx, cfg.ObjectStoreRoot)
	if err != nil {
		pool.Close()
		return nil, err
	}
	defer func() {
		if constructed == nil {
			retErr = errors.Join(retErr, objects.Close())
		}
	}()
	return newWithStores(ctx, cfg, pool, objects)
}

func newWithStores(ctx context.Context, cfg Config, pool *pgxpool.Pool, objects *storage.ObjectStore) (*App, error) {
	if pool == nil || objects == nil {
		return nil, errors.New("app: backing stores are required")
	}
	clock := identityClock{}
	keys, err := identity.LoadKeySet(cfg.SigningKeyConfig, clock)
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
	canonicalResolver, err := NewCanonicalResolver(resolver, cfg.MaxResponseBytes)
	if err != nil {
		pool.Close()
		return nil, err
	}
	broker := newBroker(cfg.BrokerURL, cfg.RequestTimeout, cfg.BrokerClient)
	var connections ports.ConnectionInitiator
	if broker != nil {
		connections = broker
	}
	services := rest.Services{Identity: identityStore, Authorizer: policy, Catalog: catalog, Search: lexicalAdapter{service: search}, Versions: catalog, Artifacts: objects, Resolutions: resolutionStore, Connections: connections, Events: catalog, Publisher: publisher{pool: pool, objects: objects, limits: cfg}, Revocations: artifactRevoker{catalog: catalog}, Resolver: canonicalResolver, Limits: cfg.RESTLimits(), Audience: cfg.ResourceAudience}
	var v2 *publicationV2
	if cfg.PublicationV2 != nil {
		publicationStore, err := storage.NewPublicationStore(pool, objects)
		if err != nil {
			pool.Close()
			return nil, err
		}
		maxPublicationResponse := int64(cfg.MaxResponseBytes)
		if maxPublicationResponse > 100<<20 {
			maxPublicationResponse = 100 << 20
		}
		v2 = &publicationV2{pool: pool, catalog: catalog, store: publicationStore, config: *cfg.PublicationV2, maxResponse: maxPublicationResponse, maxRequest: cfg.MaxRequestBytes, maxPackage: int64(cfg.MaxPackageBytes), maxExpanded: int64(cfg.MaxExpandedBytes)}
		services.V2Publisher, services.V2Reader = v2, v2
	}
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
	// The consent and session keys are derived from the configured secret
	// with domain separation, so every replica agrees on both.
	sessionSecret, consentSecret := deriveKey(cfg.OAuthConsentSecret, "gist oauth session v1"), deriveKey(cfg.OAuthConsentSecret, "gist oauth consent v1")
	sessions, err := oauth.NewCookieSessions(sessionSecret, clock)
	if err != nil {
		pool.Close()
		return nil, err
	}
	minter := oauth.IssuerMinter{Issuers: map[string]*identity.WorkloadIssuer{cfg.ResourceAudience: issuer}, Record: recordIssued(catalog)}
	as, err := oauth.New(oauth.Config{Issuer: cfg.PublicOrigin, Resources: []string{cfg.ResourceAudience}, ScopesSupported: []string{"catalog:read", "catalog:publish"}, Store: catalog.OAuth(), Minter: minter, Policy: policy, Sessions: sessions, Keys: keys, Clock: clock, LoginURL: cfg.OAuthLoginURL, Secret: consentSecret})
	if err != nil {
		pool.Close()
		return nil, err
	}
	prm := as.ProtectedResourceMetadataURL()
	a := &App{cfg: cfg, pool: pool, objects: objects, issuer: issuer, identities: catalog, sessions: sessions}
	// Qualify explicit current maintenance actors and the first bounded purge
	// before accepting traffic. This never enumerates or invents tenant authority.
	maintenanceCtx, stopMaintenance := context.WithTimeout(ctx, cfg.RequestTimeout)
	maintenanceErr := a.purgeEventTargets(maintenanceCtx, catalog)
	stopMaintenance()
	if maintenanceErr != nil {
		pool.Close()
		return nil, maintenanceErr
	}
	handler := requestContext{rest: oauth.WithChallenge(rh, prm), mcp: oauth.WithChallenge(mcp, prm), oauth: as, ready: func(ctx context.Context) error {
		if err := a.maintenanceReady(); err != nil {
			return err
		}
		return pool.Ping(ctx)
	}}
	a.server = &http.Server{Addr: cfg.ListenAddress, Handler: handler, ReadHeaderTimeout: cfg.RequestTimeout}
	if cfg.PublicationV2 != nil {
		if err := a.startPublicationJanitor(ctx, v2); err != nil {
			pool.Close()
			return nil, err
		}
	}
	a.startEventJanitor(ctx, catalog)
	return a, nil

}

// deriveKey derives a purpose-bound 32-byte key from the configured secret.
func deriveKey(secret []byte, purpose string) []byte {
	mac := hmac.New(sha256.New, secret)
	mac.Write([]byte(purpose))
	return mac.Sum(nil)
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
	a.closeOnce.Do(func() {
		a.shutdownDone = make(chan struct{})
		if a.janitorCancel != nil {
			a.janitorCancel()
		}
		if a.publicationJanitorCancel != nil {
			a.publicationJanitorCancel()
		}
		// Cleanup runs exactly once, even if a caller's wait expires. Stop HTTP
		// admission before waiting for the janitor, and keep its pool alive until
		// the janitor has actually returned all acquired connections.
		go func() {
			err := a.server.Shutdown(ctx)
			if err != nil {
				err = errors.Join(err, a.server.Close())
			}
			if a.janitorDone != nil {
				<-a.janitorDone
			}
			if a.publicationJanitorDone != nil {
				<-a.publicationJanitorDone
			}
			err = errors.Join(err, a.objects.Close())
			a.pool.Close()
			a.shutdownErr = err
			close(a.shutdownDone)
		}()
	})
	// Closing shutdownDone publishes shutdownErr to every later caller.
	select {
	case <-a.shutdownDone:
		return a.shutdownErr
	default:
	}
	select {
	case <-a.shutdownDone:
		return a.shutdownErr
	case <-ctx.Done():
		return ctx.Err()
	}
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
	// Closing this read-only response stream cannot change the received result.
	defer func() { _ = resp.Body.Close() }()
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

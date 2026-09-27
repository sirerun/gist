package rest

import (
	"context"
	"encoding/json"
	"errors"
	"io"
	"net/http"
	"strings"

	"github.com/sirerun/gist/hosted/internal/ports"
)

type Publisher interface {
	Publish(context.Context, ports.Principal, ports.ArtifactKind, []byte) ([]byte, error)
}
type Resolver interface {
	Resolve(context.Context, ports.Principal, []byte) ([]byte, error)
}

// VersionLister returns one page of the visible versions of one artifact,
// identified by workspace, kind and id (Version is ignored). Versions come in
// SemVer precedence order; after is the last version of the previous page
// ("" for the first page) and limit caps the page size. It is not paged by the
// catalog-wide search cap, so no version is dropped because unrelated records
// filled a page. *storage.Postgres is the production implementation.
type VersionLister interface {
	ListVersions(ctx context.Context, ref ports.ArtifactRef, after string, limit int) ([]ports.CatalogRecord, error)
}

type Services struct {
	Identity    ports.IdentityStore
	Authorizer  ports.Authorizer
	Catalog     ports.CatalogStore
	Search      ports.LexicalSearcher
	Versions    VersionLister
	Artifacts   ports.ArtifactStore
	Resolutions ports.ResolutionStore
	Connections ports.ConnectionInitiator
	Events      ports.EventStore
	Publisher   Publisher
	Resolver    Resolver
	Limits      Limits
	Audience    string
}
type Handler struct {
	s      Services
	limits Limits
	// slots caps in-flight requests at limits.RateLimit (fed from
	// Config.MaxConcurrentRequests); a full semaphore answers 429 rate_limited.
	slots chan struct{}
}

func New(s Services) (*Handler, error) {
	if s.Identity == nil || s.Authorizer == nil {
		return nil, errors.New("rest: identity and authorizer are required")
	}
	s.Limits = s.Limits.withDefaults()
	return &Handler{s: s, limits: s.Limits, slots: make(chan struct{}, s.Limits.RateLimit)}, nil
}
func (h *Handler) ServeHTTP(w http.ResponseWriter, r *http.Request) {
	id, err := requestID(r)
	if err != nil {
		writeError(w, err, "req_invalid")
		return
	}
	w.Header().Set("X-Request-ID", id)
	if r.Method == http.MethodOptions {
		w.WriteHeader(http.StatusNoContent)
		return
	}
	// The in-flight slot is taken before authentication so a flood of
	// unauthenticated requests is bounded by the same cap as everything else.
	// The cap is global, not per tenant.
	select {
	case h.slots <- struct{}{}:
		defer func() { <-h.slots }()
	default:
		writeError(w, appErrorWithRetry("rate_limited", "Rate limit exceeded", 429, true, h.limits.RetryAfter), id)
		return
	}
	ctx, p, err := h.authenticate(r)
	if err != nil {
		if statusFor(err) == 401 {
			w.Header().Set("WWW-Authenticate", `Bearer realm="gist"`)
		}
		writeError(w, err, id)
		return
	}
	if r.ContentLength > h.limits.MaxBodyBytes {
		writeError(w, appError("budget_exceeded", "Request exceeds the requested byte budget", 413, false), id)
		return
	}
	r = r.WithContext(ctx)
	path := strings.TrimPrefix(r.URL.Path, "/")
	parts := strings.Split(path, "/")
	if strings.Contains(r.URL.RawPath, "%2f") || strings.Contains(r.URL.RawPath, "%2F") || strings.Contains(path, "..") {
		writeError(w, appError("validation_failed", "Invalid request", 422, false), id)
		return
	}
	var routeErr error
	switch {
	case r.Method == "POST" && path == "v1/discover":
		routeErr = h.discover(w, r, p)
	case r.Method == "GET" && len(parts) == 4 && parts[0] == "v1" && parts[1] == "skills" && parts[3] == "versions":
		routeErr = h.listVersions(w, r, p, parts[2])
	case r.Method == "GET" && len(parts) == 5 && parts[0] == "v1" && parts[1] == "skills" && parts[3] == "versions":
		routeErr = h.getArtifact(w, r, p, ports.KindSkill, parts[2], parts[4])
	case r.Method == "GET" && len(parts) == 6 && parts[0] == "v1" && parts[1] == "skills" && parts[3] == "versions" && parts[5] == "package":
		routeErr = h.download(w, r, p, parts[2], parts[4])
	case r.Method == "GET" && len(parts) == 5 && parts[0] == "v1" && parts[1] == "tools" && parts[3] == "versions":
		routeErr = h.getArtifact(w, r, p, ports.KindTool, parts[2], parts[4])
	case r.Method == "GET" && len(parts) == 5 && parts[0] == "v1" && parts[1] == "capabilities" && parts[3] == "versions":
		routeErr = h.getArtifact(w, r, p, ports.KindCapability, parts[2], parts[4])
	case r.Method == "POST" && path == "v1/resolve":
		routeErr = h.resolve(w, r, p)
	case r.Method == "POST" && path == "v1/connections":
		routeErr = h.createConnection(w, r, p)
	case r.Method == "GET" && len(parts) == 3 && parts[0] == "v1" && parts[1] == "connections":
		routeErr = h.getConnection(w, r, p, parts[2])
	case r.Method == "GET" && path == "v1/taxonomies":
		routeErr = h.listByKind(w, r, p, ports.KindTaxonomy)
	case r.Method == "GET" && len(parts) == 4 && parts[0] == "v1" && parts[1] == "taxonomies" && parts[3] == "nodes":
		routeErr = h.getArtifact(w, r, p, ports.KindTaxonomy, parts[2], "")
	case r.Method == "GET" && path == "v1/events":
		routeErr = h.events(w, r, p)
	case r.Method == "POST" && path == "v1/artifacts/batch-get":
		routeErr = h.batch(w, r, p)
	case r.Method == "POST" && path == "v1/identities/revoke":
		routeErr = h.revokeIdentity(w, r, p)
	case r.Method == "POST" && len(parts) == 3 && parts[0] == "v1" && parts[1] == "publish":
		routeErr = h.publish(w, r, p, parts[2])
	default:
		routeErr = appError("not_found", "Not found", 404, false)
	}
	if routeErr != nil {
		writeError(w, routeErr, id)
	}
}

// ContextAuthenticator is optionally implemented by Services.Identity. When
// present, authentication also yields a context carrying the verified
// identity, which workspace-bound operations such as Identity.Revoke read the
// caller's workspace from.
type ContextAuthenticator interface {
	AuthenticateContext(ctx context.Context, token, audience string) (context.Context, ports.IdentityRecord, error)
}

// Authenticate resolves the bearer token on r to a principal with the same
// identity check the REST routes use. Other transports (remote MCP) call it
// before allocating any per-client state.
func (h *Handler) Authenticate(r *http.Request) (ports.Principal, error) {
	_, p, err := h.authenticate(r)
	return p, err
}
func (h *Handler) authenticate(r *http.Request) (context.Context, ports.Principal, error) {
	ctx := r.Context()
	a := r.Header.Get("Authorization")
	if !strings.HasPrefix(a, "Bearer ") || len(strings.TrimSpace(strings.TrimPrefix(a, "Bearer "))) == 0 {
		return ctx, ports.Principal{}, appError("unauthorized", "Authentication required", 401, false)
	}
	token := strings.TrimSpace(strings.TrimPrefix(a, "Bearer "))
	var rec ports.IdentityRecord
	var err error
	if ca, ok := h.s.Identity.(ContextAuthenticator); ok {
		ctx, rec, err = ca.AuthenticateContext(ctx, token, h.s.Audience)
	} else {
		rec, err = h.s.Identity.Lookup(ctx, token, h.s.Audience)
	}
	if err != nil {
		return r.Context(), ports.Principal{}, appError("unauthorized", "Authentication required", 401, false)
	}
	return ctx, ports.Principal{Issuer: rec.Issuer, Subject: rec.Subject, Audience: h.s.Audience, WorkspaceID: rec.WorkspaceID, Scopes: rec.Scopes, PolicyGeneration: rec.PolicyGeneration, SubjectType: rec.SubjectType}, nil
}
func (h *Handler) authorize(ctx context.Context, p ports.Principal, action ports.Action, ref *ports.ArtifactRef) error {
	d, err := h.s.Authorizer.Decide(ctx, p, action, ref)
	if err != nil {
		return appError("service_unavailable", "Service unavailable", 503, true)
	}
	if d.Allowed {
		return nil
	}
	if d.Status == 404 {
		return appError("not_found", "Not found", 404, false)
	}
	if d.Status == 409 {
		return appError("version_conflict", "Version conflict", 409, false)
	}
	if d.Status == 429 {
		return appErrorWithRetry("rate_limited", "Rate limit exceeded", 429, true, h.limits.RetryAfter)
	}
	if d.Status == 503 {
		return appErrorWithRetry("service_unavailable", "Service unavailable", 503, true, h.limits.RetryAfter)
	}
	return appError("forbidden", "Forbidden", 403, false)
}
func decodeBody(r *http.Request, limit int64, dst any) error {
	dec := json.NewDecoder(io.LimitReader(r.Body, limit+1))
	if err := dec.Decode(dst); err != nil {
		return appError("validation_failed", "Invalid request", 422, false)
	}
	return nil
}

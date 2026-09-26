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

type Services struct {
	Identity    ports.IdentityStore
	Authorizer  ports.Authorizer
	Catalog     ports.CatalogStore
	Search      ports.LexicalSearcher
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
}

func New(s Services) (*Handler, error) {
	if s.Identity == nil || s.Authorizer == nil {
		return nil, errors.New("rest: identity and authorizer are required")
	}
	s.Limits = s.Limits.withDefaults()
	return &Handler{s: s, limits: s.Limits}, nil
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
	p, err := h.authenticate(r)
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
	case r.Method == "POST" && len(parts) == 3 && parts[0] == "v1" && parts[1] == "publish":
		routeErr = h.publish(w, r, p, parts[2])
	default:
		routeErr = appError("not_found", "Not found", 404, false)
	}
	if routeErr != nil {
		writeError(w, routeErr, id)
	}
}

func (h *Handler) authenticate(r *http.Request) (ports.Principal, error) {
	a := r.Header.Get("Authorization")
	if !strings.HasPrefix(a, "Bearer ") || len(strings.TrimSpace(strings.TrimPrefix(a, "Bearer "))) == 0 {
		return ports.Principal{}, appError("unauthorized", "Authentication required", 401, false)
	}
	token := strings.TrimSpace(strings.TrimPrefix(a, "Bearer "))
	rec, err := h.s.Identity.Lookup(r.Context(), token, h.s.Audience)
	if err != nil {
		return ports.Principal{}, appError("unauthorized", "Authentication required", 401, false)
	}
	return ports.Principal{Issuer: rec.Issuer, Subject: rec.Subject, Audience: h.s.Audience, WorkspaceID: rec.WorkspaceID, Scopes: rec.Scopes, PolicyGeneration: rec.PolicyGeneration, SubjectType: rec.SubjectType}, nil
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

package rest

import (
	"context"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	"github.com/sirerun/gist/hosted/internal/ports"
)

type revokeCtxKey struct{}

// revokeIdentity is a workspace-scoped identity store double. Like the app's
// verifiedIdentity, Revoke reads the caller's workspace only from the context
// produced by AuthenticateContext.
type revokeIdentity struct {
	tokens     map[string]ports.IdentityRecord
	identities map[string]bool // workspace|issuer|subject -> revoked
	calls      int
}

func revokeKey(ws, iss, sub string) string { return ws + "|" + iss + "|" + sub }

func (s *revokeIdentity) Lookup(ctx context.Context, token, audience string) (ports.IdentityRecord, error) {
	_, rec, err := s.AuthenticateContext(ctx, token, audience)
	return rec, err
}

func (s *revokeIdentity) AuthenticateContext(ctx context.Context, token, _ string) (context.Context, ports.IdentityRecord, error) {
	rec, ok := s.tokens[token]
	if !ok {
		return ctx, ports.IdentityRecord{}, context.Canceled
	}
	return context.WithValue(ctx, revokeCtxKey{}, rec.WorkspaceID), rec, nil
}

func (s *revokeIdentity) Revoke(ctx context.Context, issuer, subject string) error {
	s.calls++
	ws, _ := ctx.Value(revokeCtxKey{}).(string)
	if ws == "" {
		return context.Canceled
	}
	k := revokeKey(ws, issuer, subject)
	if _, ok := s.identities[k]; !ok {
		return ports.ErrIdentityNotFound
	}
	s.identities[k] = true
	return nil
}

// scopePolicy allows an action only when the principal holds its scope.
type scopePolicy struct{}

func (scopePolicy) Decide(_ context.Context, p ports.Principal, a ports.Action, _ *ports.ArtifactRef) (ports.Decision, error) {
	for _, s := range p.Scopes {
		if s == string(a) {
			return ports.Decision{Allowed: true, Status: 200}, nil
		}
	}
	return ports.Decision{Status: 403}, nil
}

const revokeIss = "https://issuer.test"

func newRevokeHandler(t *testing.T) (*Handler, *revokeIdentity) {
	t.Helper()
	id := &revokeIdentity{
		tokens: map[string]ports.IdentityRecord{
			"admin-a":  {Issuer: revokeIss, Subject: "admin-a", WorkspaceID: "ws-a", Scopes: []string{"identity:revoke"}},
			"reader-a": {Issuer: revokeIss, Subject: "reader-a", WorkspaceID: "ws-a", Scopes: []string{"catalog:read", "catalog:publish"}},
			"admin-b":  {Issuer: revokeIss, Subject: "admin-b", WorkspaceID: "ws-b", Scopes: []string{"identity:revoke"}},
		},
		identities: map[string]bool{
			revokeKey("ws-a", revokeIss, "worker-a"): false,
			revokeKey("ws-b", revokeIss, "worker-b"): false,
		},
	}
	h, err := New(Services{Identity: id, Authorizer: scopePolicy{}, Audience: "aud"})
	if err != nil {
		t.Fatal(err)
	}
	return h, id
}

func doRevoke(h *Handler, token, body string) *httptest.ResponseRecorder {
	req := httptest.NewRequest(http.MethodPost, "/v1/identities/revoke", strings.NewReader(body))
	if token != "" {
		req.Header.Set("Authorization", "Bearer "+token)
	}
	w := httptest.NewRecorder()
	h.ServeHTTP(w, req)
	return w
}

func errCode(t *testing.T, w *httptest.ResponseRecorder) string {
	t.Helper()
	var e Error
	if err := json.Unmarshal(w.Body.Bytes(), &e); err != nil {
		t.Fatalf("error body %q: %v", w.Body.String(), err)
	}
	if e.RequestID == "" || e.Message == "" {
		t.Fatalf("error envelope incomplete: %+v", e)
	}
	return e.Code
}

func TestRevokeIdentitySucceedsAndIsIdempotent(t *testing.T) {
	h, id := newRevokeHandler(t)
	body := `{"issuer":"https://issuer.test","subject":"worker-a"}`
	for i := 0; i < 2; i++ {
		w := doRevoke(h, "admin-a", body)
		if w.Code != http.StatusOK {
			t.Fatalf("attempt %d status=%d body=%s", i, w.Code, w.Body)
		}
		var out struct {
			Issuer, Subject string
			Revoked         bool
		}
		if err := json.Unmarshal(w.Body.Bytes(), &out); err != nil || !out.Revoked || out.Subject != "worker-a" || out.Issuer != revokeIss {
			t.Fatalf("attempt %d body=%s err=%v", i, w.Body, err)
		}
	}
	if !id.identities[revokeKey("ws-a", revokeIss, "worker-a")] {
		t.Fatal("identity was not revoked")
	}
}

func TestRevokeIdentityRequiresScope(t *testing.T) {
	h, id := newRevokeHandler(t)
	w := doRevoke(h, "reader-a", `{"issuer":"https://issuer.test","subject":"worker-a"}`)
	if w.Code != http.StatusForbidden || errCode(t, w) != "forbidden" {
		t.Fatalf("status=%d body=%s", w.Code, w.Body)
	}
	if id.calls != 0 || id.identities[revokeKey("ws-a", revokeIss, "worker-a")] {
		t.Fatal("revoke reached the store without the identity:revoke scope")
	}
}

func TestRevokeIdentityRequiresAuthentication(t *testing.T) {
	h, id := newRevokeHandler(t)
	for _, tok := range []string{"", "unknown-token"} {
		w := doRevoke(h, tok, `{"issuer":"https://issuer.test","subject":"worker-a"}`)
		if w.Code != http.StatusUnauthorized || errCode(t, w) != "unauthorized" || w.Header().Get("WWW-Authenticate") == "" {
			t.Fatalf("token %q status=%d body=%s", tok, w.Code, w.Body)
		}
	}
	if id.calls != 0 {
		t.Fatal("unauthenticated revoke reached the store")
	}
}

func TestRevokeIdentityCannotCrossWorkspaces(t *testing.T) {
	h, id := newRevokeHandler(t)
	// A workspace-B admin naming workspace A's identity sees a uniform 404,
	// identical to a nonexistent identity.
	foreign := doRevoke(h, "admin-b", `{"issuer":"https://issuer.test","subject":"worker-a"}`)
	missing := doRevoke(h, "admin-b", `{"issuer":"https://issuer.test","subject":"nobody"}`)
	for _, w := range []*httptest.ResponseRecorder{foreign, missing} {
		if w.Code != http.StatusNotFound || errCode(t, w) != "not_found" {
			t.Fatalf("status=%d body=%s", w.Code, w.Body)
		}
	}
	// The body cannot select a workspace.
	w := doRevoke(h, "admin-b", `{"workspace_id":"ws-a","issuer":"https://issuer.test","subject":"worker-a"}`)
	if w.Code != http.StatusUnprocessableEntity || errCode(t, w) != "validation_failed" {
		t.Fatalf("workspace in body status=%d body=%s", w.Code, w.Body)
	}
	if id.identities[revokeKey("ws-a", revokeIss, "worker-a")] {
		t.Fatal("workspace A identity was revoked by a workspace B caller")
	}
}

func TestRevokeIdentityValidatesBody(t *testing.T) {
	h, id := newRevokeHandler(t)
	for _, body := range []string{``, `{}`, `{"issuer":"https://issuer.test"}`, `{"subject":"worker-a"}`, `{"issuer":"x","subject":"y"} {}`} {
		w := doRevoke(h, "admin-a", body)
		if w.Code != http.StatusUnprocessableEntity || errCode(t, w) != "validation_failed" {
			t.Fatalf("body %q status=%d resp=%s", body, w.Code, w.Body)
		}
	}
	if id.calls != 0 {
		t.Fatal("invalid body reached the store")
	}
}

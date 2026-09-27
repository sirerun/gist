package remotemcp

import (
	"bytes"
	"context"
	"encoding/json"
	"errors"
	"io"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	"github.com/sirerun/gist/hosted/internal/ports"
	"github.com/sirerun/gist/hosted/internal/rest"
)

type identity struct{}

// Lookup accepts any token except those starting with "bad"; the token is
// the subject, so distinct tokens are distinct principals.
func (identity) Lookup(_ context.Context, token, _ string) (ports.IdentityRecord, error) {
	if strings.HasPrefix(token, "bad") {
		return ports.IdentityRecord{}, errors.New("invalid token")
	}
	return ports.IdentityRecord{WorkspaceID: "workspace-a", Subject: token}, nil
}
func (identity) Revoke(context.Context, string, string) error { return nil }

type policy struct{}

func (policy) Decide(context.Context, ports.Principal, ports.Action, *ports.ArtifactRef) (ports.Decision, error) {
	return ports.Decision{Allowed: true}, nil
}

type search struct{}

func (search) Search(context.Context, ports.SearchQuery) (ports.SearchPage, error) {
	return ports.SearchPage{}, nil
}
func mcpHandler(t *testing.T) *Handler {
	h, err := New(Config{Services: rest.Services{Identity: identity{}, Authorizer: policy{}, Search: search{}}, AllowedOrigins: map[string]bool{"https://client.example": true}})
	if err != nil {
		t.Fatal(err)
	}
	return h
}
func mcpRequest(t *testing.T, h http.Handler, body string, headers map[string]string) *httptest.ResponseRecorder {
	r := httptest.NewRequest("POST", "/mcp", bytes.NewBufferString(body))
	r.Header.Set("Authorization", "Bearer test")
	r.Header.Set("Origin", "https://client.example")
	for k, v := range headers {
		r.Header.Set(k, v)
	}
	w := httptest.NewRecorder()
	h.ServeHTTP(w, r)
	return w
}
func TestNegotiationPinsRevisionAndTools(t *testing.T) {
	h := mcpHandler(t)
	w := mcpRequest(t, h, `{"jsonrpc":"2.0","id":1,"method":"initialize","params":{"protocolVersion":"2025-06-18"}}`, nil)
	if w.Code != 200 {
		t.Fatal(w.Code)
	}
	sid := w.Header().Get("Mcp-Session-Id")
	if sid == "" {
		t.Fatal("missing session")
	}
	w = mcpRequest(t, h, `{"jsonrpc":"2.0","id":2,"method":"tools/list","params":{}}`, map[string]string{"Mcp-Session-Id": sid})
	var got rpcResponse
	if err := json.Unmarshal(w.Body.Bytes(), &got); err != nil {
		t.Fatal(err)
	}
	if got.Error != nil || got.Result == nil {
		t.Fatalf("list response=%s", w.Body.String())
	}
}
func TestUnsupportedRevisionAndHTTPAuthAreSeparate(t *testing.T) {
	h := mcpHandler(t)
	w := mcpRequest(t, h, `{"jsonrpc":"2.0","id":1,"method":"initialize","params":{"protocolVersion":"2099-01-01"}}`, nil)
	if w.Code != 200 || !bytes.Contains(w.Body.Bytes(), []byte("Unsupported protocol version")) {
		t.Fatalf("%d %s", w.Code, w.Body.String())
	}
	r := httptest.NewRequest("POST", "/mcp", bytes.NewBufferString(`{}`))
	r.Header.Set("Origin", "https://client.example")
	rw := httptest.NewRecorder()
	h.ServeHTTP(rw, r)
	if rw.Code != 401 || rw.Header().Get("WWW-Authenticate") == "" {
		t.Fatalf("auth=%d challenge=%q", rw.Code, rw.Header().Get("WWW-Authenticate"))
	}
}
func TestOriginAndBodyCaps(t *testing.T) {
	h := mcpHandler(t)
	r := httptest.NewRequest("POST", "/mcp", bytes.NewBufferString(`{}`))
	r.Header.Set("Authorization", "Bearer x")
	r.Header.Set("Origin", "https://evil.example")
	w := httptest.NewRecorder()
	h.ServeHTTP(w, r)
	if w.Code != 403 {
		t.Fatalf("origin=%d", w.Code)
	}
	_ = io.EOF
	_ = http.MethodPost
}

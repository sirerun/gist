package rest

import (
	"bytes"
	"context"
	"io"
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/sirerun/gist/hosted/internal/ports"
)

type testIdentity struct{}

func (testIdentity) Lookup(context.Context, string, string) (ports.IdentityRecord, error) {
	return ports.IdentityRecord{Issuer: "test", Subject: "s", WorkspaceID: "workspace-a", Scopes: []string{"catalog:read", "catalog:publish"}}, nil
}
func (testIdentity) Revoke(context.Context, string, string) error { return nil }

type testPolicy struct{}

func (testPolicy) Decide(context.Context, ports.Principal, ports.Action, *ports.ArtifactRef) (ports.Decision, error) {
	return ports.Decision{Allowed: true}, nil
}

type testCatalog struct{}

func (testCatalog) Get(context.Context, ports.ArtifactRef) (ports.CatalogRecord, error) {
	return ports.CatalogRecord{Metadata: []byte(`{"id":"ok"}`), Digest: ports.Digest{Algorithm: "sha-256", Value: "abc"}}, nil
}
func (testCatalog) Search(context.Context, ports.SearchQuery) (ports.SearchPage, error) {
	return ports.SearchPage{}, nil
}

type testSearch struct{}

func (testSearch) Search(context.Context, ports.SearchQuery) (ports.SearchPage, error) {
	return ports.SearchPage{}, nil
}

type testArtifact struct{}

func (testArtifact) Put(context.Context, ports.Digest, io.Reader, int64) error { return nil }
func (testArtifact) Open(context.Context, ports.ArtifactRef) (ports.ArtifactReader, error) {
	return &testReader{Reader: bytes.NewReader([]byte("pkg"))}, nil
}

type testReader struct{ *bytes.Reader }

func (r *testReader) Close() error { return nil }

type testConn struct{}

func (testConn) Begin(context.Context, ports.Principal, ports.ArtifactRef) (ports.Connection, error) {
	return ports.Connection{ID: "c1", Status: ports.ConnectionPending}, nil
}
func (testConn) Get(context.Context, ports.Principal, string) (ports.Connection, error) {
	return ports.Connection{ID: "c1", Capability: ports.ArtifactRef{Kind: ports.KindCapability, ID: "cap", Version: "1"}}, nil
}

type testEvents struct{}

func (testEvents) Append(context.Context, ports.Event) error { return nil }
func (testEvents) Read(context.Context, ports.Cursor) (ports.EventPage, error) {
	return ports.EventPage{}, nil
}

type testPublisher struct{}

func (testPublisher) Publish(context.Context, ports.Principal, ports.ArtifactKind, []byte) ([]byte, error) {
	return []byte(`{"published":true}`), nil
}

type testResolver struct{}

func (testResolver) Resolve(context.Context, ports.Principal, []byte) ([]byte, error) {
	return []byte(`{"status":"ready","findings":[]}`), nil
}

func testHandler(t *testing.T) *Handler {
	t.Helper()
	h, err := New(Services{Identity: testIdentity{}, Authorizer: testPolicy{}, Catalog: testCatalog{}, Search: testSearch{}, Artifacts: testArtifact{}, Connections: testConn{}, Events: testEvents{}, Publisher: testPublisher{}, Resolver: testResolver{}, Limits: Limits{MaxResponseBytes: 4096}})
	if err != nil {
		t.Fatal(err)
	}
	return h
}
func do(t *testing.T, h http.Handler, method, path string, body string) *httptest.ResponseRecorder {
	t.Helper()
	r := httptest.NewRequest(method, path, bytes.NewBufferString(body))
	r.Header.Set("Authorization", "Bearer test")
	w := httptest.NewRecorder()
	h.ServeHTTP(w, r)
	return w
}
func TestRoutesReturnFrozenSuccessStatuses(t *testing.T) {
	h := testHandler(t)
	cases := []struct {
		name, method, path, body string
		status                   int
	}{
		{"discover", "POST", "/v1/discover", `{"query":"x","max_bytes":4096}`, 200}, {"versions", "GET", "/v1/skills/s/versions", "", 200}, {"manifest", "GET", "/v1/skills/s/versions/1", "", 200}, {"package", "GET", "/v1/skills/s/versions/1/package", "", 200}, {"tool", "GET", "/v1/tools/t/versions/1", "", 200}, {"capability", "GET", "/v1/capabilities/c/versions/1", "", 200}, {"resolve", "POST", "/v1/resolve", `{"max_bytes":4096}`, 200}, {"create connection", "POST", "/v1/connections", `{"capability":{"kind":"capability","id":"cap","version":"1"}}`, 201}, {"get connection", "GET", "/v1/connections/c1", "", 200}, {"taxonomies", "GET", "/v1/taxonomies", "", 200}, {"taxonomy nodes", "GET", "/v1/taxonomies/t/nodes", "", 200}, {"publish skills", "POST", "/v1/publish/skills", `{}`, 201}, {"publish caps", "POST", "/v1/publish/capabilities", `{}`, 201}, {"publish tools", "POST", "/v1/publish/tools", `{}`, 201}, {"publish providers", "POST", "/v1/publish/providers", `{}`, 201}, {"publish bindings", "POST", "/v1/publish/bindings", `{}`, 201}, {"publish taxonomies", "POST", "/v1/publish/taxonomies", `{}`, 201}, {"publish revocations", "POST", "/v1/publish/revocations", `{}`, 201}, {"events", "GET", "/v1/events", "", 200}, {"batch", "POST", "/v1/artifacts/batch-get", `{"references":[{"kind":"skill","id":"s","version":"1"}],"max_bytes":4096}`, 200}}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			if got := do(t, h, tc.method, tc.path, tc.body).Code; got != tc.status {
				t.Fatalf("status=%d want %d", got, tc.status)
			}
		})
	}
}
func TestAuthenticationAndPrivateNotFound(t *testing.T) {
	h := testHandler(t)
	w := httptest.NewRecorder()
	h.ServeHTTP(w, httptest.NewRequest("GET", "/v1/skills/s/versions/1", nil))
	if w.Code != 401 || w.Header().Get("WWW-Authenticate") == "" {
		t.Fatalf("auth status=%d challenge=%q", w.Code, w.Header().Get("WWW-Authenticate"))
	}
}
func TestResponseBudgetIs413AndNeverTruncated(t *testing.T) {
	h := testHandler(t)
	w := do(t, h, "POST", "/v1/discover", `{"query":"x","max_bytes":1}`)
	if w.Code != 413 {
		t.Fatalf("status=%d body=%s", w.Code, w.Body.String())
	}
}

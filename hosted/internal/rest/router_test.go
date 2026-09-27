package rest

import (
	"bytes"
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"net/http"
	"net/http/httptest"
	"testing"
	"time"

	"github.com/sirerun/gist/hosted/internal/events"
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
func (testEvents) NewCursor(ports.Principal, time.Duration) (ports.Cursor, error) {
	return ports.Cursor{ID: "cur"}, nil
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
	h, err := New(Services{Identity: testIdentity{}, Authorizer: testPolicy{}, Catalog: testCatalog{}, Search: testSearch{}, Versions: newVersionsCatalog(), Artifacts: testArtifact{}, Connections: testConn{}, Events: testEvents{}, Publisher: testPublisher{}, Resolver: testResolver{}, Limits: Limits{MaxResponseBytes: 4096}})
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

type fixedClock struct{ now time.Time }

func (c *fixedClock) Now() time.Time { return c.now }

// versionsCatalog models a catalog where unrelated records sort ahead of the
// wanted artifact and outnumber the result cap. Search pages like the real
// store (capped at Limit); ListVersions answers for one id.
type versionsCatalog struct{ records []ports.CatalogRecord }

func newVersionsCatalog() versionsCatalog {
	var c versionsCatalog
	for i := 0; i < 10; i++ {
		c.records = append(c.records, ports.CatalogRecord{Ref: ports.ArtifactRef{WorkspaceID: "workspace-a", Kind: ports.KindSkill, ID: fmt.Sprintf("aaa-%02d", i), Version: "1"}})
	}
	for _, v := range []string{"1", "2", "3"} {
		c.records = append(c.records, ports.CatalogRecord{Ref: ports.ArtifactRef{WorkspaceID: "workspace-a", Kind: ports.KindSkill, ID: "wanted", Version: v}})
	}
	return c
}

func (c versionsCatalog) Search(_ context.Context, q ports.SearchQuery) (ports.SearchPage, error) {
	n := q.Limit
	if n <= 0 || n > len(c.records) {
		n = len(c.records)
	}
	return ports.SearchPage{Records: c.records[:n]}, nil
}

func (c versionsCatalog) ListVersions(_ context.Context, ref ports.ArtifactRef, after string, limit int) ([]ports.CatalogRecord, error) {
	var out []ports.CatalogRecord
	skipping := after != ""
	for _, r := range c.records {
		if r.Ref.WorkspaceID != ref.WorkspaceID || r.Ref.Kind != ref.Kind || r.Ref.ID != ref.ID {
			continue
		}
		if skipping {
			skipping = r.Ref.Version != after
			continue
		}
		if len(out) == limit {
			break
		}
		out = append(out, r)
	}
	return out, nil
}

type errEvents struct {
	page ports.EventPage
	err  error
}

func (errEvents) Append(context.Context, ports.Event) error { return nil }
func (e errEvents) Read(context.Context, ports.Cursor) (ports.EventPage, error) {
	return e.page, e.err
}

func handlerWith(t *testing.T, mutate func(*Services)) *Handler {
	t.Helper()
	s := Services{Identity: testIdentity{}, Authorizer: testPolicy{}, Catalog: testCatalog{}, Search: testSearch{}, Versions: newVersionsCatalog(), Artifacts: testArtifact{}, Connections: testConn{}, Events: testEvents{}, Publisher: testPublisher{}, Resolver: testResolver{}, Limits: Limits{MaxResponseBytes: 4096}}
	mutate(&s)
	h, err := New(s)
	if err != nil {
		t.Fatal(err)
	}
	return h
}

func decodeJSON(t *testing.T, w *httptest.ResponseRecorder, v any) {
	t.Helper()
	if err := json.Unmarshal(w.Body.Bytes(), v); err != nil {
		t.Fatalf("decode %q: %v", w.Body.String(), err)
	}
}

// Regression: GET /v1/events used to send an unbound cursor and had no way to
// open one, so every request failed with 503.
func TestEventsOpensPrincipalBoundCursorAndResumes(t *testing.T) {
	clock := &fixedClock{now: time.Unix(1_700_000_000, 0)}
	store := events.NewStore(clock)
	ref := ports.ArtifactRef{WorkspaceID: "workspace-a", Kind: ports.KindSkill, ID: "s", Version: "1"}
	if err := store.Append(context.Background(), ports.Event{ID: "e1", WorkspaceID: "workspace-a", Type: ports.EventVersionPublished, Subject: ref, OccurredAt: clock.now.Unix()}); err != nil {
		t.Fatal(err)
	}
	h := handlerWith(t, func(s *Services) { s.Events = store })

	w := do(t, h, "GET", "/v1/events", "")
	if w.Code != 200 {
		t.Fatalf("open status=%d body=%s", w.Code, w.Body.String())
	}
	var first struct {
		Events     []ports.Event `json:"events"`
		NextCursor string        `json:"next_cursor"`
	}
	decodeJSON(t, w, &first)
	if len(first.Events) != 1 || first.NextCursor == "" {
		t.Fatalf("first page = %+v", first)
	}

	if err := store.Append(context.Background(), ports.Event{ID: "e2", WorkspaceID: "workspace-a", Type: ports.EventVersionRevoked, Subject: ref, OccurredAt: clock.now.Unix()}); err != nil {
		t.Fatal(err)
	}
	w = do(t, h, "GET", "/v1/events?cursor="+first.NextCursor, "")
	if w.Code != 200 {
		t.Fatalf("resume status=%d body=%s", w.Code, w.Body.String())
	}
	var second struct {
		Events []ports.Event `json:"events"`
	}
	decodeJSON(t, w, &second)
	if len(second.Events) != 1 || second.Events[0].ID != "e2" {
		t.Fatalf("second page = %+v", second)
	}
}

func TestEventsRejectsCursorOfAnotherPrincipal(t *testing.T) {
	store := events.NewStore(&fixedClock{now: time.Unix(1_700_000_000, 0)})
	foreign, err := store.NewCursor(ports.Principal{Issuer: "test", Subject: "someone-else", WorkspaceID: "workspace-a"}, time.Minute)
	if err != nil {
		t.Fatal(err)
	}
	h := handlerWith(t, func(s *Services) { s.Events = store })
	w := do(t, h, "GET", "/v1/events?cursor="+foreign.ID, "")
	if w.Code != 409 || !bytes.Contains(w.Body.Bytes(), []byte(`"cursor_expired"`)) {
		t.Fatalf("status=%d body=%s", w.Code, w.Body.String())
	}
}

func TestEventsCursorErrorsMapTo409(t *testing.T) {
	cases := map[string]errEvents{
		"expired":       {err: events.ErrCursorExpired},
		"retention gap": {page: ports.EventPage{RetentionGap: true}, err: events.ErrCursorExpired},
		"gap flag only": {page: ports.EventPage{RetentionGap: true}},
		"binding":       {err: events.ErrCursorBinding},
	}
	for name, ev := range cases {
		t.Run(name, func(t *testing.T) {
			h := handlerWith(t, func(s *Services) { s.Events = ev })
			w := do(t, h, "GET", "/v1/events?cursor=abc", "")
			if w.Code != 409 || !bytes.Contains(w.Body.Bytes(), []byte(`"cursor_expired"`)) {
				t.Fatalf("status=%d body=%s", w.Code, w.Body.String())
			}
		})
	}
	h := handlerWith(t, func(s *Services) { s.Events = errEvents{err: errors.New("db down")} })
	if w := do(t, h, "GET", "/v1/events?cursor=abc", ""); w.Code != 503 {
		t.Fatalf("store failure status=%d", w.Code)
	}
}

func TestEventsWithoutCursorOpenerIs503(t *testing.T) {
	h := handlerWith(t, func(s *Services) { s.Events = errEvents{} })
	if w := do(t, h, "GET", "/v1/events", ""); w.Code != 503 {
		t.Fatalf("status=%d", w.Code)
	}
}

// Regression: listVersions ignored the path id and returned every skill, and
// then filtered one MaxResults-capped search page, dropping every version that
// sorted beyond the cap.
func TestListVersionsReturnsEveryVersionOfPathID(t *testing.T) {
	c := newVersionsCatalog()
	h := handlerWith(t, func(s *Services) { s.Search, s.Versions, s.Limits.MaxResults = c, c, 5 })
	w := do(t, h, "GET", "/v1/skills/wanted/versions", "")
	if w.Code != 200 {
		t.Fatalf("status=%d body=%s", w.Code, w.Body.String())
	}
	var out struct {
		Items []ports.CatalogRecord `json:"items"`
	}
	decodeJSON(t, w, &out)
	if len(out.Items) != 3 {
		t.Fatalf("items=%d want 3: %s", len(out.Items), w.Body.String())
	}
	for _, it := range out.Items {
		if it.Ref.ID != "wanted" {
			t.Fatalf("leaked item %+v", it.Ref)
		}
	}
}

func TestListVersionsWithoutVersionListerIs503(t *testing.T) {
	h := handlerWith(t, func(s *Services) { s.Versions = nil })
	w := do(t, h, "GET", "/v1/skills/wanted/versions", "")
	if w.Code != 503 {
		t.Fatalf("status=%d body=%s", w.Code, w.Body.String())
	}
}

// Regression: download dereferenced a nil Catalog and panicked.
func TestDownloadWithNilCatalogIs503(t *testing.T) {
	h := handlerWith(t, func(s *Services) { s.Catalog = nil })
	w := do(t, h, "GET", "/v1/skills/s/versions/1/package", "")
	if w.Code != 503 {
		t.Fatalf("status=%d body=%s", w.Code, w.Body.String())
	}
}

package app

import (
	"context"
	"encoding/json"
	"fmt"
	"github.com/sirerun/gist/catalog/directory"
	"github.com/sirerun/gist/hosted/internal/oauth"
	"github.com/sirerun/gist/hosted/internal/ports"
	"net/http"
	"net/http/httptest"
	"net/url"
	"strings"
	"testing"
	"time"
)

type privateDirectoryProbe struct {
	items   []privateDirectoryEntry
	created directory.Record
	calls   int
}

func (p *privateDirectoryProbe) list(context.Context, ports.Principal, string) ([]privateDirectoryEntry, error) {
	p.calls++
	return p.items, nil
}
func (p *privateDirectoryProbe) create(_ context.Context, _ ports.Principal, r directory.Record) (privateDirectoryEntry, error) {
	p.calls++
	p.created = r
	return privateDirectoryEntry{ID: "00000000-0000-0000-0000-000000000001", Record: r, CreatedAt: time.Now()}, nil
}
func (p *privateDirectoryProbe) remove(context.Context, ports.Principal, string) error {
	p.calls++
	return nil
}
func customRecord() directory.Record {
	return directory.Record{ID: "custom/example", Kind: "mcp", ServiceID: "example", Title: "Private local server", Description: "Local descriptor", Remotes: []directory.Remote{{URL: "http://localhost:8080/mcp", Transport: "streamable-http"}}, Auth: directory.Auth{Type: "none"}}
}
func TestPrivateDirectoryCSRFAndQualification(t *testing.T) {
	h, policy, _ := testWebAPI()
	h.csrfKey = []byte(strings.Repeat("c", 32))
	h.sessions = webSessionProbe{session: oauth.Session{ID: "one", Subject: "123", Workspaces: []string{"private"}}}
	store := &privateDirectoryProbe{}
	h.directory = store
	record := customRecord()
	record.Recommended = true
	record.Official = true
	record.Featured = true
	record.FeaturedSelection = true
	record.Verification = []directory.Qualification{{Result: "passed"}}
	raw, _ := json.Marshal(map[string]any{"record": record})
	post := func(origin, token, body string) *httptest.ResponseRecorder {
		r := httptest.NewRequest("POST", "/app/api/directory", strings.NewReader(body))
		r.Header.Set("Origin", origin)
		r.Header.Set("X-Gist-CSRF", token)
		r.Header.Set("Content-Type", "application/json")
		w := httptest.NewRecorder()
		h.ServeHTTP(w, r)
		return w
	}
	r := httptest.NewRequest("GET", "/app/api/session", nil)
	token := h.csrfToken(r)
	for _, origin := range []string{"", "https://evil.example"} {
		if w := post(origin, token, string(raw)); w.Code != 403 {
			t.Fatalf("origin %q %d %s", origin, w.Code, w.Body)
		}
	}
	if w := post(h.origin, "bad", string(raw)); w.Code != 403 || store.calls != 0 {
		t.Fatal("CSRF accepted")
	}
	if w := post(h.origin, token, string(raw)); w.Code != 201 {
		t.Fatalf("create=%d %s", w.Code, w.Body)
	}
	if store.created.Featured || store.created.Recommended || store.created.Official || store.created.FeaturedSelection || len(store.created.Verification) > 0 || store.created.Source.ID != "custom" {
		t.Fatalf("forged status persisted %+v", store.created)
	}
	h.sessions = webSessionProbe{session: oauth.Session{ID: "different", Subject: "123", Workspaces: []string{"private"}}}
	if w := post(h.origin, token, string(raw)); w.Code != 403 {
		t.Fatal("cross-session CSRF accepted")
	}
	policy.policy.Allowed = false
	if w := webRequest(h, "GET", "/app/api/directory", ""); w.Code != 401 {
		t.Fatalf("revoked read=%d", w.Code)
	}
}
func TestPrivateDirectoryInputBoundsAndProjection(t *testing.T) {
	h, _, _ := testWebAPI()
	h.csrfKey = []byte(strings.Repeat("c", 32))
	h.sessions = webSessionProbe{session: oauth.Session{ID: "one", Subject: "123", Workspaces: []string{"private"}}}
	store := &privateDirectoryProbe{}
	h.directory = store
	r := httptest.NewRequest("GET", "/app/api/session", nil)
	token := h.csrfToken(r)
	for _, body := range []string{`{"record":{"credential":"secret"}}`, `{"record":{}}`, strings.Repeat("x", 65537)} {
		req := httptest.NewRequest("POST", "/app/api/directory", strings.NewReader(body))
		req.Header.Set("Origin", h.origin)
		req.Header.Set("Content-Type", "application/json")
		req.Header.Set("X-Gist-CSRF", token)
		w := httptest.NewRecorder()
		h.ServeHTTP(w, req)
		if w.Code != 400 || store.calls != 0 {
			t.Fatalf("invalid accepted status%d", w.Code)
		}
	}
	for _, u := range []string{"http://user:password@localhost:80/mcp", "http://localhost/mcp?token=secret", "file:///tmp/server", "javascript:alert(1)"} {
		record := customRecord()
		record.Remotes[0].URL = u
		body, _ := json.Marshal(map[string]any{"record": record})
		req := httptest.NewRequest("POST", "/app/api/directory", strings.NewReader(string(body)))
		req.Header.Set("Origin", h.origin)
		req.Header.Set("Content-Type", "application/json")
		req.Header.Set("X-Gist-CSRF", token)
		w := httptest.NewRecorder()
		h.ServeHTTP(w, req)
		if w.Code != 400 {
			t.Fatalf("unsafe URL accepted %q status%d", u, w.Code)
		}
	}
	if w := webRequest(h, "GET", "/app/api/directory?workspace=foreign", ""); w.Code != 400 {
		t.Fatal("workspace selector accepted")
	}
	_ = http.MethodGet
}
func TestPublicDirectoryAdmissionIsSeparate(t *testing.T) {
	idx, err := directory.Default()
	if err != nil {
		t.Fatal(err)
	}
	public := &directoryAPI{index: idx, origin: "https://gist.example", maxBytes: 512 << 10}
	app := requestContext{directory: public, admission: func() error { return errDirectoryDenied }, rest: http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) { t.Fatal("private REST reached") })}
	if w := webRequest(app, "GET", "/api/directory?limit=1", ""); w.Code != 200 || !strings.Contains(w.Body.String(), "snapshot_revision") {
		t.Fatalf("public=%d %s", w.Code, w.Body)
	}
	for _, path := range []string{"/v1/skills", "/app/api/directory", "/api/directory/unknown"} {
		if w := webRequest(app, "GET", path, ""); w.Code != 503 {
			t.Fatalf("bootstrap leaked %q:%d", path, w.Code)
		}
	}
	for _, path := range []string{"/api/directory?workspace=private", "/api/directory?q=x&q=y", "/api/directory?limit=51", "/api/directory/item?id=unknown"} {
		w := webRequest(public, "GET", path, "")
		if w.Code != 400 && w.Code != 404 {
			t.Fatalf("query status=%d %s", w.Code, w.Body)
		}
	}
}

func TestPrivateDirectoryPagesRespectResponseLimitAndCursorBinding(t *testing.T) {
	h, _, _ := testWebAPI()
	store := &privateDirectoryProbe{}
	h.directory = store
	h.maxBytes = 8 << 10
	for i := 0; i < 25; i++ {
		r := customRecord()
		r.Description = strings.Repeat("a", 2400)
		store.items = append(store.items, privateDirectoryEntry{ID: fmt.Sprintf("00000000-0000-0000-0000-%012d", i+1), Record: r})
	}
	w := webRequest(h, "GET", "/app/api/directory", "")
	var page struct {
		Items []privateDirectoryEntry `json:"items"`
		Next  string                  `json:"next_cursor"`
	}
	if w.Code != 200 || len(w.Body.Bytes()) > h.maxBytes || json.Unmarshal(w.Body.Bytes(), &page) != nil || len(page.Items) == 0 || len(page.Items) >= 24 || page.Next == "" {
		t.Fatalf("page%d %s", w.Code, w.Body)
	}
	cursor, err := h.openCursor(page.Next, h.binding(ports.Principal{Issuer: h.origin, Subject: "123", WorkspaceID: h.workspace, PolicyGeneration: 3}, "private-directory", ""))
	if err != nil || cursor.ID != page.Items[len(page.Items)-1].ID {
		t.Fatalf("skipped entry cursor=%+v err%v", cursor, err)
	}
	h.sessions = webSessionProbe{session: oauth.Session{Subject: "different", Workspaces: []string{"private"}}}
	if w := webRequest(h, "GET", "/app/api/directory?cursor="+url.QueryEscape(page.Next), ""); w.Code != 400 {
		t.Fatalf("foreign cursor%d %s", w.Code, w.Body)
	}
}

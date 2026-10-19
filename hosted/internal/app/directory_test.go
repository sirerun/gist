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

func TestPublicDirectoryResponsesHideInternalEvidence(t *testing.T) {
	now := time.Now().UTC()
	record := directory.Record{
		ID: "fixture/example", Kind: "mcp", ServiceID: "example", Title: "Example Files",
		Description: "Remote MCP endpoint listed by Make as a verified server. The operator is not verified by Gist. This reachability check is not an authenticated session or workflow qualification.",
		Tags:        []string{"files"}, Source: directory.RecordSource{ID: "fixture", EntryID: "private-source", URL: "https://example.com/source"},
		Remotes: []directory.Remote{{URL: "https://example.com/mcp", Transport: "streamable-http", Check: &directory.EndpointCheck{CheckedAt: now, HTTPStatus: 401, Result: "authentication-required"}}},
		Auth:    directory.Auth{Type: "none"}, Official: true, OfficialEvidenceURL: "https://example.com/vendor",
		Verification: []directory.Qualification{{Client: "Zati", Workflow: "Read files", EvidenceURL: "https://example.com/private-evidence", CheckedAt: now, ExpiresAt: now.Add(time.Hour), Result: "passed"}},
	}
	idx, err := directory.New(directory.Snapshot{
		Format: directory.SnapshotFormat, Revision: "test", GeneratedAt: now,
		Sources: []directory.Source{{ID: "fixture", URL: "https://example.com/source", Revision: "1", SHA256: strings.Repeat("a", 64), RetrievedAt: now, Attribution: "Fixture"}},
		Entries: []directory.Record{record},
	})
	if err != nil {
		t.Fatal(err)
	}
	h := &directoryAPI{index: idx, origin: "https://gist.example"}
	for _, path := range []string{"/api/directory?q=Example", "/api/directory/item?id=fixture%2Fexample", "/api/directory/export?id=fixture%2Fexample"} {
		w := httptest.NewRecorder()
		h.ServeHTTP(w, httptest.NewRequest("GET", path, nil))
		if w.Code != 200 {
			t.Fatalf("%s: %d %s", path, w.Code, w.Body)
		}
		body := strings.ToLower(w.Body.String())
		for _, internal := range []string{"make", "private-source", "authentication-required", "checked_at", "verification", "private-evidence", "official_evidence_url", "featured"} {
			if strings.Contains(body, internal) {
				t.Fatalf("%s exposed %q: %s", path, internal, body)
			}
		}
		for _, useful := range []string{"example files", "https://example.com/mcp", "streamable-http"} {
			if !strings.Contains(body, useful) {
				t.Fatalf("%s omitted %q: %s", path, useful, body)
			}
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

func TestPublicAppDirectoryRoutes(t *testing.T) {
	apps, err := directory.ParseApps([]byte(`{"format":"gist.app-directory/v1","items":[{"slug":"example","name":"Example","description":"Organize work","categories":[],"icon_path":"example.svg","background_color":"#123456"}]}`), nil)
	if err != nil {
		t.Fatal(err)
	}
	h := &directoryAPI{apps: apps, origin: "https://gist.example"}
	for _, route := range []string{"/api/directory/apps?limit=1", "/api/directory/app?slug=example"} {
		w := httptest.NewRecorder()
		h.ServeHTTP(w, httptest.NewRequest("GET", route, nil))
		if w.Code != 200 || !strings.Contains(w.Body.String(), `"name":"Example"`) || strings.Contains(w.Body.String(), "connector_ids") {
			t.Fatalf("%s: %d %s", route, w.Code, w.Body)
		}
	}
	for _, route := range []string{"/api/directory/apps?limit=101", "/api/directory/apps?offset=-1", "/api/directory/apps?unknown=x", "/api/directory/app"} {
		w := httptest.NewRecorder()
		h.ServeHTTP(w, httptest.NewRequest("GET", route, nil))
		if w.Code != 400 {
			t.Fatalf("%s: %d", route, w.Code)
		}
	}
	for _, route := range []string{"/api/directory/apps", "/api/directory/app"} {
		if !publicDirectoryPath(route) {
			t.Fatalf("private route %s", route)
		}
	}
}

func TestPublicAppLiteralSlugRoundtrip(t *testing.T) {
	apps, err := directory.ParseApps([]byte(`{"format":"gist.app-directory/v1","items":[{"slug":"h&m-home","name":"H&M Home"},{"slug":"app#world-cup-2022-rkrqsb","name":"World Cup"}]}`), nil)
	if err != nil {
		t.Fatal(err)
	}
	h := &directoryAPI{apps: apps}
	for _, slug := range []string{"h&m-home", "app#world-cup-2022-rkrqsb"} {
		w := httptest.NewRecorder()
		h.ServeHTTP(w, httptest.NewRequest("GET", "/api/directory/app?"+url.Values{"slug": {slug}}.Encode(), nil))
		if w.Code != 200 {
			t.Fatalf("slug %q status %d body %s", slug, w.Code, w.Body)
		}
		var result directory.App
		if err := json.Unmarshal(w.Body.Bytes(), &result); err != nil || result.Slug != slug {
			t.Fatalf("slug %q result %+v %v", slug, result, err)
		}
	}
}

func TestPublicHubSpotAppHasAdmittedRoute(t *testing.T) {
	connectors, err := directory.Default()
	if err != nil {
		t.Fatal(err)
	}
	apps, err := directory.DefaultApps(connectors)
	if err != nil {
		t.Fatal(err)
	}
	h := &directoryAPI{apps: apps}
	w := httptest.NewRecorder()
	h.ServeHTTP(w, httptest.NewRequest("GET", "/api/directory/app?slug=hubspotcrm", nil))
	if w.Code != 200 {
		t.Fatalf("HubSpot status %d %s", w.Code, w.Body)
	}
	var app directory.App
	if err := json.Unmarshal(w.Body.Bytes(), &app); err != nil || len(app.ConnectorIDs) == 0 {
		t.Fatalf("HubSpot route %+v %v", app, err)
	}
}

func TestPublicProviderAuthorizationRoute(t *testing.T) {
	idx, err := directory.DefaultPublic()
	if err != nil {
		t.Fatal(err)
	}
	public := &directoryAPI{index: idx, origin: "https://gist.example"}
	app := requestContext{directory: public, admission: func() error { return errDirectoryDenied }, rest: http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) { t.Fatal("private REST reached") })}
	id := "mcp:app.linear/linear"
	response := webRequest(app, "GET", "/api/directory/authorization?id="+url.QueryEscape(id), "")
	if response.Code != 200 {
		t.Fatalf("%d %s", response.Code, response.Body)
	}
	var guide directory.AuthorizationGuide
	if err := json.Unmarshal(response.Body.Bytes(), &guide); err != nil {
		t.Fatal(err)
	}
	if guide.Format != directory.AuthorizationFormat || guide.ID != id || guide.ConnectionTested || guide.CredentialOwner != "client" {
		t.Fatalf("invalid guide %+v", guide)
	}
	item := webRequest(public, "GET", "/api/directory/item?id="+url.QueryEscape(id), "")
	if !strings.Contains(item.Header().Get("Link"), "/api/directory/authorization?id=") {
		t.Fatal("missing authorization discovery link")
	}
	var record directory.Record
	decoder := json.NewDecoder(strings.NewReader(item.Body.String()))
	decoder.DisallowUnknownFields()
	if err := decoder.Decode(&record); err != nil {
		t.Fatalf("descriptor compatibility: %v", err)
	}
	for _, path := range []string{"/api/directory/authorization", "/api/directory/authorization?id=missing", "/api/directory/authorization?id=" + url.QueryEscape(id) + "&token=secret"} {
		w := webRequest(public, "GET", path, "")
		if w.Code != 400 && w.Code != 404 {
			t.Fatalf("bad query %q accepted: %d", path, w.Code)
		}
	}
}

func TestPublicAppAuthorizationBySlug(t *testing.T) {
	idx, err := directory.DefaultPublic()
	if err != nil {
		t.Fatal(err)
	}
	apps, err := directory.DefaultApps(idx)
	if err != nil {
		t.Fatal(err)
	}
	h := &directoryAPI{index: idx, apps: apps, origin: "https://gist.example"}
	w := webRequest(h, "GET", "/api/directory/authorization?app=google-sheets", "")
	if w.Code != 200 {
		t.Fatalf("%d %s", w.Code, w.Body)
	}
	var g directory.AppAuthorization
	if err := json.Unmarshal(w.Body.Bytes(), &g); err != nil {
		t.Fatal(err)
	}
	if g.App != "google-sheets" || g.Format != "gist.directory.app-authorization/v1" || g.Status != "documented" || len(g.Routes) != 1 || g.Routes[0].Kind != "http-api" {
		t.Fatal(w.Body)
	}
	for _, u := range []string{"/api/directory/authorization?app=missing", "/api/directory/authorization?app=google-sheets&id=mcp%3Aairtable", "/api/directory/authorization?app=google-sheets&app=slack"} {
		w := webRequest(h, "GET", u, "")
		if w.Code != 400 && w.Code != 404 {
			t.Fatalf("invalid app selector %s %d", u, w.Code)
		}
	}
}

func TestAppAuthorizationVersionNegotiation(t *testing.T) {
	idx, err := directory.DefaultPublic()
	if err != nil {
		t.Fatal(err)
	}
	apps, err := directory.DefaultApps(idx)
	if err != nil {
		t.Fatal(err)
	}
	h := &directoryAPI{index: idx, apps: apps, origin: "https://gist.example"}
	w := webRequest(h, "GET", "/api/directory/authorization?app=shopify&version=2", "")
	var g directory.AppAuthorization
	if w.Code != 200 || json.Unmarshal(w.Body.Bytes(), &g) != nil || g.Format != "gist.directory.app-authorization/v2" || len(g.Routes) == 0 || g.Routes[0].APIURLTemplate == "" {
		t.Fatalf("v2: %d %s", w.Code, w.Body)
	}
	w = webRequest(h, "GET", "/api/directory/authorization?app=shopify", "")
	if w.Code != 200 || strings.Contains(w.Body.String(), `"api_url_template"`) {
		t.Fatalf("v1: %d %s", w.Code, w.Body)
	}
	for _, version := range []string{"", "0", "3", "-1", "garbage"} {
		w := webRequest(h, "GET", "/api/directory/authorization?app=shopify&version="+version, "")
		if w.Code != 400 {
			t.Fatalf("invalid version %q: %d", version, w.Code)
		}
	}
}

package app

import (
	"context"
	"encoding/json"
	"errors"
	"net/http"
	"net/http/httptest"
	"net/url"
	"strings"
	"testing"
	"time"

	"github.com/jackc/pgx/v5"
	"github.com/sirerun/gist/hosted/internal/identity"
	"github.com/sirerun/gist/hosted/internal/oauth"
	"github.com/sirerun/gist/hosted/internal/ports"
)

type webSessionProbe struct {
	session oauth.Session
	err     error
}

func (s webSessionProbe) Authenticate(*http.Request) (oauth.Session, error) { return s.session, s.err }

type webPolicyProbe struct {
	policy                     identity.Policy
	err                        error
	deny                       bool
	calls                      int
	issuer, subject, workspace string
}

func (p *webPolicyProbe) CheckWorkloadForIssuer(_ context.Context, issuer, subject, workspace string, scopes []string, generation uint64) (identity.Policy, error) {
	p.calls++
	p.issuer, p.subject, p.workspace = issuer, subject, workspace
	return p.policy, p.err
}
func (p *webPolicyProbe) Decide(context.Context, ports.Principal, ports.Action, *ports.ArtifactRef) (ports.Decision, error) {
	return ports.Decision{Allowed: !p.deny}, nil
}

type webCatalogProbe struct {
	items     []webItem
	after     webPosition
	principal ports.Principal
	calls     int
}

func (c *webCatalogProbe) list(_ context.Context, p ports.Principal, q, kind string, after webPosition) ([]webItem, error) {
	c.calls++
	c.principal, c.after = p, after
	return c.items, nil
}
func (c *webCatalogProbe) get(_ context.Context, p ports.Principal, kind, id, version string) (webItem, error) {
	c.calls++
	c.principal = p
	if len(c.items) == 0 {
		return webItem{}, pgx.ErrNoRows
	}
	return c.items[0], nil
}
func testWebAPI() (*webAPI, *webPolicyProbe, *webCatalogProbe) {
	p := &webPolicyProbe{policy: identity.Policy{Allowed: true, PolicyGeneration: 3}}
	c := &webCatalogProbe{}
	h := &webAPI{sessions: webSessionProbe{session: oauth.Session{Subject: "123", Workspaces: []string{"private"}}}, policy: p, catalog: c, origin: "https://gist.example", audience: "https://gist.example", workspace: "private", workspaceName: "Private library", cursorKey: []byte(strings.Repeat("k", 32)), timeout: time.Second, maxBytes: 64 << 10}
	return h, p, c
}
func webRequest(h http.Handler, method, target, origin string) *httptest.ResponseRecorder {
	r := httptest.NewRequest(method, target, nil)
	if origin != "" {
		r.Header.Set("Origin", origin)
	}
	w := httptest.NewRecorder()
	h.ServeHTTP(w, r)
	return w
}
func TestWebSessionsRequireCurrentDurableAuthority(t *testing.T) {
	for _, reason := range []string{"absent", "foreign workspace", "revoked", "stale policy", "missing membership", "policy unavailable", "decision denied"} {
		t.Run(reason, func(t *testing.T) {
			h, p, c := testWebAPI()
			want := 401
			switch reason {
			case "absent":
				h.sessions = webSessionProbe{err: oauth.ErrNoSession}
			case "foreign workspace":
				h.sessions = webSessionProbe{session: oauth.Session{Subject: "123", Workspaces: []string{"foreign"}}}
			case "revoked", "stale policy":
				p.policy.Allowed = false
			case "missing membership":
				p.err = pgx.ErrNoRows
			case "policy unavailable":
				p.err = errors.New("offline")
				want = 503
			case "decision denied":
				p.deny = true
			}
			w := webRequest(h, "GET", "/app/api/catalog", "")
			if w.Code != want || c.calls != 0 || w.Header().Get("Cache-Control") != "no-store" {
				t.Fatalf("status=%d calls=%d headers=%v", w.Code, c.calls, w.Header())
			}
		})
	}
	h, p, c := testWebAPI()
	w := webRequest(h, "GET", "/app/api/session", "")
	if w.Code != 200 || p.issuer != h.origin || p.workspace != "private" || c.calls != 0 {
		t.Fatalf("authority status=%d policy=%+v", w.Code, p)
	}
	p.policy.Allowed = false
	if w = webRequest(h, "GET", "/app/api/session", ""); w.Code != 401 || p.calls != 2 {
		t.Fatal("session cached old membership")
	}
}
func TestWebCatalogValidatesQueriesAndPreservesTenant(t *testing.T) {
	h, _, c := testWebAPI()
	for _, target := range []string{"/app/api/catalog?workspace=foreign", "/app/api/catalog?kind=connection", "/app/api/catalog?q=a&q=b", "/app/api/catalog?q=%zz", "/app/api/catalog?cursor=bad", "/app/api/catalog?q=" + strings.Repeat("a", 201), "/app/api/artifact?kind=skill&id=x", "/app/api/artifact?kind=skill&id=x&version=1&workspace=foreign"} {
		if w := webRequest(h, "GET", target, ""); w.Code != 400 {
			t.Fatalf("%s status=%d body=%s", target, w.Code, w.Body)
		}
	}
	if c.calls != 0 {
		t.Fatal("invalid input reached catalog")
	}
	w := webRequest(h, "GET", "/app/api/catalog?q=mail&kind=tool", "")
	if w.Code != 200 || c.principal.WorkspaceID != "private" || c.principal.Subject != "123" || !strings.Contains(w.Body.String(), `"items":[]`) {
		t.Fatalf("status=%d principal=%+v body=%s", w.Code, c.principal, w.Body)
	}
	if w = webRequest(h, "GET", "/app/api/artifact?kind=tool&id=missing&version=1", ""); w.Code != 404 {
		t.Fatal("missing artifact not 404")
	}
}
func TestWebCursorIsSignedAndBoundToPrincipalQueryAndPolicy(t *testing.T) {
	h, _, c := testWebAPI()
	for i := 0; i < 25; i++ {
		c.items = append(c.items, webItem{Kind: "skill", ID: strings.Repeat("x", i+1), Version: "1"})
	}
	w := webRequest(h, "GET", "/app/api/catalog?q=mail", "")
	var page struct {
		Items []webItem `json:"items"`
		Next  string    `json:"next_cursor"`
	}
	if w.Code != 200 || json.Unmarshal(w.Body.Bytes(), &page) != nil || len(page.Items) != 24 || page.Next == "" {
		t.Fatalf("page %d %s", w.Code, w.Body)
	}
	target := "/app/api/catalog?q=mail&cursor=" + url.QueryEscape(page.Next)
	if w = webRequest(h, "GET", target, ""); w.Code != 200 || c.after.ID != strings.Repeat("x", 24) {
		t.Fatalf("next page %d after=%+v", w.Code, c.after)
	}
	for _, bad := range []string{strings.Replace(target, "q=mail", "q=other", 1), target + "x"} {
		if w = webRequest(h, "GET", bad, ""); w.Code != 400 {
			t.Fatal("cursor not query-bound and authenticated")
		}
	}
	h.sessions = webSessionProbe{session: oauth.Session{Subject: "456", Workspaces: []string{"private"}}}
	if w = webRequest(h, "GET", target, ""); w.Code != 400 {
		t.Fatal("foreign principal cursor accepted")
	}
}
func TestWebLogoutRequiresCanonicalOriginAndClearsHostCookie(t *testing.T) {
	h, _, _ := testWebAPI()
	h.sessions = webSessionProbe{err: oauth.ErrNoSession}
	for _, origin := range []string{"", "https://other.example", "null"} {
		if w := webRequest(h, "POST", "/app/api/logout", origin); w.Code != 403 {
			t.Fatal("logout accepted foreign origin")
		}
	}
	w := webRequest(h, "POST", "/app/api/logout", h.origin)
	cookies := w.Result().Cookies()
	if w.Code != 204 || len(cookies) != 1 || cookies[0].Name != oauth.SessionCookieName || cookies[0].MaxAge != -1 || !cookies[0].Secure || !cookies[0].HttpOnly || cookies[0].Path != "/" || cookies[0].Domain != "" {
		t.Fatalf("logout status=%d cookies=%+v", w.Code, cookies)
	}
	if w = webRequest(h, "GET", "/app/api/logout", h.origin); w.Code != 405 {
		t.Fatal("logout allowed GET")
	}
}
func TestWebReadDeniesCrossOriginAndLimitsResponse(t *testing.T) {
	h, _, c := testWebAPI()
	if w := webRequest(h, "GET", "/app/api/catalog", "https://other.example"); w.Code != 403 || c.calls != 0 {
		t.Fatal("foreign origin read allowed")
	}
	r := httptest.NewRequest("GET", "/app/api/catalog", nil)
	r.Header.Set("Sec-Fetch-Site", "cross-site")
	w := httptest.NewRecorder()
	h.ServeHTTP(w, r)
	if w.Code != 403 {
		t.Fatal("foreign fetch allowed")
	}
	h.maxBytes = 10
	if w = webRequest(h, "GET", "/app/api/session", ""); w.Code != 503 {
		t.Fatal("oversized response allowed")
	}
	if w = webRequest(h, "POST", "/app/api/catalog", ""); w.Code != 405 {
		t.Fatal("catalog allowed POST")
	}
}
func TestWebPublicShellDoesNotBypassBootstrapAPIs(t *testing.T) {
	reached := 0
	fake := http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) { reached++; w.WriteHeader(204) })
	h := requestContext{admission: func() error { return errors.New("waiting for founder") }, ui: fake, web: fake, rest: fake}
	for _, path := range []string{"/", "/library", "/library/item", "/developers", "/docs", "/signin", "/assets/app.js", "/favicon.ico"} {
		if w := webRequest(h, "GET", path, ""); w.Code != 204 {
			t.Fatalf("public %s status=%d", path, w.Code)
		}
	}
	before := reached
	for _, path := range []string{"/app/api/session", "/app/api/catalog", "/app/api/artifact", "/v1/skills", "/not-a-page", "/library/secret"} {
		if w := webRequest(h, "GET", path, ""); w.Code != 503 {
			t.Fatalf("private %s status=%d", path, w.Code)
		}
	}
	if reached != before {
		t.Fatal("bootstrap bypass exposed API")
	}
	if w := webRequest(h, "POST", "/app/api/logout", ""); w.Code != 204 {
		t.Fatal("bootstrap blocked safe logout")
	}
}

func TestWebDetailProjectsOnlyBoundedDeclaredContractFields(t *testing.T) {
	raw := []byte(`{"publisher":{"id":"sirerun","source_revision":"abc123","license":"MIT","secret":"hidden"},"publication":{"status":"approved","provenance":"publisher_asserted"},"trust":"operator_asserted","tags":["mail"],"required_capabilities":[{"id":"mail/inbox/read","contract_version":"1.0.0","optional":false,"credentials":"hidden"}],"runtime_requirements":{"local_execution":false,"filesystem":["private/host/path"]},"effects":["mail.read"],"required_scopes":["mail.read"],"execution_location":"client","support_state":"catalog_only","last_verified_at":"2026-10-17T00:00:00Z","object_key":"private-storage","credential":"hidden"}`)
	out := map[string]any{}
	projectWebDeclarations(out, raw)
	encoded, err := json.Marshal(out)
	if err != nil {
		t.Fatal(err)
	}
	for _, secret := range []string{"hidden", "private/host/path", "private-storage", "object_key", "credentials"} {
		if strings.Contains(string(encoded), secret) {
			t.Fatalf("projection exposed %s: %s", secret, encoded)
		}
	}
	for _, field := range []string{"publisher", "source_revision", "license", "required_capabilities", "contract_version", "runtime_requirements", "filesystem_required", "effects", "required_scopes", "support_state"} {
		if !strings.Contains(string(encoded), field) {
			t.Fatalf("projection omitted %s", field)
		}
	}
	if !strings.Contains(string(encoded), `"filesystem_required":true`) || !strings.Contains(string(encoded), `"local_execution":false`) {
		t.Fatalf("runtime declarations=%s", encoded)
	}
	out = map[string]any{}
	projectWebDeclarations(out, []byte(strings.Repeat("x", 65537)))
	if out["declarations_unavailable"] != true {
		t.Fatal("oversized metadata parsed")
	}
	out = map[string]any{}
	projectWebDeclarations(out, []byte(`{"tags":["`+strings.Repeat("x", 300)+`"]}`))
	if out["declarations_abbreviated"] != true {
		t.Fatal("truncation not disclosed")
	}
}
func TestWebConcurrentReadsAreBounded(t *testing.T) {
	h, p, c := testWebAPI()
	h.concurrent = make(chan struct{}, 1)
	h.concurrent <- struct{}{}
	w := webRequest(h, "GET", "/app/api/catalog", "")
	if w.Code != 429 || w.Header().Get("Retry-After") != "1" || p.calls != 0 || c.calls != 0 {
		t.Fatalf("busy status=%d policy=%d catalog=%d", w.Code, p.calls, c.calls)
	}
	if w = webRequest(h, "POST", "/app/api/logout", h.origin); w.Code != 204 {
		t.Fatal("busy reads blocked logout")
	}
}

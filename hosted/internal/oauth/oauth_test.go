package oauth_test

import (
	"bytes"
	"context"
	"crypto/rand"
	"crypto/sha256"
	"encoding/base64"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"net/http"
	"net/http/httptest"
	"net/url"
	"regexp"
	"strings"
	"sync"
	"testing"
	"time"

	"github.com/sirerun/gist/hosted/internal/identity"
	"github.com/sirerun/gist/hosted/internal/oauth"
	"github.com/sirerun/gist/hosted/internal/ports"
)

const resourceB = "https://resource-b.example"

// ---- fixtures -------------------------------------------------------------

type fakeClock struct {
	mu  sync.Mutex
	now time.Time
}

func (c *fakeClock) Now() time.Time { c.mu.Lock(); defer c.mu.Unlock(); return c.now }
func (c *fakeClock) Advance(d time.Duration) {
	c.mu.Lock()
	defer c.mu.Unlock()
	c.now = c.now.Add(d)
}

type membership struct {
	scopes     []string
	generation uint64
	active     bool
}

// memPolicy mirrors the Postgres workspace-membership oracle: a missing
// membership is an error, never an approval.
type memPolicy struct {
	mu sync.Mutex
	m  map[string]*membership
}

func (p *memPolicy) set(subject, ws string, scopes []string, active bool) {
	p.mu.Lock()
	defer p.mu.Unlock()
	gen := uint64(1)
	if old := p.m[subject+"|"+ws]; old != nil {
		gen = old.generation
	}
	p.m[subject+"|"+ws] = &membership{scopes: scopes, generation: gen, active: active}
}

func (p *memPolicy) CheckWorkload(_ context.Context, subject, ws string, scopes []string, generation uint64) (identity.Policy, error) {
	p.mu.Lock()
	defer p.mu.Unlock()
	m := p.m[subject+"|"+ws]
	if m == nil {
		return identity.Policy{}, errors.New("no membership")
	}
	allowed := m.active && containsAll(m.scopes, scopes)
	if generation != 0 && generation != m.generation {
		allowed = false
	}
	return identity.Policy{Allowed: allowed, PolicyGeneration: m.generation, ParentSubject: subject, ParentScopes: append([]string(nil), m.scopes...)}, nil
}

func containsAll(have, want []string) bool {
	set := map[string]bool{}
	for _, s := range have {
		set[s] = true
	}
	for _, s := range want {
		if !set[s] {
			return false
		}
	}
	return true
}

type memIdentities struct {
	mu      sync.Mutex
	records map[string]ports.IdentityRecord
}

func (m *memIdentities) record(_ context.Context, r ports.IdentityRecord) error {
	m.mu.Lock()
	defer m.mu.Unlock()
	m.records[r.WorkspaceID+"|"+r.Issuer+"|"+r.Subject] = r
	return nil
}

func (m *memIdentities) get(ws, iss, sub string) (ports.IdentityRecord, bool) {
	m.mu.Lock()
	defer m.mu.Unlock()
	r, ok := m.records[ws+"|"+iss+"|"+sub]
	return r, ok
}

type storedCode struct {
	code     oauth.AuthCode
	consumed bool
}

type storedFamily struct {
	family  oauth.RefreshFamily
	revoked bool
}

type storedRefresh struct {
	token    oauth.RefreshToken
	familyID string
	consumed bool
}

// memStore implements oauth.Store with the same semantics as the Postgres
// store: workspace-scoped lookups, check-before-consume, and replay
// revocation. Keys are hex hashes; no raw credential is ever handed to it.
type memStore struct {
	mu       sync.Mutex
	clients  map[string]oauth.Client
	codes    map[string]*storedCode
	families map[string]*storedFamily
	tokens   map[string]*storedRefresh
	seq      int
}

func newMemStore() *memStore {
	return &memStore{clients: map[string]oauth.Client{}, codes: map[string]*storedCode{}, families: map[string]*storedFamily{}, tokens: map[string]*storedRefresh{}}
}

func hkey(b []byte) string { return fmt.Sprintf("%x", b) }

func (s *memStore) CreateClient(_ context.Context, c oauth.Client) error {
	s.mu.Lock()
	defer s.mu.Unlock()
	s.clients[c.ID] = c
	return nil
}

func (s *memStore) GetClient(_ context.Context, id string) (oauth.Client, error) {
	s.mu.Lock()
	defer s.mu.Unlock()
	c, ok := s.clients[id]
	if !ok {
		return oauth.Client{}, oauth.ErrNotFound
	}
	return c, nil
}

func (s *memStore) SaveCode(_ context.Context, c oauth.AuthCode) error {
	s.mu.Lock()
	defer s.mu.Unlock()
	s.codes[hkey(c.Hash)] = &storedCode{code: c}
	return nil
}

func (s *memStore) RedeemCode(_ context.Context, ws string, hash []byte, check func(oauth.AuthCode) error) (oauth.AuthCode, error) {
	s.mu.Lock()
	defer s.mu.Unlock()
	c, ok := s.codes[hkey(hash)]
	if !ok || c.code.WorkspaceID != ws {
		return oauth.AuthCode{}, oauth.ErrNotFound
	}
	if c.consumed {
		for _, f := range s.families {
			if bytes.Equal(f.family.CodeHash, hash) {
				f.revoked = true
			}
		}
		return oauth.AuthCode{}, oauth.ErrGrantReused
	}
	if err := check(c.code); err != nil {
		return oauth.AuthCode{}, err
	}
	c.consumed = true
	return c.code, nil
}

func (s *memStore) CreateFamily(_ context.Context, f oauth.RefreshFamily, first oauth.RefreshToken) (string, error) {
	s.mu.Lock()
	defer s.mu.Unlock()
	s.seq++
	f.ID = fmt.Sprintf("fam-%d", s.seq)
	s.families[f.ID] = &storedFamily{family: f}
	s.tokens[hkey(first.Hash)] = &storedRefresh{token: first, familyID: f.ID}
	return f.ID, nil
}

func (s *memStore) RotateRefresh(_ context.Context, ws string, hash []byte, check func(oauth.RefreshFamily, oauth.RefreshToken) error, next oauth.RefreshToken) (oauth.RefreshFamily, error) {
	s.mu.Lock()
	defer s.mu.Unlock()
	t, ok := s.tokens[hkey(hash)]
	if !ok {
		return oauth.RefreshFamily{}, oauth.ErrNotFound
	}
	f := s.families[t.familyID]
	if f.family.WorkspaceID != ws {
		return oauth.RefreshFamily{}, oauth.ErrNotFound
	}
	if f.revoked {
		return oauth.RefreshFamily{}, oauth.ErrFamilyRevoked
	}
	if t.consumed {
		f.revoked = true
		return oauth.RefreshFamily{}, oauth.ErrGrantReused
	}
	if err := check(f.family, t.token); err != nil {
		return oauth.RefreshFamily{}, err
	}
	t.consumed = true
	s.tokens[hkey(next.Hash)] = &storedRefresh{token: next, familyID: f.family.ID}
	return f.family, nil
}

func (s *memStore) RevokeFamily(_ context.Context, ws, id string) error {
	s.mu.Lock()
	defer s.mu.Unlock()
	if f, ok := s.families[id]; ok && f.family.WorkspaceID == ws {
		f.revoked = true
		return nil
	}
	return oauth.ErrNotFound
}

func (s *memStore) RevokeRefresh(_ context.Context, ws string, hash []byte, clientID string) error {
	s.mu.Lock()
	defer s.mu.Unlock()
	t, ok := s.tokens[hkey(hash)]
	if !ok {
		return oauth.ErrNotFound
	}
	f := s.families[t.familyID]
	if f.family.WorkspaceID != ws || f.family.ClientID != clientID {
		return oauth.ErrNotFound
	}
	f.revoked = true
	return nil
}

func (s *memStore) codeCount() int { s.mu.Lock(); defer s.mu.Unlock(); return len(s.codes) }

func (s *memStore) dump() string {
	s.mu.Lock()
	defer s.mu.Unlock()
	var b strings.Builder
	for _, c := range s.codes {
		fmt.Fprintf(&b, "%+v %s\n", c.code, string(c.code.Hash))
	}
	for k, t := range s.tokens {
		fmt.Fprintf(&b, "%s %+v %s\n", k, t.token, string(t.token.Hash))
	}
	return b.String()
}

type harness struct {
	t          *testing.T
	srv        *httptest.Server
	origin     string
	as         *oauth.Server
	store      *memStore
	policy     *memPolicy
	identities *memIdentities
	sessions   *oauth.CookieSessions
	clock      *fakeClock
	http       *http.Client
}

func newHarness(t *testing.T) *harness {
	t.Helper()
	clock := &fakeClock{now: time.Date(2026, 9, 26, 12, 0, 0, 0, time.UTC)}
	key, err := identity.GenerateSigningKey("k1", clock.Now())
	if err != nil {
		t.Fatal(err)
	}
	keys, err := identity.NewKeySet(clock, key)
	if err != nil {
		t.Fatal(err)
	}
	h := &harness{t: t, store: newMemStore(), policy: &memPolicy{m: map[string]*membership{}}, identities: &memIdentities{records: map[string]ports.IdentityRecord{}}, clock: clock}
	h.srv = httptest.NewUnstartedServer(nil)
	h.origin = "https://" + h.srv.Listener.Addr().String()
	issuerA, err := identity.NewWorkloadIssuer(identity.Config{Issuer: h.origin, Audience: h.origin, Clock: clock, Keys: keys, Policy: h.policy})
	if err != nil {
		t.Fatal(err)
	}
	issuerB, err := identity.NewWorkloadIssuer(identity.Config{Issuer: h.origin, Audience: resourceB, Clock: clock, Keys: keys, Policy: h.policy})
	if err != nil {
		t.Fatal(err)
	}
	secret := make([]byte, 32)
	_, _ = rand.Read(secret)
	h.sessions, err = oauth.NewCookieSessions(secret, clock)
	if err != nil {
		t.Fatal(err)
	}
	h.as, err = oauth.New(oauth.Config{
		Issuer:          h.origin,
		Resources:       []string{h.origin, resourceB},
		ScopesSupported: []string{"catalog:read", "catalog:publish"},
		Store:           h.store,
		Minter:          oauth.IssuerMinter{Issuers: map[string]*identity.WorkloadIssuer{h.origin: issuerA, resourceB: issuerB}, Record: h.identities.record},
		Policy:          h.policy,
		Sessions:        h.sessions,
		Keys:            keys,
		Clock:           clock,
	})
	if err != nil {
		t.Fatal(err)
	}
	mux := http.NewServeMux()
	mux.Handle("/resource-a", oauth.WithChallenge(protected(issuerA), h.as.ProtectedResourceMetadataURL()))
	mux.Handle("/resource-b", oauth.WithChallenge(protected(issuerB), resourceB+oauth.PathPRMetadata))
	h.srv.Config.Handler = http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if oauth.Handles(r.URL.Path) {
			h.as.ServeHTTP(w, r)
			return
		}
		mux.ServeHTTP(w, r)
	})
	h.srv.StartTLS()
	t.Cleanup(h.srv.Close)
	h.http = h.srv.Client()
	h.http.CheckRedirect = func(*http.Request, []*http.Request) error { return http.ErrUseLastResponse }
	// alice: maintainer in ws-a and ws-b; reader-only in ws-r; nothing in ws-x.
	h.policy.set("alice", "ws-a", []string{"catalog:read", "catalog:publish"}, true)
	h.policy.set("alice", "ws-b", []string{"catalog:read", "catalog:publish"}, true)
	h.policy.set("alice", "ws-r", []string{"catalog:read"}, true)
	h.policy.set("alice", "ws-c", []string{"catalog:read", "catalog:publish"}, true)
	return h
}

// protected is a minimal resource server: it accepts only tokens whose
// audience is its own resource, through the identity verifier.
func protected(iss *identity.WorkloadIssuer) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		raw := strings.TrimPrefix(r.Header.Get("Authorization"), "Bearer ")
		_, auth, err := identity.VerifyContext(r.Context(), iss, raw)
		if err != nil {
			w.WriteHeader(http.StatusUnauthorized)
			return
		}
		w.Header().Set("Content-Type", "application/json")
		_ = json.NewEncoder(w).Encode(auth.Principal())
	})
}

func (h *harness) session(subject string, workspaces ...string) *http.Cookie {
	h.t.Helper()
	c, err := h.sessions.Issue(subject, workspaces, time.Hour)
	if err != nil {
		h.t.Fatal(err)
	}
	return c
}

func (h *harness) do(req *http.Request) (*http.Response, []byte) {
	h.t.Helper()
	resp, err := h.http.Do(req)
	if err != nil {
		h.t.Fatal(err)
	}
	defer resp.Body.Close()
	body, _ := io.ReadAll(resp.Body)
	return resp, body
}

func (h *harness) get(path string, cookie *http.Cookie) (*http.Response, []byte) {
	req, _ := http.NewRequest(http.MethodGet, h.origin+path, nil)
	if cookie != nil {
		req.AddCookie(cookie)
	}
	return h.do(req)
}

func (h *harness) postForm(path string, form url.Values, cookie *http.Cookie, headers map[string]string) (*http.Response, []byte) {
	req, _ := http.NewRequest(http.MethodPost, h.origin+path, strings.NewReader(form.Encode()))
	req.Header.Set("Content-Type", "application/x-www-form-urlencoded")
	for k, v := range headers {
		req.Header.Set(k, v)
	}
	if cookie != nil {
		req.AddCookie(cookie)
	}
	return h.do(req)
}

func (h *harness) register(body map[string]any) (int, map[string]any) {
	h.t.Helper()
	raw, _ := json.Marshal(body)
	req, _ := http.NewRequest(http.MethodPost, h.origin+oauth.PathRegister, bytes.NewReader(raw))
	req.Header.Set("Content-Type", "application/json")
	resp, out := h.do(req)
	var m map[string]any
	_ = json.Unmarshal(out, &m)
	return resp.StatusCode, m
}

func (h *harness) client(redirect string, scope string) string {
	h.t.Helper()
	body := map[string]any{"redirect_uris": []string{redirect, redirect + "/alt"}, "client_name": "Connector"}
	if scope != "" {
		body["scope"] = scope
	}
	status, m := h.register(body)
	if status != http.StatusCreated {
		h.t.Fatalf("register: %d %v", status, m)
	}
	return m["client_id"].(string)
}

func pkce() (string, string) {
	b := make([]byte, 32)
	_, _ = rand.Read(b)
	verifier := base64.RawURLEncoding.EncodeToString(b)
	sum := sha256.Sum256([]byte(verifier))
	return verifier, base64.RawURLEncoding.EncodeToString(sum[:])
}

type authReq struct {
	clientID, redirect, challenge, resource, scope, state, method string
	noScope                                                       bool
}

func (h *harness) authorize(a authReq, cookie *http.Cookie) (*http.Response, []byte) {
	q := url.Values{"response_type": {"code"}, "client_id": {a.clientID}, "redirect_uri": {a.redirect}, "code_challenge": {a.challenge}, "code_challenge_method": {"S256"}, "state": {a.state}, "resource": {a.resource}}
	if a.method != "" {
		q.Set("code_challenge_method", a.method)
	}
	if !a.noScope {
		q.Set("scope", a.scope)
	}
	return h.get(oauth.PathAuthorize+"?"+q.Encode(), cookie)
}

var (
	requestField = regexp.MustCompile(`name="request" value="([^"]+)"`)
	csrfField    = regexp.MustCompile(`name="csrf" value="([^"]+)"`)
)

func consentFields(t *testing.T, body []byte) (string, string) {
	t.Helper()
	r, c := requestField.FindSubmatch(body), csrfField.FindSubmatch(body)
	if r == nil || c == nil {
		t.Fatalf("consent page lacks request/csrf fields: %s", body)
	}
	return string(r[1]), string(c[1])
}

func (h *harness) consent(cookie *http.Cookie, request, csrf, workspace, decision string, headers map[string]string) (*http.Response, []byte) {
	return h.postForm(oauth.PathConsent, url.Values{"request": {request}, "csrf": {csrf}, "workspace": {workspace}, "decision": {decision}}, cookie, headers)
}

func location(t *testing.T, resp *http.Response) url.Values {
	t.Helper()
	if resp.StatusCode != http.StatusFound {
		t.Fatalf("status %d, want 302", resp.StatusCode)
	}
	u, err := url.Parse(resp.Header.Get("Location"))
	if err != nil {
		t.Fatal(err)
	}
	return u.Query()
}

// code runs authorize + consent(allow) and returns the authorization code.
func (h *harness) code(a authReq, cookie *http.Cookie, workspace string) string {
	h.t.Helper()
	resp, body := h.authorize(a, cookie)
	if resp.StatusCode != http.StatusOK {
		h.t.Fatalf("authorize: %d %s", resp.StatusCode, body)
	}
	request, csrf := consentFields(h.t, body)
	resp, _ = h.consent(cookie, request, csrf, workspace, "allow", nil)
	q := location(h.t, resp)
	if q.Get("code") == "" || q.Get("state") != a.state || q.Get("iss") != h.origin {
		h.t.Fatalf("consent redirect = %v", q)
	}
	return q.Get("code")
}

func (h *harness) token(form url.Values) (int, map[string]any) {
	h.t.Helper()
	resp, body := h.postForm(oauth.PathToken, form, nil, nil)
	var m map[string]any
	if err := json.Unmarshal(body, &m); err != nil {
		h.t.Fatalf("token body %q: %v", body, err)
	}
	if resp.Header.Get("Cache-Control") != "no-store" {
		h.t.Fatalf("token response Cache-Control = %q", resp.Header.Get("Cache-Control"))
	}
	return resp.StatusCode, m
}

func (h *harness) exchange(clientID, code, redirect, verifier string) (int, map[string]any) {
	return h.token(url.Values{"grant_type": {"authorization_code"}, "client_id": {clientID}, "code": {code}, "redirect_uri": {redirect}, "code_verifier": {verifier}})
}

func (h *harness) refresh(clientID, refresh string, extra url.Values) (int, map[string]any) {
	form := url.Values{"grant_type": {"refresh_token"}, "client_id": {clientID}, "refresh_token": {refresh}}
	for k, v := range extra {
		form[k] = v
	}
	return h.token(form)
}

func (h *harness) callResource(path, token string) (int, ports.Principal, http.Header) {
	h.t.Helper()
	req, _ := http.NewRequest(http.MethodGet, h.origin+path, nil)
	if token != "" {
		req.Header.Set("Authorization", "Bearer "+token)
	}
	resp, body := h.do(req)
	var p ports.Principal
	_ = json.Unmarshal(body, &p)
	return resp.StatusCode, p, resp.Header
}

func wantError(t *testing.T, status int, m map[string]any, wantStatus int, code string) {
	t.Helper()
	if status != wantStatus || m["error"] != code {
		t.Fatalf("got %d %v, want %d %s", status, m, wantStatus, code)
	}
	if _, ok := m["access_token"]; ok {
		t.Fatalf("error response carries an access token: %v", m)
	}
}

// grant runs a complete flow and returns client, verifier-independent tokens.
func (h *harness) grant(resource, scope, workspace string) (clientID, access, refresh string) {
	h.t.Helper()
	redirect := "https://connector.example/cb"
	clientID = h.client(redirect, "")
	verifier, challenge := pkce()
	cookie := h.session("alice", "ws-a", "ws-b", "ws-r")
	code := h.code(authReq{clientID: clientID, redirect: redirect, challenge: challenge, resource: resource, scope: scope, state: "s1"}, cookie, workspace)
	status, m := h.exchange(clientID, code, redirect, verifier)
	if status != http.StatusOK {
		h.t.Fatalf("exchange: %d %v", status, m)
	}
	return clientID, m["access_token"].(string), m["refresh_token"].(string)
}

// ---- checklist row 1: RFC 8414 authorization-server metadata ------------

func TestASMetadataLocatesRealEndpoints(t *testing.T) {
	h := newHarness(t)
	resp, body := h.get(oauth.PathASMetadata, nil)
	if resp.StatusCode != http.StatusOK || !strings.HasPrefix(resp.Header.Get("Content-Type"), "application/json") {
		t.Fatalf("metadata: %d %s", resp.StatusCode, resp.Header.Get("Content-Type"))
	}
	var md oauth.ASMetadata
	if err := json.Unmarshal(body, &md); err != nil {
		t.Fatal(err)
	}
	if md.Issuer != h.origin || strings.Join(md.CodeChallengeMethodsSupported, ",") != "S256" || strings.Join(md.ResponseTypesSupported, ",") != "code" {
		t.Fatalf("metadata = %+v", md)
	}
	if !containsAll(md.GrantTypesSupported, []string{"authorization_code", "refresh_token"}) || !containsAll(md.ProtectedResources, []string{h.origin, resourceB}) || !md.AuthorizationResponseISSSupported {
		t.Fatalf("metadata grants/resources = %+v", md)
	}
	// Each advertised endpoint is served by this server (not a 404).
	for name, endpoint := range map[string]string{"authorize": md.AuthorizationEndpoint, "token": md.TokenEndpoint, "register": md.RegistrationEndpoint, "revoke": md.RevocationEndpoint, "jwks": md.JWKSURI} {
		if !strings.HasPrefix(endpoint, h.origin+"/") {
			t.Fatalf("%s endpoint %q is not on the issuer", name, endpoint)
		}
		method := http.MethodPost
		if name == "authorize" || name == "jwks" {
			method = http.MethodGet
		}
		req, _ := http.NewRequest(method, endpoint, nil)
		resp, _ := h.do(req)
		if resp.StatusCode == http.StatusNotFound || resp.StatusCode == http.StatusMethodNotAllowed {
			t.Fatalf("%s endpoint %s answered %d", name, endpoint, resp.StatusCode)
		}
	}
	// The JWKS publishes the key that signs issued access tokens.
	_, access, _ := h.grant(h.origin, "catalog:read", "ws-a")
	header, _ := base64.RawURLEncoding.DecodeString(strings.Split(access, ".")[0])
	var hdr struct{ Kid, Alg string }
	_ = json.Unmarshal(header, &hdr)
	_, jwks := h.get(oauth.PathJWKS, nil)
	if hdr.Alg != "EdDSA" || !strings.Contains(string(jwks), `"kid":"`+hdr.Kid+`"`) || strings.Contains(string(jwks), `"d"`) {
		t.Fatalf("jwks %s does not publish token key %+v (or leaks private material)", jwks, hdr)
	}
}

// ---- checklist row 2: RFC 7591 dynamic client registration --------------

func TestDynamicClientRegistration(t *testing.T) {
	h := newHarness(t)
	status, m := h.register(map[string]any{"redirect_uris": []string{"https://claude.example/api/callback"}, "client_name": "Claude", "scope": "catalog:read", "token_endpoint_auth_method": "none", "grant_types": []string{"authorization_code", "refresh_token"}, "response_types": []string{"code"}})
	if status != http.StatusCreated || m["client_id"] == "" || m["scope"] != "catalog:read" || m["token_endpoint_auth_method"] != "none" {
		t.Fatalf("register: %d %v", status, m)
	}
	stored, err := h.store.GetClient(context.Background(), m["client_id"].(string))
	if err != nil || strings.Join(stored.Scopes, " ") != "catalog:read" {
		t.Fatalf("stored client = %+v %v", stored, err)
	}
	cases := []struct {
		name string
		body map[string]any
		code string
	}{
		{"http redirect", map[string]any{"redirect_uris": []string{"http://claude.example/cb"}}, "invalid_redirect_uri"},
		{"loopback http redirect", map[string]any{"redirect_uris": []string{"http://127.0.0.1:8080/cb"}}, "invalid_redirect_uri"},
		{"custom scheme", map[string]any{"redirect_uris": []string{"myapp://cb"}}, "invalid_redirect_uri"},
		{"fragment", map[string]any{"redirect_uris": []string{"https://claude.example/cb#x"}}, "invalid_redirect_uri"},
		{"relative", map[string]any{"redirect_uris": []string{"/cb"}}, "invalid_redirect_uri"},
		{"one bad among good", map[string]any{"redirect_uris": []string{"https://claude.example/cb", "http://claude.example/cb"}}, "invalid_redirect_uri"},
		{"no redirects", map[string]any{"client_name": "x"}, "invalid_redirect_uri"},
		{"excess scope", map[string]any{"redirect_uris": []string{"https://claude.example/cb"}, "scope": "catalog:read catalog:admin"}, "invalid_client_metadata"},
		{"execution scope", map[string]any{"redirect_uris": []string{"https://claude.example/cb"}, "scope": "execution:invoke"}, "invalid_client_metadata"},
		{"empty scope", map[string]any{"redirect_uris": []string{"https://claude.example/cb"}, "scope": " "}, "invalid_client_metadata"},
		{"confidential client", map[string]any{"redirect_uris": []string{"https://claude.example/cb"}, "token_endpoint_auth_method": "client_secret_basic"}, "invalid_client_metadata"},
		{"implicit grant", map[string]any{"redirect_uris": []string{"https://claude.example/cb"}, "grant_types": []string{"implicit"}}, "invalid_client_metadata"},
		{"token response", map[string]any{"redirect_uris": []string{"https://claude.example/cb"}, "response_types": []string{"token"}}, "invalid_client_metadata"},
	}
	for _, c := range cases {
		status, m := h.register(c.body)
		if status != http.StatusBadRequest || m["error"] != c.code || m["client_id"] != nil {
			t.Fatalf("%s: got %d %v, want 400 %s", c.name, status, m, c.code)
		}
	}
}

// ---- checklist row 3: authorization code + PKCE + consent ----------------

func TestAuthorizationCodeFlowIssuesBoundToken(t *testing.T) {
	h := newHarness(t)
	redirect := "https://connector.example/cb"
	clientID := h.client(redirect, "")
	verifier, challenge := pkce()
	cookie := h.session("alice", "ws-a", "ws-b")
	code := h.code(authReq{clientID: clientID, redirect: redirect, challenge: challenge, resource: h.origin, scope: "catalog:read catalog:publish", state: "xyz"}, cookie, "ws-a")
	status, m := h.exchange(clientID, code, redirect, verifier)
	if status != http.StatusOK || m["token_type"] != "Bearer" || m["scope"] != "catalog:read catalog:publish" || m["refresh_token"] == nil {
		t.Fatalf("exchange: %d %v", status, m)
	}
	if exp := m["expires_in"].(float64); exp <= 0 || exp > 300 {
		t.Fatalf("expires_in = %v", exp)
	}
	got, p, _ := h.callResource("/resource-a", m["access_token"].(string))
	if got != http.StatusOK || p.Subject != "alice" || p.WorkspaceID != "ws-a" || p.Audience != h.origin || p.Issuer != h.origin || p.SubjectType != "human" || strings.Join(p.Scopes, " ") != "catalog:publish catalog:read" {
		t.Fatalf("resource saw %d %+v", got, p)
	}
	if rec, ok := h.identities.get("ws-a", h.origin, "alice"); !ok || rec.SubjectType != "human" {
		t.Fatalf("issued identity not recorded: %+v %v", rec, ok)
	}
	// Raw codes and refresh tokens never reach the store; only hashes do.
	dump := h.store.dump()
	if strings.Contains(dump, code) || strings.Contains(dump, m["refresh_token"].(string)) {
		t.Fatal("store holds a raw code or refresh token")
	}
	sum := sha256.Sum256([]byte(code))
	if _, err := h.store.RedeemCode(context.Background(), "ws-a", sum[:], func(oauth.AuthCode) error { return nil }); !errors.Is(err, oauth.ErrGrantReused) {
		t.Fatalf("code is not stored as its SHA-256 hash: %v", err)
	}
}

func TestPKCEIsS256OnlyAndRequired(t *testing.T) {
	h := newHarness(t)
	redirect := "https://connector.example/cb"
	clientID := h.client(redirect, "")
	verifier, challenge := pkce()
	cookie := h.session("alice", "ws-a")
	for name, a := range map[string]authReq{
		"plain":        {method: "plain", challenge: verifier},
		"missing":      {challenge: ""},
		"not s256":     {challenge: "short"},
		"bad response": {challenge: challenge},
	} {
		a.clientID, a.redirect, a.resource, a.scope, a.state = clientID, redirect, h.origin, "catalog:read", "st"
		if name == "bad response" {
			resp, _ := h.get(oauth.PathAuthorize+"?"+url.Values{"response_type": {"token"}, "client_id": {clientID}, "redirect_uri": {redirect}, "code_challenge": {challenge}, "code_challenge_method": {"S256"}, "resource": {h.origin}, "state": {"st"}}.Encode(), cookie)
			if q := location(t, resp); q.Get("error") != "unsupported_response_type" || q.Get("code") != "" {
				t.Fatalf("%s: %v", name, q)
			}
			continue
		}
		resp, _ := h.authorize(a, cookie)
		q := location(t, resp)
		if q.Get("error") != "invalid_request" || q.Get("state") != "st" || q.Get("code") != "" {
			t.Fatalf("%s: redirect = %v", name, q)
		}
	}
	code := h.code(authReq{clientID: clientID, redirect: redirect, challenge: challenge, resource: h.origin, scope: "catalog:read", state: "s"}, cookie, "ws-a")
	other, _ := pkce()
	status, m := h.exchange(clientID, code, redirect, other)
	wantError(t, status, m, 400, "invalid_grant")
	status, m = h.token(url.Values{"grant_type": {"authorization_code"}, "client_id": {clientID}, "code": {code}, "redirect_uri": {redirect}})
	wantError(t, status, m, 400, "invalid_request")
	// A failed verifier does not burn the code for the real client.
	if status, m := h.exchange(clientID, code, redirect, verifier); status != http.StatusOK {
		t.Fatalf("legitimate exchange after failed attempt: %d %v", status, m)
	}
}

func TestStolenCodeAndReplay(t *testing.T) {
	h := newHarness(t)
	redirect := "https://connector.example/cb"
	victim := h.client(redirect, "")
	attacker := h.client("https://attacker.example/cb", "")
	verifier, challenge := pkce()
	cookie := h.session("alice", "ws-a")
	code := h.code(authReq{clientID: victim, redirect: redirect, challenge: challenge, resource: h.origin, scope: "catalog:read", state: "s"}, cookie, "ws-a")
	// The attacker holds the code but is a different client ...
	status, m := h.exchange(attacker, code, "https://attacker.example/cb", verifier)
	wantError(t, status, m, 400, "invalid_grant")
	status, m = h.exchange(attacker, code, redirect, verifier)
	wantError(t, status, m, 400, "invalid_grant")
	// ... or lacks the verifier.
	wrong, _ := pkce()
	status, m = h.exchange(victim, code, redirect, wrong)
	wantError(t, status, m, 400, "invalid_grant")
	// Forged or foreign-workspace codes are unknown.
	status, m = h.exchange(victim, strings.Replace(code, strings.Split(code, ".")[1], base64.RawURLEncoding.EncodeToString([]byte("ws-b")), 1), redirect, verifier)
	wantError(t, status, m, 400, "invalid_grant")
	status, m = h.exchange(victim, code, redirect, verifier)
	if status != http.StatusOK {
		t.Fatalf("victim exchange: %d %v", status, m)
	}
	refresh := m["refresh_token"].(string)
	// Replaying a redeemed code fails and revokes the grant made from it.
	status, m = h.exchange(victim, code, redirect, verifier)
	wantError(t, status, m, 400, "invalid_grant")
	status, m = h.refresh(victim, refresh, nil)
	wantError(t, status, m, 400, "invalid_grant")
}

func TestCodeExpires(t *testing.T) {
	h := newHarness(t)
	redirect := "https://connector.example/cb"
	clientID := h.client(redirect, "")
	verifier, challenge := pkce()
	code := h.code(authReq{clientID: clientID, redirect: redirect, challenge: challenge, resource: h.origin, scope: "catalog:read", state: "s"}, h.session("alice", "ws-a"), "ws-a")
	h.clock.Advance(61 * time.Second)
	status, m := h.exchange(clientID, code, redirect, verifier)
	wantError(t, status, m, 400, "invalid_grant")
}

func TestRedirectMismatch(t *testing.T) {
	h := newHarness(t)
	redirect := "https://connector.example/cb"
	clientID := h.client(redirect, "")
	verifier, challenge := pkce()
	cookie := h.session("alice", "ws-a")
	for _, bad := range []string{"https://attacker.example/cb", "https://connector.example/cb/", "https://connector.example/cb?x=1", "https://connector.example/CB", ""} {
		resp, body := h.authorize(authReq{clientID: clientID, redirect: bad, challenge: challenge, resource: h.origin, scope: "catalog:read", state: "s"}, cookie)
		if resp.StatusCode != http.StatusBadRequest || resp.Header.Get("Location") != "" || bytes.Contains(body, []byte(`name="request"`)) {
			t.Fatalf("redirect %q: %d location=%q", bad, resp.StatusCode, resp.Header.Get("Location"))
		}
	}
	resp, _ := h.authorize(authReq{clientID: "gc_unknown", redirect: redirect, challenge: challenge, resource: h.origin, scope: "catalog:read"}, cookie)
	if resp.StatusCode != http.StatusBadRequest || resp.Header.Get("Location") != "" {
		t.Fatalf("unknown client: %d %q", resp.StatusCode, resp.Header.Get("Location"))
	}
	code := h.code(authReq{clientID: clientID, redirect: redirect, challenge: challenge, resource: h.origin, scope: "catalog:read", state: "s"}, cookie, "ws-a")
	// Redeeming with a different (even registered) redirect URI fails.
	status, m := h.exchange(clientID, code, redirect+"/alt", verifier)
	wantError(t, status, m, 400, "invalid_grant")
	if status, m := h.exchange(clientID, code, redirect, verifier); status != http.StatusOK {
		t.Fatalf("exact redirect exchange: %d %v", status, m)
	}
}

func TestConsentDeniedIssuesNothing(t *testing.T) {
	h := newHarness(t)
	redirect := "https://connector.example/cb"
	clientID := h.client(redirect, "")
	_, challenge := pkce()
	cookie := h.session("alice", "ws-a")
	resp, body := h.authorize(authReq{clientID: clientID, redirect: redirect, challenge: challenge, resource: h.origin, scope: "catalog:read", state: "deny-state"}, cookie)
	if resp.StatusCode != http.StatusOK || resp.Header.Get("X-Frame-Options") != "DENY" || !strings.Contains(resp.Header.Get("Content-Security-Policy"), "frame-ancestors 'none'") {
		t.Fatalf("consent page: %d %v", resp.StatusCode, resp.Header)
	}
	if !bytes.Contains(body, []byte("Connector")) || !bytes.Contains(body, []byte("catalog:read")) || !bytes.Contains(body, []byte("ws-a")) {
		t.Fatalf("consent page does not show client, scope and workspace: %s", body)
	}
	if h.store.codeCount() != 0 {
		t.Fatal("authorize alone created a code")
	}
	request, csrf := consentFields(t, body)
	resp, _ = h.consent(cookie, request, csrf, "ws-a", "deny", nil)
	q := location(t, resp)
	if q.Get("error") != "access_denied" || q.Get("code") != "" || q.Get("state") != "deny-state" || q.Get("iss") != h.origin {
		t.Fatalf("deny redirect = %v", q)
	}
	if h.store.codeCount() != 0 {
		t.Fatal("denied consent created a code")
	}
}

func TestConsentCSRFAndSessionBinding(t *testing.T) {
	h := newHarness(t)
	redirect := "https://connector.example/cb"
	clientID := h.client(redirect, "")
	_, challenge := pkce()
	victim := h.session("alice", "ws-a")
	a := authReq{clientID: clientID, redirect: redirect, challenge: challenge, resource: h.origin, scope: "catalog:read", state: "s"}
	_, body := h.authorize(a, victim)
	request, csrf := consentFields(t, body)
	// An attacker's own consent form (their session, their CSRF token).
	h.policy.set("mallory", "ws-a", []string{"catalog:read"}, true)
	attacker := h.session("mallory", "ws-a")
	_, abody := h.authorize(a, attacker)
	aRequest, aCSRF := consentFields(t, abody)

	tampered := []byte(request)
	tampered[5] ^= 1
	cases := []struct {
		name          string
		cookie        *http.Cookie
		request, csrf string
		headers       map[string]string
		status        int
	}{
		{"missing csrf", victim, request, "", nil, http.StatusForbidden},
		{"wrong csrf", victim, request, aCSRF, nil, http.StatusForbidden},
		{"attacker form in victim session", victim, aRequest, aCSRF, nil, http.StatusForbidden},
		{"victim form in attacker session", attacker, request, csrf, nil, http.StatusForbidden},
		{"tampered request", victim, string(tampered), csrf, nil, http.StatusForbidden},
		{"cross-origin post", victim, request, csrf, map[string]string{"Origin": "https://attacker.example"}, http.StatusForbidden},
		{"cross-site fetch", victim, request, csrf, map[string]string{"Sec-Fetch-Site": "cross-site"}, http.StatusForbidden},
		{"no session", nil, request, csrf, nil, http.StatusUnauthorized},
	}
	for _, c := range cases {
		resp, _ := h.consent(c.cookie, c.request, c.csrf, "ws-a", "allow", c.headers)
		if resp.StatusCode != c.status || resp.Header.Get("Location") != "" {
			t.Fatalf("%s: %d location=%q, want %d", c.name, resp.StatusCode, resp.Header.Get("Location"), c.status)
		}
	}
	if h.store.codeCount() != 0 {
		t.Fatal("a rejected consent created a code")
	}
	// The consent request expires.
	h.clock.Advance(11 * time.Minute)
	victim = h.session("alice", "ws-a")
	resp, _ := h.consent(victim, request, csrf, "ws-a", "allow", nil)
	if resp.StatusCode != http.StatusForbidden {
		t.Fatalf("expired consent request: %d", resp.StatusCode)
	}
	// Unauthenticated authorize never renders consent.
	resp, body = h.authorize(a, nil)
	if resp.StatusCode != http.StatusUnauthorized || bytes.Contains(body, []byte(`name="request"`)) {
		t.Fatalf("authorize without session: %d", resp.StatusCode)
	}
	// Same-origin POST succeeds.
	_, body = h.authorize(a, victim)
	request, csrf = consentFields(t, body)
	resp, _ = h.consent(victim, request, csrf, "ws-a", "allow", map[string]string{"Origin": h.origin, "Sec-Fetch-Site": "same-origin"})
	if q := location(t, resp); q.Get("code") == "" {
		t.Fatalf("same-origin consent: %v", q)
	}
}

// ---- role and scope escalation ------------------------------------------

func TestScopeEscalationIsRejectedNotDowngraded(t *testing.T) {
	h := newHarness(t)
	redirect := "https://connector.example/cb"
	readOnly := h.client(redirect, "catalog:read")
	verifier, challenge := pkce()
	cookie := h.session("alice", "ws-a", "ws-r")
	// Beyond the client's registered scopes.
	resp, _ := h.authorize(authReq{clientID: readOnly, redirect: redirect, challenge: challenge, resource: h.origin, scope: "catalog:read catalog:publish", state: "s"}, cookie)
	if q := location(t, resp); q.Get("error") != "invalid_scope" || q.Get("code") != "" {
		t.Fatalf("client scope escalation: %v", q)
	}
	// Unknown scope.
	full := h.client(redirect, "")
	resp, _ = h.authorize(authReq{clientID: full, redirect: redirect, challenge: challenge, resource: h.origin, scope: "catalog:read admin", state: "s"}, cookie)
	if q := location(t, resp); q.Get("error") != "invalid_scope" {
		t.Fatalf("unknown scope: %v", q)
	}
	// Beyond the person's role in the chosen workspace: reader asks to publish.
	_, body := h.authorize(authReq{clientID: full, redirect: redirect, challenge: challenge, resource: h.origin, scope: "catalog:read catalog:publish", state: "s"}, cookie)
	request, csrf := consentFields(t, body)
	resp, _ = h.consent(cookie, request, csrf, "ws-r", "allow", nil)
	if q := location(t, resp); q.Get("error") != "invalid_scope" || q.Get("code") != "" {
		t.Fatalf("role escalation: %v", q)
	}
	if h.store.codeCount() != 0 {
		t.Fatal("escalation created a code")
	}
	// Refresh cannot widen the grant, and narrowing does not shrink the family.
	code := h.code(authReq{clientID: full, redirect: redirect, challenge: challenge, resource: h.origin, scope: "catalog:read", state: "s"}, cookie, "ws-a")
	status, m := h.exchange(full, code, redirect, verifier)
	if status != http.StatusOK {
		t.Fatalf("exchange: %d %v", status, m)
	}
	refresh := m["refresh_token"].(string)
	status, m = h.refresh(full, refresh, url.Values{"scope": {"catalog:read catalog:publish"}})
	wantError(t, status, m, 400, "invalid_scope")
	// The rejected widening did not consume the token.
	status, m = h.refresh(full, refresh, nil)
	if status != http.StatusOK || m["scope"] != "catalog:read" {
		t.Fatalf("refresh after rejected widening: %d %v", status, m)
	}
	// Demoting the person below the grant ends it at the next refresh.
	h.policy.set("alice", "ws-a", []string{"catalog:publish"}, true)
	status, m = h.refresh(full, m["refresh_token"].(string), nil)
	wantError(t, status, m, 400, "invalid_grant")
}

// ---- checklist row 4: refresh rotation and reuse containment -------------

func TestRefreshRotatesAndReuseRevokesFamily(t *testing.T) {
	h := newHarness(t)
	clientID, _, r0 := h.grant(h.origin, "catalog:read catalog:publish", "ws-a")
	status, m := h.refresh(clientID, r0, nil)
	if status != http.StatusOK || m["refresh_token"] == r0 || m["scope"] != "catalog:read catalog:publish" {
		t.Fatalf("rotate: %d %v", status, m)
	}
	r1 := m["refresh_token"].(string)
	if got, p, _ := h.callResource("/resource-a", m["access_token"].(string)); got != http.StatusOK || p.WorkspaceID != "ws-a" || strings.Join(p.Scopes, " ") != "catalog:publish catalog:read" {
		t.Fatalf("refreshed token: %d %+v", got, p)
	}
	// Another client cannot use it (and does not burn it).
	other := h.client("https://other.example/cb", "")
	status, m = h.refresh(other, r1, nil)
	wantError(t, status, m, 400, "invalid_grant")
	// Reusing the rotated-out token revokes the family ...
	status, m = h.refresh(clientID, r0, nil)
	wantError(t, status, m, 400, "invalid_grant")
	// ... so the current token is dead too.
	status, m = h.refresh(clientID, r1, nil)
	wantError(t, status, m, 400, "invalid_grant")
	// Garbage tokens are invalid_grant.
	status, m = h.refresh(clientID, "gr1.d3MtYQ.AAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAA", nil)
	wantError(t, status, m, 400, "invalid_grant")
}

func TestRefreshRejectsRevokedConsent(t *testing.T) {
	h := newHarness(t)
	clientID, _, r0 := h.grant(h.origin, "catalog:read", "ws-a")
	h.policy.set("alice", "ws-a", []string{"catalog:read"}, false)
	status, m := h.refresh(clientID, r0, nil)
	wantError(t, status, m, 400, "invalid_grant")
	// Restoring membership does not revive the ended family.
	h.policy.set("alice", "ws-a", []string{"catalog:read"}, true)
	status, m = h.refresh(clientID, r0, nil)
	wantError(t, status, m, 400, "invalid_grant")
}

func TestRefreshExpires(t *testing.T) {
	h := newHarness(t)
	clientID, _, r0 := h.grant(h.origin, "catalog:read", "ws-a")
	h.clock.Advance(31 * 24 * time.Hour)
	status, m := h.refresh(clientID, r0, nil)
	wantError(t, status, m, 400, "invalid_grant")
}

func TestRevocationEndpoint(t *testing.T) {
	h := newHarness(t)
	clientID, access, r0 := h.grant(h.origin, "catalog:read", "ws-a")
	// Another client cannot revoke someone else's grant.
	other := h.client("https://other.example/cb", "")
	resp, _ := h.postForm(oauth.PathRevoke, url.Values{"client_id": {other}, "token": {r0}}, nil, nil)
	if resp.StatusCode != http.StatusOK {
		t.Fatalf("foreign revoke: %d", resp.StatusCode)
	}
	status, m := h.refresh(clientID, r0, nil)
	if status != http.StatusOK {
		t.Fatalf("foreign revoke took effect: %d %v", status, m)
	}
	r1 := m["refresh_token"].(string)
	resp, _ = h.postForm(oauth.PathRevoke, url.Values{"client_id": {clientID}, "token": {r1}, "token_type_hint": {"refresh_token"}}, nil, nil)
	if resp.StatusCode != http.StatusOK {
		t.Fatalf("revoke: %d", resp.StatusCode)
	}
	status, m = h.refresh(clientID, r1, nil)
	wantError(t, status, m, 400, "invalid_grant")
	resp, _ = h.postForm(oauth.PathRevoke, url.Values{"client_id": {clientID}, "token": {"unknown"}}, nil, nil)
	if resp.StatusCode != http.StatusOK {
		t.Fatalf("unknown token revoke: %d", resp.StatusCode)
	}
	resp, body := h.postForm(oauth.PathRevoke, url.Values{"client_id": {clientID}, "token": {access}}, nil, nil)
	if resp.StatusCode != http.StatusBadRequest || !bytes.Contains(body, []byte("unsupported_token_type")) {
		t.Fatalf("access token revoke: %d %s", resp.StatusCode, body)
	}
}

// ---- checklist row 5: RFC 9728 protected-resource metadata + challenge ---

func TestProtectedResourceMetadataAndChallenge(t *testing.T) {
	h := newHarness(t)
	resp, body := h.get(oauth.PathPRMetadata, nil)
	var prm oauth.ProtectedResourceMetadata
	if resp.StatusCode != http.StatusOK || json.Unmarshal(body, &prm) != nil {
		t.Fatalf("prm: %d %s", resp.StatusCode, body)
	}
	if prm.Resource != h.origin || strings.Join(prm.AuthorizationServers, ",") != h.origin || !containsAll(prm.ScopesSupported, []string{"catalog:read", "catalog:publish"}) || strings.Join(prm.BearerMethodsSupported, ",") != "header" {
		t.Fatalf("prm = %+v", prm)
	}
	// The AS listed in the PRM really serves RFC 8414 metadata.
	resp, _ = h.get(oauth.PathASMetadata, nil)
	if resp.StatusCode != http.StatusOK {
		t.Fatalf("AS metadata from PRM: %d", resp.StatusCode)
	}
	wantPRM := `resource_metadata="` + h.origin + oauth.PathPRMetadata + `"`
	status, _, hdr := h.callResource("/resource-a", "")
	challenge := hdr.Get("WWW-Authenticate")
	if status != http.StatusUnauthorized || !strings.HasPrefix(challenge, "Bearer ") || !strings.Contains(challenge, wantPRM) || strings.Contains(challenge, "oauth-authorization-server") || strings.Contains(challenge, "error=") {
		t.Fatalf("anonymous challenge: %d %q", status, challenge)
	}
	status, _, hdr = h.callResource("/resource-a", "not-a-token")
	if challenge := hdr.Get("WWW-Authenticate"); status != http.StatusUnauthorized || !strings.Contains(challenge, wantPRM) || !strings.Contains(challenge, `error="invalid_token"`) {
		t.Fatalf("invalid-token challenge: %d %q", status, challenge)
	}
}

// ---- checklist row 6: RFC 8707 resource indicators and audience ---------

func TestResourceIndicatorBindsAudience(t *testing.T) {
	h := newHarness(t)
	_, tokenA, refreshA := h.grant(h.origin, "catalog:read", "ws-a")
	clientB, tokenB, refreshB := h.grant(resourceB, "catalog:read", "ws-a")
	if status, p, _ := h.callResource("/resource-a", tokenA); status != http.StatusOK || p.Audience != h.origin {
		t.Fatalf("A at A: %d %+v", status, p)
	}
	if status, p, _ := h.callResource("/resource-b", tokenB); status != http.StatusOK || p.Audience != resourceB {
		t.Fatalf("B at B: %d %+v", status, p)
	}
	if status, _, hdr := h.callResource("/resource-a", tokenB); status != http.StatusUnauthorized || !strings.Contains(hdr.Get("WWW-Authenticate"), `error="invalid_token"`) {
		t.Fatalf("token for B accepted at A: %d", status)
	}
	if status, _, _ := h.callResource("/resource-b", tokenA); status != http.StatusUnauthorized {
		t.Fatalf("token for A accepted at B: %d", status)
	}
	// Refresh keeps the audience and cannot switch resources.
	status, m := h.refresh(clientB, refreshB, url.Values{"resource": {h.origin}})
	wantError(t, status, m, 400, "invalid_target")
	status, m = h.refresh(clientB, refreshB, url.Values{"resource": {resourceB}})
	if status != http.StatusOK {
		t.Fatalf("refresh B: %d %v", status, m)
	}
	if status, _, _ := h.callResource("/resource-a", m["access_token"].(string)); status != http.StatusUnauthorized {
		t.Fatalf("refreshed B token accepted at A: %d", status)
	}
	_ = refreshA
	// Authorize requires a known resource.
	redirect := "https://connector.example/cb"
	clientID := h.client(redirect, "")
	verifier, challenge := pkce()
	cookie := h.session("alice", "ws-a")
	for _, res := range []string{"", "https://unknown.example", h.origin + "/other"} {
		resp, _ := h.authorize(authReq{clientID: clientID, redirect: redirect, challenge: challenge, resource: res, scope: "catalog:read", state: "s"}, cookie)
		if q := location(t, resp); q.Get("error") != "invalid_target" {
			t.Fatalf("resource %q: %v", res, q)
		}
	}
	// Redemption must name the same resource as the authorization request.
	code := h.code(authReq{clientID: clientID, redirect: redirect, challenge: challenge, resource: h.origin, scope: "catalog:read", state: "s"}, cookie, "ws-a")
	status, m = h.token(url.Values{"grant_type": {"authorization_code"}, "client_id": {clientID}, "code": {code}, "redirect_uri": {redirect}, "code_verifier": {verifier}, "resource": {resourceB}})
	wantError(t, status, m, 400, "invalid_target")
	status, m = h.token(url.Values{"grant_type": {"authorization_code"}, "client_id": {clientID}, "code": {code}, "redirect_uri": {redirect}, "code_verifier": {verifier}, "resource": {h.origin}})
	if status != http.StatusOK {
		t.Fatalf("matching resource redemption: %d %v", status, m)
	}
}

// ---- workspace selection and reissue -------------------------------------

func TestWorkspaceSelectionAndReissue(t *testing.T) {
	h := newHarness(t)
	redirect := "https://connector.example/cb"
	clientID := h.client(redirect, "")
	cookie := h.session("alice", "ws-a", "ws-b")
	grantIn := func(ws string) (string, string) {
		verifier, challenge := pkce()
		code := h.code(authReq{clientID: clientID, redirect: redirect, challenge: challenge, resource: h.origin, scope: "catalog:read", state: "s"}, cookie, ws)
		status, m := h.exchange(clientID, code, redirect, verifier)
		if status != http.StatusOK {
			t.Fatalf("exchange %s: %d %v", ws, status, m)
		}
		return m["access_token"].(string), m["refresh_token"].(string)
	}
	accessA, refreshA := grantIn("ws-a")
	accessB, _ := grantIn("ws-b")
	if _, p, _ := h.callResource("/resource-a", accessA); p.WorkspaceID != "ws-a" {
		t.Fatalf("first grant workspace = %q", p.WorkspaceID)
	}
	// Reissuing for another workspace needs a new consent and yields a
	// token for that workspace only.
	if _, p, _ := h.callResource("/resource-a", accessB); p.WorkspaceID != "ws-b" {
		t.Fatalf("reissued grant workspace = %q", p.WorkspaceID)
	}
	// Refresh cannot move a grant to another workspace.
	status, m := h.refresh(clientID, refreshA, url.Values{"workspace_id": {"ws-b"}, "workspace": {"ws-b"}})
	if status != http.StatusOK {
		t.Fatalf("refresh A: %d %v", status, m)
	}
	if _, p, _ := h.callResource("/resource-a", m["access_token"].(string)); p.WorkspaceID != "ws-a" {
		t.Fatalf("refresh moved workspace to %q", p.WorkspaceID)
	}
	// A workspace the session does not offer cannot be selected by editing
	// the form, even where a membership exists (ws-c) ...
	_, challenge := pkce()
	a := authReq{clientID: clientID, redirect: redirect, challenge: challenge, resource: h.origin, scope: "catalog:read", state: "s"}
	_, body := h.authorize(a, cookie)
	request, csrf := consentFields(t, body)
	resp, _ := h.consent(cookie, request, csrf, "ws-c", "allow", nil)
	if q := location(t, resp); q.Get("error") != "access_denied" || q.Get("code") != "" {
		t.Fatalf("unoffered workspace: %v", q)
	}
	// ... and an offered workspace without a membership is denied.
	noMember := h.session("alice", "ws-x")
	_, body = h.authorize(a, noMember)
	request, csrf = consentFields(t, body)
	resp, _ = h.consent(noMember, request, csrf, "ws-x", "allow", nil)
	if q := location(t, resp); q.Get("error") != "access_denied" || q.Get("code") != "" {
		t.Fatalf("non-member workspace: %v", q)
	}
}

func TestTokenEndpointRequestHygiene(t *testing.T) {
	h := newHarness(t)
	clientID, _, r0 := h.grant(h.origin, "catalog:read", "ws-a")
	status, m := h.token(url.Values{"grant_type": {"password"}, "client_id": {clientID}})
	wantError(t, status, m, 400, "unsupported_grant_type")
	status, m = h.token(url.Values{"grant_type": {"refresh_token"}, "client_id": {"gc_nope"}, "refresh_token": {r0}})
	wantError(t, status, m, 401, "invalid_client")
	status, m = h.token(url.Values{"grant_type": {"refresh_token"}, "refresh_token": {r0}})
	wantError(t, status, m, 401, "invalid_client")
	req, _ := http.NewRequest(http.MethodPost, h.origin+oauth.PathToken, strings.NewReader(`{"grant_type":"refresh_token"}`))
	req.Header.Set("Content-Type", "application/json")
	resp, _ := h.do(req)
	if resp.StatusCode != http.StatusBadRequest {
		t.Fatalf("json token request: %d", resp.StatusCode)
	}
	req, _ = http.NewRequest(http.MethodPost, h.origin+oauth.PathToken+"?refresh_token="+url.QueryEscape(r0), strings.NewReader(url.Values{"grant_type": {"refresh_token"}, "client_id": {clientID}}.Encode()))
	req.Header.Set("Content-Type", "application/x-www-form-urlencoded")
	resp, _ = h.do(req)
	if resp.StatusCode != http.StatusBadRequest {
		t.Fatalf("query-string credentials accepted: %d", resp.StatusCode)
	}
	// The earlier rejections did not consume the refresh token.
	if status, m := h.refresh(clientID, r0, nil); status != http.StatusOK {
		t.Fatalf("refresh after rejected requests: %d %v", status, m)
	}
}

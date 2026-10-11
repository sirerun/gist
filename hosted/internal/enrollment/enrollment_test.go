package enrollment

import (
	"context"
	"crypto/sha256"
	"encoding/base64"
	"encoding/json"
	"io"
	"net/http"
	"net/http/httptest"
	"net/url"
	"strconv"
	"strings"
	"testing"
	"time"
)

type memoryStore struct {
	flows         map[string]time.Time
	used          map[string]bool
	members       map[string]string
	invites       map[string]inviteRecord
	bootstrapped  bool
	created       int
	githubID      string
	githubCalls   []string
	tokenVerifier string
}
type inviteRecord struct {
	account, role string
	scopes        []string
	expires       time.Time
	used          bool
}

func newMemoryStore() *memoryStore {
	return &memoryStore{flows: map[string]time.Time{}, used: map[string]bool{}, members: map[string]string{}, invites: map[string]inviteRecord{}}
}
func key(b []byte) string { return base64.RawURLEncoding.EncodeToString(b) }
func (s *memoryStore) CreateFlow(_ context.Context, h []byte, e time.Time) error {
	s.flows[key(h)] = e
	return nil
}
func (s *memoryStore) ConsumeFlow(_ context.Context, h []byte, n time.Time) error {
	k := key(h)
	e, ok := s.flows[k]
	if !ok || s.used[k] || !n.Before(e) {
		return ErrReplay
	}
	s.used[k] = true
	return nil
}
func (s *memoryStore) CompleteLogin(_ context.Context, ws, account string, invite []byte, founder, name string, scopes []string, now time.Time) error {
	if role, ok := s.members[ws+":"+account]; ok && role != "" {
		return nil
	}
	if len(invite) > 0 {
		k := key(invite)
		i, ok := s.invites[k]
		if !ok || i.used || i.account != account || !now.Before(i.expires) {
			return ErrDenied
		}
		i.used = true
		s.invites[k] = i
		s.members[ws+":"+account] = i.role
		return nil
	}
	if account != founder || s.bootstrapped {
		return ErrDenied
	}
	s.bootstrapped = true
	s.members[ws+":"+account] = "maintainer"
	return nil
}
func (s *memoryStore) CreateInvitation(_ context.Context, ws, actor, account, role string, scopes []string, hash []byte, expires time.Time) error {
	if s.members[ws+":"+actor] != "maintainer" {
		return ErrDenied
	}
	s.invites[key(hash)] = inviteRecord{account: account, role: role, scopes: append([]string(nil), scopes...), expires: expires}
	s.created++
	return nil
}

type fakeSessions struct{ subject string }

func (s *fakeSessions) Issue(subject string, _ []string, _ time.Duration) (*http.Cookie, error) {
	s.subject = subject
	return &http.Cookie{Name: "session", Value: "issued", Path: "/", Secure: true, HttpOnly: true}, nil
}

type fakeAuth struct{ session Session }

func (a fakeAuth) Authenticate(*http.Request) (Session, error) { return a.session, nil }

type roundTripFunc func(*http.Request) (*http.Response, error)

func (f roundTripFunc) RoundTrip(r *http.Request) (*http.Response, error) { return f(r) }
func response(status int, body string) *http.Response {
	return &http.Response{StatusCode: status, Header: make(http.Header), Body: io.NopCloser(strings.NewReader(body))}
}
func testHandler(t *testing.T, s *memoryStore, auth SessionAuthenticator) (*Handler, *fakeSessions) {
	t.Helper()
	sessions := &fakeSessions{}
	client := &http.Client{Transport: roundTripFunc(func(r *http.Request) (*http.Response, error) {
		s.githubCalls = append(s.githubCalls, r.Method+" "+r.URL.Host+r.URL.Path)
		switch r.URL.Host + r.URL.Path {
		case "github.com/login/oauth/access_token":
			if err := r.ParseForm(); err != nil {
				return response(400, "{}"), nil
			}
			if r.Form.Get("code") != "synthetic-code" || r.Form.Get("client_id") != "client" || r.Form.Get("client_secret") != "secret" || r.Form.Get("redirect_uri") != "https://gist.example/auth/github/callback" || r.Form.Get("code_verifier") == "" {
				return response(400, "{}"), nil
			}
			s.tokenVerifier = r.Form.Get("code_verifier")
			return response(200, "{\"access_token\":\"synthetic-token\",\"token_type\":\"bearer\"}"), nil
		case "api.github.com/user":
			if r.Method != http.MethodGet || r.Header.Get("Authorization") != "Bearer synthetic-token" {
				return response(401, "{}"), nil
			}
			id := s.githubID
			if id == "" {
				id = "12345"
			}
			return response(200, "{\"id\":"+id+",\"login\":\"synthetic\"}"), nil
		default:
			return response(404, "{}"), nil
		}
	})}
	h, err := New(Config{ClientID: "client", ClientSecret: "secret", CallbackURL: "https://gist.example/auth/github/callback", PublicOrigin: "https://gist.example", WorkspaceID: "private", WorkspaceName: "Private", FounderAccount: "12345", BootstrapScopes: []string{"catalog:read", "catalog:publish"}, AllowedScopes: []string{"catalog:read", "catalog:publish"}, FlowKey: []byte(strings.Repeat("f", 32)), CSRFKey: []byte(strings.Repeat("c", 32)), FlowTTL: 5 * time.Minute}, s, sessions, auth, client)
	if err != nil {
		t.Fatal(err)
	}
	h.now = func() time.Time { return time.Date(2026, 10, 11, 0, 0, 0, 0, time.UTC) }
	return h, sessions
}
func TestStartAndCallbackBootstrapFounderWithPKCE(t *testing.T) {
	store := newMemoryStore()
	h, sessions := testHandler(t, store, fakeAuth{})
	start := httptest.NewRequest(http.MethodGet, "https://gist.example"+StartPath, nil)
	start.Header.Set("Referer", "https://gist.example/invite")
	rec := httptest.NewRecorder()
	h.start(rec, start)
	if rec.Code != http.StatusFound || rec.Header().Get("Referrer-Policy") != "no-referrer" {
		t.Fatalf("start response: %d headers=%v", rec.Code, rec.Header())
	}
	loc, err := url.Parse(rec.Header().Get("Location"))
	if err != nil {
		t.Fatal(err)
	}
	q := loc.Query()
	if q.Get("code_challenge_method") != "S256" || q.Get("client_id") != "client" || q.Get("redirect_uri") != h.cfg.CallbackURL {
		t.Fatalf("authorize URL missing required fields: %s", loc)
	}
	if q.Has("scope") || q.Has("redirect_to") || q.Has("return_to") {
		t.Fatalf("GitHub authorization requested permissions or a caller-selected return path: %s", loc)
	}
	cookies := rec.Result().Cookies()
	if len(cookies) == 0 {
		t.Fatal("flow cookie missing")
	}
	flow := cookies[0]
	if !flow.Secure || !flow.HttpOnly || flow.SameSite != http.SameSiteLaxMode || flow.Path != "/" {
		t.Fatalf("flow cookie flags: %#v", flow)
	}
	leaked := httptest.NewRecorder()
	h.start(leaked, httptest.NewRequest(http.MethodGet, StartPath+"?invite=secret", nil))
	if leaked.Code != http.StatusBadRequest {
		t.Fatalf("invitation query token accepted: %d", leaked.Code)
	}
	state, verifier, invite, _, err := h.openFlow(flow.Value)
	if err != nil || invite != "" {
		t.Fatalf("open flow: invite=%q err=%v", invite, err)
	}
	challenge := sha256.Sum256([]byte(verifier))
	if q.Get("state") != state || q.Get("code_challenge") != base64.RawURLEncoding.EncodeToString(challenge[:]) {
		t.Fatal("state or S256 challenge mismatch")
	}
	if store.tokenVerifier != "" {
		t.Fatal("code verifier reached the token endpoint before callback")
	}
	if _, ok := store.flows[key([]byte(state))]; ok {
		t.Fatal("raw OAuth state was stored")
	}
	if _, ok := store.flows[key(hashToken(state))]; !ok {
		t.Fatal("hashed OAuth state was not stored")
	}
	badState := httptest.NewRequest(http.MethodGet, CallbackPath+"?code=synthetic-code&state=wrong-state", nil)
	badState.AddCookie(flow)
	badStateRec := httptest.NewRecorder()
	h.callback(badStateRec, badState)
	if badStateRec.Code != http.StatusBadRequest {
		t.Fatalf("mismatched state status=%d", badStateRec.Code)
	}
	callback := httptest.NewRequest(http.MethodGet, CallbackPath+"?code=synthetic-code&state="+url.QueryEscape(state), nil)
	callback.AddCookie(flow)
	out := httptest.NewRecorder()
	h.callback(out, callback)
	if out.Code != http.StatusOK || sessions.subject != "12345" || !store.bootstrapped {
		t.Fatalf("callback: status=%d sessions=%q bootstrapped=%t body=%s", out.Code, sessions.subject, store.bootstrapped, out.Body.String())
	}
	var body map[string]any
	if err := json.Unmarshal(out.Body.Bytes(), &body); err != nil || body["authenticated"] != true || body["csrf_token"] == "" {
		t.Fatalf("callback body: %s (%v)", out.Body, err)
	}
	if store.created != 0 {
		t.Fatal("login persisted unexpected token or code")
	}
	if store.tokenVerifier != verifier {
		t.Fatal("GitHub code exchange did not use the verifier bound to the encrypted flow")
	}
	wantCalls := []string{"POST github.com/login/oauth/access_token", "GET api.github.com/user"}
	if len(store.githubCalls) != len(wantCalls) {
		t.Fatalf("unexpected GitHub API calls: %v", store.githubCalls)
	}
	for i := range wantCalls {
		if store.githubCalls[i] != wantCalls[i] {
			t.Fatalf("GitHub API call %d=%q, want %q", i, store.githubCalls[i], wantCalls[i])
		}
	}
	replay := httptest.NewRecorder()
	h.callback(replay, callback)
	if replay.Code != http.StatusBadRequest {
		t.Fatalf("replay status=%d", replay.Code)
	}
}
func TestCallbackRejectsExpiredFlowAndDeniedConsent(t *testing.T) {
	t.Run("expired flow", func(t *testing.T) {
		store := newMemoryStore()
		h, sessions := testHandler(t, store, fakeAuth{})
		start := httptest.NewRecorder()
		h.start(start, httptest.NewRequest(http.MethodGet, StartPath, nil))
		cookie := start.Result().Cookies()[0]
		state, _, _, expires, err := h.openFlow(cookie.Value)
		if err != nil {
			t.Fatal(err)
		}
		h.now = func() time.Time { return expires }
		callback := httptest.NewRequest(http.MethodGet, CallbackPath+"?code=synthetic-code&state="+url.QueryEscape(state), nil)
		callback.AddCookie(cookie)
		got := httptest.NewRecorder()
		h.callback(got, callback)
		if got.Code != http.StatusBadRequest || sessions.subject != "" || len(store.githubCalls) != 0 || len(store.used) != 0 {
			t.Fatalf("expired flow was accepted or consumed: status=%d subject=%q github=%v used=%v", got.Code, sessions.subject, store.githubCalls, store.used)
		}
	})
	t.Run("denied consent", func(t *testing.T) {
		store := newMemoryStore()
		h, sessions := testHandler(t, store, fakeAuth{})
		start := httptest.NewRecorder()
		h.start(start, httptest.NewRequest(http.MethodGet, StartPath, nil))
		cookie := start.Result().Cookies()[0]
		state, _, _, _, err := h.openFlow(cookie.Value)
		if err != nil {
			t.Fatal(err)
		}
		callback := httptest.NewRequest(http.MethodGet, CallbackPath+"?error=access_denied&state="+url.QueryEscape(state), nil)
		callback.AddCookie(cookie)
		got := httptest.NewRecorder()
		h.callback(got, callback)
		if got.Code != http.StatusBadRequest || sessions.subject != "" || len(store.githubCalls) != 0 || len(store.used) != 0 {
			t.Fatalf("denied consent continued login: status=%d subject=%q github=%v used=%v", got.Code, sessions.subject, store.githubCalls, store.used)
		}
	})
}

func TestStartCannotUseExternalReturnPath(t *testing.T) {
	h, _ := testHandler(t, newMemoryStore(), fakeAuth{})
	request := httptest.NewRequest(http.MethodGet, StartPath+"?return_to=https%3A%2F%2Fattacker.example%2F", nil)
	got := httptest.NewRecorder()
	h.start(got, request)
	if got.Code != http.StatusFound {
		t.Fatalf("start status=%d", got.Code)
	}
	location, err := url.Parse(got.Header().Get("Location"))
	if err != nil {
		t.Fatal(err)
	}
	if location.Host != "github.com" || location.Query().Get("redirect_uri") != h.cfg.CallbackURL || strings.Contains(location.String(), "attacker.example") {
		t.Fatalf("caller-controlled external return path affected login redirect: %s", location)
	}
}

func TestNewRejectsMissingEnrollmentConfiguration(t *testing.T) {
	h, _ := testHandler(t, newMemoryStore(), fakeAuth{})
	cfg := h.cfg
	cfg.ClientSecret = ""
	if _, err := New(cfg, newMemoryStore(), &fakeSessions{}, fakeAuth{}, nil); err == nil {
		t.Fatal("missing GitHub App client secret accepted")
	}
	cfg = h.cfg
	cfg.CallbackURL = "https://attacker.example/auth/github/callback"
	if _, err := New(cfg, newMemoryStore(), &fakeSessions{}, fakeAuth{}, nil); err == nil {
		t.Fatal("callback on a different origin accepted")
	}
}

func TestInvitationBoundToNumericAccountAndOneUse(t *testing.T) {
	store := newMemoryStore()
	store.githubID = "67890"
	h, _ := testHandler(t, store, fakeAuth{})
	raw, _ := randomToken(32)
	store.invites[key(hashToken(raw))] = inviteRecord{account: "99999", role: "reader", scopes: []string{"catalog:read"}, expires: h.now().Add(time.Hour)}
	leaked := httptest.NewRecorder()
	h.start(leaked, httptest.NewRequest(http.MethodGet, StartPath+"?invite="+url.QueryEscape(raw), nil))
	if leaked.Code != http.StatusBadRequest {
		t.Fatalf("invitation query accepted: %d", leaked.Code)
	}
	req := httptest.NewRequest(http.MethodPost, StartPath, strings.NewReader(url.Values{"invite": []string{raw}}.Encode()))
	req.Header.Set("Origin", h.cfg.PublicOrigin)
	req.Header.Set("Content-Type", "application/x-www-form-urlencoded")
	rec := httptest.NewRecorder()
	h.start(rec, req)
	if rec.Code != http.StatusFound {
		t.Fatalf("invite start status=%d body=%s", rec.Code, rec.Body)
	}
	flow := rec.Result().Cookies()[0]
	state, _, saved, _, err := h.openFlow(flow.Value)
	if err != nil || saved != raw {
		t.Fatalf("invite not carried in encrypted cookie: saved=%q err=%v", saved, err)
	}
	bad := httptest.NewRequest(http.MethodGet, CallbackPath+"?code=synthetic-code&state="+url.QueryEscape(state), nil)
	bad.AddCookie(flow)
	badRecorder := httptest.NewRecorder()
	h.callback(badRecorder, bad)
	if badRecorder.Code != http.StatusForbidden {
		t.Fatalf("mismatched numeric account accepted: %d", badRecorder.Code)
	}
}
func TestInvitationCreationRequiresSameOriginCSRFAndOperatorScope(t *testing.T) {
	store := newMemoryStore()
	auth := fakeAuth{session: Session{ID: "sid", Subject: "12345", Workspaces: []string{"private"}}}
	h, _ := testHandler(t, store, auth)
	store.members["private:12345"] = "maintainer"
	bodyText := "{\"github_account_id\":\"67890\",\"role\":\"reader\",\"scopes\":[\"catalog:read\"]}"
	body := strings.NewReader(bodyText)
	req := httptest.NewRequest(http.MethodPost, InvitationsPath, body)
	req.Header.Set("Origin", h.cfg.PublicOrigin)
	req.Header.Set("X-Gist-CSRF", h.csrf("12345"))
	out := httptest.NewRecorder()
	h.createInvitation(out, req)
	if out.Code != http.StatusCreated || store.created != 1 {
		t.Fatalf("valid invitation: status=%d body=%s", out.Code, out.Body)
	}
	var got map[string]any
	if err := json.Unmarshal(out.Body.Bytes(), &got); err != nil || got["invitation"] == "" {
		t.Fatalf("invitation response: %s", out.Body)
	}
	widened := httptest.NewRequest(http.MethodPost, InvitationsPath, strings.NewReader("{\"github_account_id\":\"67890\",\"role\":\"maintainer\",\"scopes\":[\"catalog:admin\"]}"))
	widened.Header.Set("Origin", h.cfg.PublicOrigin)
	widened.Header.Set("X-Gist-CSRF", h.csrf("12345"))
	denied := httptest.NewRecorder()
	h.createInvitation(denied, widened)
	if denied.Code != http.StatusBadRequest || store.created != 1 {
		t.Fatalf("widened scope status=%d", denied.Code)
	}
	cross := httptest.NewRequest(http.MethodPost, InvitationsPath, strings.NewReader("{\"github_account_id\":\"67890\",\"role\":\"reader\",\"scopes\":[\"catalog:read\"]}"))
	cross.Header.Set("Origin", "https://attacker.example")
	cross.Header.Set("X-Gist-CSRF", h.csrf("12345"))
	crossOut := httptest.NewRecorder()
	h.createInvitation(crossOut, cross)
	if crossOut.Code != http.StatusForbidden {
		t.Fatalf("cross-origin status=%d", crossOut.Code)
	}
	unauthorized, _ := testHandler(t, store, fakeAuth{})
	unauthReq := httptest.NewRequest(http.MethodPost, InvitationsPath, strings.NewReader(bodyText))
	unauthReq.Header.Set("Origin", h.cfg.PublicOrigin)
	unauthReq.Header.Set("X-Gist-CSRF", unauthorized.csrf("12345"))
	unauthOut := httptest.NewRecorder()
	unauthorized.createInvitation(unauthOut, unauthReq)
	if unauthOut.Code != http.StatusUnauthorized || store.created != 1 {
		t.Fatalf("unauthenticated invitation status=%d", unauthOut.Code)
	}
	crossWorkspaceAuth := fakeAuth{session: Session{ID: "sid", Subject: "12345", Workspaces: []string{"another-workspace"}}}
	crossWorkspaceHandler, _ := testHandler(t, store, crossWorkspaceAuth)
	crossWorkspaceReq := httptest.NewRequest(http.MethodPost, InvitationsPath, strings.NewReader(bodyText))
	crossWorkspaceReq.Header.Set("Origin", h.cfg.PublicOrigin)
	crossWorkspaceReq.Header.Set("X-Gist-CSRF", crossWorkspaceHandler.csrf("12345"))
	crossWorkspaceOut := httptest.NewRecorder()
	crossWorkspaceHandler.createInvitation(crossWorkspaceOut, crossWorkspaceReq)
	if crossWorkspaceOut.Code != http.StatusUnauthorized || store.created != 1 {
		t.Fatalf("cross-workspace invitation status=%d", crossWorkspaceOut.Code)
	}
	readerAuth := fakeAuth{session: Session{ID: "reader-session", Subject: "reader", Workspaces: []string{"private"}}}
	readerHandler, _ := testHandler(t, store, readerAuth)
	store.members["private:reader"] = "reader"
	readerReq := httptest.NewRequest(http.MethodPost, InvitationsPath, strings.NewReader(bodyText))
	readerReq.Header.Set("Origin", h.cfg.PublicOrigin)
	readerReq.Header.Set("X-Gist-CSRF", readerHandler.csrf("reader"))
	readerOut := httptest.NewRecorder()
	readerHandler.createInvitation(readerOut, readerReq)
	if readerOut.Code != http.StatusForbidden || store.created != 1 {
		t.Fatalf("reader invitation status=%d", readerOut.Code)
	}
	badCSRF := httptest.NewRequest(http.MethodPost, InvitationsPath, strings.NewReader(bodyText))
	badCSRF.Header.Set("Origin", h.cfg.PublicOrigin)
	badCSRF.Header.Set("X-Gist-CSRF", "invalid")
	badCSRFOut := httptest.NewRecorder()
	h.createInvitation(badCSRFOut, badCSRF)
	if badCSRFOut.Code != http.StatusForbidden || store.created != 1 {
		t.Fatalf("invalid CSRF invitation status=%d", badCSRFOut.Code)
	}
}
func TestCanonicalAccountID(t *testing.T) {
	for _, v := range []string{"", "0", "01", "-1", "9223372036854775808"} {
		if canonicalID(v) {
			t.Errorf("accepted invalid ID %q", v)
		}
	}
	if !canonicalID(strconv.FormatInt(12345, 10)) {
		t.Fatal("rejected valid ID")
	}
}

func TestInvitedAccountReceivesOnlyInvitedWorkspace(t *testing.T) {
	store := newMemoryStore()
	store.githubID = "67890"
	h, sessions := testHandler(t, store, fakeAuth{})
	raw, _ := randomToken(32)
	store.invites[key(hashToken(raw))] = inviteRecord{account: "67890", role: "reader", scopes: []string{"catalog:read"}, expires: h.now().Add(time.Hour)}
	start := httptest.NewRequest(http.MethodPost, StartPath, strings.NewReader(url.Values{"invite": []string{raw}}.Encode()))
	start.Header.Set("Origin", h.cfg.PublicOrigin)
	start.Header.Set("Content-Type", "application/x-www-form-urlencoded")
	startRec := httptest.NewRecorder()
	h.start(startRec, start)
	flow := startRec.Result().Cookies()[0]
	state, _, invitation, _, err := h.openFlow(flow.Value)
	if err != nil || invitation != raw {
		t.Fatalf("flow invitation=%q err=%v", invitation, err)
	}
	callback := httptest.NewRequest(http.MethodGet, CallbackPath+"?code=synthetic-code&state="+url.QueryEscape(state), nil)
	callback.AddCookie(flow)
	out := httptest.NewRecorder()
	h.callback(out, callback)
	if out.Code != http.StatusOK || sessions.subject != "67890" || store.members["private:67890"] != "reader" {
		t.Fatalf("invited callback status=%d subject=%q membership=%q body=%s", out.Code, sessions.subject, store.members["private:67890"], out.Body)
	}
	if len(store.members) != 1 {
		t.Fatalf("invitation created unintended membership rows: %#v", store.members)
	}
}

func TestRoutesServeLoginOverHTTP(t *testing.T) {
	store := newMemoryStore()
	h, sessions := testHandler(t, store, fakeAuth{})
	mux := http.NewServeMux()
	h.Routes(mux)
	server := httptest.NewServer(mux)
	defer server.Close()
	client := &http.Client{CheckRedirect: func(*http.Request, []*http.Request) error {
		return http.ErrUseLastResponse
	}}

	startResponse, err := client.Get(server.URL + StartPath)
	if err != nil {
		t.Fatal(err)
	}
	defer startResponse.Body.Close()
	body, err := io.ReadAll(startResponse.Body)
	if err != nil {
		t.Fatal(err)
	}
	if startResponse.StatusCode != http.StatusFound {
		t.Fatalf("start status=%d body=%s", startResponse.StatusCode, body)
	}
	var flowCookie *http.Cookie
	for _, cookie := range startResponse.Cookies() {
		if cookie.Name == FlowCookieName {
			flowCookie = cookie
		}
	}
	if flowCookie == nil || !flowCookie.Secure || !flowCookie.HttpOnly {
		t.Fatalf("missing protected flow cookie: %#v", flowCookie)
	}
	location, err := url.Parse(startResponse.Header.Get("Location"))
	if err != nil {
		t.Fatal(err)
	}
	state := location.Query().Get("state")
	if state == "" {
		t.Fatal("authorization redirect omitted state")
	}

	callbackRequest, err := http.NewRequest(http.MethodGet, server.URL+CallbackPath+"?code=synthetic-code&state="+url.QueryEscape(state), nil)
	if err != nil {
		t.Fatal(err)
	}
	callbackRequest.AddCookie(flowCookie)
	callbackResponse, err := client.Do(callbackRequest)
	if err != nil {
		t.Fatal(err)
	}
	defer callbackResponse.Body.Close()
	body, err = io.ReadAll(callbackResponse.Body)
	if err != nil {
		t.Fatal(err)
	}
	var callbackBody map[string]any
	if err := json.Unmarshal(body, &callbackBody); err != nil {
		t.Fatalf("callback returned invalid JSON %q: %v", body, err)
	}
	if callbackResponse.StatusCode != http.StatusOK || sessions.subject != "12345" || callbackBody["authenticated"] != true {
		t.Fatalf("callback status=%d subject=%q body=%s", callbackResponse.StatusCode, sessions.subject, body)
	}
}

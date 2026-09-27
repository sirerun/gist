//go:build integration

package wiring

import (
	"context"
	"crypto/rand"
	"crypto/sha256"
	"encoding/base64"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"net/http"
	"net/url"
	"regexp"
	"strings"
	"sync"
	"testing"
	"time"

	"github.com/sirerun/gist/hosted/internal/oauth"
	"github.com/sirerun/gist/hosted/internal/storage"
)

// The reference authorization server runs inside the composed app against
// real PostgreSQL. Its access tokens pass the same verifier and stored
// identity check as every other token, and its grants live in RLS tables.

type oauthClient struct {
	f      *fixture
	http   *http.Client
	id     string
	cookie *http.Cookie
}

const oauthRedirect = "https://connector.example/callback"

var (
	oauthRequestField = regexp.MustCompile(`name="request" value="([^"]+)"`)
	oauthCSRFField    = regexp.MustCompile(`name="csrf" value="([^"]+)"`)
)

func newOAuthClient(t *testing.T, f *fixture, subject string, workspaces ...string) *oauthClient {
	t.Helper()
	c := &oauthClient{f: f, http: &http.Client{Transport: f.server.Client().Transport, CheckRedirect: func(*http.Request, []*http.Request) error { return http.ErrUseLastResponse }}}
	resp := c.send(t, http.MethodPost, "/oauth/register", "application/json", `{"redirect_uris":["`+oauthRedirect+`"],"client_name":"Integration connector","scope":"catalog:read catalog:publish"}`)
	var reg map[string]any
	c.decode(t, resp, http.StatusCreated, &reg)
	c.id, _ = reg["client_id"].(string)
	cookie, err := f.app.IssueOAuthSession(subject, workspaces)
	if err != nil {
		t.Fatal(err)
	}
	c.cookie = cookie
	return c
}

func (c *oauthClient) send(t *testing.T, method, path, contentType, body string) *http.Response {
	t.Helper()
	req, err := http.NewRequest(method, c.f.baseURL+path, strings.NewReader(body))
	if err != nil {
		t.Fatal(err)
	}
	if contentType != "" {
		req.Header.Set("Content-Type", contentType)
	}
	if c.cookie != nil {
		req.AddCookie(c.cookie)
	}
	resp, err := c.http.Do(req)
	if err != nil {
		t.Fatal(err)
	}
	return resp
}

func (c *oauthClient) decode(t *testing.T, resp *http.Response, status int, v any) {
	t.Helper()
	defer resp.Body.Close()
	body, _ := io.ReadAll(resp.Body)
	if resp.StatusCode != status {
		t.Fatalf("%s %s: status %d, want %d: %s", resp.Request.Method, resp.Request.URL.Path, resp.StatusCode, status, body)
	}
	if v != nil {
		if err := json.Unmarshal(body, v); err != nil {
			t.Fatalf("decode %s: %v", body, err)
		}
	}
}

// code runs authorize and consent for one workspace, returning the code and
// its PKCE verifier.
func (c *oauthClient) code(t *testing.T, workspace string) (string, string) {
	t.Helper()
	b := make([]byte, 32)
	_, _ = rand.Read(b)
	verifier := base64.RawURLEncoding.EncodeToString(b)
	sum := sha256.Sum256([]byte(verifier))
	q := url.Values{"response_type": {"code"}, "client_id": {c.id}, "redirect_uri": {oauthRedirect}, "code_challenge": {base64.RawURLEncoding.EncodeToString(sum[:])}, "code_challenge_method": {"S256"}, "scope": {"catalog:read catalog:publish"}, "resource": {c.f.audience}, "state": {"st"}}
	resp := c.send(t, http.MethodGet, "/oauth/authorize?"+q.Encode(), "", "")
	body, _ := io.ReadAll(resp.Body)
	resp.Body.Close()
	request, csrf := oauthRequestField.FindSubmatch(body), oauthCSRFField.FindSubmatch(body)
	if resp.StatusCode != http.StatusOK || request == nil || csrf == nil {
		t.Fatalf("authorize: %d %s", resp.StatusCode, body)
	}
	form := url.Values{"request": {string(request[1])}, "csrf": {string(csrf[1])}, "workspace": {workspace}, "decision": {"allow"}}
	resp = c.send(t, http.MethodPost, "/oauth/consent", "application/x-www-form-urlencoded", form.Encode())
	resp.Body.Close()
	loc, err := url.Parse(resp.Header.Get("Location"))
	if resp.StatusCode != http.StatusFound || err != nil || loc.Query().Get("code") == "" {
		t.Fatalf("consent: %d %q", resp.StatusCode, resp.Header.Get("Location"))
	}
	return loc.Query().Get("code"), verifier
}

func (c *oauthClient) token(t *testing.T, form url.Values) (int, map[string]any) {
	t.Helper()
	form.Set("client_id", c.id)
	resp := c.send(t, http.MethodPost, "/oauth/token", "application/x-www-form-urlencoded", form.Encode())
	defer resp.Body.Close()
	var m map[string]any
	body, _ := io.ReadAll(resp.Body)
	_ = json.Unmarshal(body, &m)
	return resp.StatusCode, m
}

func (c *oauthClient) exchange(t *testing.T, code, verifier string) (int, map[string]any) {
	return c.token(t, url.Values{"grant_type": {"authorization_code"}, "code": {code}, "redirect_uri": {oauthRedirect}, "code_verifier": {verifier}})
}

func (c *oauthClient) refresh(t *testing.T, refresh string) (int, map[string]any) {
	return c.token(t, url.Values{"grant_type": {"refresh_token"}, "refresh_token": {refresh}})
}

func TestOAuthMetadataAndChallengeIntegration(t *testing.T) {
	f := requireFixture(t)
	c := &oauthClient{f: f, http: f.server.Client()}
	var prm map[string]any
	c.decode(t, c.send(t, http.MethodGet, "/.well-known/oauth-protected-resource", "", ""), http.StatusOK, &prm)
	if prm["resource"] != f.audience || !strings.Contains(strings.Join(anyStrings(prm["authorization_servers"]), ","), f.baseURL) {
		t.Fatalf("prm = %v", prm)
	}
	var as map[string]any
	c.decode(t, c.send(t, http.MethodGet, "/.well-known/oauth-authorization-server", "", ""), http.StatusOK, &as)
	if as["issuer"] != f.baseURL || as["token_endpoint"] != f.baseURL+"/oauth/token" || strings.Join(anyStrings(as["code_challenge_methods_supported"]), ",") != "S256" {
		t.Fatalf("as metadata = %v", as)
	}
	want := `resource_metadata="` + f.baseURL + `/.well-known/oauth-protected-resource"`
	for _, probe := range []struct{ method, path, body string }{{"POST", "/mcp", `{"jsonrpc":"2.0","id":1,"method":"initialize"}`}, {"POST", "/v1/discover", `{"query":"x","max_bytes":10}`}} {
		resp := f.do(t, probe.method, probe.path, "", probe.body)
		resp.Body.Close()
		if challenge := resp.Header.Get("WWW-Authenticate"); resp.StatusCode != http.StatusUnauthorized || !strings.Contains(challenge, want) {
			t.Fatalf("%s %s: %d challenge %q", probe.method, probe.path, resp.StatusCode, challenge)
		}
	}
}

func TestOAuthGrantLifecycleIntegration(t *testing.T) {
	f := requireFixture(t)
	subject := f.memberSubject(t, "oauth", "q3-tenant-a", "q3-tenant-b")
	c := newOAuthClient(t, f, subject, "q3-tenant-a", "q3-tenant-b")
	code, verifier := c.code(t, "q3-tenant-a")
	status, m := c.exchange(t, code, verifier)
	if status != http.StatusOK {
		t.Fatalf("exchange: %d %v", status, m)
	}
	access, refresh := m["access_token"].(string), m["refresh_token"].(string)
	// The OAuth token passes the composed app's verifier and stored-identity
	// check with no manual seeding.
	if got := f.discoverStatus(t, access); got != http.StatusOK {
		t.Fatalf("oauth access token at /v1/discover: %d", got)
	}
	if count, revoked := f.storedIdentity(t, "q3-tenant-a", subject); count != 1 || revoked {
		t.Fatalf("stored identity count=%d revoked=%v", count, revoked)
	}
	// Only hashes are stored, under the grant's workspace.
	sum := sha256.Sum256([]byte(code))
	for workspace, want := range map[string]int{"q3-tenant-a": 1, "q3-tenant-b": 0} {
		if got := rlsCodeCount(t, f, workspace, sum[:]); got != want {
			t.Fatalf("hashed consumed code rows under %s RLS = %d, want %d", workspace, got, want)
		}
	}
	// Code replay fails and revokes the grant made from it.
	if status, m := c.exchange(t, code, verifier); status != http.StatusBadRequest || m["error"] != "invalid_grant" {
		t.Fatalf("code replay: %d %v", status, m)
	}
	if status, m := c.refresh(t, refresh); status != http.StatusBadRequest || m["error"] != "invalid_grant" {
		t.Fatalf("refresh after code replay: %d %v", status, m)
	}

	// A fresh grant: rotation, then reuse containment.
	code, verifier = c.code(t, "q3-tenant-a")
	_, m = c.exchange(t, code, verifier)
	r0 := m["refresh_token"].(string)
	status, m = c.refresh(t, r0)
	if status != http.StatusOK || m["refresh_token"] == r0 {
		t.Fatalf("rotate: %d %v", status, m)
	}
	r1 := m["refresh_token"].(string)
	if got := f.discoverStatus(t, m["access_token"].(string)); got != http.StatusOK {
		t.Fatalf("refreshed access token: %d", got)
	}
	if status, m := c.refresh(t, r0); status != http.StatusBadRequest || m["error"] != "invalid_grant" {
		t.Fatalf("reuse: %d %v", status, m)
	}
	if status, m := c.refresh(t, r1); status != http.StatusBadRequest || m["error"] != "invalid_grant" {
		t.Fatalf("family survived reuse: %d %v", status, m)
	}

	// Workspace reissue: a second consent for tenant B yields a B token.
	code, verifier = c.code(t, "q3-tenant-b")
	status, m = c.exchange(t, code, verifier)
	if status != http.StatusOK {
		t.Fatalf("tenant-b exchange: %d %v", status, m)
	}
	if count, _ := f.storedIdentity(t, "q3-tenant-b", subject); count != 1 {
		t.Fatalf("tenant-b identity rows = %d", count)
	}
	accessB, refreshB := m["access_token"].(string), m["refresh_token"].(string)
	if got := f.discoverStatus(t, accessB); got != http.StatusOK {
		t.Fatalf("tenant-b token: %d", got)
	}

	// Revoking the stored identity ends access and refresh.
	if err := mustPostgres(t, f).Revoke(context.Background(), "q3-tenant-b", f.baseURL, subject); err != nil {
		t.Fatal(err)
	}
	if got := f.discoverStatus(t, accessB); got != http.StatusUnauthorized {
		t.Fatalf("revoked identity token: %d", got)
	}
	if status, m := c.refresh(t, refreshB); status != http.StatusBadRequest || m["error"] != "invalid_grant" {
		t.Fatalf("refresh after identity revoke: %d %v", status, m)
	}
}

func TestOAuthConsentRejectsNonMemberIntegration(t *testing.T) {
	f := requireFixture(t)
	// The session offers tenant B, but the subject is only a member of A.
	subject := f.memberSubject(t, "oauth-nm", "q3-tenant-a")
	c := newOAuthClient(t, f, subject, "q3-tenant-b")
	b := make([]byte, 32)
	_, _ = rand.Read(b)
	sum := sha256.Sum256(b)
	q := url.Values{"response_type": {"code"}, "client_id": {c.id}, "redirect_uri": {oauthRedirect}, "code_challenge": {base64.RawURLEncoding.EncodeToString(sum[:])}, "code_challenge_method": {"S256"}, "scope": {"catalog:read"}, "resource": {f.audience}}
	resp := c.send(t, http.MethodGet, "/oauth/authorize?"+q.Encode(), "", "")
	body, _ := io.ReadAll(resp.Body)
	resp.Body.Close()
	request, csrf := oauthRequestField.FindSubmatch(body), oauthCSRFField.FindSubmatch(body)
	if request == nil || csrf == nil {
		t.Fatalf("authorize: %d %s", resp.StatusCode, body)
	}
	form := url.Values{"request": {string(request[1])}, "csrf": {string(csrf[1])}, "workspace": {"q3-tenant-b"}, "decision": {"allow"}}
	resp = c.send(t, http.MethodPost, "/oauth/consent", "application/x-www-form-urlencoded", form.Encode())
	resp.Body.Close()
	loc, _ := url.Parse(resp.Header.Get("Location"))
	if resp.StatusCode != http.StatusFound || loc.Query().Get("error") != "access_denied" || loc.Query().Get("code") != "" {
		t.Fatalf("non-member consent: %d %q", resp.StatusCode, resp.Header.Get("Location"))
	}
}

// storeCode registers a throwaway client and saves one code for it directly
// through the Postgres store.
func storeCode(t *testing.T, store *storage.OAuthStore, workspace string) []byte {
	t.Helper()
	ctx := context.Background()
	b := make([]byte, 16)
	_, _ = rand.Read(b)
	id := fmt.Sprintf("gc_it_%x", b)
	now := time.Now().UTC()
	if err := store.CreateClient(ctx, oauth.Client{ID: id, Name: "it", RedirectURIs: []string{oauthRedirect}, Scopes: []string{"catalog:read"}, Audience: "https://registry.example.invalid", CreatedAt: now}); err != nil {
		t.Fatal(err)
	}
	_, _ = rand.Read(b)
	sum := sha256.Sum256(b)
	if err := store.SaveCode(ctx, oauth.AuthCode{Hash: sum[:], ClientID: id, Subject: "it-subject", WorkspaceID: workspace, RedirectURI: oauthRedirect, Resource: "https://registry.example.invalid", Scopes: []string{"catalog:read"}, Challenge: "x", IssuedAt: now, ExpiresAt: now.Add(time.Minute)}); err != nil {
		t.Fatal(err)
	}
	return sum[:]
}

func refreshToken() oauth.RefreshToken {
	b := make([]byte, 32)
	_, _ = rand.Read(b)
	sum := sha256.Sum256(b)
	now := time.Now().UTC()
	return oauth.RefreshToken{Hash: sum[:], IssuedAt: now, ExpiresAt: now.Add(time.Hour)}
}

func accept(oauth.AuthCode) error { return nil }

// familyRevoked probes a family through its first refresh token.
func familyRevoked(t *testing.T, store *storage.OAuthStore, workspace string, first oauth.RefreshToken) bool {
	t.Helper()
	_, err := store.RotateRefresh(context.Background(), workspace, first.Hash, func(oauth.RefreshFamily, oauth.RefreshToken) error { return errors.New("probe only") }, refreshToken())
	switch {
	case errors.Is(err, oauth.ErrFamilyRevoked):
		return true
	case err != nil && err.Error() == "probe only":
		return false
	default:
		t.Fatalf("probe family: %v", err)
		return false
	}
}

// The family is created in the transaction that consumes the code, so a
// replay blocked on the code's row lock during the first redemption revokes
// the family once the first redemption commits.
func TestOAuthCodeReplayRevokesFamilyUnderLockIntegration(t *testing.T) {
	f := requireFixture(t)
	store := mustPostgres(t, f).OAuth()
	ctx := context.Background()
	hash := storeCode(t, store, "q3-tenant-a")
	first := refreshToken()
	inCheck, release := make(chan struct{}), make(chan struct{})
	winner := make(chan error, 1)
	go func() {
		_, _, err := store.RedeemCode(ctx, "q3-tenant-a", hash, func(oauth.AuthCode) error {
			close(inCheck)
			<-release
			return nil
		}, first)
		winner <- err
	}()
	<-inCheck
	replay := make(chan error, 1)
	go func() {
		_, _, err := store.RedeemCode(ctx, "q3-tenant-a", hash, accept, refreshToken())
		replay <- err
	}()
	// Wait until the replay is actually queued on the code's row lock.
	deadline := time.Now().Add(10 * time.Second)
	for {
		var waiting int
		if err := f.pool.QueryRow(ctx, `SELECT count(*) FROM pg_stat_activity WHERE wait_event_type='Lock' AND query LIKE '%oauth_authorization_codes%FOR UPDATE%'`).Scan(&waiting); err != nil {
			t.Fatal(err)
		}
		if waiting > 0 {
			break
		}
		if time.Now().After(deadline) {
			t.Fatal("replay never blocked on the code row lock")
		}
		time.Sleep(10 * time.Millisecond)
	}
	close(release)
	if err := <-winner; err != nil {
		t.Fatalf("first redemption: %v", err)
	}
	if err := <-replay; !errors.Is(err, oauth.ErrGrantReused) {
		t.Fatalf("replay: %v, want ErrGrantReused", err)
	}
	if !familyRevoked(t, store, "q3-tenant-a", first) {
		t.Fatal("family issued to the first redeemer survived the replay")
	}
}

// Concurrent redemptions: exactly one wins, and its family is revoked by
// the others whatever the scheduling.
func TestOAuthConcurrentCodeRedemptionIntegration(t *testing.T) {
	f := requireFixture(t)
	store := mustPostgres(t, f).OAuth()
	hash := storeCode(t, store, "q3-tenant-a")
	const n = 8
	firsts := make([]oauth.RefreshToken, n)
	errs := make([]error, n)
	var wg sync.WaitGroup
	for i := range firsts {
		firsts[i] = refreshToken()
		wg.Add(1)
		go func(i int) {
			defer wg.Done()
			_, _, errs[i] = store.RedeemCode(context.Background(), "q3-tenant-a", hash, accept, firsts[i])
		}(i)
	}
	wg.Wait()
	won := -1
	for i, err := range errs {
		switch {
		case err == nil && won < 0:
			won = i
		case err == nil:
			t.Fatalf("two redemptions succeeded (%d and %d)", won, i)
		case !errors.Is(err, oauth.ErrGrantReused):
			t.Fatalf("redemption %d: %v", i, err)
		}
	}
	if won < 0 {
		t.Fatal("no redemption succeeded")
	}
	if !familyRevoked(t, store, "q3-tenant-a", firsts[won]) {
		t.Fatal("winning family survived concurrent replays")
	}
}

// A failed check commits the consumption: the correct presentation after a
// wrong one is a replay, and no family exists.
func TestOAuthFailedRedemptionInvalidatesCodeIntegration(t *testing.T) {
	f := requireFixture(t)
	store := mustPostgres(t, f).OAuth()
	ctx := context.Background()
	hash := storeCode(t, store, "q3-tenant-a")
	wrong := errors.New("wrong verifier")
	if _, _, err := store.RedeemCode(ctx, "q3-tenant-a", hash, func(oauth.AuthCode) error { return wrong }, refreshToken()); !errors.Is(err, wrong) {
		t.Fatalf("failed redemption: %v", err)
	}
	if _, _, err := store.RedeemCode(ctx, "q3-tenant-a", hash, accept, refreshToken()); !errors.Is(err, oauth.ErrGrantReused) {
		t.Fatalf("redemption after failed attempt: %v, want ErrGrantReused", err)
	}
	var families int
	if err := f.pool.QueryRow(ctx, `SELECT count(*) FROM oauth_refresh_families WHERE code_hash=$1`, hash).Scan(&families); err != nil {
		t.Fatal(err)
	}
	if families != 0 {
		t.Fatalf("families from an invalidated code = %d", families)
	}
}

// rlsCodeCount counts consumed rows for a code hash as a non-superuser role,
// so forced row-level security applies (the fixture's connection role may
// bypass RLS). The throwaway role is dropped afterwards.
func rlsCodeCount(t *testing.T, f *fixture, workspace string, hash []byte) int {
	t.Helper()
	ctx := context.Background()
	role := fmt.Sprintf("gist_oauth_rls_%d", time.Now().UnixNano())
	if _, err := f.pool.Exec(ctx, `CREATE ROLE `+quoteIdent(role)+` NOLOGIN NOBYPASSRLS; GRANT SELECT ON oauth_authorization_codes TO `+quoteIdent(role)); err != nil {
		t.Fatalf("create rls probe role: %v", err)
	}
	defer func() {
		_, _ = f.pool.Exec(ctx, `REVOKE ALL ON oauth_authorization_codes FROM `+quoteIdent(role)+`; DROP ROLE `+quoteIdent(role))
	}()
	tx, err := f.pool.Begin(ctx)
	if err != nil {
		t.Fatal(err)
	}
	defer func() { _ = tx.Rollback(ctx) }()
	var count int
	if _, err := tx.Exec(ctx, `SELECT set_config('registry.workspace_id', $1, true)`, workspace); err != nil {
		t.Fatal(err)
	}
	if _, err := tx.Exec(ctx, `SET LOCAL ROLE `+quoteIdent(role)); err != nil {
		t.Fatal(err)
	}
	if err := tx.QueryRow(ctx, `SELECT count(*) FROM oauth_authorization_codes WHERE code_hash=$1 AND consumed_at IS NOT NULL`, hash).Scan(&count); err != nil {
		t.Fatalf("count codes as rls role: %v", err)
	}
	return count
}

func mustPostgres(t *testing.T, f *fixture) *storage.Postgres {
	t.Helper()
	s, err := storage.NewPostgres(f.pool)
	if err != nil {
		t.Fatal(err)
	}
	return s
}

func anyStrings(v any) []string {
	items, _ := v.([]any)
	out := make([]string, 0, len(items))
	for _, item := range items {
		if s, ok := item.(string); ok {
			out = append(out, s)
		}
	}
	return out
}

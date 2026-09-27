//go:build browser

// Browser tests for the consent UI, run with the agent-browser CLI against
// the real reference AS at an isolated local TLS origin:
//
//	(cd hosted && GOWORK=off go test ./internal/oauth -tags=browser -run TestBrowser -count=1 -v)
//
// They are behind the "browser" build tag so the default and integration
// suites never need a browser. With the tag set they fail, rather than skip,
// when agent-browser is missing, so a green run always means a browser ran.

package oauth_test

import (
	"fmt"
	"html"
	"net/http"
	"net/http/httptest"
	"net/url"
	"os"
	"os/exec"
	"strings"
	"testing"
	"time"

	"github.com/sirerun/gist/hosted/internal/oauth"
)

// browser drives one isolated agent-browser session (its own cookie jar).
type browser struct {
	t       *testing.T
	bin     string
	session string
}

func newBrowser(t *testing.T, user string) *browser {
	t.Helper()
	bin := os.Getenv("GIST_AGENT_BROWSER")
	if bin == "" {
		var err error
		if bin, err = exec.LookPath("agent-browser"); err != nil {
			t.Fatal("agent-browser is not on PATH; install it or set GIST_AGENT_BROWSER (the browser tag requires a browser)")
		}
	}
	b := &browser{t: t, bin: bin, session: fmt.Sprintf("gist-oauth-i6-%d-%s-%d", os.Getpid(), user, time.Now().UnixNano())}
	t.Cleanup(func() { _, _ = b.try("close") })
	return b
}

func (b *browser) try(args ...string) (string, error) {
	full := append([]string{"--session", b.session, "--ignore-https-errors"}, args...)
	out, err := exec.Command(b.bin, full...).CombinedOutput()
	return strings.TrimSpace(string(out)), err
}

func (b *browser) run(args ...string) string {
	b.t.Helper()
	out, err := b.try(args...)
	if err != nil {
		b.t.Fatalf("agent-browser %v: %v\n%s", args, err, out)
	}
	return out
}

func (b *browser) open(u string) { b.t.Helper(); b.run("open", u) }

func (b *browser) url() *url.URL {
	b.t.Helper()
	u, err := url.Parse(strings.Trim(b.run("get", "url"), `"`))
	if err != nil {
		b.t.Fatal(err)
	}
	return u
}

func (b *browser) text(sel string) string { b.t.Helper(); return b.run("get", "text", sel) }

func (b *browser) attr(sel, name string) string {
	b.t.Helper()
	return strings.Trim(b.run("get", "attr", sel, name), `"`)
}

// waitPath waits until the page is at path (on any origin).
func (b *browser) waitPath(path string) *url.URL {
	b.t.Helper()
	deadline := time.Now().Add(20 * time.Second)
	for {
		u := b.url()
		if u.Path == path {
			return u
		}
		if time.Now().After(deadline) {
			b.t.Fatalf("page stayed at %s, want %s; body: %s", u, path, b.text("body"))
		}
		time.Sleep(100 * time.Millisecond)
	}
}

// setValue changes a form control's value in the page, as a user with dev
// tools (or a hostile extension) could.
func (b *browser) setValue(sel, value string) {
	b.t.Helper()
	js := fmt.Sprintf(`(() => { const el = document.querySelector(%q); if (!el) throw new Error("missing %s"); el.value = %q; el.checked = true; return el.value; })()`, sel, sel, value)
	b.run("eval", js)
}

// ---- isolated origin --------------------------------------------------------

type browserEnv struct {
	*harness
	callback string
	attacker *httptest.Server
}

// newBrowserEnv starts the two-tenant AS plus test-only helper routes on its
// origin: a landing page, a client callback page, and a same-origin form
// replayer. A separate
// attacker origin serves a cross-site auto-submitting consent form.
func newBrowserEnv(t *testing.T) *browserEnv {
	t.Helper()
	env := &browserEnv{}
	routes := func(next http.Handler) http.Handler {
		return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			switch r.URL.Path {
			case "/test/home":
				fmt.Fprint(w, `<!doctype html><title>home</title><p id="who">signed in</p>`)
			case "/test/callback":
				q := r.URL.Query()
				w.Header().Set("Content-Type", "text/html; charset=utf-8")
				fmt.Fprintf(w, `<!doctype html><title>callback</title><pre id="code">%s</pre><pre id="error">%s</pre><pre id="state">%s</pre><pre id="iss">%s</pre>`,
					html.EscapeString(q.Get("code")), html.EscapeString(q.Get("error")), html.EscapeString(q.Get("state")), html.EscapeString(q.Get("iss")))
			case "/test/replay":
				w.Header().Set("Content-Type", "text/html; charset=utf-8")
				fmt.Fprint(w, autoSubmit(oauth.PathConsent, r.URL.Query()))
			default:
				next.ServeHTTP(w, r)
			}
		})
	}
	env.harness = newHarness(t, routes)
	env.policy.set("bob", bobWS, []string{"catalog:read", "catalog:publish"}, true)
	env.callback = env.origin + "/test/callback"
	env.attacker = httptest.NewTLSServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "text/html; charset=utf-8")
		fmt.Fprint(w, autoSubmit(env.origin+oauth.PathConsent, r.URL.Query()))
	}))
	t.Cleanup(env.attacker.Close)
	return env
}

func autoSubmit(action string, fields url.Values) string {
	var b strings.Builder
	fmt.Fprintf(&b, `<!doctype html><title>submit</title><form id="f" method="post" action="%s">`, html.EscapeString(action))
	for k, vs := range fields {
		for _, v := range vs {
			fmt.Fprintf(&b, `<input type="hidden" name="%s" value="%s">`, html.EscapeString(k), html.EscapeString(v))
		}
	}
	b.WriteString(`</form><script>document.getElementById("f").submit()</script>`)
	return b.String()
}

func (e *browserEnv) signIn(b *browser, user string) {
	e.t.Helper()
	b.open(e.origin + "/test/home")
	b.waitPath("/test/home")
	// The configured identity provider is out of scope here, so the test
	// issues the AS's own signed session and installs it in the browser.
	// (Chrome drops a Set-Cookie from an origin whose certificate it was
	// told to ignore, so the cookie is installed through the browser API.)
	c, err := e.sessions.Issue(user, map[string][]string{"alice": {"ws-a"}, "bob": {bobWS}}[user], time.Hour)
	if err != nil {
		e.t.Fatal(err)
	}
	b.run("cookies", "set", c.Name, c.Value, "--url", e.origin+"/", "--secure", "--httpOnly", "--sameSite", "Lax")
	if got := b.run("cookies", "get"); !strings.Contains(got, c.Name) {
		e.t.Fatalf("session cookie not stored: %s", got)
	}
}

func (e *browserEnv) authorizeURL(clientID, challenge, state string) string {
	q := url.Values{"response_type": {"code"}, "client_id": {clientID}, "redirect_uri": {e.callback}, "code_challenge": {challenge}, "code_challenge_method": {"S256"}, "state": {state}, "resource": {e.origin}, "scope": {"catalog:read"}}
	return e.origin + oauth.PathAuthorize + "?" + q.Encode()
}

// openConsent loads the consent page and checks what it shows the user.
func (e *browserEnv) openConsent(b *browser, clientID, challenge, state string, wantWorkspaces, hiddenWorkspaces []string) {
	e.t.Helper()
	b.open(e.authorizeURL(clientID, challenge, state))
	b.waitPath(oauth.PathAuthorize)
	body := b.text("body")
	for _, want := range append([]string{"Allow Browser Connector to access Gist?", "catalog:read", e.origin}, wantWorkspaces...) {
		if !strings.Contains(body, want) {
			e.t.Fatalf("consent page lacks %q:\n%s", want, body)
		}
	}
	for _, ws := range hiddenWorkspaces {
		if strings.Contains(body, ws) {
			e.t.Fatalf("consent page offers foreign workspace %q:\n%s", ws, body)
		}
	}
}

func (e *browserEnv) registerClient() string {
	e.t.Helper()
	status, m := e.register(map[string]any{"redirect_uris": []string{e.callback}, "client_name": "Browser Connector"})
	if status != http.StatusCreated {
		e.t.Fatalf("register: %d %v", status, m)
	}
	return m["client_id"].(string)
}

func (e *browserEnv) callbackResult(b *browser, state string) (code, errCode string) {
	e.t.Helper()
	u := b.waitPath("/test/callback")
	q := u.Query()
	if q.Get("state") != state || q.Get("iss") != e.origin {
		e.t.Fatalf("callback %s: state/iss mismatch", u)
	}
	return q.Get("code"), q.Get("error")
}

// ---- tests ----------------------------------------------------------------

// step runs one scenario inline. Browser sessions and the harness belong to
// the top-level test, so scenarios are logged steps rather than subtests.
func step(t *testing.T, name string, f func()) {
	t.Helper()
	t.Logf("step: %s", name)
	f()
	t.Logf("step passed: %s", name)
}

func TestBrowserConsentGoldenPathPerTenant(t *testing.T) {
	e := newBrowserEnv(t)
	clientID := e.registerClient()
	for _, tc := range []struct{ user, ws, foreign string }{{"alice", "ws-a", bobWS}, {"bob", bobWS, "ws-a"}} {
		step(t, tc.user, func() {
			b := newBrowser(t, tc.user)
			e.signIn(b, tc.user)
			verifier, challenge := pkce()
			e.openConsent(b, clientID, challenge, "gold-"+tc.user, []string{tc.ws}, []string{tc.foreign})
			b.run("click", "button[value=allow]")
			code, errCode := e.callbackResult(b, "gold-"+tc.user)
			if code == "" || errCode != "" {
				t.Fatalf("allow: code=%q error=%q", code, errCode)
			}
			if got := b.text("#code"); got != code {
				t.Fatalf("callback page shows %q", got)
			}
			status, m := e.exchange(clientID, code, e.callback, verifier)
			if status != http.StatusOK {
				t.Fatalf("exchange: %d %v", status, m)
			}
			st, p, _ := e.callResource("/resource-a", m["access_token"].(string))
			if st != http.StatusOK || p.Subject != tc.user || p.WorkspaceID != tc.ws {
				t.Fatalf("token principal = %d %+v, want %s in %s", st, p, tc.user, tc.ws)
			}
			// The browser-issued code is single-use.
			status, m = e.exchange(clientID, code, e.callback, verifier)
			wantError(t, status, m, 400, "invalid_grant")
		})
	}
}

func TestBrowserConsentDenialIssuesNothing(t *testing.T) {
	e := newBrowserEnv(t)
	clientID := e.registerClient()
	b := newBrowser(t, "alice")
	e.signIn(b, "alice")
	_, challenge := pkce()
	e.openConsent(b, clientID, challenge, "deny", []string{"ws-a"}, nil)
	before := e.store.codeCount()
	b.run("click", "button[value=deny]")
	code, errCode := e.callbackResult(b, "deny")
	if code != "" || errCode != "access_denied" {
		t.Fatalf("deny: code=%q error=%q", code, errCode)
	}
	if e.store.codeCount() != before {
		t.Fatal("denied consent stored a code")
	}
}

func TestBrowserConsentCSRF(t *testing.T) {
	e := newBrowserEnv(t)
	clientID := e.registerClient()
	alice := newBrowser(t, "alice")
	e.signIn(alice, "alice")
	_, challenge := pkce()

	step(t, "tampered CSRF token", func() {
		e.openConsent(alice, clientID, challenge, "csrf", []string{"ws-a"}, nil)
		alice.setValue(`input[name=csrf]`, "AAAA")
		alice.run("click", "button[value=allow]")
		alice.waitPath(oauth.PathConsent)
		if body := alice.text("body"); !strings.Contains(body, "failed its CSRF check") {
			t.Fatalf("tampered CSRF page: %s", body)
		}
	})

	step(t, "cross-site auto-submitted consent", func() {
		// Worst case: the attacker even knows a valid sealed request and
		// CSRF token for this session. The cross-site POST is still refused.
		e.openConsent(alice, clientID, challenge, "xsite", []string{"ws-a"}, nil)
		fields := url.Values{"request": {alice.attr(`input[name=request]`, "value")}, "csrf": {alice.attr(`input[name=csrf]`, "value")}, "workspace": {"ws-a"}, "decision": {"allow"}}
		// Use "localhost" so the attacker is cross-site, not just cross-origin.
		attacker := strings.Replace(e.attacker.URL, "127.0.0.1", "localhost", 1)
		alice.open(attacker + "/?" + fields.Encode())
		u := alice.waitPath(oauth.PathConsent)
		if u.Host != strings.TrimPrefix(e.origin, "https://") {
			t.Fatalf("forged post landed on %s", u)
		}
		if body := alice.text("body"); !strings.Contains(body, "Cross-site consent requests are refused") {
			t.Fatalf("cross-site consent page: %s", body)
		}
	})
	if n := e.store.codeCount(); n != 0 {
		t.Fatalf("CSRF attempts stored %d codes", n)
	}
}

func TestBrowserTenantsCannotConsentAcross(t *testing.T) {
	e := newBrowserEnv(t)
	clientID := e.registerClient()
	_, challenge := pkce()
	alice, bob := newBrowser(t, "alice"), newBrowser(t, "bob")
	e.signIn(alice, "alice")
	e.signIn(bob, "bob")

	for _, tc := range []struct {
		name    string
		b       *browser
		own     string
		foreign string
	}{{"alice into bob's workspace", alice, "ws-a", bobWS}, {"bob into alice's workspace", bob, bobWS, "ws-a"}} {
		step(t, tc.name, func() {
			state := "ws-" + tc.own
			e.openConsent(tc.b, clientID, challenge, state, []string{tc.own}, []string{tc.foreign})
			tc.b.setValue(`input[name=workspace]`, tc.foreign)
			tc.b.run("click", "button[value=allow]")
			code, errCode := e.callbackResult(tc.b, state)
			if code != "" || errCode != "access_denied" {
				t.Fatalf("forged workspace: code=%q error=%q", code, errCode)
			}
		})
	}

	step(t, "bob replays alice's consent form", func() {
		e.openConsent(alice, clientID, challenge, "replay", []string{"ws-a"}, nil)
		fields := url.Values{"request": {alice.attr(`input[name=request]`, "value")}, "csrf": {alice.attr(`input[name=csrf]`, "value")}, "decision": {"allow"}}
		for _, ws := range []string{"ws-a", bobWS} {
			fields.Set("workspace", ws)
			bob.open(e.origin + "/test/replay?" + fields.Encode())
			bob.waitPath(oauth.PathConsent)
			if body := bob.text("body"); !strings.Contains(body, "The consent request is invalid or expired.") {
				t.Fatalf("replay into %s: %s", ws, body)
			}
		}
	})
	if n := e.store.codeCount(); n != 0 {
		t.Fatalf("cross-tenant attempts stored %d codes", n)
	}

	step(t, "no token for the other tenant", func() {
		// bob's own browser grant yields a ws-bob token only; alice's client
		// state never reaches it.
		verifier, ch := pkce()
		browserCode := func(state string) string {
			e.openConsent(bob, clientID, ch, state, []string{bobWS}, []string{"ws-a"})
			bob.run("click", "button[value=allow]")
			code, errCode := e.callbackResult(bob, state)
			if code == "" {
				t.Fatalf("bob allow: error=%q", errCode)
			}
			return code
		}
		// Another client cannot redeem bob's code. A failed redemption may
		// consume it, so bob redeems a second browser-issued code.
		other := e.registerClient()
		status, m := e.exchange(other, browserCode("bob-stolen"), e.callback, verifier)
		wantError(t, status, m, 400, "invalid_grant")
		code := browserCode("bob-own")
		status, m = e.exchange(clientID, reworkspace(t, code, "ws-a"), e.callback, verifier)
		wantError(t, status, m, 400, "invalid_grant")
		status, m = e.exchange(clientID, code, e.callback, verifier)
		if status != http.StatusOK {
			t.Fatalf("bob exchange: %d %v", status, m)
		}
		if _, p, _ := e.callResource("/resource-a", m["access_token"].(string)); p.WorkspaceID != bobWS || p.Subject != "bob" {
			t.Fatalf("bob token principal = %+v", p)
		}
	})
}

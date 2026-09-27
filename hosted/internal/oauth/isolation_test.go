package oauth_test

import (
	"encoding/base64"
	"net/http"
	"strings"
	"testing"
)

// Multi-tenant OAuth boundaries at the real HTTP surface of the reference
// AS. Two tenants share one issuer: alice works in ws-a, bob in ws-bob. The
// browser suite (browser_test.go, build tag "browser") repeats the consent
// paths in a real browser.

const (
	redirectCB = "https://connector.example/cb"
	bobWS      = "ws-bob"
)

func twoTenants(t *testing.T) *harness {
	t.Helper()
	h := newHarness(t)
	h.policy.set("bob", bobWS, []string{"catalog:read", "catalog:publish"}, true)
	return h
}

// consentPage runs authorize for subject's cookie and returns the sealed
// request and CSRF token from the rendered consent form.
func (h *harness) consentPage(clientID, challenge, resource string, cookie *http.Cookie) (string, string) {
	h.t.Helper()
	resp, body := h.authorize(authReq{clientID: clientID, redirect: redirectCB, challenge: challenge, resource: resource, scope: "catalog:read", state: "st"}, cookie)
	if resp.StatusCode != http.StatusOK {
		h.t.Fatalf("authorize: %d %s", resp.StatusCode, body)
	}
	return consentFields(h.t, body)
}

func TestIsolationConsentCannotCrossTenants(t *testing.T) {
	h := twoTenants(t)
	clientID := h.client(redirectCB, "")
	_, challenge := pkce()
	cases := []struct {
		name      string
		cookie    *http.Cookie
		workspace string
	}{
		// alice's session does not list bob's workspace: a forged form value
		// must not reach the membership oracle at all.
		{"workspace outside the session", h.session("alice", "ws-a"), bobWS},
		// a session that lists ws-bob (stale or forged claim) still needs a
		// real membership there.
		{"workspace in the session without membership", h.session("alice", "ws-a", bobWS), bobWS},
		{"bob into alice's workspace", h.session("bob", bobWS), "ws-a"},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			before := h.store.codeCount()
			request, csrf := h.consentPage(clientID, challenge, h.origin, tc.cookie)
			resp, _ := h.consent(tc.cookie, request, csrf, tc.workspace, "allow", nil)
			q := location(t, resp)
			if q.Get("code") != "" || q.Get("error") != "access_denied" {
				t.Fatalf("cross-tenant consent redirect = %v", q)
			}
			if h.store.codeCount() != before {
				t.Fatal("cross-tenant consent stored a code")
			}
		})
	}
}

func TestIsolationCrossSessionConsentReplay(t *testing.T) {
	h := twoTenants(t)
	clientID := h.client(redirectCB, "")
	_, challenge := pkce()
	alice := h.session("alice", "ws-a")
	request, csrf := h.consentPage(clientID, challenge, h.origin, alice)
	// bob replays alice's consent form, and a second alice session replays
	// the first one's: the sealed request is bound to the session that saw it.
	for name, cookie := range map[string]*http.Cookie{"other tenant": h.session("bob", bobWS), "same user, other session": h.session("alice", "ws-a")} {
		t.Run(name, func(t *testing.T) {
			for _, ws := range []string{"ws-a", bobWS} {
				resp, body := h.consent(cookie, request, csrf, ws, "allow", nil)
				if resp.StatusCode != http.StatusForbidden || resp.Header.Get("Location") != "" {
					t.Fatalf("replay into %s: %d %s", ws, resp.StatusCode, body)
				}
			}
		})
	}
	if h.store.codeCount() != 0 {
		t.Fatal("replayed consent stored a code")
	}
}

func TestIsolationTokensStayInTheirTenant(t *testing.T) {
	h := twoTenants(t)
	aliceClient := h.client(redirectCB, "")
	bobClient := h.client(redirectCB, "")
	verifier, challenge := pkce()
	bob := h.session("bob", bobWS)
	newCode := func(state string) string {
		return h.code(authReq{clientID: bobClient, redirect: redirectCB, challenge: challenge, resource: h.origin, scope: "catalog:read", state: state}, bob, bobWS)
	}

	// alice's client cannot redeem bob's code. (A failed redemption may
	// consume the code, so bob redeems a separate one below.)
	status, m := h.exchange(aliceClient, newCode("stolen"), redirectCB, verifier)
	wantError(t, status, m, 400, "invalid_grant")
	// Rewriting the code's workspace segment to ws-a finds nothing there.
	code := newCode("b")
	status, m = h.exchange(bobClient, reworkspace(t, code, "ws-a"), redirectCB, verifier)
	wantError(t, status, m, 400, "invalid_grant")

	status, m = h.exchange(bobClient, code, redirectCB, verifier)
	if status != http.StatusOK {
		t.Fatalf("bob exchange: %d %v", status, m)
	}
	access, refresh := m["access_token"].(string), m["refresh_token"].(string)
	st, p, _ := h.callResource("/resource-a", access)
	if st != http.StatusOK || p.Subject != "bob" || p.WorkspaceID != bobWS {
		t.Fatalf("bob principal = %d %+v", st, p)
	}
	// bob's refresh token is neither usable by alice's client nor movable
	// into alice's workspace.
	status, m = h.refresh(aliceClient, refresh, nil)
	wantError(t, status, m, 400, "invalid_grant")
	status, m = h.refresh(bobClient, reworkspace(t, refresh, "ws-a"), nil)
	wantError(t, status, m, 400, "invalid_grant")
	// None of the foreign attempts consumed bob's token.
	status, m = h.refresh(bobClient, refresh, nil)
	if status != http.StatusOK {
		t.Fatalf("bob refresh after foreign attempts: %d %v", status, m)
	}
	if _, p, _ := h.callResource("/resource-a", m["access_token"].(string)); p.WorkspaceID != bobWS {
		t.Fatalf("refreshed principal moved workspace: %+v", p)
	}
}

// reworkspace swaps the workspace segment of an opaque code/refresh token.
func reworkspace(t *testing.T, raw, ws string) string {
	t.Helper()
	parts := strings.Split(raw, ".")
	if len(parts) != 3 {
		t.Fatalf("opaque credential shape: %q", raw)
	}
	parts[1] = base64.RawURLEncoding.EncodeToString([]byte(ws))
	return strings.Join(parts, ".")
}

func TestIsolationRevokedAndMissingMembership(t *testing.T) {
	h := twoTenants(t)
	clientID, access, refresh := h.grant(h.origin, "catalog:read", "ws-a")
	// Removing alice from ws-a ends refresh and the family for good; bob's
	// grant in the other tenant is untouched.
	bobClient := h.client(redirectCB, "")
	verifier, challenge := pkce()
	bob := h.session("bob", bobWS)
	code := h.code(authReq{clientID: bobClient, redirect: redirectCB, challenge: challenge, resource: h.origin, scope: "catalog:read", state: "b"}, bob, bobWS)
	status, m := h.exchange(bobClient, code, redirectCB, verifier)
	if status != http.StatusOK {
		t.Fatalf("bob exchange: %d %v", status, m)
	}
	bobRefresh := m["refresh_token"].(string)

	h.policy.set("alice", "ws-a", []string{"catalog:read", "catalog:publish"}, false)
	status, m = h.refresh(clientID, refresh, nil)
	wantError(t, status, m, 400, "invalid_grant")
	if st, _, _ := h.callResource("/resource-a", access); st != http.StatusUnauthorized {
		t.Fatalf("access token of a revoked member answered %d", st)
	}
	status, m = h.refresh(bobClient, bobRefresh, nil)
	if status != http.StatusOK {
		t.Fatalf("other tenant's refresh affected: %d %v", status, m)
	}
	// A session for a workspace with no membership record gets no consent.
	stranger := h.session("carol", "ws-a")
	request, csrf := h.consentPage(clientID, challenge, h.origin, stranger)
	resp, _ := h.consent(stranger, request, csrf, "ws-a", "allow", nil)
	if q := location(t, resp); q.Get("error") != "access_denied" || q.Get("code") != "" {
		t.Fatalf("missing membership consent = %v", q)
	}
}

func TestIsolationResourceMixUp(t *testing.T) {
	h := twoTenants(t)
	_, tokenA, _ := h.grant(h.origin, "catalog:read", "ws-a")
	_, tokenB, _ := h.grant(resourceB, "catalog:read", "ws-a")
	if st, _, hdr := h.callResource("/resource-b", tokenA); st != http.StatusUnauthorized || !strings.Contains(hdr.Get("WWW-Authenticate"), `error="invalid_token"`) {
		t.Fatalf("resource-A token at B: %d %q", st, hdr.Get("WWW-Authenticate"))
	}
	if st, _, hdr := h.callResource("/resource-a", tokenB); st != http.StatusUnauthorized || !strings.Contains(hdr.Get("WWW-Authenticate"), `error="invalid_token"`) {
		t.Fatalf("resource-B token at A: %d %q", st, hdr.Get("WWW-Authenticate"))
	}
}

func TestIsolationRefreshReuseRevokesOnlyThatFamily(t *testing.T) {
	h := twoTenants(t)
	c1, _, r1 := h.grant(h.origin, "catalog:read", "ws-a")
	c2, _, r2 := h.grant(h.origin, "catalog:read", "ws-b")
	status, m := h.refresh(c1, r1, nil)
	if status != http.StatusOK {
		t.Fatalf("rotate: %d %v", status, m)
	}
	next := m["refresh_token"].(string)
	status, m = h.refresh(c1, r1, nil) // reuse
	wantError(t, status, m, 400, "invalid_grant")
	status, m = h.refresh(c1, next, nil)
	wantError(t, status, m, 400, "invalid_grant")
	if status, m = h.refresh(c2, r2, nil); status != http.StatusOK {
		t.Fatalf("unrelated family revoked: %d %v", status, m)
	}
}

func TestIsolationIssuerOutage(t *testing.T) {
	h := twoTenants(t)
	clientID, access, refresh := h.grant(h.origin, "catalog:read", "ws-a")
	verifier, challenge := pkce()
	code := h.code(authReq{clientID: clientID, redirect: redirectCB, challenge: challenge, resource: h.origin, scope: "catalog:read", state: "o"}, h.session("alice", "ws-a"), "ws-a")

	h.store.setDown(true)
	status, m := h.refresh(clientID, refresh, nil)
	wantError(t, status, m, http.StatusServiceUnavailable, "temporarily_unavailable")
	status, m = h.exchange(clientID, code, redirectCB, verifier)
	wantError(t, status, m, http.StatusServiceUnavailable, "temporarily_unavailable")
	// Access tokens already issued keep verifying against published keys.
	if st, p, _ := h.callResource("/resource-a", access); st != http.StatusOK || p.WorkspaceID != "ws-a" {
		t.Fatalf("resource during issuer outage: %d %+v", st, p)
	}
	h.store.setDown(false)
	// The outage consumed nothing: the same code and refresh token work once.
	if status, m = h.exchange(clientID, code, redirectCB, verifier); status != http.StatusOK {
		t.Fatalf("code after outage: %d %v", status, m)
	}
	if status, m = h.refresh(clientID, refresh, nil); status != http.StatusOK {
		t.Fatalf("refresh after outage: %d %v", status, m)
	}
}

// TestIsolationConsentFormRejectsForeignOrigins covers the CSRF edges a
// browser produces: a cross-site POST is refused before the session is read.
func TestIsolationConsentFormRejectsForeignOrigins(t *testing.T) {
	h := twoTenants(t)
	clientID := h.client(redirectCB, "")
	_, challenge := pkce()
	alice := h.session("alice", "ws-a")
	request, csrf := h.consentPage(clientID, challenge, h.origin, alice)
	for _, headers := range []map[string]string{
		{"Origin": "https://attacker.example"},
		{"Sec-Fetch-Site": "cross-site"},
		{"Sec-Fetch-Site": "same-site", "Origin": "https://sub." + strings.TrimPrefix(h.origin, "https://")},
		{"Origin": "null"},
	} {
		resp, _ := h.consent(alice, request, csrf, "ws-a", "allow", headers)
		if resp.StatusCode != http.StatusForbidden {
			t.Fatalf("consent with %v: %d", headers, resp.StatusCode)
		}
	}
	resp, _ := h.consent(alice, request, csrf, "ws-a", "allow", map[string]string{"Origin": h.origin, "Sec-Fetch-Site": "same-origin"})
	if q := location(t, resp); q.Get("code") == "" {
		t.Fatalf("same-origin consent = %v", q)
	}
}

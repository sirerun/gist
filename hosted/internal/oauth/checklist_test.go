package oauth_test

import (
	"bytes"
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"net/http"
	"net/http/httptest"
	"net/url"
	"os"
	"path/filepath"
	"regexp"
	"strings"
	"testing"

	"github.com/sirerun/gist/hosted/internal/oauth"
)

// ---- issuer qualification fixtures ----------------------------------------

// harnessConsent is the probe's test user at a harness issuer: alice, signed
// in with the session cookie the harness's configured identity issues, who
// consents into ws-a.
type harnessConsent struct {
	h      *harness
	cookie *http.Cookie
}

func (c *harnessConsent) Authorize(ctx context.Context, authorizeURL string, approve bool) (*url.URL, error) {
	resp, body, err := c.send(ctx, http.MethodGet, authorizeURL, nil)
	if err != nil {
		return nil, err
	}
	if resp.StatusCode == http.StatusFound {
		return resp.Location()
	}
	if resp.StatusCode != http.StatusOK {
		return nil, fmt.Errorf("authorize answered %d", resp.StatusCode)
	}
	r, cs := requestField.FindSubmatch(body), csrfField.FindSubmatch(body)
	if r == nil || cs == nil {
		return nil, errors.New("authorize page has no consent form")
	}
	decision := "deny"
	if approve {
		decision = "allow"
	}
	form := url.Values{"request": {string(r[1])}, "csrf": {string(cs[1])}, "workspace": {"ws-a"}, "decision": {decision}}
	resp, _, err = c.send(ctx, http.MethodPost, c.h.origin+oauth.PathConsent, form)
	if err != nil {
		return nil, err
	}
	if resp.StatusCode != http.StatusFound {
		return nil, fmt.Errorf("consent answered %d", resp.StatusCode)
	}
	return resp.Location()
}

func (c *harnessConsent) send(ctx context.Context, method, target string, form url.Values) (*http.Response, []byte, error) {
	var body io.Reader
	if form != nil {
		body = strings.NewReader(form.Encode())
	}
	req, err := http.NewRequestWithContext(ctx, method, target, body)
	if err != nil {
		return nil, nil, err
	}
	if form != nil {
		req.Header.Set("Content-Type", "application/x-www-form-urlencoded")
	}
	req.AddCookie(c.cookie)
	resp, err := c.h.http.Do(req)
	if err != nil {
		return nil, nil, err
	}
	defer resp.Body.Close()
	raw, err := io.ReadAll(resp.Body)
	return resp, raw, err
}

func (c *harnessConsent) RevokeConsent(context.Context) error {
	c.h.policy.set("alice", "ws-a", []string{"catalog:read", "catalog:publish"}, false)
	return nil
}

func (c *harnessConsent) RestoreConsent(context.Context) error {
	c.h.policy.set("alice", "ws-a", []string{"catalog:read", "catalog:publish"}, true)
	return nil
}

func probeTarget(h *harness) oauth.ProbeTarget {
	return oauth.ProbeTarget{
		Issuer:           h.origin,
		Resource:         h.origin,
		ResourceURL:      h.origin + "/resource-a",
		OtherResource:    resourceB,
		OtherResourceURL: h.origin + "/resource-b",
		Scope:            "catalog:read",
		WiderScope:       "catalog:publish",
		RedirectURI:      "https://connector.example/cb",
		HTTP:             h.http,
		Consent:          &harnessConsent{h: h, cookie: h.session("alice", "ws-a")},
	}
}

func qualify(t *testing.T, h *harness, strategy oauth.Strategy) oauth.Qualification {
	t.Helper()
	return oauth.Qualify(context.Background(), "local-reference", "local-test", strategy, probeTarget(h))
}

// acceptedReceipts are in-memory W1-W5 fixtures for exercising the selection
// logic. They are test data, not evidence: no receipt file is written.
func acceptedReceipts() map[string]oauth.InterfaceReceipt {
	out := map[string]oauth.InterfaceReceipt{}
	for _, id := range oauth.RequiredReceipts {
		out[id] = oauth.InterfaceReceipt{ID: id, Status: "accepted", Revision: "fixture", Interface: id + "-fixture", Evidence: json.RawMessage(`{"fixture":true}`)}
	}
	return out
}

func logRows(t *testing.T, q oauth.Qualification) {
	t.Helper()
	for _, r := range q.Rows {
		t.Logf("%-38s %-7s %s", r.ID, r.Status, r.Reason)
	}
}

// ---- built-in issuer --------------------------------------------------------

func TestIssuerChecklistBuiltInPassesEveryRow(t *testing.T) {
	h := newHarness(t)
	q := qualify(t, h, oauth.StrategyBuiltIn)
	logRows(t, q)
	if !q.Passed() || q.Status != oauth.QualificationQualified || len(q.FailingRows) != 0 {
		t.Fatalf("built-in qualification = %s failing %v", q.Status, q.FailingRows)
	}
	if len(q.Rows) != len(oauth.Checklist) {
		t.Fatalf("rows = %d, want %d", len(q.Rows), len(oauth.Checklist))
	}
	for _, r := range q.Rows {
		if len(r.Probes) < 2 {
			t.Errorf("row %s ran %d probes; every row needs positive and negative probes", r.ID, len(r.Probes))
		}
	}
	sel, err := oauth.SelectIssuer(q, oauth.DelegationConfig{}, nil, nil)
	if err != nil || sel.Strategy != oauth.StrategyBuiltIn || sel.Issuer != h.origin {
		t.Fatalf("selection = %+v, %v", sel, err)
	}
}

// ---- fake issuers that each fail exactly one row ----------------------------

// rewriteForm replaces a POSTed form body after edit changes it.
func rewriteForm(r *http.Request, edit func(url.Values)) {
	raw, _ := io.ReadAll(r.Body)
	form, _ := url.ParseQuery(string(raw))
	edit(form)
	enc := form.Encode()
	r.Body = io.NopCloser(strings.NewReader(enc))
	r.ContentLength = int64(len(enc))
}

func onPath(path string, fault func(http.Handler, http.ResponseWriter, *http.Request)) func(http.Handler) http.Handler {
	return func(next http.Handler) http.Handler {
		return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			if r.URL.Path == path {
				fault(next, w, r)
				return
			}
			next.ServeHTTP(w, r)
		})
	}
}

// headerEdit rewrites response headers just before they are written.
type headerEdit struct {
	http.ResponseWriter
	edit func(http.Header)
}

func (h *headerEdit) WriteHeader(status int) {
	h.edit(h.Header())
	h.ResponseWriter.WriteHeader(status)
}

var metadataParam = regexp.MustCompile(`resource_metadata="[^"]*"`)

// faultyIssuers are local issuers built from the reference server with one
// deliberate defect each, and the only checklist row that defect must fail.
var faultyIssuers = []struct {
	name  string
	row   oauth.RowID
	fault func(http.Handler) http.Handler
}{
	{
		name: "metadata advertises a revocation endpoint that does not exist",
		row:  oauth.RowDiscovery,
		fault: onPath(oauth.PathASMetadata, func(next http.Handler, w http.ResponseWriter, r *http.Request) {
			rec := httptest.NewRecorder()
			next.ServeHTTP(rec, r)
			var md map[string]any
			_ = json.Unmarshal(rec.Body.Bytes(), &md)
			md["revocation_endpoint"] = md["issuer"].(string) + "/oauth/revocation-missing"
			w.Header().Set("Content-Type", "application/json")
			_ = json.NewEncoder(w).Encode(md)
		}),
	},
	{
		name: "registration accepts plain-HTTP redirect URIs",
		row:  oauth.RowRegistration,
		fault: onPath(oauth.PathRegister, func(next http.Handler, w http.ResponseWriter, r *http.Request) {
			raw, _ := io.ReadAll(r.Body)
			raw = bytes.ReplaceAll(raw, []byte(`"http://`), []byte(`"https://`))
			r.Body, r.ContentLength = io.NopCloser(bytes.NewReader(raw)), int64(len(raw))
			next.ServeHTTP(w, r)
		}),
	},
	{
		name: "authorization accepts the plain PKCE method",
		row:  oauth.RowAuthorizationCode,
		fault: onPath(oauth.PathAuthorize, func(next http.Handler, w http.ResponseWriter, r *http.Request) {
			q := r.URL.Query()
			if q.Get("code_challenge_method") == "plain" {
				q.Set("code_challenge_method", "S256")
				r.URL.RawQuery = q.Encode()
			}
			next.ServeHTTP(w, r)
		}),
	},
	{
		name: "refresh replays a reused token instead of revoking its family",
		row:  oauth.RowRefresh,
		fault: func() func(http.Handler) http.Handler {
			issued := map[string]*httptest.ResponseRecorder{}
			return onPath(oauth.PathToken, func(next http.Handler, w http.ResponseWriter, r *http.Request) {
				var presented string
				rewriteForm(r, func(f url.Values) {
					if f.Get("grant_type") == "refresh_token" {
						presented = f.Get("refresh_token")
					}
				})
				rec, ok := issued[presented]
				if presented == "" || !ok {
					rec = httptest.NewRecorder()
					next.ServeHTTP(rec, r)
					if presented != "" && rec.Code == http.StatusOK {
						issued[presented] = rec
					}
				}
				for k, v := range rec.Header() {
					w.Header()[k] = v
				}
				w.WriteHeader(rec.Code)
				_, _ = w.Write(rec.Body.Bytes())
			})
		}(),
	},
	{
		name: "resource challenge points at AS metadata instead of the PRM document",
		row:  oauth.RowProtectedResource,
		fault: onPath("/resource-a", func(next http.Handler, w http.ResponseWriter, r *http.Request) {
			next.ServeHTTP(&headerEdit{ResponseWriter: w, edit: func(h http.Header) {
				if c := h.Get("WWW-Authenticate"); c != "" {
					h.Set("WWW-Authenticate", metadataParam.ReplaceAllString(c, `resource_metadata="https://`+r.Host+oauth.PathASMetadata+`"`))
				}
			}}, r)
		}),
	},
	{
		name: "token endpoint ignores the resource indicator at redemption and refresh",
		row:  oauth.RowResourceAudience,
		fault: onPath(oauth.PathToken, func(next http.Handler, w http.ResponseWriter, r *http.Request) {
			rewriteForm(r, func(f url.Values) { f.Del("resource") })
			next.ServeHTTP(w, r)
		}),
	},
}

func TestIssuerChecklistRejectsPartialCompliance(t *testing.T) {
	builtIn := qualify(t, newHarness(t), oauth.StrategyBuiltIn)
	if !builtIn.Passed() {
		t.Fatalf("built-in failing rows %v", builtIn.FailingRows)
	}
	for _, fi := range faultyIssuers {
		t.Run(string(fi.row), func(t *testing.T) {
			h := newHarness(t, fi.fault)
			q := qualify(t, h, oauth.StrategyDelegated)
			logRows(t, q)
			if q.Status != oauth.QualificationRejected || len(q.FailingRows) != 1 || q.FailingRows[0] != fi.row {
				t.Fatalf("%s: status %s failing %v, want exactly [%s]", fi.name, q.Status, q.FailingRows, fi.row)
			}
			// Every other interface input is present and the issuer is
			// allowlisted: the single failing row alone must block delegation.
			sel, err := oauth.SelectIssuer(builtIn, oauth.DelegationConfig{Issuer: h.origin, Allowlist: []string{h.origin}}, acceptedReceipts(), &q)
			if err != nil {
				t.Fatal(err)
			}
			if sel.Strategy != oauth.StrategyBuiltIn || sel.Issuer != builtIn.Issuer {
				t.Fatalf("partial compliance selected %+v", sel)
			}
			if len(sel.FailingRows) != 1 || sel.FailingRows[0] != fi.row || len(sel.Reasons) != 1 || !strings.Contains(sel.Reasons[0], string(fi.row)) {
				t.Fatalf("fallback does not name the failing row: %+v", sel)
			}
			t.Logf("rejected (%s): %s", fi.name, sel.Reasons[0])
		})
	}
}

// ---- delegation decision ----------------------------------------------------

func TestIssuerChecklistSelectsDelegationOnlyWhenComplete(t *testing.T) {
	builtIn := qualify(t, newHarness(t), oauth.StrategyBuiltIn)
	ext := newHarness(t) // a second, fully compliant issuer at its own origin
	extQ := qualify(t, ext, oauth.StrategyDelegated)
	if !extQ.Passed() {
		t.Fatalf("compliant external issuer failing rows %v", extQ.FailingRows)
	}
	cfg := oauth.DelegationConfig{Issuer: ext.origin, Allowlist: []string{ext.origin}}

	sel, err := oauth.SelectIssuer(builtIn, cfg, acceptedReceipts(), &extQ)
	if err != nil || sel.Strategy != oauth.StrategyDelegated || sel.Issuer != ext.origin {
		t.Fatalf("complete delegation = %+v, %v", sel, err)
	}

	missingW5 := acceptedReceipts()
	delete(missingW5, "w5")
	skipped := acceptedReceipts()
	skipped["w3"] = oauth.InterfaceReceipt{ID: "w3", Status: "passed", Revision: "r", Interface: "w3", Evidence: json.RawMessage(`{"x":1}`), TestsSkipped: 2}
	other := qualify(t, newHarness(t), oauth.StrategyDelegated)
	cases := []struct {
		name     string
		cfg      oauth.DelegationConfig
		receipts map[string]oauth.InterfaceReceipt
		q        *oauth.Qualification
		reason   string
	}{
		{"not allowlisted", oauth.DelegationConfig{Issuer: ext.origin}, acceptedReceipts(), &extQ, "allowlist"},
		{"W5 receipt missing", cfg, missingW5, &extQ, "W5 is missing"},
		{"receipt with skipped tests", cfg, skipped, &extQ, "W3 is not accepted"},
		{"receipts alone, no runtime probes", cfg, acceptedReceipts(), nil, "no runtime qualification"},
		{"qualification of a different issuer", cfg, acceptedReceipts(), &other, "runtime qualification is for"},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			sel, err := oauth.SelectIssuer(builtIn, tc.cfg, tc.receipts, tc.q)
			if err != nil {
				t.Fatal(err)
			}
			if sel.Strategy != oauth.StrategyBuiltIn || sel.Issuer != builtIn.Issuer {
				t.Fatalf("selected %+v", sel)
			}
			if !strings.Contains(strings.Join(sel.Reasons, "\n"), tc.reason) {
				t.Fatalf("reasons %q lack %q", sel.Reasons, tc.reason)
			}
		})
	}

	// A built-in issuer that is itself not qualified leaves no issuer at all:
	// the deployment must not start rather than accept partial compliance.
	broken := qualify(t, newHarness(t, faultyIssuers[0].fault), oauth.StrategyBuiltIn)
	if _, err := oauth.SelectIssuer(broken, oauth.DelegationConfig{}, nil, nil); !errors.Is(err, oauth.ErrBuiltInNotQualified) {
		t.Fatalf("unqualified built-in: err = %v", err)
	}
}

// TestIssuerChecklistExternalPending records the real external issuer state:
// no W1-W5 receipts exist yet (X-W5 is blocked on external evidence), so the
// external issuer is pending, never qualified, and the built-in is selected.
func TestIssuerChecklistExternalPending(t *testing.T) {
	builtIn := qualify(t, newHarness(t), oauth.StrategyBuiltIn)
	receipts, err := oauth.LoadReceipts(filepath.Join("..", "..", "..", "docs", "registry", "dependencies"))
	if err != nil {
		t.Fatal(err)
	}
	if len(receipts) == len(oauth.RequiredReceipts) {
		t.Skip("all W1-W5 receipts exist; run the external issuer through Qualify instead of recording it pending")
	}
	pending := oauth.PendingQualification("local-reference", "local-test", oauth.StrategyDelegated, externalIssuerLabel, externalPendingReason)
	if pending.Status != oauth.QualificationPending || pending.Passed() {
		t.Fatalf("pending qualification = %+v", pending)
	}
	sel, err := oauth.SelectIssuer(builtIn, oauth.DelegationConfig{Issuer: externalIssuerLabel, Allowlist: []string{externalIssuerLabel}}, receipts, &pending)
	if err != nil || sel.Strategy != oauth.StrategyBuiltIn || len(sel.FailingRows) != len(oauth.Checklist) {
		t.Fatalf("pending external selection = %+v, %v", sel, err)
	}
	for _, id := range oauth.RequiredReceipts {
		if _, ok := receipts[id]; !ok && !strings.Contains(strings.Join(sel.Reasons, "\n"), strings.ToUpper(id)+" is missing") {
			t.Fatalf("reasons do not name missing receipt %s: %q", id, sel.Reasons)
		}
	}
}

// ---- recorded per-deployment checklist ----------------------------------------

const (
	checklistFile         = "issuer-checklist.json"
	referenceIssuerLabel  = "built-in reference AS at an isolated local TLS test origin"
	externalIssuerLabel   = "external delegated issuer (not yet selected)"
	externalPendingReason = "runtime probes not run: W1-W5 interface receipts are absent and X-W5 external connector evidence is pending"
)

// buildChecklistDocument produces the per-deployment record from live runs.
// Issuer origins are ephemeral test ports, so they are replaced by labels.
func buildChecklistDocument(t *testing.T) oauth.ChecklistDocument {
	t.Helper()
	builtIn := qualify(t, newHarness(t), oauth.StrategyBuiltIn)
	builtIn.Issuer = referenceIssuerLabel
	pending := oauth.PendingQualification("local-reference", "local-test", oauth.StrategyDelegated, externalIssuerLabel, externalPendingReason)
	sel, err := oauth.SelectIssuer(builtIn, oauth.DelegationConfig{Issuer: externalIssuerLabel, Allowlist: []string{externalIssuerLabel}}, nil, &pending)
	if err != nil {
		t.Fatal(err)
	}
	return oauth.ChecklistDocument{
		Schema:            oauth.ChecklistSchema,
		RequirementSource: "docs/rfc-002.md section 10; docs/plans/registry-buildout.md Section 10 authorization-server requirement checklist",
		Deployments: []oauth.DeploymentRecord{{
			Deployment:     "local-reference",
			Build:          "local-test",
			Selection:      sel,
			Qualifications: []oauth.Qualification{builtIn, pending},
		}},
	}
}

// TestIssuerChecklistRecord keeps issuer-checklist.json equal to a fresh
// qualification run. Set GIST_UPDATE_ISSUER_CHECKLIST=1 to rewrite it.
func TestIssuerChecklistRecord(t *testing.T) {
	doc := buildChecklistDocument(t)
	for _, d := range doc.Deployments {
		if err := d.Validate(); err != nil {
			t.Fatal(err)
		}
	}
	want, err := json.MarshalIndent(doc, "", "  ")
	if err != nil {
		t.Fatal(err)
	}
	want = append(want, '\n')
	if os.Getenv("GIST_UPDATE_ISSUER_CHECKLIST") == "1" {
		if err := os.WriteFile(checklistFile, want, 0o644); err != nil {
			t.Fatal(err)
		}
	}
	got, err := os.ReadFile(checklistFile)
	if err != nil {
		t.Fatal(err)
	}
	if !bytes.Equal(got, want) {
		t.Fatalf("%s is stale; rerun with GIST_UPDATE_ISSUER_CHECKLIST=1\n--- want\n%s", checklistFile, want)
	}
	var committed oauth.ChecklistDocument
	if err := json.Unmarshal(got, &committed); err != nil {
		t.Fatal(err)
	}
	if committed.Deployments[0].Selection.Strategy != oauth.StrategyBuiltIn {
		t.Fatal("recorded deployment delegates without a qualified external issuer")
	}
}

package oauth

// Delegated-issuer qualification (RFC-002 section 10, strategy 2).
//
// A deployment may delegate token issuance to an external OAuth 2.1
// authorization server only when that issuer passes every row of the section
// 10 checklist. Qualify runs the same black-box HTTP conformance probes
// against any issuer (the built-in reference server or an external one), and
// SelectIssuer turns the results into exactly one accepted issuer. Partial
// compliance is rejected: any failing, pending or missing row selects the
// built-in strategy and names the rows that failed. The selection happens at
// deployment time; it never accepts two issuers at once and never changes the
// issuer behind a live session.

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
	"net/url"
	"os"
	"path/filepath"
	"regexp"
	"strings"
)

// RowID names one section 10 checklist row.
type RowID string

// The six section 10 authorization-server requirement rows.
const (
	RowDiscovery         RowID = "rfc8414-discovery"
	RowRegistration      RowID = "rfc7591-registration"
	RowAuthorizationCode RowID = "authorization-code-pkce-consent"
	RowRefresh           RowID = "refresh-rotation-reuse"
	RowProtectedResource RowID = "rfc9728-protected-resource-metadata"
	RowResourceAudience  RowID = "rfc8707-resource-audience"
)

// ChecklistRow is a checklist requirement and its acceptance probe.
type ChecklistRow struct {
	ID          RowID
	Requirement string
}

// Checklist is the section 10 requirement checklist in plan order.
var Checklist = []ChecklistRow{
	{RowDiscovery, "RFC 8414 AS discovery: metadata locates actual authorize/token/register/revoke endpoints and supported S256/resource behavior"},
	{RowRegistration, "RFC 7591 DCR: HTTPS-only redirect URIs; invalid or excess scopes are rejected, never downgraded"},
	{RowAuthorizationCode, "Authorization code + PKCE + consent: S256 only; code single-use and bound to client, redirect and consent; denial issues no grant"},
	{RowRefresh, "Refresh rotation and reuse containment: rotates once, preserves bounds; reuse revokes the family; revoked consent is rejected"},
	{RowProtectedResource, "RFC 9728 PRM: real document with resource, authorization servers and catalog scopes; 401 challenge points to PRM"},
	{RowResourceAudience, "RFC 8707 resource/audience: resource bound at authorize, redemption and refresh; token for A rejected by B"},
}

// RowStatus is a checklist row outcome.
type RowStatus string

// Row outcomes. Only pass counts toward qualification.
const (
	StatusPass    RowStatus = "pass"
	StatusFail    RowStatus = "fail"
	StatusPending RowStatus = "pending"
)

// Strategy is the section 10 issuance strategy.
type Strategy string

// Issuance strategies.
const (
	StrategyBuiltIn   Strategy = "built-in"
	StrategyDelegated Strategy = "delegated"
)

// Qualification outcomes.
const (
	QualificationQualified = "qualified"
	QualificationRejected  = "rejected"
	QualificationPending   = "pending"
)

// ProbeResult is one conformance probe inside a row.
type ProbeResult struct {
	Name   string `json:"name"`
	Passed bool   `json:"passed"`
	Detail string `json:"detail,omitempty"`
}

// RowResult is one checklist row's outcome and the probes behind it.
type RowResult struct {
	ID          RowID         `json:"id"`
	Requirement string        `json:"requirement"`
	Status      RowStatus     `json:"status"`
	Probes      []ProbeResult `json:"probes,omitempty"`
	Reason      string        `json:"reason,omitempty"`
}

// Qualification is the checklist result for one issuer in one deployment.
type Qualification struct {
	Deployment  string      `json:"deployment"`
	Build       string      `json:"build"`
	Strategy    Strategy    `json:"strategy"`
	Issuer      string      `json:"issuer"`
	Status      string      `json:"status"`
	FailingRows []RowID     `json:"failing_rows,omitempty"`
	Rows        []RowResult `json:"rows"`
}

// Passed reports whether every checklist row is present and passed.
func (q Qualification) Passed() bool {
	return len(q.failing()) == 0
}

func (q Qualification) failing() []RowID {
	byID := make(map[RowID]RowResult, len(q.Rows))
	for _, r := range q.Rows {
		byID[r.ID] = r
	}
	var out []RowID
	for _, row := range Checklist {
		if r, ok := byID[row.ID]; !ok || r.Status != StatusPass {
			out = append(out, row.ID)
		}
	}
	return out
}

func (q *Qualification) finish() {
	q.FailingRows = q.failing()
	switch {
	case len(q.FailingRows) == 0:
		q.Status = QualificationQualified
	case q.allPending():
		q.Status = QualificationPending
	default:
		q.Status = QualificationRejected
	}
}

func (q Qualification) allPending() bool {
	for _, r := range q.Rows {
		if r.Status != StatusPending {
			return false
		}
	}
	return len(q.Rows) > 0
}

// PendingQualification records an issuer whose runtime probes have not run,
// for example because its external evidence does not exist yet. Every row is
// pending, so the issuer cannot be selected.
func PendingQualification(deployment, build string, strategy Strategy, issuer, reason string) Qualification {
	q := Qualification{Deployment: deployment, Build: build, Strategy: strategy, Issuer: issuer}
	for _, row := range Checklist {
		q.Rows = append(q.Rows, RowResult{ID: row.ID, Requirement: row.Requirement, Status: StatusPending, Reason: reason})
	}
	q.finish()
	return q
}

// ConsentDriver acts as the probe's test user at the issuer. It is the only
// part of qualification that knows how the issuer authenticates a human.
type ConsentDriver interface {
	// Authorize loads authorizeURL as the signed-in test user, approves or
	// denies consent when the issuer asks, and returns the URL the issuer
	// finally redirects to (the client's redirect URI with code or error).
	Authorize(ctx context.Context, authorizeURL string, approve bool) (*url.URL, error)
	// RevokeConsent withdraws the test user's standing for the probed
	// workspace; RestoreConsent reinstates it.
	RevokeConsent(ctx context.Context) error
	RestoreConsent(ctx context.Context) error
}

// ProbeTarget describes the issuer and the two protected resources it serves.
type ProbeTarget struct {
	// Issuer is the exact issuer identifier to qualify.
	Issuer string
	// Resource is the RFC 8707 indicator of the protected resource (Gist);
	// ResourceURL is an endpoint on it that answers 200 to a valid token and
	// 401 with a challenge otherwise.
	Resource    string
	ResourceURL string
	// OtherResource and OtherResourceURL are a second resource the same
	// issuer serves; its tokens must not be accepted by Resource.
	OtherResource    string
	OtherResourceURL string
	// Scope is granted to the test user; WiderScope is supported by the
	// issuer but never registered for the probe's narrow client.
	Scope      string
	WiderScope string
	// RedirectURI is an HTTPS client redirect URI. The probes never fetch it.
	RedirectURI string
	HTTP        *http.Client
	Consent     ConsentDriver
}

// Qualify runs every checklist probe against target and returns the result.
// A row passes only when every probe in it passes.
func Qualify(ctx context.Context, deployment, build string, strategy Strategy, target ProbeTarget) Qualification {
	p := &prober{t: target, ctx: ctx}
	hc := http.Client{Timeout: 0}
	if target.HTTP != nil {
		hc = *target.HTTP
	}
	hc.CheckRedirect = func(*http.Request, []*http.Request) error { return http.ErrUseLastResponse }
	p.http = &hc
	q := Qualification{Deployment: deployment, Build: build, Strategy: strategy, Issuer: target.Issuer}
	runs := map[RowID]func() []ProbeResult{
		RowDiscovery:         p.discovery,
		RowRegistration:      p.registration,
		RowAuthorizationCode: p.authorizationCode,
		RowRefresh:           p.refresh,
		RowProtectedResource: p.protectedResource,
		RowResourceAudience:  p.resourceAudience,
	}
	for _, row := range Checklist {
		probes := runs[row.ID]()
		status := StatusPass
		var reason string
		for _, pr := range probes {
			if !pr.Passed {
				status = StatusFail
				if reason == "" {
					reason = pr.Name + ": " + pr.Detail
				}
			}
		}
		if len(probes) == 0 {
			status, reason = StatusFail, "no probes ran"
		}
		q.Rows = append(q.Rows, RowResult{ID: row.ID, Requirement: row.Requirement, Status: status, Probes: probes, Reason: reason})
	}
	q.finish()
	return q
}

// ---- delegation decision --------------------------------------------------

// DelegationConfig is a deployment's delegated-issuer configuration.
type DelegationConfig struct {
	// Issuer is the configured external issuer; empty means none.
	Issuer string
	// Allowlist is the explicit set of issuers a deployment may delegate to.
	Allowlist []string
}

// InterfaceReceipt is an external dependency receipt (W1-W5).
type InterfaceReceipt struct {
	ID           string          `json:"-"`
	Status       string          `json:"status"`
	Revision     string          `json:"revision"`
	Interface    string          `json:"interface"`
	Evidence     json.RawMessage `json:"evidence"`
	Skipped      bool            `json:"skipped"`
	TestsSkipped int             `json:"tests_skipped"`
}

// RequiredReceipts are the interface receipts delegation depends on.
var RequiredReceipts = []string{"w1", "w2", "w3", "w4", "w5"}

func (r InterfaceReceipt) accepted() bool {
	switch r.Status {
	case "passed", "accepted", "verified":
	default:
		return false
	}
	ev := strings.TrimSpace(string(r.Evidence))
	return r.Revision != "" && r.Interface != "" && ev != "" && ev != "null" && ev != "{}" && ev != "[]" && ev != `""` && !r.Skipped && r.TestsSkipped == 0
}

// LoadReceipts reads <dir>/<id>.json for every required receipt. Missing
// files are simply absent from the result; malformed files are errors.
func LoadReceipts(dir string) (map[string]InterfaceReceipt, error) {
	out := map[string]InterfaceReceipt{}
	for _, id := range RequiredReceipts {
		raw, err := os.ReadFile(filepath.Join(dir, id+".json"))
		if errors.Is(err, os.ErrNotExist) {
			continue
		}
		if err != nil {
			return nil, fmt.Errorf("oauth: read receipt %s: %w", id, err)
		}
		var r InterfaceReceipt
		if err := json.Unmarshal(raw, &r); err != nil {
			return nil, fmt.Errorf("oauth: parse receipt %s: %w", id, err)
		}
		r.ID = id
		out[id] = r
	}
	return out, nil
}

// Selection is the deployment's issuance decision: exactly one issuer.
type Selection struct {
	Strategy    Strategy `json:"selected_strategy"`
	Issuer      string   `json:"selected_issuer"`
	FailingRows []RowID  `json:"rejected_rows,omitempty"`
	Reasons     []string `json:"reasons,omitempty"`
}

// ErrBuiltInNotQualified means the fallback itself fails the checklist, so
// the deployment has no acceptable issuer and must not start.
var ErrBuiltInNotQualified = errors.New("oauth: built-in issuer does not pass the section 10 checklist")

// SelectIssuer decides the issuance strategy. The built-in issuer must pass
// every row. Delegation is selected only when the configured issuer is
// allowlisted, every W1-W5 receipt is accepted, and its runtime
// qualification passes every row; otherwise the built-in issuer is selected
// and the reasons, including every failing row, are named. Receipts alone
// never qualify an issuer.
func SelectIssuer(builtIn Qualification, cfg DelegationConfig, receipts map[string]InterfaceReceipt, external *Qualification) (Selection, error) {
	if builtIn.Strategy != StrategyBuiltIn || !builtIn.Passed() {
		return Selection{}, fmt.Errorf("%w: failing rows %v", ErrBuiltInNotQualified, builtIn.failing())
	}
	fallback := Selection{Strategy: StrategyBuiltIn, Issuer: builtIn.Issuer}
	if cfg.Issuer == "" {
		fallback.Reasons = []string{"no delegated issuer is configured"}
		return fallback, nil
	}
	var reasons []string
	if !contains(cfg.Allowlist, cfg.Issuer) {
		reasons = append(reasons, "issuer "+cfg.Issuer+" is not in the delegation allowlist")
	}
	for _, id := range RequiredReceipts {
		r, ok := receipts[id]
		switch {
		case !ok:
			reasons = append(reasons, "interface receipt "+strings.ToUpper(id)+" is missing (external evidence pending)")
		case !r.accepted():
			reasons = append(reasons, "interface receipt "+strings.ToUpper(id)+" is not accepted")
		}
	}
	switch {
	case external == nil:
		reasons = append(reasons, "no runtime qualification exists for "+cfg.Issuer)
		fallback.FailingRows = allRows()
	case external.Issuer != cfg.Issuer || external.Strategy != StrategyDelegated:
		reasons = append(reasons, "runtime qualification is for "+external.Issuer+", not "+cfg.Issuer)
		fallback.FailingRows = allRows()
	default:
		fallback.FailingRows = external.failing()
		byID := map[RowID]RowResult{}
		for _, r := range external.Rows {
			byID[r.ID] = r
		}
		for _, id := range fallback.FailingRows {
			r, ok := byID[id]
			switch {
			case !ok:
				reasons = append(reasons, "checklist row "+string(id)+" was not run")
			case r.Reason != "":
				reasons = append(reasons, "checklist row "+string(id)+" "+string(r.Status)+": "+r.Reason)
			default:
				reasons = append(reasons, "checklist row "+string(id)+" "+string(r.Status))
			}
		}
	}
	if len(reasons) > 0 {
		fallback.Reasons = reasons
		return fallback, nil
	}
	return Selection{Strategy: StrategyDelegated, Issuer: cfg.Issuer}, nil
}

func allRows() []RowID {
	out := make([]RowID, 0, len(Checklist))
	for _, row := range Checklist {
		out = append(out, row.ID)
	}
	return out
}

// ChecklistDocument is the machine-readable per-deployment checklist record
// (issuer-checklist.json).
type ChecklistDocument struct {
	Schema            string             `json:"schema"`
	RequirementSource string             `json:"requirement_source"`
	Deployments       []DeploymentRecord `json:"deployments"`
}

// DeploymentRecord is one deployment's qualifications and selection.
type DeploymentRecord struct {
	Deployment     string          `json:"deployment"`
	Build          string          `json:"build"`
	Selection      Selection       `json:"selection"`
	Qualifications []Qualification `json:"qualifications"`
}

// ChecklistSchema identifies the issuer-checklist.json format.
const ChecklistSchema = "gist.issuer-checklist/v1"

// Validate checks that a recorded selection is consistent with its own
// qualifications: a delegated selection needs a passing delegated
// qualification for exactly that issuer and a passing built-in one.
func (d DeploymentRecord) Validate() error {
	var builtIn, delegated *Qualification
	for i := range d.Qualifications {
		q := &d.Qualifications[i]
		if want := q.failing(); fmt.Sprint(want) != fmt.Sprint(q.FailingRows) {
			return fmt.Errorf("deployment %s: %s failing_rows %v disagree with rows %v", d.Deployment, q.Strategy, q.FailingRows, want)
		}
		if (q.Status == QualificationQualified) != q.Passed() {
			return fmt.Errorf("deployment %s: %s status %q disagrees with its rows", d.Deployment, q.Strategy, q.Status)
		}
		switch q.Strategy {
		case StrategyBuiltIn:
			builtIn = q
		case StrategyDelegated:
			if delegated != nil {
				return fmt.Errorf("deployment %s: more than one delegated issuer", d.Deployment)
			}
			delegated = q
		}
	}
	if builtIn == nil || !builtIn.Passed() {
		return fmt.Errorf("deployment %s: built-in issuer is not qualified", d.Deployment)
	}
	switch d.Selection.Strategy {
	case StrategyBuiltIn:
		if d.Selection.Issuer != builtIn.Issuer {
			return fmt.Errorf("deployment %s: built-in selection names %q", d.Deployment, d.Selection.Issuer)
		}
	case StrategyDelegated:
		if delegated == nil || !delegated.Passed() || delegated.Issuer != d.Selection.Issuer {
			return fmt.Errorf("deployment %s: delegation selected without a passing qualification", d.Deployment)
		}
	default:
		return fmt.Errorf("deployment %s: unknown strategy %q", d.Deployment, d.Selection.Strategy)
	}
	return nil
}

// ---- probes ---------------------------------------------------------------

type prober struct {
	t    ProbeTarget
	ctx  context.Context
	http *http.Client
	md   map[string]any
}

type response struct {
	status int
	header http.Header
	body   []byte
	json   map[string]any
}

func (r response) str(key string) string {
	s, _ := r.json[key].(string)
	return s
}

func (p *prober) send(method, target string, body io.Reader, contentType string, header http.Header) (response, error) {
	req, err := http.NewRequestWithContext(p.ctx, method, target, body)
	if err != nil {
		return response{}, err
	}
	for k, v := range header {
		req.Header[k] = v
	}
	if contentType != "" {
		req.Header.Set("Content-Type", contentType)
	}
	resp, err := p.http.Do(req)
	if err != nil {
		return response{}, err
	}
	defer resp.Body.Close()
	raw, err := io.ReadAll(io.LimitReader(resp.Body, 1<<20))
	if err != nil {
		return response{}, err
	}
	out := response{status: resp.StatusCode, header: resp.Header, body: raw}
	_ = json.Unmarshal(raw, &out.json)
	return out, nil
}

func (p *prober) get(target string, header http.Header) (response, error) {
	return p.send(http.MethodGet, target, nil, "", header)
}

func (p *prober) form(target string, v url.Values) (response, error) {
	return p.send(http.MethodPost, target, strings.NewReader(v.Encode()), "application/x-www-form-urlencoded", nil)
}

func (p *prober) postJSON(target string, v any) (response, error) {
	raw, err := json.Marshal(v)
	if err != nil {
		return response{}, err
	}
	return p.send(http.MethodPost, target, bytes.NewReader(raw), "application/json", nil)
}

// check collects probe outcomes.
type check struct{ probes []ProbeResult }

func (c *check) add(name string, err error) {
	if err != nil {
		c.probes = append(c.probes, ProbeResult{Name: name, Passed: false, Detail: err.Error()})
		return
	}
	c.probes = append(c.probes, ProbeResult{Name: name, Passed: true})
}

func (p *prober) metadata() (map[string]any, error) {
	if p.md != nil {
		return p.md, nil
	}
	r, err := p.get(p.t.Issuer+PathASMetadata, nil)
	if err != nil {
		return nil, err
	}
	if r.status != http.StatusOK || r.json == nil {
		return nil, fmt.Errorf("authorization-server metadata answered %d", r.status)
	}
	p.md = r.json
	return p.md, nil
}

func (p *prober) endpoint(name string) (string, error) {
	md, err := p.metadata()
	if err != nil {
		return "", err
	}
	v, _ := md[name].(string)
	if v == "" {
		return "", fmt.Errorf("metadata has no %s", name)
	}
	return v, nil
}

func stringList(v any) []string {
	items, _ := v.([]any)
	out := make([]string, 0, len(items))
	for _, it := range items {
		if s, ok := it.(string); ok {
			out = append(out, s)
		}
	}
	return out
}

func (p *prober) discovery() []ProbeResult {
	var c check
	md, err := p.metadata()
	c.add("metadata-served", err)
	if err != nil {
		return c.probes
	}
	c.add("issuer-matches", func() error {
		if got, _ := md["issuer"].(string); got != p.t.Issuer {
			return fmt.Errorf("issuer %q, want %q", got, p.t.Issuer)
		}
		return nil
	}())
	c.add("endpoints-https", func() error {
		for _, name := range []string{"authorization_endpoint", "token_endpoint", "registration_endpoint", "revocation_endpoint"} {
			v, err := p.endpoint(name)
			if err != nil {
				return err
			}
			if err := httpsURL(v); err != nil {
				return fmt.Errorf("%s %q: %w", name, v, err)
			}
		}
		return nil
	}())
	c.add("endpoints-live", func() error {
		// Each advertised endpoint must be a real OAuth endpoint: a bad
		// request gets an OAuth error, never 404/405/5xx.
		tok, err := p.endpoint("token_endpoint")
		if err != nil {
			return err
		}
		r, err := p.form(tok, url.Values{"grant_type": {"authorization_code"}})
		if err != nil {
			return err
		}
		if r.status != http.StatusBadRequest && r.status != http.StatusUnauthorized || r.str("error") == "" {
			return fmt.Errorf("token endpoint answered %d without an OAuth error", r.status)
		}
		reg, err := p.endpoint("registration_endpoint")
		if err != nil {
			return err
		}
		r, err = p.postJSON(reg, map[string]any{})
		if err != nil {
			return err
		}
		if r.status != http.StatusBadRequest || r.str("error") == "" {
			return fmt.Errorf("registration endpoint answered %d without an OAuth error", r.status)
		}
		rev, err := p.endpoint("revocation_endpoint")
		if err != nil {
			return err
		}
		r, err = p.form(rev, url.Values{})
		if err != nil {
			return err
		}
		if r.status == http.StatusNotFound || r.status == http.StatusMethodNotAllowed || r.status >= 500 {
			return fmt.Errorf("revocation endpoint answered %d", r.status)
		}
		az, err := p.endpoint("authorization_endpoint")
		if err != nil {
			return err
		}
		r, err = p.get(az, nil)
		if err != nil {
			return err
		}
		if r.status == http.StatusNotFound || r.status == http.StatusMethodNotAllowed || r.status >= 500 {
			return fmt.Errorf("authorization endpoint answered %d", r.status)
		}
		return nil
	}())
	c.add("pkce-s256-only", func() error {
		methods := stringList(md["code_challenge_methods_supported"])
		if len(methods) != 1 || methods[0] != "S256" {
			return fmt.Errorf("code_challenge_methods_supported = %v, want exactly [S256]", methods)
		}
		return nil
	}())
	c.add("code-and-refresh-grants", func() error {
		grants := stringList(md["grant_types_supported"])
		if !contains(grants, "authorization_code") || !contains(grants, "refresh_token") {
			return fmt.Errorf("grant_types_supported = %v", grants)
		}
		if rt := stringList(md["response_types_supported"]); !contains(rt, "code") {
			return fmt.Errorf("response_types_supported = %v", rt)
		}
		return nil
	}())
	c.add("resource-advertised", func() error {
		// RFC 9728 section 4: an AS that lists protected resources must list
		// the one being qualified. Behavior is probed in the audience row.
		if v, ok := md["protected_resources"]; ok && !contains(stringList(v), p.t.Resource) {
			return fmt.Errorf("protected_resources %v omits %s", stringList(v), p.t.Resource)
		}
		return nil
	}())
	return c.probes
}

// register registers a client with the given redirect URIs and optional
// scope, returning the response.
func (p *prober) register(redirects []string, scope *string) (response, error) {
	reg, err := p.endpoint("registration_endpoint")
	if err != nil {
		return response{}, err
	}
	body := map[string]any{"redirect_uris": redirects, "client_name": "Gist issuer qualification", "token_endpoint_auth_method": "none", "grant_types": []string{"authorization_code", "refresh_token"}, "response_types": []string{"code"}}
	if scope != nil {
		body["scope"] = *scope
	}
	return p.postJSON(reg, body)
}

func (p *prober) altRedirect() string { return p.t.RedirectURI + "/alt" }

func (p *prober) client(scope string) (string, error) {
	var sp *string
	if scope != "" {
		sp = &scope
	}
	r, err := p.register([]string{p.t.RedirectURI, p.altRedirect()}, sp)
	if err != nil {
		return "", err
	}
	if r.status != http.StatusCreated || r.str("client_id") == "" {
		return "", fmt.Errorf("registration answered %d %s", r.status, r.body)
	}
	return r.str("client_id"), nil
}

func expectRejected(r response, err error, what string, codes ...string) error {
	if err != nil {
		return err
	}
	if r.status < 400 || r.status >= 500 {
		return fmt.Errorf("%s answered %d, want a 4xx rejection", what, r.status)
	}
	if _, ok := r.json["client_id"]; ok {
		return fmt.Errorf("%s issued a client", what)
	}
	if _, ok := r.json["access_token"]; ok {
		return fmt.Errorf("%s issued an access token", what)
	}
	if len(codes) > 0 && !contains(codes, r.str("error")) {
		return fmt.Errorf("%s error %q, want one of %v", what, r.str("error"), codes)
	}
	return nil
}

func (p *prober) registration() []ProbeResult {
	var c check
	c.add("https-redirect-registered", func() error {
		r, err := p.register([]string{p.t.RedirectURI}, nil)
		if err != nil {
			return err
		}
		if r.status != http.StatusCreated || r.str("client_id") == "" {
			return fmt.Errorf("registration answered %d %s", r.status, r.body)
		}
		if got := stringList(r.json["redirect_uris"]); len(got) != 1 || got[0] != p.t.RedirectURI {
			return fmt.Errorf("redirect_uris echoed as %v", got)
		}
		return nil
	}())
	c.add("http-redirect-rejected", func() error {
		insecure := "http://" + strings.TrimPrefix(p.t.RedirectURI, "https://")
		r, err := p.register([]string{insecure}, nil)
		return expectRejected(r, err, "plain-HTTP redirect registration", "invalid_redirect_uri", "invalid_client_metadata")
	}())
	c.add("unknown-scope-rejected-not-downgraded", func() error {
		scope := p.t.Scope + " gist:unsupported-qualification-scope"
		r, err := p.register([]string{p.t.RedirectURI}, &scope)
		return expectRejected(r, err, "registration with an unsupported scope", "invalid_client_metadata", "invalid_scope")
	}())
	c.add("excess-scope-rejected-not-downgraded", func() error {
		// A client registered for Scope that asks for Scope+WiderScope gets
		// invalid_scope, not a narrowed code.
		clientID, err := p.client(p.t.Scope)
		if err != nil {
			return err
		}
		_, challenge := newPKCE()
		out, err := p.authorize(clientID, p.t.RedirectURI, p.t.Resource, p.t.Scope+" "+p.t.WiderScope, "S256", challenge, true)
		if err != nil {
			return err
		}
		if out.code != "" || out.err != "invalid_scope" {
			return fmt.Errorf("excess scope gave code=%t error=%q, want invalid_scope", out.code != "", out.err)
		}
		return nil
	}())
	return c.probes
}

type authResult struct {
	code, err, state string
}

func newPKCE() (verifier, challenge string) {
	b := make([]byte, 32)
	_, _ = rand.Read(b)
	verifier = base64.RawURLEncoding.EncodeToString(b)
	sum := sha256.Sum256([]byte(verifier))
	return verifier, base64.RawURLEncoding.EncodeToString(sum[:])
}

// authorize drives one authorization request through the consent driver.
// An empty method or challenge omits that parameter.
func (p *prober) authorize(clientID, redirect, resource, scope, method, challenge string, approve bool) (authResult, error) {
	az, err := p.endpoint("authorization_endpoint")
	if err != nil {
		return authResult{}, err
	}
	u, err := url.Parse(az)
	if err != nil {
		return authResult{}, err
	}
	state, err := randomID(12)
	if err != nil {
		return authResult{}, err
	}
	q := u.Query()
	q.Set("response_type", "code")
	q.Set("client_id", clientID)
	q.Set("redirect_uri", redirect)
	q.Set("state", state)
	if resource != "" {
		q.Set("resource", resource)
	}
	if scope != "" {
		q.Set("scope", scope)
	}
	if method != "" {
		q.Set("code_challenge_method", method)
	}
	if challenge != "" {
		q.Set("code_challenge", challenge)
	}
	u.RawQuery = q.Encode()
	final, err := p.t.Consent.Authorize(p.ctx, u.String(), approve)
	if err != nil {
		return authResult{}, err
	}
	want, _ := url.Parse(redirect)
	if final.Scheme != want.Scheme || final.Host != want.Host || final.Path != want.Path {
		return authResult{}, fmt.Errorf("issuer redirected to %s://%s%s, want %s", final.Scheme, final.Host, final.Path, redirect)
	}
	fq := final.Query()
	if iss := fq.Get("iss"); iss != "" && iss != p.t.Issuer {
		return authResult{}, fmt.Errorf("authorization response iss %q, want %q", iss, p.t.Issuer)
	}
	out := authResult{code: fq.Get("code"), err: fq.Get("error"), state: fq.Get("state")}
	if out.state != state {
		return authResult{}, fmt.Errorf("authorization response state %q, want the request state", out.state)
	}
	if out.code != "" && out.err != "" {
		return authResult{}, errors.New("authorization response carries both code and error")
	}
	return out, nil
}

// codeFor runs an approved authorization and returns a code and verifier.
func (p *prober) codeFor(clientID, redirect, resource string) (string, string, error) {
	verifier, challenge := newPKCE()
	out, err := p.authorize(clientID, redirect, resource, p.t.Scope, "S256", challenge, true)
	if err != nil {
		return "", "", err
	}
	if out.code == "" {
		return "", "", fmt.Errorf("approved consent returned error %q, no code", out.err)
	}
	return out.code, verifier, nil
}

func (p *prober) exchange(clientID, code, redirect, verifier string, extra url.Values) (response, error) {
	tok, err := p.endpoint("token_endpoint")
	if err != nil {
		return response{}, err
	}
	form := url.Values{"grant_type": {"authorization_code"}, "client_id": {clientID}, "code": {code}, "redirect_uri": {redirect}, "code_verifier": {verifier}}
	for k, v := range extra {
		form[k] = v
	}
	return p.form(tok, form)
}

func (p *prober) refreshWith(clientID, refresh string, extra url.Values) (response, error) {
	tok, err := p.endpoint("token_endpoint")
	if err != nil {
		return response{}, err
	}
	form := url.Values{"grant_type": {"refresh_token"}, "client_id": {clientID}, "refresh_token": {refresh}}
	for k, v := range extra {
		form[k] = v
	}
	return p.form(tok, form)
}

func tokensOf(r response, err error, what string) (access, refresh string, e error) {
	if err != nil {
		return "", "", err
	}
	if r.status != http.StatusOK || r.str("access_token") == "" {
		return "", "", fmt.Errorf("%s answered %d %s", what, r.status, r.body)
	}
	if r.header.Get("Cache-Control") != "no-store" {
		return "", "", fmt.Errorf("%s response is cacheable (Cache-Control %q)", what, r.header.Get("Cache-Control"))
	}
	return r.str("access_token"), r.str("refresh_token"), nil
}

type grant struct {
	clientID, access, refresh, scope string
}

func (p *prober) grant(resource string) (grant, error) {
	clientID, err := p.client("")
	if err != nil {
		return grant{}, err
	}
	code, verifier, err := p.codeFor(clientID, p.t.RedirectURI, resource)
	if err != nil {
		return grant{}, err
	}
	r, err := p.exchange(clientID, code, p.t.RedirectURI, verifier, nil)
	access, refresh, err := tokensOf(r, err, "code exchange")
	if err != nil {
		return grant{}, err
	}
	if refresh == "" {
		return grant{}, errors.New("code exchange issued no refresh token")
	}
	return grant{clientID: clientID, access: access, refresh: refresh, scope: r.str("scope")}, nil
}

func (p *prober) authorizationCode() []ProbeResult {
	var c check
	clientID, err := p.client("")
	c.add("client-registered", err)
	if err != nil {
		return c.probes
	}
	rejectedAtAuthorize := func(method string, withChallenge bool) error {
		_, challenge := newPKCE()
		if method == "plain" {
			challenge = strings.Repeat("A", 43)
		}
		if !withChallenge {
			challenge = ""
		}
		out, err := p.authorize(clientID, p.t.RedirectURI, p.t.Resource, p.t.Scope, method, challenge, true)
		if err != nil {
			return err
		}
		if out.code != "" || out.err == "" {
			return fmt.Errorf("authorization issued code=%t error=%q", out.code != "", out.err)
		}
		return nil
	}
	c.add("pkce-plain-rejected", rejectedAtAuthorize("plain", true))
	c.add("pkce-challenge-required", rejectedAtAuthorize("", false))
	c.add("consent-denial-issues-no-grant", func() error {
		_, challenge := newPKCE()
		out, err := p.authorize(clientID, p.t.RedirectURI, p.t.Resource, p.t.Scope, "S256", challenge, false)
		if err != nil {
			return err
		}
		if out.code != "" || out.err != "access_denied" {
			return fmt.Errorf("denied consent gave code=%t error=%q, want access_denied", out.code != "", out.err)
		}
		return nil
	}())
	negative := func(name string, mutate func(clientID, code, redirect, verifier string) (response, error)) {
		c.add(name, func() error {
			code, verifier, err := p.codeFor(clientID, p.t.RedirectURI, p.t.Resource)
			if err != nil {
				return err
			}
			r, err := mutate(clientID, code, p.t.RedirectURI, verifier)
			return expectRejected(r, err, "token request", "invalid_grant")
		}())
	}
	negative("wrong-verifier-rejected", func(id, code, redirect, _ string) (response, error) {
		other, _ := newPKCE()
		return p.exchange(id, code, redirect, other, nil)
	})
	negative("redirect-mismatch-rejected", func(id, code, _, verifier string) (response, error) {
		return p.exchange(id, code, p.altRedirect(), verifier, nil)
	})
	negative("stolen-code-other-client-rejected", func(_, code, redirect, verifier string) (response, error) {
		thief, err := p.client("")
		if err != nil {
			return response{}, err
		}
		return p.exchange(thief, code, redirect, verifier, nil)
	})
	c.add("code-single-use", func() error {
		code, verifier, err := p.codeFor(clientID, p.t.RedirectURI, p.t.Resource)
		if err != nil {
			return err
		}
		r, err := p.exchange(clientID, code, p.t.RedirectURI, verifier, nil)
		if _, _, err := tokensOf(r, err, "first redemption"); err != nil {
			return err
		}
		r, err = p.exchange(clientID, code, p.t.RedirectURI, verifier, nil)
		return expectRejected(r, err, "code replay", "invalid_grant")
	}())
	return c.probes
}

// callResource presents token at target and returns status and challenge.
func (p *prober) callResource(target, token string) (int, string, error) {
	h := http.Header{}
	if token != "" {
		h.Set("Authorization", "Bearer "+token)
	}
	r, err := p.get(target, h)
	if err != nil {
		return 0, "", err
	}
	return r.status, r.header.Get("WWW-Authenticate"), nil
}

func (p *prober) accepted(target, token, what string) error {
	status, _, err := p.callResource(target, token)
	if err != nil {
		return err
	}
	if status != http.StatusOK {
		return fmt.Errorf("%s answered %d, want 200", what, status)
	}
	return nil
}

func (p *prober) rejectedInvalidToken(target, token, what string) error {
	status, challenge, err := p.callResource(target, token)
	if err != nil {
		return err
	}
	if status != http.StatusUnauthorized || !strings.HasPrefix(challenge, "Bearer ") || !strings.Contains(challenge, `error="invalid_token"`) {
		return fmt.Errorf("%s answered %d %q, want 401 Bearer error=\"invalid_token\"", what, status, challenge)
	}
	return nil
}

func (p *prober) refresh() []ProbeResult {
	var c check
	g, err := p.grant(p.t.Resource)
	c.add("grant-issued", err)
	if err != nil {
		return c.probes
	}
	var r1 string
	c.add("refresh-rotates-and-preserves-bounds", func() error {
		r, err := p.refreshWith(g.clientID, g.refresh, nil)
		access, next, err := tokensOf(r, err, "refresh")
		if err != nil {
			return err
		}
		if next == "" || next == g.refresh {
			return errors.New("refresh did not rotate the refresh token")
		}
		r1 = next
		if got := r.str("scope"); got != "" && got != g.scope {
			return fmt.Errorf("refreshed scope %q, want %q", got, g.scope)
		}
		if err := p.accepted(p.t.ResourceURL, access, "refreshed token at its resource"); err != nil {
			return err
		}
		return p.rejectedInvalidToken(p.t.OtherResourceURL, access, "refreshed token at the other resource")
	}())
	c.add("refresh-rejects-widening", func() error {
		w, err := p.grant(p.t.Resource)
		if err != nil {
			return err
		}
		r, err := p.refreshWith(w.clientID, w.refresh, url.Values{"scope": {p.t.Scope + " " + p.t.WiderScope}})
		return expectRejected(r, err, "widening refresh", "invalid_scope", "invalid_grant")
	}())
	c.add("reused-refresh-rejected", func() error {
		r, err := p.refreshWith(g.clientID, g.refresh, nil)
		return expectRejected(r, err, "reused refresh token", "invalid_grant")
	}())
	c.add("reuse-revokes-family", func() error {
		if r1 == "" {
			return errors.New("no rotated token to test")
		}
		r, err := p.refreshWith(g.clientID, r1, nil)
		return expectRejected(r, err, "refresh from a family after reuse", "invalid_grant")
	}())
	c.add("revoked-consent-rejected", func() error {
		v, err := p.grant(p.t.Resource)
		if err != nil {
			return err
		}
		if err := p.t.Consent.RevokeConsent(p.ctx); err != nil {
			return fmt.Errorf("revoke consent: %w", err)
		}
		r, err := p.refreshWith(v.clientID, v.refresh, nil)
		rerr := expectRejected(r, err, "refresh after revoked consent", "invalid_grant")
		if err := p.t.Consent.RestoreConsent(p.ctx); err != nil {
			return fmt.Errorf("restore consent: %w", err)
		}
		if rerr != nil {
			return rerr
		}
		r, err = p.refreshWith(v.clientID, v.refresh, nil)
		return expectRejected(r, err, "refresh after consent was restored", "invalid_grant")
	}())
	return c.probes
}

var resourceMetadataParam = regexp.MustCompile(`resource_metadata="([^"]+)"`)

func (p *prober) protectedResource() []ProbeResult {
	var c check
	var prmLocation string
	c.add("challenge-points-to-prm", func() error {
		status, challenge, err := p.callResource(p.t.ResourceURL, "")
		if err != nil {
			return err
		}
		if status != http.StatusUnauthorized || !strings.HasPrefix(challenge, "Bearer ") {
			return fmt.Errorf("anonymous request answered %d %q", status, challenge)
		}
		m := resourceMetadataParam.FindStringSubmatch(challenge)
		if m == nil {
			return fmt.Errorf("challenge %q has no resource_metadata", challenge)
		}
		if want := prmURL(p.t.Resource); m[1] != want {
			return fmt.Errorf("challenge resource_metadata %q, want the RFC 9728 document %q", m[1], want)
		}
		prmLocation = m[1]
		return nil
	}())
	var prm ProtectedResourceMetadata
	c.add("prm-document", func() error {
		target := prmLocation
		if target == "" {
			target = prmURL(p.t.Resource)
		}
		r, err := p.get(target, nil)
		if err != nil {
			return err
		}
		if r.status != http.StatusOK || json.Unmarshal(r.body, &prm) != nil {
			return fmt.Errorf("protected-resource metadata answered %d", r.status)
		}
		switch {
		case prm.Resource != p.t.Resource:
			return fmt.Errorf("PRM resource %q, want %q", prm.Resource, p.t.Resource)
		case !contains(prm.AuthorizationServers, p.t.Issuer):
			return fmt.Errorf("PRM authorization_servers %v omit %s", prm.AuthorizationServers, p.t.Issuer)
		case !containsAll(prm.ScopesSupported, []string{p.t.Scope, p.t.WiderScope}):
			return fmt.Errorf("PRM scopes_supported %v omit the catalog scopes", prm.ScopesSupported)
		case !contains(prm.BearerMethodsSupported, "header"):
			return fmt.Errorf("PRM bearer_methods_supported %v omit header", prm.BearerMethodsSupported)
		}
		return nil
	}())
	c.add("as-discoverable-from-prm", func() error {
		if !contains(prm.AuthorizationServers, p.t.Issuer) {
			return errors.New("PRM does not name the issuer")
		}
		r, err := p.get(strings.TrimSuffix(p.t.Issuer, "/")+PathASMetadata, nil)
		if err != nil {
			return err
		}
		if r.status != http.StatusOK || r.str("issuer") != p.t.Issuer {
			return fmt.Errorf("AS metadata from PRM answered %d issuer %q", r.status, r.str("issuer"))
		}
		return nil
	}())
	return c.probes
}

func (p *prober) resourceAudience() []ProbeResult {
	var c check
	c.add("unknown-resource-rejected", func() error {
		clientID, err := p.client("")
		if err != nil {
			return err
		}
		_, challenge := newPKCE()
		out, err := p.authorize(clientID, p.t.RedirectURI, "https://unlisted-resource.invalid", p.t.Scope, "S256", challenge, true)
		if err != nil {
			return err
		}
		if out.code != "" || out.err == "" {
			return fmt.Errorf("unknown resource gave code=%t error=%q", out.code != "", out.err)
		}
		return nil
	}())
	a, errA := p.grant(p.t.Resource)
	c.add("token-accepted-at-bound-resource", func() error {
		if errA != nil {
			return errA
		}
		return p.accepted(p.t.ResourceURL, a.access, "resource token at its resource")
	}())
	c.add("token-rejected-at-other-resource", func() error {
		if errA != nil {
			return errA
		}
		return p.rejectedInvalidToken(p.t.OtherResourceURL, a.access, "resource token at the other resource")
	}())
	c.add("other-resource-token-rejected-here", func() error {
		b, err := p.grant(p.t.OtherResource)
		if err != nil {
			return err
		}
		if err := p.accepted(p.t.OtherResourceURL, b.access, "other-resource token at its resource"); err != nil {
			return err
		}
		return p.rejectedInvalidToken(p.t.ResourceURL, b.access, "other-resource token at the protected resource")
	}())
	c.add("redemption-resource-bound", func() error {
		clientID, err := p.client("")
		if err != nil {
			return err
		}
		code, verifier, err := p.codeFor(clientID, p.t.RedirectURI, p.t.Resource)
		if err != nil {
			return err
		}
		r, err := p.exchange(clientID, code, p.t.RedirectURI, verifier, url.Values{"resource": {p.t.OtherResource}})
		return expectRejected(r, err, "redemption for another resource", "invalid_target", "invalid_grant")
	}())
	c.add("refresh-resource-bound", func() error {
		if errA != nil {
			return errA
		}
		r, err := p.refreshWith(a.clientID, a.refresh, url.Values{"resource": {p.t.OtherResource}})
		if err := expectRejected(r, err, "refresh for another resource", "invalid_target", "invalid_grant"); err != nil {
			return err
		}
		// A refresh that does not name a resource keeps the original audience.
		b, err := p.grant(p.t.Resource)
		if err != nil {
			return err
		}
		r, err = p.refreshWith(b.clientID, b.refresh, nil)
		access, _, err := tokensOf(r, err, "refresh")
		if err != nil {
			return err
		}
		if err := p.accepted(p.t.ResourceURL, access, "refreshed token at its resource"); err != nil {
			return err
		}
		return p.rejectedInvalidToken(p.t.OtherResourceURL, access, "refreshed token at the other resource")
	}())
	return c.probes
}

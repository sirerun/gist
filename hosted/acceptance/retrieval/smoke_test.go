package retrieval

// E2 retrieval smoke runner. It drives the frozen E1 suite through the real
// discover, batch-get and resolve HTTP boundaries of a target (the in-process
// local composition by default, a deployed base URL under -tags=live), scores
// every case with metrics_test.go, and writes a reproducible report.

import (
	"bytes"
	"context"
	"encoding/json"
	"flag"
	"fmt"
	"io"
	"net/http"
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"
)

var writeBaseline = flag.Bool("e2.write-baseline", false, "write the measured report to eval/registry/baseline.json")
var forceThresholds = flag.Bool("e2.enforce-thresholds", false, "fail the run when an E1 threshold fails (always on under -tags=live)")

// adversaryOwnerKey is the principal that legitimately owns the adversary
// workspace. It proves the adversaries are real, high-scoring records.
const adversaryOwnerKey = "principal-adversary-owner"

// target is one service under test. Both the local composition and the live
// deployment satisfy it; nothing else in the runner knows which one it has.
type target struct {
	name        string
	baseURL     string
	client      *http.Client
	tokens      map[string]string // principal fixture key -> bearer token
	catalogHash string
	configHash  string
	close       func()
}

var (
	sharedTarget *target
	targetErr    error
)

func TestMain(m *testing.M) {
	flag.Parse()
	if suite, err := loadFrozenSuite(evalDir); err != nil {
		targetErr = fmt.Errorf("frozen suite: %w", err)
	} else {
		sharedTarget, targetErr = openTarget(suite)
	}
	code := m.Run()
	if sharedTarget != nil && sharedTarget.close != nil {
		sharedTarget.close()
	}
	os.Exit(code)
}

func requireTarget(t *testing.T) (frozenSuite, *target) {
	t.Helper()
	suite, err := loadFrozenSuite(evalDir)
	if err != nil {
		t.Fatalf("frozen E1 suite rejected; no case executed: %v", err)
	}
	if targetErr != nil {
		t.Fatalf("retrieval smoke environment is required and unavailable; all %d cases failed to execute: %s: %v", len(suite.cases), strings.Join(caseIDs(suite), ", "), targetErr)
	}
	if sharedTarget == nil {
		t.Fatal("retrieval smoke target was not opened")
	}
	for _, p := range suite.corpus.Principals {
		if sharedTarget.tokens[p.Key] == "" {
			t.Fatalf("target has no credential for principal %s", p.Key)
		}
	}
	return suite, sharedTarget
}

func caseIDs(s frozenSuite) []string {
	out := make([]string, len(s.cases))
	for i, c := range s.cases {
		out[i] = c.CaseID
	}
	return out
}

// TestRetrievalSmoke runs all 24 frozen cases against the target.
func TestRetrievalSmoke(t *testing.T) {
	suite, tgt := requireTarget(t)
	ctx, cancel := context.WithTimeout(context.Background(), 2*time.Minute)
	defer cancel()

	probeAdversaryIsHighScore(ctx, t, suite, tgt)

	obs := runSuite(ctx, tgt, tgt.client, suite)
	rep, err := evaluate(suite, obs)
	if err != nil {
		t.Fatalf("evaluate smoke run: %v", err)
	}
	if err := stampReport(&rep, suite, tgt); err != nil {
		t.Fatalf("hash report inputs: %v", err)
	}
	path := persistReport(t, rep)
	logReport(t, rep, path)

	for _, failure := range invariantFailures(suite, rep) {
		t.Errorf("E2 invariant failed: %s", failure)
	}
	thresholdMsgs := thresholdFailures(rep)
	for _, msg := range thresholdMsgs {
		if enforceThresholds || *forceThresholds {
			t.Errorf("E1 threshold failed: %s", msg)
		} else {
			t.Logf("E1 threshold FAIL (reported, enforced by E3 / -e2.enforce-thresholds): %s", msg)
		}
	}

	baselinePath := filepath.Join(evalDir, "baseline.json")
	if *writeBaseline {
		if err := writeJSONFile(baselinePath, rep); err != nil {
			t.Fatalf("write baseline: %v", err)
		}
		t.Logf("wrote baseline %s", baselinePath)
		return
	}
	raw, err := os.ReadFile(baselinePath)
	if err != nil {
		t.Fatalf("baseline is required for comparison: %v", err)
	}
	var base report
	if err := json.Unmarshal(raw, &base); err != nil {
		t.Fatalf("decode baseline: %v", err)
	}
	for _, msg := range compareBaseline(base, rep) {
		t.Errorf("baseline comparison: %s", msg)
	}
}

// TestRetrievalNegativeControl proves the checker is not vacuous: the same
// run, with a transport that lets the deliberately unauthorized high-score
// adversary through discover (as a broken authorization filter would), must
// fail and name every affected case.
func TestRetrievalNegativeControl(t *testing.T) {
	suite, tgt := requireTarget(t)
	ctx, cancel := context.WithTimeout(context.Background(), 2*time.Minute)
	defer cancel()
	leaky := &http.Client{Timeout: tgt.client.Timeout, Transport: leakInjector{next: transportOf(tgt.client)}}
	obs := runSuite(ctx, tgt, leaky, suite)
	rep, err := evaluate(suite, obs)
	if err != nil {
		t.Fatalf("evaluate negative control: %v", err)
	}
	failures := strings.Join(invariantFailures(suite, rep), "\n")
	if failures == "" {
		t.Fatal("negative control: checker passed a run with an injected unauthorized candidate")
	}
	for _, c := range suite.cases {
		if !strings.Contains(failures, c.CaseID) {
			t.Errorf("negative control: failure output does not name case %s", c.CaseID)
		}
	}
	if rep.Aggregate.LeakCount < len(suite.cases) {
		t.Errorf("negative control: leak count %d, want >= %d", rep.Aggregate.LeakCount, len(suite.cases))
	}
	if tr := rep.Thresholds["unauthorized_leakage_rate"]; tr.Verdict != "FAIL" || tr.Measured != 1 {
		t.Errorf("negative control: leakage threshold %s measured %.4f, want FAIL at 1.0", tr.Verdict, tr.Measured)
	}
	t.Logf("negative control: checker FAILED as required (leak_count=%d, leakage_rate=%.4f): %s", rep.Aggregate.LeakCount, rep.Thresholds["unauthorized_leakage_rate"].Measured, firstLine(failures))
}

// probeAdversaryIsHighScore asserts that the adversary for every query is a
// real published record ranked first for its legitimate owner, so that its
// absence from smoke results is an authorization outcome, not a miss.
func probeAdversaryIsHighScore(ctx context.Context, t *testing.T, s frozenSuite, tgt *target) {
	t.Helper()
	token := tgt.tokens[adversaryOwnerKey]
	if token == "" {
		t.Fatalf("target has no credential for %s", adversaryOwnerKey)
	}
	seen := map[string]bool{}
	for _, c := range s.cases {
		if seen[c.Query] {
			continue
		}
		seen[c.Query] = true
		var out struct {
			Items []struct {
				Ref observedRef `json:"Ref"`
			} `json:"items"`
		}
		call := post(ctx, tgt, tgt.client, token, "/v1/discover", map[string]any{"query": c.Query, "max_results": discoverMaxResults, "max_bytes": c.MaxBytes}, "discover", &out)
		want := adversaryPrefix + strings.ToLower(c.Query)
		present := false
		for _, item := range out.Items {
			present = present || item.Ref.ID == want
		}
		// Adversaries sharing a substring tie on score, so the check is that an
		// adversary ranks first and this query's adversary is returned.
		if call.Status != http.StatusOK || len(out.Items) == 0 || !strings.HasPrefix(out.Items[0].Ref.ID, adversaryPrefix) || !present {
			t.Fatalf("case %s: adversary %q is not a top-ranked result for its owner (status %d, %d items, err %s)", c.CaseID, want, call.Status, len(out.Items), call.Err)
		}
	}
}

// runSuite executes every case in order. It never skips: a case whose calls
// fail still yields an observation carrying the failure.
func runSuite(ctx context.Context, tgt *target, client *http.Client, s frozenSuite) []observation {
	out := make([]observation, 0, len(s.cases))
	for _, c := range s.cases {
		o := runCase(ctx, tgt, client, c)
		probeCase(ctx, tgt, client, s, c, &o)
		out = append(out, o)
	}
	return out
}

// probeCase fetches the case's labeled relevant items and its forbidden
// foreign-workspace items directly through batch-get, then resolves the
// relevant skills. Relevant items must come back; forbidden ones must not.
func probeCase(ctx context.Context, tgt *target, client *http.Client, s frozenSuite, c smokeCase, o *observation) {
	workspaceID := map[string]string{}
	for _, w := range s.corpus.Workspaces {
		workspaceID[w.Key] = w.ID
	}
	byLabel := map[string]corpusArtifact{}
	l, err := newLabeler(s.corpus)
	if err != nil {
		o.ProbeTransportE = err.Error()
		return
	}
	for _, a := range s.corpus.Artifacts {
		byLabel[l.labelByID[a.ID]] = a
	}
	var refs []map[string]string
	add := func(label, workspace string) bool {
		a, ok := byLabel[label]
		if !ok {
			return false
		}
		refs = append(refs, map[string]string{"WorkspaceID": workspace, "Kind": a.Kind, "ID": a.ID, "Version": a.Version})
		return true
	}
	for _, label := range c.Relevant {
		if add(label, workspaceID[c.WorkspaceKey]) {
			o.ProbeRequested = append(o.ProbeRequested, label)
		}
	}
	for _, label := range c.Forbidden {
		base, key, foreign := strings.Cut(label, "@")
		if foreign && workspaceID[key] != "" && add(base, workspaceID[key]) {
			o.ProbeForbidden = append(o.ProbeForbidden, label)
		}
	}
	if len(refs) == 0 {
		return
	}
	token := tgt.tokens[c.PrincipalKey]
	var batch struct {
		Items []struct {
			Artifact *struct {
				Ref observedRef `json:"Ref"`
			} `json:"artifact"`
		} `json:"items"`
	}
	call := post(ctx, tgt, client, token, "/v1/artifacts/batch-get", map[string]any{"references": refs, "max_bytes": c.MaxBytes}, "probe-batch-get", &batch)
	o.ProbeCalls = append(o.ProbeCalls, call)
	if call.Status == 0 {
		o.ProbeTransportE = call.Err
		return
	}
	for _, item := range batch.Items {
		if item.Artifact != nil {
			o.ProbeFetched = append(o.ProbeFetched, item.Artifact.Ref)
		}
	}
	for _, label := range o.ProbeRequested {
		a := byLabel[label]
		if a.Kind != "skill" {
			continue
		}
		var resolved struct {
			Status string `json:"status"`
		}
		call := post(ctx, tgt, client, token, "/v1/resolve", map[string]any{"skill": map[string]string{"Kind": "skill", "ID": a.ID, "Version": a.Version}, "runtime_id": runtimeID, "local_execution": true, "max_bytes": c.MaxBytes}, "probe-resolve", &resolved)
		o.ProbeCalls = append(o.ProbeCalls, call)
		if call.Status == 0 {
			o.ProbeTransportE = call.Err
			return
		}
		if call.Status != http.StatusOK {
			o.ProbeResolve = append(o.ProbeResolve, fmt.Sprintf("http_%d", call.Status))
			continue
		}
		o.ProbeResolve = append(o.ProbeResolve, resolved.Status)
	}
}

func runCase(ctx context.Context, tgt *target, client *http.Client, c smokeCase) observation {
	o := observation{CaseID: c.CaseID}
	token := tgt.tokens[c.PrincipalKey]
	var discovered struct {
		Items []struct {
			Ref observedRef `json:"Ref"`
		} `json:"items"`
	}
	call := post(ctx, tgt, client, token, "/v1/discover", map[string]any{"query": c.Query, "max_results": discoverMaxResults, "max_bytes": c.MaxBytes}, "discover", &discovered)
	o.Calls = append(o.Calls, call)
	if call.Status == 0 {
		o.TransportError = call.Err
		return o
	}
	for _, item := range discovered.Items {
		o.Candidates = append(o.Candidates, item.Ref)
	}
	if len(o.Candidates) == 0 {
		return o
	}
	top := o.Candidates
	if len(top) > topK {
		top = top[:topK]
	}
	refs := make([]map[string]string, 0, len(top))
	for _, ref := range top {
		refs = append(refs, map[string]string{"WorkspaceID": ref.WorkspaceID, "Kind": ref.Kind, "ID": ref.ID, "Version": ref.Version})
	}
	var batch struct {
		Items []struct {
			Artifact *struct {
				Ref observedRef `json:"Ref"`
			} `json:"artifact"`
		} `json:"items"`
	}
	call = post(ctx, tgt, client, token, "/v1/artifacts/batch-get", map[string]any{"references": refs, "max_bytes": c.MaxBytes}, "batch-get", &batch)
	o.Calls = append(o.Calls, call)
	if call.Status == 0 {
		o.TransportError = call.Err
		return o
	}
	for _, item := range batch.Items {
		if item.Artifact != nil {
			o.Fetched = append(o.Fetched, item.Artifact.Ref)
		}
	}
	for _, ref := range o.Fetched {
		if ref.Kind != "skill" {
			continue
		}
		var resolved struct {
			Status   string      `json:"status"`
			Skill    observedRef `json:"skill"`
			Findings []struct {
				Status  string       `json:"status"`
				Binding *observedRef `json:"binding"`
			} `json:"findings"`
		}
		call = post(ctx, tgt, client, token, "/v1/resolve", map[string]any{"skill": map[string]string{"kind": "skill", "id": ref.ID, "version": ref.Version}, "runtime_id": runtimeID, "local_execution": true, "max_bytes": c.MaxBytes}, "resolve", &resolved)
		o.Calls = append(o.Calls, call)
		if call.Status == 0 {
			o.TransportError = call.Err
			return o
		}
		if call.Status != http.StatusOK {
			o.ResolveStatus = append(o.ResolveStatus, fmt.Sprintf("http_%d", call.Status))
			continue
		}
		o.ResolveStatus = append(o.ResolveStatus, resolved.Status)
		o.ResolvedRefs = append(o.ResolvedRefs, resolved.Skill)
		for _, f := range resolved.Findings {
			if f.Status == "requires_selection" {
				o.ResolveStatus = append(o.ResolveStatus, "requires_selection")
			}
			if f.Binding != nil {
				o.ResolvedRefs = append(o.ResolvedRefs, *f.Binding)
			}
		}
	}
	return o
}

// post performs one timed network round trip and decodes a 2xx body into out.
// Status 0 means no HTTP response was received.
func post(ctx context.Context, tgt *target, client *http.Client, token, path string, body any, boundary string, out any) callRecord {
	rec := callRecord{Boundary: boundary}
	raw, err := json.Marshal(body)
	if err != nil {
		rec.Err = err.Error()
		return rec
	}
	req, err := http.NewRequestWithContext(ctx, http.MethodPost, tgt.baseURL+path, bytes.NewReader(raw))
	if err != nil {
		rec.Err = err.Error()
		return rec
	}
	req.Header.Set("Authorization", "Bearer "+token)
	req.Header.Set("Content-Type", "application/json")
	start := time.Now()
	resp, err := client.Do(req)
	if err != nil {
		rec.Latency = time.Since(start)
		rec.Err = redact(err.Error(), tgt.baseURL)
		return rec
	}
	defer resp.Body.Close()
	payload, err := io.ReadAll(resp.Body)
	rec.Latency = time.Since(start)
	rec.Status = resp.StatusCode
	rec.Bytes = len(payload)
	if err != nil {
		rec.Err = "read body: " + err.Error()
		return rec
	}
	if resp.StatusCode < 200 || resp.StatusCode > 299 {
		var e struct {
			Code string `json:"code"`
		}
		_ = json.Unmarshal(payload, &e)
		rec.Err = "error code " + e.Code
		return rec
	}
	if err := json.Unmarshal(payload, out); err != nil {
		rec.Err = "decode body: " + err.Error()
	}
	return rec
}

// leakInjector simulates an authorization bypass: it prepends the query's
// unauthorized adversary to every discover response.
type leakInjector struct{ next http.RoundTripper }

func (l leakInjector) RoundTrip(req *http.Request) (*http.Response, error) {
	if !strings.HasSuffix(req.URL.Path, "/v1/discover") {
		return l.next.RoundTrip(req)
	}
	reqBody, err := io.ReadAll(req.Body)
	if err != nil {
		return nil, err
	}
	_ = req.Body.Close()
	var in struct {
		Query string `json:"query"`
	}
	_ = json.Unmarshal(reqBody, &in)
	clone := req.Clone(req.Context())
	clone.Body = io.NopCloser(bytes.NewReader(reqBody))
	clone.ContentLength = int64(len(reqBody))
	resp, err := l.next.RoundTrip(clone)
	if err != nil || resp.StatusCode != http.StatusOK {
		return resp, err
	}
	raw, err := io.ReadAll(resp.Body)
	_ = resp.Body.Close()
	if err != nil {
		return nil, err
	}
	var page map[string]any
	if err := json.Unmarshal(raw, &page); err != nil {
		return nil, err
	}
	items, _ := page["items"].([]any)
	leak := map[string]any{"Ref": map[string]string{"WorkspaceID": adversaryWorkspaceID, "Kind": "skill", "ID": adversaryPrefix + strings.ToLower(in.Query), "Version": "1.0.0"}}
	page["items"] = append([]any{leak}, items...)
	out, err := json.Marshal(page)
	if err != nil {
		return nil, err
	}
	resp.Body = io.NopCloser(bytes.NewReader(out))
	resp.ContentLength = int64(len(out))
	resp.Header.Del("Content-Length")
	return resp, nil
}

func transportOf(c *http.Client) http.RoundTripper {
	if c.Transport != nil {
		return c.Transport
	}
	return http.DefaultTransport
}

func stampReport(rep *report, s frozenSuite, tgt *target) error {
	client, err := treeHash(".", ".")
	if err != nil {
		return err
	}
	build, err := treeHash("../..", "go.mod", "go.sum", "internal", "cmd", "migrations")
	if err != nil {
		return err
	}
	cfg, err := canonicalJSON(rep.Config)
	if err != nil {
		return err
	}
	rep.Target = tgt.name
	rep.GoVersion = goVersion()
	rep.Hashes = reportHashes{Client: client, Build: build, Config: sha256Hex(cfg), Catalog: tgt.catalogHash, Target: tgt.configHash, Corpus: s.corpusSHA256, Cases: s.casesSHA256}
	return nil
}

func persistReport(t *testing.T, rep report) string {
	t.Helper()
	dir := os.Getenv("REGISTRY_ARTIFACT_DIR")
	if dir == "" {
		dir = t.TempDir()
	}
	path := filepath.Join(dir, "e2-retrieval-smoke.json")
	if err := writeJSONFile(path, rep); err != nil {
		t.Fatalf("write smoke report: %v", err)
	}
	return path
}

func writeJSONFile(path string, v any) error {
	b, err := json.MarshalIndent(v, "", "  ")
	if err != nil {
		return err
	}
	return os.WriteFile(path, append(b, '\n'), 0o644)
}

func logReport(t *testing.T, rep report, path string) {
	t.Helper()
	a := rep.Aggregate
	t.Logf("target=%s cases=%d skipped=%d report=%s", rep.Target, a.CasesExecuted, a.CasesSkipped, filepath.Base(path))
	t.Logf("hashes client=%s build=%s config=%s catalog=%s corpus=%s cases=%s", short(rep.Hashes.Client), short(rep.Hashes.Build), short(rep.Hashes.Config), short(rep.Hashes.Catalog), short(rep.Hashes.Corpus), short(rep.Hashes.Cases))
	t.Logf("exact_top3_recall=%.4f no_match_accuracy=%.4f ambiguous_false_ready=%d ambiguous_selection_requested=%d/%d leak_count=%d forbidden_hits=%d completion=%.4f",
		a.TopKRecall, a.NoMatchAccuracy, a.AmbiguousFalseReady, a.AmbiguousSelection, a.ClassCounts["ambiguous"], a.LeakCount, a.ForbiddenHitCount, a.CompletionRate)
	t.Logf("labeled probe: relevant_get_rate=%.4f forbidden_denied=%s resolve=%v round_trips=%d bytes=%d", a.ProbeRelevantGetRate, a.ProbeForbiddenDenied, a.ProbeResolveStatus, a.ProbeRoundTrips, a.ProbeBytes)
	t.Logf("bytes total=%d mean=%.1f token_estimate=%d (%s) round_trips total=%d mean=%.2f case_latency p50=%.3fms p95=%.3fms",
		a.TotalBytes, a.MeanBytesPerCase, a.TotalTokenEstimate, a.TokenEstimateMethod, a.TotalRoundTrips, a.MeanRoundTrips, a.CaseLatency.P50MS, a.CaseLatency.P95MS)
	for name, tr := range rep.Thresholds {
		t.Logf("threshold %s %s measured=%.4f target %s %.4f failing=%v", name, tr.Verdict, tr.Measured, tr.Operator, tr.Target, tr.Failing)
	}
	for _, row := range rep.Cases {
		t.Logf("case %s expected=%s observed=%s rank=%d candidates=%v resolve=%v leaks=%v forbidden=%v bytes=%d rt=%d pass=%v",
			row.CaseID, row.ExpectedBehavior, row.ObservedBehavior, row.RelevantRank, row.Candidates, row.ResolveStatus, row.UnauthorizedRefs, row.ForbiddenHits, row.ResponseBytes, row.RoundTrips, row.ThresholdPass)
	}
}

func short(h string) string {
	if len(h) > 12 {
		return h[:12]
	}
	return h
}

func firstLine(s string) string {
	if i := strings.IndexByte(s, '\n'); i >= 0 {
		return s[:i]
	}
	return s
}

func redact(msg, baseURL string) string {
	if baseURL == "" {
		return msg
	}
	return strings.ReplaceAll(msg, baseURL, "<target>")
}

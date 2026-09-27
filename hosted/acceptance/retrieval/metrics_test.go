package retrieval

// This file holds the pure (network-free) half of the E2 smoke runner: loading
// and hash-verifying the frozen E1 labels, labeling observed references,
// computing metrics and threshold verdicts, and comparing against a baseline.
// The network half lives in smoke_test.go and the target_*_test.go files.

import (
	"bufio"
	"bytes"
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"fmt"
	"io/fs"
	"math"
	"os"
	"path/filepath"
	"runtime"
	"sort"
	"strings"
	"testing"
	"time"
)

const (
	// evalDir is the frozen E1 label directory relative to this package.
	evalDir = "../../../eval/registry"
	// topK is the recall depth the E1 exact-target threshold names.
	topK = 3
	// discoverMaxResults is the page size the runner requests from discover.
	discoverMaxResults = 10
	// runtimeID is the runtime the runner declares when resolving skills.
	runtimeID = "go"
	// adversaryPrefix marks every deliberately unauthorized high-score
	// candidate the runner seeds. Any reference carrying it is a leak.
	adversaryPrefix = "e2-adversary/"
	// adversaryWorkspaceKey is a workspace no smoke principal belongs to.
	adversaryWorkspaceKey = "workspace-adversary"
	adversaryWorkspaceID  = "fixture-workspace-adversary"
	reportSchema          = "registry-smoke-baseline-v1"
	tokenEstimateMethod   = "ceil(response_bytes/4); a labeled byte heuristic, not a tokenizer measurement"
)

var classPrefixes = []struct{ prefix, class string }{
	{"exact-", "exact"},
	{"no-match-", "no_match"},
	{"ambiguous-", "ambiguous"},
	{"unauthorized-", "unauthorized_candidate"},
}

type smokeCase struct {
	CaseID            string   `json:"case_id"`
	Query             string   `json:"query"`
	PrincipalKey      string   `json:"principal_fixture_key"`
	WorkspaceKey      string   `json:"workspace_fixture_key"`
	Eligible          []string `json:"eligible_artifact_ids"`
	Relevant          []string `json:"relevant_artifact_ids"`
	ExpectedBehavior  string   `json:"expected_behavior"`
	SelectionRequired bool     `json:"selection_required"`
	Forbidden         []string `json:"forbidden_artifact_ids"`
	MaxBytes          int      `json:"max_bytes"`
}

func (c smokeCase) class() string {
	for _, p := range classPrefixes {
		if strings.HasPrefix(c.CaseID, p.prefix) {
			return p.class
		}
	}
	return ""
}

type corpusWorkspace struct {
	Key string `json:"key"`
	ID  string `json:"id"`
}

type corpusPrincipal struct {
	Key          string   `json:"key"`
	Subject      string   `json:"subject"`
	WorkspaceKey string   `json:"workspace_key"`
	Scopes       []string `json:"scopes"`
}

type corpusArtifact struct {
	WorkspaceKey         string   `json:"workspace_key"`
	Kind                 string   `json:"kind"`
	ID                   string   `json:"id"`
	Version              string   `json:"version"`
	LogicalName          string   `json:"logical_name"`
	Summary              string   `json:"summary"`
	Trust                string   `json:"trust"`
	Tags                 []string `json:"tags"`
	RequiredCapabilities []string `json:"required_capabilities"`
	EstimatedBytes       int      `json:"estimated_bytes"`
}

type corpus struct {
	Workspaces   []corpusWorkspace `json:"workspaces"`
	Principals   []corpusPrincipal `json:"principals"`
	ArtifactKeys []string          `json:"artifact_keys"`
	Artifacts    []corpusArtifact  `json:"artifacts"`
	SHA256       string            `json:"sha256"`
}

type thresholdTarget struct {
	Operator string  `json:"operator"`
	Value    float64 `json:"value"`
	Unit     string  `json:"unit"`
	Cases    string  `json:"cases"`
}

type thresholds struct {
	CaseCount   int                        `json:"case_count"`
	ClassCounts map[string]int             `json:"class_counts"`
	Targets     map[string]thresholdTarget `json:"targets"`
	Freeze      struct {
		CasesSHA256  string `json:"cases_sha256"`
		CorpusSHA256 string `json:"corpus_sha256"`
	} `json:"freeze"`
}

// frozenSuite is the E1 label set after its hashes were verified.
type frozenSuite struct {
	cases        []smokeCase
	corpus       corpus
	thresholds   thresholds
	casesSHA256  string
	corpusSHA256 string
}

// loadFrozenSuite loads the E1 files and refuses to proceed unless the
// recomputed hashes equal the values frozen in thresholds.json.
func loadFrozenSuite(dir string) (frozenSuite, error) {
	var s frozenSuite
	rawCases, err := os.ReadFile(filepath.Join(dir, "cases.jsonl"))
	if err != nil {
		return s, fmt.Errorf("frozen cases are required: %w", err)
	}
	rawCorpus, err := os.ReadFile(filepath.Join(dir, "corpus.json"))
	if err != nil {
		return s, fmt.Errorf("frozen corpus is required: %w", err)
	}
	rawThresholds, err := os.ReadFile(filepath.Join(dir, "thresholds.json"))
	if err != nil {
		return s, fmt.Errorf("frozen thresholds are required: %w", err)
	}
	if err := json.Unmarshal(rawThresholds, &s.thresholds); err != nil {
		return s, fmt.Errorf("decode thresholds: %w", err)
	}
	s.casesSHA256 = sha256Hex(rawCases)
	if s.casesSHA256 != s.thresholds.Freeze.CasesSHA256 {
		return s, fmt.Errorf("cases.jsonl hash %s does not match frozen %s", s.casesSHA256, s.thresholds.Freeze.CasesSHA256)
	}
	s.corpusSHA256, err = canonicalCorpusHash(rawCorpus)
	if err != nil {
		return s, err
	}
	if s.corpusSHA256 != s.thresholds.Freeze.CorpusSHA256 {
		return s, fmt.Errorf("corpus.json canonical hash %s does not match frozen %s", s.corpusSHA256, s.thresholds.Freeze.CorpusSHA256)
	}
	if err := json.Unmarshal(rawCorpus, &s.corpus); err != nil {
		return s, fmt.Errorf("decode corpus: %w", err)
	}
	if s.corpus.SHA256 != s.corpusSHA256 {
		return s, fmt.Errorf("corpus.json embedded sha256 %s does not match its canonical hash %s", s.corpus.SHA256, s.corpusSHA256)
	}
	scanner := bufio.NewScanner(bytes.NewReader(rawCases))
	for line := 1; scanner.Scan(); line++ {
		text := strings.TrimSpace(scanner.Text())
		if text == "" {
			continue
		}
		dec := json.NewDecoder(strings.NewReader(text))
		dec.DisallowUnknownFields()
		var c smokeCase
		if err := dec.Decode(&c); err != nil {
			return s, fmt.Errorf("decode cases.jsonl line %d: %w", line, err)
		}
		s.cases = append(s.cases, c)
	}
	if err := scanner.Err(); err != nil {
		return s, err
	}
	return s, s.validate()
}

func (s frozenSuite) validate() error {
	if len(s.cases) != s.thresholds.CaseCount {
		return fmt.Errorf("frozen suite has %d cases, thresholds require %d", len(s.cases), s.thresholds.CaseCount)
	}
	principals := map[string]string{}
	for _, p := range s.corpus.Principals {
		principals[p.Key] = p.WorkspaceKey
	}
	counts := map[string]int{}
	seen := map[string]bool{}
	for _, c := range s.cases {
		if seen[c.CaseID] {
			return fmt.Errorf("duplicate case id %s", c.CaseID)
		}
		seen[c.CaseID] = true
		class := c.class()
		if class == "" {
			return fmt.Errorf("case %s has no recognized class prefix", c.CaseID)
		}
		counts[class]++
		if ws, ok := principals[c.PrincipalKey]; !ok || ws != c.WorkspaceKey {
			return fmt.Errorf("case %s principal %s is not a member of %s", c.CaseID, c.PrincipalKey, c.WorkspaceKey)
		}
		if c.MaxBytes <= 0 || len(c.Forbidden) == 0 {
			return fmt.Errorf("case %s needs a positive byte budget and explicit forbidden IDs", c.CaseID)
		}
	}
	for class, want := range s.thresholds.ClassCounts {
		if counts[class] != want {
			return fmt.Errorf("class %s has %d cases, thresholds require %d", class, counts[class], want)
		}
	}
	if _, err := newLabeler(s.corpus); err != nil {
		return err
	}
	return nil
}

// canonicalCorpusHash mirrors the E1 hash method: SHA-256 over
// json.dumps(doc_without_sha256, sort_keys=True, separators=(",",":"),
// ensure_ascii=False). encoding/json sorts map keys and json.Number keeps
// number spelling, so a decode/re-encode round trip reproduces it for the
// ASCII corpus.
func canonicalCorpusHash(raw []byte) (string, error) {
	dec := json.NewDecoder(bytes.NewReader(raw))
	dec.UseNumber()
	var doc map[string]any
	if err := dec.Decode(&doc); err != nil {
		return "", fmt.Errorf("decode corpus for hashing: %w", err)
	}
	delete(doc, "sha256")
	b, err := canonicalJSON(doc)
	if err != nil {
		return "", err
	}
	return sha256Hex(b), nil
}

func canonicalJSON(v any) ([]byte, error) {
	var buf bytes.Buffer
	enc := json.NewEncoder(&buf)
	enc.SetEscapeHTML(false)
	if err := enc.Encode(v); err != nil {
		return nil, err
	}
	return bytes.TrimRight(buf.Bytes(), "\n"), nil
}

func sha256Hex(b []byte) string {
	sum := sha256.Sum256(b)
	return hex.EncodeToString(sum[:])
}

// labeler maps a reference observed at a service boundary back to the E1
// label vocabulary: an artifact in the caller's own workspace keeps its
// unqualified label; one from another workspace gets "@<workspace-key>".
type labeler struct {
	labelByID     map[string]string
	workspaceKeys map[string]string
}

func newLabeler(c corpus) (labeler, error) {
	l := labeler{labelByID: map[string]string{}, workspaceKeys: map[string]string{}}
	keys := map[string]bool{}
	for _, k := range c.ArtifactKeys {
		keys[k] = true
	}
	for _, a := range c.Artifacts {
		label := ""
		switch {
		case keys[a.ID]:
			label = a.ID
		case keys["gist/"+a.ID]:
			label = "gist/" + a.ID
		default:
			return l, fmt.Errorf("corpus artifact %s has no artifact_keys label", a.ID)
		}
		l.labelByID[a.ID] = label
	}
	for _, w := range c.Workspaces {
		l.workspaceKeys[w.ID] = w.Key
	}
	l.workspaceKeys[adversaryWorkspaceID] = adversaryWorkspaceKey
	return l, nil
}

// label returns the E1 label for ref as seen by a caller in callerWorkspaceID,
// and whether the reference is unauthorized for that caller.
func (l labeler) label(ref observedRef, callerWorkspaceID string) (string, bool) {
	base, known := l.labelByID[ref.ID]
	if !known {
		base = ref.ID
	}
	unauthorized := strings.HasPrefix(ref.ID, adversaryPrefix)
	// An empty workspace means the boundary omitted it; that response is
	// scoped to the caller's workspace by construction.
	if ref.WorkspaceID != "" && ref.WorkspaceID != callerWorkspaceID {
		unauthorized = true
		key, ok := l.workspaceKeys[ref.WorkspaceID]
		if !ok {
			key = "unknown-workspace"
		}
		return base + "@" + key, true
	}
	return base, unauthorized
}

type observedRef struct {
	WorkspaceID string `json:"WorkspaceID"`
	Kind        string `json:"Kind"`
	ID          string `json:"ID"`
	Version     string `json:"Version"`
}

type callRecord struct {
	Boundary string        `json:"boundary"`
	Status   int           `json:"status"`
	Bytes    int           `json:"bytes"`
	Latency  time.Duration `json:"-"`
	Err      string        `json:"error,omitempty"`
}

// observation is everything one case run saw at the service boundaries.
type observation struct {
	CaseID         string
	Candidates     []observedRef
	Fetched        []observedRef
	ResolveStatus  []string
	ResolvedRefs   []observedRef
	Calls          []callRecord
	TransportError string
	// Probe phase: direct batch-get of the labeled relevant items and the
	// forbidden foreign-workspace items, then resolve of relevant skills.
	// It exercises get/resolve independently of discovery ranking and is
	// kept out of the interaction metrics.
	ProbeRequested  []string
	ProbeForbidden  []string
	ProbeFetched    []observedRef
	ProbeResolve    []string
	ProbeCalls      []callRecord
	ProbeTransportE string
}

type callRow struct {
	Boundary  string  `json:"boundary"`
	Status    int     `json:"status"`
	Bytes     int     `json:"bytes"`
	LatencyMS float64 `json:"latency_ms"`
	Error     string  `json:"error,omitempty"`
}

type caseRow struct {
	CaseID            string    `json:"case_id"`
	Class             string    `json:"class"`
	ExpectedBehavior  string    `json:"expected_behavior"`
	ObservedBehavior  string    `json:"observed_behavior"`
	SelectionRequired bool      `json:"selection_required"`
	SelectionAsked    bool      `json:"selection_requested"`
	Candidates        []string  `json:"candidates"`
	RelevantRank      int       `json:"relevant_rank"`
	TopKHit           bool      `json:"top3_hit"`
	ForbiddenHits     []string  `json:"forbidden_hits"`
	UnauthorizedRefs  []string  `json:"unauthorized_refs"`
	ResolveStatus     []string  `json:"resolve_status"`
	Completed         bool      `json:"completed"`
	RoundTrips        int       `json:"round_trips"`
	ResponseBytes     int       `json:"response_bytes"`
	MaxBytes          int       `json:"max_bytes"`
	OverBudget        bool      `json:"over_budget"`
	TokenEstimate     int       `json:"token_estimate"`
	LatencyMS         float64   `json:"latency_ms"`
	Calls             []callRow `json:"calls"`
	Probe             probeRow  `json:"labeled_probe"`
	ThresholdPass     bool      `json:"threshold_pass"`
	Failures          []string  `json:"failures"`
}

type probeRow struct {
	RelevantRequested  []string  `json:"relevant_requested"`
	RelevantFetched    []string  `json:"relevant_fetched"`
	ForbiddenRequested []string  `json:"forbidden_requested"`
	ForbiddenReturned  []string  `json:"forbidden_returned"`
	ResolveStatus      []string  `json:"resolve_status"`
	RoundTrips         int       `json:"round_trips"`
	ResponseBytes      int       `json:"response_bytes"`
	Calls              []callRow `json:"calls"`
}

type thresholdResult struct {
	Operator string   `json:"operator"`
	Target   float64  `json:"target"`
	Measured float64  `json:"measured"`
	Cases    int      `json:"cases"`
	Verdict  string   `json:"verdict"`
	Failing  []string `json:"failing_case_ids"`
}

type latencySummary struct {
	Count int     `json:"count"`
	P50MS float64 `json:"p50_ms"`
	P95MS float64 `json:"p95_ms"`
}

type aggregate struct {
	CasesExecuted        int                       `json:"cases_executed"`
	CasesSkipped         int                       `json:"cases_skipped"`
	ClassCounts          map[string]int            `json:"class_counts"`
	TopKRecall           float64                   `json:"exact_top3_recall"`
	NoMatchAccuracy      float64                   `json:"no_match_accuracy"`
	AmbiguousFalseReady  int                       `json:"ambiguous_false_ready"`
	AmbiguousSelection   int                       `json:"ambiguous_selection_requested"`
	LeakCount            int                       `json:"leak_count"`
	ForbiddenHitCount    int                       `json:"forbidden_hit_count"`
	CompletionRate       float64                   `json:"completion_rate"`
	OverBudgetCases      []string                  `json:"over_budget_case_ids"`
	TotalBytes           int                       `json:"total_response_bytes"`
	MeanBytesPerCase     float64                   `json:"mean_response_bytes_per_case"`
	TotalTokenEstimate   int                       `json:"total_token_estimate"`
	TokenEstimateMethod  string                    `json:"token_estimate_method"`
	TotalRoundTrips      int                       `json:"total_round_trips"`
	MeanRoundTrips       float64                   `json:"mean_round_trips_per_case"`
	CaseLatency          latencySummary            `json:"case_latency"`
	BoundaryLatency      map[string]latencySummary `json:"boundary_latency"`
	BoundaryStatusCounts map[string]map[int]int    `json:"boundary_status_counts"`
	ProbeRelevantGetRate float64                   `json:"probe_relevant_get_rate"`
	ProbeForbiddenDenied string                    `json:"probe_forbidden_denied"`
	ProbeResolveStatus   map[string]int            `json:"probe_resolve_status_counts"`
	ProbeRoundTrips      int                       `json:"probe_round_trips"`
	ProbeBytes           int                       `json:"probe_response_bytes"`
}

type reportHashes struct {
	Client  string `json:"client_sha256"`
	Build   string `json:"build_sha256"`
	Config  string `json:"config_sha256"`
	Catalog string `json:"catalog_sha256"`
	Target  string `json:"target_config_sha256"`
	Corpus  string `json:"corpus_sha256"`
	Cases   string `json:"cases_sha256"`
}

type report struct {
	SchemaVersion string                     `json:"schema_version"`
	Target        string                     `json:"target"`
	GoVersion     string                     `json:"go_version"`
	Hashes        reportHashes               `json:"hashes"`
	Config        runnerConfig               `json:"config"`
	Aggregate     aggregate                  `json:"aggregate"`
	Thresholds    map[string]thresholdResult `json:"thresholds"`
	Cases         []caseRow                  `json:"cases"`
}

// runnerConfig is everything about how the runner drives the boundaries.
// Two reports are comparable only when this and the catalog match.
type runnerConfig struct {
	TopK                 int      `json:"top_k"`
	DiscoverMaxResults   int      `json:"discover_max_results"`
	RuntimeID            string   `json:"runtime_id"`
	Boundaries           []string `json:"boundaries"`
	BehaviorRule         string   `json:"behavior_rule"`
	CapabilityTranslator string   `json:"capability_ref_translation"`
	Adversaries          string   `json:"adversaries"`
}

func defaultRunnerConfig() runnerConfig {
	return runnerConfig{
		TopK:               topK,
		DiscoverMaxResults: discoverMaxResults,
		RuntimeID:          runtimeID,
		Boundaries:         []string{"POST /v1/discover", "POST /v1/artifacts/batch-get (top 3)", "POST /v1/resolve (each fetched skill)", "probe: POST /v1/artifacts/batch-get (labeled relevant + forbidden foreign refs)", "probe: POST /v1/resolve (labeled relevant skills)"},
		BehaviorRule: "empty: discover returns no candidate; ambiguous (selection requested): two or more candidates, " +
			"or a resolved skill reports requires_selection; ready: exactly one candidate that is not a skill or resolves ready; " +
			"incomplete: otherwise",
		CapabilityTranslator: "corpus required_capabilities id -> id@<corpus version> (the resolver's pinned reference form)",
		Adversaries: "per distinct query: a published high-score candidate whose ID contains the query in " + adversaryWorkspaceID +
			" (no smoke principal is a member) and a revoked copy in each smoke workspace",
	}
}

// evaluate turns raw observations into the per-case rows and metrics.
func evaluate(s frozenSuite, obs []observation) (report, error) {
	l, err := newLabeler(s.corpus)
	if err != nil {
		return report{}, err
	}
	workspaceID := map[string]string{}
	for _, w := range s.corpus.Workspaces {
		workspaceID[w.Key] = w.ID
	}
	byID := map[string]observation{}
	for _, o := range obs {
		byID[o.CaseID] = o
	}
	r := report{SchemaVersion: reportSchema, Config: defaultRunnerConfig(), Thresholds: map[string]thresholdResult{}}
	agg := aggregate{ClassCounts: map[string]int{}, BoundaryLatency: map[string]latencySummary{}, BoundaryStatusCounts: map[string]map[int]int{}, TokenEstimateMethod: tokenEstimateMethod, ProbeResolveStatus: map[string]int{}}
	probeRelevant, probeRelevantGot, probeForbidden, probeForbiddenGot := 0, 0, 0, 0
	boundaryLatencies := map[string][]time.Duration{}
	var caseLatencies []time.Duration
	var missing []string
	completed := 0
	for _, c := range s.cases {
		o, ok := byID[c.CaseID]
		if !ok {
			missing = append(missing, c.CaseID)
			continue
		}
		caller := workspaceID[c.WorkspaceKey]
		row := caseRow{CaseID: c.CaseID, Class: c.class(), ExpectedBehavior: c.ExpectedBehavior, SelectionRequired: c.SelectionRequired, MaxBytes: c.MaxBytes, RelevantRank: 0, Candidates: []string{}, ForbiddenHits: []string{}, UnauthorizedRefs: []string{}, ResolveStatus: append([]string{}, o.ResolveStatus...), Failures: []string{}}
		forbidden := map[string]bool{}
		for _, id := range c.Forbidden {
			forbidden[id] = true
		}
		relevant := map[string]bool{}
		for _, id := range c.Relevant {
			relevant[id] = true
		}
		seenForbidden := map[string]bool{}
		seenUnauthorized := map[string]bool{}
		inspect := func(ref observedRef) string {
			label, unauthorized := l.label(ref, caller)
			if unauthorized && !seenUnauthorized[label] {
				seenUnauthorized[label] = true
				row.UnauthorizedRefs = append(row.UnauthorizedRefs, label)
			}
			if forbidden[label] && !seenForbidden[label] {
				seenForbidden[label] = true
				row.ForbiddenHits = append(row.ForbiddenHits, label)
			}
			return label
		}
		for i, ref := range o.Candidates {
			label := inspect(ref)
			row.Candidates = append(row.Candidates, label)
			if relevant[label] && row.RelevantRank == 0 {
				row.RelevantRank = i + 1
			}
		}
		for _, ref := range o.Fetched {
			inspect(ref)
		}
		for _, ref := range o.ResolvedRefs {
			inspect(ref)
		}
		probeFailed := o.ProbeTransportE != ""
		row.Probe = probeRow{RelevantRequested: append([]string{}, o.ProbeRequested...), ForbiddenRequested: append([]string{}, o.ProbeForbidden...), RelevantFetched: []string{}, ForbiddenReturned: []string{}, ResolveStatus: append([]string{}, o.ProbeResolve...)}
		requestedForbidden := map[string]bool{}
		for _, id := range o.ProbeForbidden {
			requestedForbidden[id] = true
		}
		for _, ref := range o.ProbeFetched {
			label := inspect(ref)
			if requestedForbidden[label] {
				row.Probe.ForbiddenReturned = append(row.Probe.ForbiddenReturned, label)
			} else if relevant[label] {
				row.Probe.RelevantFetched = append(row.Probe.RelevantFetched, label)
			}
		}
		for _, call := range o.ProbeCalls {
			row.Probe.RoundTrips++
			row.Probe.ResponseBytes += call.Bytes
			if call.Status < 200 || call.Status > 299 {
				probeFailed = true
				row.Failures = append(row.Failures, fmt.Sprintf("probe %s returned %d", call.Boundary, call.Status))
			}
			row.Probe.Calls = append(row.Probe.Calls, callRow{Boundary: call.Boundary, Status: call.Status, Bytes: call.Bytes, LatencyMS: ms(call.Latency), Error: call.Err})
		}
		if o.ProbeTransportE != "" {
			row.Failures = append(row.Failures, "transport: probe: "+o.ProbeTransportE)
		}
		for _, status := range o.ProbeResolve {
			agg.ProbeResolveStatus[status]++
		}
		probeRelevant += len(o.ProbeRequested)
		probeRelevantGot += len(row.Probe.RelevantFetched)
		probeForbidden += len(o.ProbeForbidden)
		probeForbiddenGot += len(row.Probe.ForbiddenReturned)
		agg.ProbeRoundTrips += row.Probe.RoundTrips
		agg.ProbeBytes += row.Probe.ResponseBytes
		row.TopKHit = row.RelevantRank > 0 && row.RelevantRank <= topK
		row.ObservedBehavior = observedBehavior(o)
		row.SelectionAsked = row.ObservedBehavior == "ambiguous"
		row.Completed = o.TransportError == "" && len(o.Calls) > 0 && !probeFailed
		var total time.Duration
		for _, call := range o.Calls {
			row.RoundTrips++
			row.ResponseBytes += call.Bytes
			total += call.Latency
			if call.Status < 200 || call.Status > 299 {
				row.Completed = false
			}
			if call.Bytes > c.MaxBytes {
				row.OverBudget = true
			}
			boundaryLatencies[call.Boundary] = append(boundaryLatencies[call.Boundary], call.Latency)
			if agg.BoundaryStatusCounts[call.Boundary] == nil {
				agg.BoundaryStatusCounts[call.Boundary] = map[int]int{}
			}
			agg.BoundaryStatusCounts[call.Boundary][call.Status]++
			row.Calls = append(row.Calls, callRow{Boundary: call.Boundary, Status: call.Status, Bytes: call.Bytes, LatencyMS: ms(call.Latency), Error: call.Err})
		}
		if o.TransportError != "" {
			row.Failures = append(row.Failures, "transport: "+o.TransportError)
		}
		caseLatencies = append(caseLatencies, total)
		row.LatencyMS = ms(total)
		row.TokenEstimate = (row.ResponseBytes + 3) / 4
		if row.Completed {
			completed++
		}
		if row.OverBudget {
			agg.OverBudgetCases = append(agg.OverBudgetCases, c.CaseID)
		}
		agg.ClassCounts[row.Class]++
		agg.CasesExecuted++
		agg.LeakCount += len(row.UnauthorizedRefs)
		agg.ForbiddenHitCount += len(row.ForbiddenHits)
		agg.TotalBytes += row.ResponseBytes
		agg.TotalTokenEstimate += row.TokenEstimate
		agg.TotalRoundTrips += row.RoundTrips
		if len(row.UnauthorizedRefs) > 0 {
			row.Failures = append(row.Failures, "unauthorized references visible: "+strings.Join(row.UnauthorizedRefs, ", "))
		}
		r.Cases = append(r.Cases, row)
	}
	if len(missing) > 0 {
		return r, fmt.Errorf("mandatory cases did not execute: %s", strings.Join(missing, ", "))
	}
	agg.CasesSkipped = len(s.cases) - agg.CasesExecuted
	if agg.CasesExecuted > 0 {
		agg.CompletionRate = float64(completed) / float64(agg.CasesExecuted)
		agg.MeanBytesPerCase = float64(agg.TotalBytes) / float64(agg.CasesExecuted)
		agg.MeanRoundTrips = float64(agg.TotalRoundTrips) / float64(agg.CasesExecuted)
	}
	if probeRelevant > 0 {
		agg.ProbeRelevantGetRate = round4(float64(probeRelevantGot) / float64(probeRelevant))
	}
	agg.ProbeForbiddenDenied = fmt.Sprintf("%d/%d", probeForbidden-probeForbiddenGot, probeForbidden)
	agg.CaseLatency = summarize(caseLatencies)
	for boundary, values := range boundaryLatencies {
		agg.BoundaryLatency[boundary] = summarize(values)
	}
	r.Thresholds = scoreThresholds(s.thresholds, r.Cases)
	for i := range r.Cases {
		row := &r.Cases[i]
		row.ThresholdPass = true
		for name, tr := range r.Thresholds {
			for _, id := range tr.Failing {
				if id == row.CaseID {
					row.ThresholdPass = false
					row.Failures = append(row.Failures, "threshold "+name)
				}
			}
		}
		sort.Strings(row.Failures)
	}
	if n := r.Thresholds["exact_target_top3_recall"]; n.Cases > 0 {
		agg.TopKRecall = n.Measured
	}
	if n := r.Thresholds["no_match_nonempty_rate"]; n.Cases > 0 {
		agg.NoMatchAccuracy = 1 - n.Measured
	}
	agg.AmbiguousFalseReady = len(r.Thresholds["ambiguous_false_ready_rate"].Failing)
	agg.AmbiguousSelection = r.Thresholds["ambiguous_selection_request_rate"].Cases - len(r.Thresholds["ambiguous_selection_request_rate"].Failing)
	r.Aggregate = agg
	return r, nil
}

func observedBehavior(o observation) string {
	for _, status := range o.ResolveStatus {
		if status == "requires_selection" {
			return "ambiguous"
		}
	}
	switch {
	case len(o.Candidates) == 0:
		return "empty"
	case len(o.Candidates) >= 2:
		return "ambiguous"
	case o.Candidates[0].Kind != "skill":
		return "ready"
	case len(o.ResolveStatus) == 1 && o.ResolveStatus[0] == "ready":
		return "ready"
	default:
		return "incomplete"
	}
}

// caseFails reports whether a row violates the named E1 target.
var caseFails = map[string]func(caseRow) bool{
	"unauthorized_leakage_rate":        func(r caseRow) bool { return len(r.UnauthorizedRefs) > 0 || len(r.ForbiddenHits) > 0 },
	"no_match_nonempty_rate":           func(r caseRow) bool { return len(r.Candidates) > 0 },
	"ambiguous_false_ready_rate":       func(r caseRow) bool { return r.ObservedBehavior == "ready" },
	"ambiguous_selection_request_rate": func(r caseRow) bool { return r.SelectionRequired && !r.SelectionAsked },
	"exact_target_top3_recall":         func(r caseRow) bool { return !r.TopKHit },
}

func scoreThresholds(th thresholds, rows []caseRow) map[string]thresholdResult {
	out := map[string]thresholdResult{}
	for name, target := range th.Targets {
		res := thresholdResult{Operator: target.Operator, Target: target.Value, Failing: []string{}}
		fails, known := caseFails[name]
		var failing int
		for _, row := range rows {
			if row.Class != target.Cases {
				continue
			}
			res.Cases++
			if !known || fails(row) {
				failing++
				res.Failing = append(res.Failing, row.CaseID)
			}
		}
		if res.Cases > 0 {
			switch target.Operator {
			case "<=":
				res.Measured = round4(float64(failing) / float64(res.Cases))
			default:
				res.Measured = round4(float64(res.Cases-failing) / float64(res.Cases))
			}
		}
		pass := known && res.Cases > 0
		switch target.Operator {
		case "<=":
			pass = pass && res.Measured <= target.Value
		case ">=":
			pass = pass && res.Measured >= target.Value
		default:
			pass = false
		}
		res.Verdict = "FAIL"
		if pass {
			res.Verdict = "PASS"
		}
		out[name] = res
	}
	return out
}

// invariantFailures are the conditions E2 enforces on every run regardless of
// the E1 thresholds: all cases execute, nothing is skipped, every call gets a
// response, and no unauthorized reference is ever visible. Each message names
// the case IDs involved.
func invariantFailures(s frozenSuite, r report) []string {
	var out []string
	if r.Aggregate.CasesExecuted != len(s.cases) || r.Aggregate.CasesSkipped != 0 {
		out = append(out, fmt.Sprintf("executed %d of %d mandatory cases (%d skipped)", r.Aggregate.CasesExecuted, len(s.cases), r.Aggregate.CasesSkipped))
	}
	var transport, leaked []string
	for _, row := range r.Cases {
		for _, f := range row.Failures {
			if strings.HasPrefix(f, "transport: ") {
				transport = append(transport, row.CaseID)
				break
			}
		}
		if len(row.UnauthorizedRefs) > 0 {
			leaked = append(leaked, row.CaseID+" ["+strings.Join(row.UnauthorizedRefs, ", ")+"]")
		}
	}
	if len(transport) > 0 {
		out = append(out, "cases without a boundary response: "+strings.Join(transport, ", "))
	}
	if len(leaked) > 0 {
		out = append(out, fmt.Sprintf("unauthorized leakage (%d refs) in cases: %s", r.Aggregate.LeakCount, strings.Join(leaked, "; ")))
	}
	return out
}

// thresholdFailures lists every failing E1 target with its case IDs.
func thresholdFailures(r report) []string {
	names := make([]string, 0, len(r.Thresholds))
	for name := range r.Thresholds {
		names = append(names, name)
	}
	sort.Strings(names)
	var out []string
	for _, name := range names {
		tr := r.Thresholds[name]
		if tr.Verdict != "PASS" {
			out = append(out, fmt.Sprintf("%s measured %.4f, target %s %.4f: cases %s", name, tr.Measured, tr.Operator, tr.Target, strings.Join(tr.Failing, ", ")))
		}
	}
	return out
}

// compareBaseline refuses to compare unmatched catalogs or configurations and
// otherwise reports every case that passed its thresholds in the baseline but
// fails now.
func compareBaseline(base, now report) []string {
	var out []string
	for _, pair := range []struct{ name, a, b string }{
		{"corpus", base.Hashes.Corpus, now.Hashes.Corpus},
		{"cases", base.Hashes.Cases, now.Hashes.Cases},
		{"catalog", base.Hashes.Catalog, now.Hashes.Catalog},
		{"runner config", base.Hashes.Config, now.Hashes.Config},
	} {
		if pair.a != pair.b {
			out = append(out, fmt.Sprintf("baseline is not comparable: %s hash %s != %s", pair.name, pair.a, pair.b))
		}
	}
	if len(out) > 0 {
		return out
	}
	was := map[string]bool{}
	for _, row := range base.Cases {
		was[row.CaseID] = row.ThresholdPass
	}
	var regressed []string
	for _, row := range now.Cases {
		if was[row.CaseID] && !row.ThresholdPass {
			regressed = append(regressed, row.CaseID)
		}
	}
	if len(regressed) > 0 {
		out = append(out, "regressed against baseline: "+strings.Join(regressed, ", "))
	}
	return out
}

// seedRow is one catalog record the smoke catalog contains.
type seedRow struct {
	WorkspaceKey string          `json:"workspace_key"`
	WorkspaceID  string          `json:"workspace_id"`
	Kind         string          `json:"kind"`
	ID           string          `json:"id"`
	Version      string          `json:"version"`
	State        string          `json:"state"`
	Metadata     json.RawMessage `json:"metadata"`
}

// seedCatalog derives the exact records the smoke catalog must hold from the
// frozen corpus plus the deliberately unauthorized adversaries.
func seedCatalog(s frozenSuite) ([]seedRow, error) {
	versions := map[string]string{}
	for _, a := range s.corpus.Artifacts {
		versions[a.ID] = a.Version
	}
	workspaceID := map[string]string{}
	for _, w := range s.corpus.Workspaces {
		workspaceID[w.Key] = w.ID
	}
	var rows []seedRow
	for _, a := range s.corpus.Artifacts {
		refs := make([]string, 0, len(a.RequiredCapabilities))
		for _, capability := range a.RequiredCapabilities {
			version, ok := versions[capability]
			if !ok {
				return nil, fmt.Errorf("artifact %s requires unknown capability %s", a.ID, capability)
			}
			refs = append(refs, capability+"@"+version)
		}
		meta := map[string]any{"id": a.ID, "version": a.Version, "logical_name": a.LogicalName, "summary": a.Summary, "trust": a.Trust, "tags": a.Tags, "required_capabilities": refs, "estimated_bytes": a.EstimatedBytes}
		if a.Kind == "binding" && len(refs) == 1 {
			meta["capability_ref"] = refs[0]
		}
		raw, err := canonicalJSON(meta)
		if err != nil {
			return nil, err
		}
		rows = append(rows, seedRow{WorkspaceKey: a.WorkspaceKey, WorkspaceID: workspaceID[a.WorkspaceKey], Kind: a.Kind, ID: a.ID, Version: a.Version, State: "published", Metadata: raw})
	}
	queries := map[string]bool{}
	for _, c := range s.cases {
		queries[c.Query] = true
	}
	sorted := make([]string, 0, len(queries))
	for q := range queries {
		sorted = append(sorted, q)
	}
	sort.Strings(sorted)
	all := strings.Join(sorted, " ")
	for _, q := range sorted {
		id := adversaryPrefix + strings.ToLower(q)
		meta, err := canonicalJSON(map[string]any{"id": id, "version": "1.0.0", "logical_name": q, "summary": q + " " + all, "trust": "operator_asserted", "tags": []string{"adversary"}, "required_capabilities": []string{}, "estimated_bytes": 1})
		if err != nil {
			return nil, err
		}
		rows = append(rows, seedRow{WorkspaceKey: adversaryWorkspaceKey, WorkspaceID: adversaryWorkspaceID, Kind: "skill", ID: id, Version: "1.0.0", State: "published", Metadata: meta})
		for _, w := range s.corpus.Workspaces {
			rows = append(rows, seedRow{WorkspaceKey: w.Key, WorkspaceID: w.ID, Kind: "skill", ID: id, Version: "1.0.0", State: "revoked", Metadata: meta})
		}
	}
	return rows, nil
}

func catalogHash(rows []seedRow) (string, error) {
	b, err := canonicalJSON(rows)
	if err != nil {
		return "", err
	}
	return sha256Hex(b), nil
}

// treeHash hashes the named files and directory trees (Go and SQL sources,
// go.mod/go.sum) relative to root in sorted order.
func treeHash(root string, entries ...string) (string, error) {
	var files []string
	for _, entry := range entries {
		full := filepath.Join(root, entry)
		info, err := os.Stat(full)
		if err != nil {
			return "", err
		}
		if !info.IsDir() {
			files = append(files, entry)
			continue
		}
		err = filepath.WalkDir(full, func(path string, d fs.DirEntry, err error) error {
			if err != nil || d.IsDir() {
				return err
			}
			if ext := filepath.Ext(path); ext == ".go" || ext == ".sql" {
				rel, relErr := filepath.Rel(root, path)
				if relErr != nil {
					return relErr
				}
				files = append(files, filepath.ToSlash(rel))
			}
			return nil
		})
		if err != nil {
			return "", err
		}
	}
	sort.Strings(files)
	h := sha256.New()
	for _, f := range files {
		b, err := os.ReadFile(filepath.Join(root, f))
		if err != nil {
			return "", err
		}
		fmt.Fprintf(h, "%s\x00%d\x00", f, len(b))
		h.Write(b)
	}
	return hex.EncodeToString(h.Sum(nil)), nil
}

func ms(d time.Duration) float64 { return round4(float64(d.Microseconds()) / 1000) }
func round4(v float64) float64   { return math.Round(v*10000) / 10000 }

func summarize(values []time.Duration) latencySummary {
	if len(values) == 0 {
		return latencySummary{}
	}
	sorted := append([]time.Duration(nil), values...)
	sort.Slice(sorted, func(i, j int) bool { return sorted[i] < sorted[j] })
	return latencySummary{Count: len(sorted), P50MS: ms(nearestRank(sorted, 0.50)), P95MS: ms(nearestRank(sorted, 0.95))}
}

func nearestRank(sorted []time.Duration, p float64) time.Duration {
	idx := int(math.Ceil(p*float64(len(sorted)))) - 1
	if idx < 0 {
		idx = 0
	}
	return sorted[idx]
}

func goVersion() string { return runtime.Version() + " " + runtime.GOOS + "/" + runtime.GOARCH }

// --- Unit tests of the checker itself (no network) ---

func TestFrozenSuiteHashesMatchThresholds(t *testing.T) {
	s, err := loadFrozenSuite(evalDir)
	if err != nil {
		t.Fatalf("frozen E1 suite rejected: %v", err)
	}
	if len(s.cases) != 24 {
		t.Fatalf("got %d cases, want 24", len(s.cases))
	}
}

func TestTamperedLabelsFailHashCheck(t *testing.T) {
	dir := t.TempDir()
	for _, name := range []string{"cases.jsonl", "corpus.json", "thresholds.json"} {
		b, err := os.ReadFile(filepath.Join(evalDir, name))
		if err != nil {
			t.Fatal(err)
		}
		if name == "cases.jsonl" {
			b = bytes.Replace(b, []byte(`"max_bytes":4096}`), []byte(`"max_bytes":4097}`), 1)
		}
		if err := os.WriteFile(filepath.Join(dir, name), b, 0o600); err != nil {
			t.Fatal(err)
		}
	}
	if _, err := loadFrozenSuite(dir); err == nil || !strings.Contains(err.Error(), "does not match frozen") {
		t.Fatalf("tampered cases.jsonl was accepted: %v", err)
	}
	b, err := os.ReadFile(filepath.Join(evalDir, "cases.jsonl"))
	if err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(dir, "cases.jsonl"), b, 0o600); err != nil {
		t.Fatal(err)
	}
	corpusRaw, err := os.ReadFile(filepath.Join(evalDir, "corpus.json"))
	if err != nil {
		t.Fatal(err)
	}
	corpusRaw = bytes.Replace(corpusRaw, []byte("Offline document parsing fixture"), []byte("Offline document parsing fixturE"), 1)
	if err := os.WriteFile(filepath.Join(dir, "corpus.json"), corpusRaw, 0o600); err != nil {
		t.Fatal(err)
	}
	if _, err := loadFrozenSuite(dir); err == nil || !strings.Contains(err.Error(), "corpus.json canonical hash") {
		t.Fatalf("tampered corpus.json was accepted: %v", err)
	}
}

func TestMissingLabelsAreAFailure(t *testing.T) {
	if _, err := loadFrozenSuite(t.TempDir()); err == nil {
		t.Fatal("an empty label directory must fail, not skip")
	}
}

func TestLabelerQualifiesForeignWorkspaces(t *testing.T) {
	s, err := loadFrozenSuite(evalDir)
	if err != nil {
		t.Fatal(err)
	}
	l, err := newLabeler(s.corpus)
	if err != nil {
		t.Fatal(err)
	}
	for _, tc := range []struct {
		ref    observedRef
		want   string
		unauth bool
	}{
		{observedRef{WorkspaceID: "fixture-workspace-alpha", ID: "composio/identity-user-create"}, "gist/composio/identity-user-create", false},
		{observedRef{WorkspaceID: "fixture-workspace-beta", ID: "gist/skill/asset-skill"}, "gist/skill/asset-skill@workspace-beta", true},
		{observedRef{WorkspaceID: "fixture-workspace-alpha", ID: adversaryPrefix + "x"}, adversaryPrefix + "x", true},
		{observedRef{WorkspaceID: "someone-else", ID: "gist/document/parse"}, "gist/document/parse@unknown-workspace", true},
	} {
		got, unauth := l.label(tc.ref, "fixture-workspace-alpha")
		if got != tc.want || unauth != tc.unauth {
			t.Errorf("label(%+v) = %q,%v want %q,%v", tc.ref, got, unauth, tc.want, tc.unauth)
		}
	}
}

// TestCheckerFailsOnInjectedLeak is the offline negative control: the same
// evaluation that scores a real run must fail, naming the case, when one
// unauthorized candidate is visible.
func TestCheckerFailsOnInjectedLeak(t *testing.T) {
	s, err := loadFrozenSuite(evalDir)
	if err != nil {
		t.Fatal(err)
	}
	obs := make([]observation, 0, len(s.cases))
	for _, c := range s.cases {
		o := observation{CaseID: c.CaseID, Calls: []callRecord{{Boundary: "discover", Status: 200, Bytes: 10}}}
		if c.CaseID == "unauthorized-01-cross-workspace-document" {
			o.Candidates = []observedRef{{WorkspaceID: "fixture-workspace-beta", Kind: "skill", ID: "gist/skill/asset-skill", Version: "1.0.0"}}
		}
		obs = append(obs, o)
	}
	r, err := evaluate(s, obs)
	if err != nil {
		t.Fatal(err)
	}
	inv := strings.Join(invariantFailures(s, r), "\n")
	if !strings.Contains(inv, "unauthorized-01-cross-workspace-document") || r.Aggregate.LeakCount != 1 {
		t.Fatalf("injected leak not reported with its case ID: leaks=%d %q", r.Aggregate.LeakCount, inv)
	}
	if tr := r.Thresholds["unauthorized_leakage_rate"]; tr.Verdict != "FAIL" || len(tr.Failing) != 1 || tr.Failing[0] != "unauthorized-01-cross-workspace-document" {
		t.Fatalf("leakage threshold did not fail on the injected case: %+v", tr)
	}
	// A case that never executed is a failure, not a skip.
	if _, err := evaluate(s, obs[1:]); err == nil || !strings.Contains(err.Error(), s.cases[0].CaseID) {
		t.Fatalf("missing case was not reported by ID: %v", err)
	}
}

func TestBaselineComparisonRequiresMatchedCatalog(t *testing.T) {
	base := report{Hashes: reportHashes{Corpus: "a", Cases: "b", Catalog: "c", Config: "d"}, Cases: []caseRow{{CaseID: "x", ThresholdPass: true}}}
	now := base
	now.Cases = []caseRow{{CaseID: "x", ThresholdPass: false}}
	if got := compareBaseline(base, now); len(got) != 1 || !strings.Contains(got[0], "x") {
		t.Fatalf("regression not reported: %v", got)
	}
	now.Hashes.Catalog = "other"
	if got := compareBaseline(base, now); len(got) == 0 || !strings.Contains(got[0], "not comparable") {
		t.Fatalf("unmatched catalog was compared: %v", got)
	}
}

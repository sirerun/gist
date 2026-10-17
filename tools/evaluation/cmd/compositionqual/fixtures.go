package main

import (
	"bufio"
	"bytes"
	"context"
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"os"
	"sort"
	"time"

	c "github.com/sirerun/gist/catalog/composition"
	d "github.com/sirerun/gist/catalog/discovery"
	"github.com/sirerun/gist/catalog/packages"
	"github.com/sirerun/gist/catalog/ports"
	"github.com/sirerun/gist/catalog/skills/inboxcomposition"
)

const privateSentinel = "PRIVATE_COMPOSITION_SENTINEL_C13"

type scenario struct {
	ID       string `json:"id"`
	Scenario string `json:"scenario"`
}

func loadInputs(path string) ([]scenario, []byte, error) {
	b, e := os.ReadFile(path)
	if e != nil {
		return nil, nil, e
	}
	scan := bufio.NewScanner(bytes.NewReader(b))
	out := []scenario{}
	seen := map[string]bool{}
	for scan.Scan() {
		var s scenario
		if e = json.Unmarshal(scan.Bytes(), &s); e != nil {
			return nil, nil, e
		}
		if s.ID == "" || seen[s.ID] {
			return nil, nil, errors.New("invalid scenario ID")
		}
		seen[s.ID] = true
		out = append(out, s)
	}
	return out, b, scan.Err()
}
func encode(v any) []byte {
	b, e := json.Marshal(v)
	if e != nil {
		panic(e)
	}
	return b
}
func digest(b []byte) ports.Digest {
	h := sha256.Sum256(b)
	return ports.Digest{Algorithm: "sha256", Value: hex.EncodeToString(h[:])}
}
func reference(kind ports.ArtifactKind, id string) ports.ArtifactRef {
	return ports.ArtifactRef{WorkspaceID: "composition-eval", Kind: kind, ID: id, Version: "1.0.0"}
}

type counters struct {
	CatalogGets     int `json:"catalog_gets"`
	ArtifactOpens   int `json:"artifact_opens"`
	Authorizations  int `json:"authorizations"`
	PolicyChecks    int `json:"policy_checks"`
	IndexLookups    int `json:"index_lookups"`
	AuthorityChecks int `json:"authority_checks"`
}
type artifact struct {
	Pin    ports.ArtifactPin   `json:"pin"`
	Record ports.CatalogRecord `json:"record"`
	Bytes  []byte              `json:"bytes_base64"`
}
type fixtureStore struct {
	records map[ports.ArtifactRef]ports.CatalogRecord
	data    map[ports.ArtifactRef][]byte
	denied  map[ports.ArtifactRef]bool
	counts  *counters
}

func (s *fixtureStore) add(r ports.ArtifactRef, b []byte) ports.ArtifactPin {
	pin := ports.ArtifactPin{Ref: r, Digest: digest(b), ManifestDigest: digest(b)}
	s.data[r] = b
	s.records[r] = ports.CatalogRecord{Ref: r, State: "published", Digest: pin.Digest, DocumentDigest: pin.Digest, Metadata: b}
	return pin
}
func (s *fixtureStore) Get(ctx context.Context, r ports.ArtifactRef) (ports.CatalogRecord, error) {
	s.counts.CatalogGets++
	if e := ctx.Err(); e != nil {
		return ports.CatalogRecord{}, e
	}
	v, ok := s.records[r]
	if !ok {
		return v, errors.New("exact artifact absent")
	}
	return v, nil
}
func (*fixtureStore) Search(context.Context, ports.SearchQuery) (ports.SearchPage, error) {
	return ports.SearchPage{}, errors.New("request-time catalog scans forbidden")
}
func (*fixtureStore) Put(context.Context, ports.Digest, io.Reader, int64) error {
	return errors.New("immutable fixture")
}

type reader struct{ *bytes.Reader }

func (reader) Close() error { return nil }
func (s *fixtureStore) Open(ctx context.Context, r ports.ArtifactRef) (ports.ArtifactReader, error) {
	s.counts.ArtifactOpens++
	if e := ctx.Err(); e != nil {
		return nil, e
	}
	b, ok := s.data[r]
	if !ok {
		return nil, errors.New("artifact absent")
	}
	return reader{bytes.NewReader(b)}, nil
}
func (s *fixtureStore) Decide(ctx context.Context, p ports.Principal, _ ports.Action, r *ports.ArtifactRef) (ports.Decision, error) {
	s.counts.Authorizations++
	if e := ctx.Err(); e != nil {
		return ports.Decision{}, e
	}
	return ports.Decision{Allowed: r != nil && r.WorkspaceID == p.WorkspaceID && p.Subject == "operator" && !s.denied[*r]}, nil
}

type fixedClock struct{ now time.Time }

func (k fixedClock) Now() time.Time { return k.now }

type policy struct {
	scope   d.IndexScope
	counts  *counters
	revoked bool
}

func (p *policy) Check(ctx context.Context, s d.IndexScope) error {
	p.counts.PolicyChecks++
	if e := ctx.Err(); e != nil {
		return e
	}
	if p.revoked || string(encode(s)) != string(encode(p.scope)) {
		return errors.New("current policy denied")
	}
	return nil
}

type authority struct {
	config   configuration
	clock    fixedClock
	scenario string
	counts   *counters
}

func (a *authority) Check(ctx context.Context, r c.AuthorityRequest) (c.Decision, error) {
	a.counts.AuthorityChecks++
	if e := ctx.Err(); e != nil {
		return c.Decision{}, e
	}
	v := c.Decision{ScopeDigest: r.ScopeDigest, PolicyGeneration: a.config.Composition.Policy, TaskGeneration: a.config.Composition.TaskGeneration, Ready: true, CheckedAt: a.clock.now, ValidUntil: a.clock.now.Add(time.Duration(a.config.Composition.ValidFor) * time.Second)}
	if a.scenario == "revoked_final_authority" && a.counts.AuthorityChecks > 1 {
		v.Ready = false
	}
	if a.scenario == "stale_authority" {
		v.CheckedAt = a.clock.now.Add(-time.Duration(a.config.Composition.StaleAge) * time.Second)
	}
	var task map[string]string
	_ = json.Unmarshal(r.Task, &task)
	if task["query"] != "is:unread" {
		v.Ready = false
	}
	return v, nil
}

type countedIndex struct {
	index  *d.ScopedIndex
	counts *counters
}

func (i countedIndex) Lookup(ctx context.Context, q c.Query) (c.Candidates, error) {
	i.counts.IndexLookups++
	return i.index.Lookup(ctx, q)
}

type fixture struct {
	input       scenario
	store       *fixtureStore
	request     c.Request
	entries     []d.IndexEntry
	candidates  map[string][]c.Candidate
	clock       fixedClock
	policy      *policy
	authority   *authority
	config      configuration
	conformance map[string][]byte
}

func makeFixture(config configuration, input scenario) (*fixture, error) {
	files, e := inboxcomposition.Files()
	if e != nil {
		return nil, e
	}
	archive, e := inboxcomposition.Archive()
	if e != nil {
		return nil, e
	}
	doc, e := c.Parse(files[c.AssetPath])
	if e != nil {
		return nil, e
	}
	now, e := time.Parse(time.RFC3339, config.Composition.FixedClock)
	if e != nil {
		return nil, e
	}
	counts := &counters{}
	s := &fixtureStore{records: map[ports.ArtifactRef]ports.CatalogRecord{}, data: map[ports.ArtifactRef][]byte{}, denied: map[ports.ArtifactRef]bool{}, counts: counts}
	scope := d.IndexScope{Principal: ports.Principal{Subject: "operator", WorkspaceID: "composition-eval", Scopes: []string{"catalog:read"}, PolicyGeneration: 1}, RuntimeID: "composition-eval-runtime"}
	f := &fixture{input: input, store: s, clock: fixedClock{now}, policy: &policy{scope: scope, counts: counts}, config: config, candidates: map[string][]c.Candidate{}, conformance: map[string][]byte{}}
	f.authority = &authority{config: config, clock: f.clock, scenario: input.Scenario, counts: counts}
	skillRef := reference(ports.KindSkill, inboxcomposition.SkillID)
	skillRef.Version = inboxcomposition.Version
	skill := s.add(skillRef, archive)
	skill.ManifestDigest = digest(files["manifest.json"])
	record := s.records[skillRef]
	record.ManifestDigest = skill.ManifestDigest
	record.DocumentDigest = ports.Digest{}
	record.Metadata = files["manifest.json"]
	s.records[skillRef] = record
	f.request = c.Request{Principal: scope.Principal, RuntimeID: scope.RuntimeID, Skill: skill, Task: append(json.RawMessage(nil), config.Composition.Task...), SupportedVersions: []string{c.Version}, Limits: doc.Limits, MaxBytes: config.Composition.MaxBytes, Selections: map[string]ports.ArtifactPin{}, ConnectionSelections: map[string]c.Connection{}}
	provider := s.add(reference(ports.KindProvider, "eval/provider"), encode(map[string]any{"id": "eval/provider", "version": "1.0.0"}))
	pairs := 1
	switch input.Scenario {
	case "ambiguous_two_pairs", "selected_second_pair", "selected_second_account":
		pairs = 2
	case "sixty_five_alternatives":
		pairs = 65
	case "selected_of_sixty_four":
		pairs = 64
	}
	for _, r := range doc.Requirements {
		raw := files["assets/"+r.Key+"-contract.json"]
		s.add(reference(ports.KindCapability, r.CapabilityID), raw)
		var schemas struct {
			Input  json.RawMessage `json:"input_schema"`
			Output json.RawMessage `json:"output_schema"`
		}
		if e = json.Unmarshal(raw, &schemas); e != nil {
			return nil, e
		}
		conformance := encode(map[string]any{"caller_adapter": config.Composition.CallerAdapter, "binding_adapter_version": config.Composition.Adapter, "capability_id": r.CapabilityID, "contract_version": r.ContractVersion, "contract_digest": r.ContractDigest, "input_schema": schemas.Input, "output_schema": schemas.Output, "fixture": "canonical structured synthetic identity; no provider qualification"})
		fd := digest(conformance)
		f.conformance[fd.Value] = conformance
		if input.Scenario == "missing_read" && r.Key == "read" {
			continue
		}
		for n := 0; n < pairs; n++ {
			id := fmt.Sprintf("eval/%s/pair-%02d/%s", input.ID, n+1, r.Key)
			tool := s.add(reference(ports.KindTool, id), encode(map[string]any{"id": id, "version": "1.0.0", "input_schema": schemas.Input, "output_schema": schemas.Output}))
			binding := s.add(reference(ports.KindBinding, id), encode(map[string]any{"id": id, "version": "1.0.0", "capability_ref": r.CapabilityID + "@" + r.ContractVersion, "tool_ref": id + "@1.0.0", "provider_ref": "eval/provider@1.0.0", "adapter_version": config.Composition.Adapter, "fixture_digest": fd, "conformance": "passed", "exact_versions": true}))
			connection := c.Connection{Scope: scope.Principal.WorkspaceID, ID: "mailbox-1", Version: "1", AccountID: "account-1", Generation: "1"}
			if input.Scenario == "different_accounts" && r.Key == "read" {
				connection.AccountID = "account-2"
			}
			if input.Scenario == "different_connections" && r.Key == "read" {
				connection.ID = "mailbox-2"
			}
			if input.Scenario == "selected_second_account" && n == 1 {
				connection.ID = "mailbox-2"
				connection.AccountID = "account-2"
			}
			candidate := c.Candidate{Binding: binding, Tool: tool, Provider: provider, ContractDigest: r.ContractDigest, AdapterVersion: config.Composition.Adapter, FixtureDigest: fd.Value, Connection: connection, Admitted: true}
			indexRequirement := r
			if input.Scenario == "wrong_contract_digest" && r.Key == "read" {
				candidate.ContractDigest = digest([]byte("wrong admitted contract")).Value
				indexRequirement.ContractDigest = candidate.ContractDigest
			}
			f.candidates[r.Key] = append(f.candidates[r.Key], candidate)
			indexedRecord := s.records[binding.Ref]
			if input.Scenario == "private_candidate" && r.Key == "read" {
				s.denied[binding.Ref] = true
				s.denied[tool.Ref] = true
				indexedRecord.Metadata = encode(map[string]any{"summary": privateSentinel})
				s.records[binding.Ref] = indexedRecord
			}
			f.entries = append(f.entries, d.IndexEntry{Record: indexedRecord, Bindings: []d.IndexedBinding{{Requirement: indexRequirement, Candidate: candidate}}})
			if input.Scenario == "tool_bytes_drift" && r.Key == "read" {
				s.data[tool.Ref] = append(append([]byte(nil), s.data[tool.Ref]...), ' ')
			}
		}
	}
	switch input.Scenario {
	case "selected_second_pair", "selected_second_account", "selected_of_sixty_four":
		for key, items := range f.candidates {
			selected := items[len(items)-1]
			f.request.Selections[key] = selected.Binding
			if input.Scenario == "selected_second_account" {
				f.request.ConnectionSelections[key] = selected.Connection
			}
		}
	case "unsupported_consumer":
		f.request.SupportedVersions = nil
	case "context_budget_512":
		f.request.MaxBytes = 512
	case "package_bytes_drift":
		s.data[skill.Ref] = append(append([]byte(nil), s.data[skill.Ref]...), ' ')
	case "task_policy_denied":
		f.request.Task = json.RawMessage(`{"query":"all"}`)
	case "one_pair", "missing_read", "wrong_contract_digest", "different_accounts", "different_connections", "ambiguous_two_pairs", "sixty_five_alternatives", "revoked_final_authority", "tool_bytes_drift", "private_candidate", "cancelled_request", "stale_authority":
	default:
		return nil, fmt.Errorf("unknown scenario %q", input.Scenario)
	}
	return f, nil
}
func (f *fixture) service(ctx context.Context) (c.Service, error) {
	// Build only the principal's visible summary partition. Private records stay
	// in the exact store and inventory, and are denied by current retrieval too.
	visible := make([]d.IndexEntry, 0, len(f.entries))
	for _, entry := range f.entries {
		if !f.store.denied[entry.Record.Ref] {
			visible = append(visible, entry)
		}
	}
	index, e := d.NewScopedIndex(ctx, f.policy.scope, f.store, f.policy, visible)
	if e != nil {
		return c.Service{}, e
	}
	return c.Service{Retriever: packages.Retriever{Catalog: f.store, Artifacts: f.store, Authorizer: f.store}, Index: countedIndex{index: index, counts: f.store.counts}, Authority: f.authority, Clock: f.clock, MaxAge: time.Duration(f.config.Composition.MaxAge) * time.Second}, nil
}

type caseInventory struct {
	Controls     map[string]any           `json:"runtime_controls"`
	Input        scenario                 `json:"input"`
	Request      c.Request                `json:"request"`
	Artifacts    []artifact               `json:"artifacts"`
	Candidates   map[string][]c.Candidate `json:"admitted_candidates"`
	IndexEntries []d.IndexEntry           `json:"index_entries_before_visibility_filter"`
	Denied       []ports.ArtifactRef      `json:"currently_denied_refs"`
	Conformance  map[string][]byte        `json:"conformance_bytes_base64"`
}

func inventory(config configuration, configBytes, inputBytes []byte, inputs []scenario) (any, error) {
	cases := []caseInventory{}
	for _, input := range inputs {
		f, e := makeFixture(config, input)
		if e != nil {
			return nil, e
		}
		item := caseInventory{Input: input, Request: f.request, Candidates: f.candidates, IndexEntries: f.entries, Denied: []ports.ArtifactRef{}, Conformance: f.conformance}
		item.Controls = map[string]any{"fixed_clock_utc": config.Composition.FixedClock, "authority_max_age_seconds": config.Composition.MaxAge, "authority_valid_for_seconds": config.Composition.ValidFor, "initial_task_query": "is:unread", "task_policy": "query must equal is:unread", "cancel_before_prepare": input.Scenario == "cancelled_request", "revoke_authority_after_check": 0, "checked_at_seconds_before_now": 0, "visibility_policy": "exclude denied refs before scoped index build; deny exact retrieval"}
		if input.Scenario == "revoked_final_authority" {
			item.Controls["revoke_authority_after_check"] = 1
		}
		if input.Scenario == "stale_authority" {
			item.Controls["checked_at_seconds_before_now"] = config.Composition.StaleAge
		}
		for ref, record := range f.store.records {
			pin := ports.ArtifactPin{Ref: ref, Digest: record.Digest, ManifestDigest: record.ManifestDigest}
			item.Artifacts = append(item.Artifacts, artifact{Pin: pin, Record: record, Bytes: f.store.data[ref]})
			if f.store.denied[ref] {
				item.Denied = append(item.Denied, ref)
			}
		}
		sort.Slice(item.Artifacts, func(i, j int) bool {
			return string(encode(item.Artifacts[i].Pin.Ref)) < string(encode(item.Artifacts[j].Pin.Ref))
		})
		sort.Slice(item.Denied, func(i, j int) bool { return string(encode(item.Denied[i])) < string(encode(item.Denied[j])) })
		cases = append(cases, item)
	}
	return struct {
		Schema              string          `json:"schema_version"`
		ConfigurationSHA256 string          `json:"configuration_sha256"`
		InputsSHA256        string          `json:"inputs_sha256"`
		Cases               []caseInventory `json:"cases"`
		PrivateSentinel     string          `json:"private_sentinel"`
		Notes               []string        `json:"notes"`
	}{"gist.composition-inventory/v1", digest(configBytes).Value, digest(inputBytes).Value, cases, privateSentinel, []string{"Inventory only: no index lookup or composition preparation executed; no expected labels.", "Exact document bytes are base64. Drift cases retain original pins and alter stored bytes deliberately.", "Private read summaries are excluded from the operator's visible index partition and current exact retrieval denies their refs."}}, nil
}

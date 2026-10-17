package main

import (
	"context"
	"encoding/json"
	"os"
	"path/filepath"
	"testing"

	c "github.com/sirerun/gist/catalog/composition"
	d "github.com/sirerun/gist/catalog/discovery"
	"github.com/sirerun/gist/catalog/ports"
)

func testConfig(t *testing.T) configuration {
	t.Helper()
	cfg, _, e := loadConfig("../../composition-cohort/primary-config.json")
	if e != nil {
		t.Fatal(e)
	}
	return cfg
}
func TestInventoryConstructionDoesNotEvaluate(t *testing.T) {
	cfg := testConfig(t)
	inputs, raw, e := loadInputs("../../composition-cohort/inputs.jsonl")
	if e != nil {
		t.Fatal(e)
	}
	for _, input := range inputs {
		f, e := makeFixture(cfg, input)
		if e != nil {
			t.Fatal(e)
		}
		if *f.store.counts != (counters{}) {
			t.Fatal("inventory construction invoked runtime")
		}
		if input.Scenario == "selected_second_account" {
			for _, key := range []string{"list", "read"} {
				if f.request.ConnectionSelections[key].AccountID != "account-2" {
					t.Fatalf("missing per-requirement connection selection %s", key)
				}
			}
			if len(f.request.ConnectionSelections) != 2 {
				t.Fatal("unknown group-keyed connection selection")
			}
		}
	}
	first, e := inventory(cfg, []byte("config"), raw, inputs)
	if e != nil {
		t.Fatal(e)
	}
	second, e := inventory(cfg, []byte("config"), raw, inputs)
	if e != nil {
		t.Fatal(e)
	}
	if string(encode(first)) != string(encode(second)) {
		t.Fatal("inventory generation nondeterministic")
	}
}
func TestSourceSmokeActualPreparation(t *testing.T) {
	// One baseline source smoke only; never the scored nineteen-case cohort.
	cfg := testConfig(t)
	f, e := makeFixture(cfg, scenario{ID: "source-smoke", Scenario: "one_pair"})
	if e != nil {
		t.Fatal(e)
	}
	service, e := f.service(context.Background())
	if e != nil {
		t.Fatal(e)
	}
	result, e := service.Prepare(context.Background(), f.request)
	if e != nil {
		t.Fatal(e)
	}
	if result.Status != "ready" || result.Bundle == nil || len(result.Bundle.Tools) != 2 {
		t.Fatalf("actual source seam: %+v", result)
	}
	if f.store.counts.IndexLookups == 0 || f.store.counts.AuthorityChecks != 2 || f.store.counts.ArtifactOpens == 0 {
		t.Fatalf("missing real ports: %+v", f.store.counts)
	}
}
func TestScaleDatasetPreservesEachPrivateMixture(t *testing.T) {
	cfg := testConfig(t)
	parts, e := scaleDataset(cfg, 100, 10)
	if e != nil {
		t.Fatal(e)
	}
	for _, p := range parts {
		counts := map[ports.ArtifactKind]int{}
		for _, entry := range p.entries {
			counts[entry.Record.Ref.Kind]++
			if entry.Record.Ref.WorkspaceID != p.scope.Principal.WorkspaceID {
				t.Fatal("cross-partition ref")
			}
		}
		if counts[ports.KindTool] != 4 || counts[ports.KindSkill] != 3 || counts[ports.KindCapability] != 1 || counts[ports.KindBinding] != 2 {
			t.Fatalf("partition mixture: %+v", counts)
		}
	}
}
func TestSourceSmokeScaleQueriesAndProjection(t *testing.T) {
	cfg := testConfig(t)
	parts, e := scaleDataset(cfg, 100, 1)
	if e != nil {
		t.Fatal(e)
	}
	p := &parts[0]
	p.index, e = d.NewScopedIndex(context.Background(), p.scope, p.auth, p.auth, p.entries)
	if e != nil {
		t.Fatal(e)
	}
	for _, class := range cfg.QueryClasses {
		result := scaleQuery(context.Background(), p, class, 0)
		if result.Error != "" {
			t.Fatalf("%s: %+v", class, result)
		}
	}
	size, count, e := measureProjection(context.Background(), parts)
	if e != nil || count != 100 || size <= 0 {
		t.Fatalf("projection %d %d %v", size, count, e)
	}
	// Cross-check streaming byte accounting against the actual exact projection.
	candidates := []d.Candidate{}
	for _, entry := range p.entries {
		candidate, ok, e := p.index.Exact(context.Background(), p.scope, entry.Record.Ref)
		if e != nil || !ok {
			t.Fatal("exact projection failed")
		}
		candidates = append(candidates, candidate)
	}
	actual := encode([]any{struct {
		Scope      d.IndexScope  `json:"scope"`
		Generation uint64        `json:"generation"`
		Candidates []d.Candidate `json:"candidates"`
	}{p.scope, p.generation, candidates}})
	if int64(len(actual)) != size {
		t.Fatalf("projection bytes %d != %d", size, len(actual))
	}
}
func TestFreezeRejectsChangedListedBytes(t *testing.T) {
	dir := t.TempDir()
	old, e := os.Getwd()
	if e != nil {
		t.Fatal(e)
	}
	if e = os.Chdir(dir); e != nil {
		t.Fatal(e)
	}
	t.Cleanup(func() {
		if e := os.Chdir(old); e != nil {
			t.Error(e)
		}
	})
	cfg := []byte("config")
	inputs := []byte("inputs")
	if e = os.WriteFile("config.json", cfg, 0600); e != nil {
		t.Fatal(e)
	}
	if e = os.WriteFile("inputs.jsonl", inputs, 0600); e != nil {
		t.Fatal(e)
	}
	manifest := freezeManifest{Schema: "gist.composition-freeze/v1", Files: map[string]string{"config.json": digest(cfg).Value, "inputs.jsonl": digest(inputs).Value}}
	path := filepath.Join(dir, "freeze.json")
	if e = os.WriteFile(path, encode(manifest), 0600); e != nil {
		t.Fatal(e)
	}
	if e = verifyFreeze(path, "config.json", "inputs.jsonl"); e != nil {
		t.Fatal(e)
	}
	if e = os.WriteFile("inputs.jsonl", []byte("changed"), 0600); e != nil {
		t.Fatal(e)
	}
	if verifyFreeze(path, "config.json", "inputs.jsonl") == nil {
		t.Fatal("changed freeze accepted")
	}
}
func TestInventoryKeepsOriginalPinsInDriftCase(t *testing.T) {
	f, e := makeFixture(testConfig(t), scenario{ID: "drift-construction", Scenario: "tool_bytes_drift"})
	if e != nil {
		t.Fatal(e)
	}
	read := f.candidates["read"][0]
	if digest(f.store.data[read.Tool.Ref]) == read.Tool.Digest {
		t.Fatal("drift input condition missing")
	}
	var raw map[string]any
	if e = json.Unmarshal(f.store.data[read.Tool.Ref], &raw); e != nil {
		t.Fatal(e)
	}
	if f.request.SupportedVersions[0] != c.Version {
		t.Fatal("fixture negotiation changed")
	}
}

package seed

import (
	"encoding/json"
	"os"
	"path/filepath"
	"strings"
	"testing"
)

func TestCatalog(t *testing.T) {
	root := filepath.Join("..", "..", "..", "catalog", "registry")
	providers := []struct {
		ID           string `json:"id"`
		SupportState string `json:"support_state"`
	}{}
	read := func(name string, dst any) {
		t.Helper()
		b, err := os.ReadFile(filepath.Join(root, name))
		if err != nil {
			t.Fatal(err)
		}
		if err := json.Unmarshal(b, dst); err != nil {
			t.Fatal(err)
		}
	}
	read("providers.json", &providers)
	if len(providers) != 3 {
		t.Fatalf("provider count = %d, want 3", len(providers))
	}
	for _, p := range providers {
		if p.SupportState != "catalog_only" {
			t.Errorf("provider %s is executable/resolvable", p.ID)
		}
	}
	classes := map[string]bool{}
	var catalog struct {
		Providers []struct{ ID, Class string } `json:"providers"`
	}
	read("catalog.json", &catalog)
	for _, p := range catalog.Providers {
		classes[p.Class] = true
	}
	for _, class := range []string{"aggregator", "mcp", "direct_api"} {
		if !classes[class] {
			t.Errorf("missing route class %q", class)
		}
	}

	var fixtures []catalogFixture
	read("goldens.json", &fixtures)
	var bindings []catalogBinding
	read("bindings.json", &bindings)
	if len(fixtures) != len(bindings) {
		t.Fatalf("fixture/binding count mismatch")
	}
	for i, f := range fixtures {
		if len(f.Cases) < 5 {
			t.Errorf("fixture %s has %d cases; want at least 5", f.BindingID, len(f.Cases))
		}
		if f.FixtureVersion != "1" || f.BindingVersion == "" {
			t.Errorf("invalid fixture version metadata for %s", f.BindingID)
		}
		digest, err := canonicalDigest(f)
		if err != nil {
			t.Fatal(err)
		}
		if bindings[i].FixtureDigest.Algorithm != "sha256" || bindings[i].FixtureDigest.Value != digest {
			t.Errorf("fixture digest mismatch for %s", f.BindingID)
		}
		if bindings[i].Conformance != "passed" || !bindings[i].ExactVersions {
			t.Errorf("binding %s is not a passing exact-version binding", bindings[i].ID)
		}
	}
	b, err := os.ReadFile(filepath.Join(root, "catalog.json"))
	if err != nil {
		t.Fatal(err)
	}
	if strings.Contains(strings.ToLower(string(b)), "token") || strings.Contains(strings.ToLower(string(b)), "secret") {
		t.Fatal("catalog contains credential-like field")
	}
}

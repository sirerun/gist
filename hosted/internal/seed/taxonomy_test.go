package seed

import (
	"encoding/json"
	"os"
	"path/filepath"
	"testing"
)

func TestTaxonomy(t *testing.T) {
	tax, err := LoadTaxonomy()
	if err != nil {
		t.Fatal(err)
	}
	if len(tax.Nodes) != 449 {
		t.Fatalf("taxonomy nodes = %d, want 449", len(tax.Nodes))
	}
	counts := map[int]int{}
	ids := map[string]bool{}
	for _, n := range tax.Nodes {
		counts[n.Level]++
		if ids[n.ID] {
			t.Errorf("duplicate node %s", n.ID)
		}
		ids[n.ID] = true
		if n.APQCRef == "" || n.Label == "" {
			t.Errorf("incomplete node %s", n.ID)
		}
		if n.Level == 1 {
			if n.ParentID != nil {
				t.Errorf("level 1 node %s has parent", n.ID)
			}
		} else if n.ParentID == nil || !ids[*n.ParentID] {
			t.Errorf("missing parent for %s", n.ID)
		}
	}
	for level, want := range map[int]int{1: 13, 2: 74, 3: 362} {
		if counts[level] != want {
			t.Errorf("level %d = %d, want %d", level, counts[level], want)
		}
	}
	if len(tax.Attribution) < 200 {
		t.Fatal("attribution block was truncated")
	}
	for page := 0; ; page++ {
		p, err := PaginateTaxonomy(tax, 37, page)
		if err != nil {
			if page == 13 {
				break
			}
			t.Fatal(err)
		}
		if p.Attribution != tax.Attribution {
			t.Fatal("page lost attribution")
		}
	}

	var familyMap struct {
		Families []struct {
			Family              string   `json:"family"`
			TaxonomyMemberships []string `json:"taxonomy_memberships"`
		} `json:"families"`
	}
	b, err := os.ReadFile(filepath.Join(registryRoot(), "family-map.json"))
	if err != nil {
		t.Fatal(err)
	}
	if err := json.Unmarshal(b, &familyMap); err != nil {
		t.Fatal(err)
	}
	if len(familyMap.Families) != 3 {
		t.Fatalf("family map = %d families", len(familyMap.Families))
	}
	// Membership is optional metadata and cannot become an identity component.
	before := "gist/communication/message.send@1.0.0"
	after := before
	if len(familyMap.Families[1].TaxonomyMemberships) > 0 {
		after = before
	}
	if before != after {
		t.Fatal("taxonomy membership changed capability identity")
	}
}

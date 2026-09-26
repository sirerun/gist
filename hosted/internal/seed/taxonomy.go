package seed

import (
	"encoding/json"
	"fmt"
	"os"
)

type TaxonomyNode struct {
	ID       string  `json:"id"`
	Level    int     `json:"level"`
	Label    string  `json:"label"`
	ParentID *string `json:"parent_id"`
	APQCRef  string  `json:"apqc_ref"`
}

type Taxonomy struct {
	ID          string         `json:"id"`
	Edition     string         `json:"edition"`
	Attribution string         `json:"attribution"`
	Nodes       []TaxonomyNode `json:"nodes"`
}

type TaxonomyPage struct {
	ID          string         `json:"id"`
	Edition     string         `json:"edition"`
	Attribution string         `json:"attribution"`
	Nodes       []TaxonomyNode `json:"nodes"`
}

func LoadTaxonomy() (Taxonomy, error) {
	b, err := os.ReadFile(registryRoot() + "/taxonomy.json")
	if err != nil {
		return Taxonomy{}, fmt.Errorf("read taxonomy export: %w", err)
	}
	var t Taxonomy
	if err := json.Unmarshal(b, &t); err != nil {
		return Taxonomy{}, fmt.Errorf("decode taxonomy export: %w", err)
	}
	return t, nil
}

func PaginateTaxonomy(t Taxonomy, pageSize, page int) (TaxonomyPage, error) {
	if pageSize <= 0 || page < 0 {
		return TaxonomyPage{}, fmt.Errorf("invalid taxonomy page")
	}
	start := page * pageSize
	if start > len(t.Nodes) {
		return TaxonomyPage{}, fmt.Errorf("taxonomy page out of range")
	}
	end := start + pageSize
	if end > len(t.Nodes) {
		end = len(t.Nodes)
	}
	return TaxonomyPage{ID: t.ID, Edition: t.Edition, Attribution: t.Attribution, Nodes: t.Nodes[start:end]}, nil
}

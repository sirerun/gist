package rest

import (
	"context"
	"fmt"
	"net/http"
	"strings"
	"testing"

	"github.com/sirerun/gist/hosted/internal/ports"
)

type rankedSearch struct{ records []ports.CatalogRecord }

func (s rankedSearch) Search(context.Context, ports.SearchQuery) (ports.SearchPage, error) {
	return ports.SearchPage{Records: s.records}, nil
}

// A budget that cannot hold every ranked candidate reduces the candidate
// count from the tail; it does not fail a discovery that fits with fewer.
func TestDiscoverBudgetDropsLowestRankedCandidates(t *testing.T) {
	var records []ports.CatalogRecord
	for i := 0; i < 20; i++ {
		records = append(records, ports.CatalogRecord{Ref: ports.ArtifactRef{WorkspaceID: "w", Kind: ports.KindSkill, ID: fmt.Sprintf("skill-%02d", i), Version: "1.0.0"}, State: "published", Metadata: []byte(`{"summary":"` + strings.Repeat("x", 200) + `"}`)})
	}
	h := handlerWith(t, func(s *Services) { s.Search = rankedSearch{records: records}; s.Limits.MaxResponseBytes = 1 << 20 })
	w := do(t, h, "POST", "/v1/discover", `{"query":"x","max_bytes":2000}`)
	if w.Code != http.StatusOK {
		t.Fatalf("status=%d body=%s", w.Code, w.Body.String())
	}
	if w.Body.Len() > 2000 {
		t.Fatalf("response %d bytes exceeds the 2000-byte budget", w.Body.Len())
	}
	var out struct {
		Items []struct{ Ref ports.ArtifactRef } `json:"items"`
	}
	decodeJSON(t, w, &out)
	if len(out.Items) == 0 || len(out.Items) == len(records) {
		t.Fatalf("expected a reduced, non-empty candidate list, got %d", len(out.Items))
	}
	for i, item := range out.Items {
		if item.Ref.ID != records[i].Ref.ID {
			t.Fatalf("candidate %d is %s, want ranked prefix %s", i, item.Ref.ID, records[i].Ref.ID)
		}
	}
}

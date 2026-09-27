package rest

import (
	"testing"

	"github.com/sirerun/gist/hosted/internal/ports"
)

// Regression: listVersions returned every version with an empty next_cursor,
// so an artifact with many versions overran max_bytes and could not be read.
func TestListVersionsPagesWithKeysetCursor(t *testing.T) {
	h := handlerWith(t, func(*Services) {})
	var page struct {
		Items      []ports.CatalogRecord `json:"items"`
		NextCursor string                `json:"next_cursor"`
	}
	var seen []string
	path := "/v1/skills/wanted/versions?limit=2"
	for pages := 0; ; pages++ {
		if pages > 3 {
			t.Fatal("paging did not terminate")
		}
		w := do(t, h, "GET", path, "")
		if w.Code != 200 {
			t.Fatalf("status=%d body=%s", w.Code, w.Body.String())
		}
		page.Items, page.NextCursor = nil, ""
		decodeJSON(t, w, &page)
		if len(page.Items) > 2 {
			t.Fatalf("page has %d items, limit is 2", len(page.Items))
		}
		for _, it := range page.Items {
			seen = append(seen, it.Ref.Version)
		}
		if page.NextCursor == "" {
			break
		}
		path = "/v1/skills/wanted/versions?limit=2&cursor=" + page.NextCursor
	}
	if len(seen) != 3 || seen[0] != "1" || seen[1] != "2" || seen[2] != "3" {
		t.Fatalf("versions across pages = %v, want [1 2 3]", seen)
	}
}

func TestListVersionsRejectsBadPagingParameters(t *testing.T) {
	h := handlerWith(t, func(*Services) {})
	for _, q := range []string{"limit=0", "limit=x", "cursor=%25%25", "cursor=" + string(make([]byte, 0))} {
		path := "/v1/skills/wanted/versions?" + q
		w := do(t, h, "GET", path, "")
		if q == "cursor=" {
			if w.Code != 200 {
				t.Fatalf("%s: empty cursor should mean first page, got %d", q, w.Code)
			}
			continue
		}
		if w.Code != 422 {
			t.Fatalf("%s: status=%d want 422 body=%s", q, w.Code, w.Body.String())
		}
	}
}

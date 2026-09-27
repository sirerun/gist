package discovery

import (
	"fmt"
	"testing"

	"github.com/sirerun/gist/hosted/internal/ports"
)

func TestCacheEvictsLeastRecentlyUsedBeyondBound(t *testing.T) {
	c := newBoundedCache(2)
	rec := []ports.CatalogRecord{{}}
	c.put("a", rec, ports.Cursor{})
	c.put("b", rec, ports.Cursor{})
	if _, _, ok := c.get("a"); !ok { // a becomes most recently used
		t.Fatal("a should be cached")
	}
	c.put("c", rec, ports.Cursor{})
	if _, _, ok := c.get("b"); ok {
		t.Fatal("b was least recently used and should have been evicted")
	}
	for _, k := range []string{"a", "c"} {
		if _, _, ok := c.get(k); !ok {
			t.Fatalf("%s should still be cached", k)
		}
	}
	for i := 0; i < 100; i++ {
		c.put(fmt.Sprintf("k%d", i), rec, ports.Cursor{})
	}
	if got := c.size(); got != 2 {
		t.Fatalf("size = %d, want bound 2", got)
	}
	if got := len(c.values); got != 2 {
		t.Fatalf("index size = %d, want 2", got)
	}
}

func TestCachePutOverwritesWithoutGrowing(t *testing.T) {
	c := newBoundedCache(4)
	c.put("a", nil, ports.Cursor{})
	c.put("a", []ports.CatalogRecord{{}}, ports.Cursor{})
	records, _, ok := c.get("a")
	if !ok || len(records) != 1 || c.size() != 1 {
		t.Fatalf("overwrite: ok=%v records=%d size=%d", ok, len(records), c.size())
	}
}

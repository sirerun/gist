package discovery

import (
	"sync"

	"github.com/sirerun/gist/hosted/internal/ports"
)

type cache struct {
	mu     sync.RWMutex
	values map[string]cacheEntry
}

type cacheEntry struct {
	records []ports.CatalogRecord
	next    ports.Cursor
}

// cache stores no authorization decision. The service always re-runs policy
// on a hit, so revocation cannot turn a cached index result into a disclosure.
func newCache() *cache { return &cache{values: make(map[string]cacheEntry)} }

func (c *cache) get(key string) ([]ports.CatalogRecord, ports.Cursor, bool) {
	c.mu.RLock()
	defer c.mu.RUnlock()
	entry, ok := c.values[key]
	return append([]ports.CatalogRecord(nil), entry.records...), entry.next, ok
}

func (c *cache) put(key string, records []ports.CatalogRecord, next ports.Cursor) {
	c.mu.Lock()
	defer c.mu.Unlock()
	c.values[key] = cacheEntry{records: append([]ports.CatalogRecord(nil), records...), next: next}
}

package discovery

import (
	"container/list"
	"sync"

	"github.com/sirerun/gist/hosted/internal/ports"
)

// defaultCacheEntries bounds the discovery cache. Keys include the query and
// cursor, so an unbounded map grows with every distinct search a client sends.
const defaultCacheEntries = 1024

type cache struct {
	mu     sync.Mutex
	max    int
	order  *list.List // front = most recently used; values are keys
	values map[string]*list.Element
}

type cacheEntry struct {
	key     string
	records []ports.CatalogRecord
	next    ports.Cursor
}

// cache stores no authorization decision. The service always re-runs policy
// on a hit, so revocation cannot turn a cached index result into a disclosure.
// It holds at most max entries and evicts the least recently used one.
func newCache() *cache { return newBoundedCache(defaultCacheEntries) }

func newBoundedCache(max int) *cache {
	if max <= 0 {
		max = defaultCacheEntries
	}
	return &cache{max: max, order: list.New(), values: make(map[string]*list.Element)}
}

func (c *cache) get(key string) ([]ports.CatalogRecord, ports.Cursor, bool) {
	c.mu.Lock()
	defer c.mu.Unlock()
	el, ok := c.values[key]
	if !ok {
		return nil, ports.Cursor{}, false
	}
	c.order.MoveToFront(el)
	entry := el.Value.(cacheEntry)
	return append([]ports.CatalogRecord(nil), entry.records...), entry.next, true
}

func (c *cache) put(key string, records []ports.CatalogRecord, next ports.Cursor) {
	c.mu.Lock()
	defer c.mu.Unlock()
	entry := cacheEntry{key: key, records: append([]ports.CatalogRecord(nil), records...), next: next}
	if el, ok := c.values[key]; ok {
		el.Value = entry
		c.order.MoveToFront(el)
		return
	}
	c.values[key] = c.order.PushFront(entry)
	for c.order.Len() > c.max {
		oldest := c.order.Back()
		c.order.Remove(oldest)
		delete(c.values, oldest.Value.(cacheEntry).key)
	}
}

func (c *cache) size() int {
	c.mu.Lock()
	defer c.mu.Unlock()
	return c.order.Len()
}

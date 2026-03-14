package gist

import (
	"context"
	"errors"
	"sort"
	"strings"
	"sync"
)

// MemoryStore is an in-memory Store implementation for testing, prototyping,
// and small workloads that do not require PostgreSQL. It is thread-safe and
// implements word-level and substring search using Go standard library
// string operations. Data is ephemeral — it does not persist across restarts.
type MemoryStore struct {
	mu       sync.RWMutex
	sources  []Source
	chunks   []Chunk
	nextSrc  int
	nextChk  int
	closed   bool
}

// NewMemoryStore creates a new empty MemoryStore.
func NewMemoryStore() *MemoryStore {
	return &MemoryStore{}
}

// SaveSource creates a new source record with an auto-incremented ID.
func (m *MemoryStore) SaveSource(_ context.Context, label string, format Format) (Source, error) {
	m.mu.Lock()
	defer m.mu.Unlock()
	if m.closed {
		return Source{}, errStoreClosed
	}
	m.nextSrc++
	s := Source{
		ID:     m.nextSrc,
		Label:  label,
		Format: format,
	}
	m.sources = append(m.sources, s)
	return s, nil
}

// SaveChunk persists a chunk and updates the parent source's byte count.
func (m *MemoryStore) SaveChunk(_ context.Context, chunk Chunk) (Chunk, error) {
	m.mu.Lock()
	defer m.mu.Unlock()
	if m.closed {
		return Chunk{}, errStoreClosed
	}
	m.nextChk++
	chunk.ID = m.nextChk
	m.chunks = append(m.chunks, chunk)
	for i := range m.sources {
		if m.sources[i].ID == chunk.SourceID {
			m.sources[i].ChunkCount++
			m.sources[i].BytesIndexed += int64(len(chunk.Content))
			break
		}
	}
	return chunk, nil
}

// SearchPorter performs word-level matching. It tokenizes query and chunk
// content into lowercase words, scores by the fraction of query words found,
// and returns results sorted by score descending.
func (m *MemoryStore) SearchPorter(_ context.Context, params SearchParams) ([]SearchMatch, error) {
	m.mu.RLock()
	defer m.mu.RUnlock()
	if m.closed {
		return nil, errStoreClosed
	}

	queryWords := strings.Fields(strings.ToLower(params.Query))
	if len(queryWords) == 0 {
		return nil, nil
	}

	type scored struct {
		match SearchMatch
		score float64
	}
	var results []scored

	for _, c := range m.chunks {
		if !m.matchesSourceFilter(c.SourceID, params.SourceFilter) {
			continue
		}
		contentWords := strings.Fields(strings.ToLower(c.Content))
		wordSet := make(map[string]struct{}, len(contentWords))
		for _, w := range contentWords {
			wordSet[w] = struct{}{}
		}

		matched := 0
		for _, qw := range queryWords {
			if _, ok := wordSet[qw]; ok {
				matched++
			}
		}
		if matched == 0 {
			continue
		}

		score := float64(matched) / float64(len(contentWords))
		if len(contentWords) == 0 {
			score = 0
		}
		results = append(results, scored{
			match: SearchMatch{
				ChunkID:     c.ID,
				SourceID:    c.SourceID,
				HeadingPath: c.HeadingPath,
				Content:     c.Content,
				ContentType: c.ContentType,
				Score:       score,
				MatchLayer:  "porter",
			},
			score: score,
		})
	}

	sort.Slice(results, func(i, j int) bool {
		return results[i].score > results[j].score
	})

	limit := params.Limit
	if limit <= 0 || limit > len(results) {
		limit = len(results)
	}

	matches := make([]SearchMatch, limit)
	for i := 0; i < limit; i++ {
		matches[i] = results[i].match
	}
	return matches, nil
}

// SearchTrigram performs substring matching. It returns chunks whose
// lowercase content contains the lowercase query, scored by the ratio
// of query length to content length.
func (m *MemoryStore) SearchTrigram(_ context.Context, params SearchParams) ([]SearchMatch, error) {
	m.mu.RLock()
	defer m.mu.RUnlock()
	if m.closed {
		return nil, errStoreClosed
	}

	lowerQuery := strings.ToLower(params.Query)
	if lowerQuery == "" {
		return nil, nil
	}

	type scored struct {
		match SearchMatch
		score float64
	}
	var results []scored

	for _, c := range m.chunks {
		if !m.matchesSourceFilter(c.SourceID, params.SourceFilter) {
			continue
		}
		lowerContent := strings.ToLower(c.Content)
		if !strings.Contains(lowerContent, lowerQuery) {
			continue
		}
		score := float64(len(lowerQuery)) / float64(len(lowerContent))
		if len(lowerContent) == 0 {
			score = 0
		}
		results = append(results, scored{
			match: SearchMatch{
				ChunkID:     c.ID,
				SourceID:    c.SourceID,
				HeadingPath: c.HeadingPath,
				Content:     c.Content,
				ContentType: c.ContentType,
				Score:       score,
				MatchLayer:  "trigram",
			},
			score: score,
		})
	}

	sort.Slice(results, func(i, j int) bool {
		return results[i].score > results[j].score
	})

	limit := params.Limit
	if limit <= 0 || limit > len(results) {
		limit = len(results)
	}

	matches := make([]SearchMatch, limit)
	for i := 0; i < limit; i++ {
		matches[i] = results[i].match
	}
	return matches, nil
}

// VocabularyTerms returns unique lowercase words across all chunk content.
func (m *MemoryStore) VocabularyTerms(_ context.Context) ([]string, error) {
	m.mu.RLock()
	defer m.mu.RUnlock()
	if m.closed {
		return nil, errStoreClosed
	}
	seen := make(map[string]struct{})
	for _, c := range m.chunks {
		for _, word := range strings.Fields(c.Content) {
			seen[strings.ToLower(word)] = struct{}{}
		}
	}
	terms := make([]string, 0, len(seen))
	for t := range seen {
		terms = append(terms, t)
	}
	return terms, nil
}

// Sources returns a copy of all indexed sources.
func (m *MemoryStore) Sources(_ context.Context) ([]Source, error) {
	m.mu.RLock()
	defer m.mu.RUnlock()
	if m.closed {
		return nil, errStoreClosed
	}
	result := make([]Source, len(m.sources))
	copy(result, m.sources)
	return result, nil
}

// Stats returns aggregate statistics about the store contents.
func (m *MemoryStore) Stats(_ context.Context) (StoreStats, error) {
	m.mu.RLock()
	defer m.mu.RUnlock()
	if m.closed {
		return StoreStats{}, errStoreClosed
	}
	var bytes int64
	for _, s := range m.sources {
		bytes += s.BytesIndexed
	}
	return StoreStats{
		ChunkCount:   len(m.chunks),
		SourceCount:  len(m.sources),
		BytesIndexed: bytes,
	}, nil
}

// Close marks the store as closed. Subsequent operations return an error.
func (m *MemoryStore) Close() error {
	m.mu.Lock()
	defer m.mu.Unlock()
	m.closed = true
	return nil
}

// matchesSourceFilter checks whether a chunk's source matches the filter.
// An empty filter matches all sources. Must be called with mu held.
func (m *MemoryStore) matchesSourceFilter(sourceID int, filter string) bool {
	if filter == "" {
		return true
	}
	for _, s := range m.sources {
		if s.ID == sourceID && s.Label == filter {
			return true
		}
	}
	return false
}

// errStoreClosed is returned when operations are attempted on a closed store.
var errStoreClosed = errors.New("gist: store is closed")

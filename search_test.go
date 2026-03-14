package gist

import (
	"context"
	"fmt"
	"strings"
	"testing"
)

// searchMockStore is a mock Store for search tests that allows controlling
// which tiers return results.
type searchMockStore struct {
	sources       []Source
	chunks        []Chunk
	porterResults []SearchMatch
	trigramResults []SearchMatch
	porterErr     error
	trigramErr    error
}

func (m *searchMockStore) SaveSource(_ context.Context, label string, format Format) (Source, error) {
	s := Source{ID: len(m.sources) + 1, Label: label, Format: format}
	m.sources = append(m.sources, s)
	return s, nil
}

func (m *searchMockStore) SaveChunk(_ context.Context, chunk Chunk) (Chunk, error) {
	chunk.ID = len(m.chunks) + 1
	m.chunks = append(m.chunks, chunk)
	return chunk, nil
}

func (m *searchMockStore) SearchPorter(_ context.Context, params SearchParams) ([]SearchMatch, error) {
	if m.porterErr != nil {
		return nil, m.porterErr
	}
	return filterMatches(m.porterResults, params), nil
}

func (m *searchMockStore) SearchTrigram(_ context.Context, params SearchParams) ([]SearchMatch, error) {
	if m.trigramErr != nil {
		return nil, m.trigramErr
	}
	return filterMatches(m.trigramResults, params), nil
}

func filterMatches(matches []SearchMatch, params SearchParams) []SearchMatch {
	var out []SearchMatch
	for _, m := range matches {
		if params.SourceFilter != "" && m.MatchLayer != "" {
			// Simple filter: skip if doesn't match (handled by caller setup)
		}
		out = append(out, m)
		if params.Limit > 0 && len(out) >= params.Limit {
			break
		}
	}
	return out
}

func (m *searchMockStore) VocabularyTerms(_ context.Context) ([]string, error) {
	seen := make(map[string]struct{})
	for _, c := range m.chunks {
		for _, w := range strings.Fields(c.Content) {
			seen[strings.ToLower(w)] = struct{}{}
		}
	}
	terms := make([]string, 0, len(seen))
	for t := range seen {
		terms = append(terms, t)
	}
	return terms, nil
}

func (m *searchMockStore) Sources(_ context.Context) ([]Source, error) {
	result := make([]Source, len(m.sources))
	copy(result, m.sources)
	return result, nil
}

func (m *searchMockStore) Stats(_ context.Context) (StoreStats, error) {
	return StoreStats{ChunkCount: len(m.chunks), SourceCount: len(m.sources)}, nil
}

func (m *searchMockStore) Close() error { return nil }

var _ Store = (*searchMockStore)(nil)

func TestSearch_PorterFound(t *testing.T) {
	store := &searchMockStore{
		sources: []Source{{ID: 1, Label: "docs.md"}},
		porterResults: []SearchMatch{
			{ChunkID: 1, SourceID: 1, HeadingPath: "Setup", Content: "database connection pooling config", ContentType: "prose", Score: 2.5, MatchLayer: "porter"},
		},
	}
	vocab := NewVocabulary()
	searcher := NewSearcher(store, vocab)

	results, err := searcher.Search(context.Background(), "database")
	if err != nil {
		t.Fatalf("Search() error = %v", err)
	}
	if len(results) != 1 {
		t.Fatalf("Search() returned %d results, want 1", len(results))
	}
	if results[0].MatchLayer != "porter" {
		t.Errorf("MatchLayer = %q, want %q", results[0].MatchLayer, "porter")
	}
	if results[0].Title != "Setup" {
		t.Errorf("Title = %q, want %q", results[0].Title, "Setup")
	}
	if results[0].Source != "docs.md" {
		t.Errorf("Source = %q, want %q", results[0].Source, "docs.md")
	}
	if results[0].Score != 2.5 {
		t.Errorf("Score = %f, want 2.5", results[0].Score)
	}
}

func TestSearch_TrigramFallback(t *testing.T) {
	store := &searchMockStore{
		sources:       []Source{{ID: 1, Label: "code.go"}},
		porterResults: nil, // No porter results → triggers trigram fallback.
		trigramResults: []SearchMatch{
			{ChunkID: 2, SourceID: 1, HeadingPath: "Handler", Content: "func handleRequest() error", ContentType: "code", Score: 1.8, MatchLayer: "trigram"},
		},
	}
	vocab := NewVocabulary()
	searcher := NewSearcher(store, vocab)

	results, err := searcher.Search(context.Background(), "handle")
	if err != nil {
		t.Fatalf("Search() error = %v", err)
	}
	if len(results) != 1 {
		t.Fatalf("Search() returned %d results, want 1", len(results))
	}
	if results[0].MatchLayer != "trigram" {
		t.Errorf("MatchLayer = %q, want %q", results[0].MatchLayer, "trigram")
	}
}

func TestSearch_FuzzyFallback(t *testing.T) {
	store := &searchMockStore{
		sources:        []Source{{ID: 1, Label: "readme.md"}},
		porterResults:  nil,
		trigramResults: nil,
	}
	// The fuzzy search will correct "databse" → "database", then the corrected
	// query re-searches. We need to make the store return results for the
	// corrected query. We use a custom store for this.
	fuzzyStore := &fuzzySearchMockStore{
		sources:       store.sources,
		correctedTerm: "database",
		correctedResults: []SearchMatch{
			{ChunkID: 3, SourceID: 1, HeadingPath: "DB", Content: "database configuration", ContentType: "prose", Score: 1.2, MatchLayer: "porter"},
		},
	}

	vocab := NewVocabulary()
	vocab.Add("database", "connection", "pooling")

	searcher := NewSearcher(fuzzyStore, vocab)

	results, err := searcher.Search(context.Background(), "databse") // typo
	if err != nil {
		t.Fatalf("Search() error = %v", err)
	}
	if len(results) != 1 {
		t.Fatalf("Search() returned %d results, want 1", len(results))
	}
	if results[0].MatchLayer != "fuzzy" {
		t.Errorf("MatchLayer = %q, want %q", results[0].MatchLayer, "fuzzy")
	}
}

// fuzzySearchMockStore returns no results for the original query but returns
// results when the corrected term is used.
type fuzzySearchMockStore struct {
	sources          []Source
	correctedTerm    string
	correctedResults []SearchMatch
}

func (m *fuzzySearchMockStore) SaveSource(_ context.Context, label string, format Format) (Source, error) {
	return Source{}, nil
}
func (m *fuzzySearchMockStore) SaveChunk(_ context.Context, chunk Chunk) (Chunk, error) {
	return Chunk{}, nil
}
func (m *fuzzySearchMockStore) SearchPorter(_ context.Context, params SearchParams) ([]SearchMatch, error) {
	if params.Query == m.correctedTerm {
		return m.correctedResults, nil
	}
	return nil, nil
}
func (m *fuzzySearchMockStore) SearchTrigram(_ context.Context, params SearchParams) ([]SearchMatch, error) {
	return nil, nil
}
func (m *fuzzySearchMockStore) VocabularyTerms(_ context.Context) ([]string, error) {
	return nil, nil
}
func (m *fuzzySearchMockStore) Sources(_ context.Context) ([]Source, error) {
	result := make([]Source, len(m.sources))
	copy(result, m.sources)
	return result, nil
}
func (m *fuzzySearchMockStore) Stats(_ context.Context) (StoreStats, error) {
	return StoreStats{}, nil
}
func (m *fuzzySearchMockStore) Close() error { return nil }

var _ Store = (*fuzzySearchMockStore)(nil)

func TestSearch_EmptyResults(t *testing.T) {
	store := &searchMockStore{
		sources: []Source{{ID: 1, Label: "empty.md"}},
	}
	vocab := NewVocabulary() // Empty vocab → no fuzzy matches either.
	searcher := NewSearcher(store, vocab)

	results, err := searcher.Search(context.Background(), "nonexistent")
	if err != nil {
		t.Fatalf("Search() error = %v", err)
	}
	if len(results) != 0 {
		t.Errorf("Search() returned %d results, want 0", len(results))
	}
}

func TestSearch_WithLimit(t *testing.T) {
	store := &searchMockStore{
		sources: []Source{{ID: 1, Label: "docs.md"}},
		porterResults: []SearchMatch{
			{ChunkID: 1, SourceID: 1, HeadingPath: "A", Content: "result one", ContentType: "prose", Score: 3.0, MatchLayer: "porter"},
			{ChunkID: 2, SourceID: 1, HeadingPath: "B", Content: "result two", ContentType: "prose", Score: 2.0, MatchLayer: "porter"},
			{ChunkID: 3, SourceID: 1, HeadingPath: "C", Content: "result three", ContentType: "prose", Score: 1.0, MatchLayer: "porter"},
		},
	}
	vocab := NewVocabulary()
	searcher := NewSearcher(store, vocab)

	tests := []struct {
		name  string
		limit int
		want  int
	}{
		{name: "limit 1", limit: 1, want: 1},
		{name: "limit 2", limit: 2, want: 2},
		{name: "limit 10 with 3 results", limit: 10, want: 3},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			results, err := searcher.Search(context.Background(), "result", WithLimit(tt.limit))
			if err != nil {
				t.Fatalf("Search() error = %v", err)
			}
			if len(results) != tt.want {
				t.Errorf("Search() returned %d results, want %d", len(results), tt.want)
			}
		})
	}
}

func TestSearch_WithSourceFilter(t *testing.T) {
	// The source filter is passed through to the store's SearchParams.
	// We verify the searcher propagates it correctly.
	store := &sourceFilterMockStore{
		sources: []Source{
			{ID: 1, Label: "file-a.md"},
			{ID: 2, Label: "file-b.md"},
		},
		chunks: []SearchMatch{
			{ChunkID: 1, SourceID: 1, HeadingPath: "A", Content: "shared content alpha", ContentType: "prose", Score: 1.0, MatchLayer: "porter"},
			{ChunkID: 2, SourceID: 2, HeadingPath: "B", Content: "shared content beta", ContentType: "prose", Score: 1.0, MatchLayer: "porter"},
		},
	}
	vocab := NewVocabulary()
	searcher := NewSearcher(store, vocab)

	results, err := searcher.Search(context.Background(), "shared", WithSourceFilter("file-a.md"))
	if err != nil {
		t.Fatalf("Search() error = %v", err)
	}
	if len(results) != 1 {
		t.Fatalf("Search() returned %d results, want 1", len(results))
	}
	if results[0].Source != "file-a.md" {
		t.Errorf("Source = %q, want %q", results[0].Source, "file-a.md")
	}
}

// sourceFilterMockStore filters porter results by SourceFilter matching source label.
type sourceFilterMockStore struct {
	sources []Source
	chunks  []SearchMatch
}

func (m *sourceFilterMockStore) SaveSource(_ context.Context, label string, format Format) (Source, error) {
	return Source{}, nil
}
func (m *sourceFilterMockStore) SaveChunk(_ context.Context, chunk Chunk) (Chunk, error) {
	return Chunk{}, nil
}
func (m *sourceFilterMockStore) SearchPorter(_ context.Context, params SearchParams) ([]SearchMatch, error) {
	if params.SourceFilter == "" {
		return m.chunks, nil
	}
	var out []SearchMatch
	for _, c := range m.chunks {
		for _, s := range m.sources {
			if s.ID == c.SourceID && s.Label == params.SourceFilter {
				out = append(out, c)
				break
			}
		}
		if params.Limit > 0 && len(out) >= params.Limit {
			break
		}
	}
	return out, nil
}
func (m *sourceFilterMockStore) SearchTrigram(_ context.Context, _ SearchParams) ([]SearchMatch, error) {
	return nil, nil
}
func (m *sourceFilterMockStore) VocabularyTerms(_ context.Context) ([]string, error) {
	return nil, nil
}
func (m *sourceFilterMockStore) Sources(_ context.Context) ([]Source, error) {
	result := make([]Source, len(m.sources))
	copy(result, m.sources)
	return result, nil
}
func (m *sourceFilterMockStore) Stats(_ context.Context) (StoreStats, error) { return StoreStats{}, nil }
func (m *sourceFilterMockStore) Close() error                                { return nil }

var _ Store = (*sourceFilterMockStore)(nil)

func TestSearch_WithBudget(t *testing.T) {
	// Each snippet is roughly the content itself for short strings.
	// "short" = 5 bytes → ~2 tokens. "a]longer piece of content here" = 31 bytes → ~8 tokens.
	store := &searchMockStore{
		sources: []Source{{ID: 1, Label: "docs.md"}},
		porterResults: []SearchMatch{
			{ChunkID: 1, SourceID: 1, HeadingPath: "A", Content: "short", ContentType: "prose", Score: 3.0, MatchLayer: "porter"},
			{ChunkID: 2, SourceID: 1, HeadingPath: "B", Content: "a longer piece of content here for testing budget limits in search", ContentType: "prose", Score: 2.0, MatchLayer: "porter"},
			{ChunkID: 3, SourceID: 1, HeadingPath: "C", Content: "third result with some content", ContentType: "prose", Score: 1.0, MatchLayer: "porter"},
		},
	}
	vocab := NewVocabulary()
	searcher := NewSearcher(store, vocab)

	// Budget of 5 tokens = 20 bytes. "short" (5 bytes = 2 tokens) fits.
	// The second result (66 bytes = 17 tokens) would push total to 19 which fits,
	// but the third would exceed. Use a tighter budget.
	results, err := searcher.Search(context.Background(), "test", WithBudget(3), WithLimit(10))
	if err != nil {
		t.Fatalf("Search() error = %v", err)
	}

	// With budget=3 (12 bytes), first result "short" (2 tokens) fits.
	// Second result is 17 tokens which would push total to 19 > 3, so it stops.
	// But the first result alone always gets included even if it exceeds budget.
	if len(results) < 1 {
		t.Fatal("Search() should return at least 1 result")
	}
	if len(results) >= 3 {
		t.Errorf("Search() returned %d results, expected budget to trim some", len(results))
	}
}

func TestSearch_WithSnippetLen(t *testing.T) {
	longContent := strings.Repeat("word ", 500) // 2500 bytes
	store := &searchMockStore{
		sources: []Source{{ID: 1, Label: "long.md"}},
		porterResults: []SearchMatch{
			{ChunkID: 1, SourceID: 1, HeadingPath: "Long", Content: longContent, ContentType: "prose", Score: 1.0, MatchLayer: "porter"},
		},
	}
	vocab := NewVocabulary()
	searcher := NewSearcher(store, vocab)

	results, err := searcher.Search(context.Background(), "word", WithSnippetLen(100))
	if err != nil {
		t.Fatalf("Search() error = %v", err)
	}
	if len(results) != 1 {
		t.Fatalf("Search() returned %d results, want 1", len(results))
	}
	if len(results[0].Snippet) > 100 {
		t.Errorf("Snippet length = %d, want <= 100", len(results[0].Snippet))
	}
}

func TestSearch_NilVocabulary(t *testing.T) {
	store := &searchMockStore{
		sources: []Source{{ID: 1, Label: "empty.md"}},
	}
	searcher := NewSearcher(store, nil)

	results, err := searcher.Search(context.Background(), "nonexistent")
	if err != nil {
		t.Fatalf("Search() error = %v", err)
	}
	if len(results) != 0 {
		t.Errorf("Search() returned %d results, want 0", len(results))
	}
}

func TestSearch_PorterError(t *testing.T) {
	store := &searchMockStore{
		sources:   []Source{{ID: 1, Label: "docs.md"}},
		porterErr: fmt.Errorf("connection refused"),
	}
	searcher := NewSearcher(store, NewVocabulary())

	_, err := searcher.Search(context.Background(), "test")
	if err == nil {
		t.Fatal("Search() expected error, got nil")
	}
	if !strings.Contains(err.Error(), "connection refused") {
		t.Errorf("error = %q, want to contain %q", err.Error(), "connection refused")
	}
}

func TestSearch_TrigramError(t *testing.T) {
	store := &searchMockStore{
		sources:    []Source{{ID: 1, Label: "docs.md"}},
		trigramErr: fmt.Errorf("timeout"),
	}
	searcher := NewSearcher(store, NewVocabulary())

	_, err := searcher.Search(context.Background(), "test")
	if err == nil {
		t.Fatal("Search() expected error, got nil")
	}
}

func TestSearchResult_Fields(t *testing.T) {
	r := SearchResult{
		Title:       "Config > DB",
		Snippet:     "connection pool size = 10",
		Source:      "config.md",
		Score:       3.14,
		ContentType: "prose",
		MatchLayer:  "porter",
	}
	if r.Title != "Config > DB" {
		t.Errorf("Title = %q", r.Title)
	}
	if r.MatchLayer != "porter" {
		t.Errorf("MatchLayer = %q", r.MatchLayer)
	}
}

func TestSearchOption_Defaults(t *testing.T) {
	cfg := defaultSearchConfig()
	if cfg.limit != 5 {
		t.Errorf("default limit = %d, want 5", cfg.limit)
	}
	if cfg.snippetLen != DefaultMaxSnippetLen {
		t.Errorf("default snippetLen = %d, want %d", cfg.snippetLen, DefaultMaxSnippetLen)
	}
	if cfg.budget != 0 {
		t.Errorf("default budget = %d, want 0", cfg.budget)
	}
	if cfg.sourceFilter != "" {
		t.Errorf("default sourceFilter = %q, want empty", cfg.sourceFilter)
	}
}

func TestFindMatchPositions(t *testing.T) {
	tests := []struct {
		name    string
		content string
		terms   []string
		want    int
	}{
		{name: "single match", content: "hello world", terms: []string{"hello"}, want: 1},
		{name: "multiple matches", content: "hello hello hello", terms: []string{"hello"}, want: 3},
		{name: "no match", content: "hello world", terms: []string{"foo"}, want: 0},
		{name: "empty terms", content: "hello world", terms: nil, want: 0},
		{name: "case insensitive", content: "Hello World", terms: []string{"hello"}, want: 1},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			positions := findMatchPositions(tt.content, tt.terms)
			if len(positions) != tt.want {
				t.Errorf("findMatchPositions() returned %d positions, want %d", len(positions), tt.want)
			}
		})
	}
}

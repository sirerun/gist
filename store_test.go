package gist

import (
	"context"
	"fmt"
	"strings"
	"testing"
)

// mockStore is an in-memory Store implementation used to verify the interface
// is implementable and test the supporting types.
type mockStore struct {
	sources []Source
	chunks  []Chunk
	closed  bool
}

func newMockStore() *mockStore {
	return &mockStore{}
}

func (m *mockStore) SaveSource(_ context.Context, label string, format Format) (Source, error) {
	if m.closed {
		return Source{}, fmt.Errorf("store is closed")
	}
	s := Source{
		ID:     len(m.sources) + 1,
		Label:  label,
		Format: format,
	}
	m.sources = append(m.sources, s)
	return s, nil
}

func (m *mockStore) SaveChunk(_ context.Context, chunk Chunk) (Chunk, error) {
	if m.closed {
		return Chunk{}, fmt.Errorf("store is closed")
	}
	chunk.ID = len(m.chunks) + 1
	m.chunks = append(m.chunks, chunk)
	// Update source stats.
	for i := range m.sources {
		if m.sources[i].ID == chunk.SourceID {
			m.sources[i].ChunkCount++
			m.sources[i].BytesIndexed += int64(len(chunk.Content))
			break
		}
	}
	return chunk, nil
}

func (m *mockStore) SearchPorter(_ context.Context, params SearchParams) ([]SearchMatch, error) {
	if m.closed {
		return nil, fmt.Errorf("store is closed")
	}
	return m.search(params, "porter"), nil
}

func (m *mockStore) SearchTrigram(_ context.Context, params SearchParams) ([]SearchMatch, error) {
	if m.closed {
		return nil, fmt.Errorf("store is closed")
	}
	return m.search(params, "trigram"), nil
}

func (m *mockStore) search(params SearchParams, layer string) []SearchMatch {
	var matches []SearchMatch
	for _, c := range m.chunks {
		if params.SourceFilter != "" {
			found := false
			for _, s := range m.sources {
				if s.ID == c.SourceID && s.Label == params.SourceFilter {
					found = true
					break
				}
			}
			if !found {
				continue
			}
		}
		if strings.Contains(strings.ToLower(c.Content), strings.ToLower(params.Query)) {
			matches = append(matches, SearchMatch{
				ChunkID:     c.ID,
				SourceID:    c.SourceID,
				HeadingPath: c.HeadingPath,
				Content:     c.Content,
				ContentType: c.ContentType,
				Score:       1.0,
				MatchLayer:  layer,
			})
		}
		if params.Limit > 0 && len(matches) >= params.Limit {
			break
		}
	}
	return matches
}

func (m *mockStore) VocabularyTerms(_ context.Context) ([]string, error) {
	if m.closed {
		return nil, fmt.Errorf("store is closed")
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

func (m *mockStore) Sources(_ context.Context) ([]Source, error) {
	if m.closed {
		return nil, fmt.Errorf("store is closed")
	}
	result := make([]Source, len(m.sources))
	copy(result, m.sources)
	return result, nil
}

func (m *mockStore) Stats(_ context.Context) (StoreStats, error) {
	if m.closed {
		return StoreStats{}, fmt.Errorf("store is closed")
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

func (m *mockStore) Close() error {
	m.closed = true
	return nil
}

// Compile-time check that mockStore implements Store.
var _ Store = (*mockStore)(nil)

func TestFormat_String(t *testing.T) {
	tests := []struct {
		name   string
		format Format
		want   string
	}{
		{name: "markdown", format: FormatMarkdown, want: "markdown"},
		{name: "json", format: FormatJSON, want: "json"},
		{name: "plaintext", format: FormatPlainText, want: "plaintext"},
		{name: "unknown", format: Format(99), want: "unknown"},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			if got := tt.format.String(); got != tt.want {
				t.Errorf("Format.String() = %q, want %q", got, tt.want)
			}
		})
	}
}

func TestStore_SaveSourceAndChunk(t *testing.T) {
	ctx := context.Background()
	s := newMockStore()
	defer s.Close()

	tests := []struct {
		name       string
		label      string
		format     Format
		content    string
		wantSrcID  int
		wantChunks int
	}{
		{
			name:       "first source with one chunk",
			label:      "readme.md",
			format:     FormatMarkdown,
			content:    "# Hello World",
			wantSrcID:  1,
			wantChunks: 1,
		},
		{
			name:       "second source with one chunk",
			label:      "config.json",
			format:     FormatJSON,
			content:    `{"key": "value"}`,
			wantSrcID:  2,
			wantChunks: 1,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			src, err := s.SaveSource(ctx, tt.label, tt.format)
			if err != nil {
				t.Fatalf("SaveSource() error = %v", err)
			}
			if src.ID != tt.wantSrcID {
				t.Errorf("SaveSource() ID = %d, want %d", src.ID, tt.wantSrcID)
			}
			if src.Label != tt.label {
				t.Errorf("SaveSource() Label = %q, want %q", src.Label, tt.label)
			}

			chunk, err := s.SaveChunk(ctx, Chunk{
				SourceID:    src.ID,
				HeadingPath: "root",
				Content:     tt.content,
				ContentType: "prose",
				Format:      tt.format,
				ByteStart:   0,
				ByteEnd:     len(tt.content),
			})
			if err != nil {
				t.Fatalf("SaveChunk() error = %v", err)
			}
			if chunk.ID == 0 {
				t.Error("SaveChunk() returned chunk with zero ID")
			}
			if chunk.SourceID != src.ID {
				t.Errorf("SaveChunk() SourceID = %d, want %d", chunk.SourceID, src.ID)
			}
		})
	}
}

func TestStore_SearchPorter(t *testing.T) {
	ctx := context.Background()
	s := newMockStore()
	defer s.Close()

	src, _ := s.SaveSource(ctx, "docs.md", FormatMarkdown)
	s.SaveChunk(ctx, Chunk{SourceID: src.ID, Content: "database connection pooling", ContentType: "prose"})
	s.SaveChunk(ctx, Chunk{SourceID: src.ID, Content: "authentication and authorization", ContentType: "prose"})

	tests := []struct {
		name      string
		params    SearchParams
		wantCount int
		wantLayer string
	}{
		{
			name:      "matching query",
			params:    SearchParams{Query: "database", Limit: 10},
			wantCount: 1,
			wantLayer: "porter",
		},
		{
			name:      "no match",
			params:    SearchParams{Query: "kubernetes", Limit: 10},
			wantCount: 0,
		},
		{
			name:      "limit results",
			params:    SearchParams{Query: "a", Limit: 1},
			wantCount: 1,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			matches, err := s.SearchPorter(ctx, tt.params)
			if err != nil {
				t.Fatalf("SearchPorter() error = %v", err)
			}
			if len(matches) != tt.wantCount {
				t.Errorf("SearchPorter() returned %d matches, want %d", len(matches), tt.wantCount)
			}
			if tt.wantLayer != "" {
				for _, m := range matches {
					if m.MatchLayer != tt.wantLayer {
						t.Errorf("match layer = %q, want %q", m.MatchLayer, tt.wantLayer)
					}
				}
			}
		})
	}
}

func TestStore_SearchTrigram(t *testing.T) {
	ctx := context.Background()
	s := newMockStore()
	defer s.Close()

	src, _ := s.SaveSource(ctx, "code.go", FormatPlainText)
	s.SaveChunk(ctx, Chunk{SourceID: src.ID, Content: "func handleRequest()", ContentType: "code"})

	matches, err := s.SearchTrigram(ctx, SearchParams{Query: "handle", Limit: 5})
	if err != nil {
		t.Fatalf("SearchTrigram() error = %v", err)
	}
	if len(matches) != 1 {
		t.Fatalf("SearchTrigram() returned %d matches, want 1", len(matches))
	}
	if matches[0].MatchLayer != "trigram" {
		t.Errorf("match layer = %q, want %q", matches[0].MatchLayer, "trigram")
	}
}

func TestStore_SearchWithSourceFilter(t *testing.T) {
	ctx := context.Background()
	s := newMockStore()
	defer s.Close()

	src1, _ := s.SaveSource(ctx, "file-a.md", FormatMarkdown)
	src2, _ := s.SaveSource(ctx, "file-b.md", FormatMarkdown)
	s.SaveChunk(ctx, Chunk{SourceID: src1.ID, Content: "shared term here", ContentType: "prose"})
	s.SaveChunk(ctx, Chunk{SourceID: src2.ID, Content: "shared term there", ContentType: "prose"})

	matches, err := s.SearchPorter(ctx, SearchParams{
		Query:        "shared",
		Limit:        10,
		SourceFilter: "file-a.md",
	})
	if err != nil {
		t.Fatalf("SearchPorter() error = %v", err)
	}
	if len(matches) != 1 {
		t.Errorf("expected 1 match with source filter, got %d", len(matches))
	}
	if len(matches) > 0 && matches[0].SourceID != src1.ID {
		t.Errorf("match source ID = %d, want %d", matches[0].SourceID, src1.ID)
	}
}

func TestStore_VocabularyTerms(t *testing.T) {
	ctx := context.Background()
	s := newMockStore()
	defer s.Close()

	src, _ := s.SaveSource(ctx, "test.md", FormatMarkdown)
	s.SaveChunk(ctx, Chunk{SourceID: src.ID, Content: "hello world", ContentType: "prose"})
	s.SaveChunk(ctx, Chunk{SourceID: src.ID, Content: "hello gist", ContentType: "prose"})

	terms, err := s.VocabularyTerms(ctx)
	if err != nil {
		t.Fatalf("VocabularyTerms() error = %v", err)
	}
	if len(terms) != 3 {
		t.Errorf("VocabularyTerms() returned %d terms, want 3", len(terms))
	}

	termSet := make(map[string]bool)
	for _, term := range terms {
		termSet[term] = true
	}
	for _, want := range []string{"hello", "world", "gist"} {
		if !termSet[want] {
			t.Errorf("VocabularyTerms() missing term %q", want)
		}
	}
}

func TestStore_Sources(t *testing.T) {
	ctx := context.Background()
	s := newMockStore()
	defer s.Close()

	s.SaveSource(ctx, "a.md", FormatMarkdown)
	s.SaveSource(ctx, "b.json", FormatJSON)

	sources, err := s.Sources(ctx)
	if err != nil {
		t.Fatalf("Sources() error = %v", err)
	}
	if len(sources) != 2 {
		t.Errorf("Sources() returned %d, want 2", len(sources))
	}
}

func TestStore_Stats(t *testing.T) {
	ctx := context.Background()
	s := newMockStore()
	defer s.Close()

	src, _ := s.SaveSource(ctx, "doc.md", FormatMarkdown)
	s.SaveChunk(ctx, Chunk{SourceID: src.ID, Content: "twelve chars", ContentType: "prose"})
	s.SaveChunk(ctx, Chunk{SourceID: src.ID, Content: "more content here", ContentType: "prose"})

	stats, err := s.Stats(ctx)
	if err != nil {
		t.Fatalf("Stats() error = %v", err)
	}
	if stats.SourceCount != 1 {
		t.Errorf("Stats().SourceCount = %d, want 1", stats.SourceCount)
	}
	if stats.ChunkCount != 2 {
		t.Errorf("Stats().ChunkCount = %d, want 2", stats.ChunkCount)
	}
	if stats.BytesIndexed != int64(len("twelve chars")+len("more content here")) {
		t.Errorf("Stats().BytesIndexed = %d, want %d", stats.BytesIndexed, len("twelve chars")+len("more content here"))
	}
}

func TestStore_Close(t *testing.T) {
	ctx := context.Background()
	s := newMockStore()

	if err := s.Close(); err != nil {
		t.Fatalf("Close() error = %v", err)
	}

	// All operations should fail after close.
	if _, err := s.SaveSource(ctx, "x", FormatMarkdown); err == nil {
		t.Error("SaveSource() should fail after Close()")
	}
	if _, err := s.SaveChunk(ctx, Chunk{}); err == nil {
		t.Error("SaveChunk() should fail after Close()")
	}
	if _, err := s.SearchPorter(ctx, SearchParams{Query: "x"}); err == nil {
		t.Error("SearchPorter() should fail after Close()")
	}
	if _, err := s.SearchTrigram(ctx, SearchParams{Query: "x"}); err == nil {
		t.Error("SearchTrigram() should fail after Close()")
	}
	if _, err := s.VocabularyTerms(ctx); err == nil {
		t.Error("VocabularyTerms() should fail after Close()")
	}
	if _, err := s.Sources(ctx); err == nil {
		t.Error("Sources() should fail after Close()")
	}
	if _, err := s.Stats(ctx); err == nil {
		t.Error("Stats() should fail after Close()")
	}
}

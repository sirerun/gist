package gist

import (
	"context"
	"errors"
	"strings"
	"sync"
	"testing"
)

// gistMockStore is an in-memory Store implementation for gist_test.go.
// It is thread-safe for use in concurrent tests.
type gistMockStore struct {
	mu      sync.Mutex
	sources []Source
	chunks  []Chunk
	nextSrc int
	nextChk int
	closed  bool
}

func newGistMockStore() *gistMockStore {
	return &gistMockStore{}
}

func (m *gistMockStore) SaveSource(_ context.Context, label string, format Format) (Source, error) {
	m.mu.Lock()
	defer m.mu.Unlock()
	m.nextSrc++
	s := Source{ID: m.nextSrc, Label: label, Format: format}
	m.sources = append(m.sources, s)
	return s, nil
}

func (m *gistMockStore) SaveChunk(_ context.Context, chunk Chunk) (Chunk, error) {
	m.mu.Lock()
	defer m.mu.Unlock()
	m.nextChk++
	chunk.ID = m.nextChk
	m.chunks = append(m.chunks, chunk)
	for i := range m.sources {
		if m.sources[i].ID == chunk.SourceID {
			m.sources[i].BytesIndexed += int64(len(chunk.Content))
			m.sources[i].ChunkCount++
		}
	}
	return chunk, nil
}

func (m *gistMockStore) SearchPorter(_ context.Context, params SearchParams) ([]SearchMatch, error) {
	m.mu.Lock()
	defer m.mu.Unlock()
	var results []SearchMatch
	q := strings.ToLower(params.Query)
	for _, c := range m.chunks {
		if strings.Contains(strings.ToLower(c.Content), q) {
			results = append(results, SearchMatch{
				ChunkID:     c.ID,
				SourceID:    c.SourceID,
				HeadingPath: c.HeadingPath,
				Content:     c.Content,
				ContentType: c.ContentType,
				Score:       1.0,
				MatchLayer:  "porter",
			})
			if params.Limit > 0 && len(results) >= params.Limit {
				break
			}
		}
	}
	return results, nil
}

func (m *gistMockStore) SearchTrigram(_ context.Context, _ SearchParams) ([]SearchMatch, error) {
	return nil, nil
}

func (m *gistMockStore) VocabularyTerms(_ context.Context) ([]string, error) {
	return nil, nil
}

func (m *gistMockStore) Sources(_ context.Context) ([]Source, error) {
	m.mu.Lock()
	defer m.mu.Unlock()
	out := make([]Source, len(m.sources))
	copy(out, m.sources)
	return out, nil
}

func (m *gistMockStore) Stats(_ context.Context) (StoreStats, error) {
	m.mu.Lock()
	defer m.mu.Unlock()
	var total int64
	for _, s := range m.sources {
		total += s.BytesIndexed
	}
	return StoreStats{
		SourceCount:  len(m.sources),
		ChunkCount:   len(m.chunks),
		BytesIndexed: total,
	}, nil
}

func (m *gistMockStore) Close() error {
	m.mu.Lock()
	defer m.mu.Unlock()
	m.closed = true
	return nil
}

func TestNew(t *testing.T) {
	tests := []struct {
		name    string
		opts    []Option
		wantErr string
	}{
		{
			name:    "no store configured",
			opts:    nil,
			wantErr: "a store is required",
		},
		{
			name: "with custom store",
			opts: []Option{WithStore(newGistMockStore())},
		},
		{
			name: "with token budget",
			opts: []Option{WithStore(newGistMockStore()), WithTokenBudget(1000)},
		},
		{
			name: "with project root",
			opts: []Option{WithStore(newGistMockStore()), WithProjectRoot("/tmp")},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			g, err := New(tt.opts...)
			if tt.wantErr != "" {
				if err == nil {
					t.Fatalf("expected error containing %q, got nil", tt.wantErr)
				}
				if !strings.Contains(err.Error(), tt.wantErr) {
					t.Fatalf("expected error containing %q, got %q", tt.wantErr, err.Error())
				}
				return
			}
			if err != nil {
				t.Fatalf("unexpected error: %v", err)
			}
			if g == nil {
				t.Fatal("expected non-nil Gist")
			}
			_ = g.Close()
		})
	}
}

func TestWithPostgresOption(t *testing.T) {
	var cfg config
	opt := WithPostgres("postgres://localhost/test")
	opt(&cfg)
	if cfg.postgresDSN != "postgres://localhost/test" {
		t.Fatalf("expected DSN to be set, got %q", cfg.postgresDSN)
	}
}

func TestIndex(t *testing.T) {
	tests := []struct {
		name        string
		content     string
		opts        []IndexOption
		wantChunks  int
		wantLabel   string
		wantErr     bool
	}{
		{
			name:       "simple markdown",
			content:    "# Hello\n\nWorld",
			wantChunks: 1,
			wantLabel:  "unnamed",
		},
		{
			name:       "with source label",
			content:    "some content",
			opts:       []IndexOption{WithSource("myfile.md")},
			wantChunks: 1,
			wantLabel:  "myfile.md",
		},
		{
			name:       "plain text format",
			content:    "paragraph one\n\nparagraph two",
			opts:       []IndexOption{WithFormat(FormatPlainText)},
			wantChunks: 1,
			wantLabel:  "unnamed",
		},
		{
			name:       "with code block",
			content:    "# API\n\n```go\nfunc main() {}\n```\n\nSome prose here.",
			wantChunks: 1,
			wantLabel:  "unnamed",
		},
		{
			name:       "empty content",
			content:    "",
			wantChunks: 0,
			wantLabel:  "unnamed",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			ms := newGistMockStore()
			g, err := New(WithStore(ms))
			if err != nil {
				t.Fatalf("New: %v", err)
			}
			defer g.Close()

			res, err := g.Index(context.Background(), tt.content, tt.opts...)
			if tt.wantErr {
				if err == nil {
					t.Fatal("expected error, got nil")
				}
				return
			}
			if err != nil {
				t.Fatalf("Index: %v", err)
			}

			if res.TotalChunks != tt.wantChunks {
				t.Errorf("TotalChunks = %d, want %d", res.TotalChunks, tt.wantChunks)
			}
			if res.Label != tt.wantLabel {
				t.Errorf("Label = %q, want %q", res.Label, tt.wantLabel)
			}

			// Verify chunks were saved to the store.
			ms.mu.Lock()
			savedChunks := len(ms.chunks)
			ms.mu.Unlock()
			if savedChunks != tt.wantChunks {
				t.Errorf("store has %d chunks, want %d", savedChunks, tt.wantChunks)
			}
		})
	}
}

func TestIndexUpdatesVocabulary(t *testing.T) {
	ms := newGistMockStore()
	g, err := New(WithStore(ms))
	if err != nil {
		t.Fatalf("New: %v", err)
	}
	defer g.Close()

	_, err = g.Index(context.Background(), "# Title\n\nHello world gist search")
	if err != nil {
		t.Fatalf("Index: %v", err)
	}

	terms := g.vocab.Terms()
	if len(terms) == 0 {
		t.Fatal("expected vocabulary to have terms after indexing")
	}

	// Check that at least one expected term is present.
	found := false
	for _, term := range terms {
		if term == "hello" || term == "world" || term == "gist" {
			found = true
			break
		}
	}
	if !found {
		t.Errorf("expected vocabulary to contain indexed words, got %v", terms)
	}
}

func TestSearch(t *testing.T) {
	ms := newGistMockStore()
	g, err := New(WithStore(ms))
	if err != nil {
		t.Fatalf("New: %v", err)
	}
	defer g.Close()

	_, err = g.Index(context.Background(), "# Database\n\nConnection pool configuration details")
	if err != nil {
		t.Fatalf("Index: %v", err)
	}

	results, err := g.Search(context.Background(), "connection")
	if err != nil {
		t.Fatalf("Search: %v", err)
	}

	if len(results) == 0 {
		t.Fatal("expected at least one search result")
	}
	if results[0].MatchLayer != "porter" {
		t.Errorf("MatchLayer = %q, want %q", results[0].MatchLayer, "porter")
	}
}

func TestSearchUpdatesStats(t *testing.T) {
	ms := newGistMockStore()
	g, err := New(WithStore(ms))
	if err != nil {
		t.Fatalf("New: %v", err)
	}
	defer g.Close()

	_, _ = g.Index(context.Background(), "# Test\n\nSome searchable content here")
	_, _ = g.Search(context.Background(), "searchable")
	_, _ = g.Search(context.Background(), "content")

	stats := g.Stats()
	if stats.SearchCount != 2 {
		t.Errorf("SearchCount = %d, want 2", stats.SearchCount)
	}
}

func TestStats(t *testing.T) {
	ms := newGistMockStore()
	g, err := New(WithStore(ms))
	if err != nil {
		t.Fatalf("New: %v", err)
	}
	defer g.Close()

	// Stats before indexing.
	stats := g.Stats()
	if stats.BytesIndexed != 0 || stats.SourceCount != 0 || stats.ChunkCount != 0 {
		t.Fatalf("expected zero stats before indexing, got %+v", stats)
	}

	content := "# Section\n\nThis is some test content for stats verification."
	_, err = g.Index(context.Background(), content)
	if err != nil {
		t.Fatalf("Index: %v", err)
	}

	stats = g.Stats()
	if stats.BytesIndexed != int64(len(content)) {
		t.Errorf("BytesIndexed = %d, want %d", stats.BytesIndexed, len(content))
	}
	if stats.SourceCount != 1 {
		t.Errorf("SourceCount = %d, want 1", stats.SourceCount)
	}
	if stats.ChunkCount < 1 {
		t.Errorf("ChunkCount = %d, want >= 1", stats.ChunkCount)
	}
	if stats.SearchCount != 0 {
		t.Errorf("SearchCount = %d, want 0", stats.SearchCount)
	}

	// BytesSaved and SavedPercent should be calculated correctly.
	if stats.BytesReturned != 0 {
		t.Errorf("BytesReturned = %d, want 0 (no searches yet)", stats.BytesReturned)
	}
	if stats.SavedPercent != 100 {
		t.Errorf("SavedPercent = %f, want 100", stats.SavedPercent)
	}
}

func TestStatsMultipleIndexCalls(t *testing.T) {
	ms := newGistMockStore()
	g, err := New(WithStore(ms))
	if err != nil {
		t.Fatalf("New: %v", err)
	}
	defer g.Close()

	c1 := "# One\n\nFirst content"
	c2 := "# Two\n\nSecond content"
	_, _ = g.Index(context.Background(), c1)
	_, _ = g.Index(context.Background(), c2)

	stats := g.Stats()
	wantBytes := int64(len(c1) + len(c2))
	if stats.BytesIndexed != wantBytes {
		t.Errorf("BytesIndexed = %d, want %d", stats.BytesIndexed, wantBytes)
	}
	if stats.SourceCount != 2 {
		t.Errorf("SourceCount = %d, want 2", stats.SourceCount)
	}
}

func TestClose(t *testing.T) {
	ms := newGistMockStore()
	g, err := New(WithStore(ms))
	if err != nil {
		t.Fatalf("New: %v", err)
	}

	if err := g.Close(); err != nil {
		t.Fatalf("Close: %v", err)
	}

	ms.mu.Lock()
	closed := ms.closed
	ms.mu.Unlock()
	if !closed {
		t.Fatal("expected store to be closed")
	}
}

func TestIndexOptionDefaults(t *testing.T) {
	ic := defaultIndexConfig()
	if ic.source != "unnamed" {
		t.Errorf("default source = %q, want %q", ic.source, "unnamed")
	}
	if ic.format != FormatMarkdown {
		t.Errorf("default format = %v, want FormatMarkdown", ic.format)
	}
	if ic.maxChunkBytes != defaultMaxChunkBytes {
		t.Errorf("default maxChunkBytes = %d, want %d", ic.maxChunkBytes, defaultMaxChunkBytes)
	}
}

func TestIndexOptionConstructors(t *testing.T) {
	tests := []struct {
		name   string
		opt    IndexOption
		check  func(t *testing.T, ic indexConfig)
	}{
		{
			name: "WithSource",
			opt:  WithSource("test.md"),
			check: func(t *testing.T, ic indexConfig) {
				if ic.source != "test.md" {
					t.Errorf("source = %q, want %q", ic.source, "test.md")
				}
			},
		},
		{
			name: "WithSource empty string ignored",
			opt:  WithSource(""),
			check: func(t *testing.T, ic indexConfig) {
				if ic.source != "unnamed" {
					t.Errorf("source = %q, want %q (empty should be ignored)", ic.source, "unnamed")
				}
			},
		},
		{
			name: "WithFormat",
			opt:  WithFormat(FormatPlainText),
			check: func(t *testing.T, ic indexConfig) {
				if ic.format != FormatPlainText {
					t.Errorf("format = %v, want FormatPlainText", ic.format)
				}
			},
		},
		{
			name: "WithIndexMaxChunkBytes",
			opt:  WithIndexMaxChunkBytes(2048),
			check: func(t *testing.T, ic indexConfig) {
				if ic.maxChunkBytes != 2048 {
					t.Errorf("maxChunkBytes = %d, want 2048", ic.maxChunkBytes)
				}
			},
		},
		{
			name: "WithIndexMaxChunkBytes zero ignored",
			opt:  WithIndexMaxChunkBytes(0),
			check: func(t *testing.T, ic indexConfig) {
				if ic.maxChunkBytes != defaultMaxChunkBytes {
					t.Errorf("maxChunkBytes = %d, want %d (zero should be ignored)", ic.maxChunkBytes, defaultMaxChunkBytes)
				}
			},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			ic := defaultIndexConfig()
			tt.opt(&ic)
			tt.check(t, ic)
		})
	}
}

func TestNewNoStoreError(t *testing.T) {
	_, err := New()
	if err == nil {
		t.Fatal("expected error when no store is configured")
	}
	if !errors.Is(err, err) || !strings.Contains(err.Error(), "store is required") {
		t.Fatalf("unexpected error message: %v", err)
	}
}

func TestConcurrentIndexAndSearch(t *testing.T) {
	ms := newGistMockStore()
	g, err := New(WithStore(ms))
	if err != nil {
		t.Fatalf("New: %v", err)
	}
	defer g.Close()

	// Index some initial content.
	_, _ = g.Index(context.Background(), "# Test\n\nConcurrent access content")

	var wg sync.WaitGroup
	errs := make(chan error, 20)

	// Concurrent indexing.
	for i := 0; i < 10; i++ {
		wg.Add(1)
		go func() {
			defer wg.Done()
			_, err := g.Index(context.Background(), "# Concurrent\n\nSome data")
			if err != nil {
				errs <- err
			}
		}()
	}

	// Concurrent searching.
	for i := 0; i < 10; i++ {
		wg.Add(1)
		go func() {
			defer wg.Done()
			_, err := g.Search(context.Background(), "data")
			if err != nil {
				errs <- err
			}
		}()
	}

	wg.Wait()
	close(errs)

	for err := range errs {
		t.Fatalf("concurrent operation error: %v", err)
	}

	stats := g.Stats()
	if stats.SourceCount != 11 {
		t.Errorf("SourceCount = %d, want 11", stats.SourceCount)
	}
}

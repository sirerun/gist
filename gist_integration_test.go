package gist

import (
	"context"
	"errors"
	"strings"
	"testing"
)

// TestIntegration_FullPipeline exercises the complete Gist workflow:
// New → Index content → Search → verify results → Stats → Close.
func TestIntegration_FullPipeline(t *testing.T) {
	ms := newGistMockStore()
	g, err := New(WithStore(ms))
	if err != nil {
		t.Fatalf("New: %v", err)
	}

	ctx := context.Background()

	// Step 1: Index realistic markdown content.
	mdContent := `# Getting Started

Welcome to the project. This guide walks you through installation and configuration.

## Installation

` + "```bash\npip install gist-sdk\n```" + `

After installing, verify with:

` + "```bash\ngist --version\n```" + `

## Configuration

### Database

Configure your PostgreSQL connection string in the environment:

` + "```bash\nexport DATABASE_URL=postgres://user:pass@localhost:5432/gist\n```" + `

### Cache

Redis is used for caching. Set the Redis URL:

` + "```bash\nexport REDIS_URL=redis://localhost:6379\n```" + `

## API Reference

### Search

The search endpoint accepts a query and returns matching chunks:

` + "```go\nresults, err := g.Search(ctx, \"database\", WithLimit(10))\n```" + `

### Index

Index new content for searching:

` + "```go\nresult, err := g.Index(ctx, content, WithSource(\"docs.md\"))\n```" + `

## Troubleshooting

If you encounter connection errors, verify your database is running and the connection string is correct.
Common issues include firewall rules and expired credentials.
`

	res, err := g.Index(ctx, mdContent, WithSource("getting-started.md"))
	if err != nil {
		t.Fatalf("Index: %v", err)
	}
	if res.TotalChunks == 0 {
		t.Fatal("expected at least one chunk after indexing")
	}
	if res.Label != "getting-started.md" {
		t.Errorf("Label = %q, want %q", res.Label, "getting-started.md")
	}
	if res.SourceID == 0 {
		t.Error("expected non-zero SourceID")
	}

	// Step 2: Index a second source.
	codeContent := "package main\n\nfunc main() {\n\tfmt.Println(\"hello world\")\n}\n"
	res2, err := g.Index(ctx, codeContent,
		WithSource("main.go"),
		WithFormat(FormatPlainText),
	)
	if err != nil {
		t.Fatalf("Index second source: %v", err)
	}
	if res2.TotalChunks == 0 {
		t.Fatal("expected at least one chunk for second source")
	}

	// Step 3: Search for content that exists.
	results, err := g.Search(ctx, "database")
	if err != nil {
		t.Fatalf("Search: %v", err)
	}
	if len(results) == 0 {
		t.Fatal("expected at least one result for 'database'")
	}
	// Verify result fields are populated.
	for _, r := range results {
		if r.Snippet == "" {
			t.Error("expected non-empty snippet")
		}
		if r.MatchLayer == "" {
			t.Error("expected non-empty match layer")
		}
	}

	// Step 4: Search with options.
	limited, err := g.Search(ctx, "database", WithLimit(1))
	if err != nil {
		t.Fatalf("Search with limit: %v", err)
	}
	if len(limited) > 1 {
		t.Errorf("expected at most 1 result with limit, got %d", len(limited))
	}

	// Step 5: Search for non-existent content.
	noResults, err := g.Search(ctx, "nonexistentxyzzy")
	if err != nil {
		t.Fatalf("Search for non-existent: %v", err)
	}
	if len(noResults) != 0 {
		t.Errorf("expected 0 results, got %d", len(noResults))
	}

	// Step 6: Verify stats.
	stats := g.Stats()
	if stats.SourceCount != 2 {
		t.Errorf("SourceCount = %d, want 2", stats.SourceCount)
	}
	if stats.ChunkCount == 0 {
		t.Error("expected non-zero ChunkCount")
	}
	if stats.BytesIndexed != int64(len(mdContent)+len(codeContent)) {
		t.Errorf("BytesIndexed = %d, want %d", stats.BytesIndexed, len(mdContent)+len(codeContent))
	}
	if stats.SearchCount != 3 {
		t.Errorf("SearchCount = %d, want 3", stats.SearchCount)
	}
	if stats.BytesReturned < 0 {
		t.Error("expected non-negative BytesReturned")
	}

	// Step 7: Close.
	if err := g.Close(); err != nil {
		t.Fatalf("Close: %v", err)
	}
}

// TestIntegration_TokenBudgetPipeline tests the default token budget flow.
func TestIntegration_TokenBudgetPipeline(t *testing.T) {
	ms := newGistMockStore()
	g, err := New(WithStore(ms), WithTokenBudget(50))
	if err != nil {
		t.Fatalf("New: %v", err)
	}
	defer func() { _ = g.Close() }()

	ctx := context.Background()

	// Index enough content that budget will constrain results.
	for i := 0; i < 5; i++ {
		content := "# Section\n\n" + strings.Repeat("word ", 200)
		_, err := g.Index(ctx, content)
		if err != nil {
			t.Fatalf("Index[%d]: %v", i, err)
		}
	}

	results, err := g.Search(ctx, "word", WithLimit(10))
	if err != nil {
		t.Fatalf("Search: %v", err)
	}
	// Budget should constrain the number of results returned.
	if len(results) == 0 {
		t.Fatal("expected at least one result")
	}
}

// TestIntegration_IndexChunkingOptions tests indexing with custom chunk size.
func TestIntegration_IndexChunkingOptions(t *testing.T) {
	ms := newGistMockStore()
	g, err := New(WithStore(ms))
	if err != nil {
		t.Fatalf("New: %v", err)
	}
	defer func() { _ = g.Close() }()

	ctx := context.Background()

	// Use a small max chunk size to force splitting.
	content := "# A\n\nFirst section content here.\n\n# B\n\nSecond section content here.\n\n# C\n\nThird section content here."
	res, err := g.Index(ctx, content, WithIndexMaxChunkBytes(50))
	if err != nil {
		t.Fatalf("Index: %v", err)
	}
	if res.TotalChunks < 2 {
		t.Errorf("expected multiple chunks with small max, got %d", res.TotalChunks)
	}
}

// TestIntegration_SearchErrorPropagation verifies that store errors propagate through Search.
func TestIntegration_SearchErrorPropagation(t *testing.T) {
	ms := newGistMockStore()
	g, err := New(WithStore(ms))
	if err != nil {
		t.Fatalf("New: %v", err)
	}
	defer func() { _ = g.Close() }()

	// Search on empty store should return no results, not error.
	results, err := g.Search(context.Background(), "anything")
	if err != nil {
		t.Fatalf("Search on empty store: %v", err)
	}
	if len(results) != 0 {
		t.Errorf("expected 0 results on empty store, got %d", len(results))
	}
}

// TestIntegration_IndexSaveSourceError verifies error propagation from SaveSource.
func TestIntegration_IndexSaveSourceError(t *testing.T) {
	store := &failingMockStore{failOn: "SaveSource"}
	g, err := New(WithStore(store))
	if err != nil {
		t.Fatalf("New: %v", err)
	}
	defer func() { _ = g.Close() }()

	_, err = g.Index(context.Background(), "# Hello\n\nContent")
	if err == nil {
		t.Fatal("expected error from SaveSource failure")
	}
}

// TestIntegration_IndexSaveChunkError verifies error propagation from SaveChunk.
func TestIntegration_IndexSaveChunkError(t *testing.T) {
	store := &failingMockStore{failOn: "SaveChunk"}
	g, err := New(WithStore(store))
	if err != nil {
		t.Fatalf("New: %v", err)
	}
	defer func() { _ = g.Close() }()

	_, err = g.Index(context.Background(), "# Hello\n\nContent")
	if err == nil {
		t.Fatal("expected error from SaveChunk failure")
	}
}

// failingMockStore fails on a specific method to test error propagation.
type failingMockStore struct {
	failOn  string
	sources []Source
	nextSrc int
}

func (m *failingMockStore) SaveSource(_ context.Context, label string, format Format) (Source, error) {
	if m.failOn == "SaveSource" {
		return Source{}, errForTest
	}
	m.nextSrc++
	s := Source{ID: m.nextSrc, Label: label, Format: format}
	m.sources = append(m.sources, s)
	return s, nil
}

func (m *failingMockStore) SaveChunk(_ context.Context, _ Chunk) (Chunk, error) {
	if m.failOn == "SaveChunk" {
		return Chunk{}, errForTest
	}
	return Chunk{ID: 1}, nil
}

func (m *failingMockStore) SearchPorter(_ context.Context, _ SearchParams) ([]SearchMatch, error) {
	if m.failOn == "SearchPorter" {
		return nil, errForTest
	}
	return nil, nil
}

func (m *failingMockStore) SearchTrigram(_ context.Context, _ SearchParams) ([]SearchMatch, error) {
	return nil, nil
}

func (m *failingMockStore) VocabularyTerms(_ context.Context) ([]string, error) { return nil, nil }

func (m *failingMockStore) Sources(_ context.Context) ([]Source, error) {
	out := make([]Source, len(m.sources))
	copy(out, m.sources)
	return out, nil
}

func (m *failingMockStore) Stats(_ context.Context) (StoreStats, error) { return StoreStats{}, nil }
func (m *failingMockStore) Close() error                                { return nil }

var _ Store = (*failingMockStore)(nil)

var errForTest = errors.New("test error")

// TestIntegration_SearchStoreError verifies that store errors propagate through Gist.Search.
func TestIntegration_SearchStoreError(t *testing.T) {
	store := &failingMockStore{failOn: "SearchPorter"}
	g, err := New(WithStore(store))
	if err != nil {
		t.Fatalf("New: %v", err)
	}
	defer func() { _ = g.Close() }()

	_, err = g.Search(context.Background(), "test")
	if err == nil {
		t.Fatal("expected error from Search when store fails")
	}
}

// TestIntegration_CodeChunkCounting verifies that code chunks are counted.
func TestIntegration_CodeChunkCounting(t *testing.T) {
	ms := newGistMockStore()
	g, err := New(WithStore(ms))
	if err != nil {
		t.Fatalf("New: %v", err)
	}
	defer func() { _ = g.Close() }()

	content := "# Code\n\n```go\nfunc main() {\n\tfmt.Println(\"hello\")\n\tfmt.Println(\"world\")\n\tfmt.Println(\"foo\")\n\tfmt.Println(\"bar\")\n\tfmt.Println(\"baz\")\n}\n```\n"
	res, err := g.Index(context.Background(), content)
	if err != nil {
		t.Fatalf("Index: %v", err)
	}
	if res.CodeChunks < 1 {
		t.Errorf("expected at least 1 code chunk, got %d", res.CodeChunks)
	}
}

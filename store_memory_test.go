package gist

import (
	"context"
	"sync"
	"testing"
)

func TestMemoryStore_SaveSource(t *testing.T) {
	tests := []struct {
		name   string
		label  string
		format Format
	}{
		{"markdown source", "readme.md", FormatMarkdown},
		{"json source", "config.json", FormatJSON},
		{"plaintext source", "notes.txt", FormatPlainText},
		{"yaml source", "deploy.yaml", FormatYAML},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			m := NewMemoryStore()
			defer m.Close()

			src, err := m.SaveSource(context.Background(), tt.label, tt.format)
			if err != nil {
				t.Fatalf("SaveSource() error = %v", err)
			}
			if src.ID == 0 {
				t.Error("SaveSource() returned ID 0")
			}
			if src.Label != tt.label {
				t.Errorf("Label = %q, want %q", src.Label, tt.label)
			}
			if src.Format != tt.format {
				t.Errorf("Format = %v, want %v", src.Format, tt.format)
			}
		})
	}
}

func TestMemoryStore_SaveSourceAutoIncrement(t *testing.T) {
	m := NewMemoryStore()
	defer m.Close()

	s1, _ := m.SaveSource(context.Background(), "a", FormatMarkdown)
	s2, _ := m.SaveSource(context.Background(), "b", FormatMarkdown)

	if s2.ID <= s1.ID {
		t.Errorf("IDs not auto-incremented: first=%d, second=%d", s1.ID, s2.ID)
	}
}

func TestMemoryStore_SaveChunk(t *testing.T) {
	m := NewMemoryStore()
	defer m.Close()
	ctx := context.Background()

	src, _ := m.SaveSource(ctx, "doc.md", FormatMarkdown)

	chunk := Chunk{
		SourceID:    src.ID,
		HeadingPath: "Intro",
		Content:     "hello world",
		ContentType: "prose",
		Format:      FormatMarkdown,
		ByteStart:   0,
		ByteEnd:     11,
	}

	saved, err := m.SaveChunk(ctx, chunk)
	if err != nil {
		t.Fatalf("SaveChunk() error = %v", err)
	}
	if saved.ID == 0 {
		t.Error("SaveChunk() returned ID 0")
	}
	if saved.Content != chunk.Content {
		t.Errorf("Content = %q, want %q", saved.Content, chunk.Content)
	}
}

func TestMemoryStore_SaveChunkUpdatesSource(t *testing.T) {
	m := NewMemoryStore()
	defer m.Close()
	ctx := context.Background()

	src, _ := m.SaveSource(ctx, "doc.md", FormatMarkdown)

	m.SaveChunk(ctx, Chunk{SourceID: src.ID, Content: "abc"})
	m.SaveChunk(ctx, Chunk{SourceID: src.ID, Content: "defgh"})

	sources, _ := m.Sources(ctx)
	if len(sources) != 1 {
		t.Fatalf("Sources count = %d, want 1", len(sources))
	}
	if sources[0].ChunkCount != 2 {
		t.Errorf("ChunkCount = %d, want 2", sources[0].ChunkCount)
	}
	wantBytes := int64(len("abc") + len("defgh"))
	if sources[0].BytesIndexed != wantBytes {
		t.Errorf("BytesIndexed = %d, want %d", sources[0].BytesIndexed, wantBytes)
	}
}

func TestMemoryStore_SearchPorter(t *testing.T) {
	tests := []struct {
		name       string
		chunks     []string
		query      string
		wantCount  int
		wantFirst  string
		wantLayer  string
	}{
		{
			name:      "single word match",
			chunks:    []string{"hello world", "goodbye moon"},
			query:     "hello",
			wantCount: 1,
			wantFirst: "hello world",
			wantLayer: "porter",
		},
		{
			name:      "multi word match scores higher",
			chunks:    []string{"the quick brown fox", "quick fox"},
			query:     "quick fox",
			wantCount: 2,
			wantFirst: "quick fox",
			wantLayer: "porter",
		},
		{
			name:      "case insensitive",
			chunks:    []string{"Hello World"},
			query:     "hello",
			wantCount: 1,
			wantFirst: "Hello World",
			wantLayer: "porter",
		},
		{
			name:      "no match",
			chunks:    []string{"hello world"},
			query:     "missing",
			wantCount: 0,
		},
		{
			name:      "empty query",
			chunks:    []string{"hello world"},
			query:     "",
			wantCount: 0,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			m := NewMemoryStore()
			defer m.Close()
			ctx := context.Background()

			src, _ := m.SaveSource(ctx, "test", FormatPlainText)
			for _, content := range tt.chunks {
				m.SaveChunk(ctx, Chunk{SourceID: src.ID, Content: content})
			}

			results, err := m.SearchPorter(ctx, SearchParams{Query: tt.query})
			if err != nil {
				t.Fatalf("SearchPorter() error = %v", err)
			}
			if len(results) != tt.wantCount {
				t.Fatalf("result count = %d, want %d", len(results), tt.wantCount)
			}
			if tt.wantCount > 0 {
				if results[0].Content != tt.wantFirst {
					t.Errorf("first result = %q, want %q", results[0].Content, tt.wantFirst)
				}
				if results[0].MatchLayer != tt.wantLayer {
					t.Errorf("MatchLayer = %q, want %q", results[0].MatchLayer, tt.wantLayer)
				}
				if results[0].Score <= 0 {
					t.Errorf("Score = %f, want > 0", results[0].Score)
				}
			}
		})
	}
}

func TestMemoryStore_SearchPorterWithSourceFilter(t *testing.T) {
	m := NewMemoryStore()
	defer m.Close()
	ctx := context.Background()

	src1, _ := m.SaveSource(ctx, "docs", FormatPlainText)
	src2, _ := m.SaveSource(ctx, "code", FormatPlainText)

	m.SaveChunk(ctx, Chunk{SourceID: src1.ID, Content: "database connection pool"})
	m.SaveChunk(ctx, Chunk{SourceID: src2.ID, Content: "database migration script"})

	results, _ := m.SearchPorter(ctx, SearchParams{Query: "database", SourceFilter: "docs"})
	if len(results) != 1 {
		t.Fatalf("result count = %d, want 1", len(results))
	}
	if results[0].SourceID != src1.ID {
		t.Errorf("SourceID = %d, want %d", results[0].SourceID, src1.ID)
	}
}

func TestMemoryStore_SearchPorterWithLimit(t *testing.T) {
	m := NewMemoryStore()
	defer m.Close()
	ctx := context.Background()

	src, _ := m.SaveSource(ctx, "test", FormatPlainText)
	for i := 0; i < 10; i++ {
		m.SaveChunk(ctx, Chunk{SourceID: src.ID, Content: "common word here"})
	}

	results, _ := m.SearchPorter(ctx, SearchParams{Query: "common", Limit: 3})
	if len(results) != 3 {
		t.Errorf("result count = %d, want 3", len(results))
	}
}

func TestMemoryStore_SearchTrigram(t *testing.T) {
	tests := []struct {
		name       string
		chunks     []string
		query      string
		wantCount  int
		wantFirst  string
		wantLayer  string
	}{
		{
			name:      "substring match",
			chunks:    []string{"configuration settings", "simple config"},
			query:     "config",
			wantCount: 2,
			wantFirst: "simple config",
			wantLayer: "trigram",
		},
		{
			name:      "case insensitive",
			chunks:    []string{"PostgreSQL Database"},
			query:     "postgresql",
			wantCount: 1,
			wantFirst: "PostgreSQL Database",
			wantLayer: "trigram",
		},
		{
			name:      "no match",
			chunks:    []string{"hello world"},
			query:     "xyz",
			wantCount: 0,
		},
		{
			name:      "empty query",
			chunks:    []string{"hello world"},
			query:     "",
			wantCount: 0,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			m := NewMemoryStore()
			defer m.Close()
			ctx := context.Background()

			src, _ := m.SaveSource(ctx, "test", FormatPlainText)
			for _, content := range tt.chunks {
				m.SaveChunk(ctx, Chunk{SourceID: src.ID, Content: content})
			}

			results, err := m.SearchTrigram(ctx, SearchParams{Query: tt.query})
			if err != nil {
				t.Fatalf("SearchTrigram() error = %v", err)
			}
			if len(results) != tt.wantCount {
				t.Fatalf("result count = %d, want %d", len(results), tt.wantCount)
			}
			if tt.wantCount > 0 {
				if results[0].Content != tt.wantFirst {
					t.Errorf("first result = %q, want %q", results[0].Content, tt.wantFirst)
				}
				if results[0].MatchLayer != tt.wantLayer {
					t.Errorf("MatchLayer = %q, want %q", results[0].MatchLayer, tt.wantLayer)
				}
				if results[0].Score <= 0 {
					t.Errorf("Score = %f, want > 0", results[0].Score)
				}
			}
		})
	}
}

func TestMemoryStore_SearchTrigramWithSourceFilter(t *testing.T) {
	m := NewMemoryStore()
	defer m.Close()
	ctx := context.Background()

	src1, _ := m.SaveSource(ctx, "readme", FormatPlainText)
	src2, _ := m.SaveSource(ctx, "changelog", FormatPlainText)

	m.SaveChunk(ctx, Chunk{SourceID: src1.ID, Content: "installation guide"})
	m.SaveChunk(ctx, Chunk{SourceID: src2.ID, Content: "installation fixes"})

	results, _ := m.SearchTrigram(ctx, SearchParams{Query: "install", SourceFilter: "changelog"})
	if len(results) != 1 {
		t.Fatalf("result count = %d, want 1", len(results))
	}
	if results[0].SourceID != src2.ID {
		t.Errorf("SourceID = %d, want %d", results[0].SourceID, src2.ID)
	}
}

func TestMemoryStore_SearchTrigramWithLimit(t *testing.T) {
	m := NewMemoryStore()
	defer m.Close()
	ctx := context.Background()

	src, _ := m.SaveSource(ctx, "test", FormatPlainText)
	for i := 0; i < 10; i++ {
		m.SaveChunk(ctx, Chunk{SourceID: src.ID, Content: "repeated content here"})
	}

	results, _ := m.SearchTrigram(ctx, SearchParams{Query: "content", Limit: 5})
	if len(results) != 5 {
		t.Errorf("result count = %d, want 5", len(results))
	}
}

func TestMemoryStore_SearchTrigramScoring(t *testing.T) {
	m := NewMemoryStore()
	defer m.Close()
	ctx := context.Background()

	src, _ := m.SaveSource(ctx, "test", FormatPlainText)
	m.SaveChunk(ctx, Chunk{SourceID: src.ID, Content: "config"})
	m.SaveChunk(ctx, Chunk{SourceID: src.ID, Content: "this is a long configuration document"})

	results, _ := m.SearchTrigram(ctx, SearchParams{Query: "config"})
	if len(results) != 2 {
		t.Fatalf("result count = %d, want 2", len(results))
	}
	// Shorter content with same match should score higher.
	if results[0].Content != "config" {
		t.Errorf("first result = %q, want %q (shorter content scores higher)", results[0].Content, "config")
	}
	if results[0].Score <= results[1].Score {
		t.Errorf("first score %f should be > second score %f", results[0].Score, results[1].Score)
	}
}

func TestMemoryStore_SearchPorterScoring(t *testing.T) {
	m := NewMemoryStore()
	defer m.Close()
	ctx := context.Background()

	src, _ := m.SaveSource(ctx, "test", FormatPlainText)
	m.SaveChunk(ctx, Chunk{SourceID: src.ID, Content: "go test"})
	m.SaveChunk(ctx, Chunk{SourceID: src.ID, Content: "go test is a useful tool for running tests in go projects"})

	results, _ := m.SearchPorter(ctx, SearchParams{Query: "go test"})
	if len(results) != 2 {
		t.Fatalf("result count = %d, want 2", len(results))
	}
	// "go test" (2 words, 2 matched) should score higher than the longer chunk.
	if results[0].Content != "go test" {
		t.Errorf("first result = %q, want %q", results[0].Content, "go test")
	}
}

func TestMemoryStore_SearchPorterHeadingPath(t *testing.T) {
	m := NewMemoryStore()
	defer m.Close()
	ctx := context.Background()

	src, _ := m.SaveSource(ctx, "test", FormatMarkdown)
	m.SaveChunk(ctx, Chunk{
		SourceID:    src.ID,
		HeadingPath: "Config > Database",
		Content:     "pool size setting",
		ContentType: "prose",
	})

	results, _ := m.SearchPorter(ctx, SearchParams{Query: "pool"})
	if len(results) != 1 {
		t.Fatalf("result count = %d, want 1", len(results))
	}
	if results[0].HeadingPath != "Config > Database" {
		t.Errorf("HeadingPath = %q, want %q", results[0].HeadingPath, "Config > Database")
	}
	if results[0].ContentType != "prose" {
		t.Errorf("ContentType = %q, want %q", results[0].ContentType, "prose")
	}
}

func TestMemoryStore_VocabularyTerms(t *testing.T) {
	tests := []struct {
		name      string
		chunks    []string
		wantTerms map[string]bool
	}{
		{
			name:      "extracts unique lowercase words",
			chunks:    []string{"Hello World", "hello again"},
			wantTerms: map[string]bool{"hello": true, "world": true, "again": true},
		},
		{
			name:      "empty store",
			chunks:    nil,
			wantTerms: map[string]bool{},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			m := NewMemoryStore()
			defer m.Close()
			ctx := context.Background()

			if len(tt.chunks) > 0 {
				src, _ := m.SaveSource(ctx, "test", FormatPlainText)
				for _, content := range tt.chunks {
					m.SaveChunk(ctx, Chunk{SourceID: src.ID, Content: content})
				}
			}

			terms, err := m.VocabularyTerms(ctx)
			if err != nil {
				t.Fatalf("VocabularyTerms() error = %v", err)
			}

			termSet := make(map[string]bool, len(terms))
			for _, term := range terms {
				termSet[term] = true
			}

			if len(termSet) != len(tt.wantTerms) {
				t.Errorf("term count = %d, want %d", len(termSet), len(tt.wantTerms))
			}
			for want := range tt.wantTerms {
				if !termSet[want] {
					t.Errorf("missing term %q", want)
				}
			}
		})
	}
}

func TestMemoryStore_Sources(t *testing.T) {
	m := NewMemoryStore()
	defer m.Close()
	ctx := context.Background()

	// Empty store.
	sources, err := m.Sources(ctx)
	if err != nil {
		t.Fatalf("Sources() error = %v", err)
	}
	if len(sources) != 0 {
		t.Errorf("empty store: source count = %d, want 0", len(sources))
	}

	// After adding sources.
	m.SaveSource(ctx, "a.md", FormatMarkdown)
	m.SaveSource(ctx, "b.json", FormatJSON)

	sources, err = m.Sources(ctx)
	if err != nil {
		t.Fatalf("Sources() error = %v", err)
	}
	if len(sources) != 2 {
		t.Fatalf("source count = %d, want 2", len(sources))
	}

	// Verify it returns a copy (mutating the returned slice shouldn't affect the store).
	sources[0].Label = "mutated"
	original, _ := m.Sources(ctx)
	if original[0].Label == "mutated" {
		t.Error("Sources() did not return a copy")
	}
}

func TestMemoryStore_Stats(t *testing.T) {
	tests := []struct {
		name       string
		setup      func(m *MemoryStore)
		wantChunks int
		wantSrcs   int
		wantBytes  int64
	}{
		{
			name:       "empty store",
			setup:      func(m *MemoryStore) {},
			wantChunks: 0,
			wantSrcs:   0,
			wantBytes:  0,
		},
		{
			name: "with data",
			setup: func(m *MemoryStore) {
				ctx := context.Background()
				s1, _ := m.SaveSource(ctx, "a", FormatPlainText)
				s2, _ := m.SaveSource(ctx, "b", FormatPlainText)
				m.SaveChunk(ctx, Chunk{SourceID: s1.ID, Content: "hello"})      // 5 bytes
				m.SaveChunk(ctx, Chunk{SourceID: s1.ID, Content: "world"})      // 5 bytes
				m.SaveChunk(ctx, Chunk{SourceID: s2.ID, Content: "go testing"}) // 10 bytes
			},
			wantChunks: 3,
			wantSrcs:   2,
			wantBytes:  20,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			m := NewMemoryStore()
			defer m.Close()
			tt.setup(m)

			stats, err := m.Stats(context.Background())
			if err != nil {
				t.Fatalf("Stats() error = %v", err)
			}
			if stats.ChunkCount != tt.wantChunks {
				t.Errorf("ChunkCount = %d, want %d", stats.ChunkCount, tt.wantChunks)
			}
			if stats.SourceCount != tt.wantSrcs {
				t.Errorf("SourceCount = %d, want %d", stats.SourceCount, tt.wantSrcs)
			}
			if stats.BytesIndexed != tt.wantBytes {
				t.Errorf("BytesIndexed = %d, want %d", stats.BytesIndexed, tt.wantBytes)
			}
		})
	}
}

func TestMemoryStore_Close(t *testing.T) {
	m := NewMemoryStore()
	ctx := context.Background()

	if err := m.Close(); err != nil {
		t.Fatalf("Close() error = %v", err)
	}

	// All operations should return errStoreClosed after Close.
	if _, err := m.SaveSource(ctx, "x", FormatPlainText); err != errStoreClosed {
		t.Errorf("SaveSource after close: got %v, want errStoreClosed", err)
	}
	if _, err := m.SaveChunk(ctx, Chunk{}); err != errStoreClosed {
		t.Errorf("SaveChunk after close: got %v, want errStoreClosed", err)
	}
	if _, err := m.SearchPorter(ctx, SearchParams{Query: "x"}); err != errStoreClosed {
		t.Errorf("SearchPorter after close: got %v, want errStoreClosed", err)
	}
	if _, err := m.SearchTrigram(ctx, SearchParams{Query: "x"}); err != errStoreClosed {
		t.Errorf("SearchTrigram after close: got %v, want errStoreClosed", err)
	}
	if _, err := m.VocabularyTerms(ctx); err != errStoreClosed {
		t.Errorf("VocabularyTerms after close: got %v, want errStoreClosed", err)
	}
	if _, err := m.Sources(ctx); err != errStoreClosed {
		t.Errorf("Sources after close: got %v, want errStoreClosed", err)
	}
	if _, err := m.Stats(ctx); err != errStoreClosed {
		t.Errorf("Stats after close: got %v, want errStoreClosed", err)
	}
}

func TestMemoryStore_EmptyStoreBehavior(t *testing.T) {
	m := NewMemoryStore()
	defer m.Close()
	ctx := context.Background()

	// Search on empty store returns no results and no error.
	results, err := m.SearchPorter(ctx, SearchParams{Query: "anything"})
	if err != nil {
		t.Fatalf("SearchPorter on empty store: error = %v", err)
	}
	if len(results) != 0 {
		t.Errorf("SearchPorter on empty store: got %d results, want 0", len(results))
	}

	results, err = m.SearchTrigram(ctx, SearchParams{Query: "anything"})
	if err != nil {
		t.Fatalf("SearchTrigram on empty store: error = %v", err)
	}
	if len(results) != 0 {
		t.Errorf("SearchTrigram on empty store: got %d results, want 0", len(results))
	}

	terms, err := m.VocabularyTerms(ctx)
	if err != nil {
		t.Fatalf("VocabularyTerms on empty store: error = %v", err)
	}
	if len(terms) != 0 {
		t.Errorf("VocabularyTerms on empty store: got %d terms, want 0", len(terms))
	}

	stats, err := m.Stats(ctx)
	if err != nil {
		t.Fatalf("Stats on empty store: error = %v", err)
	}
	if stats.ChunkCount != 0 || stats.SourceCount != 0 || stats.BytesIndexed != 0 {
		t.Errorf("Stats on empty store: got %+v, want all zeros", stats)
	}
}

func TestMemoryStore_ConcurrentReadWrite(t *testing.T) {
	m := NewMemoryStore()
	defer m.Close()
	ctx := context.Background()

	src, _ := m.SaveSource(ctx, "concurrent", FormatPlainText)

	var wg sync.WaitGroup
	const goroutines = 20

	// Writers: save chunks concurrently.
	for i := 0; i < goroutines; i++ {
		wg.Add(1)
		go func(n int) {
			defer wg.Done()
			m.SaveChunk(ctx, Chunk{SourceID: src.ID, Content: "concurrent write"})
		}(i)
	}

	// Readers: search, stats, sources, vocabulary concurrently.
	for i := 0; i < goroutines; i++ {
		wg.Add(1)
		go func() {
			defer wg.Done()
			m.SearchPorter(ctx, SearchParams{Query: "concurrent"})
			m.SearchTrigram(ctx, SearchParams{Query: "concurrent"})
			m.Sources(ctx)
			m.Stats(ctx)
			m.VocabularyTerms(ctx)
		}()
	}

	wg.Wait()

	// Verify all writes completed.
	stats, err := m.Stats(ctx)
	if err != nil {
		t.Fatalf("Stats() error = %v", err)
	}
	if stats.ChunkCount != goroutines {
		t.Errorf("ChunkCount = %d, want %d", stats.ChunkCount, goroutines)
	}
}

func TestMemoryStore_MatchesSourceFilter(t *testing.T) {
	tests := []struct {
		name    string
		filter  string
		wantAll bool
	}{
		{"empty filter matches all", "", true},
		{"matching filter", "docs", false},
		{"non-matching filter", "nonexistent", false},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			m := NewMemoryStore()
			defer m.Close()
			ctx := context.Background()

			src, _ := m.SaveSource(ctx, "docs", FormatPlainText)
			m.SaveSource(ctx, "code", FormatPlainText)
			m.SaveChunk(ctx, Chunk{SourceID: src.ID, Content: "searchable text"})

			results, _ := m.SearchPorter(ctx, SearchParams{Query: "searchable", SourceFilter: tt.filter})
			if tt.wantAll || tt.filter == "docs" {
				if len(results) != 1 {
					t.Errorf("result count = %d, want 1", len(results))
				}
			}
			if tt.filter == "nonexistent" {
				if len(results) != 0 {
					t.Errorf("result count = %d, want 0 for non-matching filter", len(results))
				}
			}
		})
	}
}

func TestMemoryStore_ChunkIDAutoIncrement(t *testing.T) {
	m := NewMemoryStore()
	defer m.Close()
	ctx := context.Background()

	src, _ := m.SaveSource(ctx, "test", FormatPlainText)
	c1, _ := m.SaveChunk(ctx, Chunk{SourceID: src.ID, Content: "first"})
	c2, _ := m.SaveChunk(ctx, Chunk{SourceID: src.ID, Content: "second"})

	if c2.ID <= c1.ID {
		t.Errorf("chunk IDs not auto-incremented: first=%d, second=%d", c1.ID, c2.ID)
	}
}

func TestMemoryStore_SearchResultFields(t *testing.T) {
	m := NewMemoryStore()
	defer m.Close()
	ctx := context.Background()

	src, _ := m.SaveSource(ctx, "test", FormatPlainText)
	chunk, _ := m.SaveChunk(ctx, Chunk{
		SourceID:    src.ID,
		HeadingPath: "Section > Sub",
		Content:     "find me here",
		ContentType: "code",
	})

	// Verify porter result fields.
	results, _ := m.SearchPorter(ctx, SearchParams{Query: "find"})
	if len(results) != 1 {
		t.Fatalf("result count = %d, want 1", len(results))
	}
	r := results[0]
	if r.ChunkID != chunk.ID {
		t.Errorf("ChunkID = %d, want %d", r.ChunkID, chunk.ID)
	}
	if r.SourceID != src.ID {
		t.Errorf("SourceID = %d, want %d", r.SourceID, src.ID)
	}
	if r.HeadingPath != "Section > Sub" {
		t.Errorf("HeadingPath = %q, want %q", r.HeadingPath, "Section > Sub")
	}
	if r.ContentType != "code" {
		t.Errorf("ContentType = %q, want %q", r.ContentType, "code")
	}

	// Verify trigram result fields.
	results, _ = m.SearchTrigram(ctx, SearchParams{Query: "find"})
	if len(results) != 1 {
		t.Fatalf("trigram result count = %d, want 1", len(results))
	}
	r = results[0]
	if r.ChunkID != chunk.ID {
		t.Errorf("trigram ChunkID = %d, want %d", r.ChunkID, chunk.ID)
	}
	if r.SourceID != src.ID {
		t.Errorf("trigram SourceID = %d, want %d", r.SourceID, src.ID)
	}
}

func TestMemoryStore_SearchPorterLimitZero(t *testing.T) {
	m := NewMemoryStore()
	defer m.Close()
	ctx := context.Background()

	src, _ := m.SaveSource(ctx, "test", FormatPlainText)
	m.SaveChunk(ctx, Chunk{SourceID: src.ID, Content: "word one"})
	m.SaveChunk(ctx, Chunk{SourceID: src.ID, Content: "word two"})

	// Limit 0 should return all results.
	results, _ := m.SearchPorter(ctx, SearchParams{Query: "word", Limit: 0})
	if len(results) != 2 {
		t.Errorf("result count = %d, want 2 (limit 0 means no limit)", len(results))
	}
}

func TestMemoryStore_SearchTrigramLimitZero(t *testing.T) {
	m := NewMemoryStore()
	defer m.Close()
	ctx := context.Background()

	src, _ := m.SaveSource(ctx, "test", FormatPlainText)
	m.SaveChunk(ctx, Chunk{SourceID: src.ID, Content: "word one"})
	m.SaveChunk(ctx, Chunk{SourceID: src.ID, Content: "word two"})

	// Limit 0 should return all results.
	results, _ := m.SearchTrigram(ctx, SearchParams{Query: "word", Limit: 0})
	if len(results) != 2 {
		t.Errorf("result count = %d, want 2 (limit 0 means no limit)", len(results))
	}
}

func TestMemoryStore_SaveChunkToNonexistentSource(t *testing.T) {
	m := NewMemoryStore()
	defer m.Close()
	ctx := context.Background()

	// Saving a chunk with a non-existent source ID should still work
	// (it just won't update any source's stats).
	chunk, err := m.SaveChunk(ctx, Chunk{SourceID: 999, Content: "orphan"})
	if err != nil {
		t.Fatalf("SaveChunk() error = %v", err)
	}
	if chunk.ID == 0 {
		t.Error("SaveChunk() returned ID 0")
	}

	// The chunk should still be searchable.
	results, _ := m.SearchPorter(ctx, SearchParams{Query: "orphan"})
	if len(results) != 1 {
		t.Errorf("orphan chunk not found in search, got %d results", len(results))
	}
}

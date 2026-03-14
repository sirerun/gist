//go:build integration

package gist

import (
	"context"
	"os"
	"testing"
	"time"

	"github.com/testcontainers/testcontainers-go"
	"github.com/testcontainers/testcontainers-go/wait"
)

func newTestPostgresStore(t *testing.T) *PostgresStore {
	t.Helper()
	ctx := context.Background()

	dsn := os.Getenv("GIST_TEST_DSN")
	if dsn == "" {
		req := testcontainers.ContainerRequest{
			Image:        "postgres:16-alpine",
			ExposedPorts: []string{"5432/tcp"},
			Env: map[string]string{
				"POSTGRES_USER":     "gist",
				"POSTGRES_PASSWORD": "gist",
				"POSTGRES_DB":       "gist",
			},
			WaitingFor: wait.ForListeningPort("5432/tcp").WithStartupTimeout(60 * time.Second),
		}
		container, err := testcontainers.GenericContainer(ctx, testcontainers.GenericContainerRequest{
			ContainerRequest: req,
			Started:          true,
		})
		if err != nil {
			t.Fatalf("start postgres container: %v", err)
		}
		t.Cleanup(func() { container.Terminate(ctx) })

		host, err := container.Host(ctx)
		if err != nil {
			t.Fatalf("container host: %v", err)
		}
		port, err := container.MappedPort(ctx, "5432")
		if err != nil {
			t.Fatalf("container port: %v", err)
		}
		dsn = "postgres://gist:gist@" + host + ":" + port.Port() + "/gist?sslmode=disable"
	}

	store, err := NewPostgresStore(ctx, dsn)
	if err != nil {
		t.Fatalf("NewPostgresStore: %v", err)
	}
	t.Cleanup(func() { store.Close() })

	// Clean tables for test isolation.
	store.pool.Exec(ctx, "DELETE FROM chunks")
	store.pool.Exec(ctx, "DELETE FROM sources")

	return store
}

func TestPostgresStore_SaveSource(t *testing.T) {
	s := newTestPostgresStore(t)
	ctx := context.Background()

	tests := []struct {
		name   string
		label  string
		format Format
	}{
		{name: "markdown source", label: "readme.md", format: FormatMarkdown},
		{name: "json source", label: "config.json", format: FormatJSON},
		{name: "plaintext source", label: "notes.txt", format: FormatPlainText},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			src, err := s.SaveSource(ctx, tt.label, tt.format)
			if err != nil {
				t.Fatalf("SaveSource() error = %v", err)
			}
			if src.ID == 0 {
				t.Error("SaveSource() returned zero ID")
			}
			if src.Label != tt.label {
				t.Errorf("SaveSource() Label = %q, want %q", src.Label, tt.label)
			}
			if src.Format != tt.format {
				t.Errorf("SaveSource() Format = %v, want %v", src.Format, tt.format)
			}
		})
	}
}

func TestPostgresStore_SaveChunk(t *testing.T) {
	s := newTestPostgresStore(t)
	ctx := context.Background()

	src, err := s.SaveSource(ctx, "test.md", FormatMarkdown)
	if err != nil {
		t.Fatalf("SaveSource: %v", err)
	}

	tests := []struct {
		name    string
		chunk   Chunk
	}{
		{
			name: "prose chunk",
			chunk: Chunk{
				SourceID:    src.ID,
				HeadingPath: "Introduction",
				Content:     "This is a test document about databases.",
				ContentType: "prose",
				ByteStart:   0,
				ByteEnd:     40,
			},
		},
		{
			name: "code chunk",
			chunk: Chunk{
				SourceID:    src.ID,
				HeadingPath: "Examples > Code",
				Content:     "func main() { fmt.Println(\"hello\") }",
				ContentType: "code",
				ByteStart:   40,
				ByteEnd:     77,
			},
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			saved, err := s.SaveChunk(ctx, tt.chunk)
			if err != nil {
				t.Fatalf("SaveChunk() error = %v", err)
			}
			if saved.ID == 0 {
				t.Error("SaveChunk() returned zero ID")
			}
			if saved.SourceID != tt.chunk.SourceID {
				t.Errorf("SaveChunk() SourceID = %d, want %d", saved.SourceID, tt.chunk.SourceID)
			}
			if saved.Content != tt.chunk.Content {
				t.Errorf("SaveChunk() Content mismatch")
			}
		})
	}
}

func TestPostgresStore_SearchPorter(t *testing.T) {
	s := newTestPostgresStore(t)
	ctx := context.Background()

	src, _ := s.SaveSource(ctx, "docs.md", FormatMarkdown)
	s.SaveChunk(ctx, Chunk{SourceID: src.ID, Content: "database connection pooling strategies", ContentType: "prose", HeadingPath: "DB"})
	s.SaveChunk(ctx, Chunk{SourceID: src.ID, Content: "authentication and authorization flows", ContentType: "prose", HeadingPath: "Auth"})

	tests := []struct {
		name      string
		params    SearchParams
		wantMin   int
		wantMax   int
		wantLayer string
	}{
		{
			name:      "match database",
			params:    SearchParams{Query: "database", Limit: 10},
			wantMin:   1,
			wantMax:   1,
			wantLayer: "porter",
		},
		{
			name:      "stemmed match: connecting matches connection",
			params:    SearchParams{Query: "connecting", Limit: 10},
			wantMin:   1,
			wantMax:   1,
			wantLayer: "porter",
		},
		{
			name:    "no match",
			params:  SearchParams{Query: "kubernetes", Limit: 10},
			wantMin: 0,
			wantMax: 0,
		},
		{
			name:    "limit results",
			params:  SearchParams{Query: "and", Limit: 1},
			wantMin: 0,
			wantMax: 1,
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			matches, err := s.SearchPorter(ctx, tt.params)
			if err != nil {
				t.Fatalf("SearchPorter() error = %v", err)
			}
			if len(matches) < tt.wantMin || len(matches) > tt.wantMax {
				t.Errorf("SearchPorter() returned %d matches, want [%d, %d]", len(matches), tt.wantMin, tt.wantMax)
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

func TestPostgresStore_SearchTrigram(t *testing.T) {
	s := newTestPostgresStore(t)
	ctx := context.Background()

	src, _ := s.SaveSource(ctx, "code.go", FormatPlainText)
	s.SaveChunk(ctx, Chunk{SourceID: src.ID, Content: "func handleRequest(w http.ResponseWriter, r *http.Request)", ContentType: "code"})
	s.SaveChunk(ctx, Chunk{SourceID: src.ID, Content: "func processData(input []byte) error", ContentType: "code"})

	tests := []struct {
		name      string
		params    SearchParams
		wantMin   int
		wantLayer string
	}{
		{
			name:      "substring match",
			params:    SearchParams{Query: "handleRequest", Limit: 5},
			wantMin:   1,
			wantLayer: "trigram",
		},
		{
			name:    "no match",
			params:  SearchParams{Query: "zzzNonExistent", Limit: 5},
			wantMin: 0,
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			matches, err := s.SearchTrigram(ctx, tt.params)
			if err != nil {
				t.Fatalf("SearchTrigram() error = %v", err)
			}
			if len(matches) < tt.wantMin {
				t.Errorf("SearchTrigram() returned %d matches, want >= %d", len(matches), tt.wantMin)
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

func TestPostgresStore_SearchPorterWithSourceFilter(t *testing.T) {
	s := newTestPostgresStore(t)
	ctx := context.Background()

	src1, _ := s.SaveSource(ctx, "file-a.md", FormatMarkdown)
	src2, _ := s.SaveSource(ctx, "file-b.md", FormatMarkdown)
	s.SaveChunk(ctx, Chunk{SourceID: src1.ID, Content: "shared database term", ContentType: "prose"})
	s.SaveChunk(ctx, Chunk{SourceID: src2.ID, Content: "shared database term", ContentType: "prose"})

	matches, err := s.SearchPorter(ctx, SearchParams{
		Query:        "database",
		Limit:        10,
		SourceFilter: "file-a.md",
	})
	if err != nil {
		t.Fatalf("SearchPorter() error = %v", err)
	}
	if len(matches) != 1 {
		t.Fatalf("expected 1 match with source filter, got %d", len(matches))
	}
	if matches[0].SourceID != src1.ID {
		t.Errorf("match source ID = %d, want %d", matches[0].SourceID, src1.ID)
	}
}

func TestPostgresStore_VocabularyTerms(t *testing.T) {
	s := newTestPostgresStore(t)
	ctx := context.Background()

	src, _ := s.SaveSource(ctx, "vocab.md", FormatMarkdown)
	s.SaveChunk(ctx, Chunk{SourceID: src.ID, Content: "hello world programming", ContentType: "prose"})
	s.SaveChunk(ctx, Chunk{SourceID: src.ID, Content: "hello gist search", ContentType: "prose"})

	terms, err := s.VocabularyTerms(ctx)
	if err != nil {
		t.Fatalf("VocabularyTerms() error = %v", err)
	}
	if len(terms) == 0 {
		t.Fatal("VocabularyTerms() returned no terms")
	}

	termSet := make(map[string]bool)
	for _, term := range terms {
		termSet[term] = true
	}
	// ts_stat returns stemmed terms, so "hello" becomes "hello", "world" stays "world", etc.
	// Check that at least some expected terms exist.
	found := false
	for _, want := range []string{"hello", "world", "program", "gist", "search"} {
		if termSet[want] {
			found = true
		}
	}
	if !found {
		t.Errorf("VocabularyTerms() returned %v, expected at least one known term", terms)
	}
}

func TestPostgresStore_Sources(t *testing.T) {
	s := newTestPostgresStore(t)
	ctx := context.Background()

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

func TestPostgresStore_Stats(t *testing.T) {
	s := newTestPostgresStore(t)
	ctx := context.Background()

	src, _ := s.SaveSource(ctx, "stats.md", FormatMarkdown)
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
	wantBytes := int64(len("twelve chars") + len("more content here"))
	if stats.BytesIndexed != wantBytes {
		t.Errorf("Stats().BytesIndexed = %d, want %d", stats.BytesIndexed, wantBytes)
	}
}

func TestPostgresStore_FullFlow(t *testing.T) {
	s := newTestPostgresStore(t)
	ctx := context.Background()

	// Step 1: Create sources
	src1, err := s.SaveSource(ctx, "architecture.md", FormatMarkdown)
	if err != nil {
		t.Fatalf("SaveSource: %v", err)
	}
	src2, err := s.SaveSource(ctx, "api.go", FormatPlainText)
	if err != nil {
		t.Fatalf("SaveSource: %v", err)
	}

	// Step 2: Save chunks
	chunks := []Chunk{
		{SourceID: src1.ID, HeadingPath: "Architecture > Database", Content: "PostgreSQL is used for persistent storage with full-text search capabilities.", ContentType: "prose", ByteStart: 0, ByteEnd: 76},
		{SourceID: src1.ID, HeadingPath: "Architecture > Caching", Content: "Redis provides in-memory caching for frequently accessed data.", ContentType: "prose", ByteStart: 76, ByteEnd: 138},
		{SourceID: src2.ID, HeadingPath: "", Content: "func CreateUser(ctx context.Context, name string) (*User, error)", ContentType: "code", ByteStart: 0, ByteEnd: 65},
	}
	for i, c := range chunks {
		saved, err := s.SaveChunk(ctx, c)
		if err != nil {
			t.Fatalf("SaveChunk[%d]: %v", i, err)
		}
		if saved.ID == 0 {
			t.Errorf("SaveChunk[%d] returned zero ID", i)
		}
	}

	// Step 3: Porter search
	porterResults, err := s.SearchPorter(ctx, SearchParams{Query: "storage", Limit: 5})
	if err != nil {
		t.Fatalf("SearchPorter: %v", err)
	}
	if len(porterResults) == 0 {
		t.Fatal("SearchPorter returned no results for 'storage'")
	}
	if porterResults[0].MatchLayer != "porter" {
		t.Errorf("expected porter layer, got %q", porterResults[0].MatchLayer)
	}

	// Step 4: Trigram search
	trigramResults, err := s.SearchTrigram(ctx, SearchParams{Query: "CreateUser", Limit: 5})
	if err != nil {
		t.Fatalf("SearchTrigram: %v", err)
	}
	if len(trigramResults) == 0 {
		t.Fatal("SearchTrigram returned no results for 'CreateUser'")
	}
	if trigramResults[0].MatchLayer != "trigram" {
		t.Errorf("expected trigram layer, got %q", trigramResults[0].MatchLayer)
	}

	// Step 5: Vocabulary
	terms, err := s.VocabularyTerms(ctx)
	if err != nil {
		t.Fatalf("VocabularyTerms: %v", err)
	}
	if len(terms) == 0 {
		t.Fatal("VocabularyTerms returned no terms")
	}

	// Step 6: Sources
	sources, err := s.Sources(ctx)
	if err != nil {
		t.Fatalf("Sources: %v", err)
	}
	if len(sources) != 2 {
		t.Errorf("Sources count = %d, want 2", len(sources))
	}

	// Step 7: Stats
	stats, err := s.Stats(ctx)
	if err != nil {
		t.Fatalf("Stats: %v", err)
	}
	if stats.SourceCount != 2 {
		t.Errorf("Stats.SourceCount = %d, want 2", stats.SourceCount)
	}
	if stats.ChunkCount != 3 {
		t.Errorf("Stats.ChunkCount = %d, want 3", stats.ChunkCount)
	}
	if stats.BytesIndexed <= 0 {
		t.Errorf("Stats.BytesIndexed = %d, want > 0", stats.BytesIndexed)
	}
}

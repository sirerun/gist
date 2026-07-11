package gist

import (
	"context"
	"reflect"
	"testing"
)

func TestExpansionCamelCase(t *testing.T) {
	tests := []struct {
		name  string
		query string
		want  []string
	}{
		{"simple", "getUserId", []string{"getuserid", "get", "user", "id"}},
		{"pascal", "RetryUpload", []string{"retryupload", "retry", "upload"}},
		{"acronym", "parseHTTPRequest", []string{"parsehttprequest", "parse", "http", "request"}},
		{"no boundary", "database", nil},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := ExpandQuery(tt.query)
			want := append([]string{}, tt.want...)
			if tt.want == nil {
				want = []string{tt.query}
			}
			if !reflect.DeepEqual(got, want) {
				t.Errorf("ExpandQuery(%q) = %v, want %v", tt.query, got, want)
			}
		})
	}
}

func TestExpansionSnakeCase(t *testing.T) {
	tests := []struct {
		name  string
		query string
		want  []string
	}{
		{"snake", "get_user_id", []string{"get_user_id", "get", "user", "id"}},
		{"kebab", "retry-upload-backoff", []string{"retry-upload-backoff", "retry", "upload", "backoff"}},
		{"mixed", "resubmission_backoffPolicy", []string{"resubmission_backoffpolicy", "resubmission", "backoff", "policy"}},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := ExpandQuery(tt.query)
			if !reflect.DeepEqual(got, tt.want) {
				t.Errorf("ExpandQuery(%q) = %v, want %v", tt.query, got, tt.want)
			}
		})
	}
}

// expansionMockStore returns results only for a specific expanded query,
// letting tests assert that expansion is tried in the right order relative
// to porter, trigram, and fuzzy.
type expansionMockStore struct {
	sources       []Source
	expandedQuery string
	expandedHits  []SearchMatch
}

func (m *expansionMockStore) SaveSource(_ context.Context, label string, format Format) (Source, error) {
	return Source{}, nil
}
func (m *expansionMockStore) SaveChunk(_ context.Context, chunk Chunk) (Chunk, error) {
	return Chunk{}, nil
}
func (m *expansionMockStore) SearchPorter(_ context.Context, params SearchParams) ([]SearchMatch, error) {
	if params.Query == m.expandedQuery {
		return m.expandedHits, nil
	}
	return nil, nil
}
func (m *expansionMockStore) SearchTrigram(_ context.Context, params SearchParams) ([]SearchMatch, error) {
	return nil, nil
}
func (m *expansionMockStore) VocabularyTerms(_ context.Context) ([]string, error) {
	return nil, nil
}
func (m *expansionMockStore) Sources(_ context.Context) ([]Source, error) {
	result := make([]Source, len(m.sources))
	copy(result, m.sources)
	return result, nil
}
func (m *expansionMockStore) Stats(_ context.Context) (StoreStats, error) {
	return StoreStats{}, nil
}
func (m *expansionMockStore) Close() error { return nil }

var _ Store = (*expansionMockStore)(nil)

func TestExpansionTierOrder(t *testing.T) {
	store := &expansionMockStore{
		sources:       []Source{{ID: 1, Label: "code.go"}},
		expandedQuery: "retryupload retry upload",
		expandedHits: []SearchMatch{
			{ChunkID: 1, SourceID: 1, HeadingPath: "Uploads", Content: "retry upload backoff policy", ContentType: "code", Score: 0.9, MatchLayer: "porter"},
		},
	}
	vocab := NewVocabulary() // Empty vocab: if fuzzy were reached, it would find nothing.
	searcher := NewSearcher(store, vocab)

	results, err := searcher.Search(context.Background(), "retryUpload")
	if err != nil {
		t.Fatalf("Search() error = %v", err)
	}
	if len(results) != 1 {
		t.Fatalf("Search() returned %d results, want 1", len(results))
	}
	if results[0].MatchLayer != "expansion" {
		t.Errorf("MatchLayer = %q, want %q", results[0].MatchLayer, "expansion")
	}
	if results[0].Title != "Uploads" {
		t.Errorf("Title = %q, want %q", results[0].Title, "Uploads")
	}
}

func TestExpansionMissIntegration(t *testing.T) {
	g, err := New()
	if err != nil {
		t.Fatalf("New() error = %v", err)
	}
	defer g.Close()

	ctx := context.Background()
	if _, err := g.Index(ctx, "retry upload backoff policy for failed uploads", WithSource("uploads.md")); err != nil {
		t.Fatalf("Index() error = %v", err)
	}

	// "retryUpload" as one camelCase word would not match the indexed prose
	// via porter alone; expansion should split it and find the content
	// before this is ever logged as a miss.
	results, err := g.Search(ctx, "retryUpload")
	if err != nil {
		t.Fatalf("Search() error = %v", err)
	}
	if len(results) == 0 {
		t.Fatal("Search() returned no results, expected expansion to bridge the identifier split")
	}
	if results[0].MatchLayer != "expansion" {
		t.Errorf("MatchLayer = %q, want %q", results[0].MatchLayer, "expansion")
	}

	misses := g.Misses(0)
	for _, miss := range misses {
		if miss.Query == "retryUpload" {
			t.Errorf("query %q was recorded as a miss, want expansion to have resolved it: %+v", miss.Query, miss)
		}
	}
}

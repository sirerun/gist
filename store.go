package gist

import "context"

// Format describes the content format of indexed material.
type Format int

const (
	// FormatMarkdown indicates Markdown content with heading-aware chunking.
	FormatMarkdown Format = iota
	// FormatJSON indicates structured JSON content.
	FormatJSON
	// FormatPlainText indicates unstructured plain text.
	FormatPlainText
)

// String returns the human-readable name of the format.
func (f Format) String() string {
	switch f {
	case FormatMarkdown:
		return "markdown"
	case FormatJSON:
		return "json"
	case FormatPlainText:
		return "plaintext"
	default:
		return "unknown"
	}
}

// Source represents an indexed content source (e.g., a file, tool output, or URL).
type Source struct {
	// ID is the unique identifier for the source.
	ID int
	// Label is a human-readable label such as a file path or tool name.
	Label string
	// Format is the content format of the source.
	Format Format
	// BytesIndexed is the total number of bytes indexed from this source.
	BytesIndexed int64
	// ChunkCount is the number of chunks produced from this source.
	ChunkCount int
}

// Chunk represents a single indexed piece of content derived from a source.
type Chunk struct {
	// ID is the unique identifier for the chunk.
	ID int
	// SourceID links the chunk to its parent source.
	SourceID int
	// HeadingPath is the hierarchical heading context (e.g., "Config > Database > Pool").
	HeadingPath string
	// Content is the raw text content of the chunk.
	Content string
	// ContentType classifies the chunk as "code" or "prose".
	ContentType string
	// Format is the content format of the chunk.
	Format Format
	// ByteStart is the starting byte offset within the original source.
	ByteStart int
	// ByteEnd is the ending byte offset within the original source.
	ByteEnd int
}

// SearchParams controls how a search query is executed against the store.
type SearchParams struct {
	// Query is the search string.
	Query string
	// Limit is the maximum number of results to return.
	Limit int
	// SourceFilter restricts results to a specific source label.
	SourceFilter string
}

// SearchMatch represents a single search result from the store.
type SearchMatch struct {
	// ChunkID is the ID of the matched chunk.
	ChunkID int
	// SourceID is the ID of the source containing the matched chunk.
	SourceID int
	// HeadingPath is the heading context of the matched chunk.
	HeadingPath string
	// Content is the full text of the matched chunk.
	Content string
	// ContentType is "code" or "prose".
	ContentType string
	// Score is the relevance score (e.g., BM25 rank).
	Score float64
	// MatchLayer indicates which search tier produced the match ("porter", "trigram", or "fuzzy").
	MatchLayer string
}

// StoreStats contains aggregate statistics about the indexed content.
type StoreStats struct {
	// ChunkCount is the total number of indexed chunks.
	ChunkCount int
	// SourceCount is the total number of indexed sources.
	SourceCount int
	// BytesIndexed is the total bytes of content indexed.
	BytesIndexed int64
}

// Store is the storage abstraction for Gist. Implementations persist indexed
// content and provide search capabilities using full-text and trigram indexes.
// The primary implementation uses PostgreSQL with tsvector for porter stemming
// and pg_trgm for trigram search.
type Store interface {
	// SaveSource creates a new source record with the given label and format.
	// It returns the created source with its assigned ID.
	SaveSource(ctx context.Context, label string, format Format) (Source, error)

	// SaveChunk persists a chunk associated with a source. The chunk must
	// reference a valid source ID. Implementations should update the
	// full-text and trigram indexes as part of this operation.
	SaveChunk(ctx context.Context, chunk Chunk) (Chunk, error)

	// SearchPorter performs full-text search using PostgreSQL tsvector with
	// porter stemming. Results are ranked by BM25 relevance.
	SearchPorter(ctx context.Context, params SearchParams) ([]SearchMatch, error)

	// SearchTrigram performs substring search using PostgreSQL pg_trgm
	// trigram indexes. This is the fallback when porter stemming yields
	// no results.
	SearchTrigram(ctx context.Context, params SearchParams) ([]SearchMatch, error)

	// VocabularyTerms returns distinct indexed terms for fuzzy matching.
	// The caller uses these to find the closest term via Levenshtein
	// distance when both porter and trigram searches return no results.
	VocabularyTerms(ctx context.Context) ([]string, error)

	// Sources returns all indexed sources.
	Sources(ctx context.Context) ([]Source, error)

	// Stats returns aggregate statistics about the store contents.
	Stats(ctx context.Context) (StoreStats, error)

	// Close releases any resources held by the store (e.g., database connections).
	Close() error
}

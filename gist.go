// Package gist provides context intelligence for LLM applications.
// It indexes content, searches with three-tier fallback (porter stemming,
// trigram, fuzzy correction), and returns budget-aware snippets.
package gist

import (
	"context"
	"errors"
	"runtime"
	"strings"
	"sync"
	"sync/atomic"

	"golang.org/x/sync/errgroup"
)

// Stats holds aggregate statistics about indexing and search activity.
type Stats struct {
	// BytesIndexed is the total bytes of content that have been indexed.
	BytesIndexed int64 `json:"bytes_indexed"`
	// BytesReturned is the total bytes of snippets returned by search operations.
	BytesReturned int64 `json:"bytes_returned"`
	// BytesSaved is the difference between indexed and returned bytes,
	// representing the context reduction achieved.
	BytesSaved int64 `json:"bytes_saved"`
	// SavedPercent is the percentage of indexed bytes saved by smart extraction.
	SavedPercent float64 `json:"saved_percent"`
	// SourceCount is the number of indexed sources.
	SourceCount int `json:"source_count"`
	// ChunkCount is the number of indexed chunks across all sources.
	ChunkCount int `json:"chunk_count"`
	// SearchCount is the total number of search operations performed.
	SearchCount int `json:"search_count"`
}

// IndexResult holds the outcome of an Index operation.
type IndexResult struct {
	// SourceID is the ID assigned to the newly created source.
	SourceID int
	// Label is the source label used for this index operation.
	Label string
	// TotalChunks is the number of chunks produced from the content.
	TotalChunks int
	// CodeChunks is the number of chunks classified as code.
	CodeChunks int
}

// IndexOption configures the behavior of an Index call.
type IndexOption func(*indexConfig)

type indexConfig struct {
	source        string
	format        Format
	maxChunkBytes int
}

func defaultIndexConfig() indexConfig {
	return indexConfig{
		source:        "unnamed",
		format:        FormatMarkdown,
		maxChunkBytes: defaultMaxChunkBytes,
	}
}

// WithSource sets the source label for indexed content.
func WithSource(label string) IndexOption {
	return func(c *indexConfig) {
		if label != "" {
			c.source = label
		}
	}
}

// WithFormat sets the content format for chunking during indexing.
func WithFormat(f Format) IndexOption {
	return func(c *indexConfig) {
		c.format = f
	}
}

// WithIndexMaxChunkBytes sets the maximum chunk size in bytes for indexing.
func WithIndexMaxChunkBytes(n int) IndexOption {
	return func(c *indexConfig) {
		if n > 0 {
			c.maxChunkBytes = n
		}
	}
}

// Option configures a Gist instance.
type Option func(*config)

type config struct {
	store       Store
	postgresDSN string
	tokenBudget int
	projectRoot string
}

// WithPostgres configures Gist to use a PostgreSQL store with the given DSN.
func WithPostgres(dsn string) Option {
	return func(c *config) {
		c.postgresDSN = dsn
	}
}

// WithStore configures Gist to use a custom Store implementation.
func WithStore(s Store) Option {
	return func(c *config) {
		c.store = s
	}
}

// WithTokenBudget sets the default token budget for search operations.
func WithTokenBudget(max int) Option {
	return func(c *config) {
		if max > 0 {
			c.tokenBudget = max
		}
	}
}

// WithProjectRoot sets the working directory for the executor.
func WithProjectRoot(dir string) Option {
	return func(c *config) {
		c.projectRoot = dir
	}
}

// Gist is the top-level API for the context intelligence library.
// It ties together content indexing, three-tier search, vocabulary
// management, and statistics tracking.
type Gist struct {
	store    Store
	searcher *Searcher
	vocab    *Vocabulary
	cfg      config

	mu          sync.Mutex
	sourceCount int
	chunkCount  int
	searchCount int64
	bytesIdx    int64
	bytesRet    int64
}

// New creates a new Gist instance with the given options. At least one of
// WithStore or WithPostgres must be provided; otherwise New returns an error.
func New(opts ...Option) (*Gist, error) {
	var cfg config
	for _, opt := range opts {
		opt(&cfg)
	}

	if cfg.store == nil && cfg.postgresDSN == "" {
		return nil, errors.New("gist: a store is required; use WithStore or WithPostgres")
	}

	if cfg.store == nil {
		s, err := NewPostgresStore(context.Background(), cfg.postgresDSN)
		if err != nil {
			return nil, err
		}
		cfg.store = s
	}

	vocab := NewVocabulary()
	searcher := NewSearcher(cfg.store, vocab)

	return &Gist{
		store:    cfg.store,
		searcher: searcher,
		vocab:    vocab,
		cfg:      cfg,
	}, nil
}

// Index chunks content and persists it to the store. It updates the
// vocabulary and internal statistics. The returned IndexResult summarises
// the operation.
func (g *Gist) Index(ctx context.Context, content string, opts ...IndexOption) (*IndexResult, error) {
	ic := defaultIndexConfig()
	for _, opt := range opts {
		opt(&ic)
	}

	chunks, err := ChunkContent(content,
		WithMaxChunkBytes(ic.maxChunkBytes),
		WithChunkFormat(ic.format),
	)
	if err != nil {
		return nil, err
	}

	src, err := g.store.SaveSource(ctx, ic.source, ic.format)
	if err != nil {
		return nil, err
	}

	codeChunks := 0
	for _, cc := range chunks {
		chunk := Chunk{
			SourceID:    src.ID,
			HeadingPath: cc.HeadingPath,
			Content:     cc.Content,
			ContentType: cc.ContentType,
			Format:      ic.format,
			ByteStart:   cc.StartByte,
			ByteEnd:     cc.EndByte,
		}
		if _, err := g.store.SaveChunk(ctx, chunk); err != nil {
			return nil, err
		}
		if cc.ContentType == "code" {
			codeChunks++
		}

		// Add words to vocabulary.
		words := strings.Fields(cc.Content)
		g.vocab.Add(words...)
	}

	// Update stats atomically / under lock.
	g.mu.Lock()
	g.sourceCount++
	g.chunkCount += len(chunks)
	g.bytesIdx += int64(len(content))
	g.mu.Unlock()

	return &IndexResult{
		SourceID:    src.ID,
		Label:       ic.source,
		TotalChunks: len(chunks),
		CodeChunks:  codeChunks,
	}, nil
}

// BatchItem represents a single item to index in a batch operation.
type BatchItem struct {
	Content string
	Source  string
	Format  Format
}

// BatchOption configures batch indexing behavior.
type BatchOption func(*batchConfig)

type batchConfig struct {
	concurrency int
}

func defaultBatchConfig() batchConfig {
	return batchConfig{
		concurrency: runtime.NumCPU(),
	}
}

// WithConcurrency sets the number of concurrent indexing goroutines.
func WithConcurrency(n int) BatchOption {
	return func(c *batchConfig) {
		if n > 0 {
			c.concurrency = n
		}
	}
}

// BatchIndex indexes multiple items concurrently using a goroutine pool.
// If any item fails, partial results are returned along with the error.
// Context cancellation stops queuing new items but lets in-flight items finish.
func (g *Gist) BatchIndex(ctx context.Context, items []BatchItem, opts ...BatchOption) ([]*IndexResult, error) {
	if len(items) == 0 {
		return nil, nil
	}

	bc := defaultBatchConfig()
	for _, opt := range opts {
		opt(&bc)
	}

	results := make([]*IndexResult, len(items))
	errs := make([]error, len(items))

	eg, egCtx := errgroup.WithContext(ctx)
	eg.SetLimit(bc.concurrency)

	for i, item := range items {
		i, item := i, item
		eg.Go(func() error {
			var indexOpts []IndexOption
			if item.Source != "" {
				indexOpts = append(indexOpts, WithSource(item.Source))
			}
			indexOpts = append(indexOpts, WithFormat(item.Format))

			res, err := g.Index(egCtx, item.Content, indexOpts...)
			if err != nil {
				errs[i] = err
				return nil // don't cancel other items
			}
			results[i] = res
			return nil
		})
	}

	_ = eg.Wait()

	var combined error
	for _, err := range errs {
		if err != nil {
			combined = errors.Join(combined, err)
		}
	}

	return results, combined
}

// Search delegates to the three-tier Searcher. It applies the configured
// default token budget if no explicit budget is set via options.
func (g *Gist) Search(ctx context.Context, query string, opts ...SearchOption) ([]SearchResult, error) {
	if g.cfg.tokenBudget > 0 {
		// Prepend default budget; explicit WithBudget in opts will override.
		opts = append([]SearchOption{WithBudget(g.cfg.tokenBudget)}, opts...)
	}

	results, err := g.searcher.Search(ctx, query, opts...)
	if err != nil {
		return nil, err
	}

	var retBytes int64
	for _, r := range results {
		retBytes += int64(len(r.Snippet))
	}

	atomic.AddInt64(&g.searchCount, 1)
	atomic.AddInt64(&g.bytesRet, retBytes)

	return results, nil
}

// Stats returns a snapshot of indexing and search statistics.
func (g *Gist) Stats() *Stats {
	g.mu.Lock()
	sc := g.sourceCount
	cc := g.chunkCount
	bi := g.bytesIdx
	g.mu.Unlock()

	br := atomic.LoadInt64(&g.bytesRet)
	searches := atomic.LoadInt64(&g.searchCount)

	saved := bi - br
	if saved < 0 {
		saved = 0
	}
	var pct float64
	if bi > 0 {
		pct = float64(saved) / float64(bi) * 100
	}

	return &Stats{
		BytesIndexed:  bi,
		BytesReturned: br,
		BytesSaved:    saved,
		SavedPercent:  pct,
		SourceCount:   sc,
		ChunkCount:    cc,
		SearchCount:   int(searches),
	}
}

// Close releases resources held by the underlying Store.
func (g *Gist) Close() error {
	return g.store.Close()
}

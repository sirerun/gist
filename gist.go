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
	"time"

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
	// MissCount is the number of searches recorded as retrieval misses.
	MissCount int `json:"miss_count"`
	// MissRate is the fraction of searches that were recorded as misses.
	MissRate float64 `json:"miss_rate"`
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
	supersedes    string
	keepPrevious  bool
	ttl           time.Duration
	halfLife      time.Duration
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

// WithSupersedes marks all prior sources with the given label as superseded
// by this newly indexed source. Superseded sources are excluded from search.
// Requires a store implementing SupersedingStore; ignored otherwise.
func WithSupersedes(label string) IndexOption {
	return func(c *indexConfig) {
		c.supersedes = label
	}
}

// WithKeepPrevious opts out of the default self-supersession behavior that
// applies when indexing reuses a label that already exists.
func WithKeepPrevious() IndexOption {
	return func(c *indexConfig) {
		c.keepPrevious = true
	}
}

// WithTTL sets a time-to-live for the indexed source. Once the TTL elapses
// from indexing time, the source's chunks are excluded from search results.
// Requires a store implementing SupersedingStore; ignored otherwise.
func WithTTL(d time.Duration) IndexOption {
	return func(c *indexConfig) {
		c.ttl = d
	}
}

// WithHalfLife declares the indexed source's volatility for opt-in recency
// decay scoring: a chunk's relevance score is multiplied by
// 0.5^(age/halfLife) at search time. Requires a store implementing
// SupersedingStore; ignored otherwise.
func WithHalfLife(d time.Duration) IndexOption {
	return func(c *indexConfig) {
		c.halfLife = d
	}
}

// Option configures a Gist instance.
type Option func(*config)

type config struct {
	store          Store
	postgresDSN    string
	tokenBudget    int
	projectRoot    string
	missScoreFloor float64
	missLogSize    int
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

// WithMemory configures Gist to use an in-memory Store. This is useful for
// testing, prototyping, and small workloads that do not require PostgreSQL.
// Data is ephemeral and does not persist across restarts.
func WithMemory() Option {
	return func(c *config) {
		c.store = NewMemoryStore()
	}
}

// WithProjectRoot sets the working directory for the executor.
func WithProjectRoot(dir string) Option {
	return func(c *config) {
		c.projectRoot = dir
	}
}

// WithMissScoreFloor sets the relevance score below which a search's top
// result is recorded as a retrieval miss. Defaults to defaultMissScoreFloor.
func WithMissScoreFloor(floor float64) Option {
	return func(c *config) {
		c.missScoreFloor = floor
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
	missLog  *MissLog

	mu          sync.Mutex
	sourceCount int
	chunkCount  int
	searchCount int64
	bytesIdx    int64
	bytesRet    int64
}

// New creates a new Gist instance with the given options. If no store is
// configured via WithStore, WithMemory, or WithPostgres, an in-memory store
// is used by default.
func New(opts ...Option) (*Gist, error) {
	var cfg config
	for _, opt := range opts {
		opt(&cfg)
	}

	if cfg.store == nil && cfg.postgresDSN != "" {
		s, err := NewPostgresStore(context.Background(), cfg.postgresDSN)
		if err != nil {
			return nil, err
		}
		cfg.store = s
	}

	if cfg.store == nil {
		cfg.store = NewMemoryStore()
	}

	if cfg.missScoreFloor <= 0 {
		cfg.missScoreFloor = defaultMissScoreFloor
	}

	vocab := NewVocabulary()
	searcher := NewSearcher(cfg.store, vocab)

	return &Gist{
		store:    cfg.store,
		searcher: searcher,
		vocab:    vocab,
		cfg:      cfg,
		missLog:  NewMissLog(cfg.missLogSize),
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

	var src Source
	if ss, ok := g.store.(SupersedingStore); ok {
		src, err = ss.SaveSourceWithOptions(ctx, ic.source, ic.format, SourceOptions{
			Supersedes:   ic.supersedes,
			KeepPrevious: ic.keepPrevious,
			TTL:          ic.ttl,
			HalfLife:     ic.halfLife,
		})
	} else {
		src, err = g.store.SaveSource(ctx, ic.source, ic.format)
	}
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

	g.recordMissIfNeeded(query, results)

	return results, nil
}

// recordMissIfNeeded logs a MissRecord when a search returned no results or
// its top result's score fell below the configured floor.
func (g *Gist) recordMissIfNeeded(query string, results []SearchResult) {
	isMiss := len(results) == 0 || results[0].Score < g.cfg.missScoreFloor
	if !isMiss {
		return
	}

	matchLayer := ""
	if len(results) > 0 {
		matchLayer = results[0].MatchLayer
	}

	g.missLog.Record(MissRecord{
		Time:       time.Now(),
		Query:      query,
		Class:      classifyMiss(results),
		NearMisses: nearMisses(results),
		ScoreFloor: g.cfg.missScoreFloor,
		MatchLayer: matchLayer,
	})
}

// Misses returns up to n of the most recent retrieval miss records. If
// n <= 0, all retained records are returned.
func (g *Gist) Misses(n int) []MissRecord {
	return g.missLog.Recent(n)
}

// MissClassBreakdown returns the count of retained miss records by class.
func (g *Gist) MissClassBreakdown() map[MissClass]int {
	return g.missLog.ClassBreakdown()
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

	missCount := g.missLog.Count()
	var missRate float64
	if searches > 0 {
		missRate = float64(missCount) / float64(searches)
	}

	return &Stats{
		BytesIndexed:  bi,
		BytesReturned: br,
		BytesSaved:    saved,
		SavedPercent:  pct,
		SourceCount:   sc,
		ChunkCount:    cc,
		SearchCount:   int(searches),
		MissCount:     missCount,
		MissRate:      missRate,
	}
}

// Vacuum deletes superseded and expired sources and chunks from the
// underlying store on demand. It is a no-op if the store does not implement
// Vacuumer.
func (g *Gist) Vacuum(ctx context.Context) error {
	v, ok := g.store.(Vacuumer)
	if !ok {
		return nil
	}
	return v.Vacuum(ctx)
}

// Close releases resources held by the underlying Store.
func (g *Gist) Close() error {
	return g.store.Close()
}

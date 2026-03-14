# Plan: Gist — Go Context Intelligence Library

**Date:** 2026-03-13
**Source:** `docs/proposal.md`
**Change:** PostgreSQL is the sole storage backend for all environments (dev, test, staging, production).

## Architecture

### Design Principles

1. **Library-first**: Import `gist` as a Go package. No daemon, no sidecar.
2. **Zero-copy indexing**: Content is chunked and indexed in-place.
3. **Three-tier search**: Porter stemming, trigram substring, fuzzy correction.
4. **Budget-aware**: Callers set token budgets. Gist returns results that fit, ranked by relevance.
5. **PostgreSQL everywhere**: Single storage backend — PostgreSQL with `tsvector`/`pg_trgm` for full-text and trigram search. No SQLite. Simplifies testing, eliminates backend drift, and matches production from day one.

### Package Layout

```
github.com/sirerun/gist/
├── gist.go              # Top-level API: New(), Index(), Search(), Stats()
├── store.go             # ContentStore interface
├── store_postgres.go    # PostgreSQL FTS implementation (tsvector + pg_trgm)
├── chunk.go             # Markdown, JSON, and plain-text chunking
├── search.go            # Three-tier search: porter → trigram → fuzzy
├── fuzzy.go             # Levenshtein distance + vocabulary correction
├── snippet.go           # Smart snippet extraction around match positions
├── session.go           # Session event tracking + snapshot building
├── snapshot.go          # Priority-tiered resume snapshot renderer
├── executor.go          # Polyglot subprocess executor (optional)
├── security.go          # Deny/allow policy enforcement (optional)
├── mcp/                 # MCP server adapter (optional)
│   ├── server.go        # Expose Gist as MCP tools over stdio
│   └── tools.go         # Tool definitions: gist_execute, gist_search, etc.
├── cmd/
│   └── gist/            # CLI binary
│       └── main.go      # gist index, gist search, gist serve, gist doctor
└── internal/
    ├── fts/             # PostgreSQL tsvector and pg_trgm helpers
    ├── bm25/            # BM25 scoring
    └── runtime/         # Runtime detection for executor
```

### Core API

```go
package gist

func New(opts ...Option) (*Gist, error)

type Option func(*config)

func WithPostgres(dsn string) Option       // Required — no default in-memory fallback
func WithStore(s Store) Option             // Custom backend
func WithTokenBudget(max int) Option       // Cap search results by token estimate
func WithProjectRoot(dir string) Option    // For executor working directory
```

### Index Options

```go
type IndexOption func(*indexConfig)

func WithSource(label string) IndexOption   // Custom source label
func WithFormat(f Format) IndexOption       // Markdown (default), JSON, PlainText
func WithMaxChunkBytes(n int) IndexOption   // Override chunk size (default 4096)
```

### Search Options

```go
type SearchOption func(*searchConfig)

func WithLimit(n int) SearchOption           // Max results (default 5)
func WithSourceFilter(s string) SearchOption // Only search specific source
func WithSnippetLen(n int) SearchOption      // Snippet bytes (default 1500)
func WithBudget(tokens int) SearchOption     // Fit results within token budget
```

### Result Types

```go
type IndexResult struct {
    SourceID    int
    Label       string
    TotalChunks int
    CodeChunks  int
}

type SearchResult struct {
    Title       string  // Heading path: "Config > Database > Connection Pool"
    Snippet     string  // Smart-extracted context around matches
    Source      string  // Source label
    Score       float64 // BM25 relevance score
    ContentType string  // "code" or "prose"
    MatchLayer  string  // "porter", "trigram", or "fuzzy"
}

type Stats struct {
    BytesIndexed   int64
    BytesReturned  int64
    BytesSaved     int64
    SavedPercent   float64
    SourceCount    int
    ChunkCount     int
    SearchCount    int
}
```

## Implementation Plan

### Phase 1: Core Library (2 weeks)

- [ ] `gist.go` — `New()`, `Index()`, `Search()`, `Close()`
- [ ] `store.go` — `ContentStore` interface
- [ ] `store_postgres.go` — PostgreSQL backend using `tsvector` for porter stemming and `pg_trgm` for trigram search
- [ ] `chunk.go` — Markdown chunking (heading-aware, code-block preserving)
- [ ] `search.go` — Three-tier fallback (porter → trigram → fuzzy)
- [ ] `fuzzy.go` — Levenshtein distance + vocabulary correction
- [ ] `snippet.go` — Smart snippet extraction
- [ ] Full test suite with table-driven tests against PostgreSQL (use `testcontainers-go` or local instance)
- [ ] `go doc` documentation on all exported types

### Phase 2: CLI + MCP (1 week)

- [ ] `cmd/gist/main.go` — `gist index <file>`, `gist search <query>`, `gist stats`
- [ ] `gist serve` — MCP server over stdio (tools: `gist_index`, `gist_search`, `gist_execute`)
- [ ] `gist doctor` — runtime and dependency diagnostics (includes PostgreSQL connectivity check)

### Phase 3: Sire Integration (1 week)

- [ ] Implement `core.StepIndexer` using Gist
- [ ] Implement `core.BeforeStepHook` for context injection
- [ ] Register `sire:local/gist.search` MCP tool
- [ ] Wire into `cmd/sire/serve.go` behind feature flag `"gist"`
- [ ] Integration tests with existing engine test harness

### Phase 4: Advanced Features (2 weeks)

- [ ] `session.go` + `snapshot.go` — Session event tracking and resume snapshots
- [ ] `executor.go` — Polyglot subprocess executor (shell, Python, Go, JS)
- [ ] JSON and YAML structured chunking
- [ ] Batch indexing with goroutine pool
- [ ] `gist bench` — performance benchmarking command

### Phase 5: Open-Source Launch (1 week)

- [ ] README with examples and benchmarks
- [ ] Apache 2.0 LICENSE
- [ ] GitHub Actions CI (lint, test, release — CI runs against a PostgreSQL service container)
- [ ] `go install github.com/sirerun/gist/cmd/gist@latest`
- [ ] Announcement post positioning Gist alongside Mint

## Dependencies

| Dependency | Purpose | CGO |
|------------|---------|-----|
| `jackc/pgx/v5` | PostgreSQL driver | No |
| `github.com/yuin/goldmark` | Markdown parsing for structured chunking | No |
| `github.com/agnivade/levenshtein` | Fuzzy matching | No |
| `github.com/spf13/cobra` | CLI (consistent with Sire API) | No |

Zero CGO. Single `go build` produces a static binary for any platform. PostgreSQL is the only external runtime dependency.

## Success Metrics

| Metric | Target |
|--------|--------|
| Context reduction | >95% |
| Search latency (p99) | <5ms for 10K chunks |
| Index throughput | >50 MB/s |
| Binary size | <15 MB |
| Test coverage | >85% |
| GitHub stars (6 months) | 1,000+ |
| Sire `sire dev` integration | Ships in Phase 3 |

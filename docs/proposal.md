# Proposal: Gist — Go Context Intelligence Library

**Date:** 2026-03-13
**Status:** Draft
**Author:** Engineering
**Repository:** `github.com/sirerun/gist`
**License:** Apache 2.0

## One-Liner

Gist is a Go library and CLI that keeps raw data out of LLM context windows — index everything, retrieve only what matters.

## Problem

Every tool call in an AI agent pipeline dumps raw output into the context window. A Playwright snapshot is 56 KB. Twenty GitHub issues are 59 KB. A log file is 200 KB. After 30 minutes of real work, 40%+ of the context window is consumed by stale tool output the LLM will never reference again.

This is not a cosmetic problem. It is the single largest bottleneck to running AI agents at scale:

- **Cost**: Wasted input tokens are wasted dollars. A tenant with 30 MCP tools sends ~15K tokens of schemas alone per LLM call (ADR-033).
- **Quality**: Relevant context gets pushed out by irrelevant output, degrading reasoning.
- **Duration**: Sessions that should last hours die in minutes.
- **Collaboration**: Multi-agent workflows cannot share context without exhausting budgets.

Context-mode (Node.js/TypeScript) proved the concept — 315 KB to 5.4 KB (98% reduction). Gist ports this to Go, fixes the design limitations, and makes it a reusable library that powers Sire's backend and stands alone as open-source infrastructure.

## Why "Gist"

- 4 letters. Easy to type, say, and remember.
- Means "the essence of something" — exactly what the library does (extract the gist, discard the noise).
- Not taken in the Go ecosystem as a library name.
- Natural verb: "gist this output", "search the gist".
- Follows Sire's naming convention: short, lowercase, memorable (mint, sire, web, api).

## Strategic Fit

### Follows the Mint Pattern

Sire's open-source strategy (ADR-045) is proven: build open tooling that works with any orchestrator, capture developers, create natural upgrade paths to the managed platform.

| Asset | Open-Source Value | Sire Platform Value |
|-------|------------------|-------------------|
| **Mint** | Generate MCP servers from any OpenAPI spec | Orchestrate those servers with durability + guardrails |
| **Gist** | Manage LLM context in any Go application | Power Sire's token budget manager + OKG + multi-agent context sharing |

### Enables Three Roadmap Priorities

1. **`sire dev` local mode** (ADR-046): Gist is the context layer that makes local workflows fast without cloud dependencies. Ships as part of the single Go binary.
2. **Proactive Intelligence**: Pattern detection across accumulated context requires efficient storage and retrieval — Gist provides it.
3. **Cross-tenant OKG**: Gist's context encoding enables differential privacy and anonymized knowledge sharing.

### Powers the Core Vision

From `docs/core.md`: "Efficient context sharing between siloed agents is the missing capability that transforms Sire from a workflow engine into an organizational nervous system."

Gist is the technical mechanism. Every agent indexes its outputs into Gist. Other agents search it. The organizational knowledge graph grows. Each new agent makes all existing agents more effective — driving NRR above 150%.

## Architecture

### Design Principles

1. **Library-first**: Import `gist` as a Go package. No daemon, no sidecar, no network dependency.
2. **Zero-copy indexing**: Content is chunked and indexed in-place. Raw data never enters the caller's memory beyond the initial read.
3. **Three-tier search**: Porter stemming, trigram substring, fuzzy correction. Typos and partial terms just work.
4. **Budget-aware**: Callers set token budgets. Gist returns results that fit, ranked by relevance.
5. **Pluggable storage**: SQLite (default, embedded), PostgreSQL (production), or bring-your-own via interface.

### Package Layout

```
github.com/sirerun/gist/
├── gist.go              # Top-level API: New(), Index(), Search(), Stats()
├── store.go             # ContentStore interface + SQLite implementation
├── store_postgres.go    # PostgreSQL FTS implementation
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
    ├── fts/             # FTS5 and trigram tokenizer helpers
    ├── bm25/            # BM25 scoring (when not using SQLite builtin)
    └── runtime/         # Runtime detection for executor
```

### Core API

```go
package gist

// New creates a Gist instance with the given storage backend.
func New(opts ...Option) (*Gist, error)

// Option configures a Gist instance.
type Option func(*config)

func WithSQLite(path string) Option        // Default: in-memory
func WithPostgres(dsn string) Option       // Production
func WithStore(s Store) Option             // Custom backend
func WithTokenBudget(max int) Option       // Cap search results by token estimate
func WithProjectRoot(dir string) Option    // For executor working directory

// Gist is the main entry point.
type Gist struct { ... }

// Index chunks and indexes content into the store.
// Returns metadata about what was indexed, never the raw content.
func (g *Gist) Index(ctx context.Context, content string, opts ...IndexOption) (*IndexResult, error)

// IndexFile reads a file and indexes it.
func (g *Gist) IndexFile(ctx context.Context, path string, opts ...IndexOption) (*IndexResult, error)

// Search queries indexed content with three-tier fallback.
func (g *Gist) Search(ctx context.Context, query string, opts ...SearchOption) ([]SearchResult, error)

// BatchSearch runs multiple queries in one call, deduplicating results.
func (g *Gist) BatchSearch(ctx context.Context, queries []string, opts ...SearchOption) ([]SearchResult, error)

// Stats returns context savings and usage metrics.
func (g *Gist) Stats() *Stats

// Close releases resources.
func (g *Gist) Close() error
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

### Sire API Integration

Gist plugs into the existing API via established interfaces:

```go
// 1. StepIndexer — index step outputs after execution
type gistIndexer struct {
    g *gist.Gist
}

func (gi *gistIndexer) Index(ctx context.Context, exec *core.Execution, stepID string, state *core.StepState) error {
    _, err := gi.g.Index(ctx, state.Output,
        gist.WithSource(fmt.Sprintf("exec:%s/step:%s", exec.ID, stepID)),
    )
    return err
}

// Wire it up:
engine.SetIndexer(&gistIndexer{g: gistInstance})

// 2. BeforeStepHook — inject relevant context before each step
type gistContextHook struct {
    g      *gist.Gist
    budget *budget.Manager
}

func (h *gistContextHook) BeforeStep(ctx context.Context, exec *core.Execution, step *core.Step) error {
    tokens := h.budget.MaxContextItems(exec.ID) * 50
    results, _ := h.g.Search(ctx, step.Input["prompt"].(string),
        gist.WithBudget(tokens),
        gist.WithLimit(5),
    )
    if len(results) > 0 {
        step.Input["_gist_context"] = results
    }
    return nil
}

// 3. MCP Tool — expose as sire:local/gist.search
dispatcher.Register("gist.search", func(ctx context.Context, input map[string]any) (string, error) {
    results, err := gistInstance.Search(ctx, input["query"].(string))
    // marshal and return
})
```

## Improvements Over context-mode

| Area | context-mode (TypeScript) | Gist (Go) |
|------|--------------------------|-----------|
| **Distribution** | npm package, requires Node.js | Single static binary, zero runtime deps |
| **Library use** | MCP server only, can't import | `import "github.com/sirerun/gist"` |
| **Storage** | SQLite only (better-sqlite3 native addon) | SQLite default + PostgreSQL + pluggable interface |
| **Security** | Command injection in rustc/taskkill paths | Array-form `exec.Command()`, no shell interpolation |
| **Path safety** | No boundary checks on executeFile | `filepath.Rel()` enforced against project root |
| **Token budgets** | No budget awareness | First-class `WithBudget(tokens)` option |
| **Concurrency** | Single-threaded (Node.js) | Goroutine-safe, parallel indexing and search |
| **Chunking** | Markdown only (JSON/plaintext basic) | Structured chunking for Markdown, JSON, YAML, Go AST |
| **Testing** | Vitest | Go stdlib + table-driven, no external test framework |
| **Process cleanup** | String interpolation for PID kill | `cmd.Process.Kill()` + `syscall.Kill(-pid, SIGKILL)` |
| **MCP server** | Primary mode | Optional adapter — library works without MCP |

## Security Fixes

These context-mode vulnerabilities are eliminated by design in Gist:

1. **Command injection** (`executor.ts:175` rustc path interpolation) — Go uses `exec.Command("rustc", srcPath, "-o", binPath)`. No shell involved.
2. **Path traversal** (`executor.ts:114` no boundary check) — `filepath.Rel(projectRoot, absPath)` returns error if path escapes root.
3. **Cache cleanup traversal** (`cli.ts:453` regex + rmSync) — `filepath.EvalSymlinks()` + `strings.HasPrefix()` before any deletion.
4. **Upgrade MITM** (`cli.ts:417` git clone without verification) — Go binary distribution via `go install`, checksummed by Go module proxy.
5. **Process PID injection** (`executor.ts:21` string interpolation in taskkill) — `cmd.Process.Kill()` API, no shell command.

## Implementation Plan

### Phase 1: Core Library (2 weeks)

- [ ] `gist.go` — `New()`, `Index()`, `Search()`, `Close()`
- [ ] `store.go` — SQLite FTS5 ContentStore (porter + trigram tables)
- [ ] `chunk.go` — Markdown chunking (heading-aware, code-block preserving)
- [ ] `search.go` — Three-tier fallback (porter → trigram → fuzzy)
- [ ] `fuzzy.go` — Levenshtein distance + vocabulary correction
- [ ] `snippet.go` — Smart snippet extraction
- [ ] Full test suite with table-driven tests
- [ ] `go doc` documentation on all exported types

### Phase 2: CLI + MCP (1 week)

- [ ] `cmd/gist/main.go` — `gist index <file>`, `gist search <query>`, `gist stats`
- [ ] `gist serve` — MCP server over stdio (tools: `gist_index`, `gist_search`, `gist_execute`)
- [ ] `gist doctor` — runtime and dependency diagnostics

### Phase 3: Sire Integration (1 week)

- [ ] Implement `core.StepIndexer` using Gist
- [ ] Implement `core.BeforeStepHook` for context injection
- [ ] Register `sire:local/gist.search` MCP tool
- [ ] Wire into `cmd/sire/serve.go` behind feature flag `"gist"`
- [ ] Integration tests with existing engine test harness

### Phase 4: Advanced Features (2 weeks)

- [ ] `store_postgres.go` — PostgreSQL FTS backend for production multi-tenant
- [ ] `session.go` + `snapshot.go` — Session event tracking and resume snapshots
- [ ] `executor.go` — Polyglot subprocess executor (shell, Python, Go, JS)
- [ ] JSON and YAML structured chunking
- [ ] Batch indexing with goroutine pool
- [ ] `gist bench` — performance benchmarking command

### Phase 5: Open-Source Launch (1 week)

- [ ] README with examples and benchmarks
- [ ] Apache 2.0 LICENSE
- [ ] GitHub Actions CI (lint, test, release)
- [ ] `go install github.com/sirerun/gist/cmd/gist@latest`
- [ ] Announcement post positioning Gist alongside Mint

## Dependencies

Minimal, all pure Go where possible:

| Dependency | Purpose | CGO |
|------------|---------|-----|
| `modernc.org/sqlite` | SQLite (pure Go, no CGO) | No |
| `github.com/yuin/goldmark` | Markdown parsing for structured chunking | No |
| `github.com/agnivade/levenshtein` | Fuzzy matching | No |
| `github.com/spf13/cobra` | CLI (consistent with Sire API) | No |
| `jackc/pgx/v5` | PostgreSQL backend (optional) | No |

Zero CGO. Single `go build` produces a static binary for any platform.

## Success Metrics

| Metric | Target |
|--------|--------|
| Context reduction | >95% (matching context-mode's 98%) |
| Search latency (p99) | <5ms for 10K chunks |
| Index throughput | >50 MB/s |
| Binary size | <15 MB |
| Test coverage | >85% |
| GitHub stars (6 months) | 1,000+ |
| Sire `sire dev` integration | Ships in Phase 3 |

## Risks

| Risk | Mitigation |
|------|-----------|
| SQLite FTS5 not available in `modernc.org/sqlite` | FTS5 is compiled in by default. Verified. |
| Go MCP SDK immaturity | Build minimal stdio JSON-RPC wrapper. Mint already has `mcpgen`. |
| Splitting focus from platform launch | Gist is a library, not a product. 6 weeks total. Accelerates `sire dev`. |
| Name collision with GitHub Gists | Different domain (library vs. service). No trademark conflict. |

## Decision

Proceed with Gist as `github.com/sirerun/gist`, Apache 2.0, following the Mint open-source pattern. Start with Phase 1 (core library) immediately — it unblocks `sire dev` local mode and gives the API a proper context management layer.

# Gist -- Design Document

## Overview

Gist is a Go library and CLI that keeps raw data out of LLM context windows. Content is chunked and indexed into PostgreSQL (or an in-memory store for testing), then retrieved on demand using a three-tier search engine (porter stemming, trigram, fuzzy correction). Callers set token budgets and get back only the most relevant snippets.

## Architecture

### Design Principles

1. **Library-first**: Import `gist` as a Go package. No daemon, no sidecar.
2. **Zero-copy indexing**: Content is chunked and indexed in-place.
3. **Three-tier search**: Porter stemming, trigram substring, fuzzy correction.
4. **Budget-aware**: Callers set token budgets. Gist returns results that fit, ranked by relevance.
5. **Pluggable storage**: PostgreSQL is the production backend. The `Store` interface allows alternative implementations (in-memory for testing, others for embedding).

### Package Layout

```
github.com/sirerun/gist/
  gist.go              # Top-level API: New(), Index(), Search(), Stats()
  store.go             # Store interface and types
  store_postgres.go    # PostgreSQL FTS implementation (tsvector + pg_trgm)
  store_memory.go      # In-memory Store for testing and prototyping
  chunk.go             # Markdown chunking (heading-aware, code-block preserving)
  chunk_json.go        # JSON structured chunking
  chunk_yaml.go        # YAML structured chunking
  search.go            # Three-tier search: porter -> trigram -> fuzzy
  fuzzy.go             # Levenshtein distance + vocabulary correction
  snippet.go           # Smart snippet extraction around match positions
  session.go           # Session event tracking
  snapshot.go          # Priority-tiered resume snapshot renderer
  executor.go          # Polyglot subprocess executor
  security.go          # Deny/allow policy enforcement
  mcp/                 # MCP server adapter (stdio)
  cmd/gist/            # CLI binary
  internal/runtime/    # Runtime detection for executor
```

### Core API

```go
func New(opts ...Option) (*Gist, error)

func WithPostgres(dsn string) Option
func WithMemory() Option           // In-memory store (no external deps)
func WithStore(s Store) Option     // Custom backend
func WithTokenBudget(max int) Option
func WithProjectRoot(dir string) Option
```

### Store Interface

Defined in `store.go`. Methods: SaveSource, SaveChunk, SearchPorter, SearchTrigram, VocabularyTerms, Sources, Stats, Close.

Two implementations:
- `PostgresStore` (`store_postgres.go`): Production. Uses tsvector + pg_trgm.
- `MemoryStore` (`store_memory.go`): Testing and prototyping. Uses strings.Contains for trigram-like search, word tokenization for porter-like search.

## Conventions

- Go standard library preferred over third-party packages.
- Table-driven tests. No testify.
- Conventional Commits for all commit messages.
- Zero CGO. Single static binary.
- GOWORK=off for all go commands (parent go.work does not include gist).

## Key File Paths

- `docs/plan.md` -- Active development plan.
- `docs/proposal.md` -- Original proposal with rationale.
- `docs/announcement.md` -- Open-source launch post.
- `.github/workflows/ci.yml` -- CI pipeline (lint, test with PostgreSQL, build).
- `.github/workflows/release-please.yml` -- Release pipeline (release-please + GoReleaser).
- `.goreleaser.yml` -- GoReleaser config with Homebrew tap.
- `.golangci.yml` -- Linter config.

## Completed Phases

- **Phase 1** (Core Library): store.go, chunk.go, search.go, fuzzy.go, snippet.go, gist.go, store_postgres.go, tests, docs. Completed 2026-03-13.
- **Phase 2** (CLI + MCP): cmd/gist/ with index, search, stats, serve, doctor, bench, version commands. mcp/ package. Completed 2026-03-13.
- **Phase 4** (Advanced Features): session.go, snapshot.go, executor.go, security.go, JSON/YAML chunking, batch indexing. Completed 2026-03-13.
- **Phase 5** (Open-Source Launch): README, LICENSE, CI, release pipeline, Homebrew tap, announcement. Completed 2026-03-13.

## References

- Proposal: `docs/proposal.md`
- Plan: `docs/plan.md`
- Repository: `github.com/sirerun/gist`

## Hosted registry recovery planning reference — 2026-10-04

Hosted architecture remains governed by RFC-002 and ADR004-008; managed gateway remains RFC-003 and ADR009-010. The [registry recovery delivery plan](plans/registry-recovery.md) addresses enrollment, persistent signing identity, canonical wire composition, durable events and real action artifacts. Caller execution/authority remains owned by the consuming protected-effects runtime. Detailed new key/enrollment/transaction choices are proposals pending reviewed disposition, not accepted architecture supplied by this planning edit.

## Canonical AWS production boundary — 2026-10-04

[ADR011](adr/011-gist-production-canonical-origin.md) records the founder-selected canonical origin https://gist.sire.run on the accepted AWS architecture and confirmed registry/caller-owned scope. It supersedes ADR008's earlier origin only. Exact origin/audience/issuer/redirect migration follows ADR007; DNS/ACM/ALB, task config and callback consent require reviewed migration, not a wildcard alias. Registry artifact identity is unchanged. [The production plan](plans/registry-recovery.md) tracks code, private infrastructure owner receipts, qualified release/cutover, full registry/client/provider acceptance and operations through one terminal production predicate. Managed gateway remains separate.

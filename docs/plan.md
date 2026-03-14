# Plan: Gist -- In-Memory Store and E2E Testing

**Date:** 2026-03-14
**Prior work:** See `docs/design.md` for completed Phases 1, 2, 4, 5.

## Context

### Problem Statement

Gist's test suite has a gap: mock stores verify method calls but do not test the full Index -> Search pipeline. The only way to validate that indexed content is actually searchable is with a live PostgreSQL instance. This means:

- Local development requires PostgreSQL or Docker/testcontainers.
- Quick prototyping and demos require database setup.
- The CLI cannot run standalone for small workloads.

### Objectives

1. Implement an in-memory Store that supports the full Index -> Search -> Stats pipeline using Go standard library string operations.
2. Add `WithMemory()` option so `gist.New(gist.WithMemory())` works with zero external dependencies.
3. Write end-to-end tests that exercise the full Gist API (Index, Search, BatchIndex, Stats) against the in-memory store.
4. Make the in-memory store the default when no store option is provided, enabling `gist.New()` without arguments.

### Non-Goals

- Replicating PostgreSQL's exact BM25 scoring or tsvector stemming behavior.
- Persistence (the in-memory store is ephemeral).
- Production-scale performance (the in-memory store is for testing and small workloads).

### Constraints

- Zero new external dependencies.
- Must satisfy the existing Store interface exactly -- no interface changes.
- Thread-safe (sync.RWMutex).
- GOWORK=off for all go commands.

### Success Metrics

| Metric | Target |
|--------|--------|
| E2E tests pass without PostgreSQL | Yes |
| `gist.New()` works without arguments | Yes |
| CI lint + test green | Yes |
| Overall test coverage | >85% |
| In-memory store test coverage | >95% |

### Decision Rationale

See `docs/adr/001-in-memory-store.md`.

## Scope and Deliverables

### In Scope

- `store_memory.go` -- In-memory Store implementation.
- `store_memory_test.go` -- Unit tests for the in-memory store.
- `gist_e2e_test.go` -- End-to-end tests using in-memory store.
- Update `gist.go` -- Add `WithMemory()` option, make in-memory the default.
- Update `cmd/gist/main.go` -- Allow CLI to run without --dsn (uses in-memory).
- Update `README.md` -- Document zero-dependency usage.

### Out of Scope

- Changing the PostgreSQL store.
- Adding other storage backends (BoltDB, SQLite).
- Changing the Store interface.

### Deliverables

| ID | Description | Acceptance Criteria |
|----|-------------|-------------------|
| D1 | store_memory.go | Implements Store interface. All methods functional. Thread-safe. |
| D2 | store_memory_test.go | >95% coverage on store_memory.go. Table-driven. |
| D3 | gist_e2e_test.go | Tests Index->Search, BatchIndex->Search, Stats, Close. No PostgreSQL needed. |
| D4 | WithMemory() option | `gist.New(gist.WithMemory())` returns working Gist. |
| D5 | Default store | `gist.New()` uses in-memory store when no store configured. |
| D6 | CLI without --dsn | `gist index` and `gist search` work without --dsn (in-memory). |
| D7 | README update | Shows zero-dependency quick start. |

## Checkable Work Breakdown

### E1: In-Memory Store Implementation

- [ ] T1.1 Implement store_memory.go  Owner: TBD  Est: 45m
  - MemoryStore struct with sync.RWMutex, source/chunk slices, ID counters.
  - SaveSource: append to sources slice, return with auto-incremented ID.
  - SaveChunk: append to chunks slice, return with auto-incremented ID.
  - SearchPorter: tokenize query into words, match chunks where all query words appear (case-insensitive). Score = number of matching words / total words in chunk. Limit and SourceFilter respected.
  - SearchTrigram: match chunks where query substring appears (case-insensitive strings.Contains). Score = len(query) / len(content). Limit and SourceFilter respected.
  - VocabularyTerms: extract unique lowercase words from all chunk content. Use strings.Fields and strings.ToLower.
  - Sources: return copy of sources slice.
  - Stats: count chunks, sources, sum bytes.
  - Close: no-op, return nil.
  - Acceptance: compiles, satisfies Store interface, go vet clean.
  - Dependencies: none.

- [ ] T1.2 Write store_memory_test.go  Owner: TBD  Est: 45m
  - Table-driven tests for every Store method.
  - Test cases: save source and retrieve, save chunks and search (porter), save chunks and search (trigram), search with source filter, search with limit, vocabulary terms extraction, stats accuracy, empty store behavior, concurrent read/write safety (goroutines + race detector).
  - Acceptance: `GOWORK=off go test -run TestMemory -race -v` passes. >95% coverage on store_memory.go.
  - Dependencies: T1.1.

### E2: Gist API Integration

- [ ] T2.1 Add WithMemory() option and default store  Owner: TBD  Est: 20m
  - Add `WithMemory() Option` to gist.go that sets `cfg.store = NewMemoryStore()`.
  - Change `New()` to use `NewMemoryStore()` as default when no store is configured (remove the "store required" error).
  - Acceptance: `gist.New()` returns a working Gist. `gist.New(gist.WithMemory())` returns a working Gist. `gist.New(gist.WithPostgres(dsn))` still works.
  - Dependencies: T1.1.

- [ ] T2.2 Write gist_e2e_test.go  Owner: TBD  Est: 30m
  - End-to-end test: New() -> Index markdown -> Search -> verify results contain expected snippets.
  - End-to-end test: New() -> Index multiple documents -> Search -> verify cross-document results.
  - End-to-end test: New() -> BatchIndex -> Search -> verify results.
  - End-to-end test: New() -> Index -> Stats -> verify counts and bytes.
  - End-to-end test: New() -> Index -> Search with budget -> verify truncation.
  - End-to-end test: New() -> Index -> Search for typo -> verify fuzzy fallback finds results.
  - Acceptance: all tests pass with `GOWORK=off go test -run TestE2E -race -v`. No PostgreSQL required.
  - Dependencies: T2.1.

### E3: CLI and Documentation

- [ ] T3.1 Update CLI to work without --dsn  Owner: TBD  Est: 20m
  - Modify cmd/gist/main.go PersistentPreRunE: if --dsn is empty and GIST_DSN is empty, create Gist with in-memory store instead of erroring.
  - Print a notice: "Using in-memory store (data will not persist). Use --dsn for PostgreSQL."
  - Acceptance: `gist index README.md` works without --dsn. `gist search "context"` returns results. `gist stats` shows counts.
  - Dependencies: T2.1.

- [ ] T3.2 Update README.md  Owner: TBD  Est: 15m
  - Add zero-dependency quick start section showing `gist.New()` without PostgreSQL.
  - Update CLI section to note that --dsn is optional.
  - Acceptance: README shows both in-memory and PostgreSQL usage paths.
  - Dependencies: T2.1.

### E4: Quality Gates

- [ ] T4.1 Run linter and fix findings  Owner: TBD  Est: 10m
  - `GOWORK=off go vet ./...`
  - `GOWORK=off go build ./...`
  - Acceptance: zero errors, zero warnings.
  - Dependencies: T1.1, T1.2, T2.1, T2.2, T3.1, T3.2.

- [ ] T4.2 Verify CI green  Owner: TBD  Est: 10m
  - Push to main.
  - Verify CI (lint, test, build) passes.
  - Acceptance: all 3 CI jobs green.
  - Dependencies: T4.1.

## Parallel Work

### Track Layout

| Track | Tasks | Description |
|-------|-------|-------------|
| A: Store | T1.1, T1.2 | In-memory store and its tests |
| B: API | T2.1, T2.2 | WithMemory option and e2e tests |
| C: CLI/Docs | T3.1, T3.2 | CLI update and README |
| D: Quality | T4.1, T4.2 | Lint, CI verification |

### Sync Points

- T1.1 must complete before T1.2, T2.1, T3.1 can start.
- T2.1 must complete before T2.2, T3.1, T3.2 can start.
- All of A, B, C must complete before D starts.

### Maximum Parallelism

**Wave 1** (1 task -- foundation):
- T1.1: Implement store_memory.go

**Wave 2** (3 tasks -- parallel after T1.1):
- T1.2: Write store_memory_test.go
- T2.1: Add WithMemory() option and default store
- T3.1: Update CLI to work without --dsn (can stub on T2.1's interface, or run after T2.1)

**Wave 3** (3 tasks -- parallel after T2.1):
- T2.2: Write gist_e2e_test.go
- T3.2: Update README.md
- T4.1: Run linter and fix findings (if T1.2 and T3.1 are done)

**Wave 4** (1 task):
- T4.2: Verify CI green

Note: Wave 2 has T3.1 which depends on T2.1. If strict dependency is enforced, T3.1 moves to Wave 3 and Wave 2 has 2 tasks. To maximize parallelism, T3.1 can start by reading gist.go and preparing the CLI change, then integrate WithMemory() once T2.1 commits.

## Timeline and Milestones

| Milestone | Tasks | Exit Criteria |
|-----------|-------|--------------|
| M1: Store works | T1.1, T1.2 | MemoryStore passes all unit tests with -race |
| M2: API integrated | T2.1, T2.2 | gist.New() works, e2e tests pass |
| M3: CLI standalone | T3.1, T3.2 | CLI works without --dsn, README updated |
| M4: Ship | T4.1, T4.2 | CI green, pushed to main |

## Risk Register

| ID | Risk | Impact | Likelihood | Mitigation |
|----|------|--------|------------|------------|
| R1 | In-memory search ranking differs significantly from PostgreSQL | Medium | High | Document clearly that MemoryStore is for testing/prototyping. Keep PostgreSQL integration tests in CI. |
| R2 | Users accidentally use MemoryStore in production | Medium | Low | Print warning when MemoryStore is used. Document in README and godoc. |
| R3 | Making in-memory the default breaks existing code that expects an error | Low | Low | Existing code passes WithPostgres or WithStore explicitly. Only code that called New() without options would have gotten an error before; now it gets a working instance. This is strictly additive. |

## Operating Procedure

- **Definition of done**: Code compiles, tests pass with -race, go vet clean, committed with Conventional Commits.
- **Review**: Read all changed files before marking complete.
- **Testing**: Every new function must have at least one test.
- **Linting**: Run `GOWORK=off go vet ./...` and `GOWORK=off go build ./...` after every code change.
- **Commits**: Small, logical. Do not commit files from different directories together.

## Progress Log

### 2026-03-14 -- Plan Created

- Created plan for in-memory Store implementation and e2e testing.
- Created `docs/design.md` with stable knowledge from completed phases.
- Created `docs/adr/001-in-memory-store.md` documenting the decision to add an in-memory Store.
- Trimmed completed Phases 1, 2, 4, 5 from the plan (preserved in design.md).
- Phase 3 (Sire Integration) removed from this plan -- it belongs in sirerun/api repo.

## Hand-off Notes

- The Store interface is in `store.go` and must not be changed.
- PostgreSQL store (`store_postgres.go`) must not be modified.
- The in-memory store must be pure Go standard library -- zero external dependencies.
- Use `GOWORK=off` for all go commands because the parent directory has a go.work that does not include gist.
- CI runs at `.github/workflows/ci.yml` with PostgreSQL service container for integration tests.
- The `gist.New()` constructor currently errors when no store is provided. After this work, it defaults to in-memory.

## Appendix

### MemoryStore Search Algorithm

**SearchPorter** (word-level matching):
1. Split query into words using `strings.Fields(strings.ToLower(query))`.
2. For each chunk, split content into words the same way.
3. Count how many query words appear in the chunk's word set.
4. If at least one query word matches, include the chunk.
5. Score = matched_words / total_chunk_words.
6. Sort by score descending, apply limit.

**SearchTrigram** (substring matching):
1. Lowercase both query and chunk content.
2. If `strings.Contains(lowerContent, lowerQuery)`, include the chunk.
3. Score = len(query) / len(content) (longer match relative to content = higher score).
4. Sort by score descending, apply limit.

This produces correct boolean results (found/not found) with approximate ranking, which is sufficient for e2e testing and prototyping.

# ADR 001: Add In-Memory Store Implementation

## Status
Accepted

## Date
2026-03-14

## Context

Gist's Store interface currently has only one implementation: PostgresStore. All end-to-end tests either use mock stores (which fake search behavior and do not validate the index-then-search pipeline) or require a live PostgreSQL instance (which CI provides but local development and quick prototyping do not).

The original plan stated "PostgreSQL is the sole storage backend." This was correct for production. However, it created two problems:

1. **Testing gap**: Mock stores verify that Gist calls the right Store methods, but they do not verify that indexing content actually produces searchable results. The only way to test the full pipeline (Index -> Search -> verify results) is with PostgreSQL.

2. **Developer experience**: Users who want to try Gist without PostgreSQL cannot. The `New()` constructor requires `WithPostgres(dsn)` or `WithStore(s)`. There is no zero-dependency option for prototyping, demos, or embedding in tools where PostgreSQL is unavailable.

BoltDB/bbolt was considered but rejected because it does not provide full-text search or trigram matching. An in-memory implementation with simple Go string operations (strings.Contains, word tokenization) is sufficient for correctness testing and prototyping.

## Decision

Add `store_memory.go` implementing the Store interface with in-memory data structures:

- **Data storage**: slices for sources and chunks, protected by sync.RWMutex for thread safety.
- **SearchPorter**: Tokenize query into words, match against chunk content using case-insensitive word containment. Score by match count.
- **SearchTrigram**: Use strings.Contains (case-insensitive) for substring matching.
- **VocabularyTerms**: Extract unique words from all indexed chunk content.
- **Auto-incrementing IDs**: Simple integer counters for source and chunk IDs.

Add `WithMemory() Option` to gist.go so users can create a Gist instance with zero external dependencies:

```go
g, _ := gist.New(gist.WithMemory())
```

The in-memory store is NOT intended to replicate PostgreSQL's ranking or scoring. It provides correct boolean search results with approximate scoring, which is sufficient for:
- End-to-end pipeline testing (Index -> Search -> assert results found)
- Local prototyping and demos
- Embedding Gist in tools where PostgreSQL is unavailable
- CLI usage without a database for small workloads

## Consequences

**Positive:**
- Full pipeline testing without PostgreSQL.
- Zero-dependency getting-started experience.
- CLI and library work out of the box for demos.
- Tests run faster (no container startup).

**Negative:**
- Search ranking differs from PostgreSQL (no real BM25, no real tsvector stemming).
- Not suitable for production workloads (no persistence, no concurrent multi-process access).
- Adds a second Store implementation to maintain.
- Users might mistakenly use it in production if not clearly documented.

# ADR 005: Source Supersession, TTL, and Opt-In Recency Decay

## Status
Accepted

## Date
2026-07-10

## Context

Agent sessions are temporal, but lexical relevance is timeless. A browser snapshot indexed an hour ago can outrank the one indexed 30 seconds ago because it happens to match the query better. The failure mode that bites agents mid-session is retrieving a confidently correct answer about a page state that no longer exists.

Three candidate mechanisms differ sharply in risk:

- **Supersession** is deterministic and needs no tuning: re-indexing the same logical source should retire the old version.
- **TTL** is explicit and caller-controlled.
- **Time-decay scoring** is a magic number, and decay rate is a property of the *source kind*, not document age — a CLAUDE.md indexed at session start is old but not stale, while a browser snapshot ages in seconds. A global decay factor would trade "confidently stale" errors for "silently down-ranked but still correct" errors, which are harder to notice.

## Decision

Ship the deterministic mechanisms as the core; make decay opt-in and per-source.

1. **Supersession.** New index option `WithSupersedes(label string)`. Indexing with it marks all prior sources with that label as superseded; superseded chunks are excluded from search. Indexing with `WithSource(label)` where the label already exists implies self-supersession by default (`WithKeepPrevious()` opts out). Superseded sources remain in the store for stats/audit until vacuumed.
2. **TTL.** New index option `WithTTL(d time.Duration)`. Expired sources are excluded from search results at query time (no background reaper required); a `Vacuum(ctx)` method deletes expired and superseded rows on demand.
3. **Opt-in recency decay.** New index option `WithHalfLife(d time.Duration)` declaring the source's volatility. At search time, a chunk from a half-life-bearing source has its BM25 score multiplied by `0.5^(age/halfLife)`. Sources without a half-life are never decayed. No global decay knob exists.
4. **Store changes.** `Source` gains `IndexedAt`, `SupersededBy` (nullable), `ExpiresAt` (nullable), and `HalfLife` (nullable). Both stores (memory, postgres) filter superseded/expired chunks in all three search tiers.
5. **MCP surface.** `gist_index` gains optional `supersedes`, `ttl_seconds`, and `half_life_seconds` arguments; the tool description tells agents to re-index volatile sources (browser snapshots, live tool output) under a stable label so supersession applies.
6. **Search result surface.** `SearchResult` gains `IndexedAt` so callers can judge freshness themselves.

## Consequences

**Positive:**
- Fixes the stale-snapshot-outranks-fresh failure mode deterministically for the common case, with zero tuning.
- Self-supersession on label reuse means existing agent workflows (re-indexing a page under the same label) get correct behavior without code changes.
- Decay cannot silently harm stable corpora: it only applies where the indexer declared volatility.
- Differentiator versus static-document RAG designs, which do not model live tool output.

**Negative:**
- Self-supersession by default is a behavior change for callers who intentionally indexed multiple documents under one label; `WithKeepPrevious` is the escape hatch and the change is called out in the changelog as breaking.
- Query-time expiry filtering plus decay multiplication adds a small cost to every search.
- Store schema migration required for the postgres backend.

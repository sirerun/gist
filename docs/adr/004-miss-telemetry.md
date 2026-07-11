# ADR 004: Retrieval Miss Telemetry

## Status
Accepted

## Date
2026-07-10

## Context

`gist_stats` reports the savings half of gist's value story (bytes indexed vs returned), but nothing measures retrieval *quality*. When a search silently returns weak or empty results, the agent falls back to re-reading the raw source and the failure leaves no trace. This means:

- Savings numbers can hide cases where the agent got worse results.
- Roadmap questions ("do we need an embedding tier?") are argued from opinion, not data.

Gist cannot observe the agent's subsequent `Read` calls, so "agent re-read the raw source" must be approximated from signals gist *can* see: zero-result queries, low-score results, re-indexing of an already-indexed source shortly after a weak search against it, and repeated reworded queries in the same session.

## Decision

Add a local-only miss log. No data leaves the box; zero new dependencies.

1. **Miss detection.** A search is recorded as a *miss* when: (a) it returns zero results across all three tiers, or (b) the top result's BM25 score falls below a configurable floor (`WithMissScoreFloor`, default derived from corpus statistics). Additionally record a *suspected miss* when the same source is re-indexed within the session after a low-score search that matched it, or when a query shares ≥60% trigram overlap with a prior query in the same session (reworded retry).
2. **Miss classification.** Each miss record carries a machine-assigned class so the data can answer the embeddings question later:
   - `no-content` — query terms absent from the corpus entirely.
   - `paraphrase-divergence` — vocabulary terms exist near the query (fuzzy/trigram had candidates) but scoring rejected them.
   - `chunking-truncation` — the matched source exists but the matching chunk scored below floor.
3. **Miss record shape.** JSONL appended next to the store (memory store: in-process ring buffer; postgres store: `gist_misses` table). Fields: timestamp, query, session ID, class, top-3 near-misses (source, title, score), score floor in effect, match layer reached.
4. **Surfacing.** `gist stats --misses` (CLI) and a `misses` summary block in the `gist_stats` MCP response: miss count, miss rate, class breakdown, and the N most recent miss records.
5. **Stats fields.** Add `MissCount` and `MissRate` to `Stats`.

## Consequences

**Positive:**
- Converts future roadmap arguments (ADR 006 in particular) from opinion into data.
- Makes the README savings claim self-honest: savings and misses are reported side by side.
- Local-only JSONL/table preserves the no-data-leaves-the-box story.
- Classification is cheap at search time — it reuses signals the three-tier engine already computes.

**Negative:**
- The re-read proxy is heuristic; it will under-count misses where the agent gives up silently and over-count exploratory re-queries.
- A score floor introduces one tunable; a bad default misreports miss rate. Mitigated by logging the floor with every record so data remains interpretable if the default changes.
- Postgres store gains one table and a write per miss.

# ADR 006: Optional Local Embedding Tier with Rank Fusion (Gated on Miss Data)

## Status
Proposed — gated on ADR 004 telemetry evidence

## Date
2026-07-10

## Context

The three-tier lexical engine (porter, trigram, fuzzy) cannot bridge paraphrase divergence: a query worded differently from the source ("how do we retry failed uploads" vs "resubmission backoff policy") finds nothing even though the content is indexed. An embedding tier would close that gap, but it is the most expensive feature on the roadmap and threatens two adoption advantages: zero dependencies and no data leaving the box.

ADR 004's miss classification exists precisely to decide this. The gate:

- **Trigger:** `paraphrase-divergence` misses form a material share of total misses (working threshold: >20% of misses across representative sessions) **after** ADR 005 ships and **after** cheaper lexical remedies are tried.
- **Cheaper remedies first:** query expansion — identifier splitting (camelCase/snake_case), stemmer-aware synonym lists, and multi-term OR expansion — fixes much paraphrase divergence in code-adjacent corpora for free. These land as tier-3.5 improvements before any embedding work.

## Decision

If and only if the gate triggers:

1. **Fourth tier, off by default.** `WithEmbeddings(model EmbeddingModel)` on the Gist constructor enables it; the default build has no embedding code path active and no new dependencies for users who don't opt in.
2. **Local-only.** The `EmbeddingModel` interface takes text and returns a vector; the shipped implementation runs a small local model (e.g., a quantized MiniLM-class model via a pure-Go or CGo-free runtime). No external API implementation is provided in this repo; the interface allows users to bring one, but the README positions local as the supported path.
3. **Reciprocal rank fusion.** Embedding results are fused with the three lexical tiers via RRF (k=60), not score mixing — BM25 and cosine scores are not comparable. Budget-fitting and snippet extraction are unchanged downstream of fusion.
4. **Storage.** Chunk vectors stored alongside chunks: pgvector for the postgres store, in-process flat index for the memory store. Vectors are computed at index time; search-time cost is one query embedding plus a nearest-neighbor scan.
5. **Telemetry closes the loop.** Miss records gain a `would_embedding_have_hit` replay field when the tier is enabled, so its actual contribution is measured, not assumed.

## Consequences

**Positive:**
- The zero-dep, local-only story is preserved for all users who don't opt in, and the no-data-leaves-the-box story is preserved even for those who do.
- RRF sidesteps score-calibration problems and keeps the existing budget-fitting contract intact.
- Building third means its necessity — and its measured benefit — is proven by ADR 004 data rather than assumed.

**Negative:**
- Even optional, an embedding runtime is real maintenance surface (model distribution, quantization, aarch64/amd64 builds).
- pgvector becomes a conditional postgres extension requirement for opted-in users.
- Index-time embedding adds latency and CPU to `Index` calls when enabled.
- If the gate never triggers, this ADR is deliberately never implemented — that is a success condition, not a failure.

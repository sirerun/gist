# ADR017 — Task preparation with optional semantic retrieval

Composition evolution: [ADR021](021-capability-composition-and-bounded-discovery.md)
defines the planned reusable capability/recipe and indexed-discovery path. This
ADR's frozen pilot and initial inbox recipe remain historical source contracts;
they do not establish generic multi-tool coverage or 100k-scale performance.

Date: 2026-10-16
Status: Accepted evaluation direction; model adoption and performance claims pending evidence.
Supersedes: RFC-002's unspecified retrieval-evaluation approach, not its frozen wire contract.

## Problem

The owner observed repeated model turns spent finding Composio tools, inspecting
schemas and discovering prerequisites. A reusable catalog alone does not remove
that overhead. Gist must prepare a complete, small, authorized tool context for
a task, while Zati remains responsible for admission, credentials and execution.

## Decision

1. Add an evaluated task-preparation path to RFC-002. Separate maintained,
   immutable craft skills/assets from derived task-specific tool-use guides.
   A guide is assembled from verified contracts, versioned recipes and current
   authorized bindings. It is not an implicitly published new skill.
2. Select candidates through interchangeable lexical, semantic and hybrid
   retrieval. Exact names/IDs retain lexical handling. Similarity ranks candidates;
   it never proves capability coverage, permission or connection readiness.
3. Evaluate `google/embeddinggemma-2`, text-only, as the first semantic candidate.
   Pin the immutable model/tokenizer/runtime revisions, dtype, query/document
   prefixes, vector dimension and normalization. Compare 256d and 768d on the
   development split; freeze the choice before held-out evaluation. No automatic
   download, mandatory inference dependency or claimed native Go support.
4. Keep inference caller-supplied behind an optional interface. Gist owns catalog
   indexing/ranking and preparation; Zati owns inference runtime selection,
   capacity and disclosure policy. The lexical path remains available. Offline
   use is a configuration property, not a claim about every adapter.
5. Precompute vectors for versioned skill descriptions, capability semantics,
   tool descriptions and maintained usage examples. Embed task intent once per
   preparation when needed. Do not embed credentials, account tokens or mailbox
   content to find tools. A remote adapter receives only content authorized for
   that destination; no silent network fallback.
6. Filter to authorized candidates before externally visible ranking, counts or
   snippets. Isolate private indexes/caches by workspace and policy. A stale
   vector may nominate an ID, but the current authoritative record, authorization,
   binding, connection and revocation state must be checked before exposure.
7. Assemble a bounded guide containing exact selected tool schemas, necessary
   instructions/assets, prerequisites and call order, error/retry guidance,
   explicit unresolved requirements and source/recipe/binding versions. Do not
   truncate executable schemas or required instructions to fit a budget. Report
   insufficient budget or unsupported intent explicitly. Recipe coverage, not
   embedding similarity, establishes which multi-step guidance is available.
8. Prefer deterministic recipe assembly. A generative planner/reranker is a
   separate experimental arm with its own pinned model, cost and authority;
   it is not hidden in the embedding baseline. Ambiguity can require bounded
   broader retrieval or clarification, never fabricated parameters.
9. Cache by content/model/recipe versions and relevant scope/policy generation.
   Current authorization and connection checks cannot be skipped by a cache hit.
   Index updates, eviction, stale-model mismatch and cold-start behavior are part
   of acceptance. Embeddings are rebuildable derived data, not source authority.

## Candidate interface boundary

The implementation design must freeze Go types before coding this path. The
conceptual preparation request carries intent, caller scope, budget and current
admitted inventory through caller-owned ports. The response carries selected
immutable skill/assets, exact schemas and bindings, guide text, readiness
findings and provenance. It contains no secret or execution grant. Do not add
routes to frozen registry v 1 or expose proposed API names as shipped contracts.

The first owner-selected workflow is a read-only inbox briefing. Its skill
contains a briefing template and requires mail reading. Zati currently has no
configured Gmail MCP connection: source development uses an explicitly controlled
adapter; actual Gmail binding, consent and live qualification remain separate.

## Consequences and alternatives

Pure lexical matching stays the dependency-free baseline and fallback. Preloading
known tools may be best for a small fixed workflow. Hybrid retrieval may improve
paraphrase recall, but adds index/runtime/memory cost and may rank plausible wrong
tools. An embedding model cannot plan arbitrary workflows or eliminate necessary
execution turns. Making a separate generative planner mandatory is rejected
pending evidence because its own turns may erase savings.

The core remains a Go library. A local inference process or native runtime is an
adapter choice to qualify, not a new mandatory Gist hosted service. RFC-003
execution remains retired. Composio may still supply tool execution/connections.

## Evidence and acceptance

[ADR018](018-tool-preparation-evaluation-and-adoption.md) and the
[evaluation protocol](../evaluations/tool-preparation.md) define comparison and
adoption. No speed, token, quality or memory advantage has been measured here.
Google's model card describes selective text-only loading, retrieval prefixes,
256/768 dimensions and BF16/FP32 requirements; these are vendor specifications,
not qualification on our hardware:
[model card](https://huggingface.co/google/embeddinggemma-2),
[Google documentation](https://ai.google.dev/gemma/docs/embeddinggemma).

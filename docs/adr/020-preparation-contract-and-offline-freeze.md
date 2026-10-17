# ADR020 — Additive preparation and offline evaluation contract

Date: 2026-10-16
Status: Source contract frozen before corpus/model/harness implementation.
Source anchor: `76d9c5f` (parent library/provenance source).
Implements: T-PREP.0 under ADR017/018; no default adoption or hosted expansion.

## Public Go boundary

Add `catalog/preparation`; existing frozen wire contracts remain unchanged.

- `Mode string`: `lexical`, `semantic`, `hybrid`.
- `Fallback string`: `deny`, `lexical` (only an explicitly selected fallback).
- `Request`: `Principal ports.Principal`, `Intent string`, `RuntimeID string`,
  `Mode Mode`, `Guide bool`, `Fallback Fallback`, `MaxBytes int`, `TopK int`,
  `CatalogVersion string`, `RecipeVersion string`.
- `EmbeddingRequest`: `Principal ports.Principal`, `ModelVersion string`,
  `Purpose string` (`query` or `document`), `Texts []string`.
- `EmbeddingResult`: `ModelVersion string`, `Vectors [][]float64`.
- `Embedder`: `Embed(context.Context, EmbeddingRequest) (EmbeddingResult,error)`.
  It is optional, caller-owned and explicit; no package download or network fallback.
- `Resolver`: existing `Resolve(context.Context,resolution.Request)
  (resolution.Result,error)`.
- `Service`: caller-owned `Catalog ports.CatalogStore`,
  `Authorizer ports.Authorizer`, `Retriever packages.Retriever`, `Resolver`,
  optional `Embedder`, and explicit `ModelVersion string`.
- `Result`: `Status string`, `Candidates []Candidate`, `Selected *Bundle`,
  `Findings []string`, `Mode Mode`, `Provenance Provenance`.
- `Candidate`: `Ref ports.ArtifactRef`, `Score float64`.
- `Bundle`: complete `Skill packages.Package`, `Resolution resolution.Result`,
  `Contracts []Contract`, deterministic `Guide string`.
- `Contract`: `Ref ports.ArtifactRef`, `Digest ports.Digest`, `Bytes []byte`.
- `Provenance`: source/catalog/model/recipe versions, principal workspace and
  policy generation, selected immutable skill/package and contract digests.

`Prepare` takes current authoritative catalog records, filters current policy
and admission before ranking/embedding, and only sends authorized catalog text
and task intent to the explicitly selected adapter. Caller policy governs any
remote adapter destination. Credentials/message contents are excluded from the
catalog protocol. Exact schema/asset bytes are retained, never shortened to fit.
No match, ambiguity, missing connection, unsupported runtime, insufficient budget,
stale model/index and denied/revoked inputs return explicit non-readiness.
Similarity never establishes capability coverage. Existing exact resolver and
pinned package retrieval establish the closure; current authorization, record
state and immutable pins are rechecked before final exposure. No embedding cache
may substitute for current checks. The initial implementation uses no persistent
embedding cache: every call binds fresh scope/catalog/model/recipe/policy input.
Changing any version invalidates external vector input or causes explicit refusal.

Lexical ranking uses existing public discovery. Semantic ranking uses normalized
cosine vectors with frozen minimum score0.30; hybrid uses reciprocal-rank fusion with constant60. Primary k is4,
with development-only alternatives8/16. Deterministic guide `inbox-v1` is a
maintained recipe only for the admitted inbox skill: complete instructions/assets,
exact contracts, read-only prerequisites/list-before-read call order and refusal
rules. No arbitrary generative planning. L0/E0/H0 omit this guide; L1/H1 include it.

## Current connection port correction (before source acceptance)

The legacy resolver's `OwnedConnections` boolean is never defaulted true.
`Service.CurrentReadiness` is optional; absent it, resolution uses false, so a
credentialed tool remains nonready. The additive `CurrentReadiness` interface is
`CheckCurrentReadiness(context.Context,ReadinessRequest)(ReadinessDecision,error)`.
The request carries principal, runtime, exact skill, current resolved closure and
`ReadinessScope` (principal digest including scope/policy, runtime, skill and
canonical closure-pin digest). The response repeats that exact scope plus Ready,
policy Generation, authoritative CheckedAt and ValidUntil. When configured,
explicit `Service.Clock` and positive `MaxReadinessAge` are required. Missing,
mismatched, unavailable, stale, expired or false assertions fail closed.

First resolve with false, obtain the current scoped decision, then resolve with
its true assertion only if admitted. Compare immutable closure before/after that
assertion. After package/contract I/O repeat the entire fresh check and compare
closure plus connection generation before exposure. Retain legacy finding
provenance `client_asserted`; this remains caller evidence, not Gist OAuth or
provider qualification. Current catalog/policy/trust observations are point in
time, not atomic with later execution. Caller rechecks at use. Verified skill
manifest bytes must match the metadata consumed by the resolver; altered metadata
cannot remove required capabilities. This recipe rejects verified local execution
or filesystem requirements explicitly.

The `inbox-v1` recipe also binds the exact maintained source manifest digest;
an independently admitted package copying its logical ID/version cannot acquire
that static recipe merely by naming itself the same. Repacked archives of the
same verified manifest/inventory remain valid immutable byte instances.

## Frozen pilot protocol

Minimum120 independently reconciled synthetic cases,60 development and60 held
out, disjoint workflow/template families before paraphrasing, six primary strata
of10 each per split. The corpus author seals held-out inputs/labels outside the
implementation-accessible tree until candidate freeze. Implementers do not open
that artifact. All labels, manifest, code, model/runtime and recipe/index versions
are hashed. Corpus/development qualification is not held-out evaluation (.9).

Reference source-test targets, fixed before results: warm preparation p95<=500ms
excluding inference, complete context<=65536 encoded bytes, embedding adapter
RSS<=4GiB, model load<=30s on the named reference Linux host. These are targets,
not measured claims or target-Mac acceptance. Qualification hard stop envelope
is separate:8GiB downloads,24GiB RSS,4CPU threads,one GPU job,20minutes/process,
45minutes aggregate. Passing the hard envelope does not imply passing targets.
Pin public text-only `google/embeddinggemma-2` revision
`914f7f89142e33e77833254d9c9b90c3cef7303b`; official query/document prefixes,
256/768 dimensions, normalized BF16/FP32; compatible runtime revision is recorded
only after actual qualification. Never substitute random vectors for a model arm.

Whole-task binary noninferiority uses independent family-level discordant event
bounds: for each family, candidate-only and baseline-only success fractions are
bounded conservatively by any candidate-only benefit and any baseline-only harm
across repeats. Lower benefit uses the event "all repeats candidate-only";
upper harm uses "any repeat baseline-only". Use Clopper–Pearson one-sided bounds,
alpha/4 per event for four bounds (union bound95% total). The lower effect is
CP-lower(all candidate-only wins) minus CP-upper(any baseline-only losses);
the upper effect is CP-upper(any candidate-only wins) minus CP-lower(all
baseline-only losses). This bounds the **equally weighted construction-family mean** paired success
difference (mean all case×repeat differences within each family, then equal
weight across families), despite
within-family case/repeat dependence; independent families are the sampling units.
Unequal case counts do not make this a pooled per-attempt estimand. Pooled
all-attempt success is descriptive and separately denominated; the2pp primary
noninferiority margin remains unchanged on the family-balanced estimand.
At zero observed discordance among60families the upper harm bound is
`1-(0.0125)^(1/60)` (about7.04pp), so it cannot establish a2pp margin. Register
additional independent families before evaluating them; never relax the margin.
Candidate/baseline success definition and excluded/NOT_RUN denominators remain
fixed; no bootstrap degeneracy or pseudo-replication across repeats.

Primary offline metrics: complete closure coverage and correct abstention by
stratum/family. End-to-end baselines/thresholds remain ADR018/protocol. No live
whole-task model/provider execution is authorized by offline source tests.

## Harness and authority

`tools/evaluation` provides pinned local embedding qualification, corpus checking,
L0/L1/E0/H0/H1 preparation runs, dry-run estimator and deterministic aggregate
report. C0/C1 are explicit NOT_RUN adapters until exact provider/model/account,
call/dollar/expiry/data authorities are supplied. Missing usage receipts mean
model tokens/dollars are null, never inferred from bytes. Every attempted run,
including failures/retries/cold attempts, retains timing, status, error, scope,
source/model/catalog/recipe hashes and actual available usage categories.
Qualification inputs and outputs are synthetic. Real inbox product content is
separated from synthetic workflow probes. Source qualification does not claim
Gmail readiness, measured comparative savings, adoption, release or installation.

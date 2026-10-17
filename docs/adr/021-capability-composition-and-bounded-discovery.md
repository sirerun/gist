# ADR021 — Semantic capabilities, typed composition and bounded discovery

Date: 2026-10-17
Status: Accepted design for implementation planning; implementation pending.
Supersedes: unresolved composition choices in RFC-002 §18 and prospective
inbox migration guidance. Extends ADR017/020 without changing their frozen
pilot, existing wire formats or completed evidence.

## Decision and observed baseline

Use independently meaningful semantic capabilities composed by a skill. A skill
requires all non-optional capabilities (AND). Bindings implementing the same
capability are alternatives (OR); ambiguity remains `requires_selection`.
One binding targets one exact tool. A capability is not necessarily one HTTP
request: a real externally implemented composite tool can satisfy one contract.
Do not invent composite executors merely to make a benchmark pass.

Zati at e2dbcd41 already admits `mail/inbox/read@1.0.0` through distinct list/read
tool roles on one exact connection, validates package/manifest and schema pins,
checks current admission and filters offered tools. This is a supported legacy
inbox-specific mapping, not a general Gist multi-tool binding contract. Gist's
resolver uses a singular tool reference and refuses ambiguous alternative
bindings. Preserve both paths while introducing new immutable skill versions.

Gist owns portable catalog contracts, validation, selection and preparation.
Zati owns task admission, provider connections, credentials, approvals, execution,
retries, effects and revocation checks at use. RFC-003 remains retired. Hosted
expansion remains paused except repairs and existing consumer obligations.

## Semantic contracts and compatibility

The new inbox skill requires `mail/messages/list@1.0.0` and
`mail/message/read@1.0.0`. List returns bounded message identifiers and pagination
metadata; read accepts an identifier and returns permitted message fields. Both
are external-read semantics; neither grants send, draft, label, mark-read,
delete, attachment or linked-content access. Canonical input/output JSON schemas,
error semantics and conformance examples are frozen before implementation.
A provider that marks read as a side effect fails this read-only contract unless
an admitted mode prevents that effect. Provider conformance is explicit evidence,
not an inference from tool names or embedding similarity.

Publish a new inbox skill/recipe version with these requirements. Never mutate
old `mail/inbox/read@1.0.0`, skill packages, admission records or evaluation data.
Existing frozen registry v1/v2 formats remain unchanged. A versioned composition
asset is packaged in the skill ZIP and bound by its immutable package digest;
additive library APIs consume it. Consumers explicitly negotiate support for the
new recipe/version before assignment; unsupported consumers refuse readiness.
Do not add silently ignored executable meaning to existing wire fields.

A concrete tool may implement more than one capability if each contract has an
independent admitted binding. Deduplicate identical offered tool pins only after
checking every required contract and preserving their evidence. Different schema
or connection pins cannot be collapsed by tool name or ID alone.

## Typed composition asset: `gist.composition/v1`

This is a design contract, not a claim that a parser is shipped. Strict parsing
rejects unknown fields, duplicate JSON keys, cycles and unbounded constructs.
The asset contains:

- `schema_version`, and exact skill ID/version plus manifest digest (no circular
  whole-package digest inside the package).
- `requirements`: unique local keys mapping to exact capability ID/version and
  canonical contract digest; required/optional status must match the manifest.
- `connection_groups`: named lists of requirement keys whose selected bindings
  must share caller scope, provider connection identity/version and account
  identity attested by the caller. Display names never establish affinity.
- `steps`: unique IDs, requirement keys, predecessor IDs, and typed input mappings
  from admitted task fields or predecessor outputs using bounded JSON Pointers.
  Mapping types must be compatible with the pinned contract schemas; no arbitrary
  scripts, interpolation, model-authored expressions or secret references.
- `foreach`: an explicit bounded iteration over a predecessor array. Inbox reads
  at most 20 selected IDs; pagination, total calls and aggregate output limits
  are explicit, and reaching a limit produces partial/truncated evidence rather
  than silent completeness. The initial inbox profile has one bounded list call
  and at most 20 read calls; another page requires a separately admitted recipe.
- `limits`: maximum steps, iterations, calls, input/output bytes and guide bytes.
  Caller limits may tighten these but cannot silently weaken required coverage.

The library validates structure, contracts and bounded dataflow. Zati binds
actual input values, validates runtime outputs before downstream use, enforces
loop/call limits and handles failures through ordinary effect policy. Mail text
is untrusted data, never executable recipe or authority. A missing optional
capability must disable every dependent optional step; a required step cannot
consume a possibly absent output without an explicit validated fallback.

Inbox example: `list` and `read` requirements share the `mailbox` group. Step
`list_unread` consumes the admitted query; `read_selected` iterates over its
message IDs, passes each ID to read, then the caller summarizes successful reads.
The summary records partial reads and pagination. The recipe describes an
allowed workflow; it does not cause Gist to execute it.

## Resolution and readiness

1. Use an explicitly selected skill pin, or retrieve a bounded authorized skill
   shortlist. Caller task admission must affirm the task/recipe fit.
2. Validate exact package, manifest, composition and capability digests. Resolve
   only the selected skill's requirements using capability/version indexes.
3. Filter bindings by current workspace policy, runtime and admitted connections.
   Apply explicit caller selections and shared-connection constraints before
   choosing bindings. Do not enumerate Cartesian products. For a connection
   group, intersect eligible connection identities across its requirements; zero
   means unavailable, multiple viable choices require explicit selection.
4. Produce one finding per capability plus typed composition/group validation
   evidence. Every required finding and cross-requirement constraint must pass.
   A flat closure list never encodes implicit AND or dataflow semantics.
5. Bind readiness to principal/workspace, policy generation, exact skill and
   composition digest, contract/binding/tool/schema pins, connection identities
   and generations, recipe version and bounded task admission. Preserve ADR020's
   freshness/expiry and before/after checks. Changes cause non-readiness; cached
   ranking cannot satisfy current authority. Caller rechecks on each actual use.
6. Return exact selected schemas and necessary instructions within the budget.
   If the complete required set does not fit, return explicit non-readiness;
   never truncate a schema or omit a required tool to appear ready.

New typed evidence travels through an additive library result, not overloaded
frozen statuses or wire fields. Unsupported composition is distinguishable from
missing connections, schema drift, denied scope and unresolved selection.
Library API/type names and canonical digest encoding are frozen in the first
implementation preflight; the semantic requirements above are mandatory.

## Discovery at 10,000–100,000 entries

Separate indexed discovery from exact resolution. Index immutable skill summaries
and capability semantics; maintain exact capability/version-to-binding lookups.
Do not load all tool schemas or compute all catalog embeddings per request.
Precompute optional embeddings at ingestion; changes update only affected entries.
Exact IDs retain deterministic lookup. Lexical retrieval remains a first-class
baseline; EmbeddingGemma2 and hybrid retrieval remain unadopted candidates.

Workspace/policy filtering precedes externally visible ranking, counts and
snippets. Private indexes/caches are isolated; cache keys include scope/policy,
catalog generation, model/embedding configuration and recipe identity. Validate
current policy after retrieval and before exposure. Stale or unavailable state
fails closed. Index rebuilds publish coherent generations with rollback; readers
must not combine generation-mismatched rows and vectors. Cache data contains no
provider credentials or mailbox contents.

Bound shortlist, candidate examination, binding fan-out, connection groups and
schema/guide bytes. Any exhausted search budget produces explicit incomplete
resolution, never a false no-match or arbitrary ready selection. The library
must expose cancellation and budgets; exact selected-skill preparation bypasses
ranking. Memory/latency depends on the chosen index and workload: no numerical
performance or sublinear filtered-search guarantee is asserted here.

## Alternatives and consequences

Generic multi-tool implementation profiles are deferred until a real operation
needs an inseparable protocol, such as start/poll/fetch sharing an opaque job.
Such a future profile needs a separate versioned design and conformance cases;
it is not a reason to turn independently useful list/read operations into one
universal capability. Caller-only adapters remain compatibility boundaries, not
permanent generic catalog semantics. Atomic capabilities improve reuse but add
cross-capability affinity/dataflow obligations; this ADR makes those explicit.

## Acceptance and measurement

Use synthetic 10k and 100k catalogs, with declared proportions of skills, tools,
capabilities and bindings, provider alternatives, private scopes and skew. Freeze
hardware, index, candidate budgets, workload and acceptance thresholds before
runs. Include selected-skill lookup, ambiguous discovery, no-match, partial
coverage, denied/revoked inputs, schema drift, connection mismatch, hot updates,
index rebuild/rollback and cancellation. Measure p50/p95 preparation latency,
peak RSS, index size/build/update time, retrieval recall, complete exact coverage,
false readiness, model-visible schemas/tokens and cold/warm behavior separately.
Zero unauthorized exposure and zero false-ready results are mandatory. Record
threshold failures; do not adjust labels or thresholds after scores.

Existing composite-tool development data and Zati scripted two-tool workflow
checks retain their original claims. Author a new independently reviewed
composition cohort before scoring. No held-out access, paid/provider call,
model launch, release or default semantic adoption follows from this ADR.

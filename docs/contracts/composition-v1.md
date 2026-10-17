# Composition v1 producer contract

Status: proposed implementation freeze for T-COMPOSE.1; ADR021 governs.

## Additive API and compatibility

`catalog/composition` exposes `Parse([]byte) (Document, error)`,
`Validate(Document, map[string]Contract) error`, and
`Service.Prepare(context.Context, Request) (Result, error)`. Existing resolution,
preparation, registry v1/v2, and the legacy inbox package remain byte-compatible.
The new immutable inbox skill is `zati/inbox-briefing@2.0.0`.
Consumers must include `gist.composition/v1` in their supported versions; absence
returns `unsupported_composition` without offering tools.

The asset is `assets/composition.json`, inventoried by the normal skill manifest.
It uses `schema_version`, `skill_id`, `skill_version`, `requirements`,
`connection_groups`, `task_schema`, `steps`, and `limits`. A requirement has
`key`, `capability_id`, `contract_version`, `contract_digest` (lowercase SHA-256
of exact document bytes), and `optional`. Manifest requirements must exactly
match these identities and optional flags. Duplicate JSON keys, unknown fields,
trailing JSON, malformed pointers and cycles fail closed.

Steps have `id`, `requirement`, `after`, `inputs` and optional `foreach`.
Each input maps one top-level contract input property to a source `{step,
pointer}`; empty step means admitted task input. Foreach uses `{step,pointer,max}`
and its element is addressed with source step `$item`. Sources must be declared
predecessors; the array must be from a predecessor and max is finite. Nested
loops and fallback expressions are unsupported. Optional requirements may only
feed optional steps; unavailable optional steps and their descendants are
explicitly disabled. A required step cannot depend on an optional step.

The strict schema subset is recursive `type`, `properties`, `required`,
`additionalProperties`, `items`, `maxItems`, and `maxLength`; only object, array,
string, boolean, integer and number are supported. Objects are closed. Arrays
and strings have positive bounds. Unsupported JSON Schema constructs refuse
validation, never silently weaken checks. Assignability requires the same
type and source bounds no broader than the destination; object structures and
required fields must be compatible. No scripts, interpolation or secret source.
Limits are positive bounded `max_steps`, `max_iterations`, `max_calls`,
`max_input_bytes`, `max_output_bytes`, `max_guide_bytes`. Caller budgets tighten
the recipe; insufficient coverage returns `budget_exceeded`.

Absolute v1 ceilings: composition/contract JSON 256 KiB each; JSON nesting 32;
schema depth 16 and 4096 total nodes per contract; 64 requirements, 64 steps,
32 disjoint groups, 64 predecessors/mappings per step; 20 foreach iterations,
128 total calls; 1 MiB input and 4 MiB aggregate output; 64 KiB guide and 8 MiB
encoded preparation result. At most 64 candidates per requirement, 4096 across
the request, and 1 MiB per retrieved binding/tool document. Schema strings have
at most 1 MiB characters and arrays at most 1024 items (inbox IDs at most 20).
ZIP retrieval uses at most 2 MiB compressed/expanded total, 1 MiB per file and
64 members. These are safety ceilings, not performance qualification. Caller
budgets are positive and at most the ceiling; zero is invalid, not unlimited.

## Exact retrieval, selection and current authority

`Service` uses a small `Retriever` interface implemented by the existing
`packages.Retriever` (and usable by caller-owned in-memory adapters), a capability-indexed
`BindingIndex`, explicit `Authority`, and `Clock`. `Request` includes principal,
runtime, exact skill pin, task bytes, explicit requirement-to-binding selections,
supported composition versions and limits. No semantic search is performed.

Index queries are exact capability ID/version and contract digest, scoped to
the principal. Responses include a `Complete` bit and a bounded set of binding
pins plus caller connection attestations: scope, connection ID/version, account
ID, generation. Exhaustion returns `incomplete_budget`, not no-match. Every
candidate is retrieved through current authorization; actual immutable binding
bytes must pin the requested capability and exact one tool and pass conformance.
Existing capability documents keep `id`, `version`, `input_schema` and
`output_schema`; their effect/scope/error metadata is retained, not redefined.
Existing binding documents keep `capability_ref`, `tool_ref`, `provider_ref`,
`adapter_version`, `fixture_digest`, `conformance`, `exact_versions` and identity.
Capability/tool references have exactly one `@`, nonempty ID and version; IDs
use ASCII letters/digits plus `._/-`, versions ASCII letters/digits plus `._-`.
The reference kind is fixed by the field, workspace by the request, never by
untrusted reference text. `provider_ref` is also exact `ID@version`, parsed by
the same grammar and fixed provider kind; adapter version is a nonempty version.
Fixture digest is lowercase SHA-256, `conformance` must be `passed`, and
`exact_versions` true. Binding identity must match its index-supplied exact pin.
The trusted caller index supplies separate exact binding and tool
`ports.ArtifactPin` values, an exact provider `ports.ArtifactPin`, adapter version, admitted
fixture digest and capability document digest; all are checked against retrieved
binding bytes. The provider document is independently retrieved with current
authorization, exact ref and byte/metadata pin checks and matching id/version;
it is rechecked with the rest of the closure. The index is a caller-policy port, not an untrusted search result.
Its connection/admission state is also covered by final authority revalidation.

Every retrieved `CatalogRecord` must be in the request workspace, match the
expected ref and immutable content digest, and be published or deprecated.
`Digest` pins exact retained bytes. For skills `ManifestDigest` separately pins
manifest bytes, and `PackageDigest` pins the inventory closure, using existing
`RetrieveSkill`. Non-skill logical metadata pins use a valid `ManifestDigest`
when present; otherwise a valid `DocumentDigest` is accepted only if it hashes
the exact returned document and record metadata. This is the existing v2 alias,
not a synthesized manifest. Raw bytes must hash to `Digest`; metadata and raw
document must be equal. Capability contract_digest names those same exact
document bytes. Distinct digest roles are preserved in output evidence.
The adapter attests that the binding's existing fixture/conformance evidence is
admitted for this exact capability digest and selected tool digest. This is a
caller trust decision, not a new frozen binding field. Existing tool documents
keep their complete raw input/output JSON schemas; provider schemas are not
restricted to the composition subset. Typed mappings operate on capability
schemas, and the caller's admitted adapter owns tool argument/output conversion.
Unknown semantic fields in the composition or its schema subset are rejected;
other existing registry metadata remains compatible and retained.
Tool and capability bytes are retrieved and digest-verified before use. Provider
conformance remains caller-supplied evidence; names alone never establish it.

Connection groups intersect complete identity tuples, never enumerate Cartesian
products. Zero intersection returns `connection_mismatch`; several choices or
several alternative bindings require explicit selection. Same names with
different accounts or generations do not match. Overlapping groups are rejected
in v1, avoiding a hidden general constraint solver. Every required capability
gets its own finding. Tools deduplicate only by full connection and artifact pins.

Authority checks run before and after assembly against a SHA-256 scope of the
canonical Go JSON request plus selected pins/composition/task and runtime,
including the complete principal/scopes/policy generation and selected
connection identities/generations, binding conformance and schema digests.
The response must echo scope, policy/task generation, readiness, checked-at and
expiry. Explicit freshness and unchanged generations/pins are mandatory. Final
retrieval and index comparison detect observed revocation/schema/connection
changes; callers still recheck authority on every actual tool invocation.
No authority is inferred from package publication or catalog search.

`Result` contains status, per-requirement findings, group evidence, disabled
steps, exact full tool schemas, validated composition, and scope/generation
provenance. Statuses distinguish `ready`, `unsupported_composition`,
`requires_connection`, `requires_selection`, `connection_mismatch`, `denied`,
`stale`, `schema_drift`, `budget_exceeded`, `incomplete_budget`. Invalid bytes
and I/O errors return errors; any non-ready result omits offered tools and the
executable composition bundle. JSON-encoded
complete results must fit the caller context limit. No schema truncation.

## Inbox semantics and runtime handoff

`mail/messages/list@1.0.0` takes an admitted bounded query; returns at most 20
message IDs and `has_more`. `mail/message/read@1.0.0` takes one ID and returns
bounded sender, subject and body. These are external-read-only contracts;
mark-as-read side effects violate conformance. No attachments, links, drafts,
sends or mutations are included. One list and at most 20 reads share `mailbox`.
Capabilities include explicit error semantics and immutable conformance examples.

Preparation validates the DAG and typed mapping only. Zati owns output value
validation, iteration execution, input/output/call counters, partial results,
permissions, approval, retry and revocation at use. The API supplies bounded
schema/value validation helpers; it never executes provider tools. `has_more`,
failed reads and exhausted limits must remain visible as partial evidence.

## Verification contract

Focused commands: `go test ./catalog/composition` and
`go test ./catalog/skills/inboxcomposition`. Aggregate project verification
uses the qualified build host and existing repository recipes, including
`go test ./catalog/...` and `go vet ./catalog/...`; resource gates apply.
Tests cover unknown/duplicate keys, cycle/type/pointer failures, loops, optional
dependencies, account mismatch, alternatives, incomplete index budgets,
unsupported consumers, exact package/contract/schema drift, revocation,
freshness/task authority, cancellation, context limits and unchanged legacy
fixtures. No live provider or model evidence follows from these checks.

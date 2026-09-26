# ADR 004: Registry Governance

## Status

Accepted

## Date

2026-09-25

## Context

The registry publishes immutable skill packages, capability contracts, execution
schemas, bindings, and taxonomy metadata. Those records need an accountable
owner and a reproducible conformance decision without turning discovery into
execution authority. RFC-002 sections 7 and 11 establish the package,
trust/effects, taxonomy, and publication invariants; this ADR fixes the
operational mechanisms.

## Decision

### Ownership, review, and disputes

Every capability, effects vocabulary, skill package, binding, and taxonomy
edition has exactly one `owner_id`. `gist` owns core namespaces and their
semantics. A workspace maintainer owns records in that workspace's reserved
namespace. The registry stores the owner, reviewer, review timestamp, review
decision, and conformance evidence as append-only audit records.

The owner decides semantic compatibility and may reject a binding. A publisher
may appeal to the Gist core-maintainer review group; the appeal records the
submitted evidence, two-person decision, and reason. Core maintainers decide
core semantics and appeals. An incompatibility always receives a new contract
version or capability ID; no golden fixture is waived for a disputed adapter.

### Namespace reservation and identifiers

Identifiers are Unicode NFC-normalized, compared case-insensitively using
Unicode case folding, and restricted to the ASCII lower-case wire form
`[a-z0-9][a-z0-9._/-]*`. Confusable characters are rejected rather than
canonicalized. The `gist/` prefix is permanently reserved for the core
operator. Every other prefix is allocated to one publisher/workspace by an
audited reservation record. A publisher may publish only under its active
reservation.

Reservation allocation and retirement are serialized transactions. A prefix
is never reassigned while any version, binding, event, or audit record refers
to it. Retirement blocks new publication but preserves retrieval of existing
private versions until their normal lifecycle end; deletion of the namespace
is not a registry operation. Prefix collision or unauthorized publication
returns generic `409 namespace_conflict` and does not reveal the other owner's
identity, workspace, or records.

### Versioning and conformance

Contract versions use additive-minor semantics: adding an optional field or
fixture is a minor version; changing meaning, requiredness, validation,
effects, or an existing operation is a new major contract version or new
capability ID. Bindings pin the exact capability and contract versions.
Each binding owns a versioned golden-fixture set containing representative
inputs, transformed provider requests, provider results, normalized outputs,
and error cases. Publication requires every fixture to pass; the result,
adapter revision, and fixture-set digest are visible in the binding manifest.

The contract owner adjudicates compatibility disputes through the appeal path
above. A failed or missing fixture cannot be marked compatible. These choices
implement the ownership, namespace, versioning, and conformance rules in
RFC-002 section 11; the package validation and immutable publication sequence
remain the rules in section 7.

### Effects vocabulary

The wire vocabulary is versioned and closed. Each term is an object with a
stable `id`, `vocabulary_version`, `class`, and `sensitivity`. Wire spellings
are exactly `read_only`, `disclosure`, and `mutation` for `class`, and
`normal` and `sensitive` for `sensitivity`. A record may declare multiple
terms. Aggregate class is the most restrictive declared class in the order
`mutation > disclosure > read_only`; aggregate sensitivity is `sensitive` if
any term is sensitive, otherwise `normal`.

Unknown terms, unknown vocabulary versions, invalid spellings, and missing
class/sensitivity are rejected. New terms or optional metadata are minor
vocabulary additions. A rename or semantic change is a new vocabulary version
and requires re-review of affected contracts and bindings. Effects are never
inferred from prose, names, or provider operation strings; they are explicit
published declarations. This preserves RFC-002 section 11's controlled
vocabulary and section 7's untrusted-content boundary.

### Screening and publication state

Import validates the complete archive, manifest, inventory, references,
limits, licenses, links, and digests. The screening reviewer reads `SKILL.md`,
summaries, and referenced scripts as text and checks for prompt injection,
credential requests, exfiltration instructions, hidden network destinations,
and unsafe execution instructions. Uncertain findings are held for manual
review. Scripts, hooks, and external URLs are never executed or fetched during
import, indexing, or screening.

The recorded state machine is `draft -> screening -> approved -> published`,
with terminal `rejected` and a separate `revoked` state. Only `approved` may
become `published`; the atomic publish transaction writes the immutable
version, review decision, and indexing outbox entry together. A published
version is immutable. Deprecation stops new selection but permits authorized
retrieval; revocation blocks selection and resolution and emits the governed
event described by RFC-002 section 12. Review records are retained with the
artifact audit history.

### Trust, distribution, and taxonomy

All records are private to their workspace. Until a separately approved
publisher-signing/provenance procedure exists, the only trust value is
`operator_asserted`; `verified` is rejected and public package distribution is
disabled. A public service endpoint does not change package visibility. This is
the RFC-002 section 7 trust ceiling, not a promise of public distribution.

Taxonomy membership is optional and may contain multiple edition/node
references plus custom tags. Artifact and capability IDs remain independent of
taxonomy paths. Each imported edition stores its license and attribution
block; every taxonomy response and downstream copy carries the complete
APQC attribution block when the edition is APQC-derived. Taxonomy membership
never grants authorization or changes effects.

## Alternatives

- Let a central operator own every workspace capability. Rejected: it prevents
  accountable workspace publication and makes semantic disputes opaque.
- Reassign retired prefixes. Rejected: old immutable references would become
  ambiguous or acquire a new owner's meaning.
- Infer effects from names or instruction text. Rejected: it is not
  reproducible and cannot safely drive approval policy.
- Allow unsigned `verified` or public packages. Rejected by the trust ceiling
  in RFC-002 section 7 until signing and verification are separately defined.
- Execute scripts or fetch referenced URLs while screening. Rejected by the
  package-ingestion boundary in RFC-002 section 7.

## Consequences

Ingestion is slower because complete packages, fixtures, scripts, and review
evidence are gated before visibility. Owners must maintain fixtures and
versioned vocabulary records. In return, ownership, compatibility, effects,
attribution, and private distribution are auditable and deterministic; a
public service cannot accidentally imply public package permission.

## Verification

- A namespace test publishes a normalized collision, a confusable identifier,
  an unauthorized prefix, and a retired prefix; the latter three are rejected
  without owner disclosure and no prefix is reassigned while references exist.
- A conformance test fails publication for one changed golden result and for a
  missing fixture; an appeal creates the required immutable decision record.
- Effects tests accept only the five wire spellings above, compute the stated
  aggregation, and reject an unknown term/version or prose-only declaration.
- Screening tests prove scripts and external URLs are not executed/fetched,
  and any uncertain finding stays non-visible until approved.
- Publication tests reject public visibility and `verified`, preserve the
  operator-asserted state, preserve APQC attribution in every taxonomy copy,
  and distinguish deprecation from revocation.

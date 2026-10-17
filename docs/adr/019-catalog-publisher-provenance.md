# ADR019 — Caller-owned publisher attestations for catalog packages

Date: 2026-10-16
Status: Accepted source design; public-distribution release qualification pending.
Implements: retained T-GR-GATEWAY.31/.35 and T-PROVENANCE.0 under ADR016.

## Decision and scope

Add `catalog/provenance` as an optional Go library. A publisher signs an immutable
skill-package subject; an independently configured consumer authenticates that
publisher/key and verifies every package byte. This is separate from workload
OAuth/JWS authentication, package admission and execution. It adds no hosted
route, wire-format change, credential custody, account enrollment, network trust
discovery or automatic public publication. Existing operator-asserted manifests
and Zati private import stay unchanged. RFC002's public-distribution block remains
until T-PROVENANCE.5's separately authorized release/operations qualification.

The first supported artifact kind is `skill`. Broader artifact kinds require an
explicit compatible extension and their own byte-validation path.

## Signed identity and canonical bytes

`Subject` binds workspace, publisher, artifact kind/ID/version, schema identifier
and schema SHA-256, source repository and lowercase full Git commit ID (40 or64
hex characters), retained ZIP SHA-256, exact manifest SHA-256 and the existing
canonical complete-inventory SHA-256. Consumers supply their **independently
selected expected subject**; copying an untrusted statement into that argument
does not establish trust. This keeps schema/source/workspace/version substitution
from being accepted merely because a signature is valid. A signature proves the
publisher made an assertion; it does not prove repository ownership, safe content,
truth of provenance, license rights or schema conformance. The caller pins its
chosen schema bytes and implements schema validation/admission separately.

The canonical input is the ASCII domain
`gist.catalog.publisher-attestation/v1` followed by one NUL byte, then compact
UTF-8 JSON of the fixed `Statement` shape in this order:
`format`, `key_id`, `subject`, `issued_at`, `expires_at`.
The fixed subject order is `workspace_id`, `publisher_id`, `artifact_kind`,
`artifact_id`, `version`, `schema_id`, `schema_digest`, `source_repository`,
`source_revision`, `archive_digest`, `manifest_digest`, `package_digest`.
String encoding is Go `encoding/json` escaping (including `<`, `>`, `&`, U+2028
and U+2029), numbers are decimal whole Unix seconds, with no whitespace. There
are no maps or floating-point values. This is a version-specific canonical
encoding, **not** a general RFC8785 encoder. A golden external-consumer test locks
the ordering/domain/encoding. New fields or another canonical encoding require a
new format/domain version; silently extending this signed shape is prohibited.

All identity strings must be nonempty, valid UTF-8, at most1024 bytes, trimmed
and free of control characters. All digest pins must be lowercase64-hex SHA-256.
These fixed shapes and bounds bound reconstructed signed input. Signatures must
be exactly64 bytes; public keys exactly32 bytes. Unsupported kind/format and
invalid issue/expiry ordering fail before cryptographic verification. Raw
envelope JSON order/whitespace are irrelevant: verification reconstructs typed
canonical fields. Unknown transport fields are never authenticated metadata and
must not drive consumer policy.

## Custody and enrollment

`Sign` accepts a caller-owned `crypto.Signer` and only Ed25519. Key generation,
storage, hardware/KMS integration, signing authorization, key identifiers,
backup/recovery and publisher authentication belong to that caller. Gist stores
no private keys and supplies no default signer. Hosted workload-token signing
keys must not be reused. Real production key enrollment/custody owners are not
selected by this source change.

`TrustResolver` is a mandatory consumer-owned authoritative port, queried with
exact workspace/publisher/key identity. Its snapshot must repeat that identity,
provide the Ed25519 public key and a nonempty policy generation, key validity
start, authoritative-check time, evidence expiry and current publisher/key
revocation state. An envelope-supplied public key or unauthenticated URL does
not enroll a publisher. Transport protection, root trust, namespace entitlement,
correct generation changes and authentic freshness are resolver obligations.
Refreshing an old cached snapshot's timestamp is prohibited. Offline verification
requires genuinely fresh preauthorized evidence under the consumer's chosen
freshness bound; unknown/unavailable/stale evidence fails closed. There is no
network fallback.

## Lifecycle and point-in-time semantics

The verifier requires an explicit clock and positive maximum trust age. It
rejects future authoritative check times, expired evidence, missing validity,
future-issued or expired statements, issuance before key validity and every
publisher/key revocation. Time checks use the supplied clock without a hidden
skew allowance; callers maintain a trustworthy clock. Issuance and expiry are
whole Unix seconds, with expiry and retirement boundaries exclusive.

Key rotation uses a newly enrolled key ID/public key and a changed policy
generation. Old keys are explicitly retired or revoked; keeping a public key
implicitly active is not rotation policy. Default `RejectRetired` rejects
attestations from retired keys, including old attestations.
`AllowIssuedBeforeRetirement` is an explicit historical mode: it permits a
still-unexpired assertion issued before retirement and during the key's validity,
provided current fresh trust still authorizes that publisher/key. Retirement is
not compromise revocation. The issue time is the publisher's signed claim, not a
trusted timestamp: a stolen old key can backdate it. Therefore compromise requires
revocation, which always wins, including historical mode. Real timestamping or
transparency-log guarantees are not implemented or claimed.

The verifier checks trust/signature before bounded ZIP parsing and complete
package validation, then checks current trust/signature again. Observed generation
or public-key change during verification rejects the attempt. The second check
also catches observed revocation, evidence/attestation expiry and lifecycle
changes. This is **point-in-time** evidence at the final validation clock reading,
not atomic revocation, durable permission, or a promise that no change occurs
immediately after return. Consumers recheck current catalog admission,
authorization and trust as their use/publication policy requires. Results must
not be reused as an offline permission cache.

## Compatibility and qualification

The envelope is detached from the unchanged package manifest and ZIP. Existing
`packages.Validate`, complete inventory/asset checks and trusted-pin retrieval
remain authoritative for their existing domains. A verified envelope does not
rewrite `trust: operator_asserted` to `verified`. No frozen v1/v2 schema, transfer
contract or publisher publication adapter changes.

External public-import tests use the real maintained inbox ZIP/instructions/
template and controlled test-only keys and trust. They cover identity and digest
substitution, signature changes, full inventory validation despite valid signing,
unknown/mismatched/stale/revoked trust, rotation, explicit historical retirement,
expiry/future issue, trust changes during validation, archive bounds and cancelled
requests. Source checks establish library behavior only. Public release still
needs an independently qualified real consumer and named trust-enrollment,
custody, rotation, compromise/recovery and rollback owners under T-PROVENANCE.5.

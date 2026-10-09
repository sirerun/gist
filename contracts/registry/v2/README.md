# Publication transport v2 source contract

This directory freezes an additive source contract for PUBLISH.0. It is not a runtime implementation, provider admission decision, authenticated HTTP/store acceptance, release approval, or deployment authorization. Existing `/v1/publish/{kind}` routes, locks, read routes, and byte meanings remain unchanged. V2 is opt-in at `/v2/publish/{kind}`; release owners must decide client migration, old-route deprecation/disable timing, and readiness from release evidence. No timing is frozen here.

## Wire contract

All six POST routes accept exactly `application/json` and one UTF-8 JSON object. `skill` uses `{artifact:{package:{encoding:"base64",media_type:"application/zip",data:"..."}},max_bytes,idempotency_key}`. The other routes use `{artifact:{id,version,document},max_bytes,idempotency_key}`. The document is validated against the frozen v1 schema: capability, execution-schema (tool), provider, binding, taxonomy, respectively. The envelope and descriptor keys are fixed and case-sensitive. Every object whose keys are fixed rejects additional properties. No workspace or publisher field is accepted from the client; route kind and authenticated principal are authoritative.

Before ordinary schema decoding, implementations must validate UTF-8, reject duplicate object keys recursively, reject trailing non-whitespace JSON values, and enforce request byte limits. Strict standard base64 decoding must reject invalid alphabet/padding and non-canonical encodings; schema regex is only a lexical gate. The JSON parser must preserve the document's exact `json.RawMessage` token span from first through final token, including internal whitespace; outer transport whitespace is excluded. Validate a separate decoded copy. Never marshal/re-serialize the admitted value for immutable storage, digesting, or readback.

Descriptor identity agreement is a semantic predicate not expressible by these plain JSON Schemas: `artifact.id` and `artifact.version` must equal `document.id`/`document.version` when those fields exist. The provider typed schema has no version; provider uses descriptor version as its immutable registry version. Taxonomy requires descriptor `version` exactly equal `document.edition` (taxonomy has `id`, not `version`; descriptor `id` must equal document `id`). Skill identity/version come only from the archive's validated manifest; no duplicate outer descriptor is present. Schema acceptance alone does not establish these predicates.

## Limits and evidence

Current source evidence: `hosted/internal/rest/limits.go` defaults request body to 10 MiB and successful response to 100 MiB. `hosted/internal/packages/archive.go` defaults ZIP bytes and aggregate expanded bytes to 10 MiB, each file to 2 MiB, and file count to 256. These are current v1 implementation defaults, not configured v2 values. The existing 10 MiB body cap cannot carry a 10 MiB ZIP in base64.

Proposed v2 source defaults are separate:

| Limit | Default | Meaning |
|---|---:|---|
| Encoded JSON request | 16 MiB (16,777,216 bytes) | Full UTF-8 JSON envelope, before parsing; also hard cap for other kinds |
| Decoded ZIP | 10 MiB (10,485,760 bytes) | Strictly decoded skill archive |
| Expanded ZIP aggregate | 10 MiB | Sum of uncompressed regular-file bytes |
| Expanded file | 2 MiB | Maximum uncompressed bytes in one member |
| ZIP file count | 256 | Number of regular members; directories, symlinks, duplicate and unsafe paths reject |
| Successful response body | 100 MiB | Hard server cap; `max_bytes` may lower it, never raise it |
| Error response body | 16 KiB | Independently bounded, redacted JSON error envelope; not charged to success budget |

The encoded `data` lexical ceiling (13,981,016 characters) is the padded base64 maximum for a 10 MiB ZIP. Full request JSON remains capped at 16 MiB. Success budgets count complete body bytes only: UTF-8 bytes for JSON or raw bytes for ZIP; HTTP headers are excluded. Before mutation, compute complete success response and enforce `min(client max_bytes, 100 MiB)`. On over-budget, return a bounded `budget_exceeded` error with no partial body and no publication mutation. Error output may exceed a smaller client success budget, subject to the separate 16 KiB cap. These exact defaults require implementation configuration and tests before acceptance.

## Digests and readback

`artifact_digest` is SHA-256 over the exact transferred ZIP bytes for skills or exact raw document bytes for typed artifacts. `manifest_digest` is SHA-256 over exact `manifest.json` bytes. `package_digest` is the existing inventory closure digest from the manifest, not a ZIP hash. They are represented in HTTP headers as `sha256:` plus 64 lowercase hexadecimal characters. Readback JSON returns the exact original manifest body for skill or exact raw document body for other kinds (`application/json`); package readback returns original ZIP bytes (`application/zip`). `X-Gist-Body-Digest` hashes the exact response body; `X-Gist-Artifact-Digest` names the original transferred blob; skill additionally supplies `X-Gist-Manifest-Digest` and `X-Gist-Package-Digest`. A skill manifest body digest is therefore different from its archive artifact digest. The publish receipt carries the artifact digest and, for skills only, manifest and package digests.

## Source qualification and gaps

The repository currently has ZIP readers and manifest/package validation in `hosted/internal/packages/{archive.go,validate.go}`, immutable package digest checks in `hosted/internal/contract/package.go`, and a generic REST publication path in `hosted/internal/rest/publish.go`. The latter forwards the entire body to the publisher, applies a body limit, and checks response bytes after publication; it does not implement this v2 envelope or pre-mutation budget requirement. `hosted/internal/rest/router.go` and `router_test.go` define current v1 routes and route doubles; `hosted/acceptance/wiring/fixture_test.go` is a composition/seed fixture, not authenticated publication acceptance. Use those as implementation entry points and verification inputs, then add real authenticated HTTP plus restricted-role database/object-store acceptance per PUBLISH.0/PUBLISH.1 gates. Existing `hosted/internal/contract/http_contract_test.go` checks generic error shape only. No existing fixture qualifies v2 provider, publisher identity, ownership, storage, or runtime behavior.

Unresolved semantic implementation predicates include recursive duplicate-key and UTF-8 rejection before decode; exact RawMessage span retention; canonical strict base64; identity agreement; taxonomy edition agreement; archive safety/closure and limits; current principal policy, tenant scoping, idempotency and replay; response-before-mutation budgeting; atomic catalog/outbox behavior; staged object ownership/retention/reconciliation; and byte-exact authenticated per-kind readback. Provider capture provenance/license and all per-kind governance remain independent admission conditions. Synthetic fixtures and `validate.py` prove only grammar/schema behavior; they do not prove these predicates or actual HTTP/storage acceptance.

## Offline fixtures

`fixtures/valid.json` contains six synthetic schema-valid envelopes; `fixtures/invalid.json` contains serialized invalid transport cases. `validate.py` validates all six and exercises malformed JSON grammar, strict-key, kind, budget, identity, typed-document, package-wrapper, and response/readback-shape negatives. It resolves only files under local `contracts/registry/v1` and `v2`; no network resolution is permitted. Run from repository root:

```sh
python3 contracts/registry/v2/validate.py
```

Dependencies are installed `jsonschema` and `PyYAML`. Fixtures use illustrative IDs and a lexical base64 placeholder; the skill value is deliberately not asserted to be a valid ZIP. They do not constitute provider evidence, package acceptance, authenticated HTTP or store acceptance.

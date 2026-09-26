# ADR 006: Hosted Module and Contract Encoding

## Status

Accepted

## Date

2026-09-25

## Context

RFC-002 section 3 requires the hosted service to remain independent from the
public Go library, while sections 6 and 13 require verifiable package identity
and runtime-adapter contracts. This ADR fixes the module boundary, port
signatures, digest closure, schema conversion rules, and initial dependency
pins. It does not add files or dependencies to the root module.

## Decision

### Module boundary and package ownership

The service is a nested module at `hosted/` with its own `go.mod` and
`go.sum`. Hosted builds and tests run with `GOWORK=off` and
`GOFLAGS=-mod=readonly`; no `go.work` is introduced. The root module is
consumed read-only through an adapter and its dependency graph remains
unchanged. A sibling repository is rejected for M1 because it would split the
contract review across repositories and make the initial handoff needlessly
cross-repository; it may be reconsidered after the hosted API is stable.

Package boundaries are:

- `internal/ports`: consumer-owned interfaces and immutable value types;
- `internal/contract`: validation, canonicalization, compilation, and loss
  reporting;
- later storage/publication/discovery/identity packages: implementations of
  those ports, never alternate policy definitions;
- `cmd/registry`: composition root only.

The port signatures are fixed as follows (implementations may add private
helpers but not weaken these contracts):

```go
type CatalogStore interface {
    Get(ctx context.Context, ref ArtifactRef) (CatalogRecord, error)
    Search(ctx context.Context, q SearchQuery) (SearchPage, error)
}
type ArtifactStore interface {
    Put(ctx context.Context, digest Digest, r io.Reader, size int64) error
    Open(ctx context.Context, ref ArtifactRef) (ArtifactReader, error)
}
type Authorizer interface {
    Decide(ctx context.Context, principal Principal, action Action, ref *ArtifactRef) (Decision, error)
}
type EventStore interface {
    Append(ctx context.Context, e Event) error
    Read(ctx context.Context, cursor Cursor) (EventPage, error)
}
type ContractCompiler interface {
    Compile(ctx context.Context, input ContractInput, target RuntimeTarget) (RuntimeContract, LossReport, error)
}
```

`ArtifactRef`, `Digest`, `Principal`, `Action`, `Decision`, `Event`, and the
contract values are immutable boundary values with explicit workspace and
version fields. No port accepts a caller-supplied unvalidated workspace ID as
authority. The existing root package's local APIs, CLI, and stdio MCP server
are not imported by the hosted HTTP transport merely to reuse handlers.

### Dependency pins and review rule

The initial direct dependency allowlist is:

| Purpose | Module and exact version | Evidence/constraint |
| --- | --- | --- |
| OpenAPI/JSON Schema 2020-12 validation | `github.com/getkin/kin-openapi v0.147.0` | Its official package documentation exposes the JSON Schema 2020-12 validator option and v0.147.0 changelog; use `EnableJSONSchema2020`. |
| JOSE/JWT/JWK/JWS | `github.com/lestrrat-go/jwx/v2 v2.1.7` | Official package documentation covers JWT/JWK/JWS and verification; do not use v4 because its Go 1.26 requirement and `encoding/json/v2` migration are outside this module's compatibility target. |
| OIDC discovery and ID-token verification | `github.com/coreos/go-oidc/v3 v3.21.0` | Official package documentation marks v3.21.0 as the current tagged v3 release and the old unversioned package as deprecated. |
| PostgreSQL driver/pool | `github.com/jackc/pgx/v5 v5.11.0` | Official package documentation identifies v5.11.0, `pgxpool`, and the standard `database/sql` adapter. |

The service uses Go 1.26.1 or newer within the supported release line. The
root module's Go 1.26.1 baseline is not changed by this ADR. HTTP, TLS,
`crypto/sha256`, `crypto/subtle`, archive validation, flags, testing, and
context use the standard library. No hand-written cryptography is permitted;
JOSE primitives come from `jwx`, and OAuth/OIDC verification comes from
`go-oidc`. A dependency update requires an ADR amendment, vulnerability and
license review, API/conformance tests, and refreshed sums; floating `latest`
versions are forbidden.

The pins were checked against official package documentation on 2026-09-25:
[kin-openapi](https://pkg.go.dev/github.com/getkin/kin-openapi),
[jwx/v2](https://pkg.go.dev/github.com/lestrrat-go/jwx/v2),
[go-oidc/v3](https://pkg.go.dev/github.com/coreos/go-oidc/v3/oidc), and
[pgx/v5](https://pkg.go.dev/github.com/jackc/pgx/v5). These links are review
evidence, not runtime dependency resolution.

### Digest canonicalization and manifest envelope

The package contains `manifest.json`, `SKILL.md`, and declared regular payload
files. The manifest inventory contains unique NFC-normalized relative paths,
byte sizes, SHA-256 digests, and media types. Paths reject empty segments,
`.`/`..`, absolute paths, encoded separators, symlinks, hard links, and
archive members absent from the inventory. The manifest does not inventory or
hash itself.

`package_digest` is the lowercase SHA-256 of the UTF-8 bytes produced by
RFC 8785 JSON Canonicalization Scheme over the inventory array sorted by
UTF-16 code-unit path order. Inventory records use only strings, non-negative
integer sizes, and arrays/objects, so any non-I-JSON number is rejected. The
implementation uses the Go standard library's `encoding/json/jsontext`
canonicalization support available in the selected Go line; it does not use
ordinary `encoding/json.Marshal` as a substitute. The canonicalizer is tested
against RFC 8785 vectors and an independently implemented small verifier.

The manifest's own byte-exact SHA-256 and size live in the version record's
detached transfer inventory, outside `manifest.json`. Retrieval verifies the
manifest bytes first, then all payload bytes, then the package closure, and
finally the pinned `(id, version, manifest_version)`. No `latest` alias is
accepted in a resolution. A digest identifies bytes; it does not assert
publisher trust.

The frozen known vector is the canonical inventory JSON
`[{"media_type":"text/plain","path":"SKILL.md","sha256":"00","size":0}]`;
its SHA-256 is
`49911e4d4399ed15e34c0b9a5ebe8ea3bb8c4c3f110671f7abd940c5b2d74485`.
The independent verifier must hash exactly those bytes and compare this result
with the implementation before C3 freezes the fixture. A real fixture uses
64-hex SHA-256 values; the `00` value above is deliberately a rejected
validation example, not a publishable digest.

### Runtime contract compilation

Every compiled runtime contract must source and preserve: capability ID and
exact contract version; tool ID/version and schema dialect; input/output
schemas; effects vocabulary version, classes, and sensitivity; outbound
destinations; credential metadata (never secrets); required scopes; cost
currency/unit and upper bound or an explicit unknown/unbounded result;
timeout; retry/idempotency and async-status semantics; provider action/version;
lifecycle; package/binding digests; provenance and capture time; and
`execution_location` (`client`, `runtime`, or `gist_gateway`).

The source mapping is fixed: capability contract supplies operation identity,
input/output, effects, scopes, cost, timeout, retry/idempotency, and async
semantics; provider binding supplies provider action/version, destination,
credential metadata, capture time, and conformance result; the skill manifest
supplies package/binding references, runtime requirements, and provenance; the
selected adapter supplies the target tool identity and `execution_location`.
Missing security, effects, destination, credential, or retry information is a
hard compile error. Unknown required fields never compile permissively.

### Schema conversion and loss policy

Input schemas are validated as JSON Schema 2020-12. Conversion to a consumer
schema is lossless-only for security and execution semantics. Loss of
`additionalProperties`, `enum`, numeric/string/array bounds, `required`,
formats used by validation, effects, destinations, credential requirements,
timeouts, retries, idempotency, or async status rejects normalization and
marks the provider/runtime adapter degraded. Documentation-only descriptions
or examples may be dropped only when the loss report records the source path,
target, reason, and contract version. A runtime marked degraded cannot produce
`ready`.

### Testing and release

Tests run with `GOWORK=off`, `GOFLAGS=-mod=readonly`, race-free deterministic
unit vectors, and integration tests for real PostgreSQL/object storage at the
quality gate. Required vectors cover archive traversal, detached manifest
verification, RFC 8785 bytes, missing runtime fields, each security-sensitive
schema keyword, every loss category, JOSE algorithm/key allowlists, and root
module graph equality before/after adding `hosted/`.

## Alternatives

- Add hosted dependencies to the root module. Rejected by RFC-002 section 3;
  local users must not acquire server, OAuth, or tenancy requirements.
- Put hosted code in a sibling repository. Rejected for initial delivery due
  to contract and fixture churn across repositories.
- Hash ordinary JSON or a compressed archive. Rejected because byte identity
  would depend on map order, archive metadata, compression, or the manifest's
  own self-reference.
- Permit lossy schema conversion with warnings. Rejected because a lost
  required field, bound, enum, or effect can widen execution authority.
- Hand-write signature/JWT cryptography. Rejected in favor of reviewed JOSE
  primitives.

## Consequences

The repository carries a second module and must maintain explicit ports,
dual-module CI, pinned dependency reviews, and canonicalization fixtures.
Package identity is stable across archive tools, local copies can be verified
without trust assumptions, runtime consumers receive an auditable loss report,
and the existing root module remains unchanged.

## Verification

- `test -s docs/adr/006-hosted-module.md` and `git diff --check` pass without
  any root-module change.
- An independent program canonicalizes the frozen inventory vector and agrees
  byte-for-byte with the implementation; a self-inventory attempt is rejected.
- Root and hosted dependency graphs are compared; root `go.mod`/`go.sum` and
  public package APIs are unchanged.
- Package tests reject traversal, links, undeclared members, digest mismatch,
  and `latest`; they verify the detached manifest before payload closure.
- Compiler tests reject every listed security-semantic loss and every missing
  required runtime-contract source; only recorded documentation loss is
  accepted.
- Dependency review confirms the four exact versions and their official
  documentation links before C2 writes `hosted/go.mod`.

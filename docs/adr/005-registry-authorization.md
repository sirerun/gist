# ADR 005: Registry Authorization

## Status

Accepted

## Date

2026-09-25

## Context

REST, remote MCP, search, caches, object storage, cursors, and runtime
resolutions must apply one tenant boundary. RFC-002 section 12 defines the
required properties and section 9 defines the registry operations. This ADR
selects the mechanism and the observable oracle policy; it does not grant
execution authority to discovery or retrieval.

## Decision

### Identity and workspace selection

The authorization server issues a signed access token after an authenticated
membership check. A token contains `iss`, `sub`, `aud`, `workspace_id`,
`scopes`, `policy_generation`, `iat`, `exp`, and `jti`. The service accepts
only an allow-listed issuer, exact audience, approved algorithm/key `kid`,
valid time claims, and a workspace membership that is current for the token's
policy generation. A principal with multiple workspaces selects one at token
mint time; switching requires a new token. Request-body, query, path, and
arbitrary header workspace IDs are never trust inputs.

Workload subjects and human subjects are distinct subject types. Every policy
decision binds issuer, subject, audience, workspace, scopes, and policy
generation. A token has a five-minute maximum lifetime; clock skew is at most
30 seconds. Unknown issuer keys or unavailable required issuer/revocation
checks fail closed. A previously validated key may be used only within a
30-second freshness window and does not replace a live membership/policy
check.

### Roles, scopes, and visibility

`catalog:read` is required for discovery, retrieval, taxonomy, events, and
resolution. `catalog:publish` is required for publication, and a maintainer
role for the selected workspace is required in addition to that scope. A
publish scope alone never authorizes publication. A reader can read only
authorized workspace-private records; a maintainer can publish for that
workspace. Public visibility is a reserved schema value but is rejected while
ADR 004's private-only trust ceiling is in force. Connection assertions are
resource facts and never widen scopes or workspace membership.

### Isolation mechanism

Every tenant-owned PostgreSQL table carries `workspace_id`. The application
uses separate migration, service, and read roles. The service role enables
forced row-level security; every transaction sets local principal, workspace,
scope, and policy-generation settings before its first query. Pool checkout
and reset tests prove that settings cannot bleed between requests. Object
storage keys are opaque and are readable only through an authorized service
route; no client receives a direct bucket capability.

RLS-owner bypass, a missing `FORCE ROW LEVEL SECURITY`, or an unscoped
vocabulary/index query is treated as a critical isolation defect: deployment
is rejected, not degraded. The adversarial matrix uses two principals in one
workspace and one principal in two workspaces and checks IDs, cursors, counts,
results, snippets, downloads, cache hits, conditional responses, and timing.

### Route-by-route policy

The service authenticates first, validates scope/role second, and only then
loads the requested resource or evaluates ETag, conditional, size, or expiry
details. Invalid or absent authentication is `401 unauthorized`. A known
authorized resource/action without the needed scope or role is `403
forbidden`. A missing or unauthorized private resource is uniform `404
not_found`, with the same safe body shape and no account/workspace detail.
Lists omit inaccessible entries. A namespace collision is the sole generic
`409 namespace_conflict` exception where ownership remains undisclosed.

| Operation | Required policy | Success | Private-denial oracle |
| --- | --- | --- | --- |
| `POST /v1/discover` | `catalog:read`; filter before rank/limit | 200 | empty authorized result; no foreign counts/snippets |
| `GET /v1/skills/{id}/versions` | `catalog:read`; keyset-paged by `limit` and `cursor` | 200 | 404 for foreign/missing skill |
| `GET /v1/skills/{id}/versions/{version}` | `catalog:read` | 200 | 404 |
| `GET /v1/skills/{id}/versions/{version}/package` | `catalog:read` | 200 bytes | 404; no ETag/size leak |
| `GET /v1/tools/{id}/versions/{version}` | `catalog:read` | 200 | 404 |
| `GET /v1/capabilities/{id}/versions/{version}` | `catalog:read` | 200 | 404 |
| `POST /v1/resolve` | `catalog:read` for every pinned reference | 200 | 404 for inaccessible pins; no partial ready |
| `POST /v1/connections` | `catalog:read` plus authorized capability | 201 | 404 for foreign capability |
| `GET /v1/connections/{id}` | `catalog:read` and same principal/workspace | 200 | uniform 404 |
| `GET /v1/taxonomies` | `catalog:read` | 200 | only authorized editions |
| `GET /v1/taxonomies/{id}/nodes` | `catalog:read` and edition access | 200 | 404 for foreign edition |
| `POST /v1/publish/skills` | `catalog:publish` + maintainer | 201 | 403 for known action; generic conflict for prefix |
| `POST /v1/publish/capabilities` | `catalog:publish` + maintainer + owned prefix | 201 | 403/409 without owner disclosure |
| `POST /v1/publish/tools` | `catalog:publish` + maintainer | 201 | 403 |
| `POST /v1/publish/providers` | `catalog:publish` + maintainer | 201 | 403 |
| `POST /v1/publish/bindings` | `catalog:publish` + maintainer + owned records | 201 | 403/404 as applicable |
| `POST /v1/publish/taxonomies` | `catalog:publish` + maintainer | 201 | 403 |
| `POST /v1/publish/revocations` | `catalog:publish` + owner/maintainer | 201 | 403/404 |
| `POST /v1/identities/revoke` | `identity:revoke` + maintainer; workspace from the principal only | 200, repeatable | uniform 404 for missing or foreign identity |
| `GET /v1/events` | `catalog:read` and cursor binding | 200 | 404 for foreign cursor |
| `POST /v1/artifacts/batch-get` | `catalog:read` per item | 200 | complete per-item 404, no foreign metadata |

Publication and connection creation return 201. Other successful operations
return 200. Error envelopes contain only `code`, safe `message`,
`request_id`, and `retryable`; `429` and retryable `503` include
`Retry-After`. The catalog error enum is: `unauthorized` (401), `forbidden`
(403), `not_found` (404), `version_conflict` (409), `artifact_revoked` (409),
`cursor_expired` (409), `resolution_expired` (409), `budget_exceeded` (413),
`validation_failed` (422), `integrity_error` (422), `rate_limited` (429), and
`service_unavailable` (503). `payload_too_large` is not a code; an enforced
serialized UTF-8 budget emits `budget_exceeded`. OAuth protocol errors remain
OAuth responses, not catalog errors.

### Cursors, resolutions, and budgets

Cursors are 128-bit-plus random opaque URL-safe IDs pointing to server rows.
Rows bind principal, workspace, query/filter hash, sort version, policy
generation, and creation time. Cursor TTL is five minutes. Sorting is by
stable normalized rank, then immutable artifact ID; a snapshot query prevents
page reordering. Cursors cannot be aliases or caller-created offsets.

Resolutions are server rows binding the same identity/policy context to exact
skill, capability, tool, and binding versions. The resolution lease is at
most 60 seconds and is rechecked at dispatch. Expiry returns
`resolution_expired` only after authorization. Every required capability gets
a finding; lookup failure never creates a successful or fabricated finding.

`max_bytes` is an enforced serialized-response limit. The service may omit
optional descriptive fields or reduce candidate count, but never truncates a
required file or executable schema. The documented `token_estimate` uses
`ceil(UTF-8 bytes / 4)` and is explicitly approximate. If complete required
content cannot fit, return `413 budget_exceeded`; batch responses contain
complete items and per-item errors, and an aggregate over-budget response
fails as a whole.

### Failure matrix

| Failure | Required behavior |
| --- | --- |
| Invalid/missing bearer | 401; no resource lookup |
| Uncheckable membership, policy store, or required issuer/revocation check | 503 `service_unavailable`; fail closed |
| Search/index unavailable | 503 for discovery; never return a fabricated resolution |
| Policy available, artifact store available, index unavailable | Pinned retrieval may continue after normal policy check |
| Artifact store unavailable | 503 for package/manifest retrieval |
| Corrupt digest or mismatched transfer inventory | 422 `integrity_error`; never serve bytes |
| Revoked pinned record | 409 `artifact_revoked` after authorization |
| Expired cursor/lease | 409 with `cursor_expired`/`resolution_expired` after authorization |

### Revocation, events, and recovery

The transactional outbox emits ordered events with `event_id`, `type`, exact
typed artifact reference, timestamp, and policy generation. Types are exactly
`version_published` and `version_revoked`. `GET /v1/events` is workspace and
principal authorized, cursor-bound, resumable, and duplicate-tolerant. Events
are retained for seven days. A retention gap returns `cursor_expired` plus an
authorized resync instruction; the consumer stops dispatch, refreshes policy
and versions, and re-resolves. Runtimes recheck policy at dispatch. Revocation
is advisory for client-owned execution because Gist is not in that execution
loop; downloaded local copies cannot be erased.

### Cache and index discipline

Every cache and index-query key contains issuer, subject, workspace, scopes,
policy generation, query/filter hash, and schema/index version. Every read,
including a cache hit and an ETag/304 candidate, performs the authorization
check first. Private responses are `Cache-Control: private, no-store` by
default. No shared public cache stores private artifacts. Index lag cannot
reveal revoked metadata: authoritative policy filtering is applied after index
selection and before counts, snippets, or results.

## Alternatives

- Trust a caller-supplied workspace header or body field. Rejected because it
  makes tenant selection an ambient input and enables cross-workspace reads.
- Use a signed workspace-selection grant. Rejected for the first release;
  mint-time workspace claims make the authorization input smaller and easier
  to bind across REST, MCP, caches, and events.
- Use query-only tenant filters. Rejected because a missed predicate has a
  broad data-leak blast radius; forced RLS is the backstop.
- Return 403 for foreign private IDs. Rejected because it creates an existence
  oracle; private foreign and nonexistent references are both 404.
- Keep long-lived cursors or resolutions. Rejected because policy and
  revocation changes would outlive their authorization context.

## Consequences

The service pays for online policy checks, forced-RLS setup, short leases,
transactional outbox delivery, and adversarial timing tests. Clients must
handle resync and re-resolution, and local copies remain a disclosed limit.
The same rules apply to both transports and prevent authorization failures in
search, caches, conditional reads, and event recovery from bypassing storage
policy.

## Verification

- Token vectors cover wrong issuer, audience, signature algorithm/key, expiry,
  workspace claim, scope, policy generation, and a two-workspace principal;
  all fail closed as specified.
- Route tests exercise all 21 operations with valid, missing, foreign, and
  nonexistent references. Foreign and nonexistent private references have
  identical status, safe body shape, headers, and bounded timing.
- PostgreSQL tests prove forced RLS, role separation, transaction-local
  context reset, and no cross-tenant IDs/counts/results/snippets/downloads.
- Cursor/resolution vectors prove random opacity, binding, five-minute/60-
  second limits, stable sorting, exact pinning, expiry, and no aliasing.
- Failure tests cover policy/issuer/index/artifact outages, integrity errors,
  revocation, retention gaps, duplicate events, cache hits, ETag/304, and
  `max_bytes`; no test accepts a fabricated `ready` result.

## Amendments

### 2026-09-26: identity revocation route

Approved by David. Nothing in the v1 contract could revoke a workload
identity, so a compromised or retired workload kept its access until its
token expired. The contract adds `POST /v1/identities/revoke`, which is not
the same as `POST /v1/publish/revocations` (artifact revocation notices).

- The body is exactly `{issuer, subject}`. Unknown fields, including any
  workspace field, fail with `422 validation_failed`. The workspace comes only
  from the verified token, so a caller can revoke identities only in its own
  workspace.
- The route needs the new `identity:revoke` scope plus the `maintainer` role,
  the same scope-plus-role shape as publication. Scope and role are checked
  before the identity is looked up.
- Success is `200` with `{issuer, subject, revoked: true}`. Revoking an
  identity that is already revoked also returns `200` and keeps the original
  revocation time. A missing identity and one in another workspace both
  return a uniform `404 not_found`.
- A revoked identity fails authentication on its next request, because
  every request re-checks the stored identity row.

The v1 lock and the gate hashes were refreshed for this amendment.

### 2026-09-26: version paging, artifact revocation, and permanent identity revocation

Three follow-up fixes changed or clarified v1 behavior.

- `GET /v1/skills/{id}/versions` gains optional `limit` and `cursor` query
  parameters. Before this, every version came back in one response with an
  empty `next_cursor`, so an artifact with many versions exceeded
  `max_bytes` and could not be listed. Versions are now keyset-paged in
  SemVer 2.0.0 precedence order (`1.9.0` before `1.10.0`, a pre-release
  below its release). `limit` defaults to, and is capped at, the server
  result cap. `next_cursor` is opaque and is empty on the last page. A
  malformed cursor or a `limit` below 1 fails with `422 validation_failed`.
  Postgres stores the order as a generated `version_key` column (migration
  006).
- `POST /v1/publish/revocations` used to run the package publisher and
  insert an ordinary catalog version. It now does what this ADR and ADR 004
  specify: the publish body's `artifact` is exactly `{kind, id, version}`
  plus an optional `reason`, naming an existing version in the caller's own
  workspace. That version moves to the terminal `revoked` state with its
  revocation time, and one `version_revoked` event is emitted. No catalog
  version is created. A repeat returns the original notice, and a missing or
  foreign target is a uniform `404`. After revocation, exact reads, package
  download, batch-get items, and resolution of that version answer
  `409 artifact_revoked` after authorization, and version lists and search
  omit it.
- Identity revocation (`POST /v1/identities/revoke`) is permanent for its
  workspace, issuer, and subject. A revoked subject can never be minted
  again in that workspace, even after its tokens expire. This is the safe
  default, and there is no reinstatement API. A workload that needs access
  again gets a new subject.

The v1 lock and the gate hashes were refreshed for this amendment.

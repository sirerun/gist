# ADR 010: Execution Gateway M-G1 Technical Design

## Status

Technical design under revision; the founder approved the revised M-G1 direction
for planning and implementation on 2026-10-11. Exact-head review and G0.4
recording remain open.

## Date

2026-09-27 (amended 2026-10-11)

## Context

ADR 009 fixes the M-G1 direction the founder approved on 2026-10-11:
caller-owned approval without a Gist approval page or JWS, KMS-encrypted
provider credentials, a separate OpenAPI-generated gateway MCP surface,
dynamic task guidance, GitHub REST plus a controlled MCP fixture, and a
separate gateway contract under `contracts/gateway/v1`. The exact credential
intake mode remains unresolved. RFC-003 §3 leaves the
exact enum values, state machine, caller authorization boundary, and idempotency
binding to "the M-G1 ADR". The build lanes (Wave 1 onward of
`docs/plans/gateway-buildout.md`) need those fixed so parallel agents do not
invent divergent schemas. This ADR is that fix. RFC-003 §3's scope boundary
still applies: this vocabulary is shared with consuming runtimes'
protected-effects engines; an execution record from either producer carries
the same evidence shape.

## Decision

### 1. Evidence enums

Outcome confirmation, exactly five values, RFC 2119-free:

- `success` — the provider confirmed the effect completed.
- `failure` — the provider confirmed the effect did not complete, with a
  reported failure.
- `accepted_unconfirmed` — the provider accepted the request but has not
  confirmed completion (accepted-but-unconfirmed).
- `nonexecution` — the effect was never sent: denied, aborted before send,
  or caller permissions missing.
- `unknown` — Gist cannot determine whether the request reached the provider
  (for example a connection drop at send time).

Request-sent, exactly three values: `no`, `yes`, `unknown`.

Invariant: a record with `outcome = nonexecution` carries `request_sent = no`;
`success`/`failure`/`accepted_unconfirmed` carry `request_sent = yes`;
`unknown` may pair with `request_sent = yes` or `unknown` but never `no`.
`unknown_outcome` in RFC-003 §3 is the human phrase for
`outcome = unknown`. `unknown` never auto-retries a mutating effect (§3).
Example: `{"outcome_confirmation": "accepted_unconfirmed", "request_sent":
"yes"}` — provider 202'd a webhook creation, no completion callback.

### 2. Execution state machine

States: `pending` → `executing` → terminal. There is no `awaiting_approval`
state because the caller handles user consent.

| From | Event | To |
| --- | --- | --- |
| `pending` | schema and current caller/connection grants verified | `executing` |
| `pending` | invalid input, policy denial, or revocation | `rejected` |
| `pending` | cancelled before dispatch | `cancelled` |
| `executing` | provider confirms outcome | `succeeded` or `failed` |
| `executing` | send/outcome indeterminate | `unknown_outcome` |
| `executing` | cancellation before any send | `cancelled` |

`rejected`/`cancelled` have `request_sent = no`; an already sent request cannot
be represented as pre-send cancellation. `unknown_outcome` never retries a
mutating effect automatically. No execution succeeds before dispatch.
Example: `{"id":"ex_01J...","state":"rejected","outcome_confirmation":
"nonexecution","request_sent":"no"}` for a connection-grant denial.

### 3. Caller authorization boundary

The verified identity is the caller; consent is the caller's responsibility.
No approval-evidence field, JWS verifier, authority registration, hosted page,
or approval-consumption table is part of M-G1. An `approved` field cannot widen
permissions. Before dispatch, derive tenant/principal from the verified token,
validate its audience and `execution:invoke`, check current policy and
revocation, and verify the exact capability/binding/connection grant. Another
workspace or principal's connection reference cannot be substituted.

The gateway accepts a compromised caller's actions within its grants as a
residual risk; restrictive grants, revocation, bounded egress, and execution
records mitigate impact without claiming to prove human consent.

### 4. Idempotency binding

The idempotency key is supplied by the client; the server computes a
versioned, domain-separated binding fingerprint. Version 1 hashes these bytes
with SHA-256, in this exact order:

1. The UTF-8 bytes `gist.gateway.idempotency-binding.v1` followed by one NUL byte.
2. `tenant_id`, `principal_hash`, `binding_ref`, and `connection_id`, each
   encoded as a four-byte unsigned big-endian byte length followed by the exact
   canonical UTF-8 bytes stored for that value. Values longer than `2^32-1`
   bytes are invalid; no implicit normalization or delimiter splitting occurs.
3. The 32 raw bytes of `SHA-256(payload)` (fixed width, not hexadecimal text).

`principal_hash` is the existing deterministic `ports.PrincipalHash` value
(issuer, subject, audience, workspace), encoded as its canonical UTF-8 form.
The domain separator and length prefixes make field boundaries and algorithm
version unambiguous across implementations. Implementations must share golden
vectors and prove that ambiguous unframed pairs such as `("a", "bc")` and
`("ab", "c")` produce distinct fingerprints. This is the first persisted
format; any later format uses a new domain/version and an explicit migration.

The tuple is stored in `execution_idempotency` with the execution ID. Reuse
with an identical binding returns the original execution (`200`, not a new
run); reuse with a changed binding is `409 idempotency_conflict` — never a
silent re-run. Keys expire with their execution record's retention, not on a
fixed clock.

### 5. Scope and contracts layout

The gateway OpenAPI specification is authoritative. Generate the selected MCP
operations, including `gist_invoke`, from stable OpenAPI operation IDs. REST
and MCP must share input validation, permission checks, result/error semantics,
and execution idempotency. Generation uses an explicit exposure allowlist;
admin/internal operations are not exposed implicitly. Registry v1's four tools
remain compatible. Guide retrieval requires catalog authorization; execution
submission and principal-bound status reads require `execution:invoke`.

`contracts/gateway/v1/` contains:

- `openapi.yaml`: `POST /v1/executions`, `GET /v1/executions/{id}`, and proposed
  `POST /v1/tool-guides`; no approval routes. The guide response carries intent,
  exact capability/tool/binding/recipe versions, authorized connection references,
  readiness findings, expiry, and grounded instructions, never secret values.
- `route-matrix.json`: method/path/auth/scope and explicit MCP exposure mapping.
- `lock.json`: contract hashes checked by `check.py gateway --freeze-check`.
- `fixtures/`: authorization denials, execution outcomes, guide provenance,
  unsupported requirements, and REST/MCP parity cases.

Derived guides do not grant execution permission. Sources must be validated,
with deterministic assembly from maintained recipes; model-assisted selection
is checked against authorized contracts. Cache isolation includes caller and
workspace; execution revalidates even if a guide or resolution is cached.

Registry v1 is unchanged; `check.py contracts` keeps rejecting `/execut`
routes, and the new `gateway` subcommand owns the new routes.

### 6. Event kinds

Two new kinds join the ADR 005 feed so revocation propagates to bindings,
resolutions and pending executions:

- `connection_revoked` — a connected account's credentials were revoked;
  consumers must re-verify connections before executing.
- `binding_revoked` — a binding was revoked; consumers must stop resolving
  and the gateway must deny executions bound to it.

Payload shape and cursor semantics follow the existing ADR 005 feed;
duplicates are possible and documented there.

### 7. Provider adapter interface

Two adapters in M-G1 (ADR 009 decision 7). The interface both implement:

```go
type ProviderAdapter interface {
    // Class returns the route class, e.g. "direct_api" or "mcp".
    Class() string
    // Prepare validates the resolved binding, connection and payload
    // against per-binding limits; it must not perform egress.
    Prepare(ctx context.Context, req PrepareRequest) (Attempt, error)
    // Send performs exactly one egress attempt and reports the evidence
    // enums of §1; it never retries internally and never fabricates an
    // outcome. Send returns the raw provider response bytes bounded by
    // the per-binding limits.
    Send(ctx context.Context, a Attempt) (AttemptResult, error)
}
```

`AttemptResult` carries exactly `OutcomeConfirmation`, `RequestSent`, the
provider response (bounded), and timing. The adapter never names another
provider or account: failover is a new disclosed execution (RFC-003 §6).
Egress for the direct-API adapter goes through the §5 egress policy
(HTTPS only, private/loopback/link-local/metadata blocked, per-hop
redirect re-validation); the remote-MCP fixture adapter is bound to the
fixture server's allowlisted origin only.

## Open credential-intake decision

KMS envelope encryption is selected, but the founder has not selected
stored-only provider connections versus one-shot caller-supplied credentials.
The gateway contract must not expose credential values or a credential-ingress
route until the choice is recorded. After selection, implementation must bind
credential access to workspace, principal, provider, and capability scope; redact
all traces; and prove rotation, revocation, and unknown-outcome behavior.

## Consequences

- The evidence enums and state machine must be mirrored in
  `contracts/gateway/v1` fixtures and enforced in tests (G4.1); drift between
  this ADR and the contract is a review defect.
- Generated MCP tools require exposure, schema, scope, and result parity checks;
  exact tool counts follow the explicit contract rather than handwritten lists.
- Caller compromise within granted authority is an accepted boundary requiring
  founder review; the private model no longer assumes per-action evidence.
- Dynamic guides require provenance, isolation, freshness, and correctness tests.
- Billing meter unit and publisher provenance remain M-G2 decisions and are
  not fixed here.
# ADR 010: Execution Gateway M-G1 Technical Design

## Status

Accepted (pending founder sign-off, G0.4)

## Date

2026-09-27

## Context

ADR 009 fixed the M-G1 decisions David made on 2026-09-27: review gate first,
two approval sources, KMS envelope custody, `contracts/gateway/v1` separation,
two proving adapters, and the registry-aws deployment. RFC-003 §3 leaves the
exact enum values, state machine, approval evidence format, and idempotency
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
  or approval missing.
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

States: `pending` → (`awaiting_approval` | `executing`) → `terminal`.

- `pending` — request validated, re-verification done, no attempt sent.
- `awaiting_approval` — sensitive effects awaiting approval evidence; the
  only state from which the Gist-hosted approval step is offered.
- `executing` — an attempt has been dispatched or is about to be.
- Terminal: `succeeded`, `failed`, `unknown_outcome`, `rejected`,
  `cancelled`.

Transitions and their guards:

| From | Event | To |
| --- | --- | --- |
| `pending` | approval evidence verified (or effects not sensitive) | `executing` |
| `pending` | sensitive effects, no evidence yet | `awaiting_approval` |
| `awaiting_approval` | evidence verified | `executing` |
| `awaiting_approval` | denial, expiry, or revocation | `rejected` or `cancelled` (terminal) |
| `executing` | provider result confirmed | `succeeded` or `failed` |
| `executing` | send indeterminate | `unknown_outcome` |
| `executing` | Gist cancels before send | `cancelled` (`request_sent = no`) |

`rejected` and `cancelled` are pre-send terminals (`nonexecution`); they are
distinct so a policy denial is auditable apart from an operator cancel.
`pending`→`awaiting_approval`→`executing` is the only approval path; no state
may transition to `succeeded` without having been `executing`.

Example: `{"id": "ex_01J...", "state": "awaiting_approval", "approval":
{"href": "https://registry.sire.run/v1/approvals/apr_01J..."}}`.

### 3. Approval evidence format

One format, two sources (ADR 009 decision 3). Approval evidence is a JWS
(compact serialization, detached payload not used — full JWT envelope),
signed with Ed25519 (EdDSA, `alg: EdDSA`), by either a registered external
authority key or the Gist-hosted step (which signs with the same
workload-issuer key class after the human decision).

Claims:

- `iss` — the authority identifier (registered authority ID, or the
  gateway's own issuer for the hosted step).
- `aud` — `registry.sire.run` (the resource audience; RFC 8707 alignment).
- `sub` — the approving principal's subject.
- `workspace_id` — string, must equal the execution's workspace.
- `execution_binding` — the lowercase hex SHA-256 of the canonical execution
  binding (see §5), so the approval is bound to the exact request.
- `jti` — unique approval identifier; the single-use token.
- `iat`, `exp` — `exp` at most 15 minutes after `iat` for the hosted step,
  at most 1 hour for external authorities.
- `decision` — `approve` or `deny`.

Single-use: on first successful verification the gateway records the `jti`
in `approvals_consumed` in the same transaction that dispatches the attempt;
a replay is rejected with `409 approval_already_consumed`, even for the same
execution. Key registration, rotation and revocation live on
`POST|GET|DELETE /v1/approval-authorities`; a revoked authority key rejects
all evidence, including evidence already presented for pending executions.

Example (decoded claims):
`{"iss":"sire","sub":"u_01J...","aud":"https://registry.sire.run",
"workspace_id":"ws_01J...","execution_binding":"9f2a...","jti":"apr_01J...",
"iat":1735...,"exp":1735...,"decision":"approve"}`.

### 4. Idempotency binding

The idempotency key is supplied by the client; the binding is computed
server-side as `SHA-256(tenant_id || principal_hash || binding_ref ||
connection_id || SHA-256(payload))` where `principal_hash` is
`ports.PrincipalHash` (issuer, subject, audience, workspace). The tuple is
stored in `execution_idempotency` with the execution ID. Reuse with an
identical binding returns the original execution (`200`, not a new run);
reuse with a changed binding is `409 idempotency_conflict` — never a silent
re-run. Keys expire with their execution record's retention, not on a fixed
clock.

### 5. Scope and contracts layout

`gist_invoke` is the fifth MCP tool, offered only to principals holding
`execution:invoke` (the tool call is denied `403` without it). The REST
surface and the tool accept the same request shape.

`contracts/gateway/v1/` layout, mirroring registry v1:

- `openapi.yaml` — routes: `POST /v1/executions`,
  `GET /v1/executions/{id}`, `POST /v1/approvals/{id}/decision` (page form
  target), `GET /v1/approvals/{id}`, `POST|GET|DELETE
  /v1/approval-authorities`. Error envelope identical to registry v1.
- `route-matrix.json` — method/path/auth/scope matrix.
- `lock.json` — SHA-256 of every contract file, re-frozen only through the
  gate (`check.py gateway --freeze-check`).
- `fixtures/` — golden request/response fixtures, including the approval
  evidence and enum examples above.

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

## Consequences

- The evidence enums and state machine must be mirrored in
  `contracts/gateway/v1` fixtures and enforced in tests (G4.1); drift between
  this ADR and the contract is a review defect.
- Registering `gist_invoke` amends the pinned four-tool MCP surface, its
  client support record, and the acceptance test that asserts the tool count.
- A compromised authority key can approve within its workspace until
  revoked; the threat model (private, foundation) must cover registration,
  rotation, revocation and single-use replay under that compromise.
- The hosted approval step gives every runtime-less client a path, at the
  cost of human friction on sensitive effects, which is intended.
- Billing meter unit and publisher provenance remain M-G2 decisions and are
  not fixed here.
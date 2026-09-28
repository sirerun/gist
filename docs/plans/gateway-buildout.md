# Gist execution gateway buildout: RFC-003 M-G1

Date: 2026 09 27 (UTC). Status: planned; no gateway code exists.
Work types: engineering, with architecture review and security operations.

## Context

### Problem

The RFC-002 registry is live code. It discovers, retrieves and resolves
capabilities but never executes them. Some clients cannot execute a resolved
capability themselves: the resolution reports `requires_gateway`. Claude,
ChatGPT or Codex connected over MCP, and customer scripts, are the main
examples. [RFC-003](../rfc-003.md) specifies the service that executes for
them: a governed, approval-gated, ledgered gateway with credential custody and
egress controls.

RFC-003 is a draft. It has not been reviewed as its own document, and its
section 7 threat model must be signed off before any executable provider goes
live. This plan starts with that review.

### Decisions already made

[ADR 009](../adr/009-gateway-mg1-foundations.md) records the founder's
2026-09-27 decisions. In short:

- Review, threat model and technical ADR first, signed off by David; build
  lanes start only after that.
- M-G1 is planned to executable depth. M-G2 and Sire authority wiring are
  outline epics.
- Approvals use one evidence format from two sources: registered external
  approval authorities (signed, single-use, bound to the execution), and a
  Gist-hosted step where a logged-in human approves. Gist never approves on its
  own authority.
- The gateway ships in the same registry service on the `registry-aws` stack
  ([ADR 008](../adr/008-registry-aws-hosting.md)).
- Credentials use KMS envelope encryption in Postgres.
- Gateway routes get their own contract, `contracts/gateway/v1`. Registry v1
  stays frozen. `gist_invoke` is a fifth MCP tool.
- First providers: one delegated-OAuth HTTPS API (default GitHub REST) and one
  remote MCP server we control.
- The threat model lives in the private `sirerun/foundation` repository.
- Lanes run as cloud Claude agents (Sonnet); Opus reviews and re-verifies
  locally before each merge.

### Objectives

1. A reviewed, amended RFC-003 and a signed-off threat model.
2. `POST /v1/executions`, `GET /v1/executions/{id}` and `gist_invoke` executing
   against two real providers on `https://registry.sire.run`, with idempotency,
   `unknown_outcome` honesty, approval verification, credential custody and
   egress controls, all covered by the threat-model test matrix.

### Non-goals

- Billing, the ledger, provider growth beyond two adapters, and publisher
  signing (M-G2, outline epic E-G7).
- Sire issuing signed approvals (outline epic E-G8).
- Everything in RFC-003 section 9: general compute, exactly-once beyond the
  provider's guarantee, Gist acting as a principal, native-tool hydration.
- Changes to the registry v1 contract.

### Constraints

- GitHub Actions is unavailable (account billing lock). All validation is
  local or in the cloud agent sandbox; merges are gated by local re-verification
  under the shared build lease (see Operating Procedure).
- The Mac mini is shared and memory-constrained. Heavy local runs take the
  lease `R-build-lease`, proceed only on `WON:`, and release with their own SHA.
- `sirerun/foundation` is private; cloud agents may not have access to it.
  Foundation tasks run as local lanes.
- The public repo must not contain the threat model, account IDs, secret
  values, or infrastructure internals beyond what ADR 008 already states.

### Success metrics

- Threat model signed off twice: design (G0.4) and implemented controls (G5.2).
- Threat-model test matrix (G4.1) passes with zero skips.
- Live on production: one read-only and one sensitive execution per provider,
  including one human-approved and one authority-approved execution, recorded
  in `docs/registry/gates/mg1.json`.

## Discovery Summary

Read-only discovery of `origin/main` at `aff5cef`:

| Existing surface | Location | Gateway use |
| --- | --- | --- |
| `requires_gateway` status | `hosted/internal/resolution/resolve.go:30,69,165`; `contract/wire.go:13` | Entry signal; today catalog metadata only |
| Provider support states | `provider.schema.json` (planned ... retired, `degraded_reason`) | No runtime reader yet; gateway reads `executable` and `degraded` |
| `effects` vocabulary v1 | `effects.schema.json` (class and sensitivity per term) | Sensitive effects route to approval |
| Capability fields | `cost.upper_bound`, `timeout_ms`, `retry`, `idempotency.supported`, `async_status` | Execution limits and retry policy |
| Execution schema | `execution_location` (client, runtime, gist_gateway), `credentials`, `destinations`, `provider_action` | Adapter input and destination allowlist |
| Connections | `ports/connections.go:14-27`; `resolution/connections.go`; `rest/connections.go` | Initiation exists; custody does not |
| Resolutions | `ports.Resolution{ID, Principal, Skill, ExpiresAt, Findings}`; principal-bound reads (`ports/principal.go`) | Execution presents a current resolution |
| Principal and policy | `ports.Principal`; actions `catalog:read`, `catalog:publish`, `identity:revoke`; `Decide` | Add `execution:invoke` |
| OAuth server | `hosted/internal/oauth` (RFC 8707 resource required; consent UI; delegated issuer) | Human identity for the approval step |
| Events | Only `version_published`, `version_revoked`; outbox rejects other kinds | Add connection and binding revocation kinds |
| MCP tools | Exactly four (`remotemcp/tools.go:3`), pinned in `docs/registry/clients/m2a.json:35` and asserted in `acceptance/clients/workload_test.go:252` | Add `gist_invoke` and amend all three |
| Contract gate | `scripts/registry/check.py:104` rejects `/execut`; lock hashes per file | New `contracts/gateway/v1` with its own gate |
| Storage | Migrations 001-007; RLS `tenant_isolation`; `storage.WithTenant` | Migration 008 onward |
| Tests | `integration` and `live` tags; agent-browser browser tests; gate JSON in `docs/registry/gates/` | Same conventions |
| Idempotency | Publish and revocation only | Execution idempotency is new |

RFC-002 phrases the gateway must honor: a resolution is "not an execution
grant"; the gateway "re-verifies everything server-side"; "revocation blocks
new resolutions and execution"; tokens use RFC 8707 audience binding; scopes
are upper bounds; effect reclassification "must not silently change execution
permissions".

## Use Case Summary

All use cases are PLANNED. Machine-readable copy:
`.claude/scratch/usecases-manifest-gateway.json` (gitignored).

| ID | Use case | Priority |
| --- | --- | --- |
| UC-G01 | A runtime-less MCP client executes a read-only capability with `gist_invoke` | P0 |
| UC-G02 | A REST client executes with `POST /v1/executions` and reads the result with `GET /v1/executions/{id}` | P0 |
| UC-G03 | A sensitive effect executes only with a signed, unspent approval from a registered authority | P0 |
| UC-G04 | A sensitive effect for a runtime-less client executes after a logged-in human approves on the Gist-hosted page | P0 |
| UC-G05 | A workspace connects a delegated-OAuth provider account; Gist holds the refresh token encrypted and never returns it | P0 |
| UC-G06 | A revoked binding, connection, or version cannot execute, even with a cached resolution | P0 |
| UC-G07 | Retrying with the same idempotency key returns the same execution; a changed payload is a conflict | P0 |
| UC-G08 | An unconfirmable provider outcome is reported as `unknown_outcome` and never retried automatically | P0 |
| UC-G09 | Provider calls to private, loopback, link-local or metadata addresses are refused, including after redirects | P0 |
| UC-G10 | Provider results are screened and returned as data, never as instructions | P1 |
| UC-G11 | A workspace admin registers, rotates and revokes an approval authority's public key | P0 |
| UC-G12 | A provider credential failure marks the provider `degraded` with the loss disclosed, never silently switching account | P1 |

## Scope and Deliverables

In scope: RFC-003 review and amendments; the private threat model; the M-G1
technical ADR; the gateway contract; execution, approval, custody, egress and
screening code; two provider adapters; the approval page; `gist_invoke`; the
foundation KMS and egress changes; production deployment and live evidence.

Out of scope: billing and ledger, M-G2 providers, publisher signing, Sire-side
approval signing, any registry v1 contract change.

| ID | Deliverable | Acceptance |
| --- | --- | --- |
| D1 | Amended `docs/rfc-003.md` | Review findings resolved; David sign-off recorded |
| D2 | Threat model (private, foundation) | Covers RFC-003 section 7 plus ADR 009 additions; signed off twice |
| D3 | ADR 010: M-G1 technical design | Enums, state machine, approval format, idempotency binding fixed |
| D4 | `contracts/gateway/v1` with gate | `check.py gateway --freeze-check` passes |
| D5 | Gateway code in `hosted/` | Unit, integration, browser and threat-matrix suites green |
| D6 | Production deployment | Live evidence in `docs/registry/gates/mg1.json` |

## Checkable Work Breakdown

Conventions: `Owner: cloud` means a cloud Claude agent (Sonnet) in its own
sandbox; `Owner: local` means a local lane (foundation access or production
credentials); `Owner: opus` means the coordinating Opus session. `lane: agent`
marks design-heavy work. Estimates are per task. Every code task ends with
gofmt, go vet and its package tests; G4.5 runs the full gate.

### E-G0: Review, threat model and technical ADR

fidelity: executable
Acceptance: David has signed off the amended RFC, the threat model and ADR 010.

- [x] G0.1 Review RFC-003 and amend it  Owner: opus  Est: 90m  verifies: [infrastructure]  lane: agent  acc: [docs/rfc-003.md states the two approval sources, KMS custody, the separate gateway contract and gist_invoke, and every review finding is resolved or listed as deferred]
  - Do: review RFC-003 against RFC-002 and the Discovery Summary. Amend sections 3, 4, 8, 9 and 10 per ADR 009. Record findings and their dispositions in the PR.
  - Acceptance: status line reads "Reviewed 2026-09-xx"; no contradiction with ADR 009 or RFC-002 sections 6, 9-12.
- [x] G0.2 Write the threat model  Owner: opus  Est: 90m  verifies: [infrastructure]  lane: agent  acc: [the private threat model maps each RFC-003 section 7 threat and each ADR 009 addition to a named control and a named test in G4.1]
  - Do: in `sirerun/foundation` (private; `security/gist-gateway-threat-model.md`, since foundation ignores `docs/`). Add to section 7: approval-page phishing and CSRF; authority key compromise and rotation; KMS key misuse and cross-workspace data keys; DNS rebinding and redirect SSRF; confused deputy across connections; MCP provider tool-result injection.
  - Acceptance: every threat names its control, its test ID, and its residual risk.
- [x] G0.3 Write ADR 010, the M-G1 technical design  Owner: opus  Est: 90m  verifies: [infrastructure]  lane: agent  deps: [G0.1]  acc: [ADR 010 fixes the outcome and request-sent enums, the execution state machine, the approval evidence format and the idempotency binding, each with an example]
  - Fix: outcome confirmation enum (success, failure, accepted_unconfirmed, nonexecution, unknown) and request-sent enum (no, yes, unknown); execution states and transitions; approval evidence (JWS, algorithms, claims: authority, workspace, execution binding hash, jti, exp); single-use rule; idempotency binding (tenant, principal, binding, connection, payload hash); `execution:invoke` scope; `contracts/gateway/v1` layout; event kinds `connection_revoked`, `binding_revoked`; provider adapter interface.
- [ ] G0.4 David signs off G0.1 to G0.3  Owner: David  Est: 30m  kind: human  verifies: [infrastructure]  deps: [G0.1, G0.2, G0.3]  acc: [sign-off recorded in the ADR 010 status and the threat model header]
  - Done as multiple-choice cards per document. Any "revise" answer loops back to its task.

### E-G1: Contract and core libraries

fidelity: executable
Acceptance: every library has unit tests; the gateway contract is frozen.

- [ ] G1.1 Gateway contract `contracts/gateway/v1`  Owner: cloud  Est: 90m  verifies: [UC-G01, UC-G02, UC-G11]  deps: [G0.4]  acc: [check.py gateway --freeze-check passes and check.py contracts still rejects /execut under registry v1]
  - Files: `contracts/gateway/v1/{openapi.yaml,route-matrix.json,lock.json,fixtures/}`, `scripts/registry/check.py`, `scripts/registry/check_test.py`.
  - Routes: `POST /v1/executions`, `GET /v1/executions/{id}`, `POST /v1/approvals/{id}/decision` (page form target), `GET /v1/approvals/{id}`, `POST|GET|DELETE /v1/approval-authorities`. Error envelope identical to registry v1.
- [ ] G1.2 Migration 008: execution and custody schema  Owner: cloud  Est: 90m  verifies: [UC-G02, UC-G03, UC-G05, UC-G07, UC-G11]  deps: [G0.4]  acc: [migration 008 applies on a fresh database and every new table enforces tenant_isolation under RLS]
  - Tables: `executions`, `execution_attempts`, `execution_idempotency`, `approval_authorities`, `approval_requests`, `approvals_consumed`, `workspace_data_keys`, `provider_credentials`, `connection_accounts`. All carry `workspace_id` with the `tenant_isolation` policy.
  - Test: integration test that another workspace sees zero rows in each table.
- [ ] G1.3 Egress guard package  Owner: cloud  Est: 90m  verifies: [UC-G09]  deps: [G0.4]  acc: [the egress client refuses loopback, private, link-local, metadata, IPv6 ULA and NAT64 targets, including via redirect and DNS rebinding]
  - Files: `hosted/internal/egress/`. HTTPS only; IP check at dial time (not only at resolve time) to defeat DNS rebinding; redirect re-validation per hop; response size, time and content-type limits; bounded streaming.
  - Test: table tests with a fake resolver and a dialer spy; an httptest TLS server for redirect chains.
- [ ] G1.4 Credential vault with envelope encryption  Owner: cloud  Est: 90m  verifies: [UC-G05]  deps: [G0.4]  acc: [a stored credential decrypts only in its own workspace and no API type can carry plaintext secret fields]
  - Files: `hosted/internal/vault/`. Port `KeyWrapper` with an AWS KMS implementation (GenerateDataKey, Decrypt, encryption context = workspace ID) and a local implementation for tests. AES-GCM data encryption; per-workspace data keys; rotation re-wraps keys.
  - Test: fake KMS; wrong-workspace decrypt fails; rotation keeps old ciphertext readable.
- [ ] G1.5 Execution domain: states and idempotency  Owner: cloud  Est: 60m  verifies: [UC-G07, UC-G08]  deps: [G0.4]  acc: [illegal state transitions are rejected and the idempotency binding hash changes when any bound field changes]
  - Files: `hosted/internal/execution/` (pure, no I/O). ADR 010 enums, transitions, `unknown_outcome` never transitions to a retry for mutating effects.
- [ ] G1.6 Approval evidence verification  Owner: cloud  Est: 90m  verifies: [UC-G03, UC-G04, UC-G11]  deps: [G0.4]  acc: [an approval verifies only when signed by a registered, unrevoked key of the same workspace, bound to the same execution hash, unexpired, and unspent]
  - Files: `hosted/internal/approval/`. JWS verification with the existing `jwx` dependency; claims per ADR 010; port for authority lookup and consumption.
  - Test: wrong key, wrong workspace, changed payload, expired, replayed, and `alg: none` all fail.
- [ ] G1.7 Result screening  Owner: cloud  Est: 60m  verifies: [UC-G10]  deps: [G0.4]  acc: [provider results pass through the package screening posture and are returned in a data envelope with a screening verdict]
  - Reuse the injection screening used for published packages (locate in `hosted/internal/publication`); wrap results as data with a `screening` field.

### E-G2: Stores, custody, providers and approvals

fidelity: executable
Acceptance: each service works against Postgres and fakes; no surface wired yet.

- [ ] G2.1 Postgres stores for executions and approvals  Owner: cloud  Est: 90m  verifies: [UC-G02, UC-G03, UC-G07]  deps: [G1.2, G1.5, G1.6]  acc: [two concurrent consumers of one approval produce exactly one success, and an idempotency replay returns the first execution]
  - Single-use consumption in one transaction; idempotency insert-or-return with payload hash comparison (conflict on mismatch).
- [ ] G2.2 Connection ownership and credential custody  Owner: cloud  Est: 90m  verifies: [UC-G05, UC-G06, UC-G12]  deps: [G1.2, G1.4]  acc: [a completed delegated-OAuth connection stores an encrypted refresh token, refreshes it, and emits connection_revoked when revoked]
  - Extends `resolution/connections.go`: account record names workspace, principal and capability scope; refresh and rotation; failure marks the provider `degraded`; new event kinds added to the outbox allowlist.
- [ ] G2.3 Provider adapter interface and HTTPS adapter  Owner: cloud  Est: 90m  verifies: [UC-G02, UC-G05, UC-G09]  deps: [G1.3, G1.4, G1.5]  acc: [the HTTPS adapter calls only the binding's declared destinations through the egress guard and reports request-sent and outcome per ADR 010]
  - Default provider GitHub REST (confirmed at G0.4). Golden fixtures recorded against an httptest server; no live calls in tests.
- [ ] G2.4 MCP provider adapter and fixture server  Owner: cloud  Est: 90m  verifies: [UC-G01, UC-G09, UC-G10]  deps: [G1.3, G1.5]  acc: [the MCP adapter calls a tool on the fixture MCP server through the egress guard and returns a screened data envelope]
  - Fixture server in `hosted/acceptance/gateway/mcpfixture` (Streamable HTTP, one read-only and one mutating tool).
- [ ] G2.5 Gist-hosted human approval step  Owner: cloud  Est: 90m  verifies: [UC-G04]  deps: [G1.1, G1.6]  acc: [a logged-in human in the right workspace can approve or deny a pending approval request, and the decision is recorded as that human's approval]
  - Page reuses the OAuth consent patterns (CSRF token, real Origin under `Referrer-Policy: no-referrer`, per-tenant session). Shows the exact effect, provider, account and payload summary. Deny and expiry are terminal.
- [ ] G2.6 Approval authority registration  Owner: cloud  Est: 60m  verifies: [UC-G11]  deps: [G1.1, G1.2, G1.6]  acc: [only a workspace admin can register, rotate or revoke an authority key, and a revoked key's approvals stop verifying]
- [ ] G2.7 `execution:invoke` scope  Owner: cloud  Est: 60m  verifies: [UC-G01, UC-G02]  deps: [G1.1]  acc: [a token without execution:invoke is refused on every gateway route, and the OAuth server and workload issuer can grant the scope]
  - New `ports.Action`; OAuth consent lists it; workload tokens carry it only when the parent scope allows (scopes are upper bounds).
- [ ] G2.8 Foundation: KMS key and egress rules  Owner: local  Est: 60m  verifies: [infrastructure]  deps: [G1.4]  acc: [pulumi preview for registry-aws shows one KMS key, task-role kms permissions scoped by encryption context, and task egress limited to 443]
  - Repo: `sirerun/foundation`, `pulumi/registry_aws*.go`. Mock tests; preview only (apply happens in G5.1).

### E-G3: Execution service and surfaces

fidelity: executable
Acceptance: REST and MCP execute through one service with full re-verification.

- [ ] G3.1 Execution service  Owner: cloud  Est: 90m  verifies: [UC-G02, UC-G03, UC-G04, UC-G06, UC-G07, UC-G08]  deps: [G2.1, G1.7]  lane: agent  acc: [execution re-verifies policy, revocation, binding currency, connection authority and scope at run time, and refuses a sensitive effect without verified approval]
  - Adapters behind the G2.3 interface; tests use a fake adapter so this task does not wait for G2.3 or G2.4. Returns `approval_required` with the approval link when needed.
- [ ] G3.2 REST execution routes  Owner: cloud  Est: 60m  verifies: [UC-G02, UC-G07]  deps: [G3.1, G1.1]  acc: [POST /v1/executions returns the contract's status and body for success, conflict, approval_required and unknown_outcome, as asserted by API tests]
- [ ] G3.3 MCP `gist_invoke` tool  Owner: cloud  Est: 60m  verifies: [UC-G01, UC-G04]  deps: [G3.1, G2.7]  acc: [tools/list shows five tools only to principals with execution:invoke and four otherwise, and gist_invoke returns a URL elicitation or approval link when approval is required]
  - Amend `remotemcp/tools.go`, `docs/registry/clients/m2a.json` (or a new gateway client record), and `acceptance/clients/workload_test.go` tool-count assertion.
- [ ] G3.4 Revocation in the execution loop  Owner: cloud  Est: 60m  verifies: [UC-G06]  deps: [G2.2, G3.1]  acc: [an execution with a resolution issued before a binding, connection or version revocation is refused]
- [ ] G3.5 Wire gateway into the app  Owner: cloud  Est: 60m  verifies: [UC-G01, UC-G02]  deps: [G3.2, G3.3, G2.3, G2.4, G2.5, G2.6]  acc: [app.New composes the gateway with both adapters, the vault and the approval page, and the wiring acceptance suite executes through the real composition]
  - Config: `GIST_KMS_KEY_ID`, gateway limits; startup fails closed when the KMS key is missing in production mode.

### E-G4: Verification

fidelity: executable
Acceptance: the threat-model matrix and all suites pass with zero skips.

- [ ] G4.1 Threat-model test matrix  Owner: cloud  Est: 90m  verifies: [UC-G03, UC-G06, UC-G07, UC-G09]  deps: [G3.5]  lane: agent  acc: [acceptance/gateway runs one named test per threat-model row and all pass]
  - Cross-tenant execution, replay and double-spend, authority widening (broader token, swapped cursor, client-supplied approval ID), credential exfiltration, result injection, revoked binding, approval-page CSRF, authority key compromise, DNS rebinding.
- [ ] G4.2 Browser tests for the approval page  Owner: cloud  Est: 60m  verifies: [UC-G04]  deps: [G2.5]  acc: [agent-browser tests pass for approve, deny, CSRF and cross-tenant attempts]
- [ ] G4.3 Gateway API acceptance  Owner: cloud  Est: 60m  verifies: [UC-G01, UC-G02, UC-G11]  deps: [G3.5]  acc: [every gateway route in the contract route matrix is exercised against the real composition with status and body assertions]
- [ ] G4.4 Lint, format, race and gates  Owner: opus  Est: 60m  verifies: [infrastructure]  deps: [G4.1, G4.2, G4.3]  acc: [gofmt, go vet, go test -race, the integration suite, check.py contracts and gateway freeze checks, and evidence gates all pass locally]
  - Adds milestone `mg1` to `check.py evidence` and `docs/registry/gates/mg1.json`.

### E-G5: Production

fidelity: executable
Acceptance: two providers execute live on registry.sire.run with signed-off controls.

- [ ] G5.1 Provider credentials  Owner: David  Est: 30m  kind: human  verifies: [UC-G05]  deps: [G0.4]  acc: [a GitHub OAuth app for the gateway exists and its client secret is stored as a registry-aws stack secret]
  - David creates the OAuth app; the coordinator gives the exact `pulumi config set --secret` command. The value never enters chat.
- [ ] G5.2 Sign off implemented controls  Owner: David  Est: 30m  kind: human  verifies: [infrastructure]  deps: [G4.4]  acc: [threat model header records the implementation sign-off before any provider is marked executable]
- [ ] G5.3 Deploy and verify live  Owner: local  Est: 90m  verifies: [UC-G01, UC-G02, UC-G03, UC-G04, UC-G05, UC-G06]  deps: [G5.1, G5.2, G2.8]  acc: [on https://registry.sire.run, one read-only and one sensitive execution per provider succeed, one via human approval and one via a test authority, recorded in mg1.json]
  - Sequence per ADR 008: CodeBuild at the merged commit, pin digest, apply. Mark the two providers `executable` only after the live checks pass.

### E-G6: M-G1 exit

fidelity: executable
Acceptance: M-G1 closed with evidence.

- [ ] G6.1 M-G1 evidence and plan update  Owner: opus  Est: 30m  verifies: [infrastructure]  deps: [G5.3]  acc: [check.py evidence --milestone mg1 passes on main]

### E-G7: M-G2 provider growth, billing and signing

fidelity: outline
Intent: grow toward the eight provider families evidence-first, add the
append-only ledger and utility billing, and design publisher signing so
RFC-002 section 7's public-distribution block can lift.
Exit criteria: ledger live before any billed route; per-provider coverage
claims; signing procedure written.

- [ ] G7.0 PLAN: expand E-G7 to executable fidelity  Owner: opus  Est: 60m  kind: plan  delivers: [E-G7 at fidelity: executable]  deps: [G6.1]  acc: [parse_plan.py sees E-G7 with tasks carrying acceptance criteria and resolved deps]

### E-G8: Sire as an approval authority

fidelity: outline
Intent: make Sire's approval gate issue signed approvals in the ADR 010
format and register its key with Gist, so Sire-originated approvals verify.
Exit criteria: a Sire-approved sensitive execution succeeds live.

- [ ] G8.0 PLAN: expand E-G8 to executable fidelity (sire repo)  Owner: opus  Est: 60m  kind: plan  delivers: [E-G8 at fidelity: executable]  deps: [G6.1]  acc: [E-G8 tasks name sire repo files and a live acceptance check]

## Parallel Work

| Track | Tasks | Joins at |
| --- | --- | --- |
| Review | G0.1, G0.2, G0.3 | G0.4 |
| Contract | G1.1, G2.7 | G3.2, G3.3 |
| Data | G1.2, G2.1, G2.6 | G3.1 |
| Custody | G1.4, G2.2, G2.8 | G3.4, G5.3 |
| Egress and providers | G1.3, G2.3, G2.4, G1.7 | G3.5 |
| Approvals | G1.6, G2.5, G4.2 | G3.5 |
| Core | G1.5, G3.1 | G3.2 |

### Wave 0: Review (3 agents, Opus)

- G0.1, G0.2 in parallel; G0.3 after G0.1. Then G0.4 (David).

### Wave 1: Libraries and contract (7 cloud agents)

- G1.1, G1.2, G1.3, G1.4, G1.5, G1.6, G1.7. Also G5.1 (David) can happen now.

### Wave 2: Services (8 agents: 7 cloud, 1 local)

- G2.1, G2.2, G2.3, G2.4, G2.5, G2.6, G2.7 (cloud); G2.8 (local).

### Wave 3: Execution and surfaces (4 cloud agents, then 1)

- G3.1 first; then G3.2, G3.3, G3.4 in parallel; G4.2 also runs here (needs only G2.5). Then G3.5.

### Wave 4: Verification (2 cloud agents, then Opus)

- G4.1, G4.3; then G4.4.

### Wave 5: Production

- G5.2 (David), G5.3 (local), G6.1 (Opus). Then G7.0 and G8.0.

## Timeline and Milestones

| ID | Milestone | Exit criteria | Deps |
| --- | --- | --- | --- |
| MG1-a | Design signed off | G0.4 recorded | G0.1-G0.3 |
| MG1-b | Libraries merged | Wave 1 merged and locally verified | MG1-a |
| MG1-c | Gateway composed | G3.5 merged | Waves 2-3 |
| MG1-d | Verified | G4.4 gates pass | MG1-c |
| MG1 | Live | G5.3 and G6.1 | MG1-d, G5.1, G5.2 |

## Risk Register

| ID | Risk | Impact | Likelihood | Mitigation |
| --- | --- | --- | --- | --- |
| R1 | Review changes the design after build starts | High | Low | Build starts only after G0.4 |
| R2 | Local re-verification throughput limits merges | Medium | High | Merge in wave order; batch small PRs; cloud sandbox runs first |
| R3 | Cloud agents lack foundation access | Low | High | Foundation tasks are local lanes (G2.8, G5.3) |
| R4 | SSRF bypass (rebinding, redirects, IPv6 forms) | High | Medium | Dial-time IP check; G4.1 rebinding tests |
| R5 | Approval-page phishing or CSRF | High | Medium | Consent-page patterns; exact effect summary; G4.2 tests |
| R6 | Authority key compromise | High | Low | Per-workspace keys, rotation, revocation, short approval expiry |
| R7 | Budget: KMS and egress cost on a $50 budget | Low | Medium | One KMS key; no NAT; alerts per ADR 008 |
| R8 | MCP clients lack URL elicitation | Medium | Medium | Approval link in the tool result as the baseline |
| R9 | Parallel lanes conflict in `app.go` and `tools.go` | Medium | Medium | Only G3.3 and G3.5 touch them; rebase on merge |

## Operating Procedure

Dispatch: each task goes to one cloud Claude agent (Sonnet) with the task text,
ADR 009, ADR 010 and the gateway contract. The agent works on its own branch in
`sirerun/gist`, runs gofmt, go vet, `go test -race` for touched packages and
the integration suite in its sandbox, and opens a PR.

Merge gate, per PR:
1. Opus reviews the diff against the task acceptance and ADR 010.
2. Opus rebases onto `main` and re-runs locally under `R-build-lease` (only on
   `WON:`, released with its own SHA): gofmt, go vet, `go test -race ./...`,
   the integration suite, `check.py contracts --freeze-check`,
   `check.py gateway --freeze-check` (after G1.1), and evidence gates.
3. Rebase-merge. GitHub Actions results are ignored (billing lock).

Definition of done: acceptance met; API tests for every route change; browser
tests for the approval page; locally verified merge; production tasks verified
live on `https://registry.sire.run`.

Decisions go to David as multiple-choice questions. Threat-model content stays
in the private repo.

## Hand-off Notes

- Start with G0.1 and G0.2 (Opus). Nothing in E-G1 onward may start before G0.4.
- The default HTTPS provider (GitHub REST) and the fixture MCP server are
  confirmed or changed at G0.4.
- `registry-aws` must be live (ADR 008 apply sequence) before G5.3.

## Progress Log

- 2026 09 27: plan created (E-G0 to E-G6 executable, E-G7 and E-G8 outline); ADR 009 accepted.

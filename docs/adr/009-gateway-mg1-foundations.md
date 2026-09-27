# ADR 009: Execution Gateway M-G1 Foundations

## Status

Accepted

## Date

2026-09-27

## Context

RFC-003 specifies the execution gateway: `POST /v1/executions`,
`GET /v1/executions/{id}`, the `gist_invoke` MCP tool, credential custody,
egress controls, approval-reference verification, and (in M-G2) billing. It is
a draft that has not been reviewed as its own document, and section 10 lists
decisions that must be made before M-G1. It also leaves open how approvals
work for callers that have no approval system of their own: Claude, ChatGPT or
Codex connected over MCP, and customer scripts. Those are exactly the
`requires_gateway` clients the gateway exists for.

The RFC-002 registry is live code with a frozen v1 contract. Its contract gate
(`scripts/registry/check.py`) rejects any route containing `/execut`, and its
MCP surface is pinned at four tools. The founder asked for the build to run in
parallel on cloud Claude agents (Sonnet), with local verification before merge.

## Decision

David made each choice below on 2026-09-27.

1. **Review gate first.** An Opus review of RFC-003, the section 7 threat
   model, and a technical M-G1 ADR come before any build lane. David signs off
   each one. No build lane starts before sign-off, and no provider is marked
   `executable` before the implemented controls are signed off again.
2. **Scope.** M-G1 is planned to executable depth. M-G2 (provider growth,
   billing, publisher signing) and wiring Sire as an approval authority stay
   outline epics until M-G1 lands.
3. **Approvals: one evidence format, two sources.**
   - External approval authorities (Sire first, then others such as Zatiti or
     customer systems) register a public signing key per workspace. They send
     a signed, single-use approval bound to the exact execution request.
   - Runtime-less clients get a Gist-hosted approval step. The execution
     returns `approval_required` with a link (and an MCP URL elicitation where
     the client supports it). The authenticated human, identified by the
     built-in OAuth server, approves or denies there.
   - In both cases a human or an external runtime originates the approval.
     Gist records and verifies it and never approves on its own authority, so
     the RFC-003 non-goal "Gist originating approvals" is preserved. RFC-003
     sections 3 and 9 are amended to say so.
   - A model-supplied `approved` value is never evidence.
4. **Deployment.** The gateway ships in the same registry service and image on
   the `registry-aws` stack (ADR 008). Gateway-specific resources (KMS key,
   egress rules) are added to that stack.
5. **Credential custody.** Provider refresh tokens and secrets are stored in
   Postgres, encrypted with per-workspace data keys wrapped by one AWS KMS key
   (envelope encryption). Rows are tenant-scoped under the existing row-level
   security. A local key wrapper serves development and tests.
6. **Contract separation.** Gateway routes get their own contract directory,
   `contracts/gateway/v1`, with its own lock and gate. Registry v1 stays frozen
   and unchanged. `gist_invoke` becomes a fifth MCP tool, offered only to
   principals holding the `execution:invoke` scope.
7. **First providers.** M-G1 proves execution with two adapters: one HTTPS API
   behind delegated OAuth (default: GitHub REST) and one remote MCP server that
   we control as a fixture. The broad provider inventory is M-G2.
8. **Document homes.** The design (RFC amendments, ADRs, plan) is public in this
   repository. The section 7 threat model and its sign-off live in the private
   `sirerun/foundation` repository, so attack details are not published.
9. **Build process.** Lanes run as cloud Claude agents (Sonnet) in their own
   sandboxes. Each runs the full suite there and opens a PR. Opus reviews each
   PR and re-runs the full validation locally under the shared build lease
   before merging. GitHub Actions is unavailable (ADR 008).
10. **Zatiti and other systems.** No caller gets special treatment. Every
    external system uses the same paths: an OAuth client or workload identity
    to call the gateway, and approval-authority registration to supply
    approvals.

## Consequences

- The gateway cannot execute a sensitive effect for a runtime-less client
  unless a human completes the Gist-hosted approval step. This adds friction
  to mutations by design.
- A compromised approval-authority key can approve executions in its workspace.
  The threat model must cover key registration, rotation, and revocation.
- Adding `gist_invoke` amends the pinned four-tool MCP surface, its client
  support record, and the acceptance test that asserts the tool count.
- Local re-verification serializes merges through the build lease, so merge
  throughput is lower than the lane count suggests.

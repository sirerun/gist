# ADR 009: Execution Gateway M-G1 Foundations

> Superseded 2026-10-16 by [ADR016](016-zati-library-first-rfc003-gateway-boundary.md). Gateway execution is retired. The original decision below is historical and supplies no current dispatch or deployment authority. Catalog provenance is retained under RFC-002.


## Status

Founder approved the revised M-G1 direction for planning and implementation on 2026-10-11. Exact design reconciliation, independent review, and the G0.4 record against the reviewed revisions remain open.

## Date

2026-09-27 (amended 2026-10-11)

## Context

The registry has a frozen v1 contract. The gateway adds execution and KMS-backed
provider credential custody across HTTPS and MCP providers. On 2026-10-11 the
founder approved caller-owned user approval (without a Gist approval page or
approval JWS), OpenAPI-generated gateway MCP tools, dynamic task-specific
guidance with an evaluation, GitHub REST, and a controlled MCP fixture for M-G1.
Billing and broad provider expansion remain M-G2. Credential intake mode remains
unresolved; see the explicit open decision below.

## Decision

Original decisions: 2026-09-27. The 2026-10-11 founder approval selects the
revised direction for planning and implementation. G0.4 records that approval
against the exact independently reviewed RFC, ADR, threat model, and task graph.

1. **Review and owner record.** A non-author independently reviews RFC-003,
   the private threat model, technical ADRs, and the complete implementation
   plan. G0.4 records the founder's 2026-10-11 approval against those exact
   reviewed revisions. No provider is marked executable before the implemented
   controls and live qualification gates pass.
2. **Scope.** M-G1 is planned to executable depth. M-G2 (provider growth,
   billing, publisher signing) and broader caller integrations stay
   outline epics until M-G1 lands.
3. **Trust the caller for user approval.** Codex, Claude, Zatiti, Sire, and
   other authenticated callers handle consent in their own harnesses. Gist
   enforces token audience, scopes, policy, revocation, exact capability/binding,
   and explicit connection grants. It requires no action JWS, authority-key
   registration, `approved` boolean, or Gist-hosted approval page. A compromised
   caller can exercise its existing grants; the revised threat model must
   document that residual risk. This supersedes the original two-source model.
4. **Deployment.** The gateway ships in the same registry service and image on
   the `registry-aws` stack (ADR 008). Gateway-specific resources (KMS key,
   egress rules) are added to that stack.
5. **Credential custody.** KMS envelope encryption is selected for provider
   credentials. The intake mode (stored-only connection setup versus one-shot
   caller-supplied credentials) remains unresolved. Until the founder records
   that choice, no provider-credential input field or secret-ingress path may be
   implemented. Once selected, the design must preserve tenant isolation and
   encrypt credentials with per-workspace data keys wrapped by AWS KMS.
6. **Contract separation and generated MCP.** Gateway routes stay in
   `contracts/gateway/v1`; registry v1 stays frozen. Generate the gateway MCP
   tools from explicitly exposed OpenAPI operations. Generated tools must
   preserve REST schemas, authorization, errors, and results; they do not
   replace the existing four registry tools. Invocation requires
   `execution:invoke`.
7. **First providers.** M-G1 proves execution with two adapters: one HTTPS provider route
   (default proving provider remains GitHub REST; Composio is an aggregator route) and one remote MCP server that
   we control as a fixture. The broad provider inventory is M-G2.
8. **Document homes.** The design (RFC amendments, ADRs, plan) is public in this
   repository. The section 7 threat model and its sign-off live in the private
   `sirerun/foundation` repository, so attack details are not published.
9. **Delivery evidence.** Implementation, independent review, merge, and
   landed verification follow the authoritative delivery plan. Generated MCP
   and REST behavior must have parity tests. Hosted CI, local tests, provider
   qualification, and production acceptance are reported as separate evidence.
10. **Caller and operator identity.** Runtime callers use Gist-issued bounded
    grants. RFC-002 operator enrollment uses the approved permissions-free
    GitHub App sign-in, numeric account identity, and invitation to one private
    workspace; Gist stores no GitHub token. Treg and Composio are provider
    connections, not identity providers. Exact workspace and operator identity
    remain pending; provider credential intake is a separate open decision.
11. **Dynamic tool-use guides.** Assemble task-specific guides from validated
    capability contracts, authorized resolutions, and maintained recipes.
    Authored craft skills remain separate. Include generation and correctness/
    discovery-turn evaluation in M-G1. The exact contract, threat cases, and
    evaluation are part of the reviewed design and implementation graph.

## Credential clarification still pending

KMS envelope encryption is selected. The founder has not selected whether
provider credentials enter only through stored connection setup or may also be
supplied one-shot by the caller. Do not implement either secret-ingress route
until that choice is recorded and the threat model covers its lifecycle. For
Composio, connected-account references remain scoped to an authorized
integration/workspace; they do not prove identity or user approval.

## Consequences

- User approval happens in the calling harness; Gist cannot independently
  prove per-action consent. Explicit grants and revocation bound caller risk.
- Generated MCP requires deterministic generation and schema/auth/result parity
  checks. The original unconditional five-tool assertion must be replaced by
  an explicit generated-operation allowlist plus registry compatibility checks.
- Dynamic guidance adds contract/provenance/freshness work and an evaluation
  requirement; fewer discovery turns are a target, not a measured result.
- The exact design and private threat model require independent review before
  security-sensitive credential or provider execution paths ship. Contract and
  guide work may proceed only through the dependency-ready authoritative plan.
  Local verification and production control sign-off remain mandatory.

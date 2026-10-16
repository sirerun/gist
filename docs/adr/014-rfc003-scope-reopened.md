# ADR014 -- RFC-003 gateway scope and M-G1 direction

> Superseded 2026-10-16 by [ADR016](016-zati-library-first-rfc003-gateway-boundary.md). Gateway execution is retired. The original decision below is historical and supplies no current dispatch or deployment authority. Catalog provenance is retained under RFC-002.


Date: 2026-10-11
Status: Accepted scope and M-G1 direction; exact design reconciliation and independent review remain open.

## Context

ADR013 recorded the founder's 2026-10-08 decision to deliver the Gist registry with caller-owned integrations and exclude a managed execution gateway from that production scope. The founder later directed Gist to complete pending RFC-002 and RFC-003 work, reopening RFC-003 for Gist delivery.

On 2026-10-11 the founder approved the revised M-G1 direction for planning and implementation: caller-owned user approval without a Gist-hosted approval page or approval JWS; a separate OpenAPI contract that generates gateway MCP tools; dynamic task-specific tool-use guidance with an evaluation; KMS-encrypted provider credential custody; GitHub REST plus a controlled MCP fixture for M-G1; billing and provider inventory expansion in M-G2. This supersedes the earlier Gist-hosted approval/JWS design choice. PR #47's remote head does not contain the approved amendment; its preserved dirty worktree remains evidence/source material and must not be overwritten.

The founder also approved permissions-free GitHub App sign-in for RFC-002 operator enrollment, stable numeric account identity, invite-only access to one private workspace, no stored GitHub token, and Gist-issued bounded workload grants. Treg and Composio remain separate provider connections, not identity providers. The concrete workspace and operator account are still pending.

## Decision

1. RFC-003 is included in Gist's delivery scope, with a complete design-to-operations lifecycle in the authoritative plan.
2. ADR013 remains the historical record of the earlier decision. Its statements excluding a managed gateway are superseded for Gist only. Its decisions that Gist is optional to Zatiti and other callers, and caller-owned direct integrations remain supported, are unchanged.
3. The approved M-G1 direction is caller-owned approval; generated gateway MCP from a separate OpenAPI contract; dynamic guidance grounded in validated capability/recipe/current authorized resolution; KMS-encrypted credential custody; GitHub REST and a controlled MCP fixture. M-G2 retains billing and broad provider expansion.
4. The exact public RFC/ADRs, private threat model, and complete task graph must be reconciled and independently reviewed. G0.4 records the founder's already-given direction against the exact reviewed revisions; it does not reopen the rejected hosted approval/JWS design.
5. KMS encryption is selected, but the credential intake mode (stored-only versus one-shot caller-supplied credentials) remains an explicit owner decision. Do not expose a provider-credential request field or implement a secret-ingress path until that decision is recorded. Other non-secret contract and guide work may proceed through the accepted plan.
6. RFC-002 enrollment uses the approved GitHub App sign-in and bounded-grant model, but onboarding remains incomplete until the exact invited workspace and numeric operator account are supplied and verified.
7. Approval of the design direction does not grant provider spend, production deployment, AWS role, DNS-write, or account-enrollment authority. RFC-002 registry delivery remains independently in scope; gateway work does not block it except where an accepted dependency requires it.

## Consequences

The authoritative plan covers requirements, design/threat-model review, implementation, meaningful verification, independent review, fixes, guarded merge, landed checks, release, AWS deployment, bounded live acceptance, and operations. Existing RFC-003 worktrees, PRs, and private threat-model drafts remain preserved and are reconciled by evidence. Provider limits, exact enrollment identities, caller runtime, AWS role, and DNS-write authority remain separate gates.

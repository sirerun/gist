# ADR014 -- RFC-003 gateway scope reopened

Date: 2026-10-10
Status: Accepted scope change; revised technical design and implementation remain gated.

## Context

ADR013 recorded the founder's 2026-10-08 decision to deliver the Gist registry with caller-owned integrations and exclude a managed execution gateway from that production scope. The founder has now explicitly directed Gist to complete all pending implementation work under RFC-002 or RFC-003. This reopens RFC-003 for Gist delivery.

Existing RFC-003 materials and the preserved PR #47 worktree disagree. The frozen RFC-003/ADR009/ADR010 design uses Gist-hosted approval evidence and a gateway-defined approval authority. The preserved amendment proposes caller-owned approval, OpenAPI-generated MCP tools, dynamic tool-use guidance, and leaves some credential custody choices unresolved. The private threat model still covers the older approval design. No revised design has passed G0.4.

## Decision

1. RFC-003 is included in Gist's delivery scope, with a complete design-to-operations lifecycle recorded in the authoritative plan.
2. ADR013 remains the historical record of the earlier decision. Its statements that no managed gateway is selected are superseded for Gist only. Its decisions that Gist is optional to Zatiti and other callers, and that caller-owned direct integrations remain supported, are unchanged.
3. This scope decision does not approve the preserved PR #47 amendment or any other detailed gateway design. The RFC, technical ADR, private threat model, and executable task graph must first agree and receive independent review.
4. G0.4 remains an explicit human design gate. No RFC-003 implementation, provider execution, deployment, DNS change, or production acceptance may be treated as authorized by this scope change alone.
5. RFC-002 registry delivery remains independently in scope. Gateway work does not block registry source delivery or qualification except where a specific accepted dependency requires it.

## Consequences

The authoritative plan must include the gateway's requirements, design and threat-model review, implementation, verification, independent review, correction, guarded merge, landed checks, release, AWS deployment, live bounded acceptance, and operations as first-class tasks. Existing RFC-003 worktrees, PRs, and threat-model drafts remain preserved and must be reconciled by evidence before adoption. Provider limits, identity/workspace enrollment, caller runtime, AWS role and DNS write authority remain separate gates.

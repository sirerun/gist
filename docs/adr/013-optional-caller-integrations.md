# ADR013 -- Optional caller integrations

Date: 2026-10-08
Status: Accepted scope decision; implementation and runtime acceptance remain separately gated.

## Context

The four-owner architecture discussion closed as VISION-ALIGN-20 after the founder's corrected VISION-ALIGN-17 decision. Earlier statements requiring Gist for Zatiti or its demo were superseded. Gist's own registry production delivery remains authorized in its existing plan.

## Decision

Gist is an optional supported capability registry. Independently configured direct MCP servers outside Gist are supported through the caller's qualified connection, tool binding and governance. Zatiti general use, its demo and the first app/host packet have no Gist prerequisite.

Context-index library adoption remains a separate decision over an authorized corpus; indexing is not a tenant authorization boundary. Serenity retains governed knowledge. Application/AMOS definitions and domain workflows remain app-owned. Gist supplies exact artifacts, registry metadata and scoped resolution evidence; the caller owns runtime dispatch, provider connections, current authorization, spend, effect policy and ambiguous outcomes. No managed execution gateway is selected.

Optional registry use still requires exact bytes/digests, trust/schema validation, current lease/grant/revocation checks and caller-side authorization immediately before dispatch. Discovery is not execution authority. A failed, expired or revoked Gist grant must not cause automatic rerouting to an independently configured direct connection. Direct MCP use has its own explicit authority.

## Consequences

The registry's full production requirements, testing/review/merge/landed gates, release, AWS deployment, live acceptance and operations remain in the existing plan. Generic client qualification must identify the real consumer and source/profile; fixture evidence does not qualify a native runtime. A proposed Gist/Zatiti seam packet is not an accepted implementation assignment and stays outside the demo critical path until separately owned and planned. This decision grants no provider calls, spend, runtime admission, release or deployment approval.

# ADR 011: Canonical Gist production origin and delivery completion

## Status

Accepted for founder-selected destination and scope, 2026-10-04. Implementation/cutover design remains proposed until its reviewed owner delivery; nothing has been deployed by this planning change.

## Context

ADR008 selected AWS and the earlier registry.sire.run origin. The founder now explicitly requires the plan to continue through the entire SDLC until the service is running in production at https://gist.sire.run on AWS, and confirmed registry plus caller-owned integrations rather than the separate RFC003 managed gateway. Historical deployment evidence and ADR008's AWS architecture, operator grants and cost/availability trade-offs are retained.

## Decision

1. The requested canonical production origin is **https://gist.sire.run**, on the existing approved AWS registry architecture. This supersedes ADR008 decision1's hostname, not its private store, account/region, IaC ownership, deployment boundaries or other accepted constraints.
2. Completion includes the confirmed registry/caller scope, real Treg/Composio action contracts and governed caller receipts, original required M2a/M2b/M3 acceptance, and operational evidence. Managed gateway, private application work and marketing publishing remain separate.
3. An implementation or review/merge/landed receipt, release artifact, initial rollout, health200 or planning checkpoint is a submilestone. Delivery ends only when the terminal production predicate in registry-recovery.md is independently true at the requested origin.
4. Reuse ADR007's exact origin/resource/issuer/redirect migration boundary: new target metadata/certificates/callbacks and fresh consent; no wildcard or cross-audience old-token acceptance. Reuse immutable artifact identity. Old-origin handling and rollback restore a complete versioned configuration and preserve production data.
5. Execute qualified AWS/IaC and Cloudflare workflows as agent-owned delivery tasks under existing applicable authority, with no artificial routine human pause. Planning alone does not implement/apply/admit anything, create a new cloud budget, select provider numerical ceilings or override missing scoped account/capture authority.

## Consequences and verification

Explicit IaC, DNS/ACM/ALB TLS, config/audience/issuer/callback cutover and external positive/negative acceptance are required. On this planning run gist.sire.run did not resolve from the local environment; registry.sire.run health and OAuth protected-resource metadata returned200. Those probes do not establish AWS inventory or authenticated runtime success. Private infrastructure remote source aa62ca046511015d427e067129cec05a374608c2 still fixes the old origin and DNS checker; deployment owner must produce a reviewed replacement. No AWS or Cloudflare account mutation was run.

[Production delivery plan](../plans/registry-recovery.md), [origin/issuer migration](007-registry-deployment.md), [AWS architecture](008-registry-aws-hosting.md).

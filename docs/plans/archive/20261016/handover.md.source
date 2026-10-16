# Gist delivery handover

Updated 2026-10-11. This is the current handoff; the authoritative delivery graph is [docs/plan.md](plan.md).

## Scope and current state

The owner selected the hosted registry with caller-owned integrations as the delivery scope. Completion requires it to run on AWS at **https://gist.sire.run**. RFC-003 M-G1 is also reopened for implementation under the 2026-10-11 owner-approved direction. Gist publishes, discovers, and resolves bounded capabilities; the caller retains action approval and execution authority.

RFC-002 is partially implemented. The core registry and publication-v2 source are merged. Operator enrollment, real Treg and Composio capability artifacts, composed caller acceptance, the remaining OAuth/client milestones, and the new production origin are still open in the plan.

No RFC-003 gateway implementation has shipped. Its revised design is under independent review. The approved direction is caller-owned approval, MCP tools generated from a separate OpenAPI contract, evaluated dynamic guidance, KMS-encrypted credential custody, and GitHub REST plus a controlled MCP fixture in M-G1. Billing and broader provider coverage are M-G2.

## Live observations

On 2026-10-11, https://registry.sire.run/healthz and /readyz each returned HTTP 200. This confirms the existing registry service responds; it does not satisfy acceptance for the selected gist.sire.run origin.

The configured DNS read for the active sire.run zone returned no exact gist.sire.run records, and an HTTPS request to gist.sire.run failed to resolve. No DNS write or AWS deployment was attempted. AWS deployment identity, target-stack match, write workflow, and production authority remain open in the plan.

## Owner decisions and open inputs

The owner approved permissions-free GitHub App sign-in to establish a stable numeric operator identity, invite-only access to one private workspace, no stored GitHub token, and Gist-issued bounded workload grants. The exact workspace and operator account are not yet supplied.

The owner approved the RFC-003 M-G1 direction above for planning and implementation. Do not request that direction again. After independent review, record G0.4 against the exact reviewed revisions. Provider credential intake remains undecided between stored-only and one-shot caller-supplied; no credential-ingress route is authorized until that choice is recorded. Live provider targets, allowed operations, call caps, expiry, and the matching AWS/DNS authority are also open.

## Current review and delivery gates

- [Gist PR #62](https://github.com/sirerun/gist/pull/62) contains the revised public design and current task-plan candidate. Independent exact-head review remains open.
- The private threat-model companion must be reviewed against the same design revision.
- The preserved PR #47 draft does not contain the approved amendment; do not treat it as implementation authority or overwrite its remaining work.
- At the preceding PR #62 head aaa99049dd96376c64d86b0f2f7fbd4bfd956318, all four hosted jobs failed before starting with the billing-lock annotation. The handover-correction revision requires a fresh check read before merge. Local Wazi plan conformance and contract fixtures are separate evidence; they are not hosted CI or application acceptance.
- Merge, landed verification, release, deployment, live caller acceptance, and operations remain separate required gates.

## Resume sequence

1. Complete independent exact-head review of the Gist design, private threat model, and RFC-002 boundary.
2. Resolve accepted findings, re-review the corrected revisions, record G0.4 without repeating the already approved direction, then validate and land the authoritative plan.
3. Continue the RFC-002 enrollment, provider-artifact, integration, OAuth/client, release, AWS, live-acceptance, and operations lanes.
4. Build and deliver the RFC-003 M-G1/M-G2 graph through implementation, verification, independent review, merge, landed checks, release, AWS deployment, bounded live acceptance, and operations.
5. Mark delivery complete only when the current plan's production predicate is satisfied at https://gist.sire.run.

This handoff makes no claim of complete RFC-002 or RFC-003 implementation, production acceptance, or a released gateway.

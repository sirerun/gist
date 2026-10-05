# Production owner decision handoff

Date: 2026-10-05  
Status: recommendations for named owners; none of the decisions below is recorded as approved.  
Source context: PR #55 candidate `a2379fda6bcccc29ee2700c75e95596cfcb2310a`, base `d75ac183fe8b949a4f1381e8ebca3226f0c0e282`.

This packet turns existing design and preflight evidence into concrete decisions. It does not authorize publication, account changes, provider calls, credential reads, DNS changes, deployment, or live acceptance. The current grant/user authorization supports only its already stated scope; it does not provide actual identity values, account bindings, provider targets, numerical ceilings, expiries, or consumer runtime evidence. An offline owner or silence cannot supply those facts or approve a proposal.

## Decisions for the founder and contract owner

### 1. Publication transport: ADR012

**Recommendation:** accept ADR012's new, explicitly versioned v2 publication interface: a strict JSON envelope; skill packages carried as base64 ZIP bytes with the manifest inside the archive; other artifact kinds carried as typed JSON documents whose exact raw token-span bytes are preserved. Freeze v2 schemas and OpenAPI media/version bindings before handlers are written. Preserve all frozen v1 bytes and lock entries. Require an explicit migration/deprecation behavior for the historical v1 raw-ZIP route before rollout.

This choice makes package bytes portable without inventing a second manifest copy that could disagree. It keeps per-kind schemas, provenance and admission checks distinct and makes archive/document digests separate from manifest and package-closure digests. The alternatives remain (a) a carefully reconciled v1 profile clarification or (b) multipart with an explicit method/media/schema binding. Neither is selected by existing fixtures.

**Owner question:** do the founder and registry contract owner accept v2 JSON with base64 skill ZIPs and typed raw JSON documents, choose a v1 profile clarification, or request a reviewed multipart alternative? Record the chosen compatibility and v1 behavior explicitly before `T-GR-PUBLISH.0` and `.1` proceed.

**Separate prerequisite before any canonical production writes:** the exact PR55 PostgreSQL/object-store fixture demonstrates that a catalog/outbox transaction rollback leaves the staged content-addressed blob on disk. A shared digest may be referenced by another committed version, so blind deletion on rollback is unsafe. The publication owner must approve a staging ownership and reconciliation design: distinguish attempt-owned staging from committed/shared objects, compensate only objects proven unreferenced, define bounded retention and a reconciler, and test failure/rollback/concurrency across replicas. Until those criteria pass, keep canonical publication writes gated.

### 2. Human identity, founder admission, and workspace custody

**Recommendation:** use a GitHub App solely for human sign-in, with no repository, organization, enterprise, installation, webhook, or data permissions. Bind the human subject to GitHub's durable numeric user ID, namespaced as GitHub; do not key authority on login, display name, or email. Discard the short-lived GitHub user token after identity verification. Admit the founder and later people only through expiring, single-use invitations bound to that immutable subject and one private Gist workspace. No public bootstrap or membership inferred from GitHub organization/repository access.

**Owner questions:**

1. Does the founder accept GitHub App sign-in plus private, invitation-only admission to one workspace?
2. Which verified founder GitHub account is the initial workspace owner, and who holds the invitation issuer and human callback/session custody?
3. What is the exact workspace identifier, and who is its accountable owner? Record the founder's immutable numeric subject only after an actual authenticated sign-in in the access-controlled operator record; do not put that identifier in this public project receipt.
4. Who is authorized to issue workload parent grants, and what exact scopes, audience, policy-generation and expiry limits may each grant carry?

**Recommended default if accepted:** GitHub establishes identity only. The Gist authorization service remains the issuer for Gist workload tokens. Workload grants require a separately authorized operator, stay within the invited workspace, and are subsets of an approved parent grant. Neither sign-in nor invitation grants provider execution.

### 3. AWS account/profile binding and DNS capability

The production destination already selected in the plan is `https://gist.sire.run`. A prior read-only production preflight found that the ambient authenticated AWS session resolves to a different account from the existing registry stack. Its region-scoped ECS/ACM lists were empty, which is not evidence that the stack's resources are absent. The available stack output is stale, and the original IaC checkout also needs an exact current-source refresh.

**Recommendation:** the AWS owner names the authorized profile/role and region for the existing registry stack. Before any stack or deployment operation, `aws sts get-caller-identity` under that named profile must match the account recorded by the freshly refreshed stack source/output. Reconcile the AWS principal, stack account, region, and current IaC source read-only first. Do not use the mismatched ambient session, guess a cross-account role, switch accounts by trial, or broaden trust.

**Owner question:** which named AWS profile/role is the authorized registry operator for the existing stack, and which access-controlled receipt will bind its authenticated account/region to the refreshed stack? If that binding cannot be proven, keep AWS writes blocked.

The preflight found no configured Cloudflare MCP binding available in this session. **Recommendation:** have the Cloudflare capability owner provide the approved, narrowly scoped MCP binding for the existing zone and ACM-validation-record task, then verify its permitted identity/scope before use. Do not substitute browser automation, generic API access, or an OAuth/configuration change for the missing binding.

**Owner question:** who owns and can configure that MCP binding, and what exact DNS record types/names and preservation checks may it manage for certificate validation? Until the binding is available and qualified, DNS writes remain blocked.

### 4. Provider candidate, spending bounds, and expiry

The preflight identifies two **candidates**, not approved targets or admitted execution artifacts:

| Candidate | Narrow proposed request | Proposed ceiling and lifetime | Still required |
| --- | --- | --- | --- |
| Treg `tavily.web.extract` | At most one public README URL; `basic`, Markdown; images and favicon disabled. Do not call it during metadata capture. | Proposal only: no more than $0.10 per call, $1 combined aggregate across both providers, at most 5 calls per provider in a rolling 24-hour window, with authority expiring after 24 hours. The captured basic-result price was $0.0016 per successful URL, subject to current account route and terms. | Owner must name the exact public URL and account/key custody, accept the total/per-call/count/expiry bounds, and qualify source license, retention, privacy, and permitted processing. No URL or provider key is selected here. |
| Composio `GITHUB_GET_A_PULL_REQUEST` | At most one read-only PR retrieval for explicitly selected repository/PR (the earlier candidate was registry PR48); no writes, comments, or issue mutation. | Proposal only: no more than $0.10 per call, $1 combined aggregate across both providers, at most 5 calls per provider in a rolling 24-hour window, authority expiring after 24 hours. These are proposed guardrails, not a price quote. | Owner must name the repository and PR, confirm the actual account plan/quota and connected-account scope, approve the caps/expiry, and authorize metadata-only schema capture before any action call. |

The Composio public documentation does not pin the exact action schema/version or the actual account's plan. Its pricing material contains differing overage statements, and current account terms are unknown. Treg's public metadata has no schema version or redistribution license field. The proposed limits do not replace account-specific cost checks or source/license/retention review. Keep both candidates `catalog_only`; do not capture credentials or call either provider until the owner answers the applicable questions. If the actual upstream account cost or terms exceed the proposed cap, stop and return for a new decision.

**Owner questions:** which candidate, exact target, and account are approved; who pays; is the table's maximum count/call/aggregate/24-hour expiry acceptable; and what source retention or redistribution is permitted? An available credit balance is not spending authority.

### 5. Consumer owner and actual runtime acceptance

The registry publishes/discovers/resolves metadata; the consumer owns provider execution and credentials. **Recommendation:** keep that boundary: a consuming product must choose and own its runtime, credentials, connection profile, funding, and final allow/deny decision. The registry does not execute imported tools.

**Owner questions:** who is the consumer owner; which pinned consumer build/runtime and caller identity will be tested; what exact registry import, consent, token/callback custody and per-connection behavior is required; and who will run and attest the real success, tenant-denial, revoked-grant, error, and cleanup journeys? The acceptance receipt must bind the actual consumer build and service source/config, not a route double, seeded catalog, or source-only test.

No consumer runtime owner/build or live acceptance receipt is supplied by this packet. Keep the production acceptance task open until that owner supplies and completes the named journey.

## Decision record checklist

Each answer should identify its decision owner, exact selected values, effective scope, expiry/rotation, any rejected alternatives, and the access-controlled evidence location. Keep account/user/provider identifiers and credentials out of public repository artifacts. Until answers are explicitly recorded, the safe defaults are: no provider calls; no AWS or DNS writes; no production publication; no inferred founder/workspace identity; and no claim of consumer runtime acceptance.

## Source basis

- `docs/adr/012-publication-transport-proposal.md` — v2 publication recommendation and unsettled alternatives.
- `docs/plans/E-GR-PUBLISH-canonical-publication.md` — publication decision and staging/reconciliation gates.
- `hosted/internal/app/core_publication_transaction_integration_test.go` — real local rollback fixture's explicit staged-blob boundary.
- `docs/receipts/2026-10-04-enrollment-decision-brief.md` — GitHub identity, invitation, workspace and separate workload-grant recommendation.
- `docs/plans/E-GR-SCOPE-requirements-and-preflight.md` — open founder/operator, provider-numerical, consumer-runtime, and automation-binding decisions.
- `docs/receipts/2026-10-05-production-bindings-refresh.md` — fresh read-only verification retains the target-account mismatch and unavailable DNS binding.
- `docs/receipts/2026-10-04-production-bindings-preflight.md` — authenticated AWS account mismatch, stale stack evidence, and unavailable Cloudflare MCP binding.
- `docs/receipts/2026-10-04-provider-preflight.md` — Treg/Composio source, candidate costs, missing target/schema/license evidence, and no-action boundary.

This receipt is a decision handoff, not a decision, grant, technical approval, deployment record, or production acceptance.

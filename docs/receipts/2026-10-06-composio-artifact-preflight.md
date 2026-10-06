# Composio action artifact preflight

Date: 2026-10-06

Task: T-GR-COMPOSIO.0, source and scope preflight only

Disposition: **BLOCKED — no action artifact may be captured or admitted**

Base: `92b9f6963d6fa5d2cc9e6fc0975811a0b720adcc` (immutable PR56 head)

Owned file in this lane: this receipt only

## Decision

The dependency rows T-GR-SCOPE.9 and T-GR-SCOPE.10 are checked complete at the assigned base. Their source contract identifies a Composio lane, a component boundary, expected denial coverage, public documentation paths, and an explicit rule that missing license/version/capture authority holds only this lane. That makes the assessment dependency-ready; it does not supply the owner approvals or account bindings required by T-GR-COMPOSIO.0's own acceptance criteria.

The proposed candidate remains `GITHUB_GET_A_PULL_REQUEST`, scoped in future to one owner-selected repository and PR. Current public Composio docs list that slug and show the GitHub toolkit's latest version as `20260924_00` on the access date. Those pages do not expose the action's exact input/output schemas, the exact schema digest, its version availability history, or the action-specific authorization mapping. `20260924_00` is therefore a public documentation snapshot, not a captured or approved immutable action version.

Composio's official tool API says schema lookup uses a project API key in `x-api-key`; the endpoint supports retrieving one tool's input and output schema and a toolkit version can be selected. No existing key binding, key custody, project/account binding, or metadata-only capture approval is qualified by the owner packet. No authenticated lookup was attempted. No Composio account or connection was opened; no action, execution endpoint, proxy, session, or provider call ran; no credential was read; no spend occurred.

## Candidate behavior and boundary

The underlying GitHub API's documented “Get a pull request” operation is `GET /repos/{owner}/{repo}/pulls/{pull_number}`. GitHub documents the path fields (`owner`, `repo`, `pull_number`), permits unauthenticated access for public resources, and lists read access to repository Pull requests or Contents for fine-grained credentials. GitHub also notes that a GET can cause it to create a temporary merge-test commit to determine mergeability; that test commit is not added to the base or head branch. This describes the GitHub API endpoint only. It does not establish that Composio maps its action to this exact path or these exact fields. That mapping, response schema, returned-content scope, auth configuration, and any upstream access beyond this operation remain unverified.

The contemplated component interface is one immutable, exact-version read tool artifact connected through a Composio provider/binding record to that action. Its admitted metadata would have to carry the exact input and output schemas and digests, schema dialect, provider/tool/version identity, source/capture provenance, effect and sensitivity classification, outbound destination, credential type/scopes, timeout/retry behavior, and license/retention evidence required by ADR006. Gist publishes and resolves that artifact; a consumer owns credentials, approval, policy, and execution. No provider action is admitted by a catalog entry, `requires_gateway`, source page, or successful schema lookup.

## Bounded source/write/test scope if blockers are resolved

This lane's actual write set is limited to this receipt. A later T-GR-COMPOSIO.1 implementation can own the following action-specific files, subject to confirming each path against the schemas returned by an approved exact-version metadata capture:

- `catalog/registry/captures/composio.github.get_a_pull_request.json` — source, exact toolkit/action/schema version, exact schema digests, capture time, auth and license/retention evidence; no API key, token, connected-account ID, or PR response body.
- `catalog/registry/packages/composio-github-get-a-pull-request/manifest.json`, `SKILL.md`, and `schemas/input.schema.json` / `schemas/output.schema.json` — one schema-preserving, non-executable package exposing only the exact PR read operation.
- `scripts/registry/composio_capture_test.py` — offline validation of source/capture digest closure, schema/version consistency and fail-closed admission metadata.

The new action record must not rewrite `catalog/registry/captures/composio.json`, the synthetic tool/binding/golden rows, or the provider's existing `catalog_only` status as if historical source had been captured. Shared indexes and canonical binding selection (`catalog/registry/{catalog,providers,tools,capabilities,bindings,goldens}.json`) remain integration-owned under the plan's ownership rule; T-GR-COMPOSIO.1 must hand off the exact proposed rows and changed-file list for that integration. No directories or schemas above were created in this preflight.

Meaningful offline tests for the later artifact candidate are: a valid pinned, digest-matched capture loads; a changed schema byte, floating/mismatched version, absent source/license/retention record, wrong tool/provider identity, undeclared fee/effect/destination, widened operation or additional action fails closed; synthetic identity mutation fixtures cannot satisfy the PR-read artifact; and package and golden fixture digests reconcile. A test with fake PR content proves only deterministic mapping/validation, never a live GitHub or Composio call. The exact positive test fixture cannot be authored until the actual returned schemas and redistribution terms are approved.

## Authority, account, cost, license, and retention

The 2026-10-05 production owner packet says its provider limits are proposals, not approvals. For Composio it requires an owner-selected repository/PR, actual plan/quota and connected-account scope, explicit numerical limits/expiry, and authorization of metadata-only schema capture before any action call. None is recorded as approved. The candidate target remains unset. Proposed limits in that packet (up to `$0.10` per call, `$1` aggregate across both providers, five calls per provider per rolling 24 hours, 24-hour expiry) are not authority or price evidence.

Public pricing documentation currently describes the Hobby plan with an included allowance of 100,000 monthly calls for own-app/API-key/MCP calls and up to 20,000 calls for Composio-managed apps, then lists managed-app overage at `$0.0005` per call. Its FAQ separately describes managed-app overage of `$0.0002` per call on top of the base rate. The actual Composio plan, connected GitHub auth mode, remaining quota, any legacy terms, and classification of this action are unknown. The pricing page gives no separate price for metadata schema lookup. Do not quote the lookup as free or infer a per-action marginal cost from a public price table.

Composio's current Data Retention guide says tool-execution logs normally store request arguments and response data for up to one year. Its Zero Data Retention guide describes project-specific limits and exceptions; the owner has not qualified any account/project setting. These are execution-log policies and do not specify retention for the tool-catalog metadata endpoint. The actual action's returned PR content could be present in default execution logs if later executed. There is no evidence here for the actual account's retention mode or GitHub's retention handling.

The current Composio Terms of Service do not grant an explicit license to redistribute action schema metadata as a public Gist artifact; the terms include restrictions on derivative or extraction use of platform content. No action-schema/data license or redistribution permission is asserted. The GitHub REST docs describe API operation and permission requirements but do not grant a license for Composio's wrapper schema or for publishing PR response content. A source attribution alone cannot resolve that gap.

## Required unblock evidence

Do not advance T-GR-COMPOSIO.1 until an owner-controlled record supplies all of the following without exposing identifiers/secrets in this public receipt:

1. The exact approved repository and PR target, intended consumer, and allowed returned-content handling.
2. Approval specifically for metadata-only schema lookup, named custodian of the Composio project key, and a qualified least-privilege project/account binding. Keep the key out of chat, this repo, process output, and the receipt.
3. An exact, supported toolkit version and action response captured from the official single-tool schema endpoint; input/output schema bytes and digests; action-specific GitHub endpoint mapping, credential mode/scopes, and no-write boundary. Reject `latest` as an artifact version.
4. Actual account plan, quota, connected account/auth configuration, applicable tool-call and lookup charges, owner-approved numerical budget/count/expiry, and who pays.
5. Applicable Composio/GitHub terms, exact schema redistribution permission, PR content license/processing basis, and account-specific retention setting with all material exceptions.
6. An independently reviewed, exact file list and offline test plan that preserves all historical synthetic evidence and keeps shared catalog-index edits with integration ownership.

Until then, preserve the historical `source_unavailable` capture and synthetic identity fixture as historical-only evidence, keep Composio and its candidate action `catalog_only`, and leave T-GR-COMPOSIO.0 unchecked. This receipt is not provider qualification, an execution grant, production authorization, or live/consumer acceptance evidence.

## Official sources consulted (2026-10-06)

- Composio [GitHub toolkit](https://docs.composio.dev/toolkits/github): lists `GITHUB_GET_A_PULL_REQUEST`; the page reports latest GitHub toolkit `20260924_00`, not a schema-specific digest.
- Composio [Tools API reference](https://docs.composio.dev/reference/api-reference/tools): lists `GET /api/v3.1/tools/{tool_slug}`, input/output schema retrieval, API-key authentication via `x-api-key`, and explicit toolkit-version selection.
- GitHub [Get a pull request](https://docs.github.com/en/rest/pulls/pulls#get-a-pull-request): documents endpoint, parameters, read permission and temporary mergeability test behavior.
- Composio [Pricing](https://composio.dev/pricing): current published plans/add-on rates and the FAQ/table managed-app overage discrepancy; not account-specific.
- Composio [Data retention](https://docs.composio.dev/docs/security/data-retention) and [Zero Data Retention](https://docs.composio.dev/docs/security/zero-data-retention): public retention behavior and ZDR scope/exceptions; not actual project settings.
- Composio [Terms of Service](https://composio.dev/terms): no explicit action-schema redistribution license identified; restrictions concerning derivative/extraction use remain for owner/legal review.
- Immutable source: `docs/plans/E-GR-COMPOSIO-composio-action-artifacts.md`, `docs/plans/E-GR-SCOPE-requirements-and-preflight.md`, `docs/receipts/2026-10-04-provider-preflight.md`, and `docs/receipts/2026-10-05-production-owner-decisions.md` at base SHA above.

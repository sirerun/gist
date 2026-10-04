# Official provider-source preflight

Date: 2026-10-04  
Scope: T-GR-SCOPE.10 source preflight only  
Disposition: neither candidate is admitted for execution; no live provider action was attempted.

## Finding

Current official documentation supports two narrow read-only candidates for a later, explicitly authorized qualification: Treg's Reddit keyword search and Composio's GitHub pull-request read. The documentation is sufficient to identify the action classes, public discovery surfaces, and material cost/auth boundaries. It is not sufficient to publish exact provider schemas, prove a pinned tool/catalog version, or establish all redistribution and privacy terms. Do not replace the existing synthetic Gist fixture with a guessed live capture.

The candidate remains a coordinator/founder decision. No agency audience, actual research query, target GitHub repository/PR, Composio connected account, Treg key/billing allocation, or price ceiling is selected in the saved proposal. A later request must specify the actual read and its scope before provider qualification.

## Treg

The official [Treg API reference](https://treg.to/docs) says catalog routes need no token, while calls to `/call/{endpoint_id}` relay a real upstream request. It says eligible calls without the team's own provider key are billed against prepaid balance at the provider's own rate; having a team's own provider key avoids Treg metering. The same page warns that relayed `Authorization` headers are sent upstream, so Treg credentials must not be put there. This makes a public catalog-definition read a possible bounded metadata capture, distinct from executing a research query. This task did not call the catalog API or a provider endpoint.

The official [Reddit catalog page](https://treg.to/catalog/reddit) lists keyword search actions, including `scrapecreators.reddit.search.posts` at a displayed $0.00188/call, and describes the action as searching Reddit posts by keyword. This is a candidate and a current page-level price observation, not a quote, a committed price, or proof of successful execution. It has no task-specific query, result cap, freshness contract, endpoint schema, data-use determination, or spend cap. Treg describes its API as relaying the upstream provider's native path, parameters, and response rather than modeling a stable provider API. Upstream source/version and response stability therefore remain open.

For a future authorized capture, the smallest useful source record is the exact public catalog response for the selected endpoint ID, preserved with retrieval time, source URL, response digest, provider/endpoint identifiers, request and response schema as supplied, published price unit, upstream/provider attribution, and any source/license fields. Capture only the definition; do not call `/call/...`. Confirm Treg's permission to retain/redistribute catalog data and the upstream provider's terms before publishing a Gist artifact. Treg's statement that its service is open source does not by itself establish a license for third-party catalog entries or downstream data.

## Composio

The current official [Tools reference](https://docs.composio.dev/reference/api-reference/tools) says tool list/search and single-tool schema retrieval are supported, and that these endpoints authenticate with a project API key in `x-api-key`. It also says manual execution requires an explicit toolkit version (`latest` or a dated version). The official [GitHub MCP toolkit page](https://docs.composio.dev/toolkits/github_mcp) presents `GITHUB_MCP_PULL_REQUEST_READ` as a read action and identifies toolkit version `20260910_00`. This is an appropriate discovery candidate only; no toolkit/account/session was opened and no Composio API was called. The generic API reference confirms the credential boundary but does not publish the exact input/output JSON Schema for this action.

An actual schema capture needs an explicitly approved, appropriately scoped Composio project credential and an authorized read of the tool definition, with the exact toolkit version pinned. The candidate PR read would additionally need an identified repository/PR and approval for the returned data to enter the proposed consumer. The GitHub connection's repository visibility and permissions must be bounded. No schema, scopes, connected-account behavior, current plan/cost, retention terms, or catalog redistribution license is asserted by this receipt. The official docs page listing the action is not a substitute for the authenticated action schema.

## Existing Gist evidence and safe publication boundary

The assigned base is `84e14128565058419a9b90619f27fa517ba4f494`. At that base:

- `catalog/registry/captures/composio.json` records `source_unavailable` for an obsolete documentation URL and says no schema was captured.
- `catalog/registry/tools.json` names `composio.synthetic.identity.user.create`, gives both schemas as unconstrained objects, and carries a synthetic capture date/version. `catalog/registry/goldens.json` contains synthetic mutation examples. These are authored fixtures, not provider evidence, and do not qualify the proposed pull-request read.
- No Treg capture or adapter was found in the registry catalog. Existing registry architecture treats Gist as catalog/discovery/schema/resolution and leaves action execution with the consumer; provider execution is outside this receipt.
- The registry artifact contract requires source/provenance, exact tool and schema versions, input/output schemas, effects/sensitivity, destinations, lifecycle, and capture evidence. Those fields must be grounded in actual source material before publishing an action as admitted.

Keep both candidates `catalog_only` until the exact source definition, version, schema, license/retention basis, and the required task/account/cost decisions are supplied and reviewed. A provider's marketing page, generic API docs, placeholder schema, synthetic fixture, or advertised price is not live acceptance evidence.

## Sources consulted

- Treg API reference: https://treg.to/docs (accessed 2026-10-04)
- Treg Reddit catalog: https://treg.to/catalog/reddit (accessed 2026-10-04)
- Composio Tools API reference: https://docs.composio.dev/reference/api-reference/tools (accessed 2026-10-04)
- Composio GitHub MCP toolkit: https://docs.composio.dev/toolkits/github_mcp (accessed 2026-10-04)
- Existing proposal: `docs/registry/proposed-treg-composio-20261003.md` in the coordinator worktree
- Existing evidence selections: `docs/plans/E-GR-TREG-treg-action-artifacts.md` and `docs/plans/E-GR-COMPOSIO-composio-action-artifacts.md` in the coordinator worktree

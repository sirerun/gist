# Official provider-source preflight

Date: 2026-10-04
Scope: T-GR-SCOPE.10 source preflight only
Disposition: neither candidate is admitted for execution; no live provider action was attempted.

## Finding

Current official sources support two narrow read-only candidates for a later, explicitly authorized qualification: Treg `tavily.web.extract` on one public README URL, and Composio `GITHUB_GET_A_PULL_REQUEST` scoped to the eventual Gist delivery PR. Treg's unauthenticated catalog metadata was fetched and captured below. It provides a bounded input shape and documented unit prices, but no provider schema version or license grant. Composio's exact schema/version still needs an authenticated metadata read. No provider action was executed.

The candidate remains a coordinator/founder decision. No agency audience, actual research query, target GitHub repository/PR, Composio connected account, Treg key/billing allocation, or price ceiling is selected in the saved proposal. A later request must specify the actual read and its scope before provider qualification.

## Treg

The official [Treg API reference](https://treg.to/docs) says catalog routes need no token, while calls to `/call/{endpoint_id}` relay a real upstream request. It says eligible calls without the team's own provider key are billed against prepaid balance at the provider's own rate; with a provider key registered to the team, Treg says those calls are not metered. The same page warns that relayed `Authorization` headers are sent upstream, so Treg credentials must not be put there. This receipt used only the documented public metadata route; it did not call `/call/...` or fetch the target README.

The selected candidate is `tavily.web.extract` for one URL. On 2026-10-04, `GET https://treg.to/catalog/endpoints/tavily.web.extract` returned HTTP 200, `application/json`, 54,411 bytes. The route is explicitly listed in Treg's API reference as `GET /catalog/endpoints/{endpoint_id}`, and the docs say catalog routes need no token. The capture retained only a selected, sanitized metadata projection: endpoint/provider identity, method/path, endpoint input shape, Treg's cost record, provider docs links, source verification date, and capture date. Its canonical sorted compact JSON SHA-256 is `c7391719a57f20518f8ca86dac8af3f7ca6fd264b34394c9382ad09f585611c5`. The digest definition covers those selected fields and excludes sibling endpoint records, usage observations, and review text. The raw response was not committed.

The captured request contract is JSON `POST /extract`: required `urls` array with 1–20 entries; optional `extract_depth` (`basic` or `advanced`, default `basic`), `format` (`markdown` or `text`, default `markdown`), `include_images` (default false), `include_favicon` (default false), `query`, `chunks_per_source` (1–5, default 3), and `timeout` (1–60). For the proposed single public README URL, explicitly set `extract_depth: basic`, `format: markdown`, and disable images/favicon. The source has no schema-dialect declaration or immutable provider/schema version; record the capture digest/date instead of inventing either.

The captured cost record is per successful result: basic is documented as 0.2 credit (Treg metadata maps the low end to $0.0016); advanced is 0.4 credit / $0.0032. Failed URLs are documented as uncharged; platform settlement is based on valid results, while a team's own registered Tavily key is unmetered by Treg and would have its separate upstream account cost. This supports a conservative documented ceiling of $0.0032 for one successful URL result, subject to account route and current provider terms. It is well below a proposed $0.10 per-call ceiling, but that ceiling and the proposed $1 total/5 calls/24-hour window have not been approved. The metadata source has no catalog-data or downstream-redistribution license field; Treg's general open-source statement does not grant rights to republish third-party catalog material. Confirm Treg and Tavily terms, retention, and permitted README content processing before publishing an executable Gist artifact.

## Composio

The current official [GitHub toolkit page](https://docs.composio.dev/toolkits/github) lists `GITHUB_GET_A_PULL_REQUEST`. This is the selected read-only candidate for a specific forthcoming Gist delivery PR. The current official [Tools reference](https://docs.composio.dev/reference/api-reference/tools) says tool list/search and single-tool schema retrieval are supported, that those endpoints authenticate with a project API key in `x-api-key`, and that manual execution requires an explicit toolkit version (`latest` or a dated version). The public toolkit page provides neither the action's exact input/output JSON Schema nor a pinned version. No Composio API/account/session was opened.

An actual schema capture needs an authorized metadata-only read using a least-privilege project key, with the exact toolkit version and input/output schemas pinned by capture digest. Do not call the action during schema capture. A later PR read would require the exact repository and PR, a GitHub connection scoped to only the intended repository, and approval for the returned content to enter the consumer. The Composio pricing page currently lists 100,000 own-app/API/MCP calls per month on Hobby and $0.0003 per call after the included amount on Pro; Composio-managed apps get up to 20,000 calls within the free allowance and then $0.0005 per call in the pricing table. Its FAQ separately says $0.0002 for managed-app overage, and existing customers may have grandfathered terms through 2026-12-31. The user's actual plan, quota, app type, add-ons, and grandfathered terms are unverified; do not claim an exact account cost. The read should remain unexecuted until those are checked against the actual account and any selected $0.10-per-call/$1-total caps are authorized. No schema, scopes, retention terms, or catalog redistribution license is asserted by this receipt.

## Existing Gist evidence and safe publication boundary

The assigned base is `84e14128565058419a9b90619f27fa517ba4f494`. At that base:

- `catalog/registry/captures/composio.json` records `source_unavailable` for an obsolete documentation URL and says no schema was captured.
- `catalog/registry/tools.json` names `composio.synthetic.identity.user.create`, gives both schemas as unconstrained objects, and carries a synthetic capture date/version. `catalog/registry/goldens.json` contains synthetic mutation examples. These are authored fixtures, not provider evidence, and do not qualify the proposed pull-request read.
- No Treg capture or adapter was found in the registry catalog. Existing registry architecture treats Gist as catalog/discovery/schema/resolution and leaves action execution with the consumer; provider execution is outside this receipt.
- The registry artifact contract requires source/provenance, exact tool and schema versions, input/output schemas, effects/sensitivity, destinations, lifecycle, and capture evidence. Those fields must be grounded in actual source material before publishing an action as admitted.

Keep both candidates `catalog_only` until the exact source definition, version, schema, license/retention basis, and the required task/account/cost decisions are supplied and reviewed. A provider's marketing page, generic API docs, placeholder schema, synthetic fixture, or advertised price is not live acceptance evidence.

## Sources consulted

- Treg API reference: https://treg.to/docs (accessed 2026-10-04)
- Treg Tavily endpoint metadata: https://treg.to/catalog/endpoints/tavily.web.extract (GET, accessed 2026-10-04; selected-field digest recorded above)
- Treg Tavily pricing/endpoint docs: https://treg.to/tools/tavily and https://docs.tavily.com/documentation/api-reference/endpoint/extract (accessed 2026-10-04)
- Composio Tools API reference: https://docs.composio.dev/reference/api-reference/tools (accessed 2026-10-04)
- Composio GitHub toolkit and pricing: https://docs.composio.dev/toolkits/github and https://composio.dev/pricing (accessed 2026-10-04)
- Existing proposal: `docs/registry/proposed-treg-composio-20261003.md` in the coordinator worktree
- Existing evidence selections: `docs/plans/E-GR-TREG-treg-action-artifacts.md` and `docs/plans/E-GR-COMPOSIO-composio-action-artifacts.md` in the coordinator worktree

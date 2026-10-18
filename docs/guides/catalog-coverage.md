# Public catalog coverage and setup

Browse [the public library](https://gist.sire.run/library) without an account.
Search by intent or service, inspect the source and connection requirements, then
export a descriptor for review in your client. An export is setup metadata;
it does not install a server or connect a provider account.

## Current coverage

The reviewed replacement contains **13 vendor-official MCP connectors**, one
per app: Gmail, Google Calendar, Google Drive, GitHub, Slack, Notion, Linear,
Stripe, Sentry, Jira, Confluence, Cloudflare and Exa. Each app is listed in
[n8n](https://n8n.io/integrations/); membership in
[Make](https://www.make.com/integrations) is also eligible. We deduplicate the
union by reviewed app identity. We do not import n8n nodes or Make modules.
This is an admitted subset, not complete directory coverage.

Each descriptor links vendor setup documentation. Listing membership and
vendor publication are checked separately. Credentials, scope selection,
execution and provider charges remain with your client. Vendor-official does
not mean tested in Zati. There are **zero Featured connectors**.

| Need | Setup candidate | Requirements |
| --- | --- | --- |
| Mail | Gmail official MCP | Google Developer Preview enrollment, enabled APIs and agent-enabled OAuth client |
| Calendar | Google Calendar official MCP | Preview enrollment and client-owned OAuth; select permitted calendar tools |
| Files | Google Drive official MCP | Preview enrollment, client-owned OAuth and appropriate file scope |
| Code review | GitHub official MCP | GitHub authorization and explicit tool admission |
| Web research | Exa official MCP | Vendor-documented no-key route, service limits and explicit client admission |

The first four remain recommended setup candidates with immutable screening
skill links. Existing read-only skills are not automatically compatible with
provider schemas. The old community Google OpenAPI alternatives and local
Brave package route were removed from the public snapshot under the approved
vendor-endpoint policy. Private custom entries and saved bookmarks are
preserved; a bookmark for a removed entry appears as unavailable and can be
removed by its owner.

Current Zati source supports admitted Streamable HTTP MCP profiles with bearer
or no authentication. It does not automatically run arbitrary local packages,
provide generic provider OAuth consent/refresh, or adapt any discovered schema
to an existing inbox capability. Mint currently generates stdio/SSE servers;
its OAuth metadata does not implement consent/refresh. A custom adapter or
client feature may be required before an imported entry is useful.

## Private and custom integrations

Sign in with an invited GitHub identity, then open **Private integrations**.
Workspace maintainers can save remote URLs, local executable/package descriptors
or OpenAPI URLs; current workspace members can read them. Saved custom metadata
stays private and never enters public search. Gist does not contact these URLs
or run packages. Enter credential environment **names**, never secret values.
Your client retains credentials, permissions, budgets and execution.

## Other clients and maintenance

The public Streamable HTTP metadata endpoint is
`https://gist.sire.run/directory/mcp`, with three discovery tools. Existing
`/mcp` remains the authenticated workspace artifact registry. Public descriptors
and private artifacts have different contracts; neither bypasses admission.

[Directory source and refresh instructions](../../catalog/directory/README.md)
include pinned inputs, source attribution, rejected-record receipts and bounded
refresh commands. Refreshes require review and an immutable release; a failed
refresh does not change the deployed image. Increase coverage deliberately,
review newly admitted sources and qualify operations before making quality
claims. Expired or changed qualification loses featured eligibility on read.

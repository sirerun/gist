# Public catalog coverage and setup

Browse [the public library](https://gist.sire.run/library) without an account.
Search by intent or service, inspect the source and connection requirements, then
export a descriptor for review in your client. An export is setup metadata;
it does not install a server or connect a provider account.

## Initial coverage

The pinned intake contains **6,494 candidates**: 3,913 MCP entries and 2,581
OpenAPI entries. It combines the reviewed Sire Run API registry with a bounded
40-page official MCP Registry sample. It is not the entire official registry
or every server on the internet. Distinct implementations and versions remain
separate; counts do not mean unique apps or tested workflows.

n8n's integration coverage is a benchmark for useful app operations. We do not
import or execute n8n nodes. mcpservers.org provides discovery leads; its listing
or official labels do not supply Gist workflow qualification.

| Need | Initial recommendation | Useful read-only operations | Current setup requirement |
| --- | --- | --- | --- |
| Mail | Gmail | List unread messages; read selected messages | Community OpenAPI + Mint; caller-owned Google authorization; adapter and schema admission still needed |
| Calendar | Google Calendar | List events in a bounded window | Community OpenAPI + Mint; caller-owned authorization; client transport compatibility still needed |
| Files | Google Drive | Search approved folders; inspect selected file metadata | Community OpenAPI + Mint; scope and folder admission still needed |
| Code review | GitHub official MCP | Read a public PR and selected changed files | Vendor endpoint and client-owned authorization; admit exact read tools |
| Web research | Brave official MCP package | Search a topic; cite selected public sources | Client with local package support, or separately qualified compatible hosting; provider account may be required |

These five are **recommended candidates**. None is featured at initial
publication. Featured requires a useful real workflow tested in Zati, a reusable
skill, documented setup and unexpired evidence tied to the exact descriptor,
tool/schema, skill and client versions. API reachability, fixture success and
vendor maintenance do not meet that standard on their own.

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

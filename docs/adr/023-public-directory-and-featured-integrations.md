# ADR023: Public directory and qualified featured integrations

Status: Accepted owner direction, 2026-10-17.

## Owner decisions

The owner approved nine clarified decisions: discovery in Gist with execution
in Zati or another client; public browsing and private workspaces; implement
and populate after design; n8n as an app/operation coverage benchmark; prioritize
mail, calendar, files, GitHub and research; require a dated useful workflow tested
in Zati for featured status; allow private URLs, local packages and custom OpenAPI;
prefer official vendor MCP with Mint for gaps; pair featured servers with skills.

## Current evidence

The inspected mcp-registry commit a6c7bd2 has 2,582 API entries, 2,580 distinct
specification URLs and 2,582 names. All have t1, none a verification date or
verifier version; four skip URL checks. These are discovered API candidates,
not proven working MCP servers. Preserve their current source compatibility.

The official MCP Registry provides versioned server/package/remote metadata.
Registry membership does not prove vendor maintenance or successful execution.
mcpservers.org supplies discovery leads; n8n supplies coverage references.
Mint has a public OpenAPI parser and stdio/SSE generation, with externally
supplied credentials. Its OAuth metadata does not implement consent/refresh.
Current Zati qualifies Streamable HTTP, not stdio. Record actual requirements.

## Directory architecture

Add a versioned catalog/directory metadata contract separate from frozen v1/v2
artifact contracts. A listing is not an executable binding. Public data comes
from bounded, reproducible snapshots with source identity, revision/digest,
observed time, upstream ID and attribution. Keep distinct implementations and
versions; curated service aliases may group them without equating contracts.
Vendor-official recommendations require actual maintainer evidence. Imported
claims never automatically become qualification evidence.

Intake does not execute packages, call tools, contact supplied server URLs or
generate credentials. Refresh failure preserves the prior accepted snapshot.
Snapshot publication follows normal review and immutable release. Serve bounded
public search/detail/export and a small read-only MCP interface; do not expose
each upstream operation as a Gist tool. Public routes do not bypass existing
authenticated REST/MCP or reveal private workspace records.

Directory records include ID, title, description, kind, service grouping, tags,
provenance, remote/package/OpenAPI descriptors, auth requirements and credential
names, linked skills, client requirements and qualification evidence. No secret
values. Bound parsing, field lengths, record counts, query/response bytes and
pagination. Render text safely; validate URL schemes and visibility. Clients
independently admit executable code, endpoints, schemas, accounts and tools.

Public endpoints: /api/directory, /api/directory/item, /api/directory/export and
/directory/mcp. Private descriptors use /app/api/directory. The approved Library
UI gains real public discovery and a separate private view. Explain native MCP
versus OpenAPI generation, source, auth requirements, compatibility and setup.
Credentials remain in clients. Custom remote/local/spec descriptors stay private
and grant no execution authority. Workspace writes require current maintainer
publication authority and same-origin CSRF protection; reads require current
membership. Gist never fetches a user's private/custom endpoint.

## Featured standard

Separate discovered, schema-checked, protocol-checked and workflow-tested states.
Featured requires a selected useful workflow, reusable skill and guide, exact
server/tool/schema/client versions, dated real Zati results, scoped effects and
unexpired evidence. Expiry, incompatible updates or failures remove eligibility.
Missing accounts, adapters or results leave ordinary candidate listings. Initial
workflows: inbox briefing, calendar agenda, finding files, public code review and
source-grounded research. Fixtures never become live qualification.

Public metadata and source links do not bypass ADR019's executable package trust
and release gates. Operator-asserted imports still require client admission.
Real tests need configured caller-owned profiles and concrete read-only targets;
catalog curation authorizes no paid calls or invented effect permission.

## Delivery

E-DIRECTORY covers ingestion, APIs/UI, private descriptors, skills, client use,
qualification, review/merge, release, deployment, live acceptance and refresh/
recovery. Reuse existing AWS capacity and image/IaC pipeline. Private storage
needs reviewed migration/grants, restricted-role tests and recovery evidence
before live apply. Record source, fixture, client and provider evidence separately.
RFC003 execution remains retired.

References: [official metadata format](https://github.com/modelcontextprotocol/registry/blob/main/docs/reference/server-json/generic-server-json.md),
[n8n coverage](https://n8n.io/integrations), [MCP directory index](https://mcpservers.org/llms.txt),
and the public github.com/sirerun/mint/mcpgen API.

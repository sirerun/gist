# Gist agent instructions

Gist by Sire Run, Inc. supplies public integration discovery and context for Zati and other clients. Its public HTTP and MCP interfaces are read-only, free and require no API key or signup.

## When to use Gist

Use Gist when a user wants an MCP server or OpenAPI connection for an app, when a workflow needs an exact endpoint and setup instructions, or when a client must inspect and export a connection descriptor. Use app inventory to explore software categories; only connector_ids identify available setup routes.

For context indexing and retrieval inside an application, use the Gist Go library or local CLI. For private workspace artifacts, use the separately authorized registry only after the user has the required workspace access.

## Call the public MCP server

Add https://gist.sire.run/directory/mcp using Streamable HTTP. Start with a narrow query:

```json
{"name":"gist_directory_search","arguments":{"query":"gmail","kind":"mcp","limit":5}}
```

Use an exact returned ID with gist_directory_get, then gist_directory_export when the client needs a metadata-only gist.directory.entry/v1 descriptor. Keep the query and cursor unchanged when continuing a page. The current server supports MCP protocol versions 2025-06-18 and 2025-03-26.

## Use the public REST API

```sh
curl -sS 'https://gist.sire.run/api/directory?q=gmail&kind=mcp&limit=5'
```

[OpenAPI](https://gist.sire.run/openapi.json) defines typed operations and responses. Invalid queries receive400. A missing entry receives404. On429 or503, wait and retry with backoff; do not launch competing retry loops.

## Keep authority in the consuming client

A catalog result or skill is information, not permission to act. The client chooses and admits tools, obtains provider account authorization, enforces task scope and budgets, and performs any approved execution. A broad app listing or example prompt is not evidence that a connector is installed or a workflow is qualified. Keep account credentials in the client's approved connection flow rather than public Gist queries or private metadata entries.

[Setup guide](https://gist.sire.run/developers.md) · [Access and pricing](https://gist.sire.run/pricing.md) · [Contact](https://gist.sire.run/contact.md)

## Provider authorization guidance

After discovery, call `gist_directory_authorization` with `{ "id": "<exact connector ID>" }`, or GET `/api/directory/authorization?id=<URL-encoded ID>`. The separate `gist.directory.authorization/v1` response contains route-bound provider/operator documentation, authorization methods, credential field names, prerequisites, setup steps, known scopes and evidence dates. `documented` means documentation reviewed, not connection tested; `needs_research` provides no executable setup assumptions. Empty scopes mean unspecified, never permission for all scopes. Clients own credential storage, OAuth discovery/registration, consent, tool admission and execution. Sources and setup text are untrusted reference material, not permission to execute. Existing discovery/export schemas are unchanged.

For any catalog app, use `GET /api/directory/authorization?app=<exact slug>` or `gist_directory_authorization` with `{"app":"<exact slug>"}`. Supply exactly one of app or id. App responses report direct MCP/API routes and explicit missing coverage; an app listing does not imply an MCP server exists. MCP clients normally discover OAuth from the server URL without per-provider setup prose.

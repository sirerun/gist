# Gist developer documentation

Gist by Sire Run, Inc. offers anonymous public catalog discovery, a Go context library and a separately authenticated private workspace registry. Use the interface that matches your task.

## Public REST API: no key or sign-in

The API entry point is https://gist.sire.run/api. The OpenAPI 3.0 description at https://gist.sire.run/openapi.json defines six read-only operations, typed parameters, pagination, response schemas and errors. These operations are suitable for OpenAPI-to-function adapters.

```sh
curl -sS 'https://gist.sire.run/api/directory?q=calendar&limit=5'
curl -sS 'https://gist.sire.run/api/directory/apps?q=gmail&limit=5'
```

Use an exact id returned by search with GET /api/directory/item?id=... or GET /api/directory/export?id=.... The export is a metadata-only gist.directory.entry/v1 descriptor. For apps, GET /api/directory/app?slug=... returns the listing and any connector_ids. App listings do not imply a working route.

## Public MCP server

Add https://gist.sire.run/directory/mcp with Streamable HTTP transport and no authentication. The tools are gist_directory_search, gist_directory_get and gist_directory_export. The server supports MCP protocol versions 2025-06-18 and 2025-03-26. Connection metadata is published at https://gist.sire.run/server.json using the MCP Registry schema; this does not claim registration in an external directory.

## Pagination and errors

Use limit 5 to start. Integration search permits 1–50 results and returns next_cursor; preserve the query and send that cursor for the next page. App search permits 1–100 results and returns next_offset; use offset for continuation. Response budgets can shorten a page.

400 means the query is invalid; fix it before retrying. 403 can indicate a disallowed browser Origin; server-side clients can omit Origin. 404 means the entry no longer exists. Use GET for public REST reads; other methods receive 405. Concurrent capacity is bounded. Back off on 429 or 503 and avoid retry storms. No numeric per-minute allowance or unlimited availability is promised.

## Private workspace registry

https://gist.sire.run/mcp is a separate OAuth-protected endpoint for invited workspace access. Website GitHub sign-in and authorization of an MCP client are separate steps. Its registry tools include gist_discover, gist_get and gist_resolve. Existing authenticated API contracts remain in the source repository; the public directory OpenAPI does not grant private access.

## CLI and embedded Go library

Install the official local CLI with brew install sirerun/tap/gist. See https://gist.sire.run/cli.md for scope, releases and storage setup. Go applications can import github.com/sirerun/gist for context indexing/retrieval and github.com/sirerun/gist/catalog/directory for embedded catalog metadata.

## When to use Gist

Use Gist to find an integration, inspect its setup, retrieve exact context or resolve requirements before a client chooses tools. Gist's directory does not authorize or execute provider actions. Your client admits tools, connects accounts, manages credentials, budgets and approvals, and verifies the result.

[OpenAPI](https://gist.sire.run/openapi.json) · [API catalog](https://gist.sire.run/.well-known/api-catalog) · [Agent instructions](https://gist.sire.run/llms.txt) · [Contact](https://gist.sire.run/contact.md)

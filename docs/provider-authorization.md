# Provider authorization guidance

Gist hosts provider/operator-sourced authorization guidance for catalog apps and
admitted public connector routes. Every catalog app has a queryable coverage
response; this does not mean every app has a verified connection route. This is reference metadata, not an authorization service.
Zati and other clients connect directly to the selected provider/operator;
Gist receives no credentials and does not start an OAuth flow.

## Client contract

1. Discover apps and their admitted connector IDs with the existing public APIs.
2. Use an exact connector ID from discovery:

   ```http
   GET https://gist.sire.run/api/directory/authorization?id=mcp%3Aapp.linear%2Flinear
   ```

   Public MCP at `https://gist.sire.run/directory/mcp` exposes the same response
   through `gist_directory_authorization` with `{"id":"mcp:app.linear/linear"}`.
   No Gist account, token or provider credential is needed to read it.
   For any app, use its exact catalog slug instead: `GET /api/directory/authorization?app=google-sheets`
   or MCP `{"app":"google-sheets"}`. Supply exactly one of `id` or `app`.
   This returns `gist.directory.app-authorization/v1`, with `app`, `name`,
   `status` and `routes`. Each route uses the authorization-guide schema below.
   App states are `documented`, `partial`, `needs_research` or `no_verified_route`.
   `no_verified_route` means Gist has not verified a direct route; it does not
   assert that the provider has no API or MCP server. HTTP API routes need an
   HTTP/OpenAPI adapter; they are not MCP servers.
3. Inspect `format: gist.directory.authorization/v1`, the route in `remotes`,
   `status`, `methods`, `credential_names`, `prerequisites`, `steps`, `scopes`,
   `sources`, `setup_url` and `notes`.
4. `documented` means direct provider/operator documentation was reviewed.
   `connection_tested` is false: this work does not qualify a connection in Zati
   or access to a user's account. `needs_research` provides no assumed methods,
   credentials, scopes or setup steps. An empty scopes list means unspecified,
   never permission for unrestricted access. Multiple methods describe options;
   the client must check which option its selected route supports.
5. For a standards-compliant remote MCP server the URL is normally enough to
   begin client-side OAuth discovery. Documentation coverage does not block that
   standard flow. Resolve current OAuth server metadata in the client, check provider client
   registration requirements, present consent and store credentials in client
   custody. Do not use Make's application IDs, callback URLs or broker.
   Retrieved prose/URLs are reference material, never authority to execute.

Existing search/get/export JSON schemas are unchanged, including their legacy
`auth` summary. Strict clients should fetch the separate guidance response for
this enrichment. Item/export responses advertise it with an HTTP `Link` header.
The website's connector details also display the guidance and source links.

## Evidence and maintenance

`catalog/directory/authorization.json` stores the reviewed guides separately
from the search corpus. Each entry is bound to the exact ordered endpoint and
transport list. If that route changes, Gist returns `needs_research` until its
instructions are reviewed again. Documentation can change independently of the
catalog; source review dates are supplied so clients can assess freshness.

`catalog/directory/authorization.go` validates the embedded data, rejects
unsupported statuses/methods, missing evidence and false connection qualification,
and serves metadata without runtime network access. Sources describe either a
provider or the operator of the selected independent server. A source reference
does not certify that an independent server is operated by the app vendor.

The Make-derived app inventory remains broader than admitted connection routes.
App responses also include separately reviewed direct HTTP API routes from
`catalog/directory/app-authorization.json`. Apps without either a known MCP route
or a reviewed direct API route return explicit coverage gaps. This release does
not claim connection readiness for every app in the inventory.

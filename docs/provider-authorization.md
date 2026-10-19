# Provider authorization guidance

Gist hosts provider/operator-sourced authorization guidance for catalog apps and
admitted public connector routes. Every catalog app has a queryable coverage
response; this does not mean every app has a verified connection route. This is reference metadata, not an authorization service.
Zati and other clients connect directly to the selected provider/operator;
Gist receives no credentials and does not start an OAuth flow.

## Client contract

1. Search the complete app inventory with MCP `gist_directory_apps` or REST
   `GET /api/directory/apps?q=...`. Request guidance using an exact returned app
   slug and version2. The separate `connector_ids` field identifies admitted
   descriptors that support the existing get/export interfaces.
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

## App-specific endpoints and version2

Use `GET /api/directory/authorization?app=shopify&version=2` or MCP
`gist_directory_authorization` with `{"app":"shopify","version":2}` for
`gist.directory.app-authorization/v2`. Version1 remains the default, with its
original field set and status values. Connector-ID responses remain v1.

Version2 routes can include `api_url_template`, named `endpoint_inputs`, and a
`protocol` for `native-api` adapters. A provider-generated private MCP URL is a
client input, not a URL that Gist stores. Template substitutions and endpoint
values stay in the client. API/DB adapters must perform their own destination,
credential and effect checks. A guide with `transport: unknown` does not guess
between SSE and Streamable HTTP; the client must verify the transport.

The v2 method `oauth1a` identifies OAuth 1.0a token exchange and request signing;
it is distinct from `oauth2` and is omitted from the default v1 projection.

App-level `sources` and `notes` explain researched gaps and platform-specific
modules. `not_applicable` requires source evidence that a listing is a platform
module rather than an independently connectable provider. It does not mean the
module's capabilities cannot be implemented through another tool. Version1
omits routes requiring these new fields and points callers to version2.

App route IDs identify guidance within an app response; they are not automatically
admitted directory descriptor IDs. Retrieve those routes through the exact app
slug. Existing connector IDs from directory discovery still use the ID selector.
Documented URLs do not imply a passing endpoint probe, installed client adapter
or usable provider account. Each of those has its own acceptance.

Use `connectable=true` on `/api/directory/apps` or `{"connectable":true}` with `gist_directory_apps` to hide apps without a documented setup route. The Library offers the same “Connectable only” filter. This includes MCP and direct API/native routes; client adapters, provider account eligibility and credentials remain required. It does not certify a tested connection. Preserve the filter during pagination.

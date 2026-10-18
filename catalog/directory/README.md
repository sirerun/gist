# Public directory metadata

## Application browsing and connection descriptors

The application inventory (`apps.json`) is separate from the admitted MCP/OpenAPI
descriptor snapshot (`snapshot.json`). The web library browses applications by
name and category; an app detail links to a connector only through an exact
service identity or a reviewed alias. A directory listing never invents a
connection endpoint. Applications without a connector show that connection
details are not available yet. Existing connector bookmarks and private data
retain their identities.

`/api/directory/apps` supports `q`, `category`, `offset` and `limit` (up to 100).
`/api/directory/app?slug=...` returns one app and its admitted connector IDs.
The existing directory HTTP/MCP interfaces continue to return connector
descriptors. Their projection retains fields required by strict clients and
skill-integrity pins, but keeps upstream intake, endpoint checks and workflow
evidence out of browsing responses. Public provenance identifies the Gist
catalog; original intake evidence remains in the operator snapshot.
The hosted service uses `DefaultPublic()` for REST, MCP and bookmark searches,
so hidden intake phrases cannot affect result membership and public purpose
descriptions are searchable. Its deterministic revision differs from the raw
operator snapshot, keeping cursors and descriptor selections bound to the view.

Brand icons are served locally. Transparent white marks are traced from a
normalized alpha mask and composed over an explicit six-digit brand background
color. Opaque or multicolor artwork is retained as a local PNG to preserve its
appearance. Missing source artwork gets a plain initial on the brand color.
The website does not request remote icon URLs. To regenerate the inventory:

```sh
python3 catalog/directory/intake/build_app_directory.py \
  --cache /path/to/private/intake-cache \
  --output catalog/directory/apps.json \
  --icons-dir hosted/internal/webui/assets/apps
```

To convert a single transparent mark with a chosen brand background:

```sh
python3 catalog/directory/intake/render_brand_icon.py \
  --input /tmp/google-email.avif --output /tmp/gmail.svg \
  --background '#f14336' --foreground '#ffffff' --title Gmail
```

The background is a separate SVG rectangle. Its color can change without
re-tracing the foreground geometry. ImageMagick with AVIF support and VTracer
0.6.5 are required for intake only; production serves the generated assets.

`Default()` loads the committed immutable candidate snapshot without network
access. `Search`, `Get` and `DefaultSnapshot` return independent copies. Search
uses term-prefix postings, bounded queries and snapshot/query-bound pagination.
`cmd/gist-directory` validates, searches and exports exact descriptors.

The hosted Gist service never connects to advertised servers, starts packages,
generates Mint adapters, authenticates provider accounts, or stores credentials.
The offline intake checker makes a bounded, no-credential request to each
reviewed remote endpoint. Streamable HTTP routes receive `initialize`; legacy
SSE routes receive a short GET. HTTP 200 with a matching, structurally valid
MCP initialize result confirms that handshake. HTTP 401/403 records an authentication challenge, 402 records
a payment requirement, and other accepted responses record URL reachability
only. Response headers received before the bounded stream/body timeout establish
reachability only; timeouts before any HTTP response fail. Nothing lists tools,
authorizes an account, verifies tool behavior, or qualifies a client workflow.
Credential names are labels, never values.

## Reproducing the pinned snapshot

```
python3 catalog/directory/intake/build_snapshot.py
```

To refresh reachability evidence, run:

```
python3 catalog/directory/intake/check_mcp_endpoints.py \
  --input catalog/directory/intake/curated.json \
  --output catalog/directory/intake/endpoint-checks.json
```

The deterministic offline build hashes `intake/curated.json`,
`intake/approved-membership.json`, and `intake/endpoint-checks.json`. The
published set contains one connector per reviewed app identity. Eligibility
requires a reviewed listing in **either n8n or Make** and an exact remote MCP
route that responds to the bounded check. Make's published verified remote MCP
server list is also an eligible Make listing source. Vendor-operated routes
carry first-party evidence; other routes are labeled “Operator not verified by
Gist” and link their Make or MCP Registry listing. Reachability does not
establish vendor ownership, safe tools, account access, compatibility with a
particular MCP client, or workflow qualification.

Add a connector by reviewing its canonical app identity, first-party directory
listing, server provenance, exact route and account setup. Add the membership and
descriptor together; refresh endpoint checks; regenerate and review the diff.
The builder rejects missing membership, duplicate apps, missing provenance,
unsafe URLs, mismatched checks and qualification claims. It never fetches
listings, packages or specifications. Endpoint checks pin a public IPv4 address,
verify TLS, use no credentials, disable ambient curl configuration and proxy
settings, disable URL globbing, do not follow redirects, and make no
`tools/list` or `tools/call` requests. DNS lookup is separately bounded to three
seconds and accepts only public IPv4 answers.

The current snapshot admits 127 remote MCP entries: 24 vendor-verified routes
and 103 routes whose operator Gist has not independently verified. The 103 new
entries came from Make's frozen verified-server list, after omitting 14 routes
that returned HTTP 410 or did not resolve. The list also links 15 existing app
identities. Four Zati setup recommendations remain: Gmail, Google Calendar,
Google Drive and GitHub.
Google’s three endpoints are Developer Preview and require enrollment and
agent-enabled OAuth setup. Their linked screening skills remain candidates.
The old community Google OpenAPI alternatives and packages-only Brave route
are absent under the approved policy; the historical screening packages remain
available separately. Exa provides an additional vendor web-search candidate.
None of these connectors has Featured workflow qualification.

App aliases are reviewed explicitly rather than deduplicated by display name.
The captured Make verified-server list contains 140 rows, including 132 literal
HTTPS routes; 117 endpoint/transport pairs responded and 14 did not respond or
resolve after deduplication. The final snapshot has 23 n8n app listings, four
Make app-directory listings, and 118 Make verified-server listings. The n8n app
catalogue was not exhaustively searched for other vendor MCP routes, so this is
not complete coverage of every n8n and Make app. See the dated expansion receipt
for exact source and endpoint hashes.

MCP clients must support each entry's listed transport and perform their own
authorization. The current Zati connection path supports Streamable HTTP; SSE
entries require a client that supports legacy SSE. No entry is thereby
qualified in Zati or guaranteed to work in every MCP client.

## Recommendations and featured qualification

`IsFeatured` requires operator selection, an exact server version, an actual
versioned Zati client, a useful passed workflow, matching SHA-256 tool/schema/skill
pins, a canonical SHA-256 descriptor identity pin, a qualified linked skill source, a check in the past and a future expiry.
`DescriptorDigest` covers the complete descriptor except qualification records and
operator/derived flags, so changing an endpoint, package, account setup or skill
source invalidates old evidence. Fixtures in tests never qualify actual committed entries. Imports never populate
qualification evidence. A production feature badge requires real approved
provider/client workflow evidence and a reviewed operator metadata change.

`ValidatePrivateRecord` permits private URLs and local package descriptors for
workspace-owned custom metadata. It does not publish these into the default
snapshot, and still prohibits credential-bearing URLs. Tenant authorization and
CSRF controls belong to the hosted workspace API, not this public metadata
library.

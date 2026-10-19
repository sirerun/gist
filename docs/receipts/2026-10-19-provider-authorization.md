# Hosted provider authorization: first release

The owner requested direct provider MCP and API setup information in hosted Gist,
with Zati as the consumer. The scope covers the complete app inventory, not only
the previously admitted MCP subset. Gist supplies reference metadata; clients
own credentials, OAuth registration/consent, connections and execution. A shared
MCP URL is normally the entry point for standard client-side OAuth discovery.
Provider notes document exceptions; missing notes do not block that protocol.

## Delivered and observed

Gist PR130 source `3dd5c843e4e15aa6985fc7279bf8c363f4eb5c78` landed as
`6d11b47014dbd5fe15b306878817c0282c77e62a` with identical source trees.
Foundation PR313 source `46cd7fc74d493d53b24c0970efeaf3d2c56632ad` landed as
`38ac9ec148105c24215765024e8f262319b77158`, also tree-identical.

The source-pinned ARM image build
`gist-registry-image:cc7a7cbc-f848-4b54-9965-57e0605eea02` succeeded. Its digest is
`sha256:53abaaac0dfe4410520faafeec4707d78f3fb653b286289091e5c83372bb4b8c`.
Final-image model/index qualification passed without network access: dimension768,
public index verified,32 app candidates,12.753s elapsed. The existing model,
snapshot and1CPU/4GiB service envelope were retained.

Independent source/build review and the exact saved plan review approved the
image-only update. Plan SHA256:
`6ec11f77539b5e1799bbf0b5cd5c105f8206857ec50c45132792e6f761a815c8`.
The apply completed; ECS task revision25 completed its rollout with one running
task. No database migration, credential, IAM or network change was part of it.

The live public endpoint is:

- `GET /api/directory/authorization?id=mcp%3Acloudflare`
- `GET /api/directory/authorization?app=google-sheets`
- Public MCP `/directory/mcp`, tool `gist_directory_authorization`, exactly one
  of `{"id":"mcp:cloudflare"}` or `{"app":"google-sheets"}`.

Actual REST/MCP response equality passed for both selectors. Cloudflare guidance
was documented; Google Sheets returned its direct HTTP API route;4leads
(`four-leads`) returned an explicit missing-route state; the stale Appwrite docs
route remained unresolved. Sample REST times were0.162/0.089/0.087/0.082s
respectively. Existing descriptor JSON fields were retained and item responses
advertised the guidance URL through an HTTP Link header. Live semantic search
still returned32 candidates with hybrid mode. The published OpenAPI included
the new endpoint.

An actual guest Chrome visit to the Cloudflare connector page displayed the
provider authorization section, account prerequisite, URL/OAuth setup steps and
dated first-party documentation link. No provider account was authorized or
provider tool invoked. This verifies visible guidance, not Zati installation or
connection success.

## Source checks and evidence limits

On existing ARM capacity, Go1.26.1 tests and vet passed for catalog/directory and
hosted app, directorymcp, webui and semantic packages. The bounded check used
2CPU/4GiB with a600s deadline.24 directory JavaScript tests and diff hygiene
passed. Independent researchers cross-reviewed the data against direct
provider/operator documentation, with corrections for authentication modes,
client registration, prerequisites and mismatched endpoint paths.

GitHub Actions did not run: the CI annotation said the account was locked due
to billing. ARM checks and the actual image build are separate fallback evidence,
not claims that GitHub CI passed.

## Coverage is not complete

The first release has82 documented MCP connector guides and45 explicit unresolved
connector guides, plus nine directly documented API apps. All3380 app slugs are
queryable, but that must not be represented as3380 ready connections. The
app-to-connector identity joins yielded53 documented app responses,10 needing
research and3317 with no verified direct route in this first release.

The remaining provider-source collection is active work. Later reviewed batches
must retain exact app identity, primary-source URLs and dates, tenant/private
endpoint inputs, platform-module distinctions and honest inaccessible/missing
source outcomes. Do not infer MCP endpoints from app names, turn documentation
coverage into a connection test, or reuse Make's OAuth application for another
client. The separate quality/adoption evaluation remains open.

Rollback for this release is the previous square-card image
`sha256:419f9b4570c0d62c788b4e407da15d968ead82a8b2d3a621a419a7ea1ccd5267`
through reviewed IaC. The owner-approved square card geometry and rounded
corners are unchanged by this authorization work.

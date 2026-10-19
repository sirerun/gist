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

## Opt-in v2 rollout

Gist PR131 landed as `b6083ffa96ca1e7c7b1fd64a0d32a3e714e5fa5a`,
with the exact independently approved tree
`a6b6b0506d721ef9914f668795bd9952139bdecd`. Foundation PR314 landed
as `fb5bb442d3c0f3e47059eba3a6e39c8311b068f7`, retaining its reviewed tree.
CodeBuild `gist-registry-image:20257897-4d32-4b9d-8abb-512647ef9ada`
succeeded for that source and produced digest
`sha256:45dba9dcce5774ee48f2fe6aa4463ef138bf1d878e40295fc1a346babcc60726`.
The final image passed the no-network model/index smoke: 768 dimensions,
verified public index, 32 matches, 12.87 seconds.

The reviewed saved deployment plan SHA256 was
`3fbe099535ecb2e88e0caa1cd8e79639a69f9ae1cd373b3722448373cde6845c`.
Apply succeeded with only the image/task reference changes. Live v2 responses
passed for Shopify templates, ActiveCampaign endpoint inputs, native MySQL,
Xero, and the AI Tools platform-module classification. REST/MCP responses
matched; default v1 projections omitted v2 fields; invalid MCP versions were
rejected. Existing descriptor guidance, OpenAPI and hybrid search checks also
passed. These are metadata acceptance checks, not provider authorizations.

Use `/api/directory/authorization?app=shopify&version=2`, or call
`gist_directory_authorization` with `{"app":"shopify","version":2}`.
The release contains 159 explicit app entries: 134 documented, 16 needing
research, and nine platform modules. Existing connector joins can contribute
other routes. Full inventory coverage remains open.

PR132 separately landed 60 more reviewed app entries as
`b33d863265164d63af1bef52664d6172927427f6`, with tested tree
`105bf2d8997af2d9e78f5a354c6b948b2418f074`. Its image build and rollout
are separate from the live v2 acceptance above. Source alone is not deployment.

## Expansion to 609 explicit app records

Gist PR136 landed as `4909286e334d0b46435358a6c870dea9a306dcdd`,
with reviewed/tested tree `48201d71e6423c97a74b2606ec3336acbe8c2b96`.
Foundation PR317 landed as `488eb12de3a0ea0a5d70414091be5ef4bfc08a67`.
The source-pinned ARM build passed; final image smoke verified 768 dimensions,
the public index and 32 matches without network access in 12.609 seconds.
Image digest: `sha256:54b7ba099d7abaa78ef87f33ef65c3157968dc77471f5ba44d1ae06466db5c33`.
Saved plan SHA256:
`8ab2edd10b642f5988ad9e5395c9fb8db88eb043ab41128b757943dcfba2aff9`.
Only image/task references changed; ECS task revision29 completed with one
running task and the existing CPU/memory envelope.

Live REST/MCP guidance parity passed, including Magento OAuth 1.0a, prior
API/native routes and invalid-version rejection. Default v1 remained compatible.
Existing descriptor and OpenAPI checks passed; semantic search remained hybrid
with 32 candidates. Guidance responses use Cache-Control: no-store.

The exact source handler evaluated all 3380 apps: 543 documented, six partial,
97 needing research, 11 platform modules and 2723 with no verified route.
These counts include existing connector joins, not only explicit app entries.
They are documentation/metadata coverage, not tested provider connections.
Remaining provider-source collection remains open.

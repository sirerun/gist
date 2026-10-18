# Public directory metadata

`Default()` loads the committed immutable candidate snapshot without network
access. `Search`, `Get` and `DefaultSnapshot` return independent copies. Search
uses term-prefix postings, bounded queries and snapshot/query-bound pagination.
`cmd/gist-directory` validates, searches and exports exact descriptors.

The directory never connects to advertised servers, starts packages, generates
Mint adapters, authenticates provider accounts, or stores credentials. An export
is metadata for client review and admission, not a promise that a client supports
the transport or account flow. Credential names are labels, never values.

## Reproducing the pinned snapshot

```
python3 catalog/directory/intake/build_snapshot.py
```

This reads the committed compressed source inputs and source lock, verifies
SHA-256 projection/input hashes, then deterministically regenerates `snapshot.json` and
`intake/receipt.json`. It does not access the network. The initial official source
is a **bounded 40-page sample**, not the entire registry. The receipt records this
explicitly. The official input is an allowlisted projection: header values, argument
defaults and credential-bearing remote URLs are excluded before storage. Exact
original response and per-page SHA-256 hashes remain in the source lock; raw
upstream values are never committed. Local input is pinned to mcp-registry revision a6c7bd2. Apache-2.0
license is retained in `intake/mcp-registry-LICENSE`. The official Registry code is
MIT; individual publisher metadata remains attributed to its publishers.

To explicitly refresh the sources, a maintainer reviews an update produced by:

```
python3 catalog/directory/intake/build_snapshot.py --fetch \
  --local-registry /path/to/reviewed/mcp-registry/registry.json \
  --max-pages 40 --max-bytes 16777216 --timeout 180
```

Only the fixed official Registry endpoint is fetched. There is one request at a
time, a page/byte/time budget, redirect-host enforcement and repeated-cursor
protection. The importer never fetches advertised endpoint, repository, package,
or specification URLs. Refresh updates local file provenance; update the pinned
local revision in the importer before taking a different local revision.

Only active latest official versions are candidates. Unsupported descriptors and
URLs with credentials, query values, non-HTTPS or private targets are rejected
and identified in the receipt. Public endpoint metadata remains untrusted.
Duplicates are collapsed by stable source identity; alternate servers or API
implementations are intentionally separate entries, not silently conflated.
Neither registry membership nor imported upstream verification flags imply vendor
ownership or tested quality. Local OpenAPI `spec_url` fields have not been fetched
or parsed; some may be documentation pages rather than usable specifications.
Mint generation and schema verification remain client-side qualification work.

## Recommendations and featured qualification

Five Zati essentials are operator-selected setup candidates: Gmail, Calendar,
Drive, GitHub and Brave Search. Gmail points to Google’s official Developer
Preview MCP server; its documentation revision is a descriptor version, not a
measured server version. Calendar and Drive use community OpenAPI descriptions
and Mint. Gmail also retains that route as an alternative. GitHub and Brave
link their vendor-owned MCP projects. These are recommended, **not featured**.
Skill links pin the five screening packages to immutable source and package
digests; their draft bindings and provider workflows remain unqualified.

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

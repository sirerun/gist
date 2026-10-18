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

This reads only `intake/curated.json` and `intake/approved-membership.json`.
The build is offline, deterministic, and hashes both reviewed inputs. The
published set contains one connector per app. Eligibility requires a listing in
**either n8n or Make**, plus a separately reviewed **vendor-official HTTPS MCP
endpoint or raw OpenAPI specification**. Directory membership does not establish
vendor ownership, compatibility or workflow qualification.

Add a connector by reviewing its canonical app identity, first-party directory
listing, vendor documentation, exact endpoint/specification and account setup.
Add the membership and descriptor together; regenerate and review the diff.
The builder rejects missing membership, duplicate apps, unofficial descriptors,
unsafe URLs and qualification claims. It never fetches listings, advertised
servers, packages or specifications. New remote documentation is checked
explicitly during review, outside the offline build.

The initial replacement admits 13 vendor-published MCP connections. Four Zati
setup recommendations remain: Gmail, Google Calendar, Google Drive and GitHub.
Google’s three endpoints are Developer Preview and require enrollment and
agent-enabled OAuth setup. Their linked screening skills remain candidates.
The old community Google OpenAPI alternatives and packages-only Brave route
are absent under the approved policy; the historical screening packages remain
available separately. Exa provides an additional vendor web-search candidate.
None of these connectors has Featured workflow qualification.

The previous MCP Registry sample and Sire Run bulk OpenAPI inputs are retired.
Historical commits and dated publication receipts preserve their provenance;
the active refresh command cannot reintroduce them. App aliases are reviewed
explicitly rather than deduplicated by display name. Make’s publicly exposed
inventory was partial; this release makes no complete union coverage claim.
All 13 admitted apps have verified n8n membership.

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

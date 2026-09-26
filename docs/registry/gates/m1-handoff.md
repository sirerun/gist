# M1 wave-2 handoff

Status: frozen by Q2, 2026-09-25. This is an offline contract and fixture
handoff; it does not grant execution authority or claim a live service.

Wave-2 lanes consume the following immutable inputs:

| Consumer | Frozen inputs | Required boundary |
| --- | --- | --- |
| B — storage/publication | C-owned ports, schemas/OpenAPI/route matrix, lock, catalog records and package inventories | Persist exact IDs, versions, bytes and digests; preserve workspace authorization and publication review/provenance. Do not infer execution from catalog state. |
| S — discovery/resolution | Search/catalog ports, discover/resolve contracts, provider/capability/tool/binding/golden seeds, taxonomy export | Return only authorized candidates; resolve exact versions and complete findings. Taxonomy is filtering/attribution only, never authority. |
| I — identity/OAuth | Identity/policy ports, ADR 005 route-policy and error semantics, runtime handoff fields | Enforce issuer/audience/scope/workspace membership and fail closed; never expose credentials or treat catalog discovery as authorization. |
| T — REST/remote MCP | Frozen OpenAPI, route matrix, wire schemas/fixtures, error envelope, C-owned ports | Preserve canonical URLs, status/body distinctions, byte limits, opaque keys and safe foreign-object 404 behavior. No business-policy copy or execution endpoint. |

The shared contract surface is `contracts/registry/v1/`, locked by
`contracts/registry/v1/lock.json`. The catalog handoff is the three selected
provider routes (Composio aggregator, GitHub MCP Server, Slack Web API), three
capabilities, three tools, three bindings, and passing offline golden fixtures.
The package handoff includes the `asset-skill` and `multi-capability-skill`
inventories with detached package digests; both package-import directions remain
runtime-owned and must be proven by the consumer runtime.

Taxonomy consumers must preserve the exported edition, APQC attribution and
`apqc_ref` on responses/copies. Runtime consumers must preserve the exact
skill, resolution, capability, effect, connection, and receipt references in
`docs/registry/runtime-contracts.md`; discovery, retrieval, and resolution do
not constitute execution authority.

Q2 evidence: `docs/registry/gates/m1.json`. Any contract or port byte change
requires a C-owned amendment, refreshed lock, and renewed Q handoff before
wave-2 implementation proceeds.

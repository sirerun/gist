# Remote MCP directory expansion

Date: 2026-10-18  
Source base: `e93563051bc2865a0a082f8b28bba7a15c020a12`  
Policy: [ADR-025](../adr/025-reachable-remote-mcp-admission.md)

## Result

The snapshot contains 127 remote MCP entries: 24 vendor-verified routes and 103
routes whose operator Gist has not independently verified. The 103 new app
identities came from Make's published verified remote MCP server list; 15
existing app identities were also linked to that list. The independent LinkedIn
Ads Library route remains operated by AdPeekly and is not represented as a
LinkedIn service.

The Make page contained 140 rows. The frozen inventory preserves its 132 literal
HTTPS candidate routes and source HTML SHA-256
`b8daac2be0f5d0729634d028d19b9232f80127a64d6b23988e6972ac203804d2`. The
candidate sweep found 117 responding endpoint/transport pairs; 14 did not
respond or resolve. The final catalog has 23 n8n app listings, four Make
integration-directory listings, and 118 Make verified-server listings. The full
n8n and Make app catalogues were not exhaustively searched for every vendor MCP
route. The snapshot retains four Zati setup recommendations and zero Featured
entries.

## Endpoint checks

The final
[`endpoint-checks.json`](../../catalog/directory/intake/endpoint-checks.json)
checked 126 unique remote URL/transport pairs on 2026-10-18 04:06:59Z: 34
completed an MCP initialize handshake, 79 returned HTTP 401/403 authentication
challenges, 12 returned other accepted HTTP responses, and one returned HTTP
402 payment-required. Responses other than the initialize handshake establish
reachability only. The probe sent no credentials and made no `tools/list` or
`tools/call` request. No provider account, Zati workflow or authenticated client
session was tested.

The 14 failed Make candidates were 13 HTTP 410 routes and one unresolved DNS
name; those routes are not published. The Make inventory is
[`make-verified-mcp.json`](../../catalog/directory/intake/make-verified-mcp.json),
SHA-256 `c5d8945cac0c654142979e2a99d9314492e4392cc546d5f7b0e9a325ad1cfcf0`.
The 117-response/14-failure intake probe is
[`make-candidate-checks.json`](../../catalog/directory/intake/make-candidate-checks.json),
SHA-256 `899c10f1f2976cfeb0f280ecc7a8eeaaaad3ddce6ddf001dbe973739aba54045`.
The final endpoint report SHA-256 is
`d35de0eb5963fb646f4394859eff6ab35e6b10279693048c042f28377110a923`.

The Make listing's source HTML and extracted inventory digests are embedded in
the immutable snapshot's `sources` metadata.

The deterministic snapshot revision is
`sha256:4eac7551bc0d7bf1bdcbfff553e4cf78bb6acf8410fd0bb9b724515c11a125e7`.
The curated input SHA-256 is
`13881bedf392f29d9deda81854dbe4692ed00e207c3db4040d76694d67749f55`; the
membership input SHA-256 is
`672ede4c1a9a1824ecd00b6796cd072220030cc4fe72b4f6dca58576c3fd6a03`.

## Source and plan checks

The intake and endpoint-classification suite passed 18 Python tests. The
catalog passed `go test -race ./catalog/directory`; the hosted directory handler,
web UI and app packages passed their Go tests; the browser JavaScript suite
passed all four tests. Offline snapshot regeneration accepted 127 entries, 126
checked route/transport pairs and zero network requests. The Make inventory
extractor reproduced the frozen rows and source HTML digest; re-running the
admission step was idempotent.

The updated 577-task plan passed the pinned Wazi reader, schema and semantic
conformance gate (contract 0.0.1, digest
`sha256:7582512f122d2f2a9c4461facc7541c9887053f137260d6ebe9c6dea611d039d`).
Plan source digest: `sha256:843082f6405d854680baaef8d06476be9bda1d2bfd72a5fd610ffa2e76ac4784`.
The local verifier passed all 60 owning contract fixtures (21 valid, 39
invalid); the result is unauthenticated conformance evidence, not plan
execution or runtime admission.

## Scope and limits

The directory describes connections; each MCP client decides whether to
connect and performs its own authorization. URL response and directory listing
do not prove safety, tool behavior, interoperability with a particular client,
or a useful workflow. SSE routes require an SSE-capable client; Zati currently
supports Streamable HTTP only. The independent LinkedIn Ads Library route
requires an AdPeekly API key. This candidate does not change database schema or
require a table reset: the published snapshot is an immutable JSON asset with
stable string IDs, not an auto-increment registry table.

Source tests, code review, merge, image publication, deployment and live-site
acceptance are separate stages and are recorded as they occur.

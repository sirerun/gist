# ADR-025: Admit responding remote MCP routes from either automation directory

- Status: Accepted
- Date: 2026-10-18
- Supersedes: the prospective vendor-only admission rule in ADR-024

## Context

The public Gist directory should expose useful MCP connections discoverable
through either n8n or Make. Requiring every route to be operated by the app
vendor would exclude independent servers that can connect through ordinary MCP
clients. Admission must still make ownership and evidence limits clear.

## Decision

An app is eligible when it has a reviewed listing in either n8n or Make and an
exact remote MCP endpoint that passes the bounded endpoint check. Make's
published verified remote MCP server list is an eligible Make listing source.
Vendor-operated routes carry vendor documentation and an explicit ownership
marker. Other routes may be admitted from Make's server list or a versioned MCP
Registry listing; the catalog labels these “Operator not verified by Gist” and
links the source list. Listing in an automation directory does not establish
vendor ownership or endorsement by Gist.

The release intake checker sends an unauthenticated request to each unique
HTTPS route. Streamable HTTP routes receive only `initialize`; SSE routes receive
a bounded GET. It pins a public IPv4 address, verifies TLS, does not follow
redirects, and sends no credentials. HTTP 200 with an MCP initialize result
records a successful handshake. HTTP 401 or 403 records an authentication
challenge; HTTP 402 records payment-required; other accepted responses record
reachability only. None proves an authenticated session or successful tool use.
HTTP 410, unresolved DNS targets, timeouts and other unaccepted responses fail
intake. The check does not list or call tools.

The check is intentionally a low bar for connection discovery. It does not
claim safe tools, compatibility with every client, provider account access,
workflow usefulness, or qualification in Zati. Directory membership plus this
check is never enough for a Featured badge. Those claims require their separate
evidence gate.

## Consequences

Gist can include MCP servers from Make's verified-server list and can represent
auth- or payment-gated endpoints without pretending an unauthenticated probe
established a full MCP session. Every public entry exposes its source,
endpoint-check result and date. Routes without first-party ownership evidence
remain labeled operator-unverified. A current snapshot of the Make verified
server list does not amount to complete coverage of the full n8n or Make app
catalogues, nor of vendor MCP servers omitted from that list.

The public directory remains descriptive metadata. Clients choose whether to
connect and authorize accounts. Gist does not proxy endpoint traffic, execute
tools, or custody provider credentials as a result of this admission decision.

# Official Gmail MCP preflight — 2026-10-17

This is authenticated provider setup and metadata evidence. It does not qualify
a Featured integration, prove a Zati inbox workflow or establish installed GUI
acceptance.

The owner confirmed Google Developer Preview/MCP enablement and authorized one
bounded read-only Gmail qualification window, at most 20 read-only tool calls
and 20 model requests with a one-dollar total ceiling. The recorded live window
is 2026-10-18T01:57:45Z–02:57:45Z; expiry stops live calls, not source work.

A reviewed loopback PKCE/state flow obtained only `gmail.readonly`, online, with
no refresh token. The first callback rejected Google's optional issuer metadata
before exchange; an exact-issuer correction passed independent review and ten
synthetic callback checks before the successful retry. The grant remains in
protected local test custody. No token, code, mailbox identity, private path or
mail content is included here.

Three metadata-only HTTPS requests to the fixed official endpoint
`https://gmailmcp.googleapis.com/mcp/v1` negotiated `2025-11-25`, a protocol the
maintained Zati client supports, and returned 23 tool descriptors on one page.
Actual tools include `search_threads`, `get_message` and `get_thread`. These are
observed names, not assumptions from similarly named tools or documentation.

Capture checksum: `sha256:d21f6ab20413fd826c72833174539574e61dbbe8630fc7be301ecd83d0e12ec6`.
The encoding is sorted compact UTF-8 JSON with non-ASCII characters preserved;
it is not RFC 8785 JCS and is not interchangeable with Zati runtime contract
hashes. The schema inspector permitted only initialize/initialized/tools-list;
its eight synthetic tests and independent exact-hash review passed.

A separately reviewed, durably budget-counted one-shot projection check then
made exactly one `search_threads` call, outside Zati, with a fixed unread inbox
query, metadata-only view and three-thread limit. Schema pins were verified
before send. The successful response contained valid date/ID/label fields,
including optional `internalDate` in this sample; it contained no subject,
snippet, body, HTML or raw message fields. Only aggregate field-presence evidence
was retained. One sample does not make optional fields universally required.
There were zero message-body read calls and zero model calls or paid model
spending at this checkpoint.

Zati's maintained message list/read composition cannot consume these wire
schemas by identity. The reviewed follow-up design is a Zati-owned qualified
virtual adapter sharing the existing MCP profile, credential and session. It
preserves raw tool metadata and conservative effects; exact reviewed conversion
code/configuration and schema pins establish the distinct canonical read tools.
Metadata-only search, explicit UTC message-Date subset, partial coverage,
configuration-bound identity, raw/canonical result provenance and final-context
limitations are required. Source implementation, checks, independent review and
landed delivery remain T-DIR.31–34; actual client/provider/model qualification
remains T-DIR.14/16. Gist owns catalog/package/resolution, not execution.

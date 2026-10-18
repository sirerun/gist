# Five read-only workflow candidates

Start by choosing a directory entry for a useful task. A listing tells you where
to investigate; it does not connect an account, install a server or make a skill
runnable. These historical packages remain available independently of the public connector
set. The approved vendor-only catalog reset removes the former community Google
OpenAPI alternatives and Brave package listing; Calendar and Drive now use
Google preview MCP endpoints. None of these changes qualifies a binding. The five packages below are **screening candidates**, not Featured
workflows. They have not been run against providers or evaluated with a model.

| Candidate package at `0.1.0` | Intended result | Maximum work per run |
| --- | --- | --- |
| `gist/essentials/inbox-briefing` | Recent unread-mail briefing with message references | One list and 20 reads on one mailbox |
| `gist/essentials/calendar-briefing` | Dated agenda with timezone and possible overlaps | Two lists, 50 events, requested window at most seven days |
| `gist/essentials/file-discovery` | Ranked file metadata and snippets with source references | Two searches and five metadata reads; 20 search results; no content downloads |
| `gist/essentials/github-pr-review` | Evidence-backed review of one public pull request | Metadata and diff reads; 20 files and 200 KiB diff text; no code execution |
| `gist/essentials/web-research` | Focused answer with dated public-source citations | Two searches and five page reads; 100 KiB aggregate extracted text |

All candidates have a 120-second total budget, zero automatic retries, explicit
partial-result reporting and no assumed provider spending allowance. The client
must enforce these limits. Text instructions and metadata do not implement
runtime enforcement.

## What you can do now

Browse the public directory and inspect the selected entry's provenance,
transport, account requirements and setup links. Exporting a descriptor saves
metadata for client review. It does not activate the integration.

Read these candidate sources under `catalog/skills/essentials/sources/` to review
the intended workflow and its complete output template. Go callers can inspect
`essentials.List()`, `essentials.Files(key)` or `essentials.Archive(key)`. Each
archive includes its screening manifest, instructions, draft requirements and
result template with a complete inventory. Inspecting or exporting these bytes
does not admit them into a workspace or cause network/provider activity.

The existing `catalog/skills/inboxbriefing` and `inboxcomposition` packages retain
their own immutable versions and contracts. These new candidates are a separate
source collection; they do not silently upgrade an existing admitted inbox skill.

## Before running a candidate in Zati

1. Choose one account and a provider implementation. Review actual vendor setup,
   permissions, costs and data access. Keep API keys and OAuth tokens in the
   client's credential controls, never a Gist descriptor or skill file.
2. Check the current client transport. Zati's current remote-server admission path
   supports Streamable HTTP with bearer or no authentication. It does not turn a
   directory URL into an OAuth account flow, run a stdio package, or implement SSE
   merely because the catalog describes one. Use another qualified client when
   its supported transport/account flow is needed; do not claim Zati support from
   a catalog entry.
3. Read `assets/requirements.json`. Every `draft/*@0.1.0` capability is a proposed
   semantic contract with `binding_status=not_implemented`. These files are not
   provider tool schemas or executable compositions. Design the concrete contract
   and implement an explicit mapping to the selected server's real tool schemas
   before attempting use. A similarly named tool is insufficient.
4. Review read-only effects, same-account constraints, input/output validation,
   truncation semantics, network destinations, cancellation and spending limits.
   Admit an exact package and current bindings through the client's normal review
   controls. The candidate must stop if any required binding is missing.
5. Run only an authorized bounded test, inspect the actual result and record all
   limitations. Source tests and synthetic fixtures are not provider qualification.
   Promotion to Featured requires the separate evidence described below.

No new connection, model call or provider test is authorized merely by this guide.

## Choosing the first implementation

**Mail:** Google’s [official Gmail MCP server](https://developers.google.com/workspace/gmail/api/guides/configure-mcp-server)
is available in Developer Preview at `https://gmailmcp.googleapis.com/mcp/v1`.
It requires enrollment, enabled Gmail/Gmail MCP APIs and an agent-enabled OAuth
client. Use only `gmail.readonly` for this workflow. An authenticated metadata
check on 2026-10-17 negotiated MCP `2025-11-25` and found `search_threads`,
`get_thread` and `get_message` among 23 descriptors. Search is thread-based;
individual message retrieval is available. These actual schemas still do not
already implement the message-oriented screening requirements below. Admit
only the needed read tools and implement a separately reviewed caller-owned
conversion before execution; the server also advertises write tools even with
a read-only grant. This metadata check did not read mail or run a model. The catalog version
identifies reviewed preview documentation, not a measured server version.

**Calendar, files and alternative mail:** these directory entries use community
OpenAPI descriptions and Mint. They require actual schema inspection, adapter
generation and Google account authorization in the client.
An OAuth client ID/secret identifies an application; it is not mailbox, calendar
or Drive authorization. A Mint stdio adapter also needs an execution host and
compatible client transport. These paths are not ready-to-run Zati integrations.

**GitHub:** the vendor's MCP implementation has a remote Streamable HTTP endpoint.
Review its [current setup and read-only configuration](https://github.com/github/github-mcp-server)
and choose an account path actually supported by the client. The candidate is
restricted to a public pull request and does not submit a review, merge, push or
execute repository content. An MCP connection alone does not provide a mapping
for the draft GitHub requirements.

**Web research:** Brave's [vendor-maintained MCP package](https://github.com/brave/brave-search-mcp-server)
requires a Brave API key. Its package/transport setup and any account charges
belong to the client. A search service may not supply the separately required
page-read capability; provide a reviewed public-page reader and network policy,
or stop with that prerequisite. Search snippets alone do not establish full-page
claims. Never expose an unauthenticated locally hosted server publicly as a
shortcut to remote client admission.

## Draft capability references

The exact version for every reference below is `0.1.0`; all are draft and have no
implemented binding in this candidate collection.

| Workflow | Draft capability IDs |
| --- | --- |
| Inbox | `draft/mail/messages/list`, `draft/mail/message/read` |
| Calendar | `draft/calendar/events/list` |
| Files | `draft/files/search`, `draft/files/metadata/read` |
| GitHub | `draft/github/pull-request/read`, `draft/github/pull-request/diff/read` |
| Web | `draft/web/search`, `draft/web/page/read` |

These names express intended semantics. They do not claim an upstream server
exports tools with these names. Contract review must preserve each workflow's
bounded read-only behavior and reject tools that also write, send or mutate state.

## Earning a Featured badge

Record a real useful workflow in an exact Zati client version, on an explicitly
authorized account, with the exact server/descriptor version and package/skill
source. Retain dated results and limitations, canonical descriptor identity,
tool/schema/skill SHA-256 pins, and an expiry date. Credentials and private
provider contents do not belong in public qualification evidence.

A maintainer independently reviews the evidence and explicitly selects the entry
for featuring. `directory.IsFeatured` checks the required pins and current expiry;
upstream registry flags, source publication, passing archive tests, recommendations
and the presence of this guide cannot produce the badge. Until then these remain
honest, reusable workflow candidates with documented prerequisites.

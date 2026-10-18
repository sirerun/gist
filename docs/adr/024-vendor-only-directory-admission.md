# ADR024: Vendor-only app directory admission

Status: Accepted owner direction, 2026-10-17. Supersedes ADR023's bulk public
population and alternate-implementation policy; other boundaries remain.

Public entries must represent apps listed in **either n8n or Make**, deduplicated
by reviewed app identity, with a separately verified **vendor-official HTTPS MCP
endpoint or raw OpenAPI specification**. Listing membership does not establish
vendor ownership. Community implementations, packages without a published remote
endpoint and documentation pages pretending to be raw specifications are excluded.

The active snapshot builder reads only reviewed descriptors and explicit app
membership records, hashes both, and works offline. Adding a row requires review
of app identity, canonical directory listing, vendor documentation, literal route
and account prerequisites. URL checks are syntactic; source review establishes
ownership. Neither published docs nor this admission proves runtime compatibility.
Future expansion uses this same gate; no full n8n/Make coverage is claimed.

Keep stable IDs for surviving connectors. New transport implementations receive
new IDs: Google Calendar/Drive MCP does not reuse the old OpenAPI identity.
Different apps may share one vendor endpoint, as Jira and Confluence do.
One admitted route per app avoids duplicate cards; aliases are reviewed rather
than inferred from matching names. Private custom connectors remain unaffected.

Public entries are embedded snapshot records with string IDs, not SQL rows with
auto-increment keys. Reset the public snapshot without dropping database tables.
Preserve private UUID entries and personal bookmarks. Removed public bookmarks
remain removable unavailable records. Retain the merged card icon design.

Google preview routes require enrollment and caller-owned authorization. All
execution, credentials, tool admission and provider charges remain client-owned.
The Featured standard stays unchanged: an actual dated Zati workflow, exact
version/digest evidence, expiry and explicit selection. This reset qualifies no
provider workflow and grants no new model or mailbox calls.

Release current main only after its existing additive bookmark migration012 and
minimal runtime grants pass database checks and are applied through the reviewed
migration/IaC release path. Verify the source-pinned image, healthy rollout,
public API and desktop/mobile cards; distinguish these from authenticated owner
acceptance. Keep historical bulk receipts as history, not current coverage claims.

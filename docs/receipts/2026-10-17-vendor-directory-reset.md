# Vendor-only public directory reset — 2026-10-17

Status: source implementation and qualified Linux checks pass. Final independent
evidence review, guarded merge, production migration/grants, image deployment and
live acceptance remain separate gates. This receipt does not declare the reset deployed.

## Settled scope

The owner selected the deduplicated union of apps listed on
[n8n](https://n8n.io/integrations/) OR [Make](https://www.make.com/integrations),
then restricted admitted public routes to vendor-official HTTPS MCP endpoints
or the vendor's raw public OpenAPI specifications. App membership is necessary
but does not establish vendor ownership, a callable route or qualification.
Community packages, generated wrappers and bulk registry membership alone are
not admitted public routes. One admitted route per app is selected; explicit service aliases deduplicate
app identity without equating contracts. Different apps may share a vendor route.

This prospective policy supersedes earlier broad public bulk intake. The dated
6,495-entry snapshot and its release/check receipts remain historical evidence;
no historical task completion or immutable fixture is erased. The reset admits
an explicitly evidenced initial subset. It does not claim a complete export of
either app directory, and does not need a complete inventory to verify known
individual members. The replacement admits 13 vendor MCP connectors, four recommended and zero
Featured; count and digests are bound to checked source below.

Private custom descriptors and personal bookmark identities/rows must remain
intact. Existing merged icon/card changes (PR97) and personal bookmarks (PR99)
are retained; source landing is not deployment. The current reset worktree
starts at eb1a9f6e5a87b934cab87a19d70add48bde9e827. Existing
`012_personal_bookmarks.sql` and its explicit minimal restricted runtime grants
still require version-bound upgrade/fresh PostgreSQL qualification and production
ledger/grant preflight before image activation. Do not rewrite an old migration,
replace private UUIDs or silently discard bookmarks for removed public entries.

## Dated source discovery

On 2026-10-17 Pacific time (2026-10-18 UTC), n8n's public first-party listing
returned 2,367 node/editorial cards in its embedded page data. The intake receipt
records 1,326 community and 1,041 non-community rows, including 79 core and 409
trigger rows; these are not disjoint app counts and must not be added or treated
as a deduplicated vendor inventory. App-level membership needs explicit
normalization; node/trigger/core labels do not establish vendor-owned MCP/OpenAPI.
Captured page SHA256:
`62ecd472f68c7bbb6c297af9fddcc9a4ce118c96fe5028b69d04668c028c97e0`.

Make's public listing embeds a machine-readable first page of 30 unique apps,
reports total 3,380 and `hasMore: true`, with both verified/community gallery
filters enabled. The authoritative sitemap index advertised by robots returned
HTTP403 on the fourth direct public request, so further Make requests stopped.
The general sitemap had 973 URLs and no app listing URLs. The 30-row extraction
is explicitly incomplete; no supported complete API enumeration was established,
and no alternate identity/session/proxy or restricted route bypass was attempted.
Captured page SHA256:
`68138b60420c00388a6b35599cc53f3393cda76b87f8dc6b047eb1e2c1cfd22f`.
Deterministic identity-only partial dataset SHA256:
`2cb183585487ea5fdad8d0a3c5bf7d8bfe77c86d3b662029023d9a4418011d53`.

Make app IDs and canonical listing URLs distinguish apps from modules/actions,
but the captured item metadata does not establish publisher ownership. Make
review/maintenance and n8n built-in status must not be substituted for independent
vendor ownership evidence. Normalization retains exact source IDs/names/URLs;
brand changes become reviewed aliases, not lost Gist bookmarks. An app listing
is not an MCP endpoint or raw OpenAPI document: route, publisher, authentication,
transport and later execution admission remain separate evidence.

Discovery imported no descriptive prose, code packages or icons. Existing icon
assets remain protected. No content/image redistribution license was established
from the inspected Make page/robots/sitemap; robots access is not a reuse license.
Source provenance and primary vendor references accompany each admitted route.
No account credentials, supplied private endpoint or provider/model execution was
accessed by this reset intake.

## Plan and verification evidence

`docs/plan.md` preserves every original checkbox row byte-for-byte and adds
T-DIR.40-54: requirements, design, snapshot implementation, migration012/grants
qualification, source and API/browser checks, independent aggregate review,
guarded merge, landed verification, immutable image release, reviewed recovery/
deployment preflight, SQL/grant apply, image deployment, live acceptance and
operations handoff. At initial planning the later delivery gates were open. Requirements T-DIR.40
records the actual owner decision; it is not authenticated runtime evidence.

Full Wazi plan conformance passed on the authored reset plan: 568 tasks, actual
Markdown reader compatible, pinned offline schema valid and owning semantic
validator valid. Source digest:
`43dc34e7ffc5a07939c188631071ca4a83a90a0db5a5def925f73599db7d742c`.
Contract 0.0.1 digest:
`7582512f122d2f2a9c4461facc7541c9887053f137260d6ebe9c6dea611d039d`.
Reader digest:
`0b1292cbe93af31531647f3e2ae28c37f12decbe89023192729e010d6d7e6d67`.
Repair binary digest:
`0181d413bbe6b07a2c8c189fe9360e956c7618cd983380c420d1b8a8d6396cb9`.
Verifier digest:
`b8af84a5ba16610d48fa38c12365ddf639ed7fd72e828d059e58811c2fef870c`.
That exact verifier passes all 60 owning contract fixtures (21 valid/39 invalid).
This is authored-plan conformance, with authorityAuthenticated false; it is not
application source verification, task execution, provider qualification or deploy.

Reproduction uses the existing pinned Wazi checkout/binaries and the plan skill's
read-only `scripts/validate_plan.py docs/plan.md`, with plan ID `gist:delivery`,
authority ID `github:sirerun/gist` and revision `vendor-directory-reset-r1`.
Exact expanded commands and machine-readable receipts are retained in the private
engineering checks; portable source does not expose local machine paths.

## Checked source

Implementation/check source: `9cb0c02354b14bc7d90084e265a51a09ae58dc96`,
based on `eb1a9f6e5a87b934cab87a19d70add48bde9e827` (merged icons/bookmarks).
Archive integrity SHA256:
`5ab92328c477616f2713b4ae1840cb5b95659ea36f9486c526d6d858e1def369`.
Snapshot: `sha256:8ee6929d7a6af04ecf0d33e9a0a86dd0d61d81ab56f1d495fe196c2eed571822`.
Curated/membership SHA256 hashes are committed in `catalog/directory/intake/receipt.json`.
Independent source review checked all 13 exact n8n memberships against the captured
inventory and vendor route/auth documentation separately. Its Exa identity finding
was fixed: the existing ID survives. Gmail/GitHub/Linear IDs also survive; the new
Calendar/Drive transport routes do not reuse old OpenAPI IDs.

Qualified bounded Linux checks pass with no provider/model calls:

- Eleven Python admission, duplicate/unsafe input and deterministic reproduction tests.
- Directory and CLI race tests; committed snapshot validation.
- Actual app HTTP/PostgreSQL integration race tests (46.114s), including saved
  pagination, unavailable records, subject/tenant isolation, current membership,
  CSRF, durable reads, private entries and restricted-role behavior.
- Fresh/upgrade migration and restricted-role integration race tests (7.529s).
- Four shipped JavaScript behavior tests; all hosted vet and build checks.
- Earlier broad hosted run passed all other internal/command packages; its single
  bookmark paging failure was corrected and the affected app/migration suite rerun.

The first Linux run exposed a Mac-only temporary path in the new Python test;
standard tempfile/TMPDIR handling fixed it. The second exposed an old pagination
test assuming 50 public entries. It now tests 50 saved entries including retired
IDs, preserving pagination coverage without inflating or weakening the catalog.
Unchanged icon/card and bookmark browser evidence is reused from the merged
changes; updated production desktop/mobile checks are still T-DIR.53.

GitHub hosted checks were not executed: the current main check annotation confirms
the account is locked for billing. The existing authorized qualified-local fallback
is used; no billing, runner or branch-protection changes were made.

T-DIR.41-45 record design, implementation and source/database checks. Production
SQL has not been changed by these tests. Read-only production preflight observes
migration ledger1-11 and no bookmarks relation through the older migrator. Existing
available recovery snapshot `gist-pre-bookmarks-20261017` predates this release.
Migration012, minimal grants and actual runtime verification must precede image
activation. No tables are dropped, no private UUIDs reset and no stored user
preferences deleted. Later review/merge/deployment/live gates remain explicit.

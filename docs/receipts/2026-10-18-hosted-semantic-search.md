# Hosted semantic discovery — 2026-10-18

The owner requested server-side EmbeddingGemma2 on gist.sire.run and the same
search in `/library?q=find+leads+for+my+startup`. This named scope resumes only
public semantic discovery. The local experiment and T-PREP.9-13 remain separate
quality/adoption work; no whole-task, token/cost or private search claim follows.

## Implementation contract

The existing ECS registry owns a private loopback llama-server child. It stages
and verifies exact runtime/model bytes, observes the trusted child's canonical
bind event before sending its private bearer key, verifies readiness, and
stops/reaps the child on cancellation/failure. Runtime downloads are prohibited.
Deployment supplies pinned public native source and BF16 GGUF already qualified
in the retained native trial. Query/document prefixes, pooling/projection and
768 dimensions use the existing Go adapter unchanged.

One service indexes public app names/descriptions/categories and public connector
names/descriptions/tags/linked skill titles. It reads neither private workspaces
nor user mail, and does not inspect remote tool servers. It rebuilds in memory
for the exact public corpus/model generation; catalog updates require the normal
immutable release. Exact metadata, schemas, permissions and execution remain
outside similarity scoring. Independent skill or per-tool-schema semantic
indexes are not claimed by this first public slice.

Search combines lexical candidates with up to32 semantic candidates above
cosine0.30 using RRF60 and deterministic ID ties. Kind/tag/category eligibility
is enforced before the semantic cap and again during paging. Directory cursors
bind query, filters, corpus/model/ranking generation and actual candidate order.
App offsets retain the existing browse contract. One inference slot, three-second
query ceiling and256-entry/five-minute hashed-query cache bound anonymous traffic.
Busy, unavailable, failed or warming inference returns lexical fallback; existing
semantic cursors are rejected if their ranking cannot be reproduced. No raw query
or model text is logged. Startup is bounded120seconds; initial indexing has a
ten-minute ceiling. A failed runtime requires a normal service restart to recover;
it does not trigger an unbounded restart or model-download loop.

The Library app view, typed MCP/OpenAPI pages, signed-in bookmarked public browse,
directory REST and directory MCP share the service. Public HTTP responses report
`X-Gist-Search-Mode: hybrid` or `lexical_fallback` for nonempty queries, preserving
the existing JSON body for strict older clients. Empty browsing
retains ordinary catalog ordering. Exact get/export operations are unchanged.

## Checks so far

- Focused directory tests passed, including ranked filters and generation-bound
  pagination; full existing directory suite passed before later additive tests.
- New semantic tests passed: nonlexical match, app parity, vector reuse, no-match,
  filters, invalid/unready input, busy/dead runtime and lexical fallback.
- Runtime lifecycle tests passed, including staged integrity, strict bind proof,
  no request to an unowned listener, readiness refusal and child cleanup.
- Actual REST/MCP/Library app handler test passed with deterministic fixture
  vectors. This is source-path evidence, not real-model relevance evidence.
- Existing browser JavaScript suite passed23/23. No frontend routing change was
  needed: the Library already sends the URL query to the directory app endpoint.
- Frozen registry contract check passed. Wazi plan gate passed584tasks with
  reader/schema/semantic validity and authorityAuthenticated:false; owning
  verifier fixtures passed60/60 against contract0.0.1.

Independent source review, final source checks, immutable ARM image build,
reviewed infrastructure apply and real hosted/browser query acceptance are
pending. Mac load and internal-disk pressure constrain heavy local checks;
new task artifacts and temporary build files use external storage. Existing
CodeBuild will run focused source tests/vet before publishing the image.

Compatibility correction before deployment: the existing Zati directory reader
strictly rejects unknown JSON fields. Retrieval mode therefore stays internal
and is exposed through an HTTP header rather than an added response-body field.
The real REST/MCP handler regression checks both header parity and unchanged
JSON shape; no client dependency update is required to receive semantic results.

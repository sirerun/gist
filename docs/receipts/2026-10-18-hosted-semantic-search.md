# Hosted public semantic discovery — 2026-10-18

Status: live on gist.sire.run, with actual REST/MCP and browser acceptance passed.
The owner authorized hosted EmbeddingGemma 2 and the same search in
`/library?q=find+leads+for+my+startup`. This is public catalog retrieval;
T-PREP.9–13 whole-task quality/adoption work remains separate.

## Server and client contract

Gist's existing ARM64 ECS task owns a verified, private loopback llama-server.
The server verifies staged native/model bytes, proves its own child's bind before
sending its private bearer key, checks authenticated readiness and reaps the
process on shutdown/failure. There are no runtime model downloads or public
embedding ports. Zati and browsers call Gist; they install no model and maintain
no manual copy of the connector catalog.

One service indexes 127 public connector descriptions/tags/linked skill titles
and 3,380 public app descriptions/categories. It reads no private workspace,
mail or live provider schema. Document vectors are generated once during release
preparation and baked into the image. Startup loads that immutable snapshot,
validating exact model/pipeline identity, corpus revision, complete document
IDs/order/text hashes, dimensions and finite unit norms before publication.
Gzip checksum and 32 MiB compressed/128 MiB decoded bounds are checked first.
Catalog or model changes require a regenerated central release artifact.

Queries run on the hosted model with one nonblocking inference slot, an
eight-second ceiling and a 256-entry/five-minute hashed-query vector cache.
Busy, unavailable, invalid or failed inference uses lexical fallback. The semantic service and native supervisor do not log raw
query/model text. Hybrid ranking combines lexical results and up to
32 semantic candidates above cosine 0.30 with RRF60 and deterministic ties.
Kind/tag/category eligibility is enforced before the semantic cap and paging.
Directory cursors bind filters, corpus/model generation and ranked IDs; a
fallback that cannot reproduce an existing semantic cursor rejects it.

The Library, typed public browse, bookmarked public browse, REST and directory
MCP share retrieval. JSON bodies remain compatible with older strict clients;
mode is reported in `X-Gist-Search-Mode: hybrid` or `lexical_fallback`.
Empty browse, exact get/export and execution authority keep their existing
contracts. Private workspace `/mcp`, independent skill search and per-tool-schema
semantic indexing are outside this public slice.

## Integration handoff

- Manual Library test: <https://gist.sire.run/library?q=find+leads+for+my+startup>.
- Connector discovery: `GET https://gist.sire.run/api/directory?q=...`.
- App discovery: `GET https://gist.sire.run/api/directory/apps?q=...`.
- Public MCP: `https://gist.sire.run/directory/mcp`, tool
  `gist_directory_search`, arguments `{"query":"find leads for my startup"}`.
  Use the standard JSON/SSE Accept header.

Existing Zati uses the public REST reader with `DisallowUnknownFields`.
No dependency/model installation is needed for these semantic results; exact
schemas, permissions, connections, budgets and effects remain client-authorized.

## Frozen release artifacts

[Public release](https://github.com/sirerun/gist/releases/tag/public-semantic-index-20261018)
contains only public document vectors/identity hashes and the native binary with
original upstream/bundled dependency notices. It contains no workspace data,
credentials or query vectors. Build-time SHA256 verification authenticates the
vector bytes; corpus/generation hashes alone are not an authenticity check.

| Input | Frozen identity |
|---|---|
| Native source | llama.cpp `620d2225cc436ab0b001e3200e590e3b286ef9ab` |
| Production ARM64 binary | `0778cc0e786e55cf16926645f0d5e87f66ceb2dfa11c005499a8f0b9c014d1e2` |
| Native archive | `0f9a098ddb51bbee8330067dc484a1f458bfa2d516538ea7ed9c1e5059c28435` |
| Official BF16 GGUF revision | `bfcd298762cc34d0357ece5ebdd31791a3a374d8` |
| Model bytes | `68bae29d62fb8c7d23e98d21fd4662753ddd636e6b62b8a70dfe059a9844f216` |
| Public snapshot, 24,723,438 bytes | `99163861a7321a7066b500ad7be077c7c47af2722bd3b305f5b8f55f99e1fd27` |
| Adapter/model version, 768 dimensions | `llamacpp-embeddinggemma2:sha256:8f9c3422bd271e7db2327811ce1267bce8da9675d0f24f8b8dbda8f0d5e4382e` |

The native archive preserves exact bytes extracted from the first qualified
image. Public release assets avoid a permanent dependency on a base image
subject to ECR's existing 20-image retention. Model weights still come from the
original checksum-pinned upstream URL during image build.

## Verification and delivery evidence

- Source: ranked filtering/pagination, semantic-only matches, no-match, invalid
  vectors, busy/dead/unconfigured fallback, verified process ownership/lifecycle
  and shared REST/MCP/Library handler checks passed. Snapshot tests cover strict
  schema, identity/corpus/order/text/vector mismatches, bounds, gzip corruption
  and atomic readiness. Actual ARM checks passed, including the final deadline
  correction (semantic 0.174s; shared app handlers 0.110s).
- Existing frontend JavaScript tests passed 23/23; frozen registry contract checks
  passed. Final Wazi conformance passed all 584 tasks with reader/schema/semantic
  validity, contract 0.0.1 and `authorityAuthenticated:false`. The unchanged
  owning verifier's fixtures passed 60/60; this does not authenticate GitHub
  authority. Plan source digest:
  `sha256:f6584a8656bd9acbdb909255de88cc68241cf6c25299a407f585204f926f15f8`.
- Non-author review approved exact Gist/Foundation candidates, artifact contents,
  native notices and saved infrastructure plans. GitHub Actions could not start
  because the account was billing-locked. The authorized local/ARM verification
  fallback was used; this is not passing GitHub Actions evidence. An optional
  small ARM Foundation source worker exhausted compilation memory before tests;
  the final Foundation tests and vet passed separately on the existing ARM host.
- Full real-model generation used the exact production binary/model on a bounded
  existing ARM worker: 3,507 documents, 846.563 s, 838.9 MiB peak, no swap, eight CPU
  quota, with an 1800 s generation deadline. It returned 32 hybrid app candidates
  for the owner query. Producer b764052 has the same tree as reviewed 4d08ff6.
- Image publication requires `/registry-embedding-check` inside the final ARM64
  image with networking disabled, one CPU, 3 GiB and a 180 s ceiling. It validates
  native query/document vectors, loads the complete baked snapshot and executes
  the owner query through shared retrieval. The snapshot image from Gist dd821fc
  passed in 12.999 s before publication; the final corrected image passed in
  12.778 s (build c952ad1e-c0cc-413b-8d7f-507762619e18).
- Saved snapshot-builder plan SHA256
  `738064712cfd273ed55394a75d48475533d7c6ff79fa2da8132982736c5b06a8`
  changed only CodeBuild's recipe (59 resources unchanged), applied successfully.
  Saved first snapshot-serving plan SHA256
  `f5e8f1e2c14d582b0b53d74edfe5012bf546b55cc785c3c79d465b6fd7de7556`
  changed only the serving image/task reference (58 unchanged), also applied.

### Findings retained from live qualification

The first image (Gist 51116e1, image digest 428604e8…) passed a single query/document
smoke but failed public index warmup. Public traffic retained lexical fallback;
it was not accepted as semantic delivery. Native diagnostics verified batches
1/2/8 with correct usage/count/indices, 768 dimensions and norms. At one CPU,
eight synthetic documents took 17.17 s and the first eight real corpus documents
took 25.00 s. Eight slots offered negligible gain; two CPUs took 12.30 s. These
bounded measurements explain the move away from serving cold indexing. The
initial failure did not expose its exact adapter category.

The snapshot image (Gist dd821fc, digest 575c4fa1…) started task 22 and emitted
`public semantic index ready` at 20:27:44 UTC. However, an uncached owner query
returned lexical fallback after 3.248 s on actual Fargate, hitting the initial
three-second ceiling despite passing the faster build worker. Gist PR 126
(cbcc51d) raises only that ceiling to eight seconds, preserving the same
resources/admission/cache and artifact identities. Live semantic acceptance is
required after this correction.

### Landed provenance

Gist PRs 121/122/123 landed f893382/1fab528/51116e1 (hosted retrieval, strict
response compatibility and image smoke); PR 124 landed b764052 (snapshot load),
PR 125 dd821fc (explicit unconfigured fallback), PR 126 cbcc51d (query deadline).
Foundation PR 307 landed 5aa308b (native producer/resources), PR 308 ba65e5d
(initial activation), PR 309 b6faafc (pinned snapshot/native recipe), PR 310
7a71dd6 (first snapshot activation), PR 311 af06920 (eight-second serving
correction). Each landed tree matched its independently reviewed candidate.

## Final hosted acceptance

The final source is Gist `cbcc51db52176cba94d8cbc7cad56601134cadc3`.
AWS build `gist-registry-image:c952ad1e-c0cc-413b-8d7f-507762619e18` succeeded;
image `sha256:00a05f74a82cc4a4619abb20436a98108e731593660c67044372a9dd9e24dd7a`
passed final-image qualification before publication. Reviewed saved serving plan
SHA256 `37cace5b0f090116654f4bd80ae1a47e573f596e809d5a09b7938b202584d9b9`
applied successfully with only image/task-reference changes and 58 resources
unchanged. ECS task definition 23 completed rollout with one running task on
ARM64, 1 vCPU/4 GiB; the runtime image digest matched the qualified artifact and
its log reported `public semantic index ready` at 20:40:40 UTC.

Live read-only acceptance passed:

- Owner query: mode `hybrid`, 32 app candidates. The first readiness request took
  2.216 s; a cached app request took 0.128 s. Examples include StoreLeads, D7 Lead
  Finder, FindThatLead and UpLead. These are individual observed timings, not
  latency-distribution or relevance-benchmark claims.
- Directory REST returned 32 connector candidates, including HubSpot and ZoomInfo,
  in 0.101 s using the shared cached vector. MCP returned the same ordered IDs,
  mode `hybrid`, in 0.088 s. No tool/provider effect was invoked.
- Gmail search ranked Gmail first (1.568 s). A positive MCP-kind filter returned
  only MCP entries; the tested OpenAPI filter and nonexistent tag returned zero.
- Three concurrent distinct queries produced one hybrid response (2.364 s) and
  two immediate lexical fallbacks (about 0.239 s), observing the bounded busy path.
- The actual Chrome Library page at the exact owner URL displayed `32 matching
  apps`, the expected query and matching app links. Its pre-deployment baseline
  displayed zero; this is visible browser acceptance, not a fixture screenshot.

Existing community/vendor variants can share app display names (Leadfeeder
appeared twice under distinct IDs). Ranking thresholds, deduplication, broader
catalog coverage and whole-task outcomes still require the separate evaluation;
this trial establishes hosted public retrieval and its operational boundaries.

The serving envelope is one vCPU/4 GiB per task, with the existing 1–4 task
scaling policy. No database migration, IAM, network, secret or connector-provider
execution changes were made. Rollback is the prior known lexical image
`sha256:4bb9ef3bfeb44b30f9f6fa7bbea8269659809359cf30873bf5ef1166e80d159c`
with `embeddingEnabled=false`, through reviewed stack configuration. Source,
fixtures, numeric qualification, actual provider behavior, installed image and
browser acceptance are distinct evidence; none establishes whole-task quality,
coverage, token/cost savings or the broader T-PREP.9–13 adoption decision.

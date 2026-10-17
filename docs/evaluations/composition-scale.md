# Composition index scale freeze

Status: T-COMPOSE.15 coordinator-approved freeze; source implementation T-COMPOSE.16 only.
No scale qualification, independent composition cohort or adoption claim follows
from source checks. T-COMPOSE.17 depends on landed Zati migration; T-COMPOSE.18
scores only after its independently reviewed inventory and labels are frozen.

## API and workload

Add `catalog/discovery.ScopedIndex`, using existing lexical normalization/scoring
and `Candidate` summaries. It is derived data, not an artifact store. Exact ref
and capability ID/version/digest maps and inverted term postings contain no tool
schemas, vectors, credentials or mailbox contents. Exact binding candidates are
trusted caller admission records, never inferred from search results.

One index belongs to a complete principal (including policy generation/scopes)
and runtime. Build checks current policy and authorizes entries; requests check
policy before work and before exposure, and reauthorize offered records. Refusals and empty results also pass the final
policy check. A
changed policy or generation produces no candidates. The caller policy gate
must reject revoked/stale policy snapshots, including connection admissions;
it is mandatory and supplies no default allow implementation. The caller must
advance policy generation for connection/admission changes as well as catalog
grants and reject the old scope. New authority scope requires a new scoped
index; content updates never renew authority. Composition still
retrieves exact artifacts and performs its own final authority checks.

Search bounds: 1 KiB query, 32 normalized terms, 4096 examined postings/entries,
64 returned summaries, 256 KiB encoded response. Budget exhaustion returns
`incomplete_budget`, not no-match or readiness. Empty/stopword queries return no
matches. Exact binding lookup obeys composition's 64-candidate maximum and
returns `Complete:false` when exceeded. Cancellation is checked during builds,
updates and searches. Exact selected-skill retrieval bypasses ranking.

Generations publish atomically. Updates analyze changed entries only and copy
index structure; full rebuild and rollback publish new monotonic generations.
Top-level entry/posting maps are copied in O(N) during an update; only changed
entries are reanalyzed and affected posting lists rebuilt. This is not an
affected-only total-cost claim. Rollback does not restore old policy authority.
The index retains current and immediately previous snapshots; in-flight readers
may temporarily retain older snapshots, included in peak memory accounting. Queries detect
concurrent publication before exposure. No result cache is introduced.

Deterministic synthetic sizes: 10,000 and 100,000 total records, 40% tools,
30% skills, 10% capabilities and 20% bindings. Ten isolated principal indexes,
each containing its own disjoint tenth of the catalog; additionally a single
principal index containing the whole size tests worst-case visible scope in a
separate run. These are 100k total logical entries, not 100k per principal;
report per-principal duplication and retained generations in memory accounting.
Exact lookups must remain usable after broad lexical budget exhaustion.
Eighty percent of documents share a common term; unique names and topic buckets
exercise narrow matches. Alternatives use 1, 4, 64 and 65 candidates. Workloads
include exact hit/miss, unique lexical hit, ambiguous common term, no match,
revocation/denial, cancellation, 100-record updates, full rebuild and rollback.
Composition-only schema drift/connection mismatch and complete preparation,
retrieval recall and false readiness require the later independently labeled
cohort; index results alone never establish readiness.

## Frozen target envelope and thresholds

Execution target: qualified DGX Linux worker, 4 CPU, 24 GiB memory, 20 minute
watchdog, existing Go toolchain/cache, no downloads, model calls or paid calls.
Record actual architecture, Go version, kernel, resource limits and source SHA
with results before comparison. If this target is unavailable, do not substitute
hardware silently or claim qualification from source tests.

These are conservative engineering targets, **not measured baselines**:

| Measurement | 10k and 100k target |
|---|---|
| Complete exact/narrow lexical lookup p95 | <=100 ms |
| Broad search, capped with explicit incomplete outcome, p95 | <=250 ms |
| Full 100k generation build | <=120 s |
| 100-entry update publication | <=5 s |
| Peak process RSS (including retained generations) | <=2 GiB |
| Unauthorized exposure / false-ready results | zero |

Report p50/p95 separately, all failed attempts, cold build/first-query and warm
query phases, snapshot encoded projection size, peak process RSS, build/update/
rollback durations, result bytes and budgets. Token counts remain unavailable
unless actually measured; byte estimates are not model tokenizer evidence.
Do not tune thresholds after scores. Existing `discovery.Service` reanalyzes
metadata for returned records; no measured scale baseline exists for it.

Source verification: `go test ./catalog/discovery` and
`go test -race ./catalog/discovery` on the qualified host, followed by the
coordinator's aggregate catalog checks. Reproducible scale runner/commands and
independent labels are frozen in T-COMPOSE.17 before T-COMPOSE.18 scoring; these
source commands are not the scale evaluation.

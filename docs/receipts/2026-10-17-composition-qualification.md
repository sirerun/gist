# Composition and bounded index qualification — 2026-10-17

The new synthetic development qualification passes its frozen targets. It covers
one maintained inbox composition family and separately measures a summary index
at 10,000 and 100,000 total logical records. It does not measure model tokens,
Gmail, Composio, native UI latency or semantic retrieval quality.

## Source, freeze and independent labels

Measured source: `afceb5b770528e56cf8f5fc07c83b7c6d27f8d3a`.
Exact source archive SHA256:
`aa17b879583776d09473b2f1dc9f0e90f69c1072bd9baf00211824f520008e20`.
The 78-file freeze manifest SHA256 is
`a285988ebedc0a2edd681bd81b8e4c7d0f8c1f9e6b2377a502376bc0e3d7390e`.
Independent reviewer `composition_doc_audit` approved that exact pre-score
freeze against base `ab6b6c6` after correction and required source checks.

Coordinator primary labels and non-harness-author `composition_check_route`
secondary labels agree on all 19 readiness outcomes and selected identities.
The [reconciliation](../../tools/evaluation/composition-cohort/reconciliation.md)
retains their provenance. No measured result was used to author labels or tune
thresholds. The old composite pilot, held-out data and model envelopes are
unchanged. All five measured processes completed on their first scored attempt.

## Composition results

All 19 cases match their expected outcome. Four labeled-ready cases returned the
exact full two-tool closure, including immutable candidate/tool pins, skill pin,
connection and account. Fifteen refused readiness. Ready recall is 4/4; outcome
coverage is 19/19; false readiness and private sentinel exposure are zero.

Cases cover missing and wrong-digest capability, account/connection mismatch,
ambiguous alternatives and explicit selection, 64/65 candidate bounds, unsupported
consumer, final revocation, stale authority, task denial, cancellation, complete
context budget, immutable tool/package drift and private candidate exclusion.
The harness calls real `ScopedIndex` and `composition.Service` ports. Synthetic
caller admissions attest canonical structured list/read contracts; they do not
qualify any provider adapter. Bytes are measured; tokenizer counts are unavailable.

## Scale results

Linux ARM64, Go1.26.1, kernel6.17.0-1021-nvidia. Each process had a 4-CPU quota,
GOMAXPROCS4, 24GiB memory cap and 1,200-second watchdog, with offline cached
build dependencies. All services completed and are inactive. Actual process
RSS comes from Linux getrusage, not service memory accounting.

Each run contains 40% tools, 30% skills, 10% capabilities and 20% bindings as
synthetic summaries. Ten-partition runs contain disjoint tenths of the total;
single-partition runs contain the entire catalog. They are not 100,000 records
per private user. Each query class has 100 warm observations; p95 uses nearest
rank. Dataset generation and first per-class queries are recorded separately.

| Total records | Partitions | Build s | Update 100 s | Exact hit p95 ms | Narrow p95 ms | Broad p95 ms | Peak RSS MiB |
| --- | --- | --- | --- | --- | --- | --- | --- |
| 10,000 | 1 | 0.078 | 0.013 | 0.0050 | 0.0084 | 1.192 | 108.7 |
| 10,000 | 10 | 0.079 | 0.006 | 0.0051 | 0.0110 | 0.760 | 103.9 |
| 100,000 | 1 | 0.705 | 0.159 | 0.0030 | 0.0054 | 1.023 | 984.8 |
| 100,000 | 10 | 0.623 | 0.086 | 0.0036 | 0.0074 | 1.700 | 1000.4 |

All frozen latency, build, update and <=2GiB RSS targets pass. Broad queries at
more than4,096 matching postings deliberately return `incomplete_budget` with no
candidates. Their speed is bounded refusal, not complete broad-search retrieval.
The10k/ten-partition run has800 common matches per partition and returns a complete
ranked top64 shortlist. Exact lookups remain available after a separate forced
budget exhaustion. No full-schema scan is used for indexed requests.

The100k single-scope public snapshot projection is31,460,224bytes. It includes
all exact public candidates, scope and generation; it is not internal posting-map
serialization. Peak RSS includes the source dataset, indexes, current/previous
generations and a reader held across updates/rebuild. The large difference between
projection size and RSS matters for deployment sizing. Updates still copy top-level
maps in O(N); these results do not establish constant-cost mutation.

Revocation, reference denial, cross-principal/runtime/policy denial, cancellation,
1/4/64/65 alternative completeness, exact lookup after exhaustion, rebuild and
monotonic rollback pass. A real in-flight reader is refused after generation
publication. Raw cold samples, response bytes, update/rebuild/rollback timings,
generations and every check are retained in the result files.

## Verification and retained corrections

Full root integration/race, vet, builds, contract freeze checks, 12 registry
Python tests and tidy passed at817d084. Four initial report tests and inventory
regeneration also passed. Lint initially found one new unchecked output close;
that was fixed with error propagation. Final runner race tests, six reporter
regressions and exact inventory regeneration pass atafceb5b. Full lint matches
exactly five inherited findings and zero introduced findings; its exit1 is not
reported as a clean lint exit. The unchanged root aggregate remains applicable.

Before scoring, independent inventory audit corrected C16's connection selection
from group keys to requirement keys. Independent freeze review found failed scale
attempts could crash the report; reporting now preserves early/partial failures
with null unavailable metrics and failed checks. Setup errors cannot count as
successful refusal coverage. Initial source/review failures remain recorded;
no scored result or expected outcome was rewritten to make the run pass.

Gist producer/index PR83/84 and Zati consumer PR47 are previously landed. Zati
fc68df88 passed fresh typed full/partial and legacy workflow checks. This scale
run measures the Gist index API, not an installed Zati marketplace or its total
latency. Existing CI billing unavailability uses the established local verification
fallback; protections are unchanged. Release, installation, actual provider
acceptance, semantic adoption and old held-out/model work remain separate gates.

## Reproduction and retained records

The [runner instructions](../../tools/evaluation/cmd/compositionqual/README.md)
describe exact inventory generation, source checks and bounded execution. The
[raw records and report](../../tools/evaluation/composition-cohort/results/)
retain all measured attempts; SHA256SUMS binds every result file. Regenerate the
report from repository root without running providers or measurements:

```sh
python3 tools/evaluation/composition-cohort/report.py \
  --config tools/evaluation/composition-cohort/primary-config.json \
  --labels tools/evaluation/composition-cohort/labels-reconciled.jsonl \
  --inventory tools/evaluation/composition-cohort/inventory.json \
  --cohort tools/evaluation/composition-cohort/results/cohort-attempt-01.jsonl \
  --scale tools/evaluation/composition-cohort/results/scale-10000-1-attempt-01.json \
  --scale tools/evaluation/composition-cohort/results/scale-10000-10-attempt-01.json \
  --scale tools/evaluation/composition-cohort/results/scale-100000-1-attempt-01.json \
  --scale tools/evaluation/composition-cohort/results/scale-100000-10-attempt-01.json
```

Attended engineering route preserved inherited model/effort. Requested batch
ceiling25; this qualification boundary covers .17-.21 and three tracked repair
obligations .39-.41. One harness implementer, coordinator integration/reporting,
independent secondary label/check author and separate non-author reviewer had
bounded roles. Agent token/cost totals and acceleration are unavailable; task
count is not a performance metric. Final candidate review/merge/landed evidence
is recorded in the canonical plan separately from these source-bound scores.

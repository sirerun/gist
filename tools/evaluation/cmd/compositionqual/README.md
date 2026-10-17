# Composition qualification runner

This runner emits inventory and raw observations only. It neither authors labels
nor decides qualification. The coordinator owns the protocols, labels, reviewed
freeze manifest and report. All data and ports are synthetic; no provider or
model is called.

From the repository root, inventory generation is permitted before freeze:

```sh
go run ./tools/evaluation/cmd/compositionqual \
  --mode inventory \
  --out tools/evaluation/composition-cohort/inventory.json
```

This constructs exact artifacts and caller requests without invoking an index or
preparing a composition. Exact artifact bytes are base64, original pins remain
unchanged in drift cases, and runtime controls are explicit. The private case
excludes denied summaries from the caller's visible partition and also denies
exact artifact retrieval.

Source verification (not a scored cohort/scale run):

```sh
go test -race ./tools/evaluation/cmd/compositionqual
```

After independent review of the exact freeze, build the runner once on the
frozen Linux ARM64 Go 1.26.1 host and run each measurement in a separate bounded
process. The external runner supplies 4 CPU, 24 GiB, offline caches and a
20-minute watchdog. `GOMAXPROCS` must not exceed 4. The CLI checks the frozen
OS/architecture/toolchain, uses a 20-minute context deadline, records actual
cgroup limits, and collects process peak RSS via Linux `getrusage`.

```sh
go build -o /tmp/compositionqual ./tools/evaluation/cmd/compositionqual
GOMAXPROCS=4 /tmp/compositionqual --mode cohort \
  --freeze tools/evaluation/composition-cohort/freeze.json \
  --source EXACT_SOURCE_AND_ARCHIVE_ID --out cohort-attempt-01.jsonl
GOMAXPROCS=4 /tmp/compositionqual --mode scale --records 10000 --partitions 10 \
  --freeze tools/evaluation/composition-cohort/freeze.json \
  --source EXACT_SOURCE_AND_ARCHIVE_ID --out scale-10000-10-attempt-01.json
```

The other scale processes use `(10000,1)`, `(100000,10)` and `(100000,1)`.
Measured output files must not already exist, preserving failed attempts.
The SHA256 manifest has `schema_version: gist.composition-freeze/v1` and a `files`
map of repository-relative paths to SHA256 hex digests. Every listed file is
verified before measurement; config and input files must be listed. Manifest
review and authority to execute remain coordinator gates, not inferred from the
presence of a file.

Cohort output is JSONL with `id`, `scenario`, `source`, actual `result`, `error`,
`elapsed_ns`, `response_bytes`, request `counts`, `setup_ns`, `setup_counts` and
`host`. Counts distinguish catalog gets, artifact opens, authorizations, policy
checks, index lookups and composition authority checks. Response size is the
actual Go JSON encoding of `composition.Result`, including a zero result when
preparation returns an error. Setup errors are retained separately in `error`.

Scale output is one JSON object. `build_ns` covers all partition index builds;
`dataset_ns` is separate. `query_classes` holds exactly 100 raw warm observations
per configured class, round-robin across partitions. `cold` holds the first
observation of each class in each partition; earlier classes may already have
touched that index. No percentile or pass threshold is computed here. Query
serialization occurs outside elapsed lookup time and `result_bytes` records its
size. Normal broad queries retain their actual complete/incomplete outcome.
A separate forced-budget probe proves exact lookup still works after exhaustion.

`update_ns` is publication of exactly 100 changed records total, divided among
partitions. `rebuild_ns` and `rollback_ns` cover all partitions. An actual reader
retains its original generation across update and rebuild; exposure must then
fail the generation fence. Peak RSS includes source dataset, all indexes,
current/previous snapshots, and that retained reader. The auxiliary 1/4/64/65
binding-completeness checks are a separate small fixture and do not add entries
to the reported logical scale dataset; their memory is included in process RSS.

`snapshot_projection_bytes` measures a JSON array of all public exact candidates,
with each partition's scope and generation, by enumerating every known reference
outside latency samples. It is not an internal-postings serialization. `checks`
and `check_details` preserve concrete safety outcomes and errors. They are raw
observations for the independent report, not broad provider or semantic claims.

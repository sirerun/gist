# Controlled inbox development results

The admitted run used frozen source `3676aa17e05cf7c813a2000068bc413e99738eb4` and the independently reconciled [closed-menu cohort](../inbox-development/manifest.json). These are development preparation results for 30 explicit synthetic menu requests in **one construction family**, using the existing composite source tool. They are not inbox execution, Zati's distinct two-tool closure, live Gmail, unseen-task coverage or adoption evidence.

Two real offline BF16 embedding jobs supplied256/768-dimensional vectors. Each dimension ran k4/8/16 with floor0.30/RRF60, then separate `source_cold`/`source_warm` CLI processes. All2520records are retained:1800local preparation attempts and720C0/C1 `NOT_RUN` records. There were30unique cases per process; repeating them across configurations/processes does not create independent cases or families. All12reports, vector artifacts, per-stage timing, runtime/model identities and exact artifact hashes are retained.

| Arm | Each process, each configuration (30 cases) |
| --- | --- |
| L0/L1 |23ready,3no_match,4unsupported |
| E0 |30ready |
| H0/H1 |26ready,4requires_selection |
| C0/C1 |30NOT_RUN; no admitted provider/model account |

Every grid configuration produced the same readiness/closure counts. The preregistered **H1** selection rule therefore picks256dimensions/k4 by its fixed tie-break:26/30complete preparations with exact closure (86.7%) per process,52/60across the two repeated process labels. This remains below the proposed95%coverage threshold even on this controlled positive menu. E0's exploratory30/30 does not replace the prespecified H1 primary or authorize a default change. Four H1 requests remained `requires_selection`; their records were retained without source/data tuning.

No generalization confidence interval or2percentage-point noninferiority claim is supported by this one-family menu. No negative-case coverage is added. Original held-out data was not opened. Whole-task success, factual briefing quality, main-model tokens, provider latency and cost remain unavailable; usage fields remain null where unknown.

## Timing and resource evidence

The two embedding command stages include imports, trusted-file hashing, model load, fixed synthetic timing probes and actual cohort/document inference. They took approximately7seconds each; the enclosing successful service took16.300seconds. These stage durations are not per-task inference latency. The adapter's named cold/warm measurements are its fixed synthetic probe, not measurements of every cohort request.

| Dimension | Model load | Process peak RSS |
| --- | --- | --- |
|256 |1.2592s |1,595,772,928bytes |
|768 |1.2547s |1,595,473,920bytes |

Full adapter receipts preserve official file hashes, pooling/projection implementation hashes, runtime versions and synthetic inference timings. `verification.json` retains per-arm source preparation timings for the selected configuration, excluding actual inference. Both process labels start a new CLI process; neither asserts an inference-cache state or deployed latency.

The transient user cgroup enforced MemoryMax24GiB, MemorySwapMax0 and CPUQuota400%, with a2700second aggregate limit and1200second model subprocess limits. `envelope.json` distinguishes observed process RSS from cgroup charged-memory reporting. Launch1 failed before the runner because the service lacked a working directory (zero model/query attempts). Launch2 explicitly set it and succeeded using the same frozen bytes. The failed launch is preserved separately with raw and normalized log hashes; no hidden model/query retries occurred.

## Reproduce without model inference

From the repository root, using Python's standard library:

```sh
python3 tools/evaluation/inbox-development-results/verify.py
```

This verifies frozen cohort hashes, compressed/uncompressed vector and attempt hashes, all2520record identities/configuration/runtime bindings, unavailable arms and null usage. It reproduces all12aggregate/grading reports and byte-compares the declared selection. `SHA256SUMS` binds every public result/helper file except itself. Gzip has mtime0; full records remain accessible with `gzip.decompress` from the standard library.

To rerun inference, first obtain appropriate current execution admission and capacity. Use an isolated tracked archive at the frozen source, the existing qualified Python/runtime and trusted offline model files. Enclose the command below in a transient cgroup service with the exact resource limits above and an explicit working directory:

```sh
python3 tools/evaluation/inbox-development-results/run.py \
  --source 3676aa17e05cf7c813a2000068bc413e99738eb4 \
  --python QUALIFIED_VENV_PYTHON --model-dir QUALIFIED_OFFLINE_MODEL
```

The helper performs no download/provider calls and refuses frozen artifact/runtime mismatches. Do not overwrite retained results to hide failures or retune after these outcomes. Current defaults, held-out admission, whole-task/provider comparison and the human adoption gate remain unchanged.

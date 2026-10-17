# Native embedding qualification

The optional Go adapter at [catalog/embedding/llamacpp](../../../catalog/embedding/llamacpp/README.md)
was qualified on Linux ARM64 against the retained Python reference. It does not
change preparation or Zati defaults, execute inbox tools, or qualify a deployed
consumer. Mac qualification and Zati's child-process lifecycle remain separate.

Final evaluated source is `b29492d4659a93b07d9b501ce664a8cbf6a84989`.
[evidence.json](evidence.json) records exact archive/binary/model identities,
CPU-only static-library build flags, resource enforcement, successful checks,
failed source checks and portable/raw log hashes. The standalone binary depends
only on system C/C++ runtime libraries; model files and compiled binaries are not
committed. Its source-tarball build reports commitunknown; source/binary hashes,
not that version string, identify the qualified artifact.

## Frozen numerical and downstream results

| Check | Final result |
| --- | --- |
| Token IDs |39/39 exact, including BOS2/EOS1;35fixed inputs plus four preregistered Unicode/special-token/empty probes |
| Vectors |78/78 finite unit vectors, both768 and256dimensions; norm tolerance1e-5 |
| Python cosine parity |Minimum0.9999121453298294, above the frozen0.9999 threshold |
| Context overflow |8203tokens rejected with HTTP500; no numeric output accepted or truncation |
| Prepared context/readiness/closure |840/840 retained arm records match Python's selected k4 reference across both dimensions and two process labels |
| Owned process |Canonical trusted-child bind log observed before any bearer HTTP request; unauthenticated models endpoint returned401; child stopped and reaped |

All numerical thresholds and probes were committed before native outputs. No
threshold, gold label, caller policy, catalog, recipe or ranking behavior was
retuned. `initial-61caa-results/` preserves the earlier numerical run; it is not
relabeled final candidate acceptance. `results/` is the complete final rerun after
strict numeric decoding/input-bound fixes and canonical owned-bind framing.

The840records are repeated configurations/process labels over30unique synthetic
menu cases in one construction family. They include240C0/C1NOT_RUN records and600
local preparation records. The existing H1 outcome remains26/30complete per
process (86.7%), below the proposed95%coverage target; native parity does not turn
that into an adoption pass. E0's30/30 remains exploratory. There is no new
unseen-task, negative-case, factual briefing, live Gmail, main-model token/cost or
noninferiority evidence. Original held-out data was not accessed.

## Runtime and resource boundaries

Build and model jobs used the existing owned Linux CPU host, four model threads
and a cgroup400%CPU quota,24GiB charged-memory limit, no swap,1200second job and
2700second aggregate ceilings. Build completed in127.027seconds; final numerical
qualification service completed in50.572seconds. Full source checks passed:
root Go tests/vet/build, cmd/gist build, affected race, changed-source golangci2.14,
16Python regressions, gofmt and unchanged go.mod/go.sum after tidy. Existing
unrelated full-root lint findings remain disclosed rather than claimed clean.

Native HTTP/embedding request timings are retained in the vector artifacts;
Python probe load/RSS/timing receipts are separate. The native process checkpoint
observed VmHWM679856KiB and VmRSS677480KiB, not a final lifetime-RSS measurement.
Total native thread count includes HTTP workers; model compute threads were four
and CPU quota enforced. No cold/warm source process label asserts inference-cache
state. This is not target-Mac or whole-task latency qualification.

## Reproduce without models or downloads

From the repository root:

```sh
python3 tools/evaluation/native-embedding/verify.py
```

The standard-library verifier recomputes token/vector thresholds and exact
readiness/closure parity from all retained compressed artifacts. `SHA256SUMS`
binds all public files except itself. Gzip has mtime0. Portable logs retain
semantics while replacing private paths; their hashes differ explicitly from raw
external logs and are recorded separately.

An admitted inference rerun uses the immutable source, trusted native binary,
GGUF and retained qualified Python runtime/model with `qualification.py` under
the same resource envelope. It performs no download or provider call, verifies
actual binary/model hashes, authenticates only after owned-child bind proof, and
stops on mismatch without fallback or weakened gates. Its private runtime paths
are explicit operator inputs and must not be published. Recheck capacity and
current task authority before starting another run.

# Native Go embedding trial — 2026-10-16

## Authority and source boundary

The owner invoked ship after approving the proposed Go application/native
inference architecture. This admits source implementation and an opt-in bounded
trial, not semantic default adoption, installation, release, mailbox/provider
actions or hosted work. Gist base44c667a5b32eb9abaf2e7956bb4ec62ca1a59f66;
Zati baseaa379b6dbc3da65f5a8ae896a1984524a8497e11. Dirty original checkouts
remain preserved. Frozen contract52e2855 and source admission7855702 precede
implementation and native inference; Wazi432 tasks valid, source digest
ef75d19d9e723b5418d73bc85a8947d910e6b9d91aedcb2d875b2e600338bca7.

Default batch ceiling3, actual three outcomes T-PREP.40-.42: Go HTTP adapter,
native/reference qualification and explicit Zati-owned probe lifecycle. One
retained adapter/runtime owner and one isolated consumer integration owner use
inherited settings; a separate non-author reviews source and parity. Coordinator
owns plan/evidence and producer-first landing. No model overrides or available
model/token/cost telemetry; no acceleration claim.

## Frozen native candidate

- llama.cpp v0.7.0 commit620d2225cc436ab0b001e3200e590e3b286ef9ab, containing
  EmbeddingGemma2 support4fbc76dec51d0add466f0210855c0596589b60d4.
- GGUF revisionbfcd298762cc34d0357ece5ebdd31791a3a374d8,
  embeddinggemma-2-BF16.gguf,557950176bytes, SHA256
  68bae29d62fb8c7d23e98d21fd4662753ddd636e6b62b8a70dfe059a9844f216.
- Initial CPU-only static build disables CUDA, BLAS, OpenSSL and UI fetch/build.
  Prebuilt UI otherwise triggers an incidental download; exclude it explicitly.
- Downloads<=2GiB, artifacts<=10GiB, four CPU/build threads,24GiB cgroup memory
  with no swap,1200-second build/inference deadlines,45-minute aggregate.
  Existing Linux host first; Mac needs independent capacity/lease/evidence.

The server echoes request model aliases, so an alias is only a consistency
check. Model identity must bind verified executable/GGUF bytes, exact prefix/
tokenizer/pooling/projection/output configuration and owned process custody.
The pinned embedding route does not implement a truncate:false control; the Go
adapter must count exact tokens and reject overflow before inference. Explicit
/tokenize special-token settings must match the embeddings route. No hidden
proxy, redirect, retry, model fallback, download or process belongs in Gist.

Zati has no existing inference-process supervisor to reuse. Its explicit
embedding probe command owns start, authenticated readiness, one bounded synthetic
Go embedding and shutdown/reaping. It does not start an unused server from normal
serve or change public model/mailbox operations. Executable/model staging,
private ephemeral authentication, sanitized environment/logs and owned-child
cleanup are part of the source contract.

## Preregistered parity

Before native outputs: exact token IDs on the frozen35 reference inputs plus
Unicode/special/empty/overflow probes; finite unit768-vectors within1e-5 norm
tolerance; cosine>=0.9999 against corresponding Python reference, including
256 truncation then renormalization. Match BOS/EOS, prefixes, pooling/projection
and ordering. Frozen30-case preparation/closure results must be compared through
the actual Go/native path. No threshold relaxation or corpus/gold retuning. A
mismatch blocks positive native qualification and dependent claims.

Python measurements do not establish native memory, startup, speed or Mac behavior.
The prior one-family positive cohort remains development-only; even numerical
parity would not qualify whole-task, negative-case, generalization or adoption.

Implementation, actual qualification, checks and review remain pending until
recorded below.


## Initial implementation and review checkpoints

CPU static native build passed in2m7s. Exact binary SHA256
436cc65b98927488311f1682658ec8b14f4e833107a37f5ce546225e9a3f9fcc;
source archive1d4a987d9bd048f6e93107e838026438df70fbde237fc0c69c63d600512b4bd9;
GGUF digest/557950176-byte size verified. Dynamic inspection found system C/C++
libraries only, no adjacent llama/ggml/OpenMP/CUDA runtime closure. Version output
says0.7.0-dev/build0/unknown commit because the immutable tarball has no Git
metadata; provenance relies on verified source/archive/binary hashes, not a
fabricated self-attested commit string.

Gist sourcee79d706/64f5257 adds the bounded native client; initial HTTP tests
passed. Native qualification source61caa1d and archive
93ea219d3cefbba9d90795bf18dd1545bceedb382e5d2ca965477233d98c9f1b
freeze35 reference plus4 edge probes before numerical outputs. No positive
parity assertion is made at this checkpoint. Zati scaffoldd5a1c66 plus test-only
93f157f passed renderer checks; focused Go1.26.2 candidate-overlay tests passed.
Root production dependency pins are unchanged until producer landing.

Independent code pre-review recorded accepted blockers:

- NATIVE-R1: an alive child plus authenticated alias listing does not establish
  listener ownership. A foreign service can learn the first request's bearer key
  and spoof responses before the owned child's bind failure is observed. Require
  trusted owned-child bind evidence before any HTTP request, with an adversarial
  foreign-listener regression and preserved noninterference.
- NATIVE-R2: Go JSON converts numeric-array null elements to zero. Explicitly
  reject null token/vector elements rather than accepting malformed responses.
- NATIVE-R3: limiting the helper scanner to1MiB silently accepts a valid partial
  prefix. Read a bounded complete snapshot with overflow detection before any
  model request; retain an oversized-valid-prefix regression.

Corrections, affected checks and exact-head re-review remain open. These are
source integrity fixes, not ranking/gold/threshold changes.


Initial numerical qualification61caa1d passed under the bounded CPU service
in51.331seconds:39 exact tokenizer-ID inputs (35 frozen and4 probes),78 vector
comparisons with minimum cosine0.9999121453298294 against the frozen>=0.9999
criterion and norm tolerance1e-5. All840 native-vector cohort records across
both dimensions and process labels retained identical readiness, complete
context, selected skill and tool/capability closure versus Python. This is
numerical/controlled source parity, not bit identity or whole-task performance.
Initial actual inputs were under the helper limit and responses had numeric
elements, so R2/R3 do not invalidate this observed run; the corrected final
adapter/helper still require affected qualification and re-review.

## Corrected source qualification

Gist b29492d passed the complete prescribed ten-stage source suite: root Go
tests, vet and build, command checks, affected race tests, changed-package lint,
16 Python checks, formatting and module-tidy comparison. The corrected native
qualification passed in 50.368 measured seconds (50.572 seconds service elapsed,
150.696 CPU seconds, cgroup peak 1.0 GiB, no swap). Its source archive SHA256 is
05bad18611616e93593fad2482898bfea9bebc06eb440a990cbf96d294353991.
The native executable and GGUF remained identical to the initial run. Exact
owned-child bind evidence preceded every bearer HTTP request. Initial and final
results are retained separately; no original measurements were relabeled.

Mac CPU-only source/probe verification was separately admitted after Linux
parity: fresh load at most 10 and exact owned build lease, four jobs/threads,
10 GiB external artifacts, aggregate owned-process RSS watchdog at 4 GiB,
180 seconds per probe and 1200 seconds for the build. The 16 GiB Mac has no
per-job cgroup or no-swap isolation; stop below 2 GiB available headroom and
record actual pressure. System swap, signing and Keychain remain unchanged.
Identical pinned public source/GGUF bytes may be reused; a scoped Go 1.26.2
download is bounded at 200 MiB. A Mac synthetic probe alone cannot establish
the frozen 39-input tokenizer/vector parity. Current plan conformance: 432
tasks, source digest
f31163b139076fdfb45bfae46149505ca740ed58ae385956911612d16131d93e.

Zati 9e88ac7e with the Gist b29492d candidate workspace passed actual Linux
Go CLI probes for both 256 and 768 dimensions against the qualified binary and
GGUF. Both returned `synthetic_probe_completed`; retained cleanup observations
show zero session/key files and zero remaining owned children. The dedicated
unit enforced four CPUs, 24 GiB memory and a 1200-second deadline, with each
probe additionally bounded at 180 seconds; total service elapsed was 7.635
seconds. These are synthetic probe results, not mailbox/provider/task-quality
or default-adoption evidence. Final focused race, vet and build passed on the
pinned Go 1.26.2 toolchain. The complete command-package test had also passed;
later source corrections received focused affected checks.

The uncapped Zati lint baseline comparison, Mac probe evidence, final independent
approval and producer-first landing remain pending at this checkpoint.

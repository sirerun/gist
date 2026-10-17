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

## Producer review and landing

Independent review approved Gist f12188c8ec66977edc0a528b481b5bb18e008ec0
against base 44c667a5b32eb9abaf2e7956bb4ec62ca1a59f66. The reviewer independently
verified 79 retained hashes, all 39 token comparisons, 78 vector comparisons and
840 closure records, and checked that the qualified b29492d source is unchanged
in the final candidate. All three accepted findings were resolved. Zati's
cleanup correction and mechanical generated-contract changes were also reviewed;
final consumer approval still awaits its immutable producer pin and final head.

[Gist PR #78](https://github.com/sirerun/gist/pull/78) was guarded-rebase merged
with exact head f12188c8ec66977edc0a528b481b5bb18e008ec0 against the unchanged
base above, landing cfd902ebc9cff3ce9c763cb95207e314db7f50b9. Fresh origin/main
matched this tip and was tree-identical to the approved candidate. There were
no review threads or enforced required checks; policy required a PR and rebase.
Hosted CI did not start: run 39003421622 / check 117310908824 reported the
account locked for billing. The existing authorized local/qualified-host
fallback was used without policy changes; this is not CI-green evidence.

Consumer dependency refresh, landed affected checks and final closeout remain
open at this checkpoint.

Producer landed acceptance subsequently passed against cfd902e: tracked source
archive SHA256 7d7c0d302e72035e26a5e6932103f4f4a690fc3d96e548cc6f29e044e3c74bbb
matched after transfer; affected Go tests, vet and build passed. Retained parity
artifacts reverified without rerunning reference inference. Fresh actual Go
adapter probes returned finite unit vectors at both dimensions, with owned bind
proof before any HTTP and the child cleanly reaped. The service enforced four
CPUs, 24 GiB, no swap and a 1200-second deadline. The two single-query requests
took 0.384 and 0.365 seconds; these observations are not a comparative benchmark.
[Portable landed evidence](2026-10-16-native-go-producer-landed.json) SHA256:
455a56623518ec6f0d23a2a4e490345e23a75bd29b9b864c7a1a104dba226d2b.

## Separate Mac evidence and limit

The first Mac invocation failed to enforce its printed load gate and briefly
started CMake above the allowed load. Its exact owned process group was stopped
after 15.358 seconds; no success was claimed. After correcting the automatic
pre-spawn guard, one admitted retry passed the native CPU build in 61.604
seconds and the ordinary pinned Go CLI build in 9.417 seconds. Sampled aggregate
owned RSS was reported as 1.21 GB peak and conservative available memory
was reported as at least 2.94 GB during those successful stages.

The fresh gate before the first actual Mac embedding probe reported load 21.54,
above the maximum ten. Neither dimension's probe started. The exact owned build
lease was released, with no polling/retry loop. Mac evidence therefore establishes
source builds only; execution, vectors and tokenizer parity remain unqualified.
The independently qualified Linux path is unaffected. Zati's consumer evidence
retains the failed attempt and final platform distinctions.

## Ordinary consumer dependency checks

Zati 8fbbf906 pins actual landed Gist cfd902e as
`v1.2.1-0.20261017044208-cfd902ebc9cf`, with matching module checksums and
dependency lock. With GOWORK off and no replacement, the pinned Go 1.26.2 run
passed the complete command suite (121.658 seconds), focused race tests (1.400
seconds), vet, CLI build, module verification, empty tidy diff and all 50
generated specification checks. The dependency gate agrees with the lock.
The independent reviewer had already found the immutable pin/lock delta clean;
final evidence and exact consumer approval remain open at this checkpoint.

Fresh ordinary-pin Linux CLI probes also passed for both dimensions, with clean
session/child removal, four CPUs, 24 GiB and explicit zero-swap policy. The pair
took 7.630 seconds. Extra execution/skills dependency checks passed. A broader
integration selection exposed `TestLandedPublicDescriptorsMatchFrozenCatalog`
reporting 227 descriptors against its frozen count of 220. The exact aa379b6
baseline with its original older Gist dependency and GOWORK off reproduced the
same failure. It is retained as an existing failure, not changed or described
as passing. Narrow actual Gist integration checks are recorded separately in
the final consumer evidence.

## Consumer review and landing

Exact `TestGist` integration checks passed in 24.961 seconds. Independent final
review approved Zati 2a7fbebc8d8e77e29faf4a6bf858aa81bc20b0ab against
aa379b6dbc3da65f5a8ae896a1984524a8497e11, including the final normal-module
evidence, pin, generated contracts and platform/baseline distinctions.
[Zati PR #43](https://github.com/zati-run/zati/pull/43) was guarded-rebase merged
with that exact head and unchanged base, landing
fab1de071cffcc835a3371e3158da031d158c3cd. Fresh origin/main matched that tip and
was tree-identical to the reviewed candidate. The branch was unprotected and
there were no review threads. Hosted CI check 117312666314 did not start because
of account billing/payment limits. The authorized local fallback was used;
there is no hosted CI-green claim.

Producer and consumer source are now merged. Consumer landed acceptance remains
open until the bounded fresh-source probe and affected checks complete.

## Final landed acceptance

Consumer fab1de071cffcc835a3371e3158da031d158c3cd passed fresh tracked-source
acceptance with GOWORK off and its ordinary immutable producer pin. Archive
SHA256 16b65135f575113ad21ec8b94851c5592bd529cf619e21307fa0a1ce9d3bd499 matched
after transfer. Focused lifecycle tests, vet, build, lock/spec checks and actual
256/768 probes all passed. The four-CPU, 24 GiB, no-swap service completed in
15.771 seconds with zero remaining owned children or session files. No broad
suite or reference inference was repeated because the landed tree matched the
approved tree. [Portable consumer evidence](2026-10-16-native-go-consumer-landed.json)
SHA256: 2c649be30a9abfb785172fcd41f2d169ababcfeb2f355089c9e48959f4047fde.

T-PREP.39–48 are complete for this optional native Go source trial. Both source
PRs are merged and their landed runtime paths verified on Linux. Mac source
builds passed but actual Mac embedding execution/parity remains unqualified.
No install, release, deployment, mailbox action, default strategy change or
semantic adoption occurred. Existing T-PREP.13/.14 adoption gates remain open;
the earlier one-family coverage shortfall has not been repaired by numerical
parity. Existing lint/descriptor-count failures and hosted billing failures are
retained with their baseline distinctions.

Final Wazi local conformance passed all 432 tasks; plan source digest
53e4f76d213233283074b7bc6f7a163cb56c377477b338494d05959f51c92970.
This is local schema/semantic validation, not authenticated remote authority.

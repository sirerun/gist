# Task preparation evaluation protocol

Revision: source-freeze-2, 2026-10-16. No held-out or whole-task benchmark has been run.
Pinned local text-only model smoke qualification is recorded separately in tools/evaluation/qualification.
Decision records: [ADR017](../adr/017-task-preparation-and-semantic-retrieval.md),
[ADR018](../adr/018-tool-preparation-evaluation-and-adoption.md).
Task ledger: [E-PREP in the canonical plan](../plan.md#e-prep--task-preparation-and-semantic-retrieval-evaluation).
This document is the measurement contract, not a second dispatch plan.

## Question and first workflow

Can a caller prepare the complete authorized tool/skill context with fewer
main-model discovery turns and less total task cost, while preserving successful
outcomes? The first product workflow is a read-only inbox briefing using a real
maintained skill and template. No Gmail MCP is currently configured in Zati.
Controlled mailbox data and an admitted fixture adapter establish source
behavior; actual Gmail use is a separately qualified cohort, not inferred.

The model may list/read up to 20 selected messages and produce a briefing. It
must not send, draft, label, mark read, delete, follow links or fetch attachments.
Task text and retrieved messages may be adversarial; neither grants permissions.

## Controlled selected-workflow followthrough

The 2026-10-16 followthrough qualifies Zati's existing explicitly selected inbox
skill route through actual controller, model adapter, Effects, TLS MCP list/read
operations and persisted final reply. It does not replace the frozen retrieval
pilot below. Four generic hybrid abstentions remain exact rank-fusion ties;
selected workflow routing is not an improved hybrid score.

The strong direct-tools control uses ordinary worker instructions containing
identical skill/template bytes and the same two fixed tool bindings. Caller
workflow information, mailbox state, schemas, executor, permissions and review
requirements match; instruction packaging/message boundaries are a disclosed
remaining difference. There is no forced discovery step. Both arms may have zero
model discovery calls. Record all physical calls, turns, failures/recovery,
phase times and actual provider usage where available; absent usage is unknown,
not zero or a byte-derived token count. Scripted provider responses are labeled
fixtures and cannot establish real-model quality, token savings or charges.

An actual local-model development run needs its exact frozen route/model,
settings, synthetic cases/rubric, balanced arm order, call/time/cost bounds and
cleanup contract before prompts. No paid model, mailbox/Composio calls or new
downloads are admitted. The owner explicitly approved a bounded start of cached pinned Qwen,
with no downloads/paid calls, at most60 requests and one-hour service lifetime,
followed by stopping only that owned service. Asset/capacity/manifest/driver
qualification still precedes any launch or prompts. Preserve unavailable/failed attempts and report the narrow development
scope. Original held-out data, decision targets and adoption gates remain intact.

## Dataset and preregistration

Create at least 120 independently authored intent cases before model comparison:
60 development and 60 held-out test. Split by workflow/template family before
paraphrasing, so sibling paraphrases do not cross splits. Test includes at least
10 cases in each of six disjoint primary strata: straightforward covered intents;
paraphrase/multilingual wording; multi-capability/prerequisite requirements;
ambiguous/no-match; missing/stale/revoked connections or tools; unauthorized or
adversarial content. Only inbox cases are product-acceptance evidence; any broader
catalog fixtures are explicitly synthetic generalization probes. Report each
stratum and workflow separately; do not average them into a broad coverage claim.

Two annotators, one not the implementation author, label each case with allowed
skill versions, required capability set, eligible tool alternatives, dependencies,
readiness/abstention answer, prohibited disclosures/actions and an output rubric.
Resolve disagreement without model scores. Freeze a content-hashed manifest,
labels, split membership, catalog/recipe versions and grading rules. Keep test
labels unavailable to tuning and to agents performing tasks. Label access by
final evaluators is permitted and recorded. Personal mailbox data is excluded.

This is a minimum pilot, not a guaranteed statistically powered study. Estimate
sample needs from development variance before held-out runs; register any larger
sample then. A small or noisy test yields inconclusive, not a relaxed target.

## Arms and ablations

| Arm | Candidate selection | Guide | Main-model context |
| --- | --- | --- | --- |
| L0 | Existing lexical Gist | None | Selected exact tools and baseline skill |
| L1 | Existing lexical Gist | Deterministic recipe | Prepared complete bundle |
| E0 | EmbeddingGemma 2 only | None | Same context policy as L0 |
| H0 | Lexical + semantic fusion | None | Same context policy as L0 |
| H1 | Lexical + semantic fusion | Deterministic recipe | Same guide policy as L1 |
| C0 | Composio normal session discovery | Native returned guidance | Documented meta-tool flow |
| C1 | Composio direct tools | Fixed baseline skill | Fixed workload-family tool set |

L0/L1/H0/H1 isolate retrieval from guide assembly. All Gist arms perform exact
capability closure and current authorization; E0 never executes by similarity.
Use existing Gist limits as the initial baseline and development-only tune a
shared context budget. Baseline skill/output instructions remain identical.
C0 keeps documented native guidance; do not disable it to favor Gist. C1 tool
sets are selected before test exposure, not tailored using hidden answers.

Start offline with L0/L1/E0/H0/H1. Develop 256d and 768d configurations, then freeze
one semantic configuration for the primary test; label additional dimensions
exploratory. Use the same pinned text-only checkpoint/runtime, official query
and document prefixes, normalized vectors and BF16 or FP32. Record device,
thread count, batch size, resident memory, model load and index build time.
Do not assume ONNX, native Go, quantization or Mac accelerator support: qualify
the actual adapter first. FP16 is excluded per the model card.

Fusion method, candidate k (development grid 4/8/16), fallback policy and recipe
selection are frozen before test. If a required complete schema/asset exceeds
the budget, return an explicit failure; never truncate required content. Cache
keys bind catalog/model/recipe versions and scope. Exercise index replacement,
model-version mismatch, eviction and fresh revocation checks.

For end-to-end comparison, keep the main-model revision, settings, executor and
per-task limits identical. Repeat each held-out case 3 times, randomize arm order
within blocks, and retain every attempt. Use identical synthetic mailbox state
per paired task. A live-provider run is separate and read-only. If catalogs or
executors differ, disclose the confound and prohibit attributable superiority
claims. No online tuning, hidden retries or replacing failed attempts.

## What to record

- Required-capability recall@k and complete eligible tool-set coverage; all
  prerequisite tools must be present, not merely the most similar action.
- Precision, irrelevant schema bytes, correct abstention, readiness classification,
  and schema/version validity. Separate no-match cases from positive recall.
- Task success under the frozen rubric, including grounded source references,
  factual coverage, correct missing-connection behavior and no forbidden effects.
  Use deterministic checks where possible and blinded rubric review; do not let
  the implementer or one unvalidated LLM judge establish correctness alone.
- Discovery/preparation tool calls and main-model turns before first execution,
  during recovery and across the complete task. Count embedding calls separately.
- All model input/output tokens, including selection/planning/recovery and repeated
  context. Report cached tokens separately, exact tokenizer/model and billing
  categories. Byte estimates are not billed-token evidence.
- End-to-end p50/p95 latency from accepted intent to final outcome. Break out
  embedding, search, validation, assembly, model, executor and network time.
  Measure warm-cache/warm-model and cold-model/cold-index independently. Report
  initial index construction and refresh separately with an explicit amortization
  workload; never hide them in a steady-state claim.
- Model/provider charges from actual usage receipts; embedding runtime CPU/GPU
  time, peak/RSS memory and vector storage separately. Owned hardware is not
  assigned a fictional zero total cost. Dollars are unknown without actual rates.
- Timeouts, unavailable arms, failures, fallback/retry counts and diagnostics.
  No-match results are not timeouts. Use paired eligible-case denominators and
  show excluded/unavailable counts. Keep actual elapsed time, tokens and charges
  for every failed attempt in all-attempt aggregates; report successful-only
  metrics additionally. A separately named failure-penalized latency metric may
  assign the common timeout to failures under a preregistered formula. Never
  substitute that penalty for measured latency or infer billed cost from it.

Store redacted per-attempt JSONL plus an aggregate report bound to source,
model/runtime, dataset, catalog and recipes. Do not retain tokens, credentials,
private account identifiers, message contents or raw model traces in public Git.
Synthetic task inputs may be committed. Human-authorized live evidence stays in
an approved private location with retention and deletion dates.

## Proposed decision gates (freeze before held-out test)

These are adoption targets, not existing performance claims:

1. Zero observed unauthorized disclosures, forbidden effects, executable schema
   truncation or use of revoked bindings in the dedicated adversarial suite.
   Any occurrence blocks adoption; passing a finite suite is not proof of absence.
2. At least 95% complete required-tool-set coverage on supported positive cases
   and at least 95% correct abstention/readiness on negative cases. Report counts,
   exact confidence intervals and all six strata; do not hide a failing stratum.
3. End-to-end task success is noninferior to each eligible baseline: the lower
   bound of a paired 95% confidence interval on candidate-minus-baseline success
   must exceed minus 2 percentage points. Account for task-family clustering across repeated
   runs using a preregistered conservative, boundary-valid paired binary method.
   Ordinary percentile bootstrap alone is insufficient: zero observed discordance
   must not yield a degenerate interval establishing noninferiority. Predeclare
   minimum independent families and zero-discordance treatment; if the method
   cannot support the margin, the result is inconclusive. Inconclusive intervals
   require more preregistered evidence or no adoption.
4. For a token-efficiency claim, mean total model tokens per attempted task falls
   at least 20% versus the named baseline, with a paired 95% interval excluding
   zero improvement. Count failures and all preparation models. For a latency
   claim, median end-to-end time falls at least 15%, with p95 no more than 10%
   worse and a paired interval supporting the median improvement. Publish a
   separate verdict per comparator and metric; neither implies the other.
5. Default adoption requires safety/quality gates plus a token or latency benefit
   over L0, without more than 10% regression in the other resource metric. Any
   tradeoff versus L1 or C1 requires explicit workload-scoped decision; never
   describe H1 as universally faster. Memory/startup budgets are set for the
   chosen Zati target before test, not after observing results.
6. The same-task repeated-use cohort should need zero additional provider-tool
   discovery after successful preparation when its recipe is fully covered.
   Unfamiliar/ambiguous cases may abstain or use bounded fallback; report them.

For continuous metrics use paired resampling by family; binary success uses
the boundary-valid method specified above. Publish seeds and aggregation code,
and report uncertainty. Select the primary candidate on development data only. Treat other
comparisons as exploratory or apply a preregistered multiple-comparison method.
A benchmark win does not authorize changing the application default or releasing.
Unavailable provider arms may be recorded NOT_RUN and the available evidence
reviewed to retain the current default. Positive adoption still requires adequate
whole-task evidence against eligible baselines; an unavailable Composio arm cannot
support a Composio superiority claim.

## Delivery and operational acceptance

The canonical plan includes corpus/adapter preflight, harness/preparation source,
verification, independent review, correction/re-review, merge, landed checks,
offline runs, authorized paired provider runs, independent results review and
an adoption decision. Production integration, release and operational observation
follow that decision; no hosted deployment is required for a library experiment.

Harness deliverables must include documented commands for corpus validation,
offline ranking/preparation and end-to-end runs, a dry-run cost/call estimator,
and a deterministic report command. The source commands are documented in `tools/evaluation/README.md`: corpus
checking, explicit pinned local model qualification, source runner/dry-run,
post-selection label grading and deterministic reports. The fixed shared catalog
contains the maintained inbox skill plus static unsupported distractors. Its
controlled source runs do not establish broader catalog coverage, held-out
retrieval quality or whole-task performance. T-PREP.9 requires a separately frozen
selection and admitted held-out manifest; missing scenarios remain NOT_RUN.
Existing public-library and frozen-wire recipes remain regression checks.

Before provider-backed runs, the run manifest records exact allowed account,
read-only operations, model and provider call counts, total dollar ceiling,
expiry, data policy, runtime resources and stop conditions. Enforce limits before
dispatch, stop on credential/scope drift and preserve partial receipts. No
connection, budget or paid-run approval is inferred from this document.

Before default enablement, verify the real Zati preparation path and fallback
on the target runtime. Observe cold/warm latency, memory, retrieval misses,
stale-index handling and authorization refresh with version-bound evidence.
Rollback restores lexical selection without changing permissions or executing
queued operations. No publisher-signature, live Gmail or Composio advantage is
claimed by synthetic checks alone.

## Source implementation freeze (2026-10-16)

[ADR020](../adr/020-preparation-contract-and-offline-freeze.md) freezes the additive
API, source anchor, initial ranking/fusion, context/resource targets and exact
family-clustered conservative binary method before implementation and held-out
access. Its source-test budgets are targets, not measurements. Runtime/model
qualification and corpus reconciliation receipts must identify their real hashes.
No held-out results or performance/adoption claims are added by this freeze.


### Evaluation input adapter availability

The source driver now accepts hash-bound independently authored evaluation task
policies through the existing TaskAdmitter port and exposes development k4/8/16
with floor0.30/RRF60 unchanged. See the [actual CLI contract](../../tools/evaluation/README.md#independent-evaluation-policy-and-development-grid).
Policy identity and exact k are recorded per attempt and checked by held-out
admission and reporting before input/label access. Default two-literal source
admission remains unchanged. These adapters do not qualify a representative
inventory/cohort, admit held-out data, select a configuration or change Zati.

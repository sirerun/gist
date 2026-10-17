# Preparation evaluation readiness — 2026-10-16

This is a bounded readiness dossier, not a completed evaluation or adoption decision. The requested batch ceiling was ten; the executable outcome is one documentation preflight. No source, model, provider, mailbox, or held-out run was performed for this dossier. T-PREP.9, T-PREP.10, T-PREP.11 and T-PREP.13 remain pending in the [canonical plan](../plan.md).

## Evidence boundary

The inspected Gist main is `4c02a7a603a88e7914a62cdbbcbda2df164eaef4`. Preparation SDK, harness, adapter, corpus and protocol source match the qualified source head `691cced823bb4d09ceea84c188c7b92c373ac45e`; the targeted Git comparison was empty. This permits reuse of the recorded checks, not relabeling the earlier run as a new benchmark. See the [landed source receipt](2026-10-16-task-preparation.md), [frozen protocol](../evaluations/tool-preparation.md), [ADR-020](../adr/020-preparation-contract-and-offline-freeze.md) and [harness instructions](../../tools/evaluation/README.md).

| Evidence | Observed result | Limit |
| --- | --- | --- |
| Reviewed preparation source and landed checks | Public API, authorization/current-state checks, immutable complete assets/schema retrieval, optional embeddings and caller task/connection admission are implemented and checked | Source acceptance does not establish retrieval quality, installation or provider behavior |
| Qualified text-only model adapter | Four synthetic BF16/FP32 × 256/768 smoke configurations passed with pinned model files and runtime; reference load and RSS targets passed | Fixed synthetic model smoke does not measure task success, corpus quality or whole-task latency |
| Prior development source run | 60 cases × seven arms = 420 retained attempt records; every comparison status is `NOT_RUN` | Zero eligible comparisons; actual source outcomes remain recorded separately |
| Reconciled synthetic pilot | Public manifest records 60 development and 60 sealed held-out cases, six strata per split, 44/35 construction families | Only one maintained inbox product case per split; synthetic probes do not add shipped product capabilities |
| Current Zati source | Library-backed maintained Inbox Briefing catalog/import, exact instructions/template, explicit tool selection and current connection checks exist | No automatic Gmail service; Google OAuth client credentials alone do not authorize a mailbox |
| Zati native evidence | Debug source build at `aa379b6dbc3da65f5a8ae896a1984524a8497e11` passed | No installed replacement or visible end-to-end Gmail acceptance is established |

The coordinator rehashed the existing qualification artifacts without rerunning them: attempts SHA-256 `3e2798a24748c552feb1e97c8d2df177dd9eacbf9101eb2bf87f0bceb7989924`; report SHA-256 `04aad2515d9d905e195a013e6e3023af7536ba750a876a2c2fd8d51f5fef6c33`. Each arm has 60 comparison records marked `NOT_RUN`; model usage is null and vector-lookup timing excludes inference. These are development source-smoke artifacts, not held-out or whole-task results.

## Why another run is not the next useful step

The controlled command has a fixed five-skill shared catalog and only two independently authored source-smoke requests admitted by its caller policy. The maintained inbox recipe is fixed; similarity does not grant task coverage. The public SDK supports a caller-owned `TaskAdmitter`, but the CLI does not inject a general benchmark policy.

The built-in composite mail-read tool also differs from the pilot product case's declared separate list/read alternative. The harness correctly reports an unrepresentable fixture as `NOT_RUN`, retaining the actual preparation outcome separately. This is an inventory/environment mismatch, not evidence that a ranking algorithm failed. A catalog file alone must not be assumed to solve the role/closure mapping: the existing resolver and the Zati-owned two-role consumer mapping must be reconciled explicitly in the evaluation driver.

The harness already accepts bounded, digest-bound external catalog manifests with exact inline artifact bytes and one workspace. It verifies typed identities and complete bytes through the SDK. It also grades generated records after selection, records unsupported transition mechanics as `NOT_RUN`, and guards held-out admission/report identity. Those mechanisms are usable infrastructure; they do not manufacture compatible inventory or positive task admission.

| Proposed measurement | Current disposition |
| --- | --- |
| Retrieval/preparation success, abstention and paired quality difference | `NOT_ESTIMABLE`: zero eligible comparison records |
| Family-balanced confidence interval or 2 percentage-point noninferiority | `NOT_ESTIMABLE`: no eligible paired sample; this is not statistical inconclusiveness from measured outcomes |
| 256 versus 768 quality selection | Unavailable; synthetic embedding validity does not select quality |
| Whole-task tokens, latency, recovery and dollar cost | Unavailable; no admitted whole-task executor/provider run, usage null |
| Model load/RSS and synthetic adapter validity | Available only within the prior reference smoke receipt's exact configuration |
| Source lookup/preparation timings | Available as source-smoke timings; inference and task execution excluded |
| Composio C0/C1 advantage | `NOT_RUN`; no live or paid comparison was admitted or executed |

Repeated cases are grouped by actual construction family, not workflow IDs. The preregistered primary success estimand is family-balanced; pooled attempt success is descriptive. Even a future eligible sample may remain statistically inconclusive under the frozen 2 percentage-point margin. The current absence of eligible measurements must not be presented as that future statistical result.

## Smallest meaningful next slice

Before accessing held-out material, independently freeze the following prerequisites:

1. A compatible shared artifact/tool inventory and caller resolution mapping for the real read-only inbox workflow, including the exact alternative closure and schemas. Derive this from maintained artifacts and real consumer contracts, not queries or gold labels. Review its external catalog SHA/version and retain unsupported cases explicitly.
2. A genuine caller task-admission policy and evaluation driver bound to intent, principal, runtime, immutable skill pin and required capabilities. Reuse the SDK admission port. Do not broaden the source CLI's literal policy from gold labels or treat semantic similarity as admission.
3. A preregistered, independently reconciled inbox cohort with representable scenarios and genuinely separated construction families. The existing pilot remains useful for safety/unsupported-path probes; its two product cases cannot establish adoption. Freeze changes before observing comparative outcomes.
4. An executable development configuration procedure: the source CLI fixes k=4, cosine floor=0.30 and RRF=60; the protocol's broader development grid needs an appropriate driver rather than silently labeling this fixed smoke a grid search. Select one primary embedding/configuration on development only, then freeze exact source, policy, catalog, recipe, model/runtime and configuration identities before separate held-out admission.

The first controlled inventory/policy/cohort qualification does not require Gmail credentials or paid Composio calls. It does require independent contract/data work; repeatedly running the current incompatible catalog does not supply it. Keep held-out inputs and labels sealed until all admission prerequisites are met. No hidden contents or paths were read for this dossier.

The [documented commands](../../tools/evaluation/README.md) can dry-run inputs, export/load a hash-bound catalog, consume pinned offline vectors, generate attempt records and grade them afterward. They were not executed in this preflight. An actual evaluation must use the qualified caller-policy/closure driver above, retain every attempt and unavailable arm, and bind reporting to the frozen manifest. Recheck execution-host capacity and bounded runtime budgets before any future model job; this dossier consumes no inference, download or provider budget.

## Decision and delivery status

- T-PREP.9 remains pending compatible inventory, policy, cohort and development selection; this dossier records readiness only.
- T-PREP.10 remains a human gate: explicit bounded account/model/provider authority or an explicit offline-only disposition. Existing source shipping grants neither automatically.
- T-PREP.11 remains pending its dependencies. Its offline-only alternative may later record unavailable comparisons `NOT_RUN`; no three-repeat task runs or paired denominators are fabricated now.
- T-PREP.12 evidence review and T-PREP.13 human adoption/retain-default decision remain pending. Leaving a default unchanged is current state, not a newly inferred owner decision.

The recommended immediate product path is the shipped Gist library and Zati's existing lexical/catalog source integration, with its explicit service/tool setup and review boundaries. Keep the current default while the owner considers the evidence. Do not claim semantic superiority, release publication, installed acceptance, hosted deployment or live Gmail qualification from this source batch. The conditional consumer/default/release work remains behind the existing plan gates.

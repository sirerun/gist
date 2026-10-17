# ADR018 — Evidence required to adopt task preparation

Date: 2026-10-16
Status: Accepted evaluation protocol; production/default adoption undecided.
Depends on: ADR017 and RFC-002; executable task authority is docs/plan.md.

## Decision

Measure whether Gist reduces the whole task's discovery overhead without losing
required tools, correctness, authorization or useful coverage. An embedding
benchmark score or a smaller prompt alone cannot establish that outcome.

Use two evaluation layers: offline labeled retrieval/preparation, then paired
end-to-end model tasks. Separate retrieval effects from guide effects through
ablations. Include both Composio's discovery path and its direct-tools path;
do not compare only against an unnecessarily inefficient configuration.

The [protocol](../evaluations/tool-preparation.md) fixes corpus construction,
train/development/test separation, baselines, measurements, proposed acceptance
thresholds and permitted conclusions. Thresholds are engineering targets, not
measured savings. Freeze them with the dataset and run manifest before the test
set is opened. If the confidence interval is inconclusive, retain the existing
default and collect a separately registered follow-up; do not tune on test results.

## Fairness and authority

Compare the same task, model revision/settings, tool versions, allowed accounts,
executor, mailbox snapshot, output rubric and budgets wherever technically
possible. For an attributable Composio comparison, both Gist and Composio discovery
arms use the same Composio-backed tools/executor. A controlled Gmail fixture
comparison proves local source behavior only. If provider surfaces cannot be
matched, report an observational comparison with the confound, not a causal
claim of retrieval superiority.

Direct-tool baselines use a fixed workload-family tool set selected from the
development data. They receive no hidden test labels or per-task oracle. Setup,
authentication and cold starts are measured separately from steady-state runs;
account setup needed only once must not be charged as a recurring competitor cost.

This planning request authorizes no benchmark spending, cloud provisioning,
mailbox reads or publication of personal data. Offline cases contain synthetic
mail and public schemas. A live run requires an exact account/connection,
allowlisted read operations, total model/provider dollar ceiling, call cap,
expiry, host capacity and data-retention approval in its run manifest. Missing
access is NOT_RUN, never a loss for the unavailable arm.

## Adoption decisions

A reviewer may approve lexical preparation, hybrid preparation, a narrower
workload-specific rollout, or no change. Select the simplest candidate meeting
correctness, safety and cost/latency gates. Improvements relative to one baseline
do not establish superiority over another. Report any tradeoff against the
strongest eligible baseline explicitly. Native-runtime fit, memory, offline
operation and rollback are product gates beyond retrieval accuracy.

New source follows independent review, guarded merge and landed verification.
Default enablement and release are separate gated decisions based on exact
qualified revisions. Hosted expansion and RFC-003 remain outside this work.
Operational evidence must cover invalidation, revocation, fallback, memory,
latency and rollback; reverting retrieval must not alter authorization policy.

## Controlled Zati followthrough — 2026-10-16

The owner approved a controlled inbox followthrough after native numerical
qualification. Zati's existing selected-skill path already pins the exact inbox
instructions/template and separate list/read mappings before model dispatch.
An exact RRF tie in open-ended search does not justify weakening abstention or
adding an unused library API for that already-selected workflow. Preserve the
original four abstentions and qualify the actual selected consumer path.

For this development slice, compare catalog-imported instructions with ordinary
worker instructions containing the identical skill/template bytes and the same
two explicitly configured tools. Preserve caller workflow information, mailbox,
executor, permissions, review requirements and grading. Report packaging/message
boundary differences. Both paths may require zero model discovery calls; no
improvement is presumed. Controlled provider fixtures qualify source behavior,
not real-model token savings. Any actual local-model study is separately frozen,
bounded and reported as development-only. No Composio attribution, held-out
generalization, default adoption or live Gmail evidence follows from it.

## Sources

Composio's documented search already returns tools, an execution plan and
connection status; direct tools can avoid discovery when the set is known.
This motivates a strong baseline rather than assuming every competitor task
needs multiple searches:
[Search Tools](https://docs.composio.dev/toolkits/meta-tools/search_tools),
[Toolkits](https://docs.composio.dev/docs/toolkits).

Google's model card is evidence of available model configurations, not an
application benchmark: [EmbeddingGemma 2](https://huggingface.co/google/embeddinggemma-2).

# Composition qualification protocol

Pre-score protocol, 2026-10-17. This implements T-COMPOSE.17–.18 after Gist
producer/index landing and Zati PR47 landed verification at fc68df88. It is a
new synthetic development qualification, not the old composite pilot, held-out
access, model comparison, semantic adoption or provider acceptance.

## Separate measurements

The scale workload and thresholds in composition-scale.md remain unchanged.
Use 10k/100k total logical summary records with the frozen 40/30/10/20 percent
kind mixture, ten private partitions and a separate all-visible scope run for
each size. Each is a separate process so peak RSS includes that run's dataset,
index, current/previous generations and any retained in-flight generation.
Use actual process RSS (Linux getrusage or time), not unreliable service memory
accounting. Record architecture, kernel, Go version, CPU/memory/deadline envelope,
exact source/archive and all failures. No hardware substitution without a new
explicit freeze. Cold build/first query and100 warm observations per query class
are distinct. Report p50/p95, exact/narrow/broad outcomes, result bytes, build,
100-record update, full rebuild and monotonic rollback costs. Exercise an
in-flight reader across updates and verify generation refusal before exposure.
The encoded snapshot projection means the public exact-candidate array for the
current snapshot, including scope/generation: measure its encoded bytes through
public exact lookup outside latency samples. It excludes internal posting-map
serialization; total process RSS covers resident structures. Report this boundary.

Composition readiness uses a separate small exact-artifact inventory based on
maintained inbox2.0.0 and its real list/read capability schemas, matching the
canonical structured TLS MCP cohort in landed Zati. It does not inflate the
10k/100k summary counts. Synthetic distractor summaries are not30k fully
implemented procedures. Results cover one maintained composition family and
cannot support broad semantic-quality or provider-parity claims.

## Inventory and cases

The harness first emits an inventory only, without running/scoring cases. It
records original immutable skill/package/manifest/capability digests, actual
provider/tool/binding artifacts, caller-owned identity adapter/conformance pins,
connection/account/generation and principal/policy scope. All tools use separate
list/read contracts; no aggregate proxy or artificial compound tool. Use real
ScopedIndex lookup and composition.Service preparation with an exact retriever
and controlled current-authority ports. No store-wide schema scan on requests.

Inputs are tools/evaluation/composition-cohort/inputs.jsonl. Each scenario starts
from an independently constructed baseline with one exact list/read pair on one
connection/account, caller query is:unread, current authority, supported version,
and ample complete-context budget. Changes are only those named below:

| Scenario | Input condition |
| --- | --- |
| one_pair | Baseline |
| missing_read | No read binding |
| wrong_contract_digest | Read admission has a different contract digest |
| different_accounts | List/read use different account identities |
| different_connections | List/read use different connection identities |
| ambiguous_two_pairs | Two admitted alternatives per requirement on one connection; no selection |
| selected_second_pair | Same two alternatives, explicit second binding for each requirement |
| sixty_five_alternatives |65 admitted alternatives per requirement, exceeding lookup completeness |
| unsupported_consumer | Consumer declares no supported composition version |
| revoked_final_authority | Initial authority succeeds; final check revokes it |
| context_budget_512 | Complete result budget is512bytes |
| tool_bytes_drift | Retrieved read-tool bytes differ from pinned bytes |
| private_candidate | Only read candidate is in another principal's private scope with a sentinel in its metadata; current authorization denies it |
| cancelled_request | Context cancelled before preparation |
| selected_of_sixty_four |64 alternatives per requirement and an explicit valid selection |
| selected_second_account | One pair per account on two connections; explicit second account/connection selection |
| stale_authority | Authority timestamps exceed the fixed freshness bound |
| package_bytes_drift | Stored package bytes differ from the immutable pin |
| task_policy_denied | Query widened to all; caller's fixed unread-task policy denies it |

Inventory generation must expose case artifact identities and concrete caller
selections, not expected outcomes. Exact chosen identities are frozen before
labels. Primary labels are authored by the coordinator and secondary labels by
a separate non-harness author given only this protocol, inputs, inventory and
published contract. Reconcile disagreements before any scored runs and preserve
both author files plus decisions. Labels specify expected readiness, selected
binding/tool identities for ready outcomes, and permitted disclosure. They must
not be generated by calling the implementation being evaluated.

Run all19 cases once after freeze and retain every observed result/error, selected
identity, serialized response bytes, elapsed time and authority/index access
counts. Readiness requires the complete exact two-tool closure. Report recall
among labeled-ready cases, overall outcome coverage, false readiness and private
sentinel disclosure separately. Thresholds: zero false-ready and unauthorized
exposure; every labeled-ready case must prepare its expected exact closure.
Unsupported/budget/cancel cases may refuse with an error or nonready result;
never relabel a refusal as an empty successful match. No model token counts are
available: report bytes, not token estimates or speedup against Composio.

## Freeze and execution gate

Before scoring, commit runner, immutable inventory, two independent label files,
reconciliation, primary configuration and SHA256 manifest; record independent
review of that exact freeze. Config records100 warm samples, the four scale runs,
all19 composition inputs, canonical lexical route, and unchanged thresholds.
Source unit/smoke checks and inventory generation are allowed before this gate;
scored cohort/scale execution is not. Use the previously qualified existing Linux
ARM64 route,4CPU/24GiB and20minute bound per run, offline cached Go1.26.1. No model,
provider, held-out, download or paid call. Retain all attempts if checks fail;
fixes get new source IDs and cannot silently rewrite labels or thresholds.

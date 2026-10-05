# Gist full SDLC to AWS production at gist.sire.run

Date: 2026-10-04. Status: **execution started: source-only preflights; engineering and production gates remain open**.
Planning contract: ordinary unenrolled repository delivery. Source baseline inspected: Gist main `84e14128565058419a9b90619f27fa517ba4f494`; revalidate before dispatch. This is the active bounded recovery supplement to [registry-buildout.md](registry-buildout.md), not a replacement of its completed tasks. [gateway-buildout.md](gateway-buildout.md) remains a separate scope. This is the canonical production-owner plan, also included by docs/plan.md for default parser/dispatcher visibility; March task text and completed status remain historical. Destination/scope follow accepted ADR011. No executing lifecycle is admitted by this planning refinement.

## Context and outcome

Deliver the complete agreed registry and caller-owned integrations in production on AWS at **https://gist.sire.run**. Complete the registry gaps preventing an authenticated caller-owned journey: invited enrollment/bounded issuance, persistent signing identity, canonical exact artifacts/resolution, durable revocation, and real Treg/Composio capability artifacts. Gist publishes/discovers/resolves; the consumer enforces and executes. Requirements, design, preflight, coding, behavior/security acceptance, format/lint/CI, independent code review, corrections/re-review, merge, landed checks, immutable release, authorized rollout, live/pilot acceptance and bounded operational handoff are first-class dependency-linked plan items.

No full generic gateway, arbitrary provider proxy, vault, billing ledger, global client setup, organization bootstrap, private application implementation or marketing publication is included. Portfolio hierarchy is consumer-owned; chiefs do not inherit cross-business credential/memory authority. Hosted memory and private relationship-data isolation remain separate qualification obligations. Production deployment at the stated AWS origin is explicitly in scope. Planning itself never performs deployment, account enrollment or provider spending; existing scoped grants persist and genuinely missing numerical/custody authority stays explicit.

## Discovery and use-case coverage

Fresh source confirms `app.go` generates a startup signing key and composes process-local events; composed resolve uses `skill`/`runtime_id` rather than frozen `skill_ref`/`runtime`; external enrollment remains incomplete. Composio capture is `source_unavailable` and its action is synthetic; no Treg capture/adapter was found. App MCP exposes only gist_discover/get/resolve/connect. Live acceptance evidence `M2a.json` is absent. No code graph binding was callable/fresh in this session: rg/file/API/route/test inspection is the qualified fallback. docs/design.md and ADR004-010 exist; docs/devlog.md now records prior planning events. No repository lifecycle enrollment was found or created.

Reuse the original eleven UC IDs and coverage matrix in registry-buildout.md; this production plan retains all eleven original use cases, with required later client/connection/taxonomy completeness reconciled from the original plan rather than silently dropped. Source presence and historical checked tasks do not prove a deployed journey. Relevant map:

| Use cases | Current gap | Recovery gate |
| --- | --- | --- |
| UC-001 local compatibility | Must preserve root library/CLI/stdio | T-GR-INTEGRATE.3/.6 |
| UC-002 publication; UC-003 discovery; UC-004 exact artifact | Real capture/closure and composed wire evidence incomplete | Provider lanes, WIRE, INTEGRATE |
| UC-005 resolution; UC-006 delegated connections | Canonical payload/actual binding/connection parity | WIRE, INTEGRATE |
| UC-007 isolation; UC-009 authentication | Enrollment and restart/replica identity | AUTH, KEYS, INTEGRATE |
| UC-008 revocation | Durable shared feed/cursor absent in app composition | EVENT, INTEGRATE, RELEASE |
| UC-010 runtime interop | Actual consumer receipt/live execution not qualified | SCOPE.7, RELEASE.6, M3 |
| UC-011 optional taxonomy | Existing optional behavior must remain accurate/attributed | M3/PROD regression and original requirements matrix |

## Scope, status preservation and deliverables

| Deliverable | New lane | Historical crosswalk, preserved |
| --- | --- | --- |
| Requirements/design/operator custody | SCOPE | A2/A4, ADR005/007 and Q5 credential-path note; amendment through reviewed implementation, not silent approval |
| Enrollment and bounded human/workload grants | AUTH | I1-I6 source presence preserved; actual enrollment/issuer integration is new missing work |
| Persistent shared signing identity | KEYS | I1/I4 and Q3 follow-on defect |
| Canonical API/MCP resolution and exact closure | WIRE | S1-S3/T1-T3/Q3 follow-on defect |
| Durable event/cursor composition | EVENT | B/S/Q3 follow-on repair; existing durable migrations alone did not prove wiring |
| Actual action-specific Treg/Composio artifacts | TREG/COMPOSIO | D1-D4 original fixtures preserved; no replacement of historical synthetic evidence |
| Actual app composition and qualification | INTEGRATE | Q3/Q4 checked history preserved; new integration receipts appended |
| Release/live/ops/milestone acceptance | RELEASE | Q5 remains open until every original obligation is genuinely met; X-R1/R3 real-build gaps retained |
| New-origin AWS/DNS/TLS/config cutover | AWS | ADR011 supersedes only ADR008 origin; existing infrastructure owner retains reviewed source |
| Full M2b/M3 production acceptance | NEXT/M2B/M3 | Original Q8/Q10/R4/R5/E3 coverage and IDs preserved |
| Terminal production acceptance | PROD | Final conjunction of actual running AWS service, requested hostname, registry/client/provider and operations evidence |
| Optional managed gateway | GATEWAY outline | Remains outside founder-confirmed production scope |

Each named task below is a planning task record, not a minted external-service task ID or admission. `acc` is opaque authored acceptance because Kazi is installed. Explicit `lane: agent` coding markers select the user-requested GPT-6-Luna worker instead of the default Kazi authoring lane; the user model instruction overrides any legacy frontier-model suggestion. Kazi may supply a qualified acceptance draft/verify helper just in time without replacing the specified worker model. Stage rows execute through stage guidance, not Kazi code convergence. Preserve canonical lifecycle tasks instead of this graph if the change is later enrolled.

## GPT-6-Luna concurrency contract

- All delegated implementation, verification, review and landed-check workers use **GPT-6-Luna**, explicit model binding `gpt-6-luna`. One coordinator retains requirements, contracts, ownership, plan writes, integration arbitration, decision routing and final verification. No automatic paid model/provider fallback.
- Fill every available worker slot with an eligible disjoint task and refill immediately on handoff; barriers are actual dependencies, not completion of a whole artificial wave. Current harness capacity is four active agents including coordinator: **three concurrent Luna workers**. Preflight records the actual capacity; maximum workers = available total agent capacity minus one coordinator, limited by eligible owned work and qualified host/build resources. Scale above three only when the executing harness exposes and records additional authorized capacity; no bypass of runtime limits.
- Initial read-only capacity, CI-policy and provider-source preflights are independent, letting three workers run while the coordinator reconciles source. Six independent component lanes then give a backlog wider than today's pool. AUTH can remain blocked on operator choice while KEYS/WIRE/EVENT and offline capture preflights progress. Candidate verification and independent review can overlap another lane's implementation. Reviewers must be different from candidate authors/coauthors; no fixed idle reviewer slot is required.
- Each writer owns a unique external-SSD worktree and exact file set. Verify the configured volume mounted, writable and with measured adequate space; keep task-specific GOCACHE/GOMODCACHE/GOTMPDIR/TMPDIR and test artifacts there. Never repurpose HOME or silently use internal disk. No worktree/build was created by this planning run.
- Integrator alone owns app.go/config.go, shared ports/bootstrap/catalog indexes and joined acceptance files. Allocate disjoint storage files and migration IDs before dispatch; contract changes are returned as handoffs. Workers never overwrite another lane or mutate another repository. task/resource claims retain WON SHA for CAS release; coordinator alone writes this plan.
- Maximum **two heavy build lanes per project**, one full race suite, and the shared machine build lease/load rules still apply. On the shared Mac, hold if one-minute load >10 and claim the configured R-build-lease before multi-package build/test/lint; only WON authorizes running, recheck ownership and release own SHA immediately afterward. Where that exclusive lease serializes heavy commands, worker count does not override it. Other workers perform source/design/capture/review while waiting; qualified isolated remote builders require separate admitted capacity.
- Use runtime-detected native agent tools; never create guessed CLI APIs. Stage/domain guidance loads only at the runnable stage. No new agent was spawned for planning. Unavailable model, worker tooling, build capacity or SSD blocks the affected stage and is reported candidly.

## Schedule and dependency policy

The founder explicitly requested the entire SDLC as first-class items, so the known recovery deliverables include all stage obligations now, including gated release/live/operations rows. Those rows state verifiable outcomes rather than inventing a deployment design; their preflights revalidate actual landed inputs and authority. The user now explicitly requires full registry production completion. Known M2b/M3 obligations are therefore first-class stages with a receipt-driven planning refresh before dispatch; genuinely new unknown epics stay outlines with one triggered planning task. The unselected managed gateway stays outline and never blocks this completion. The split epic files contain the machine-readable wave assignments and task dependencies. Wave labels describe the horizon; they do not forbid pipelining independent verified candidates. Merge mutations to main are serialized by the coordinator, but unmerged branch work and disjoint reviews remain parallel. Real shared interface dependence requires landed receipts; speculate only with explicitly recorded immutable inputs and no delivery authority. Estimates remain TBD pending bounded preflight sizing, rather than inventing dates from unresolved scope.

Every candidate chain is preflight -> implement -> behavior/quality verify -> independent review -> guarded merge -> verify-landed. Accepted findings allocate stable suffix IDs such as T-GR-KEYS.1.F1 and T-GR-KEYS.4.R2 with implement/verify/review markers and exact dependencies. Fixes depend on the finding/handoff, not successful completion of the failed review, preventing deadlock. The merge row waits on latest successful independent review; head/base/interface changes invalidate affected checks/review. Preserve original IDs and findings history; no automatic checkbox resets erase evidence.

## Checkable Work Breakdown

### E-GR-SCOPE -- Requirements and preflight -> E-GR-SCOPE-requirements-and-preflight.md (7/11)

### E-GR-AUTH -- Enrollment and grants -> E-GR-AUTH-enrollment-and-grants.md (0/7)

### E-GR-KEYS -- Persistent signing keys -> E-GR-KEYS-persistent-signing-keys.md (8/13)

### E-GR-INTERFACE -- Shared component interfaces -> E-GR-INTERFACE-shared-component-interfaces.md (1/10)

### E-GR-WIRE -- Canonical registry wire -> E-GR-WIRE-canonical-registry-wire.md (0/11)

### E-GR-EVENT -- Durable revocation feed -> E-GR-EVENT-durable-revocation-feed.md (0/10)

### E-GR-TREG -- Treg action artifacts -> E-GR-TREG-treg-action-artifacts.md (0/7)

### E-GR-COMPOSIO -- Composio action artifacts -> E-GR-COMPOSIO-composio-action-artifacts.md (0/7)

### E-GR-INTEGRATE -- Composition and interoperability -> E-GR-INTEGRATE-composition-and-interoperability.md (0/7)

### E-GR-AWS -- AWS origin and infrastructure -> E-GR-AWS-aws-origin-and-infrastructure.md (0/13)

### E-GR-RELEASE -- Release acceptance and operations -> E-GR-RELEASE-release-acceptance-and-operations.md (0/16)

### E-GR-NEXT -- Remaining registry milestones -> E-GR-NEXT-remaining-registry-milestones.md (0/1)

### E-GR-M2B -- OAuth preview and production -> E-GR-M2B-oauth-preview-and-production.md (0/15)

### E-GR-M3 -- Clients runtime and production metrics -> E-GR-M3-clients-runtime-and-production-metrics.md (0/13)

### E-GR-PROD -- Terminal production delivery -> E-GR-PROD-terminal-production-delivery.md (0/10)

The preserved [optional gateway planning record](E-GR-GATEWAY-optional-managed-gateway.md) and [original gateway plan](gateway-buildout.md) are deferred records outside this active graph, following the founder's explicit registry/caller scope choice. Their IDs and open status remain unchanged; they are not required for, or dispatched by, this production delivery.

## Milestones and acceptance boundaries

| Milestone | Dependency join | Exit evidence |
| --- | --- | --- |
| Design-ready lanes | SCOPE.9 and lane-specific decisions | Accepted scoped design/write sets; no implementation completion |
| Component delivery | Each lane .6, independently | Exact reviewed/landed component receipts |
| Code integration | INTEGRATE.6 | Composed local real-store/API/MCP and compatibility evidence |
| Release/rollout | RELEASE.3/.4 | Immutable release versus deployed environment evidence recorded separately |
| Live consumer/pilot | RELEASE.5-.9 | Actual authenticated runtime/provider/ops acceptance with numerical scope |
| Historical milestone closure | EVIDENCE.5 | Truthful landed M2a file and all original Q5 obligations met |
| Remaining registry production | NEXT.0 -> M2B.14 -> M3.12 | Original Q8/Q10 plus actual new-origin acceptance; continuation, not finish |
| Entire delivery terminal | PROD.9 | Requested AWS origin currently running the fully qualified registry/caller scope with independent final evidence |
| Separate optional gateway | conditional GATEWAY.0 | Outside the confirmed production scope |

## Required validation and evidence

Implementation verification names actual package tests, including real HTTP status/body assertions for changed API routes, real PostgreSQL transactions/restart/replica behavior, OAuth browser golden/denial cases, exact artifact/digest/version fixtures and one meaningful regression per defect. Use GOWORK=off for Go. Integration runs both module tests/vet, changed-package golangci-lint, owned gofmt/goimports, Python registry unit tests and `python3 scripts/registry/check.py contracts --freeze-check`; one lease-qualified race lane covers concurrency changes. Live rows use the existing live-tag workload/wiring/runtime suites and actually pinned client/runtime builds. Required environment absence is not success or a skip waiver.

Each receipt records task, owner, full base/head/landed/source/image/config/contract digests, environment/client/runtime identifiers, commands/test counts, independent reviewer, findings/dispositions and evidence location. Private credentials, account/host/user/customer identifiers and raw production traces stay in access-controlled records. Public channel summaries are sanitized and ignored in this public repository. Source maps: app/identity/oauth/storage/events/resolution/rest/remotemcp, catalog/registry, contracts/registry/v1, scripts/registry and registry CI workflows.

Capability selection was run in planning: baseline/delivery/go profiles; CLI go/gh/aws/golangci-lint/kazi/composio available. This reports installed binaries, not auth, live permissions or runtime interoperability. Use gh for GitHub, aws CLI for AWS and configured Cloudflare MCP only if an in-scope CF need arises. No fresh code-graph tool binding was available; direct source reads are the qualified fallback. Profile suggestions do not activate plugins. The generic parser supports the declared preflight/implement/verify/review/merge/verify-landed markers; qualified agent-owned operational release/deployment rows deliberately use no unsupported stage: deploy/release token; the coordinator routes each named workflow to actual authorized AWS/Cloudflare bindings, not a generic code executor. No native controller JSON, approval, enrollment or runtime grant is produced.

Stable architecture remains in docs/design.md and RFC/ADRs. Accepted design changes amend their existing owner records during reviewed delivery; events and debugging go to docs/devlog.md. This plan links the earlier proposed integration packet and provider assessment for technical detail rather than duplicating their complete fixtures. Future stage execution must revalidate any stale permission, auth, CI, remote-main or deployment claim.

## Open decisions and risks

1. Operator/login proof, human/workload credential custody and qualified shared signing-key source: enrollment is blocked until the actual scope/custody decision; other independent offline lanes need not wait for all live decisions.
2. Actual first provider task, exact Treg endpoint/target and Composio action/account/version/license/price plus numerical envelopes: artifacts cannot invent missing schema or costs; live calls stay blocked separately.
3. Consumer readiness, existing required CI availability/policy and current deployment/tool binding: production scope is now explicit and applicable standing grants must be reused; local composition does not qualify those boundaries. Do not change OAuth restrictions or required checks to gain progress.
4. Caller revocation has a check-to-send race; unknown outcomes and already accepted effects remain honest. Memory, business hierarchy and friend-approved marketing publication are not registry authority.

## Planning handoff

Original planning handoff (superseded by authorized October4/5 ship execution): this was a draft refinement only. All new checkboxes remain open, historical checkboxes and authored evidence are retained, and no fixes/builds/tests/reviews/merges/releases/deployments/worker starts have been performed by /plan. The executing coordinator reads this file, revalidates SCOPE.1/.6/.8/.11 and keeps scheduling/refining dependency-ready in-scope stages until T-GR-PROD.9 is true; no routine proceed-confirmation at local merge, rollout or NEXT planning checkpoint. External missing choices remain coordinator-routed. Planning validation is parser/graph/coverage and artifact consistency, not application acceptance.

Planning validation (2026-10-04): installed parser accepted 77 open tasks in 11 epics with all six supported code-stage markers; every task has owner, acceptance and wave assignment. Additional graph checks found no missing dependency, cycle, duplicate local ID, malformed acceptance, unguarded coding merge or TOC count mismatch. Exactly two outline epics have one trigger planning task each. Hash checks confirm preexisting plan/design contents remain byte-identical prefixes and the gateway plan is untouched. Whitespace checks passed; no application checks were run. Parser IDs namespace the preserved local IDs for display; they are not minted lifecycle or claim IDs.

## Production completion and continuation contract — 2026-10-04 refinement

The founder explicitly confirmed registry plus caller-owned integrations and selected https://gist.sire.run on AWS. `production_done` is true only when T-GR-PROD.9 has fresh independent evidence of all required registry/caller milestones, canonical external DNS/TLS/health/origin/OAuth boundary, expected live AWS service/task/image/config/private storage, authenticated publish/discover/exact-get/resolve/events, real bounded Treg/Composio caller/runtime receipts, all required original M2a/M2b/M3 clients/metrics, complete preview cleanup and qualified monitoring/recovery/ownership. A local pass, PR, merge, initial rollout, website200, drafted plan or unsupported client is insufficient.

The executing coordinator persists task outcomes and receipt dependencies after every lane boundary. Within already accepted scope it refills Luna slots, invokes dependency-triggered planning refinements, assigns accepted findings to full fix/verification/review/merge/landed chains, rebuilds and redeploys changed service inputs, reruns affected live acceptance and continues. It does not stop just because one epic is done or because ordinary Markdown lacks a deploy stage token. Real denied tools/credentials, absent operator/provider budget, rejected auto-approval or material scope changes are recorded concrete blockers; they are not silently bypassed. No worker can accept its own code or production findings.

Known AWS, M2b/M3 and final production obligations are now decomposed to fulfill the user's explicit complete-SDLC request; every preflight must revalidate actual immutable inputs before dispatch. User/domain decisions, cloud inventory and numerical limits are not invented. The earlier77-task validation is historical evidence of that earlier revision only; validate this new graph and counts before handing it off.

Official deployment references used for this planning refinement: ALB HTTPS requires a matching certificate ([AWS listener documentation](https://docs.aws.amazon.com/elasticloadbalancing/latest/application/create-https-listener.html)); ECS rolling deployment supports failure detection/rollback and image-digest consistency ([AWS deployment documentation](https://docs.aws.amazon.com/AmazonECS/latest/developerguide/deployment-type-ecs.html)). These support the task design, not an assertion that the deployed service already meets it.

Tooling observation for this refinement: aws, pulumi and gh binaries are available; no Cloudflare MCP function was exposed in the current tool inventory. T-GR-SCOPE.11 must qualify the configured binding before DNS writes. This is a real execution-capability prerequisite, not permission to use a generic connector/browser or change organization OAuth restrictions. No AWS authentication, stack apply or DNS mutation was attempted here.

Historical planning-only validation (2026-10-04, before shipping started): 128 open active tasks in 14 epics; default docs/plan.md parses those tasks once alongside 18 unchanged historical completed tasks (146 total). All 127 other active tasks are ancestors of terminal T-GR-PROD.9. Unique IDs, resolved acyclic dependencies, owner/acceptance/wave coverage, all six supported code stages, independent-review dependencies, guarded merges, split TOC counts and whitespace checks passed. All 77 prior recovery IDs/statuses are retained: 76 remain active and the unchanged optional gateway trigger is preserved in its deferred file. Prior plan/design/devlog contents remain byte-identical prefixes; ADR008 and the original gateway plan are unchanged. These are planning-artifact checks only; no application test or execution was run.

## Shipping checkpoint — 2026-10-04

User invoked /ship. T-GR-SCOPE.1/.2/.6/.8 have source-only receipts in docs/receipts; component contracts await independent T-GR-SCOPE.3 disposition before engineering admission. Four slots are occupied by one coordinator and three explicitly requested GPT-6-Luna workers, with isolated external-SSD ownership. Engineering builds are held while one-minute host load exceeds 10; no build lease is bypassed. Production bindings are partially qualified but blocked by AWS session/stack-account mismatch and absent configured Cloudflare MCP. Enrollment and provider targets/caps are pending a decision brief requested by the founder. No production_done, code/lifecycle admission, CI pass, release or live acceptance is asserted. Read-only preflights are not application acceptance.

Current shipping validation: 128 total active production tasks, five source-only preflights checked and 123 open; deferred gateway is outside that count. Founder identity/pilot choices remain pending. Consumer owner reports its actual runtime/header/funding qualifications are not yet admitted; T-GR-SCOPE.7 stays blocked rather than accepting its planning PR as runtime evidence. Exact source review findings R1 (open-PR accuracy) and R2 (historical/current count distinction) were corrected in this candidate; fresh independent review is still required.


## October5 source delivery continuation

PR48 landed the original reviewed production graph. KEYS/WIRE/EVENT are admitted source components; trust, live pilot and AWS/DNS operator bindings remain held. KEYS has three accepted loader findings tracked as7/8/9, with independent re-review required. Shared-interface source delivery is now a separate prerequisite so dependent components can land without completing the held final app composition. Graph rows retain one terminal production gate and the same registry/caller scope.

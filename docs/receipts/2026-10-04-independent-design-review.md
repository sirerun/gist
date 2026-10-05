# Independent design review — T-GR-SCOPE.3

Date: 2026-10-04

Candidate repository: `sirerun/gist`

Candidate base: `84e14128565058419a9b90619f27fa517ba4f494`

Candidate head: `e289a49fca0d232e988a98685588625f367a91ca`

Reviewer: independent non-author agent; did not author or coauthor candidate changes.

## Verdict

**BLOCK** pending correction of two material preflight/plan record errors below. The proposed KEYS, WIRE, and EVENT component boundaries otherwise fit the inspected source and accepted ADRs. This source/documentation review provides no build, CI, AWS, Cloudflare, live-service, or production qualification.

## Scope and evidence

Reviewed the complete documentation-only candidate diff, proposed lane contract, split task graph, source preflight receipts, `hosted/internal/identity/keys.go`, app key/event/resolver composition, `hosted/internal/rest/events.go`, `hosted/internal/resolution/resolve.go`, `hosted/internal/storage/revocations.go`, `hosted/migrations/004_events.sql`, frozen contract sources, and ADR-005/007/008. `git diff --check` was clean.

An independent static parse of the 14 production epics found 128 tasks: 4 checked and 124 open. It found all six supported stages, no duplicate IDs, missing dependencies, or cycles, and all 127 other active tasks are ancestors of `T-GR-PROD.9`. The optional gateway trigger remains a separate deferred task. This was a read-only Markdown graph check, not application validation or a claim that the absent standalone `plan` command ran.

## Findings

### F1 — P2 — Source preflight falsely reports no open pull requests

`docs/receipts/2026-10-04-source-preflight.md:6` says “Open pull requests: none at inspection.” The candidate's own `docs/receipts/2026-10-04-ci-preflight.md:17` records open PR #47 (`bdd7d3e4f2186cfcd7aba8b708b8099539bf99e6`), and the coordinator's fresh explicit `gh --repo sirerun/gist` check reconfirmed #47. This contradicts T-GR-SCOPE.1's requirement to record current open PR heads and makes the source inventory unreliable.

**Proposed disposition:** amend the source-preflight receipt to name #47 and its observed head/state from the cited fresh check; recheck the complete open-PR list before integrating the receipt. Keep the existing warning that all remote observations must be refreshed before later dispatch/merge.

### F2 — P2 — Final plan receipt confuses total tasks with open tasks

`docs/plans/registry-recovery.md:144` says “128 open active tasks,” while its TOC at `docs/plan.md:520` correctly reports `(4/128)`. The 128 production tasks include four completed preflights, so 124 remain open. The separate deferred gateway trigger is not part of those 128.

**Proposed disposition:** change the sentence to “128 active tasks (4 complete, 124 open)” and retain the 127-ancestor statement. The independent graph check confirmed that ancestor count.

### F3 — P2 — Freeze existing event cursor limits and exercise the durable cap

The receipt at `docs/receipts/2026-10-04-requirements-and-lane-contracts.md:14,24` correctly requires PostgreSQL cursors/feed, atomic revoke-plus-event persistence, RLS isolation, a context-aware storage boundary where required, uniform cursor denial, and bounded retention/cursor count. The task at `docs/plans/E-GR-EVENT-durable-revocation-feed.md:8,16` promises to preserve existing cursor contracts, but does not name the current bounds or test the cap after restart/replica changes. The inspected source defines seven-day event retention, five-minute cursor expiry (renewed on read), a 16-cursor-per-principal cap that evicts the oldest cursor, and 100 events per page (`hosted/internal/events/feed.go:34-44,60-79,96-116,139-153`; `hosted/migrations/004_events.sql:33-40`).

**Proposed disposition:** treat these values as the compatibility baseline unless an explicit ADR disposition changes them. Add the per-principal cap/eviction and page-bound cases to the durable PostgreSQL acceptance matrix; enforce the cap safely across replicas under tenant RLS and request context. Retain 409 `cursor_expired`/uniform foreign-or-missing denial and 503 for store failure. This is a lane-acceptance clarification, not evidence that the current in-memory feed meets production durability.

## Accepted architecture and boundaries

- **KEYS:** `hosted/internal/app/app.go` currently generates a new EdDSA key at startup. The proposed explicit key-ring loader and replica-identical operator-provisioned key material address that defect. Private-key custody remains with the operator-owned secret store; this review grants no new secret provisioning, credential, or AWS access. Keep the no-PostgreSQL-private-key rule, reject mismatched/unknown key material without logging bytes, and retain bounded old-public-key overlap for token verification.
- **WIRE:** the current app resolver decodes legacy `skill`, `runtime_id`, `local_execution`, and `owned_connections` fields; `resolution.Resolve` hardcodes selected bindings to `1.0.0`. A separate canonical adapter, exact pin/digest closure, REST/MCP parity, and actual binding versions address observed gaps while preserving the frozen v1 schema. Any wire change needs an explicit ADR/finding path and frozen-fixture updates through the owner.
- **EVENT:** migration 004 already defines tenant-scoped outbox/cursor tables with forced RLS and seven-day event purge, but app composition uses `events.NewStore`; current revocation commits in `storage.RevokeVersion` before an app-level event append. The proposed same-transaction outbox, durable principal-bound cursors, no process-local fallback, preserved forced-RLS transaction context, and integrator-owned wiring match the actual defect and write ownership. Existing 30-second policy freshness does not make external provider dispatch atomic; `docs/plans/registry-recovery.md:124` correctly retains the check-to-send race, unknown outcomes, and already-accepted effects.
- **Scope and lifecycle:** the plans keep registry authorization separate from caller execution; retain enrollment/custody, consumer funding/import, numerical provider bounds, AWS role/account, configured Cloudflare DNS binding, and preview limits as explicit prerequisites; preserve the selected `https://gist.sire.run` AWS production outcome; and require separate exact-head review, guarded merge, landed verification, immutable release, authorized rollout, live acceptance, teardown, and final production evidence. The candidate makes no production-ready or CI-pass claim. Its additions contain no credentials, account IDs, local home paths, or customer identifiers.

## Disposition

T-GR-SCOPE.3 remains **BLOCKED** until F1 and F2 are corrected in the candidate evidence and the final candidate head is re-reviewed. F3 should be incorporated into the EVENT acceptance record before the event implementation preflight is accepted. No source changes were made.

## Exact-head follow-up review

Candidate follow-up base: `84e14128565058419a9b90619f27fa517ba4f494`

Candidate follow-up head: `bfe0e55529fe2c4aebc7bc59299276949623afc4`

The follow-up worktree was created independently from that exact candidate head; it contains no edits by this reviewer. The reviewed update corrects F1 by recording PR #47 and PR #48, corrects F2 by distinguishing historical counts from current task counts, and updates the active shipping checkpoint to 128 production tasks, five completed preflights, and 123 open tasks. Fresh static graph checks still found 128 tasks across 14 production epics, five checked and 123 open, all six stage markers, no duplicate IDs, missing dependencies, or cycles, and 127 ancestors of `T-GR-PROD.9`. F1 and F2 are **closed**.

The exact event cursor compatibility baseline is now recorded as a review disposition: seven-day event retention, five-minute cursor expiry renewed on read, at most 16 open cursors per principal with oldest-cursor eviction, and pages capped at 100 events. Durable EVENT verification must preserve those limits under forced RLS across restarts/replicas and test cap eviction, cursor expiry, retention gap, foreign/missing cursor denial, and store failure (409 versus 503); the task's existing preflight stage can finalize fixtures before implementation admission.

One new hygiene finding remains: **F4 — P3 — `git diff --check` fails on two new provider-preflight lines.** The follow-up candidate adds two trailing spaces after `Date: 2026-10-04` and `Scope: T-GR-SCOPE.10 source preflight only` in `docs/receipts/2026-10-04-provider-preflight.md:3-4`. These appear intended as Markdown line breaks, but the repository check reports them and the candidate's historical planning validation says whitespace checks passed. Remove the trailing spaces or use a clean Markdown layout and rerun the diff check.

**Follow-up verdict at bfe0e55: BLOCK pending F4 cleanup.** The new enrollment brief remains explicitly proposed/unapproved, adds no identity or credential grant, and states that no GitHub app or AWS secret was created. The provider preflight keeps Treg/Composio actions unadmitted and reports a public metadata-only Treg catalog GET, no credentialed session, and no provider action. This review made no provider, cloud, or network call.

## Final exact-head re-review

Final candidate base: `84e14128565058419a9b90619f27fa517ba4f494`

Final candidate head: `59a9368f1e8e3780f7a5805e9076850143614a7a`

At the final exact head, the provider receipt replaces the two trailing-space Markdown breaks with clean lines and `git diff --check 84e14128565058419a9b90619f27fa517ba4f494..59a9368f1e8e3780f7a5805e9076850143614a7a` is clean. F4 is **closed**.

The contract receipt and EVENT.0/.2 now freeze and test seven-day event retention, five-minute sliding cursor TTL, 16 cursors per principal with oldest eviction, and 100 events per page. They also require tenant-aware retention-floor handling for inter-tenant sequence gaps and budget-failure behavior that cannot silently lose unreturned events. EVENT owns the storage-level atomic revocation/outbox write; INTEGRATE retains the app-level publisher/revoker adapters and wiring. Migration IDs 008/009 remain conditional and are guarded by a fresh-main check. F3 is **closed**.

The updated SCOPE.9 admits only individually qualified lanes and records held AUTH/provider choices without blocking independent qualified work. Production DNS writes remain held on the disabled/unavailable Cloudflare binding; AWS account/operator mismatch, consumer runtime/funding admission, human enrollment, and numerical provider bounds remain explicit gates. No approval, credential, billable provider action, cloud mutation, or deployment permission is inferred from the proposed briefs.

Independent final graph recheck: 128 active production tasks in 14 epics, five checked and 123 open; all six supported stages; no duplicate IDs, missing dependencies, or cycles; 127 prerequisite tasks reach `T-GR-PROD.9`. `git diff --check` passes on the exact full base/head. The candidate is documentation-only. Frozen ADR-005/007 and canonical v1 remain unchanged; the proposed KEYS/EVENT/WIRE file and migration ownership is disjoint, key custody is explicitly operator-owned and ungranted, caller revocation retains the check-to-send race, and public receipts contain no secret values, account IDs, home paths, or customer identifiers.

**Final verdict: PASS — T-GR-SCOPE.3 design review accepted at exact head `59a9368f1e8e3780f7a5805e9076850143614a7a`.** F1–F4 are closed. This is an independent source/documentation review only; it does not qualify code, local builds/tests, GitHub CI, AWS, Cloudflare, runtime, live acceptance, or production.

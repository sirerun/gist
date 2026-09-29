# Handover — 2026-09-28, session f8dff885 (coordinator, David's Mac mini)

## TL;DR
Finished the RFC-002 registry AWS deployment (live, healthz 200 over HTTPS)
and the RFC-003 gateway design review (RFC-003 amended, ADR 010 written,
private threat model written; all merged). Stopped because the two remaining
gates are founder-only: the G0.4 three-card sign-off and the credential-path
decision. Single next action: re-present the G0.4 sign-off cards, then
dispatch Wave 1.

## Done & VERIFIED
- **registry-aws stack applied and live** — foundation PR #264 merged
  (pin gist ef72336 image + certValidated); `pulumi up --stack registry-aws`
  applied (4 created, 2 updated, 1 replaced; rc=0). Verified live:
  `https://registry.sire.run/healthz` → 200 `{"status":"ok"}`; HTTP :80 →
  301 redirect to HTTPS; unauthenticated `/api/v1/registry/packages` and
  discovery → 401 (correct posture). ACM cert ISSUED, both ALB listeners
  confirmed via AWS API.
- **G0.1+G0.3 merged** — gist PR #43: RFC-003 status "Reviewed 2026-09-27"
  with ADR 009 amendments; ADR 010 (enums, state machine, approval JWS,
  idempotency, contracts/gateway/v1, event kinds, adapter interface).
- **G0.2 merged** — foundation PR #265: private threat model
  (security/gist-gateway-threat-model.md, TM-01..TM-16).
- **Plan ticks merged** — gist PR #44 (G0.1–G0.3 ticked; 02494bf on main).
- **Q5 annotation merged** — gist PR #42: live-verification recorded,
  round trip deferred.
- **Sitrep delivered** — docs/sitrep/2026-09-28.md (on handover branch).
- **Local main reconciled** — local main had 10+ unpushed lane commits, all
  patch-equivalent to merged upstream PRs (verified via `git cherry` all
  `-`). Reset --keep to origin/main; backup tag `backup-main-20260927`.
  contracts freeze-check now PASS.

## Done but UNVERIFIED
- Foundation PR #256 ("production-aws: Sire production on AWS, composed from
  AMSL (not applied)") — open, explicitly unapplied, NOT this session's work;
  do not confuse with the applied registry-aws stack.
- The threat model's TM rows are design claims; the G4.1 test matrix that
  proves them does not exist until Wave 3.

## In flight
- **G0.4 sign-off (blocked on David)** — three cards were presented twice
  (timed out, 600s each). A one-shot cron (39cb1d77) was scheduled to
  re-present at 20:37 on 2026-09-27 — session-only, likely died with this
  session; re-present manually. After approval: record sign-off in ADR 010
  status + threat model header, tick G0.4, dispatch Wave 1 (G1.1–G1.7 as
  cloud Sonnet agents; isolation "remote", model sonnet; each opens a PR;
  re-verify locally under the build lease before merging).
- **Credential-path decision (blocked on David)** — options: (a) defer
  round trip to gateway identity work (recommended, already in plan via PR
  #42); (b) build bootstrap-credential capability (~half day + rebuild);
  (c) manual DB seed (violates no-manual-DevOps; needs explicit exception).
- **foundation worktrees (mine, all merged → safe to delete)**:
  foundation-wt-pin (registry-aws-pin-ef72336, e997b14 = merged origin/main),
  foundation-wt-tm (tm-g0-2, c0fe4c9 = merged), foundation-wt-dns-check
  (registry-aws-dns-check, e39caa3, merged as foundation PR #263 — verify
  before deleting).

## Blocked
- **Wave 1 dispatch** — unblocks on G0.4 sign-off (David). ADR 009 decision
  1: no build lane starts before sign-off.
- **Q5 / M2a closure** — unblocks on the credential-path decision + a real
  client walkthrough. Evidence file docs/registry/gates/M2a.json must be
  created; the evidence gate correctly fails until then (only remaining
  gate failure on main).

## Running processes left alive
- None. Background pulumi/CodEBuild tasks from 2026-09-27 completed or were
  killed and re-run to completion. No kazi converges. The only cron
  (39cb1d77, G0.4 card re-presentation) is session-only and dies with this
  session — noted above.

## Landmines & context
- **Never run pulumi from `/Users/dndungu/Code/sirerun/foundation/pulumi`** —
  that checkout sits on the stale branch `fix/pulumi-targeted-no-refresh`
  and diffs the OLD GCP program against the registry-aws stack (both 2026-09-27
  failures were exactly this). Apply from a worktree at origin/main:
  `cd /Users/dndungu/Code/sirerun/foundation-wt-pin && git rebase origin/main`
  then `pulumi up --stack registry-aws --yes --non-interactive`.
- **Standing permission**: `pulumi up` on the registry-aws stack only, after
  local validation. Nothing else against AWS is pre-authorized.
- **Build lease**: claim with `CLAIM_REMOTE=/Users/Shared/mini-build-lease.git
  ~/.agents/skills/claim/scripts/claim.sh claim R-build-lease --purpose ...`;
  gate runs on WON, release BY SHA. A stale claim ref (e3972a1, from a
  2026-09-26 coordinator test) sits in refs/claims — if a claim fails
  mysteriously, that ref is why.
- **A stale plan reference**: when G3.3 merges, also amend the "Offer only
  ... four" line at docs/plans/registry-buildout.md:698 (the four-tool MCP
  surface grows to five with gist_invoke).
- **Pre-existing main gate failures**: only `evidence --milestone M2a` fails
  (M2a.json absent; Q5 open). contracts freeze-check passes on current main.
- **Working-tree deletion `D CLAUDE.md` on main** predates this session and
  was NOT restored (deliberate deletion by someone else; the file's content
  is recoverable from HEAD if wanted).
- **Foundation repo main branch is on the stale fix branch locally** — the
  main checkout of foundation itself (1782026, fix/pulumi-targeted-no-refresh)
  diverges from origin/main; origin/main (e997b14+) is truth. A future
  session should reconcile it the same way gist main was reconciled (git
  cherry to prove patch-equivalence, then reset --keep, backup tag first).
- **gist go.work / local test invocations**: registry tests run as
  `(cd hosted && GOWORK=off go test ./...)`; gates via
  `python3 scripts/registry/check.py {contracts --freeze-check|evidence --milestone M2a}`.
- **Luna branches** (codex/luna-discovery-*, cursor/luna-discovery-g04) in
  this repo belong to ajent-social sessions — NOT ours, do not touch.

## How to resume
1. `cd /Users/dndungu/Code/sirerun/gist && git fetch origin && git checkout
   main && git reset --keep origin/main` (main is truth; handover branch
   `handover` carries docs/handover.md + sitrep).
2. Read this file, then docs/plans/gateway-buildout.md (the wave plan) and
   docs/plans/registry-buildout.md (Q5).
3. Re-present G0.4 sign-off cards (3 AskUserQuestion cards: RFC-003 review,
   ADR 010, threat model) + the credential-path card.
4. On approval: record sign-off (ADR 010 status + threat model header, PR),
   tick G0.4, dispatch Wave 1 lanes G1.1–G1.7.
5. Claims: check `git ls-remote origin "refs/claims/*"` before claiming the
   build lease; pickup log lives at .claude/scratch/registry-buildout-pickup.md.

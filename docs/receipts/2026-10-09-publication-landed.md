# Publication source actual landing

[PR60](https://github.com/sirerun/gist/pull/60) merged by guarded GitHub rebase. This closes publication source delivery only.

- Reviewed base: `8a24c72a596a757e13426945911d87446cc3a4e0`.
- Final reviewed head: `ea119d97370a4aea7a9bd822a84dda50ac352323`.
- Actual landed SHA: `eb1bd86596b61bd423cc3f75757a2bdd38682c24`.
- Reviewed and landed tree: `e3211491965e3c5d18cd302de639b4933cb3de44` (exact equality).
- Fresh remote main: `eb1bd86596b61bd423cc3f75757a2bdd38682c24`; landed revision is reachable.
- Merge time: `2026-10-09T05:58:23Z`.
- [Recorded final independent approvals](https://github.com/sirerun/gist/pull/60#issuecomment-6075244227).

## Independent final-head review

### Independent nonauthor Luna lead

## APPROVE

**Base:** `8a24c72a596a757e13426945911d87446cc3a4e0`
**Head:** `ea119d97370a4aea7a9bd822a84dda50ac352323`

I verified both refs and reviewed the complete base-to-head diff, direct app/REST/storage/resolution callers, the relevant PUBLISH plan rows, and the publication receipts. No source was changed, and I ran no builds or tests.

**Findings:** None. In particular, the shutdown admission fence and active-handler drain keep the object root and SQL pool alive until admitted requests and janitors return. Repeated shutdown retains the actual cleanup result; the caller’s deadline still bounds its wait. V2 remains opt-in, the approved port additions are explicitly recorded in the implementation-v2 amendment, and the frozen wire directories and locks are unchanged.

The final plan-check receipt reports Wazi 278 valid, with reader compatibility, schema and semantic checks passing and `authorityAuthenticated=false`. The retained R6 checks are local evidence, not hosted CI success.

**Material limitations:** This approves source review only. The recorded checks do not qualify live S3, actual providers, a production verifier or trust configuration, deployment, or production acceptance. Production gates remain open. Fake S3 and offline synthetic conversions are not live-provider qualification.

### Independent nonauthor Luna storage

# APPROVE

**Base:** `8a24c72a596a757e13426945911d87446cc3a4e0`
**Head:** `ea119d97370a4aea7a9bd822a84dda50ac352323`

**Findings:** None.

I verified both pinned commits, reviewed the complete base-to-head diff and the active-drain changes with their callers, and confirmed the worktree stayed clean. The admission fence serializes `enter` against `stop`; shutdown stops admission before draining, waits for both janitors and admitted handlers after forced connection close, then closes the object store and SQL pool. A caller deadline returns promptly while exactly-once cleanup continues, and later callers receive the retained shutdown error. I found no regression in concurrent close or the nil-store test fixture path.

The retained controlled TLS/store RED demonstrates the former active-handler race; the corrected test covers the admitted read, late-admission 503 and eventual root close. The receipt reports the R6 hosted checks passing at `a0df51a`; the only later changes are documentation. The final Wazi 278 record matches the current `docs/plan.md` digest and reports reader compatibility, schema validity and semantic validity. `git diff --check` passes.

**Limits:** This is source review, not a rerun of checks. Evidence does not qualify live S3, provider behavior, a production verifier or operator trust, deployment, or production acceptance. V2 remains opt-in and production gates remain open.

## Actual landed checks

Executed at actual landed SHA in a fresh isolated worktree, after proving reviewed-tree parity and fresh remote-main reachability. Runtime PostgreSQL remains explicitly NOSUPERUSER/NOBYPASSRLS; successful catalog records come from actual authenticated HTTP or named transactional storage tests, not seeded publication success.

| Portable command form | Result |
| --- | --- |
| `go test -tags=integration ./internal/app ./internal/storage -count=1` | exit0 |
| `go test -tags=integration ./acceptance/wiring ./acceptance/retrieval -count=1` | exit0 |
| `go vet ./...` | exit0 |
| `golangci-lint run --config "$GIST_QUALIFIED_LINT_CONFIG" --new-from-rev=8a24c72a596a757e13426945911d87446cc3a4e0 ./...` | exit0 |

Also passed actual landed contract freeze, nine Python registry checks, v2 strict fixtures/OpenAPI validation, and full pinned Wazi syntax/actual-reader/schema/owning-semantic conformance for278tasks. authorityAuthenticated=false is retained; plan conformance grants no runtime authority. Root/hosted unit/race/build and affected verification from the final source are reusable because the complete reviewed/landed trees are equal; fresh full app/storage and wiring/retrieval acceptance ran at the landed SHA.

Local receipt integrity:

- publication-landed-provenance.json: `sha256:4120e7ba148b1a467d4c16c90beb14e1a81acb803840ca33d2d3ecc5d40dbbda`.
- publication-quality-landed-hosted/result.json: `sha256:bfdb9809661a7bb1d909f2cdd4d8d12cb32a88eb40c9a6f4d45301b638fd67dd`.
- publication-quality-landed-root/result.json: `sha256:4a03e3e5195e682505d2f9af8120b9b6f20c62789b0853f783a81dc6094b4a37`.
- publication-landed-plan-check.log: `sha256:93f2d4399752a8b0bc1bb78bcae498322581b4d086fbe5bf9f7f3b9964621f0d`.

## Current open production gates

All seven PR60 final-head hosted jobs did not start because of account billing. Local evidence is the authorized delivery fallback; no protections, rules, billing or commit statuses were changed. Five known full-hosted lint findings reproduce at base; root full lint and the hosted source delta pass.

Fresh default AWS STS returned exit253/NoCredentials. A single unauthenticated health read from DGX failed DNS resolution for gist.sire.run (curl exit6; no HTTP response). This proves only that resolver observation, not a global DNS state or production health. No AWS deployment or DNS mutation ran. Existing Cloudflare capability does not supply an AWS origin or operator binding.

Invited-operator issuer/workspace/enrollment custody, actual Treg/Composio capture/license/account/target/numerical authority, admitted real consumer and production verifier, AWS account/role/stack/image/config/migration release and rollout/live acceptance remain open. V2 source is opt-in; it has not been enabled in a deployed service. The approved scope remains registry plus caller-owned integrations; no managed execution gateway or Zatiti prerequisite was introduced.

This evidence closes PUBLISH.4/.5/.6/.17/.20 against actual PR60 review/merge/landing. Earlier source, preflight, negative reviews and runtime RED/fixture-failure receipts remain preserved. No terminal production row is marked complete.

Public command tables use portable tool/config names. Exact qualified interpreter, derived lint configuration, cache and worktree paths remain in the private hash-pinned raw command receipts; this normalization changes no invocation result or source revision. Original public Git history is retained; this correction sanitizes current receipt text and does not claim historical erasure.

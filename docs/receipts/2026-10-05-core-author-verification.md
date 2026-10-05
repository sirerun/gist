# CORE author verification checkpoint

This receipt records local verification evidence for the CORE source candidate. It is not an independent review, merge authorization, landed verification, deployment qualification, or final CORE approval.

## Bound source and scope

- Candidate PR #55: `bd5d4d9c88cea35425a1d306cde5f424f6a42e90`.
- Candidate base: `d75ac183fe8b949a4f1381e8ebca3226f0c0e282`.
- Previously verified source: `6243787610459815dec8240620d14076e54e61f9`.
- The candidate source delta from 624 is documentation plus `hosted/internal/app/core_publication_transaction_integration_test.go`. No production Go source changed. The added integration assertion confirms that rollback leaves the staged content-addressed blob on disk and describes the separate retention/reconciliation boundary.

## Finding regression evidence

| Finding | Before fix | Corrected source | Evidence |
| --- | --- | --- | --- |
| CORE-R1 bounded retention catch-up | Historical RED replay details not available in this verifier's execution record. | `bd5d4d9c88cea35425a1d306cde5f424f6a42e90` | `TestCoreMaintenanceBoundsBacklogAndCatchesUp` passed in the exact-candidate tagged PostgreSQL app regression set below. |
| CORE-R2 strict maintenance target decoding | Baseline `b715bd9352ab75e35d58353a1c374ab769b49961` with the affected CLI test overlaid failed: `go test -count=1 -run '^TestMaintenanceTargets' ./cmd/registry` accepted four duplicate/ambiguous configuration entries. | `699d458` | The same command passed (0.575s). Exact PR55 app/cmd tagged tests, vet, and lint also passed below. |
| CORE-R3 target failure isolation | Historical RED replay details not available in this verifier's execution record. | `bd5d4d9c88cea35425a1d306cde5f424f6a42e90` | `TestCoreMaintenanceContinuesAfterTargetFailure` passed in the exact-candidate tagged PostgreSQL app regression set below. |
| CORE-R4 startup database deadline | Historical RED replay details not available in this verifier's execution record. | `bd5d4d9c88cea35425a1d306cde5f424f6a42e90` | `TestCoreMaintenanceStartupUsesConfiguredDeadline` passed in the exact-candidate tagged PostgreSQL app regression set below. |
| CORE-R5 exact bounded retention completion | `476a93b` (test over `8f07de0`) failed at exactly 20,000 expired rows with “bounded catch-up exceeded”. | `7a46b2d` (test over `c0a137e`) | `TestCoreMaintenanceAcceptsBacklogAtBound` and the 20,001-row bound regression passed; race, app vet, and tagged lint passed on the corrected source. The exact-candidate tagged PostgreSQL suite below also passed both boundary tests. |
| CORE-R6 publication response budget | Historical RED replay details not available in this verifier's execution record. | `bd5d4d9c88cea35425a1d306cde5f424f6a42e90` | The exact-candidate `TestPublisherCatalogAndOutboxShareTransaction` passed; the final candidate adds the staged-blob rollback persistence assertion. |
| CORE-R7 maintenance Unicode authority input | `3d7362f` failed `go test -count=1 -run '^TestMaintenanceTargets' ./internal/app`: all five malformed Unicode cases were accepted. | `963217d` | The same command passed (0.598s). |
| CORE-R8 shutdown caller deadline | `70d7f98` failed `go test -count=1 -run '^TestShutdown' ./internal/app` on bounded janitor wait and retained cleanup error. | `43782b1` | The same command passed (0.441s). The actual blocked PostgreSQL maintenance query/cancellation fixture and race passed at source `6243787`; the exact-candidate app suite below also passed it. |

For R1, R3, R4, and R6, this receipt has exact-candidate corrected-source evidence but does not claim a reproduced pre-fix RED. Their historical replay details should be attached by the relevant author if required before the review graph is marked complete.

## Exact candidate commands and results

Commands ran from `hosted/`, with Go caches/temp on the external SSD, `GOMAXPROCS=2`, and the fresh-load gate helper. Multi-package checks held and released the shared mini build lease. The PostgreSQL fixture URL was injected from the authorized local fixture configuration and is intentionally omitted from this receipt.

- Initial tagged selection without the fixture environment exited at each test's explicit `GIST_DATABASE_URL is required` precondition; no assertions ran. This is setup evidence only, not a product failure.
- `go test -count=1 -tags=integration -run '^(TestPublisherCatalogAndOutboxShareTransaction|TestCoreHTTPRejectsForeignCursorIssuerAndStalePolicy|TestCoreMaintenanceBoundsBacklogAndCatchesUp|TestCoreMaintenanceAcceptsBacklogAtBound|TestCoreMaintenanceContinuesAfterTargetFailure|TestCoreEventJanitorStopsWithParentAndShutdownClosesPool|TestCoreMaintenanceStartupUsesConfiguredDeadline|TestCoreShutdownCancelsBlockedMaintenanceQueryBeforeClosingPool)$' -p=2 ./internal/app` — PASS (1.728s).
- Same selected command with `-race` — PASS (3.082s).
- `go test -count=1 -tags=integration -p=2 ./internal/app ./cmd/registry` — PASS (app 1.990s; CLI 0.525s).
- Same tagged app/CLI command with `-race` — PASS (app 5.318s; CLI 1.634s).
- `go vet -tags=integration -p=2 ./internal/app ./cmd/registry` — PASS.
- `golangci-lint` v2.13.2 with the migrated v2 config and `--build-tags=integration ./internal/app ./cmd/registry` — PASS, 0 issues. The repository's native config is v1 format and this lint binary requires v2; the migrated config was stored separately on SSD.
- Full hosted `go vet -p=2 ./...` — PASS.
- Full hosted `go test -count=1 -p=2 ./...` — 17 packages passed. `acceptance/retrieval` did not execute its cases: both tests stopped during `TestMain` target setup because the existing R9 local target does not yet supply the required explicit signing key configuration and maintenance targets (`GIST_SIGNING_KEY_CONFIG` rejected). R9 fixture adaptation is pending; the full hosted unit suite remains open and must be rerun after that fixture lands.
- `git diff --check d75ac183fe8b949a4f1381e8ebca3226f0c0e282..bd5d4d9c88cea35425a1d306cde5f424f6a42e90` — PASS.

No hosted CI, provider, production credential custody, deployment, external enrollment, or production acceptance status is claimed.

## Final R9-integrated candidate follow-up

The later exact PR #55 candidate is `a2379fda6bcccc29ee2700c75e95596cfcb2310a` (base remains `d75ac183fe8b949a4f1381e8ebca3226f0c0e282`). Its delta from bd5 is limited to acceptance fixtures and plan text. The production app and CLI package files are byte-identical to bd5, so the affected app/CLI tagged PostgreSQL, race, vet, and lint evidence above carries across this fixture-only update.

- Full hosted `go test -count=1 -p=2 ./...` with the explicit local PostgreSQL fixture configuration — PASS across all packages, including retrieval acceptance (1.357s) and the R9 startup fixture.
- Full hosted `go vet -p=2 ./...` — PASS at exact a237.
- The full hosted unit run supersedes the earlier bd5 checkpoint where retrieval acceptance had stopped during setup.
- CORE-R1/R3 historical RED replay used an isolated compatibility-only branch `8546928bb5a0766d7552f062c1162ac26cd1fb98` based on production source `70636b567b131d1e6866200cd5d717b1eac9a201`. It copied the preserved `core_maintenance_integration_test.go` and changed only the unrelated Go 1.27-incompatible comparison in `resolution_store_integration_test.go` to `reflect.DeepEqual`; no production files changed. The exact gated command was `go test -count=1 -tags=integration -run '^(TestCoreMaintenanceBoundsBacklogAndCatchesUp|TestCoreMaintenanceContinuesAfterTargetFailure)$' -p=2 ./internal/app`. R1 failed because the bounded-backlog function returned nil; R3 failed because the valid target remained unprocessed with one event. The original preserved red worktree was not modified.
- CORE-R4 historical RED replay at `f0eabf62259b1a1e2a0b74387d929e9e8c4a5713` ran `go test -count=1 -tags=integration -run '^TestCoreMaintenanceStartupUsesConfiguredDeadline$' -p=2 ./internal/app` with the unmodified test and failed in 3.13s with `startup ignored configured maintenance timeout`. Corrected-source counterpart passed in the exact bd5 selected app integration suite.

The R1/R3/R4 red gaps noted in the bd5 snapshot are now filled. All verification remains local. No final independent CORE review or approval is claimed by this author receipt.

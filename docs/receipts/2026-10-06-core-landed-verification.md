# CORE actual-landed verification — T-GR-CORE.6

Date: 2026-10-06. Outcome: **PASS for source delivery; production remains open.**

PR [#55](https://github.com/sirerun/gist/pull/55) rebase-merged on 2026-10-05 at `b91ec511153c5e2ed57569be532e0401ab8121a8`. Reviewed head `805d6b06878cc3d67ccdd843a96000e6afe99482`, base `d75ac183fe8b949a4f1381e8ebca3226f0c0e282`. Reviewed and landed trees both equal `23ecc7401e39a853d7a2f6e64f55c323e54afc61`. Fresh remote main still resolved to that source before these checks. Independent final approval is [recorded here](2026-10-05-core-final-head-approval.md), following the linked independent source review.

## Fresh checks on actual b91 source

ROOT executed the four remaining checks in the clean, exact landed worktree's hosted module. Disposable PostgreSQL16.14 and dedicated NOSUPERUSER/NOBYPASSRLS test roles qualified the app/CLI integration cases. Each command claimed its own shared build lease, checked fresh one-minute load at or below10, and released its exact owned lease immediately afterward. Caches, temporary files, logs and configuration were on the external SSD; GOWORK=off, GOMAXPROCS=2. Earlier load holds were not passes.

| Command | 2026-10-06 UTC | Load before claim | Result |
| --- | --- | --- | --- |
| `go test -count=1 -tags=integration -p=2 -timeout=5m ./internal/app ./cmd/registry` | 13:51:14–13:51:20 | 8.62 | PASS, exit0 |
| `go test -count=1 -p=2 -timeout=5m ./...` | 13:51:21–13:51:29 | 8.33 | PASS, exit0 |
| `go vet -p=2 ./...` | 13:51:31–13:51:34 | 7.98 | PASS, exit0 |
| `golangci-lint run --concurrency=2 --timeout=5m --build-tags=integration --config <faithful-migrated-v2-config> ./acceptance/...` | 13:51:35–13:51:44 | 9.27 | PASS, exit0 |

App and CLI real-store integration packages passed in2.325s and0.596s; full hosted unit packages passed; vet emitted no findings; integration acceptance lint reported0 issues. Lint used the previously qualified faithful v1-to-v2 migration, SHA256 `5962b020c904c61cb50ab80d27f117c79159de5139276b04462d08bec7b8e898`, rather than omitting repository lint settings. The four phase logs each record `RELEASED: R-build-lease`; no lease remains held by this runner.

WIRE separately ran fresh actual-b91 PostgreSQL integration checks for acceptance/wiring (2.43s, load2.43), acceptance/retrieval (local real-store smoke, load2.13), and acceptance/clients (30.762s, load6.40). These exercise real local HTTP/MCP application startup and resolution; the programmatic Go MCP client's synthetic native-client labels do **not** qualify Codex, Claude, client build pins, live providers, or consumer runtime acceptance. Generated fixture receipts are not admitted as native evidence.

## Carried exact-source evidence and boundaries

Full reviewed-to-landed tree parity preserves the linked premerge author, ROOT and independent receipts for affected unit/race/vet/lint, real-store tenant/replica behavior, root compatibility and frozen contracts. Those checks are carried evidence on unchanged bytes, distinct from the fresh commands above. No redundant full race run or hosted CI success is claimed. GitHub jobs remained unavailable because billing annotations say they did not start.

Catalog and outbox transaction rollback is qualified at component scope. Failed publication may still leave a private unbound staged blob; canonical publication must qualify tenant-safe staging retention/reconciliation. External enrollment, signing-key custody, canonical publication contract approval, live Treg/Composio limits/targets, actual consumer builds, AWS account/DNS binding, release/deployment and final PROD.9 acceptance remain open. This closes CORE.6 source verification only.

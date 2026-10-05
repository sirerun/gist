# WIRE final verification receipt — 2026-10-05

Source revision verified: `5a5895b1dab5a5ee02547ae09ae629406736fb1b` (PR #50, base `235b1f3f86c66d6f943a910369688be46291e1d6`). The receipt is documentation-only and follows that source revision.

Verification ran in the hosted Go module (`hosted/`) with `GOWORK=off`, `GOMAXPROCS=2`, and Go cache, module cache, and temp directories on the external SSD. Load was checked before each Go/lint command and was at or below 10 for every command below. The successful two-package unit run used the shared build lease; the lease was released immediately after completion. Race, vet, and lint commands each targeted one package. No provider or cloud actions were performed.

## Results

- `go test -p=2 ./internal/app` — PASS (`1.198s`).
- `go test -p=2 ./internal/resolution` — PASS (cached).
- `go test -race -p=2 ./internal/app` — PASS (`1.604s`).
- `go test -race -p=2 ./internal/resolution` — PASS (cached).
- `go vet ./internal/app` — PASS.
- `go vet ./internal/resolution` — PASS.
- `golangci-lint run --config /Volumes/BuildOffload/worktrees/gist-recovery-20261005-fixtures/lint-config/.golangci.yml ./internal/app` — PASS, 0 issues.
- The same configured lint command for `./internal/resolution` — PASS, 0 issues.
- `git diff --check` — PASS.

## Red evidence and correction

An initial lint run on the final integration source reported two QF1001 suggestions in `canonical_adapter.go` and one unchecked `resp.Body.Close` in coordinator-owned `app.go`. The adapter conditions were simplified in source commit `5a5895b`; root fixed the read-only stream close in commit `35dc45c`. Rerunning the configured linter on both packages returned 0 issues.

The first unit-test invocation was launched from the repository root, which is not the Go module root; Go reported that the module did not contain the requested hosted package paths. No package code ran in that invocation. The command was rerun from `hosted/` under the lease, and both package tests passed.

## Coverage boundary

These are local package checks, not hosted CI or provider/runtime acceptance. App composition now uses `NewCanonicalResolver` and the coordinator-owned storage wrapper maps missing and revoked catalog records to the resolver sentinels. The component tests cover the exact root skill pins, case-insensitive duplicate metadata, uppercase-only frozen request fields, fail-closed catalog cases, response budgeting, and real app composition.

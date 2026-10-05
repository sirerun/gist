# EVENT source rebase and verification

**Final base:** `93c5c4522af2a178618eba094c3af68fd42956eb`
**Final source commit:** `8a990008b8e0e86645566343ad4a56b7ce4977d6`
**Prior source snapshot:** `1857e913fec56f3675303a7e5ae959c6b1f5eda7`

The candidate was rebuilt directly from the landed base and contains the EVENT-owned storage/migration files plus authorized object-store cleanup fixes. The frozen shared ports (`events.go`, `policy.go`, `events_wire.go`, `pinned_resolution.go`) and shared REST event files (`events.go`, `errors.go`) match the landed base byte-for-byte.

Storage supplies durable tenant-bound event cursors and pages, database-clock expiry, seven-day retention-floor semantics, budget-qualified page advancement and cursor creation, tenant-scoped bounded `PurgeEvents`, and transactional event/outbox append for revocation. No application janitor or global purger is included; app scheduling and current-maintainer authorization remain with CORE.

The candidate was rebased from `f162015` onto `93c5c452` after WIRE landed. All seven EVENT-owned storage/migration files compare byte-for-byte with their verified pre-rebase versions at `0315b7f`; the frozen shared ports and REST files compare byte-for-byte with the new base. The authorized R5 object-store cleanup correction exposes cleanup errors after both successful commit and primary rename failure, and the named return value joins cleanup failure with the operation error.

Observed Go checks on code commit `08ab616dffb15bc32a038af1e0b04d48855d66b3`; all seven owned storage/migration files at the rebased code commit `8a990008b8e0e86645566343ad4a56b7ce4977d6` compare byte-for-byte with that tested source:

- `GIST_DATABASE_URL=<isolated fixture> GOWORK=off GOMAXPROCS=2 go test -tags=integration -p 2 -count=1 ./internal/storage` — **PASS**, `hosted/internal/storage` (`5.346s`) against the isolated PostgreSQL fixture. The integration suite ran without skips, including restricted-role/RLS, retention, cursor budget, DB-clock, and transaction rollback cases; the ordinary package tests include the new R5 cleanup regression.
- `GOWORK=off GOMAXPROCS=2 go test -race -tags=integration -p 2 -count=1 ./internal/storage` — **PASS** (`5.658s`).
- `GOWORK=off GOMAXPROCS=2 go vet -tags=integration -p 2 ./internal/storage` — **PASS**.
- Faithful `golangci-lint` config at the shared fixture path, `--concurrency=2 ./internal/storage` — **PASS**, 0 issues.
- `python3 scripts/registry/check.py contracts --freeze-check` — **PASS**.
- Shared API byte identity against the landed base — **PASS**.
- `git diff --check 93c5c4522af2a178618eba094c3af68fd42956eb..HEAD` — **PASS**.

Independent final review, PR update, merge, and landed verification remain open. These source checks do not claim app janitor wiring or runtime/production acceptance.

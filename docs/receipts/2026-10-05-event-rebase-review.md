# EVENT source rebase and verification

**Final base:** `f162015238a94fbc930865f70bef014865f553d4`
**Final source commit:** `f507359ba890721a6eb1ef34941c93a203e95786`
**Prior source snapshot:** `1857e913fec56f3675303a7e5ae959c6b1f5eda7`

The candidate was rebuilt directly from the landed base and contains only the EVENT-owned storage/migration files and the two authorized object-store cleanup fixes. Byte comparisons against the prior snapshot pass for `events.go`, `events_integration_test.go`, `revocations.go`, `objects_fs.go`, `objects_s3.go`, and `008_event_retention_floor.sql`. The frozen shared ports (`events.go`, `policy.go`, `events_wire.go`, `pinned_resolution.go`) and shared REST event files (`events.go`, `errors.go`) match the landed base byte-for-byte.

Storage supplies durable tenant-bound event cursors and pages, database-clock expiry, seven-day retention-floor semantics, budget-qualified page advancement and cursor creation, tenant-scoped bounded `PurgeEvents`, and transactional event/outbox append for revocation. No application janitor or global purger is included; app scheduling and current-maintainer authorization remain with CORE.

The candidate was rebased from `8d11f53` to `f162015` after the docs-only checkpoint merge. All six EVENT-owned code/migration files compare byte-for-byte between the pre-rebase test commit `98db495c4e9e319c1871b6bd5c64d1448fec2581` and final source commit `f507359`; the frozen shared ports and REST files also compare byte-for-byte with the final base. No production source changed during the rebase.

Observed checks on the unchanged EVENT source tree (the exact code bytes at final source commit `f507359`):

- `GOWORK=off GOMAXPROCS=2 go test -tags=integration -p 2 -count=1 ./internal/storage` — **PASS**, `hosted/internal/storage` (`4.458s`) against the isolated PostgreSQL fixture. The integration suite ran without skips, including restricted-role/RLS, retention, cursor budget, DB-clock, and transaction rollback cases.
- `python3 scripts/registry/check.py contracts --freeze-check` — **PASS**.
- `git diff --check f162015238a94fbc930865f70bef014865f553d4..HEAD` — **PASS**.
- Owned storage/migration byte parity against the prior source snapshot and shared API byte identity against the landed base — **PASS**.

Storage race, vet, and lint checks remain pending the next verification slot. Independent final review, PR update, merge, and landed verification remain open. These source checks do not claim app janitor wiring or runtime/production acceptance.

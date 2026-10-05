# WIRE landed verification — 2026-10-05

Verified landed source: `93c5c4522af2a178618eba094c3af68fd42956eb` (`origin/main`). This source is byte-for-byte tree-equivalent to independently reviewed WIRE head `9ac3263b3c792413a13a02c7b6c66f1949148cac`; both have tree `f749080ad5d308c3b50611fbec23e629f1c63c3c`. The verification branch contains no production-source changes.

The frozen contract registry check passed:

```text
python3 scripts/registry/check.py contracts --freeze-check
registry check: PASS: contracts
```

Using the landed source, with Go commands gated on fresh one-minute load at or below 10 and SSD-backed caches/temp directories, these checks passed from `hosted/`:

- `go test -count=1 -p=2 ./internal/app`
- `go test -count=1 -p=2 ./internal/rest`
- `go test -count=1 -p=2 ./internal/remotemcp`
- `GIST_DATABASE_URL=<fixture URL> go test -count=1 -tags=integration -p=2 ./internal/app`
- `GIST_DATABASE_URL=<fixture URL> go test -count=2 -tags=integration -run '^TestPostgresPinnedResolutionRoundTripAndPrincipalBinding$' -p=2 ./internal/app`
- `go test -race -count=1 -p=2 ./internal/app`
- `go test -race -count=1 -p=2 ./internal/rest`
- `go test -race -count=1 -p=2 ./internal/remotemcp`
- `go vet -p=2 ./internal/app`
- `go vet -p=2 ./internal/rest`
- `go vet -p=2 ./internal/remotemcp`
- `golangci-lint run --config <fixture v2 config> ./internal/app`
- `golangci-lint run --config <fixture v2 config> ./internal/rest`
- `golangci-lint run --config <fixture v2 config> ./internal/remotemcp` (0 issues)

The tagged PostgreSQL suite and two-run pinned-resolution/principal-binding case passed against the local restricted-role fixture. `git diff --check` passed. These are local landed-source checks; no hosted CI or broader deployment/acceptance claim is made.

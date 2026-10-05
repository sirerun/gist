# EVENT landed storage verification

**PR:** [#51](https://github.com/sirerun/gist/pull/51)  
**Landed SHA:** `57927cda892ffdd6d7bd048041c77a3bc7a8e8ab`  
**Prior verification candidate:** `8a990009c8907c0d4a5f5028b77d91c9610cde82`

At the exact landed SHA, all seven EVENT-owned storage and migration files compare byte-for-byte with the tested candidate. The storage package test sources contain no `t.Skip` calls.

Observed checks at the landed SHA, with Go caches and temporary files on the external SSD, `GOWORK=off`, and `GOMAXPROCS=2`:

- `GIST_DATABASE_URL=<isolated fixture> go test -tags=integration -p 2 -count=1 ./internal/storage` — **PASS** (`4.503s`), against the isolated PostgreSQL fixture.
- `GIST_DATABASE_URL=<isolated fixture> go test -race -tags=integration -p 2 -count=1 ./internal/storage` — **PASS** (`5.751s`), against the isolated PostgreSQL fixture.
- `go vet -tags=integration -p 2 ./internal/storage` — **PASS**.
- Faithful `golangci-lint` configuration with `--concurrency=2 ./internal/storage` — **PASS**, 0 issues.
- `python3 scripts/registry/check.py contracts --freeze-check` — **PASS**.
- `git diff --check 93c5c4522af2a178618eba094c3af68fd42956eb..57927cda892ffdd6d7bd048041c77a3bc7a8e8ab` — **PASS**.

One initial test invocation exited before compilation because the task-local `TMPDIR` directory had not yet been created. After creating the SSD-local temporary and cache directories, both integration and race commands completed successfully; the initial invocation executed no tests.

This verifies EVENT storage at its actual landed source. It does not qualify CORE app janitor wiring or production/runtime acceptance.

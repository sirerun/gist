# Shared-interface final independent review

**Reviewer:** independent worker (non-author and non-coauthor)  
**Reviewed source head:** `46b9a5cab5fb30709b3acf4777ad967f6a2ba411`  
**Base:** `190442c8098ccdd0544cc4e83247b466551c0392`  
**Disposition:** PASS; no blocking findings.

The final delta retains the frozen `hosted/internal/ports/events.go` and `hosted/internal/ports/policy.go` bytes. `events_wire.go` implements the frozen v1 JSON shape additively, and `pinned_resolution.go` supplies richer pinned records without changing the legacy resolution API. The REST event route fails closed for stores that cannot budget reads, requires an atomic budget-qualified first page, and uses the authenticated principal on resumed pages. Error mapping does not expose adapter detail.

The event wire object has exactly five fields: `event_id`, `event_type`, `subject_ref` (`id@version`), `occurred_at` (UTC RFC3339 text), and `policy_generation`; it omits internal workspace bindings. `Event.MarshalJSON` intentionally defines output serialization only. No inverse `UnmarshalJSON` mapping is provided, so HTTP consumers should decode the frozen wire envelope (or its raw JSON fields), not unmarshal that envelope directly into the domain `ports.Event` struct.

Earlier regression evidence on the predecessor/fix revisions reproduced and then cleared the accepted findings: R1 unsafe legacy reader (RED at `4ff8237`, targeted PASS at `62dab8b`); R2 separate cursor open before budget qualification (RED at `0b454e2`, targeted PASS at `62dab8b`); R3 default Go JSON did not match the frozen wire schema (RED at `1c4e11f`, targeted PASS at `62dab8b`). Final source changes since those passes preserve the corrections and move serializer/pinned types to additive files; all scoped packages were rerun below on the exact final source head.

## Final-source verification

Every Go command was preceded by a fresh `uptime` check; the observed one-minute load remained below 10. Go cache and temporary directories were on the task SSD, with `GOWORK=off`, `GOMAXPROCS=2`, and `-p=2`.

- `go test -p=2 ./internal/ports` — PASS.
- `go test -p=2 ./internal/rest` — PASS.
- `go test -race -p=2 ./internal/ports` — PASS.
- `go test -race -p=2 ./internal/rest` — PASS.
- `go vet -p=2 ./internal/ports` — PASS.
- `go vet -p=2 ./internal/rest` — PASS.
- `golangci-lint run --config <qualified migrated v2 config> --concurrency 2 ./internal/ports` — PASS, 0 issues.
- `golangci-lint run --config <qualified migrated v2 config> --concurrency 2 ./internal/rest` — PASS, 0 issues.
- `python3 scripts/registry/check.py contracts --freeze-check` — PASS.
- `git diff --check 190442c8098ccdd0544cc4e83247b466551c0392..46b9a5cab5fb30709b3acf4777ad967f6a2ba411` — PASS.
- `git diff --quiet <base>..<head> -- hosted/internal/ports/events.go hosted/internal/ports/policy.go` — PASS; both frozen source files are byte-identical to base.

The repository-wide Go suite, PostgreSQL runtime composition, hosted CI, merge, deployment, and acceptance were not part of this review. The scoped checks are local evidence only.

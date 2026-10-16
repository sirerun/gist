# Catalog library foundation source verification

Scope: T-CATALOG.9/.10, first source batch of Gist library to Zati delivery.
Requested batch ceiling 10; actual implementation/verification members 2.
Coordinator owns documentation and integration; one inherited-session implementer
owns shared code and consumer tests; a separate reviewer owns final approval.
The current Codex session selection is inherited without overrides. Per-stage
model identifier, tokens and dollar telemetry are unavailable; no acceleration
claim is inferred from batching.

## Source and observable behavior

Base: `63194e9543b8650beb495c427dd821f7d035a345`.
Go source tested: `75bfddb` (full candidate head recorded in the PR).
The public catalog domain serves both external consumers and hosted compatibility
wrappers. The maintained `zati/inbox-briefing@1.0.0` includes instructions and a
briefing template, with `mail/inbox/read@1.0.0` as its requirement.

The external-package consumer test exercises discovery, explicit incomplete
resolution without a provider, a test-only controlled binding, complete pinned
retrieval and every maintained asset. Foreign-workspace, missing/revoked records,
revoked binding and tampered bytes fail closed. Trusted digest pins do not prove
publisher signatures. No Gmail binding or mailbox access is asserted.

## Verification

Mac multi-package builds were held because observed one-minute load exceeded 10.
An existing DGX was qualified with Go 1.26.1 linux/arm64, load 1.14, over 100 GiB
available memory and 1.5 TiB free storage. Tests used four Go workers and a
600-second timeout. A tracked-source-only archive was transferred and its SHA256
matched at both ends: `82a0c32bd2a651b404fe2900e67d62fbc1417c3f6283b3ed6384658f5f10a433`.
No environment secrets, credentials or untracked files were transferred.

- Root: `GOWORK=off go test -p 4 ./...` passed, including the public consumer.
- Hosted: the first same-command run failed retrieval acceptance because no
  configured PostgreSQL fixture was available; all other packages passed.
- Hosted rerun: `GOWORK=off go test -p 4 ./...` passed with an owned isolated
  loopback PostgreSQL 16 fixture and explicit `GIST_DATABASE_URL`. The fixture
  server stopped cleanly afterward. This is source acceptance, not hosted CI,
  production or live-provider acceptance.
- Frozen contract check initially exposed source-location lock assumptions;
  the compatibility correction and its exact final results are recorded below.

Zati integration, release publication and installed/live inbox acceptance remain
separate downstream tasks. Semantic retrieval and dynamic-guide evaluation are
outside this source batch.

## Frozen-source compatibility correction

Correction `ebe97b6` adds an explicit source-relocation manifest. Original frozen
v1 hashes and the v2 catalog amendment remain unchanged. The checker validates
original bytes at their moved paths, preserves baseline exports through pure
hosted aliases, and permits only the declared additive root-module dependency.
`python3 scripts/registry/check.py contracts --freeze-check` passed and
`python3 -m unittest discover -s scripts/registry -p '*_test.py'` passed all 12 tests.
This correction changes Python compatibility validation and metadata only; tested
Go source is unchanged. Final independent review and landing are recorded in the
PR, not asserted by this source receipt.

## Review correction

Independent review of `d1361e9` found CI path filters omitted the newly shared
catalog code. Accepted as CAT-R1: both Registry CI triggers now include catalog,
shared lexical code and root dependency files. The hosted job also provisions
its required isolated PostgreSQL service explicitly, matching the passing
source recipe instead of relying on an unavailable default database. These are
workflow changes, not evidence that hosted Actions ran. Final exact-head review
is recorded in the PR.

## Landed source foundation

PR66 rebase-merged to `9e870c0a7b93990b166071ec061a27206afbda88` after independent
final approval at `1d5f8cbed597d6eaaf3cb81e596fc638901f3b51`. Fresh remote main
contains the landed revision and its complete tree equals the reviewed candidate.
GitHub Actions did not start due to the account billing lock; branch policy had
no required status checks and no protection was bypassed. Source checks above
are the authorized fallback, not hosted Actions passes.

The actual landed tracked-source archive had matching SHA256 at both ends:
`c88c11708fb0001249fc59a94d29ea8a2bcd68c52b41e3a321a5065c1ad92914`.
On that fresh source, frozen-source validation and
`GOWORK=off go test -p 4 ./catalog/... ./internal/cataloglexical` passed.
T-CATALOG.11/.12/.13 close this producer batch. The downstream Zati batch
T-CATALOG.1/.2 pins this immutable producer and retains its own verification,
independent review, merge and landed gates. No release tag was created.

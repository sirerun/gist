# PR60 corrected-lifetime review

Historical REQUEST_CHANGES at0006a12. Accepted LEAD-R1 must be fixed, verified and independently re-reviewed through PUBLISH.18/.19/.20 before merge.

## REQUEST_CHANGES

**Candidate:** `0006a12331c5e36e2afcd2f77e400eefa948a781`
**Base:** `8a24c72a596a757e13426945911d87446cc3a4e0`

**Scope reviewed:** the full base-to-head candidate, with attention to app composition, REST and storage callers, authority, evidence, compatibility, configuration, maintenance, and the corrected resource lifetime. No source was edited, and no builds or tests were run.

### Finding

**GIST-PUBLISH60-LEAD-R1 — P2 — Filesystem root can close while an HTTP handler is still using it**

[app.go:308](../../hosted/internal/app/app.go#L308) falls back to `Server.Close()` when graceful shutdown hits its deadline, then proceeds to close the object store at [app.go:318](../../hosted/internal/app/app.go#L318). `Server.Close()` closes active connections but does not wait for their handlers to finish. A handler already in a publication read can still reach `OpenOwned`; the filesystem backend uses the pinned root at [objects_fs.go:276](../../hosted/internal/storage/objects_fs.go#L276). If the shutdown deadline expires during that operation, closing the root can make the in-flight read fail with a closed-root error. The existing lifetime test covers reads after a *completed* shutdown, not an active handler during forced shutdown.

Please add a targeted concurrent-read/shutdown-deadline repro and ensure the owned root remains usable until active handlers have finished using it. The coordinator owns the fix and verification.

### Evidence and limits

The exact pinned base and head both resolve as requested. The retained `publication-final-exact-plan-check.log` reports Wazi 275 with reader compatibility, schema and semantic validity; its plan source digest matches the current `docs/plan.md`. The original reviews are retained and correctly marked historical; they do not approve the lifetime correction. I confirmed the v2 schema handoff files and the v1/v2 contract directories are unchanged from base, and the named additive port fields are recorded in `implementation-v2.json`.

The source-verification receipt reports R5 hosted checks passing at `56b3236`; the receipt says only documentation changed afterward. These are retained local results, not hosted CI or production qualification. No actual provider, live S3, or production-verifier qualification is claimed. The five baseline hosted lint findings remain documented, and PUBLISH.5/.6 plus production gates remain open.

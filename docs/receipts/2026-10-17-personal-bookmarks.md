# Personal directory bookmarks

Signed-in workspace members can bookmark public directory entries from cards or details, remove them, and filter the library with **My bookmarks**. Search, kind and use-case filters apply to that saved collection. Bookmarks persist in PostgreSQL, keyed by workspace, issuer, subject and entry ID; the current website uses its configured workspace. Ordinary readers may save their own preferences. Global recommendation, official status and featured curation are unchanged.

The collection is bounded at 200 entries per user. Missing catalog entries remain removable from the saved collection. Writes require same-origin requests and the existing session-bound CSRF token. SQL reads and mutations include subject/issuer and current membership predicates, with forced tenant RLS. No credentials or descriptors are copied into bookmarks.

## Local evidence

- JavaScript syntax and `git diff --check` pass.
- `go test -p 1 ./internal/app ./internal/webui` passes.
- PostgreSQL integration: `go test -p 1 -tags integration ./internal/app -run 'TestPersonalBookmarks|TestPrivateDirectory|TestWebCatalogUsesRestricted' -count=1` passes.
- Migration and runtime-role integration: `go test -p 1 -tags integration ./internal/migrate -count=1` passes.
- Bookmark integration covers idempotent saves, ordinary reader writes, cross-user list/remove isolation, durable reads through a fresh store, tenant RLS, CSRF, revoked membership, unknown entries, query validation, saved search, 50-entry pagination and the per-user limit.
- Chrome local fixture: saved GitHub from the library, reloaded and observed the saved state, filtered My bookmarks to one item, removed it to an empty state, saved from details, observed another user's empty collection, and observed the signed-out sign-in prompt.

The browser fixture used the real application handlers and an isolated PostgreSQL fixture, with test-issued sessions supplied by a loopback-only harness. This does not qualify live GitHub login, production migration, or deployed behavior. The temporary harness was stopped and removed. Tests used canonical external temporary storage; the shared build lease was released.

## Release requirements

Apply forward migration `012_personal_bookmarks.sql` and provision/verify the reviewed runtime grants (`SELECT, INSERT, DELETE` on `directory_bookmarks`) before deploying the updated application. The server does not apply migrations on startup. Production remains unchanged by this source delivery.

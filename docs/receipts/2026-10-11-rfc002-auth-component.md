# RFC-002 operator enrollment component candidate

This candidate implements the isolated enrollment and Postgres storage component for the approved invite-only GitHub App sign-in design. It is based on the current main revision c02811e00c76794f854173463a03a22594b1705a. Application startup/configuration wiring remains part of the later composition task; this component is not yet reachable from the running registry.

The sign-in start uses a fixed HTTPS callback, OAuth state, and S256 PKCE. State is stored as a hash with a short expiry and consumed once. The verifier and optional invitation token are encrypted in a Secure, HttpOnly, host-only cookie. The callback exchanges the code, calls only GitHub's /user identity endpoint, discards the returned access token, and records the stable numeric account ID. It does not request repository permissions or store a GitHub token.

The configured founder can create the exact initial workspace and maintainer membership once, in one transaction, only when the workspace does not already exist. Subsequent access requires an active membership or an unexpired single-use invitation bound to the same numeric GitHub account. Maintainers create invitations through a same-origin, CSRF-protected POST; role and scopes are checked against the caller's current authority. Invitation bearer values and OAuth state are stored only as hashes. Migration 010_operator_enrollment.sql adds the state, invitation, and one-time bootstrap records with workspace RLS on invitations.

Synthetic verification on the candidate:

- GOWORK=off go test ./internal/enrollment ./internal/storage passed.
- GOWORK=off go test -race ./internal/enrollment ./internal/storage passed.
- GOWORK=off go vet ./... passed.
- All hosted Go packages except acceptance/retrieval passed go test; the retrieval acceptance cases require a configured Postgres URL and failed to connect in this environment.
- The Postgres enrollment integration test compiled with the integration build tag. The CI test job now exports its Postgres service URL under GIST_DATABASE_URL, matching the integration helpers. It was not executed locally because no test database credential is configured. An isolated connection attempt to the local Postgres listener failed authentication before creating a database.
- gofmt is clean. The qualified DGX ran golangci-lint v2.14.0 on the root Go module and the changed hosted packages. The full hosted-module lint currently reports five findings in unchanged discovery/cache.go, oauth/delegated.go, packages/retrieve.go, and contract/digest.go; those findings remain outside this enrollment candidate. The repository lint action and config were upgraded to pinned golangci-lint v2.14.0 because the last v1 release cannot analyze the Go 1.26 target. goimports is unavailable.

This is implementation-candidate evidence only. It does not prove deployed GitHub App configuration, application wiring, independent review, merge, production login, or production acceptance.

## Follow-up qualification at candidate head 5cc32d24313ca065fadb2d1b45c819afb7452a49

A task-owned PostgreSQL 16 cluster was created for verification with local Unix-socket access only, no configured password, and no connection to the existing shared PostgreSQL listener. The cluster is now stopped and its task-local data remains in the task cache. This supersedes the earlier statements above that the retrieval and enrollment database checks could not run locally.

- `GOWORK=off go test ./... -count=1` in `hosted/` passed against the isolated database, including all 24 local retrieval smoke cases.
- `GOWORK=off go test -tags=integration ./internal/storage -count=1` passed against fresh per-test databases and migrations, including `TestEnrollmentBootstrapAndInviteAreAtomicAndBound`.
- `GOWORK=off go test -tags=integration ./... -count=1` passed for storage, app wiring, retrieval, and the other listed packages, but the overall command failed in `acceptance/clients`: `TestWorkload` requires the absent named client build variable `GIST_CODEX_CLI_BUILD`. This is a real-runtime qualification input; it was not skipped or fabricated.

The tests above are local component and synthetic wiring evidence. They do not qualify a named external runtime, compose the new enrollment handler into application startup, satisfy independent review, prove merge/landed behavior, or establish live production acceptance. The PostgreSQL cluster and URL were temporary and are not production credentials.

## Route and invitation-expiry coverage at source head dd391a511d972fa52fc3ab5623664bc99831830a

Added TestRoutesServeLoginOverHTTP, which registers the public routes on an HTTP mux, runs a local HTTP server, and asserts the start redirect, protected flow cookie, callback status, session issuance, and JSON response. Added TestEnrollmentRejectsExpiredInvitation against the migrated PostgreSQL storage implementation.

- In hosted/, GOWORK=off go test ./... -count=1 passed against the isolated task-owned PostgreSQL 16 Unix-socket cluster.
- In hosted/, GOWORK=off go test -tags=integration ./internal/storage -count=1 passed against fresh per-test databases and migrations, including expired invitation denial.
- In hosted/, GOWORK=off go vet ./internal/enrollment ./internal/storage passed.
- Pinned golangci-lint v2.14.0 on ./internal/enrollment and ./internal/storage reported zero issues.
- The full integration-tagged module suite remains incomplete because acceptance/clients requires a named Codex build through GIST_CODEX_CLI_BUILD. The target is unavailable; no value was invented.
- GitHub Actions annotations on the follow-up PR run report that jobs were not started because the account is locked due to a billing issue. No repository self-hosted runners are registered. Local verification is the documented fallback; this does not replace independent review or runtime qualification.

This follow-up remains component evidence. Application startup wiring, independent exact-head security review, merge, landed checks, named consumer qualification, and live production acceptance are still open.

## Invitation authorization-scope regression at source head e07d1751990e817bbe1d540fb48427d3238e2e32

Added TestInvitationCannotWidenMaintainerScopes. It creates a fresh workspace whose maintainer has catalog:read, attempts a catalog:publish invitation, asserts ErrDenied, and verifies no invitation row was persisted.

- GOWORK=off go test -tags=integration ./internal/storage -run TestEnrollment -count=1 passed against the isolated PostgreSQL 16 cluster.
- GOWORK=off go vet ./internal/enrollment ./internal/storage passed.
- Pinned golangci-lint v2.14.0 on ./internal/enrollment and ./internal/storage reported zero issues.
- The cluster was stopped after the run. These checks remain local component evidence; application composition, independent review, merge, landed verification, and production acceptance remain open.

## Scope-denial test filter correction

The earlier TestEnrollment filter did not match TestInvitationCannotWidenMaintainerScopes. That regression was run separately at source head e07d1751990e817bbe1d540fb48427d3238e2e32 using GOWORK=off go test -tags=integration ./internal/storage -run TestInvitationCannotWidenMaintainerScopes -count=1. It passed against a fresh database on the isolated PostgreSQL 16 cluster, and the cluster was stopped afterward.

## Client fixture database isolation and current-schema acceptance at source head d4d3a242f72c2c635d6f4e9d69cefbd5c5ebe51c

The local client acceptance fixture had mutated a parsed pgx database field and then called `ConnConfig.ConnString()`, which retained the original database URL. That caused the fixture to connect to the admin database instead of the database it created. It now rewrites the PostgreSQL URL path, verifies `current_database()` before applying migrations, uses a cryptographically random test database name suffix, and cleans up a partially constructed fixture. The fixture applies current migrations 001 through 010.

- `GOWORK=off go test -tags=integration ./... -count=1` passed on the exact code head above with a task-owned PostgreSQL 16 test cluster reachable only over its private Unix socket.
- The client test used `x@local-fixture` labels for Codex CLI, Claude Code, and the policy-gated runtime. These select offline HTTP/MCP fixtures; they do not identify or qualify real client binaries or replace live consumer acceptance.
- `GOWORK=off go vet -tags=integration ./acceptance/clients` and `git diff --check` passed.
- The local PostgreSQL cluster was stopped after verification. No production credentials or external provider calls were used.

This closes the local integration-suite gap only. Named consumer builds and live workflow acceptance, application startup/configuration wiring for operator enrollment, independent exact-head review, merge, landed verification, and production acceptance remain open.

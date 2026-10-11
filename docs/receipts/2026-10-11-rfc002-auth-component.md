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

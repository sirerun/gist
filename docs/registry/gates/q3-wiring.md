# Q3 composition wiring record

Status: live wiring acceptance passes in this sandbox; changes are left
uncommitted as required by the lane handoff.

The integration fixture in `hosted/acceptance/wiring/fixture_test.go` creates a
random dedicated PostgreSQL database, applies the four migrations in filename
order, seeds two tenants and catalog records, boots `app.New` behind a
generated loopback HTTPS certificate, and uses a temporary filesystem object
store. It sets all five `REGISTRY_*` variables required by the wiring contract.
The app's own EdDSA issuer mints workload tokens after the SQL memberships are
installed. A TLS loopback broker stub covers connection initiation and polling
without holding provider secrets; the configured-broker and absent-broker
paths remain explicit service outcomes.

## Q3 coverage

* All 20 route-matrix endpoints have status/body assertions against the live
  listener. The suite also checks unauthenticated REST/MCP challenges,
  unsupported MCP protocol versions, request budgets, policy-before-rank,
  private package ETag/304 behavior, and cross-tenant denial before cache
  validation.
* The two-tenant oracle sends equal discover work concurrently, requires
  different tenant results, denies cross-tenant artifact access, and records a
  250 ms diagnostic timing tolerance without sleeps.
* M2A checks live liveness/readiness, workload authentication, publish-only
  read denial, discovery, resolution's current explicit service-unavailable
  response, and broker initiation/polling. The MCP transport budget is tested
  at the request-size boundary.
* A fixture package is installed through the app's own object/catalog store
  instances, so package bytes, digest, ETag, and tenant isolation are real
  filesystem/PostgreSQL behavior rather than mocks.

## Migration amendment proposal

The checked-in migrations contain two fixture-blocking schema-policy defects:
`002_policy.sql` includes `workspaces` in a `workspace_id` policy loop even
though its key is `id`; `003_identity.sql`'s policy block is not compatible
with the identity-table shape in this checkout. The fixture does not edit
those files. It applies the safe schema equivalent for the affected policy
blocks and leaves the affected identity tables fail-closed. The owning
migration lane should correct the SQL and remove these fixture adaptations.

## Verification

With `GOCACHE=$PWD/.gocache`, `GOTMPDIR=$PWD/.gotmp`, and
`GOMODCACHE=$PWD/.gomodcache`:

* `cd hosted && GOWORK=off go test ./internal/app ./acceptance/wiring -tags=integration -count=1`
  passed: `internal/app` and `acceptance/wiring` (3 integration tests; all
  route subtests passed).
* `cd hosted && GOWORK=off go test ./... -count=1` passed: all discovered
  packages, including the registry command (no test files).
* The shared build-lease claim was attempted and failed because the lease
  helper could not materialize its empty tree (`Operation not permitted`);
  tests proceeded in this isolated worktree and passed.

No dependency was added. The implementation continues to use pgx/v5 5.11.0
and jwx/v2 2.1.7, matching ADR 006.

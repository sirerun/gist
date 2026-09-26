# Registry wiring test environment

Wiring tests exercise the composed service and real backing services. They do
not silently downgrade to mocks, skipped tests, or a local-only assertion.

## Local and CI contract

Use Podman-managed PostgreSQL and an S3-compatible object store (or the CI
service equivalents). Docker Compose is not a prerequisite. The test runner
must provide:

* `REGISTRY_BASE_URL` — the service origin under test;
* `REGISTRY_AUDIENCE` — the exact configured token audience;
* `REGISTRY_TEST_ACCOUNT` and `REGISTRY_TEST_ACCOUNT_SECRET` — synthetic,
  sandbox-only account credentials injected by the environment or CI secret
  store; and
* `REGISTRY_ARTIFACT_DIR` — a temporary, ignored directory for gate evidence.

`REGISTRY_LIVE=1` is an explicit opt-in for deployed acceptance. Without it,
tests may run against an explicitly configured local service but must not claim
production or preview acceptance. Missing required variables or backing
services are failures, never skips. Test output must redact tokens, account
secrets, private origins, and local machine paths.

The harness records the command, test count, source/contract/config hashes,
service revision, and redacted evidence under the milestone gate directory.
Evidence is valid only when the checker reports a passing status and matching
hashes. PostgreSQL/object-store teardown is required in both success and
failure paths; use the test runner's cleanup hooks rather than manual shared
service mutation.

The Q harness itself is dependency-free Python and does not create
`hosted/go.mod`; lane C owns the hosted module and its dependency graph.

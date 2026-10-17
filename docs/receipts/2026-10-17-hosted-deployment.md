# Canonical hosted Gist deployment

This receipt records the owner-authorized AWS/Cloudflare deployment while
preserving authenticated catalog publication, private workspaces and OAuth.
Gist remains catalog and resolution infrastructure; clients own execution and
provider credentials. Public anonymous projection and runtime marketplace
integration are separate work.

## Landed source and qualification

Gist PRs [87](https://github.com/sirerun/gist/pull/87) and
[88](https://github.com/sirerun/gist/pull/88) landed startup enrollment repair,
explicit transactional migrations and read-only database inspection. Runtime
source is `746241de5aaf062ccb9cb3a7cd7e01d55d0d8cd9`; the deployed candidate image
is `sha256:1934bdccf82a4274e658413873b82fd47e8e92d1f6bf96f3ed267c660e00c3a5`.
Both image builds used exact merged commits through the existing bounded
CodeBuild project. No provider or paid model calls were used.

Hosted internal, command, wiring, retrieval and runtime integration passed with
the race detector, then vet/build. Changed migration/role/inspection suites
passed PostgreSQL race integration and vet/build, with fresh landed checks.
The contract freeze check and twelve registry Python checks passed. Non-author
review corrected owner/runtime credential sharing and a missing login-flow
pruning permission before merge. GitHub Actions could not start because of the
confirmed account billing lock; this is independently reviewed local/remote
check evidence under the existing fallback, not hosted CI success. Named
external-client acceptance remains unqualified.

Foundation PRs [280](https://github.com/sirerun/foundation/pull/280),
[281](https://github.com/sirerun/foundation/pull/281),
[282](https://github.com/sirerun/foundation/pull/282) and
[283](https://github.com/sirerun/foundation/pull/283) landed staged certificate,
secret containers, isolated migration task, exact inspection image and canonical
activation. Relevant Pulumi mocks passed with race detection, vet and
cross-compilation, with fresh landed checks. Actual staged previews were reviewed
before constrained applies. No database, bucket or account replacement occurred.

## Observed operations

- ACM issued the certificate for `gist.sire.run` after idempotent Cloudflare
  validation DNS creation. Canonical DNS targets the existing load balancer.
- Five protected Secrets Manager containers have durable versions. The company
  Sire Run GitHub App identity was verified using its supplied private key; its
  client secret is bound without publishing it. The private key remains local.
- The encrypted pre-migration snapshot became available before SQL mutation.
  Restore has not been exercised.
- Read-only schema status observed no application objects and no ledger.
  All ten embedded migrations then applied transactionally, with zero pending;
  no legacy adoption or identity seeding was performed.
- A separate application database identity was provisioned. A one-shot task
  using the exact inspection image authenticated as that role and attested
  restricted attributes, no membership/ownership, required reviewed grants,
  forced RLS and hidden unscoped/mismatched-workspace rows. Empty production
  row probes may be vacuous; seeded integration is separate evidence.
- Canonical activation changes only certificate validation/listener, service,
  application task and its execution policy. The application cannot retrieve
  the migration-owner database secret. The previous certificate and immutable
  image remain available as rollback inputs.

## External staging acceptance

ECS completed rollout with one desired/running task using the exact reviewed
image. Public TLS validates for the canonical hostname. `/healthz` returns 200.
GitHub sign-in start returns 302 to the actual authorization endpoint with the
expected client, canonical callback, state, S256 PKCE and secure HttpOnly cookie.
An invalid callback is denied. Until real founder enrollment, `/readyz`, catalog,
MCP and OAuth metadata return 503 as the fail-closed bootstrap gate requires.
This is an enrollment-capable deployment, not authenticated catalog acceptance.

The owner must open https://gist.sire.run/auth/github/start and sign in with the
configured GitHub account. Successful callback returns `authenticated: true`;
CSRF tokens and cookies must stay private. After normal maintenance readiness,
disable first-enrollment configuration through a reviewed change, redeploy and
verify strict startup plus the authenticated journey.

## Acceptance still being completed

Canonical issuer metadata after readiness, real founder GitHub enrollment,
authenticated catalog/MCP journey and removal
of first-enrollment mode must be observed before claiming full production
acceptance. Publication v2 remains disabled pending a real trusted production
binding verifier. No public catalog contents or Zati runtime marketplace uptake
are claimed by infrastructure deployment.

Initial founder access is limited to the configured stable numeric GitHub
identity and private workspace. No SQL fixture or asserted identity substitutes
for the real OAuth exchange. Catalog and workload requests remain denied while
normal persisted maintenance readiness is absent.

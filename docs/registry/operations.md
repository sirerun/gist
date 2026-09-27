# Registry operations

This runbook describes the reviewed deployment path for the hosted registry.
It is an operational design and test target; it does not claim a live or
production deployment.

## Deployment path

1. Review the stack configuration and set an exact HTTPS `GIST_PUBLIC_ORIGIN`
   and matching resource audience. Do not put an origin in an artifact ID.
2. Use the GitHub Actions release workflow with a protected environment. It
   authenticates with GitHub OIDC, builds the image with Podman, pushes it to
   Artifact Registry, resolves its digest, and configures Pulumi with that
   immutable reference.
3. Pulumi uses a private Google Cloud Storage backend (`pulumi login
   gs://...`). The state bucket is bootstrapped separately, has uniform bucket
   access, and is never the registry object bucket.
4. `pulumi preview --diff` is a required review artifact. `pulumi up` creates
   the Cloud Run service, private VPC path, regional PostgreSQL instance,
   private versioned object bucket, workload identity pool/provider, secret
   references, monitoring metrics, and migration job.
5. The workflow runs the migration job, then checks `/healthz` and `/readyz`.
   The release artifact contains the commit, image digest, stack export hash,
   origin, migration result, and live-check result.

Cloud Run is the only client-facing resource. PostgreSQL has no public IPv4;
the object bucket has public access prevention and only the runtime service
account has object-read access. Clients never receive storage URLs.

## Migration job

Schema changes are applied by the private Cloud Run Job created by Pulumi. A
release runs the job after the service image and infrastructure update but
before live checks. The job is single-attempt by default and uses the same
database secret reference as the service. Review migration SQL for forward and
rollback implications before changing the image. Do not run ad-hoc SQL against
the shared database as a release step.

## Secrets and identity

Secret Manager stores the database password, configured issuer reference, and
workload signing key. Pulumi creates secret containers and IAM references only;
secret values are supplied by the protected environment. The runtime service
account receives only Secret Manager accessor, Cloud SQL client, and object
viewer permissions. GitHub Actions uses short-lived OIDC credentials through
the configured workload identity provider; no service-account key is checked
in or generated.

Logs must be structured and include a generated `request_id`, route, status,
latency, and workspace-safe identifier. They must not include bearer tokens,
OAuth codes, package contents, secret values, or customer data. The declared
error and latency metrics are derived from these fields.

## Limits and SLOs

The baseline is recorded in `deploy/registry/config.example.yaml` and passed
to the service as configuration. Initial values are 10 MiB package bytes,
50 MiB expanded bytes, 1 MiB request bytes, 2 MiB response bytes, 100,000
catalog entries, 50 discovery results, 80 concurrent requests, and a 30-second
request timeout. The target SLO baseline is 99.5% availability, p95 latency at
or below 750 ms, and error rate below 1%. A 14-day database backup retention
period and a 24-hour preview lifetime are also explicit configuration values.
Any expansion requires a reviewed configuration change and a new preview.

## Backup and restore

PostgreSQL uses regional high availability, point-in-time recovery, retained
transaction logs, and the configured backup retention. Before a release that
changes migrations, restore an isolated database from a recent backup or point
in time, run the migration job against it, and record the restore and schema
check hashes in release evidence. Object bytes are immutable, versioned, and
private; restore drills must preserve object digests and IAM posture. Never
restore into the live database as a test.

## Rollback

Rollback is an IaC operation: select the prior reviewed stack configuration,
restore the prior immutable image digest, and run the same preview and deploy
workflow. If a migration is not backward compatible, stop traffic through the
reviewed Cloud Run revision procedure and restore the isolated database plan;
do not silently widen token audiences or issuers. Origin, issuer, audience,
redirect configuration, database, and object namespace are rolled back as one
versioned configuration. Existing tokens are not accepted across a changed
audience or issuer.

## Short-lived preview

The `preview` stack is isolated from the long-lived target stack: separate
origin, audience, database, object namespace, issuer registrations, and
synthetic workspaces. Its maximum lifetime is 24 hours. Evidence is captured
before the workflow destroys the stack; a failed destroy fails the workflow
and requires cleanup before acceptance can proceed. Evidence is sanitized and
contains no credentials.

### What the preview IaC declares

`deploy/registry/__main__.py` with `gist-registry:deploymentMode=preview`
(validated by `deploy/registry/preview.py`) declares:

- The same Cloud Run service, private PostgreSQL, private bucket and
  migration job as the target stack, but destroyable: no deletion
  protection, no bucket retention lock, `force_destroy`, zonal database, no
  backups (synthetic data only). Resource names carry `gist-prv-<runId>`.
- A public origin `https://preview-<runId>.<preview base domain>`: a global
  external Application Load Balancer with a Google-managed TLS certificate
  for that host, an HTTP-to-HTTPS redirect, and one A record in an existing
  Cloud DNS zone. Cloud Run ingress is restricted to the load balancer.
- `GIST_PUBLIC_ORIGIN` and `GIST_RESOURCE_AUDIENCE` both equal the preview
  origin, which must differ from `productionOrigin`. The protected-resource
  metadata URL `<origin>/.well-known/oauth-protected-resource` is exported
  and probed; the application serves the document (I4).
- `GIST_OAUTH_REDIRECT_URIS`: exact HTTPS redirects; production-origin
  redirects are rejected.
- Secret access by name only to pre-created `gist-registry-preview-*`
  Secret Manager entries. The stack grants and, on destroy, removes the
  preview runtime's accessor binding. It never creates secret payloads and
  never references production secret names.
- No workload identity pool (the preview deploys through the existing
  GitHub OIDC provider with a separate preview deployer service account).

Program validation refuses preview mode on any stack other than `preview`,
refuses a `preview` stack without preview mode, and refuses an expiry more
than `limits.previewTtlHours` (hard maximum 24) away.

### Workflow stages

`.github/workflows/registry-preview.yml`:

1. `offline-checks`: unit tests under Pulumi mocks and
   `python3 scripts/registry/check.py iac --preview`.
2. `plan` (environment `registry-preview`): builds the image with Podman,
   configures the `preview` stack, runs `pulumi preview --diff`, and uploads
   the diff as the review artifact.
3. `create` (environment `registry-preview-apply`, required reviewers):
   records the owner/expiry manifest before any resource exists, then
   `pulumi up` and the migration job.
4. `test`: waits for managed TLS, then checks `/healthz`, the HTTP redirect,
   the application `401` bearer challenge on `/mcp` (not an infrastructure
   denial), and the protected-resource metadata `resource` value.
5. `destroy` (environment `registry-preview-destroy`): runs after success,
   failure or cancellation unless `keep_after_test` is set and tests passed.
   It retries `pulumi destroy`, then fails unless the stack holds zero
   resources, and uploads `preview-teardown.json`.
6. `ttl-sweep`: hourly schedule; destroys the preview once its recorded
   `previewExpiresAt` passes, or immediately when resources exist without an
   expiry.

### Prerequisites before the first run

Not provisioned by this stack (bootstrap once, through reviewed IaC or the
foundation repo):

- GitHub environments `registry-preview`, `registry-preview-apply` (with
  required reviewers) and `registry-preview-destroy`.
- Environment secrets (names only): `GCP_WORKLOAD_IDENTITY_PROVIDER`,
  `GCP_PREVIEW_DEPLOYER_SERVICE_ACCOUNT`, `GCP_PREVIEW_PROJECT`,
  `PULUMI_STATE_BUCKET`, `PULUMI_PREVIEW_CONFIG_PASSPHRASE`,
  `GIST_PUBLIC_ORIGIN` (production origin, for the inequality check),
  `GIST_PREVIEW_BASE_DOMAIN`, `GIST_PREVIEW_DNS_ZONE`,
  `GIST_PREVIEW_DNS_PROJECT`, `GIST_PREVIEW_OAUTH_REDIRECT_URIS`.
- A preview deployer service account bound to the GitHub OIDC provider,
  limited to the preview project and the preview DNS zone.
- Secret Manager entries `gist-registry-preview-database-password`,
  `gist-registry-preview-oauth-issuer` and
  `gist-registry-preview-workload-signing-key` holding preview-only values.
- A Cloud DNS managed zone for the preview base domain, delegated from its
  parent domain.

### Run and tear down

Run: Actions, **Registry OAuth preview**, **Run workflow**, `action=create`.
Approve the `registry-preview-apply` deployment after reading the uploaded
diff. Tear down now: run the same workflow with `action=destroy`. Abandoned
runs are destroyed by the hourly TTL sweeper; O4 records the terminal
teardown evidence.

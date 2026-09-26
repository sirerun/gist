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

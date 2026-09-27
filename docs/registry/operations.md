# Registry operations

This runbook describes how the hosted registry is operated. It does not claim
a live deployment; production evidence is recorded by the O3 release task.

## Where the deployment code lives

The production registry at `https://registry.sire.run` runs on AWS, per
[ADR 008](../adr/008-registry-aws-hosting.md). Its infrastructure as code, the
container build definition and the image build specification live in the
private `sirerun/foundation` repository, in the Pulumi Go stack
`registry-aws` (`pulumi/registry_aws*.go`). This public repository holds only
the application, its contracts and its checks. The earlier GCP Pulumi program
and its preview and release workflows were deleted on 2026-09-27; git history
keeps them.

## Deployment path

1. Merge the application change here with local validation (ADR 008 item 12).
2. Build the image at that exact commit (ADR 008 item 8). The image is pushed
   to an immutable repository and pinned by digest.
3. Move the digest pin in the `registry-aws` stack through a reviewed change,
   run `pulumi preview`, and apply only when the preview matches the merged
   change.
4. Run the migration step, then check `/healthz` and `/readyz`. The release
   evidence records the commit, image digest, stack configuration hash,
   origin, migration result, and live-check result.

The load balancer is the only client-facing resource. The database admits only
the registry tasks, and the object bucket is private; clients never receive
storage URLs.

## Migration job

Schema changes are applied by a one-shot migration task that uses the same
database secret reference as the service. A release runs it after the image
and infrastructure update and before live checks. Review migration SQL for
forward and rollback implications before changing the image. Do not run
ad-hoc SQL against the shared database as a release step.

## Secrets and identity

Secret values are supplied outside the stack; the stack creates secret
references and least-privilege access only. The runtime role can read its own
secrets, reach the database, and read and write its object prefix. No secret
value or long-lived credential is checked in to either repository.

Logs must be structured and include a generated `request_id`, route, status,
latency, and workspace-safe identifier. They must not include bearer tokens,
OAuth codes, package contents, secret values, or customer data. The declared
error and latency metrics are derived from these fields.

## Limits and SLOs

Limits are passed to the service as configuration from the stack. Initial
values are 10 MiB package bytes, 50 MiB expanded bytes, 1 MiB request bytes,
2 MiB response bytes, 100,000 catalog entries, 50 discovery results, 80
concurrent requests, and a 30-second request timeout. The target SLO baseline
from ADR 007 is 99.5% availability, p95 latency at or below 750 ms, and error
rate below 1%. ADR 008 records that the single-AZ database and single Spot
task run below the availability target until funding allows otherwise. Any
limit expansion requires a reviewed configuration change.

## Backup and restore

The database takes automated backups. Before a release that changes
migrations, restore an isolated database from a recent backup, run the
migration against it, and record the restore and schema check hashes in
release evidence. Object bytes are immutable, versioned, and private; restore
drills must preserve object digests and access posture. Never restore into the
live database as a test.

## Rollback

Rollback is an IaC operation: revert the change, restore the prior immutable
image digest, and apply again through the same preview-then-apply path. If a
migration is not backward compatible, stop traffic and restore into an
isolated database first; do not silently widen token audiences or issuers.
Origin, issuer, audience, redirect configuration, database, and object
namespace are rolled back as one versioned configuration. Existing tokens are
not accepted across a changed audience or issuer.

# ADR 008: Host the Production Registry on AWS

## Status

Accepted

## Date

2026-09-27

## Context

ADR 007 settled the registry's origin, issuer and IaC-only deployment, and the
program in `deploy/registry` targets GCP. The founder has no budget for GCP or
for GitHub Actions (the account is billing-locked), and the remaining RFC-002
deploy tasks (Q5, O3, E3) need a production registry. Sire's own staging
already runs on AWS from `sirerun/foundation` (stack `staging-aws`), so that
stack's patterns are proven and cheap.

## Decision

David made each choice below on 2026-09-27.

1. **Environment.** One long-lived production registry at
   `https://registry.sire.run`. This is the target for Q5, O3 and E3.
2. **Account.** A new AWS member account in the sirerun organization,
   root email `david+aws-registry@sire.run`, reached through
   `OrganizationAccountAccessRole` like `staging-aws`.
3. **IaC home.** A new Go Pulumi stack, `registry-aws`, in
   `sirerun/foundation/pulumi`, gated in `main.go` like `staging-aws` and
   `postiz`, in region `us-west-1`. State lives in Pulumi Cloud. The GCP
   program in `deploy/registry` stays but is not used for production.
4. **Network.** A new VPC with two public subnets (`us-west-1a`,
   `us-west-1c`) and no NAT. Tasks get public IPs for egress. Security
   groups admit inbound traffic only from the ALB, and the database admits
   only the tasks.
5. **Compute.** ECS on Fargate Spot, ARM64 (Graviton), 0.25 vCPU / 0.5 GB,
   autoscaling from 1 to 4 tasks on 60% CPU. With one Spot task, an
   interruption causes a brief outage until a replacement starts; this cost
   trade-off was accepted.
6. **Database.** RDS Postgres, `db.t4g.micro`, single-AZ, encrypted, with
   automated backups. It can move to Multi-AZ later without a rebuild.
7. **Blob storage.** A new S3 backend for the object store, selected by an
   `s3://bucket/prefix` value in `GIST_OBJECT_STORE_ROOT`. The filesystem
   backend stays for local development and tests. The bucket is private,
   versioned and encrypted, and is reached through the task role.
8. **Images.** AWS CodeBuild builds from the public `sirerun/gist` repository
   at an exact commit, started manually per release. It pushes to an
   immutable ECR repository, and the stack pins the image by digest.
9. **DNS.** `sire.run` is served by Cloudflare. Pulumi cannot own the records
   because no token with token-creation rights is available. The stack
   outputs the ACM validation and ALB CNAME records; they are written through
   the Cloudflare MCP, and a checked-in read-only script checks that Cloudflare
   matches the stack outputs. If a scoped token is added later, the stack
   takes the records over.
10. **Budget.** An AWS Budget of $50 a month on the new account, emailing at
    50%, 80% and 100% of forecast. Alerts only; nothing is stopped
    automatically. Expected run cost is about $25 to $35 a month.
11. **Data.** The catalog launches empty and fills only through authenticated
    publish calls. The connection broker stays unset until R4.
12. **Applying changes.** GitHub Actions is unavailable, so every change is
    merged on local validation. The agent may run `pulumi up` on the
    `registry-aws` stack once `pulumi preview` matches the merged change.
    This standing permission covers this stack only.

## Consequences

- Production has no human review of each apply and no CI. Local validation
  and the preview-matches-PR check are the only gates. A bad apply is
  recovered by reverting the change and applying again.
- Two DNS records live outside Pulumi state. The reconcile script is the
  control for drift.
- Single-AZ database and single Spot task mean availability is below ADR
  007's 99.5% target until funding allows Multi-AZ and a second on-demand
  task. RFC-002 production evidence (E3, Q8) records that gap rather than
  hiding it.

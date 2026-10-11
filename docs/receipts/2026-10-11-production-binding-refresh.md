# Production binding read-only refresh

Date: 2026-10-11. Scope: refresh AWS and DNS bindings for the existing Gist production destination. Read-only checks only.

- The AWS CLI authenticated as a user principal. Its account was compared locally with the `registry-aws` Pulumi stack `accountId`; they differ. Three CLI profiles are configured, two authenticate to an account, and none matches the stack target. Account IDs and profile names are intentionally omitted. The DGX environment has no AWS CLI.
- The Cloudflare MCP returned HTTP 200 for the exact `sire.run` zone lookup; the zone is active. An exact `gist.sire.run` DNS-record lookup returned HTTP 200 with zero records. This confirms current read access and absence of the queried record, not write authority.
- No AWS role assumption, stack preview/apply, DNS write, certificate request, provider action, deployment, or cost-incurring operation was attempted.

Remaining: the owner-approved AWS account/role and deployment workflow, current artifact path, numerical change/cost limits, and DNS write authority. T-GR-SCOPE.11 and all production/deployment gates remain open.

# Production binding preflight — T-GR-SCOPE.11

Date: 2026-10-04. Outcome: BLOCKED for production writes; partial read-only qualification.

- aws, pulumi and gh binaries are installed; GitHub authentication and Pulumi identity succeed.
- Read-only Pulumi registry-aws stack output exists, has its current production cluster/service, object store, database and CodeBuild/ECR path, and retains the prior canonical origin/certificate validation record. Its last listed update is 2026-09-28. Output presence is not current live inventory or permission proof.
- The environment AWS session authenticates, but its account differs from the registry-aws stack account. ECS and ACM lists in that session's region were empty; this does not mean production resources are absent. Default profile with environment credentials removed has no credentials. Do not deploy into the wrong account, guess a cross-account role, or widen trust.
- The private infrastructure original checkout is stale; read exact remote main before candidate work and use its own isolated external-SSD worktree/ownership. No stack selection, update, refresh, secret retrieval or build/deployment was performed.
- No configured Cloudflare MCP binding is callable in this harness inventory; local configuration records that server disabled. No global activation or OAuth change was made. Sites tools are not a permitted Cloudflare fallback. Required configured binding/owner remains an execution-capability block for DNS writes.
- Reuse applicable accepted ADR008 reviewed-preview grant and CI alternative once actual operator/account and inputs are qualified. Those grants do not remove wrong-account, absent binding, numerical provider or separately billable preview gates.

Unblock: qualify the existing authorized registry AWS operator session/role, configured Cloudflare MCP execution owner, actual stack/source bindings and applicable preview/build bounds. Account IDs, credential identities, backend URLs and internal endpoints stay in private local evidence rather than public records.

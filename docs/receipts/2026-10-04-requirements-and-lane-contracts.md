# Gist recovery requirements and proposed lane contracts

Tasks: T-GR-SCOPE.2 and independent disposition by T-GR-SCOPE.3.
Baseline: `84e14128565058419a9b90619f27fa517ba4f494`.
Status: proposed contracts pending independent reviewer; no engineering lane admitted yet.

## Required outcomes

| Requirement | Owner lane | Positive and denial evidence |
| --- | --- | --- |
| Invited human login and bounded workload parent issuance | AUTH | Real approved issuer/membership, PKCE/consent; no public bootstrap, asserted identity, foreign-workspace or revoked-parent issuance |
| Replica/restart-safe signing keys | KEYS + INTEGRATE | Same explicitly provisioned key material across independent keysets; invalid/missing key material refuses startup; bounded old-public-key overlap, unknown/expired keys denied |
| Canonical frozen v1 resolve and exact closure | WIRE + INTEGRATE | Frozen request/response schema, actual immutable pins/digests/bindings, REST/MCP parity; incomplete/private/revoked/over-budget never ready |
| Durable events and atomic revocation | EVENT + INTEGRATE | PostgreSQL persisted cursors/feed and one atomic revoke+event transaction; restart/replica resume, foreign/missing/expired cursors uniform denial, bounded retention and cursor count |
| Action-specific immutable provider artifacts | TREG/COMPOSIO | Official real schema/provenance/license/version closure; no invented schemas/live evidence or unbounded execution grants |
| Actual caller qualification | consumer owner + RELEASE/M3 | Exact import/runtime profile and protected funding; deny before send, persistent unknown outcomes and no retry after uncertainty |
| Canonical AWS origin and full production | AWS + RELEASE/M2B/M3/PROD | Existing original milestone/use-case obligations, exact image/config identity, actual DNS/TLS/OAuth/client acceptance, cleanup/recovery/ops; terminal conjunction only |

Existing use-case IDs and historical task statuses remain intact. Managed execution gateway, business organization setup and private application work remain outside this delivery.

## Proposed component allocation

- KEYS owns only new `hosted/internal/identity/keyring.go` and `keyring_test.go` plus its receipt. Export a strict explicit-config key-ring loader producing KeySet, not a generator at service startup. Current private key and kid are supplied by the operator-owned secret store, with optional explicit retired public keys and retirement timestamps. Reject duplicate/reused kid, mismatched public/private key, unrecognized fields, invalid algorithm/length and malformed data; never include key bytes in errors. Do not derive workload keys from consent secret or put private keys in PostgreSQL. Coordinator owns config/CLI wiring; actual secret provisioning waits for AWS operator qualification.
- EVENT owns new `hosted/internal/storage/events.go`, durable event tests, `storage/revocations.go` atomic outbox changes and an allocated follow-up migration only if needed. Design must retain existing forced-RLS schema, transaction-scoped tenant isolation and the existing rest.EventCursorOpener interface or return a narrow context-aware extension for coordinator wiring. Preserve seven-day event retention, five-minute sliding cursor TTL, 16 active cursors per principal with oldest eviction, and at most 100 events per page; verify those bounds plus restart, replica, unavailable-store and transaction-failure cases. Retention-floor bookkeeping must distinguish purged tenant events from sequence gaps belonging to other tenants. Response-budget failures must not silently discard unreturned events, and repeated delivery remains duplicate-tolerant. No new broker or process-local success fallback. App and publication wiring remain integrator-owned.
- WIRE owns resolution package and a new separate canonical adapter file/test in app; coordinator alone extracts/removes the legacy adapter from app.go when inputs land. Frozen contract changes require an explicit finding/ADR path, never silently regenerated baselines. A binding is selected at its actual version, not hardcoded 1.0.0.
- AUTH remains blocked on actual issuer/operator trust and custody. Provider implementation cannot create paid sessions/calls or invent absent schemas. Independent offline lanes do not inherit these trust decisions.
- INTEGRATE alone owns app.go/config.go, app/adapters.go, hosted/cmd/registry/main.go, shared ports and joined live harness. Reserve migration 008 for EVENT follow-up only if existing 004 is insufficient, and 009 for AUTH enrollment only after trust selection; verify these names remain unused against fresh main before dispatch. No other lane owns these migration IDs. Merges serialized; exact head/base change requires affected checks and independent re-review.

Review: different GPT-6-Luna identity from the candidate author, exact base/head and findings/disposition recorded. No participant accepts its own changes. Builds require load <=10 and won shared lease; external SSD caches only. Current engineering admission is held pending independent contracts, capacity/CI receipts and build sanity.

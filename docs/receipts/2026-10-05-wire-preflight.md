# WIRE preflight — 2026-10-05

Original base: `235b1f3f86c66d6f943a910369688be46291e1d6`; admission commit: `fb3518e` (`87cec14`). Shared portable-pin dependency: `4bde90c88d7eecb9c495d2a3c23a5d04b92f00d8`. Candidate source base is the shared dependency head until coordinator wiring lands. The admitted write set is `hosted/internal/resolution/**` and new `hosted/internal/app/canonical_adapter.go` with its tests. Coordinator owns app.go, config, ports, legacy resolver adapter extraction, and application wiring. Frozen `contracts/registry/v1/**` is read-only.

## Contract and baseline findings

- Frozen resolve request is exactly `skill_ref`, `runtime` (`id`, optional `owned_connections`), and positive `max_bytes`; additional properties are forbidden. Workspace and principal come only from authenticated `ports.Principal`.
- Current resolver's binding selection comes from undeclared `selected_bindings` and constructs a binding ref with hardcoded version `1.0.0`. It does not emit immutable digest closure. Current app adapter reads legacy `skill`, `runtime_id`, `local_execution`, `owned_connections`, `selected_bindings`, and optional `max_bytes` fields.
- Binding v1 requires capability/tool/provider refs, adapter version, fixture digest, passed conformance and `exact_versions:true`. Resolve must deny ambiguous binding selection, missing or invalid closure members, unsupported runtime, gateway execution, revoked/private records and any incomplete set. `owned_connections` is a disclosed client assertion only.
- Frozen wire fixture examples establish aggregate/findings fields, including `required` and optional `provenance: client_asserted`; no frozen resolve response schema exists. The implementation preserves those names and emits explicit exact references/digests as closure data without touching the baseline.
- The resolve request names `runtime.id` but defines no compatibility enum. The resolver enforces declared `local_execution` against the pinned tool's frozen `execution_location`, denies gateway-only execution, and does not infer broader runtime compatibility from opaque IDs. Filesystem requirements cannot be proven because v1 has no runtime permission field; those findings stay unready.

## Intended API and checks

- Extend only resolution's local catalog dependency with workspace-scoped binding discovery; keep public shared `ports` unchanged. Keep caller-supplied binding IDs out of canonical request handling.
- Add `app.NewCanonicalResolver(*resolution.Resolver, int)` and an adapter implementing the existing `rest.Resolver` interface; strict decode rejects duplicate/unknown/missing/invalid fields and maps authenticated principal into the resolution request.
- Package fixtures cover sole binding at a version other than `1.0.0`, ambiguous bindings, unsupported runtime, gateway-only, incomplete closure, revoked/private access, max-byte refusal over final response envelope, and REST/MCP common resolver body behavior.
- Host load at preflight: 17.17 1-minute load (checked after dispatch); source edits/gofmt permitted, Go build/test/lint held until load <=10 and shared lease acquired for multipackage commands. No RED/GREEN run is claimed before an allowed test run.

Claims won by this lane: T-GR-WIRE.0 `3c29d6b3bf6e9d9324b602ad50bc4f725c06518f`; T-GR-WIRE.1 `3b31997e7c9dce4729ba83363ec315aa742b948c`.

Implementation boundary: catalog pins are trusted values persisted in the publication transaction. PostgreSQL stores metadata as JSONB, so the resolver validates digest shape and reference/state consistency and emits persisted package/manifest pins; it does not hash normalized JSONB to recreate the original manifest-byte digest. Exact package digest verification remains the artifact download/package verifier's responsibility.

Exact WIRE-owned write set: `hosted/internal/resolution/resolve.go`, `hosted/internal/resolution/resolve_test.go`, `hosted/internal/resolution/policy_test.go`, `hosted/internal/app/canonical_adapter.go`, `hosted/internal/app/canonical_adapter_test.go`, and this receipt. Coordinator wiring/adapters and REST/MCP route-parity tests remain outside this component candidate. Source tests are still held pending coordinator test-slot reconciliation; `gofmt` and `git diff --check` have run, no Go test result is claimed.

# Runtime handoff contracts

Status: R1 contract mapping, 2026-09-25. This is a handoff contract, not an
implementation or a claim that any runtime has passed acceptance. RFC-002 §13
is the authority. The registry supplies data and receipts; the consuming
runtime remains responsible for its own worker, connection, policy, and
protected-effects lifecycle.

## Common handoff envelope

Every adapter carries these fields and preserves them in its local receipt:

| Field | Requirement |
| --- | --- |
| `principal` / `workspace` | Derived from the validated credential and membership; never trusted from a caller-supplied workspace field. |
| `skill_ref` | Immutable skill ID, exact version, package digest, and source revision. |
| `resolution_ref` | Exact resolution ID, contract version, lease expiry, and resolver policy revision. |
| `capability_ref` | Namespaced capability ID and exact contract version. |
| `effects` | Versioned effect terms, class (`read-only`, `disclosure`, or `mutation`), and sensitivity. |
| `connection` | Runtime-owned connection requirement/assertion status; no provider secret or Gist bearer token is copied into a worker. |
| `receipt_ref` | Stable, redacted evidence reference linking the operation to the runtime build and fixture. |

The adapter must report all unresolved or unsupported requirements before the
worker is marked ready. Discovery, resolution, and a successful fetch never
constitute execution authority.

## Policy-gated runtime

Selected M2a target: **OpenAI Codex CLI, stable public release pinned at R3**.
Codex CLI is a suitable policy-gated target because it exposes explicit
approval/sandbox policy surfaces around tool and command execution. R1 does
not claim that Codex already accepts Gist grants; X-R1 must qualify the adapter
against the pinned build. The handoff below is the required boundary.

### Grant and policy fields

| Field | Required value/behavior |
| --- | --- |
| `tool_uri` | Canonical URI for one resolved operation, including namespace; exact tool URI match only. A taxonomy, family, prefix, or `*` grant is invalid. |
| `tool_version` | Exact immutable capability contract version; absent or range-valued versions are invalid for a grant. |
| `parent_bounds` | Parent principal, workspace, allowed child/worker identity, allowed capability set, effect ceiling, and expiry. A child cannot widen any bound. |
| `spend_policy` | Separate policy object containing currency/amount ceiling, period, approval requirement, and recipient/merchant constraints. It is evaluated independently from capability authorization. |
| `source_refs` | Skill ID/version/digest, `SKILL.md` or package entrypoint, capability ID/version, resolution ID, and source revision. |
| `dispatch_state` | Current grant revision, lease expiry, revocation state, and last event cursor. |
| `connection_assertion` | Runtime-owned connection status and scope/audience summary only; never credentials. |
| `protected_effects` | Existing runtime attempt record, `unknown` outcome state, submission key, timeout, approval route, and no-hidden-retry behavior. |

### Call sequence

1. `gist_resolve` returns a complete result. The adapter rejects missing,
   unsupported, wildcard, range-valued, or conflicting bindings.
2. `gist_get` fetches each exact execution schema and source reference. The
   adapter verifies the returned version and digest before registration.
3. The adapter maps each operation to an exact runtime tool URI/version,
   attaches parent bounds and the independent spending policy, and preserves
   the source/resolution references.
4. The runtime's existing policy and protected-effects machinery validates
   connection requirements and marks the worker ready only when every
   requirement is resolved. Otherwise it returns `unsupported_runtime`,
   `requires_connection`, or `unresolved_requirement` with itemized causes.
5. Immediately before dispatch, the runtime rechecks the current grant,
   parent bounds, spend policy, connection assertion, lease, and effect
   approval. It must not rely on readiness-time state.
6. The runtime executes through its existing protected-effects engine. Gist
   execution-schema idempotency and timeout fields become its submission-key
   and timeout inputs; Gist effect classes become approval-routing inputs.
7. The runtime consumes the §9 event feed. A matching revoke/version event
   invalidates the local grant; a missed cursor, lease expiry, policy-store
   failure, or uncertain event stream forces resync and a dispatch recheck.
8. The adapter emits a redacted receipt with the decision, policy revision,
   event cursor, submission key, and outcome. It never forwards a Gist access
   token as a provider token.

### Error cases

| Condition | Required result |
| --- | --- |
| Wildcard/taxonomy grant or widened parent | Reject binding; `invalid_grant`; worker not ready. |
| Spend policy absent, merged into capability policy, or over ceiling | Reject before readiness/dispatch; `spending_policy_required` or `spending_denied`. |
| Missing source/digest or schema version mismatch | Reject; `invalid_artifact` or `version_mismatch`. |
| Revoked version, expired lease, or missed event cursor | Stop dispatch, resync, and re-resolve; no best-effort execution. |
| Missing connection or wrong audience/scope | Return `requires_connection`/`connection_invalid`; reuse the runtime-owned challenge. |
| Protected-effects engine reports `unknown` | Persist the engine's attempt/unknown state and require its confirmation/recovery flow; never hidden-retry. |
| Policy/event store unavailable | Fail closed for dispatch; exact retrieval may continue only if authorization remains evaluable. |

### Policy evidence receipt

The X-R1 receipt must include runtime product/build and adapter revision,
principal/workspace fixture, exact URI/version grants, parent-boundary negative
test, independent spend-policy negative test, source/resolution references,
connection assertion disclosure, dispatch-time revoke test, missed-event
resync test, protected-effects attempt ID/submission key, final decision, and
redacted logs or trace hashes. Contract-only fixtures must be labeled as such;
they are not live acceptance.

## Artifact-importing runtime

This archetype is selected for the M3 importing-runtime line after X-Z3
provides a pinned releasable build. It may be any runtime that already owns
artifact upload/import, validation, evaluation, worker binding, and
protected-effects APIs; this document deliberately does not invent or replace
those APIs.

### Import fields

The adapter records the immutable `skill_ref`, complete file inventory and
digest closure, source revision, required capability/contract versions,
runtime requirements, effects, connection requirements, and the runtime's
native artifact ID/import ID/evaluation ID/binding ID. The native IDs must map
back to the Gist `skill_ref` and `receipt_ref`; a filename or mutable display
name is not identity.

### Call sequence

1. Resolve and fetch the exact package version from Gist as a complete package,
   including every referenced file. Verify the manifest, file digests, package
   digest, size limits, and digest closure before handing bytes to the runtime.
2. Submit the verified package through the runtime's **existing artifact-upload,
   validation, evaluation, and worker-binding lifecycle**. Do not register a
   second Gist-specific install path, execute scripts during import, or bypass
   runtime validation.
3. Let the runtime's existing validation and evaluation operations run. Record
   their native results and preserve unresolved requirements; evaluation does
   not silently grant missing capabilities.
4. Bind the evaluated artifact through the runtime's normal worker-binding
   operation. Preserve the Gist version/digest and explicit binding revision.
5. Reuse the runtime's existing connection and protected-effects machinery.
   Map execution-schema idempotency, timeout, and effect class into that
   machinery; do not insert RFC-003 gateway execution when the runtime can
   execute through its own engine.
6. On `GET /v1/skills/{id}/versions` or an event notice, show a new version as
   available. Fetch, verify, validate, evaluate, and update the binding through
   the normal runtime operation only after explicit operator/runtime policy
   permits it. Never silently replace a package already attached to a worker.
7. Emit an import receipt containing package and closure digests, native
   operation IDs, binding revision, update decision, connection/effects state,
   and the runtime build.

### Error cases

| Condition | Required result |
| --- | --- |
| Missing asset, bad digest, incomplete closure, or oversize package | Abort before import; `invalid_artifact`; retain no partial binding. |
| Upload/import API unavailable | Report `runtime_import_unavailable`; do not use a second path. |
| Validation or evaluation failure | Preserve native diagnostics; worker remains unbound/not ready. |
| Capability/connection requirement unsupported | Report itemized `unsupported_runtime`/`requires_connection`; no implied readiness. |
| New version notice | Mark available and require explicit normal update; never replace in place silently. |
| Revoked bound version | Consume the event, stop or quarantine according to the runtime's existing policy, and require re-evaluation/rebind. |
| Protected-effects `unknown` outcome | Use the runtime's attempt/confirmation/recovery machinery; no hidden retry. |

### Import evidence receipt

The X-R2 receipt must include runtime product/build and adapter revision, exact
package/version/digest, complete closure manifest hash, upload/import,
validation, evaluation, and binding operation IDs, explicit-update test,
unsupported/error cases, connection assertion disclosure, protected-effects
mapping, and redacted trace hashes. It must state whether the worker was
externally driven or used a qualified hosted model (X-Z1); it must not claim
M3 acceptance from a contract-only run.

## Shared protected-effects invariant

Both archetypes retain the runtime's existing protected-effects machinery:
attempt records, `unknown` outcome confirmation, submission keys bound to
principal and operation, timeout handling, effect-class approval routing, and
no hidden retries. Gist supplies mappings and evidence requirements only. A
runtime that cannot expose these controls is unsupported for the corresponding
acceptance line, even if discovery and artifact retrieval succeed.

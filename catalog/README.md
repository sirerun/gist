# Reusable skill and tool catalog

The `catalog` packages expose the same domain implementation used by the hosted
registry. A caller supplies storage, authorization, current connection state and
resolution persistence. Importing the library starts no server and executes no
provider operation.

| Package | Responsibility |
| --- | --- |
| `discovery` | Authorized bounded lexical candidate discovery |
| `resolution` | Exact capability/binding closure and explicit readiness findings |
| `packages` | Bounded ZIP parsing, complete inventory verification and pinned retrieval |
| `ports` | Shared catalog, artifact, policy and connection contracts |
| `preparation` | Optional intent ranking and complete authorized skill/schema/guide context |
| `provenance` | Detached publisher attestations with caller-owned signing and current trust |
| `integrity` | Canonical digest and complete-package verification helpers |
| `skills/inboxbriefing` | Maintained read-only briefing instructions and template |

Start with the external-package example in
[`skills/inboxbriefing/consumer_test.go`](skills/inboxbriefing/consumer_test.go).
It calls `discovery.New(...).Search`, `resolution.NewResolver(...).Resolve` and
`packages.Retriever.RetrieveSkill` through public imports. The controlled test
binding is not a configured Gmail connection. Without an admitted provider the
same skill resolves incomplete.

`RetrieveSkill` requires an externally trusted `ports.ArtifactPin` identifying
the exact retained ZIP and manifest SHA-256 digests, plus explicit archive limits.
It checks current authorization, identity, admission state, package inventory,
all declared file bytes, and state again after retrieval. `Package.Files` retains
instructions, manifest and assets; `ManifestBytes` retains the exact verified
manifest, including fields not represented in the minimal Go `Manifest` type.
Consumers must preserve and interpret required capability declarations rather
than silently dropping unknown metadata.

Digest verification is not publisher authentication. The bundled skill declares
operator-asserted provenance; the consumer must decide whether to trust its
pinned source, import it as a draft, and admit it through its own policy. Public
publisher signing is available as an additive source API in `provenance`; public
release qualification remains separate. See [ADR019](../docs/adr/019-catalog-publisher-provenance.md).

`provenance.Sign` accepts a caller-owned Ed25519 `crypto.Signer` and a complete
immutable statement. `provenance.Verifier.VerifySkill` takes an independently
selected expected subject, detached envelope, retained ZIP and explicit archive
limits. Configure its `TrustResolver`, clock, maximum authoritative trust age and
retired-key policy. The resolver authenticates exact workspace/publisher/key
identity and current revocation with freshness evidence; Gist discovers no trust
or keys. The verifier authenticates the statement, checks every declared package
byte, then rechecks trust. Its result is point-in-time publisher evidence, not
execution authority or an automatic manifest-trust upgrade. The public consumer
example and negative cases are in [`provenance/consumer_test.go`](provenance/consumer_test.go).
Source/schema claims are signed assertions; consumers still select the expected
schema/source and enforce their own schema validation and admission. Public
distribution remains blocked pending the separately qualified release.

Resolution is not execution permission. Consumers must recheck their current
connection, tool contract, scope and effect policy before exposing or executing
tools. Revoked records and bindings must not be retained as an offline permission
cache. Discovery and resolution budgets reject incomplete output rather than
truncating executable contracts.

The inbox workflow may list/read up to 20 selected messages and prepare a briefing.
It must not send, draft, label, mark read, delete, follow links or fetch attachments.
Connection setup, credentials, mailbox consent and execution belong to the
consumer. No provider credentials are included in the maintained package.

Run the source consumer checks with `go test ./catalog/...`. They use controlled
storage and bindings, not a live mailbox. Optional preparation is available as experimental source in `preparation`.
Its caller supplies current catalog/policy, exact resolver, package retriever and
optional scoped embedding adapter. Positive task coverage requires an explicit `TaskAdmitter` scoped to exact
intent/principal/runtime/immutable skill and capability contract; similarity alone
never makes an unrelated task ready. Credentialed readiness additionally requires
an explicit fresh `CurrentReadiness` port bound to principal/runtime/skill/closure;
absence never means a connected account. Only the maintained inbox recipe is
covered. Complete verified instructions/assets/schemas are returned or the budget
is refused; no execution grant is returned. See ADR020 and `tools/evaluation`.
The real pinned text-only embedding adapter is locally qualified, but held-out
quality, whole-task comparisons, Zati integration/default enablement and release
remain separate. No token or latency advantage is claimed.

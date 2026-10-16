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
publisher signing remains a separate qualification track.

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
storage and bindings, not a live mailbox. Semantic retrieval and dynamic guide
assembly are planned evaluation work; this library foundation makes no measured
token or latency improvement claim.

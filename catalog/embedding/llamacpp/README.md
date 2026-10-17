# Explicit local native embeddings

This optional adapter implements `preparation.Embedder` for an explicitly
caller-owned, integrity-verified `llama-server`. It neither downloads nor starts a
runtime, changes preparation defaults, authorizes tasks, nor executes tools.

```go
embedder, err := llamacpp.New(llamacpp.Config{
    Origin: "http://127.0.0.1:PORT",
    APIKey: privateKey,
    BinarySHA256: verifiedBinarySHA,
    ModelSHA256: llamacpp.ModelSHA256,
    Dimension: 256, // or768
    Alias: configuredServerAlias,
})
// Handle err, retain caller ownership and close transport resources on shutdown.
service.Embedder = embedder
service.ModelVersion = embedder.ModelVersion()
```

The caller must verify the actual executable/model bytes, own the child process,
private API key and loopback endpoint, authenticate readiness, enforce resources,
and stop/reap it. Configuration digests are assertions from that trusted caller;
a server response cannot attest its binary/model identity. The adapter omits the
request `model` field because upstream otherwise echoes it, and additionally
checks the configured response alias. Never log Config or private request bodies.

`New` accepts only explicit numeric loopback HTTP origins with ports. Its private
transport ignores environment proxies, refuses redirects and has no automatic
retry/reused connections. A complete Embed operation has a60second ceiling,
respecting earlier caller cancellation. Errors omit response bodies, keys and
input text.

Purpose prefixes are applied once to raw input: query `task: search result | query: `,
document `title: none | text: `. Already prefixed input is refused. Each complete
input is tokenized with special-token insertion/parsing enabled before any
inference. BOS2/EOS1,8192tokens/input,32768tokens/batch,64texts,32768UTF-8bytes/text
and262144rawbytes/batch are enforced. Empty text is allowed with its required
purpose prefix; an empty batch is refused. This avoids relying on unsupported
`truncate:false` server behavior.

Responses must have canonical ASCII field names, unique exact indices, matching
alias/count/token usage and768finite nonzero L2-normalized values. The adapter
refuses malformed or nonunit native vectors. For256dimensions it truncates the
native768 vector and renormalizes;768 is also normalized before return. Runtime,
GGUF, native pipeline, exact prefixes and requested dimension bind ModelVersion.
Different binaries or dimensions cannot silently reuse a vector/index identity.

Reference engine commit is `620d2225cc436ab0b001e3200e590e3b286ef9ab` (v0.7.0),
model is the pinned BF16 text GGUF identified by `ModelSHA256`. Exact native/Python
numerical qualification and its limits are recorded under
[tools/evaluation/native-embedding](../../../tools/evaluation/native-embedding/).
No Mac, live Gmail, unseen-task quality, quantization or adoption follows from
Linux source/runtime checks.

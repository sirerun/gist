# Independent publication proposal and delivery-graph review

**PR:** [#54](https://github.com/sirerun/gist/pull/54)  
**Base:** `57927cda892ffdd6d7bd048041c77a3bc7a8e8ab`  
**Head:** `59a8e890cf2e7fe49f0ada03beeb4db97ca63f04`  
**Review:** PASS — no blocking documentation or graph findings.

The exact PR diff contains six documentation files only. All six match the prior reviewed candidate content at `9364d9b92e6dc55a2f435cfb0b6dbfd68f5c407c` and the corresponding content at `3f871da`; the base refresh introduced no content change to those files. `git diff --check` passes against the exact base.

The proposal keeps ADR012 explicitly unapproved and unimplemented. It presents versioned JSON/base64 skill transport as a recommendation, documents alternatives and compatibility consequences, preserves frozen v1 entries, and leaves T-GR-PUBLISH.8 as an explicit owner decision. T-GR-PUBLISH.0–.6 remain gated behind that decision. The proposal distinguishes original archive, manifest, inventory, and document-byte digests; specifies bounded complete-response handling and idempotency; and requires per-kind admission, current authorization, transaction/outbox integrity, byte-exact retrieval and restricted-role real-store acceptance before source implementation. It does not claim the historical v1 publisher is canonical or that proposed v2 routes exist.

A read-only absolute plan-parser run on `docs/plan.md` found 239 tasks across 24 epics: 221 active delivery tasks (61 done, 2 open, 158 blocked) and 18 checked historical tasks. The 18 WIRE completions are included in the 61 active completions. There are no duplicate IDs, malformed acceptance rows, undefined or ambiguous wave references, unresolved dependency references, ambiguous dependency references, or cycles. All 220 active tasks other than T-GR-PROD.9 are reachable as its ancestors. The explicit terminal joins include PUBLISH.6 before INTEGRATE.0; CORE merge waits on R4–R8 review rows .18/.21/.24/.27/.30; EVENT merge waits on R5 review .18; and WIRE merge waits on R7 review .19.

This is a documentation and dependency-graph review only. It does not qualify production source, application tests, CI, publication behavior, release, or deployment. The owner decision at T-GR-PUBLISH.8 and all subsequent implementation and acceptance gates remain open.

# Pre-score independent labeling record

Primary author: coordinator `/root`. Secondary author:
`/root/composition_check_route`, a non-harness author. Runner author:
`/root/composition_producer`. Final reviewer is a separate non-author.

Both authors used the frozen protocol, case inputs, public composition contract
and immutable artifact inventory. Neither used evaluated readiness outputs.
The secondary author did not inspect harness source or primary labels. The
coordinator fixed its four ready-case criteria before receiving the secondary
summary, then transcribed exact IDs from the inventory; the primary file was
written before inspecting the secondary file.

All 19 readiness labels and selected binding/tool/connection/account identities
agree. C01, C07, C15 and C16 require a complete ready closure. The other 15
cases must refuse readiness without an executable bundle. Reasons differ only
in wording. Primary labels forbid the private sentinel in every case; secondary
labels name it for C13, the only case containing it. Reconciliation uses the
primary's stronger global disclosure check without changing any expected result.

The secondary inventory audit found C16 initially keyed ConnectionSelections by
group instead of requirement. Before any scoring, the fixture was corrected to
list/read keys and regenerated from source e0d6598. Runtime-control facts were
also made explicit. Artifact bytes and selected identities stayed unchanged.
The earlier inventory/provisional labels remain in the operational attempt record.
The corrected inventory SHA256 is
`d0d6e4ba456a8aa7333c4b26fbcb2022f339e12a08dffb104f046e9e6c447f40`.

`labels-reconciled.jsonl` is the grading input. The report additionally compares
full selected candidate pins and returned tool pins with the frozen inventory,
so an ID-only match cannot conceal different versions, digests or connections.
No post-score relabeling or threshold adjustment is permitted. This is one
maintained composition family, not independent evidence across 19 skill families.

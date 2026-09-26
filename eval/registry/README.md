# Registry smoke labels

This directory freezes the E1 labeled corpus before an evaluation runner exists.
The 24 rows in `cases.jsonl` are six exact-target, six no-match, six ambiguous,
and six unauthorized-candidate adversarial cases. Labels are independent of
ranking implementation.

Each row names a principal and workspace fixture, the eligible and relevant
artifact IDs, an expected `ready`, `empty`, or `ambiguous` behavior, explicit
forbidden IDs, and a positive byte budget. The ambiguous rows require a
selection request whenever competing eligible bindings remain. The exact rows
require the relevant ID in the top three. Unauthorized rows require zero
forbidden candidates in visible results.

`corpus.json` uses the real IDs and versions represented by `catalog/registry/`
for the selected skills, capabilities, tool, and bindings. Its workspace
copies, authorization arrangements, and confusable summaries are synthetic but
shape-valid fixtures. No credentials, tokens, customer data, or live-service
results are included.

The corpus hash is SHA-256 over UTF-8 canonical JSON of the full document with the top-level `sha256` field removed, canonicalized as `json.dumps(doc, sort_keys=True, separators=(",", ":"), ensure_ascii=False)`. `cases.jsonl` is hashed as committed bytes.
`thresholds.json` records both frozen hashes. They are labels and release
targets only; E1 does not run retrieval evaluation or claim observed results.

Expected release targets are zero unauthorized leakage, zero no-match
non-empty results, zero ambiguous false-ready results, selection requested for
all ambiguous cases, and 100% exact-target top-three recall.

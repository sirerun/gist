# Four inbox preparation ties

The four retained H1 abstentions are an exact consequence of the frozen ranking
policy, not a native tokenizer/vector error or a missing provider connection.
This diagnostic explains them; it does not select a candidate or change the SDK,
caller policy, gold labels, catalog, thresholds, defaults or old results.

| Case | Request | Lexical top two | Semantic top two |
| --- | --- | --- | --- |
| inbox-menu-14 | Compact unread-email overview with message references | send-mail, inbox | inbox, send-mail |
| inbox-menu-28 | German unread inbox briefing | send-mail, inbox | inbox, send-mail |
| inbox-menu-29 | Portuguese unread inbox briefing | send-mail, inbox | inbox, send-mail |
| inbox-menu-30 | Italian unread inbox briefing | send-mail, inbox | inbox, send-mail |

For every case and both256/768-dimensional retained embeddings, rank1/rank2 are
reversed. Equal-weight RRF60 therefore gives both candidates exactly
`1/61 + 1/62 = 123/3782` (approximately0.032522474881). The larger semantic cosine
gap is not used by this rank-only fusion. Changing k4/8/16 cannot remove a tie
already detected before truncation. All12retained dimension/k/process artifacts
show the same four H0/H1 `requires_selection` cases.

Lexical retrieval ranks distinct matching terms first, then their best field
weights: name3, capability/taxonomy2, description1. In case14, send-mail matches
email/message in naming fields (two terms, weight6), while inbox matches
unread/email in its description (two terms, weight2). In the other cases, the
English mail/email fragment scores higher in send-mail's naming field than in
inbox's capability or description. This is a conflict between retrieval ranks;
it does not mean sending email was admitted or that these briefing requests are
unsupported by the maintained read-only recipe.

## Reproduce without inference

From the repository root:

```sh
python3 tools/evaluation/inbox-diagnostics/rrf_ties.py
python3 -m unittest discover -s tools/evaluation/inbox-diagnostics -p 'test_*.py'
```

`diagnosis.json` is the deterministic output. `frozen-hashes.json` binds19public
source/artifact files: the lexical/discovery/preparation source, fixed catalog,
30development inputs, two vector artifacts and12attempt artifacts. The
reproducer verifies fingerprints before interpretation and refuses any file
outside that allowlist. It does not read either annotator's labels, reconciled
gold, original held-out data or model weights. Cosines are arithmetic over
already retained vectors; there is no model/provider/build job.

The lexical transcription is scoped to this fingerprinted, fixed corpus. It is
not a new general retrieval implementation. Source mismatches fail instead of
silently adapting its analysis. Tests verify exact rational ties, lexical field
weights, every retained configuration, fingerprint refusal and exclusion of gold
paths. `SHA256SUMS` binds all diagnostic files except itself.

## Settled follow-through

Preserve generic `requires_selection`. Do not pick the lexicographically first
ID, retune weights/floor on these outcomes, delete distractors or rewrite gold to
force30/30. The existing ambiguity regression
`TestTopKOneCannotHideSemanticAmbiguity` intentionally prevents truncation from
hiding a tied candidate. The current recipe/task-admission checks occur after
ranking/abstention; those checks are not evidence that a tied send-mail skill can
execute.

Zati already supplies an explicitly selected immutable skill route. At inspected
revision `fab1de071cffcc835a3371e3158da031d158c3cd`,
[`internal/execution/catalog_skills.go`](https://github.com/zati-run/zati/blob/fab1de071cffcc835a3371e3158da031d158c3cd/internal/execution/catalog_skills.go#L47)
resolves the recipe's selected `SkillVersions` directly, then checks exact
admitted two-role tool/connection versions and current schemas. It does not call
Gist's lexical/semantic ranking. Reuse and qualify that actual controlled path;
no additional Gist pinned-selection API is needed for it.

That caller-selected route is a different preparation condition, not an improved
H1 score. Preserve the original26/30H1 outcome and its one-family limitations.
Any full-task comparison must give both arms equivalent task/tool/selection
information or disclose the routing-information confound. Controlled end-to-end
execution, factual output quality, full-model turns/tokens/latency, live Gmail
and default/adoption decisions remain separate gates.

# Synthetic preparation pilot corpus

This corpus is an **initial author-labelled pilot**, not reconciled evaluation
truth or an evaluation result. A second annotator must label each input without
reading author-1 labels, then resolve disagreements without retrieval/model
scores before `label_status` changes.

There are 60 development cases and 60 separately sealed held-out cases. Each
split has ten cases in each disjoint primary stratum: straightforward,
paraphrase/multilingual, multiple capabilities/prerequisites, ambiguous/no-match,
missing/stale/revoked, unauthorized/adversarial. Each case retains a distinct workflow identity (`workflow_family`), chosen before
wording. `family` now clusters correlated construction templates conservatively:
44 development groups and35 held-out groups, not60 independent families per
split. Unique IDs never establish independence. A v2 preregistration repair
replaced32 held-out prompts before any scores, and independent construction-split
review is pending. Shared capability infrastructure does not by itself mean
shared task output. Manual intents vary
by domain, requested evidence, source composition, language and abstention cause.

Only one case per split uses the maintained real inbox skill. All other cases
are synthetic nonproduct probes; their catalogue names are fixture definitions,
not shipped product features. This small product cohort cannot qualify inbox
performance, six-stratum product coverage or adoption. A richer independently
labelled inbox cohort must be preregistered before any positive product adoption
decision, before its results are exposed. The broader corpus tests
preparation mechanics and synthetic generalization only. No real mailbox,
private account, provider credential or actual customer record is included.

## Files and schema

`development.inputs.jsonl` contains one input per line:

- `id`, `split`, `family`, `workflow_family`, `stratum`, `intent` identify the independently authored
  request and preregistered grouping.
- `fixture.workflow` chooses the maintained inbox fixture or a named synthetic
  workflow. `fixture.status` is supplied environment state, not a prediction:
  `ready`, `missing_connection`, `revoked`, `unauthorized`, `tampered`,
  `unsupported_runtime`, `no_match`, `ambiguous`.
- `fixture.evidence_class` is `product` or `synthetic_nonproduct`.
- `fixture.skill_id`, `skill_version`, `capabilities`, `tool_alternatives`,
  `dependencies`, `runtime_id`, `workspace_id`, `policy_generation` describe the
  synthetic catalogue/environment. Capability entries have `id` and
  `contract_version`; versioned skill/tool/dependency labels use `id@version`.
- `tool_alternatives` is an array of complete alternative tool closures, not an
  instruction to infer equivalence from names. State and current authorization
  still govern whether any closure is eligible. The alternate closure suffix
  denotes a separately admitted equivalent fixture contract.

`development.labels.author1.jsonl` contains:

- `id`, `family`, `annotator`, `evidence_class`.
- `allowed_skill_versions`: eligible skill pins. Empty for denied, ambiguous and
  no-match cases; missing-connection/runtime cases retain the relevant skill pin
  while refusing a ready bundle.
- `required_capabilities`: complete intent capability closure, including when
  unavailable or unauthorized.
- `eligible_tool_alternatives`: complete currently eligible tool closures, empty
  when the environment cannot safely offer a ready bundle.
- `dependencies`: exact requested/candidate fixture dependency pins. Required
  capabilities and dependencies describe the request or candidate closure, not
  permission or executable eligibility. Only currently eligible complete tool
  alternatives can be exposed, and these are empty for every nonready case.
- `expected_status`: `ready`, `requires_connection`, `denied`, `unsupported`,
  `no_match`, or `requires_selection`.
- `prohibited_actions`, `prohibited_disclosures`, and `output_rubric`: independent
  safety and observable outcome requirements. A safety prohibition still applies
  when a task abstains.

Optional fixture transitions describe synthetic environment changes during
preparation. A runner must enact a transition at its declared phase and observe
real checks, or report that scenario NOT_RUN. It must not use labels or scenario
status as the result oracle. Unsupported probe workflows remain unsupported;
fixtures do not turn synthetic capabilities into product support.

`manifest.json` records counts and exact SHA-256 file hashes. It includes only
an opaque held-out artifact ID and hashes/counts, never held-out intents, labels,
family names or storage locations. `v2_blind_reconciliation_pending` must not be interpreted as two-annotator
qualification. `manifest.v1.json` and retained source history preserve the
superseded initial corpus. Both original label sets remain intact; reconciled
artifacts are separate files. V1 reconciliation clarified requested-versus-
executable capability metadata and normalized an ambiguous rubric prefix so
malicious requested actions are never an instruction to satisfy them.

## Blind seal and reconciliation

The corpus author stores held-out inputs, author-1 labels and the deterministic
construction recipe outside the source checkout in an owner-only directory
(mode `0700`), with files mode `0600`. The coordinator holds its location. File
permissions prevent accidental sharing, not access by another process under the
same OS account: implementers must honor the recorded no-access instruction.
The implementation candidate and Git history contain no held-out text.

The second annotator first receives input-only files for both splits, writes
separate `author2` labels, records hashes and reports disagreement. Only then may
that annotator inspect author-1 labels and reconcile differences with the author.
The reconciled development labels may be shared for development. Held-out inputs
and labels remain sealed until source/model freeze and explicit T-PREP.9
admission. Do not inspect evaluation scores while reconciling labels.

The source harness owner consumes the frozen development schema. Any fixture
limitation is reported explicitly; it is not repaired by changing gold answers
after observing scores. No run, model access or adoption is authorized by this
corpus alone.

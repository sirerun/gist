# Offline preparation evaluation

This source toolchain is experimental. Qualification uses synthetic text, no mail
or paid provider. Held-out evaluation, whole-task comparisons, default adoption
and release are separate plan gates. Do not open sealed held-out inputs/labels
while developing the candidate.

## Explicit local embedding qualification

Create an isolated Python environment on the admitted reference host, retaining
its qualified platform torch build. Install the exact packages in
`requirements-embedding.txt`; never mutate shared packages. Set CPU/BLAS thread
limits to4. The explicit first fetch is pinned to the immutable public model
revision in `embedding_adapter.py`; no Hub account or token is required for its
current public Apache2 access. Every local model/config/tokenizer/pooling file must match the committed
`model-files.sha256.json` before any loader sees it and again before output.
Missing, altered, symlinked or additional files fail closed. The canonical file-set
root also binds vector/index identity. The operator must prevent concurrent
writers to the selected model directory. File hashes and runtime versions are recorded.

```sh
python3 -m venv --system-site-packages .runtime
.runtime/bin/pip install -r tools/evaluation/requirements-embedding.txt
OMP_NUM_THREADS=4 OPENBLAS_NUM_THREADS=4 MKL_NUM_THREADS=4 timeout 1200 \
  .runtime/bin/python tools/evaluation/embedding_adapter.py \
  --model-dir .models/embeddinggemma2 --download --dtype bfloat16 --dimension 256 \
  --output vectors.json --qualification qualification.json
```

Only use `--download` for the explicitly approved initial fetch. Subsequent
invocations omit it and set `local_files_only` plus offline environment flags.
Qualify float32 and768d as well, sequentially under the resource envelope. Inspect
all target verdicts: a successful inference can still fail the RSS/startup target.
The adapter refuses token overflow; it never silently truncates the embedding
input. Vectors carry model/runtime/dtype/dimension identity and exact input text
and purpose. Query prefix is `task: search result | query: `; document prefix is
`title: none | text: `; shortened vectors are normalized after truncation.

The adapter consumes optional JSONL records `{"purpose":"query","texts":[...]}`
or `{"purpose":"document","texts":[...]}` via `--input`. Store only synthetic
public inputs. The output contains vectors, not verified capability coverage or
execution grants. Do not send credentials or private mail through this interface.

Resource limits and statistical protocol are frozen in ADR020. Source/controlled
qualification evidence must not be described as live Gmail, Composio superiority
or native Zati performance.

## Corpus, source runs and offline grading

Validate only the public development split while implementing:

```sh
python3 tools/evaluation/corpus_check.py \
  --inputs tools/evaluation/corpus/development.inputs.jsonl \
  --labels tools/evaluation/corpus/development.labels.jsonl
```

The runner uses one fixed shared catalog: the independently maintained inbox
package plus four static distractors. It never copies a case intent or gold label
into a catalog description, skill instruction or recipe. Scenario inputs only
request supported current-state overlays; unknown transitions are NOT_RUN.
Only the inbox recipe is implemented. Broader synthetic tasks can therefore
fail/abstain; that limitation stays visible. This is a controlled source smoke,
not a qualified held-out retrieval benchmark or a main-model task execution.

On the admitted Go host, replace `SOURCE_SHA` with the exact tracked candidate:

```sh
go run ./tools/evaluation/cmd/prepeval --source SOURCE_SHA \
  --inputs tools/evaluation/corpus/development.inputs.jsonl --dry-run
go run ./tools/evaluation/cmd/prepeval --source SOURCE_SHA \
  --inputs tools/evaluation/corpus/development.inputs.jsonl --embedding-inputs > embedding-inputs.jsonl
# Run the admitted local adapter above with --input embedding-inputs.jsonl,
# producing development-vectors.json; subsequent loads omit --download.
go run ./tools/evaluation/cmd/prepeval --source SOURCE_SHA \
  --inputs tools/evaluation/corpus/development.inputs.jsonl \
  --vectors development-vectors.json > attempts.jsonl
python3 tools/evaluation/report.py --attempts attempts.jsonl \
  --labels tools/evaluation/corpus/development.labels.jsonl > report.json
```

All L0/L1/E0/H0/H1 paths call the real public preparation API. Semantic paths use
real precomputed local vectors identified by exact text/purpose/model version;
their measured source lookup time excludes the separately qualified inference
and must not be reported as complete preparation/task latency. Missing vectors
or unavailable C0/C1 accounts produce NOT_RUN. C0 retains native discovery and
C1 requires a preregistered direct tool set when separately authorized; no fake
Composio executor or paid model is supplied by this source batch.

Records retain exact source, phase/input hash, catalog/recipe/model identity,
actual context/closure and content-result hash, status/error, measured elapsed,
explicit repeat/attempt index and usage categories. Unknown tokens/dollars stay
null. To retain a retry, append a new run with `--attempt-index 2 --run-kind
recovery`; do not replace the failed attempt. The report includes every retained
attempt's timing and available usage, unavailable denominators, stratum and actual
construction-family counts. Gold labels are consumed only afterward for closure,
readiness and abstention grading; they never construct candidates. Source readiness
is not whole-task success. Binary interval helpers use equal construction-family
weighting; pooled per-attempt rates are separate descriptive statistics.

## Held-out admission after candidate freeze

Do not open or generate vectors for sealed held-out cases while tuning. After
source landing and development-only selection, the coordinator records a frozen
JSON admission manifest with:

```json
{
  "candidate_frozen": true,
  "candidate_source": "EXACT_LANDED_SHA",
  "development_selection_sha256": "64_HEX",
  "inputs_sha256": "SEALED_INPUT_64_HEX",
  "labels_sha256": "SEALED_LABEL_64_HEX",
  "catalog_version": "source-smoke-shared-catalog-v1",
  "recipe_version": "inbox-v1",
  "model_version": "EXACT_QUALIFIED_MODEL_RUNTIME_FILESET_VERSION",
  "top_k": 4,
  "semantic_floor": 0.30,
  "rrf_constant": 60
}
```

The runner accepts `--phase heldout --admission admission.json` only when source,
configuration, dataset hash, selected model identity and split match. Its
`--embedding-inputs` path requires admission too. The report requires the same
admission before reading held-out labels and checks their exact frozen hash.
This guard prevents accidental evaluation before selection; it is not an
identity/authorization service. The coordinator controls release of sealed data.
Current source catalog coverage remains limited; T-PREP.9 must qualify the shared
catalog/distractor adequacy before claiming a retrieval benchmark. No unsupported
scenario or unavailable arm is counted as successful task evidence.

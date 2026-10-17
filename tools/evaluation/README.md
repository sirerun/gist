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

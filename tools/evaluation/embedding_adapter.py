#!/usr/bin/env python3
"""Explicit local text-only EmbeddingGemma2 adapter/qualification; no auto network.

Run --download explicitly once in an isolated operator-selected cache; all later
loads use local_files_only. JSONL input records contain purpose=query|document and
texts; output records bind exact text/purpose/model version to normalized vectors.
No task execution, provider access, hidden model fallback or credential handling.
"""
import argparse
import hashlib
import importlib.metadata
import json
import os
from pathlib import Path
import resource
import sys
import time

MODEL = "google/embeddinggemma-2"
REVISION = "914f7f89142e33e77833254d9c9b90c3cef7303b"
PREFIXES = {"query": "task: search result | query: ", "document": "title: none | text: "}


def main():
    parser = argparse.ArgumentParser(description=__doc__)
    parser.add_argument("--model-dir", required=True)
    parser.add_argument("--download", action="store_true")
    parser.add_argument("--dtype", choices=("bfloat16", "float32"), default="bfloat16")
    parser.add_argument("--dimension", type=int, choices=(256, 768), default=256)
    parser.add_argument("--input")
    parser.add_argument("--output", required=True)
    parser.add_argument("--qualification", required=True)
    args = parser.parse_args()
    model_dir = Path(args.model_dir)
    if args.download:
        from huggingface_hub import snapshot_download
        snapshot_download(MODEL, revision=REVISION, local_dir=model_dir,
                          allow_patterns=["*.json", "*.safetensors", "*.model", "*.txt", "README.md"],
                          max_workers=4)
    if not (model_dir / "model.safetensors").is_file():
        raise SystemExit("NOT_RUN: pinned local model unavailable; explicit --download required")
    # Disable all incidental Hub/network access after the explicit fetch.
    os.environ["HF_HUB_OFFLINE"] = "1"
    os.environ["TRANSFORMERS_OFFLINE"] = "1"
    import torch
    from transformers import AutoConfig, AutoModel, AutoTokenizer
    from sentence_transformers.sentence_transformer.modules.pooling import Pooling
    from sentence_transformers.base.modules.normalize import Normalize
    torch.set_num_threads(4)
    versions = {name: importlib.metadata.version(name) for name in
                ("torch", "transformers", "sentence-transformers", "huggingface-hub", "safetensors", "tokenizers", "numpy")}
    versions["python"] = sys.version.split()[0]
    if versions["transformers"] != "5.20.1" or versions["sentence-transformers"] != "6.1.0":
        raise SystemExit("runtime pin mismatch")
    started = time.monotonic()
    config = AutoConfig.from_pretrained(str(model_dir), local_files_only=True,
                                        vision_config=None, audio_config=None)
    tokenizer = AutoTokenizer.from_pretrained(str(model_dir), local_files_only=True)
    model = AutoModel.from_pretrained(str(model_dir), config=config, local_files_only=True,
                                     dtype=getattr(torch, args.dtype), attn_implementation="eager")
    model.eval()
    if getattr(model, "vision_tower", None) is not None or getattr(model, "audio_tower", None) is not None:
        raise ValueError("text-only adapter unexpectedly loaded a modality tower")
    pooling_config = json.loads((model_dir / "1_Pooling/config.json").read_text())
    if pooling_config != {"embedding_dimension": 768, "pooling_mode": "mean", "include_prompt": True}:
        raise ValueError("official pooling contract changed")
    pooler = Pooling(**pooling_config)
    normalizer = Normalize()
    load_seconds = time.monotonic() - started
    version = f"{MODEL}@{REVISION}:transformers={versions['transformers']}:sentence-transformers={versions['sentence-transformers']}:torch={versions['torch']}:tokenizers={versions['tokenizers']}:numpy={versions['numpy']}:python={versions['python']}:{args.dtype}:{args.dimension}d"

    def embed(purpose, texts):
        if purpose not in PREFIXES or not isinstance(texts, list) or any(not isinstance(t, str) or len(t) > 8192 for t in texts):
            raise ValueError("invalid explicit text request")
        prefixed = [PREFIXES[purpose] + text for text in texts]
        # Refuse token overflow instead of silently truncating model input.
        tokenized = tokenizer(prefixed, truncation=False, add_special_tokens=True, padding=True, return_tensors="pt")
        if tokenized["input_ids"].shape[1] > 8192:
            raise ValueError("input exceeds complete model context")
        with torch.inference_mode():
            hidden = model(**tokenized).last_hidden_state
            features = pooler({"token_embeddings": hidden, "attention_mask": tokenized["attention_mask"]})
            pooled = normalizer(features)["sentence_embedding"]
            vectors = torch.nn.functional.normalize(pooled[:, :args.dimension].float(), p=2, dim=1)
        if vectors.shape[1] != args.dimension or not bool(torch.isfinite(vectors).all()):
            raise ValueError("wrong or nonfinite model output")
        return vectors.tolist()

    smoke = ["Summarize unread email without changing the mailbox."]
    cold_started = time.monotonic()
    cold = embed("query", smoke)
    cold_seconds = time.monotonic() - cold_started
    warm_seconds = []
    for _ in range(5):
        before = time.monotonic()
        warm = embed("query", smoke)
        warm_seconds.append(time.monotonic() - before)
    entries = []
    if args.input:
        with open(args.input, encoding="utf-8") as stream:
            for line in stream:
                request = json.loads(line)
                vectors = embed(request["purpose"], request["texts"])
                entries.extend({"purpose": request["purpose"], "text": text, "vector": vector}
                               for text, vector in zip(request["texts"], vectors, strict=True))
    else:
        entries = [{"purpose": "query", "text": smoke[0], "vector": cold[0]},
                   {"purpose": "document", "text": "Read-only inbox briefing Summarize recent unread email without changing the mailbox.",
                    "vector": embed("document", ["Read-only inbox briefing Summarize recent unread email without changing the mailbox."])[0]}]
    rss = resource.getrusage(resource.RUSAGE_SELF).ru_maxrss * 1024
    if rss > 24 * 1024**3:
        raise SystemExit("qualification resource envelope exceeded")
    hashes = {}
    for path in sorted(model_dir.glob("**/*")):
        if path.is_file() and ".cache" not in path.parts:
            digest = hashlib.sha256()
            with path.open("rb") as stream:
                for block in iter(lambda: stream.read(1024 * 1024), b""):
                    digest.update(block)
            hashes[str(path.relative_to(model_dir))] = digest.hexdigest()
    output = {"model_version": version, "entries": entries}
    Path(args.output).write_text(json.dumps(output, sort_keys=True, allow_nan=False) + "\n")
    import inspect
    from transformers.models.embedding_gemma2 import modeling_embedding_gemma2
    implementation_hashes = {}
    for name, implementation in (("transformers_model", modeling_embedding_gemma2), ("sentence_pooling", Pooling), ("sentence_normalize", Normalize)):
        implementation_hashes[name] = hashlib.sha256(Path(inspect.getfile(implementation)).read_bytes()).hexdigest()
    qualified = {"model": MODEL, "revision": REVISION, "runtime": versions, "dtype": args.dtype,
                 "dimension": args.dimension, "device": "cpu", "threads": 4, "text_only": True,
                 "loaded_parameters": sum(p.numel() for p in model.parameters()),
                 "pooling": pooling_config, "implementation_sha256": implementation_hashes,
                 "projection": "official model forward projects per-token512to768 before maintained masked-mean pooling; prompts included; maintained normalization then truncate and renormalize",
                 "adapter_sha256": hashlib.sha256(Path(__file__).read_bytes()).hexdigest(),
                 "prefixes": PREFIXES, "load_seconds": load_seconds, "cold_inference_seconds": cold_seconds,
                 "warm_inference_seconds": warm_seconds, "peak_rss_bytes": rss, "file_sha256": hashes,
                 "model_version": version, "synthetic_smoke_only": True,
                 "targets": {"rss_4gib_pass": rss <= 4 * 1024**3, "load_30s_pass": load_seconds <= 30},
                 "usage": {"provider_calls": 0, "paid_model_calls": 0, "model_tokens": None, "dollars": None}}
    Path(args.qualification).write_text(json.dumps(qualified, indent=2, sort_keys=True, allow_nan=False) + "\n")
    print(json.dumps({key: qualified[key] for key in ("dtype", "dimension", "load_seconds", "cold_inference_seconds", "warm_inference_seconds", "peak_rss_bytes", "targets")}))


if __name__ == "__main__":
    main()

#!/usr/bin/env python3
"""Reproduce four frozen RRF ties without inference, gold labels or selection.
This is a scoped transcription of the fingerprinted lexical/preparation source,
not a replacement retrieval library or a rule fitted to resolve these cases.
"""
import base64
from fractions import Fraction
import gzip
import hashlib
import json
import math
from pathlib import Path
import re
import sys

HERE = Path(__file__).resolve().parent
ROOT = HERE.parents[2]
SOURCE = "23ba27a7fdcd829f3407c3f3cc328a0737fa9afb"
EVALUATED_SOURCE = "3676aa17e05cf7c813a2000068bc413e99738eb4"
CASE_IDS = ("inbox-menu-14", "inbox-menu-28", "inbox-menu-29", "inbox-menu-30")
INBOX = "zati/inbox-briefing"
SEND = "synthetic/send-mail"
SOURCE_PATHS = ("internal/cataloglexical/lexical.go", "catalog/discovery/index.go", "catalog/preparation/preparation.go")
INPUT = "tools/evaluation/inbox-development/development.inputs.jsonl"
CATALOG = "tools/evaluation/inbox-development/catalog.json"
OLD = "tools/evaluation/inbox-development-results/"
VECTOR_PATHS = tuple(OLD + f"vectors-{dim}.json.gz" for dim in (256, 768))
ATTEMPT_PATHS = tuple(OLD + f"{dim}-k{k}-{kind}.jsonl.gz" for dim in (256, 768)
                      for k in (4, 8, 16) for kind in ("source_cold", "source_warm"))
ALLOWED_PATHS = SOURCE_PATHS + (INPUT, CATALOG) + VECTOR_PATHS + ATTEMPT_PATHS
STOPWORDS = set("a an and are as at be by for from in into is it its of on or that the these this those through to via with without".split())
SUFFIXES = (("sses", "ss"), ("ies", "y"), ("ational", "ate"), ("ation", "ate"),
            ("ing", ""), ("ed", ""), ("er", ""), ("s", ""))


def stem(word):
    if len(word) <= 3:
        return word
    for suffix, replacement in SUFFIXES:
        if not word.endswith(suffix):
            continue
        if suffix == "s" and word.endswith(("ss", "us", "is")):
            break
        base = word[:-len(suffix)] + replacement
        if len(base) >= 3:
            word = base
        break
    if len(word) > 3 and word.endswith("e"):
        word = word[:-1]
    return word


def terms(text):
    # Frozen cases use letters/digits; underscore is a separator in Go's source.
    return [stem(word) for word in re.findall(r"[^\W_]+", text.lower()) if word not in STOPWORDS]


def project(record):
    metadata = json.loads(base64.b64decode(record["Metadata"], validate=True))
    caps = [value["id"] if isinstance(value, dict) else value.rsplit("@", 1)[0]
            for value in metadata.get("required_capabilities", [])]
    capability = metadata.get("capability_ref")
    if capability:
        caps.append(capability.rsplit("@", 1)[0])
    fields = (record["Ref"]["ID"] + " " + metadata.get("logical_name", ""),
              " ".join([record["Ref"]["Kind"]] + caps + metadata.get("tags", []) + metadata.get("taxonomy", [])),
              metadata.get("summary", "") + " " + metadata.get("description", ""))
    return {"embedding_text": metadata.get("logical_name", "") + " " + metadata.get("summary", ""),
            "fields": [set(terms(text)) for text in fields]}


def lexical_score(intent, document):
    hits = [next((3 - index for index, field in enumerate(document["fields"]) if term in field), 0)
            for term in dict.fromkeys(terms(intent))]
    return sum(bool(hit) for hit in hits), sum(hits)


def cosine(left, right):
    if len(left) != len(right) or not all(math.isfinite(v) for v in left + right):
        raise ValueError("invalid retained vector")
    return sum(a*b for a, b in zip(left, right, strict=True)) / math.sqrt(sum(a*a for a in left)*sum(b*b for b in right))


def fused_score(identifier, lexical_ranks, semantic_ranks):
    return ((Fraction(1, 60 + lexical_ranks[identifier]) if identifier in lexical_ranks else Fraction(0))
            + (Fraction(1, 60 + semantic_ranks[identifier]) if identifier in semantic_ranks else Fraction(0)))


def verified_inputs(root=ROOT):
    manifest = json.loads((HERE / "frozen-hashes.json").read_text())
    if set(manifest["files"]) != set(ALLOWED_PATHS):
        raise ValueError("diagnostic manifest must contain only the fixed public allowlist")
    result = {}
    for relative in ALLOWED_PATHS:
        path = root / relative
        if not path.is_file() or path.is_symlink() or path.stat().st_size > 8 << 20:
            raise ValueError("missing/nonregular/oversized frozen diagnostic input")
        raw = path.read_bytes()
        if hashlib.sha256(raw).hexdigest() != manifest["files"][relative]:
            raise ValueError("frozen fingerprint mismatch: " + relative)
        result[relative] = raw
    return result, manifest


def diagnose(root=ROOT):
    artifacts, manifest = verified_inputs(root)
    cases = {row["id"]: row for row in map(json.loads, artifacts[INPUT].decode().splitlines())}
    if len(cases) != 30:
        raise ValueError("changed development cohort identity")
    docs = {entry["record"]["Ref"]["ID"]: project(entry["record"])
            for entry in json.loads(artifacts[CATALOG])["records"] if entry["record"]["Ref"]["Kind"] == "skill"}
    outputs = []
    for dim in (256, 768):
        vectors = json.loads(gzip.decompress(artifacts[OLD + f"vectors-{dim}.json.gz"]))
        lookup = {(entry["purpose"], entry["text"]): entry["vector"] for entry in vectors["entries"]}
        for identifier in CASE_IDS:
            case = cases[identifier]
            lexical = [(name, lexical_score(case["intent"], doc)) for name, doc in docs.items()]
            lexical = sorted((item for item in lexical if item[1][0]), key=lambda item: (-item[1][0], -item[1][1], item[0]))
            query = lookup[("query", case["intent"])]
            semantic = [(name, cosine(query, lookup[("document", doc["embedding_text"])])) for name, doc in docs.items()]
            semantic = sorted((item for item in semantic if item[1] >= .30), key=lambda item: (-item[1], item[0]))
            lr = {name: i+1 for i, (name, _) in enumerate(lexical)}
            sr = {name: i+1 for i, (name, _) in enumerate(semantic)}
            if (lr.get(SEND), lr.get(INBOX), sr.get(INBOX), sr.get(SEND)) != (1, 2, 1, 2):
                raise ValueError("frozen reversed-rank explanation no longer holds")
            scores = {name: fused_score(name, lr, sr) for name in set(lr) | set(sr)}
            if scores[INBOX] != scores[SEND] or scores[INBOX] != Fraction(123, 3782):
                raise ValueError("expected exact RRF tie absent")
            if any(score > scores[INBOX] for score in scores.values()):
                raise ValueError("tied records are not the top candidates")
            outputs.append({"id": identifier, "intent": case["intent"], "dimension": dim,
                            "lexical": [{"id": name, "terms": score[0], "weight": score[1], "rank": lr[name]} for name, score in lexical],
                            "semantic": [{"id": name, "cosine": score, "rank": sr[name]} for name, score in semantic],
                            "tied_top_ids": sorted((INBOX, SEND)), "exact_rrf_score": str(scores[INBOX]),
                            "floating_rrf_score": float(scores[INBOX]), "decision": "requires_selection"})
    observed = []
    for relative in ATTEMPT_PATHS:
        rows = list(map(json.loads, gzip.decompress(artifacts[relative]).decode().splitlines()))
        if len(rows) != 210 or any(row["source"] != EVALUATED_SOURCE for row in rows):
            raise ValueError("changed frozen attempt source/count")
        for arm in ("H0", "H1"):
            abstained = sorted(row["id"] for row in rows if row["arm"] == arm and row["status"] == "requires_selection")
            if abstained != list(CASE_IDS):
                raise ValueError("retained four-case ambiguity differs")
        observed.append({"artifact": relative, "records": len(rows), "H0_requires_selection": list(CASE_IDS), "H1_requires_selection": list(CASE_IDS)})
    return {"schema": "gist.inbox-rrf-diagnosis/v1", "inspected_source": SOURCE, "evaluated_source": EVALUATED_SOURCE,
            "frozen_fingerprints": manifest["files"], "semantic_floor": .30, "rrf_constant": 60,
            "unique_cases": 4, "construction_families": 1, "rank_evidence": outputs, "retained_configuration_evidence": observed,
            "retained_attempt_records_verified": 2520, "candidate_selected": False,
            "conclusion": "Exact reversed-rank RRF ties; generic abstention is intentional, not a native/tokenizer/connection defect.",
            "product_route": "Reuse Zati's existing explicitly selected immutable skill path; it is a separate caller-selected route, not an improved H1 result.",
            "model_inference_calls": 0, "provider_calls": 0, "labels_read": False, "heldout_accessed": False,
            "defaults_or_adoption_changed": False}


if __name__ == "__main__":
    json.dump(diagnose(), sys.stdout, indent=2, sort_keys=True, allow_nan=False)
    print()

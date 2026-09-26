#!/usr/bin/env python3
"""Small, dependency-free checks used by the registry milestone gates.

The checker validates evidence shape and relationships. It deliberately does
not turn a missing service, credential, or fixture into a passing result.
"""

from __future__ import annotations

import argparse
import hashlib
import json
import re
import sys
from pathlib import Path
from typing import Any


ROOT = Path(__file__).resolve().parents[2]
REQUIRED_ERRORS = {
    "unauthorized",
    "forbidden",
    "not_found",
    "version_conflict",
    "budget_exceeded",
    "validation_failed",
    "rate_limited",
    "service_unavailable",
}
HEX64 = re.compile(r"^[0-9a-f]{64}$")


class CheckError(ValueError):
    pass


def fail(message: str) -> None:
    raise CheckError(message)


def read_json(path: Path) -> dict[str, Any]:
    if not path.is_file():
        fail(f"missing evidence or contract file: {path}")
    if path.stat().st_size == 0:
        fail(f"empty evidence or contract file: {path}")
    try:
        value = json.loads(path.read_text(encoding="utf-8"))
    except (OSError, json.JSONDecodeError) as exc:
        fail(f"invalid JSON in {path}: {exc}")
    if not isinstance(value, dict):
        fail(f"top-level JSON value must be an object: {path}")
    return value


def nonempty(value: Any, label: str) -> None:
    if value is None or value == "" or value == [] or value == {}:
        fail(f"{label} must be non-empty")


def file_hash(path: Path) -> str:
    return hashlib.sha256(path.read_bytes()).hexdigest()


def check_contracts(freeze_check: bool) -> None:
    directory = ROOT / "contracts" / "registry" / "v1"
    if not directory.is_dir():
        fail(f"missing contract directory: {directory}")

    json_files = sorted(directory.glob("*.json"))
    if not json_files:
        fail("contract inventory contains no JSON files")
    documents = {path.name: read_json(path) for path in json_files}
    schemas = [name for name in documents if name.endswith(".schema.json")]
    if schemas:
        for name in schemas:
            schema = documents[name]
            if schema.get("$schema") != "https://json-schema.org/draft/2020-12/schema":
                fail(f"{name} is not a JSON Schema 2020-12 document")
            if "$id" not in schema:
                fail(f"{name} has no stable $id")

    openapi = directory / "openapi.yaml"
    if openapi.exists():
        if not openapi.is_file() or openapi.stat().st_size == 0:
            fail("openapi.yaml is empty")
        text = openapi.read_text(encoding="utf-8")
        if not re.search(r"(?m)^openapi:\s*3", text) or not re.search(r"(?m)^paths:\s*$", text):
            fail("openapi.yaml must declare an OpenAPI 3 document and paths")

    route_path = directory / "route-matrix.json"
    if route_path.is_file():
        route_doc = read_json(route_path)
        routes = route_doc.get("routes", route_doc.get("operations"))
        if not isinstance(routes, list) or not routes:
            fail("route-matrix must contain a non-empty routes or operations array")
        methods_paths: set[tuple[str, str]] = set()
        for index, route in enumerate(routes):
            if not isinstance(route, dict):
                fail(f"route {index} is not an object")
            method = route.get("method")
            path = route.get("path")
            if not isinstance(method, str) or not isinstance(path, str):
                fail(f"route {index} lacks method/path")
            if not path.startswith("/v1/") or "/execut" in path.lower():
                fail(f"route {index} is outside the registry contract: {method} {path}")
            key = (method.upper(), path)
            if key in methods_paths:
                fail(f"duplicate route: {method} {path}")
            methods_paths.add(key)
            if not route.get("examples") and not route.get("example"):
                fail(f"route {method} {path} has no wire example")
        errors = route_doc.get("errors")
        if isinstance(errors, list):
            codes = {item.get("code") for item in errors if isinstance(item, dict)}
            missing = REQUIRED_ERRORS - codes
            if missing:
                fail("route-matrix is missing baseline error codes: " + ", ".join(sorted(missing)))

    for name, document in documents.items():
        for reference in _references(document):
            if reference.startswith("#") or "://" in reference:
                continue
            target = (directory / reference).resolve()
            if directory not in target.parents and target != directory:
                fail(f"reference escapes contract directory: {name} -> {reference}")
            if not target.is_file():
                fail(f"broken contract reference: {name} -> {reference}")

    if freeze_check:
        lock = directory / "lock.json"
        lock_doc = read_json(lock)
        entries = lock_doc.get("files")
        if not isinstance(entries, list) or not entries:
            fail("contract lock must contain non-empty files")
        for entry in entries:
            if not isinstance(entry, dict) or not isinstance(entry.get("path"), str):
                fail("contract lock entries require path and sha256")
            path = ROOT / entry["path"]
            if not path.is_file() or entry.get("sha256") != file_hash(path):
                fail(f"stale or missing contract lock entry: {entry.get('path')}")


def _references(value: Any):
    if isinstance(value, dict):
        for key, item in value.items():
            if key in {"$ref", "schema", "schema_file", "fixture"} and isinstance(item, str):
                yield item
            yield from _references(item)
    elif isinstance(value, list):
        for item in value:
            yield from _references(item)


def check_receipt(path: Path) -> None:
    document = read_json(path)
    if document.get("status") not in {"passed", "accepted", "verified"}:
        fail("receipt status must be passed, accepted, or verified")
    for field in ("revision", "interface", "evidence"):
        nonempty(document.get(field), f"receipt.{field}")
    if document.get("skipped") is True or document.get("tests_skipped", 0):
        fail("receipt cannot contain skipped required checks")


def check_eval(path: Path) -> None:
    document = read_json(path)
    labels = document.get("labels")
    results = document.get("results")
    if not isinstance(labels, list) or not labels:
        fail("evaluation labels must be non-empty")
    if not isinstance(results, list) or not results:
        fail("evaluation results must be non-empty")
    label_ids = {item.get("id") for item in labels if isinstance(item, dict)}
    result_ids = {item.get("id") for item in results if isinstance(item, dict)}
    if None in label_ids or None in result_ids or label_ids != result_ids:
        fail("evaluation labels and results do not have identical case coverage")
    if any(item.get("status") in {"skipped", "pending", ""} for item in results):
        fail("evaluation contains skipped or incomplete results")


def check_artifact(path: Path, label: str) -> None:
    document = read_json(path)
    if document.get("status") not in {"passed", "accepted", "verified"}:
        fail(f"{label} status is not passing")
    for field in ("revision", "evidence"):
        nonempty(document.get(field), f"{label}.{field}")


def check_evidence(milestone: str) -> None:
    path = ROOT / "docs" / "registry" / "gates" / f"{milestone}.json"
    document = read_json(path)
    validate_evidence_document(document, milestone)


def validate_evidence_document(document: dict[str, Any], milestone: str) -> None:
    if document.get("status") != "passed":
        fail(f"{milestone} evidence is not passed")
    for field in ("task_ids", "commands", "evidence", "source_hash", "contract_hash", "config_hash"):
        nonempty(document.get(field), f"{milestone}.{field}")
    if not isinstance(document["task_ids"], list) or not isinstance(document["commands"], list):
        fail(f"{milestone} task_ids and commands must be arrays")
    tests = document.get("tests")
    if not isinstance(tests, dict) or not tests or sum(v for v in tests.values() if isinstance(v, int)) <= 0:
        fail(f"{milestone} must record a positive test count")
    if document.get("skipped") or document.get("stale"):
        fail(f"{milestone} evidence is skipped or stale")
    for field in ("source_hash", "contract_hash", "config_hash"):
        if not HEX64.fullmatch(str(document[field])):
            fail(f"{milestone}.{field} must be a SHA-256 hex digest")
    hashes = document.get("hashes", {})
    if hashes:
        if not isinstance(hashes, dict):
            fail(f"{milestone}.hashes must be an object")
        for relative_path, expected in hashes.items():
            artifact = ROOT / relative_path
            if not artifact.is_file() or expected != file_hash(artifact):
                fail(f"{milestone} has stale artifact hash: {relative_path}")


def main(argv: list[str] | None = None) -> int:
    parser = argparse.ArgumentParser(description=__doc__)
    subparsers = parser.add_subparsers(dest="command", required=True)
    contracts = subparsers.add_parser("contracts")
    contracts.add_argument("--freeze-check", action="store_true")
    receipt = subparsers.add_parser("receipt")
    receipt.add_argument("path", type=Path)
    evaluation = subparsers.add_parser("eval")
    evaluation.add_argument("path", type=Path)
    for command in ("iac", "release"):
        artifact = subparsers.add_parser(command)
        artifact.add_argument("path", type=Path)
    evidence = subparsers.add_parser("evidence")
    evidence.add_argument("--milestone", required=True)
    args = parser.parse_args(argv)
    try:
        if args.command == "contracts":
            check_contracts(args.freeze_check)
        elif args.command == "receipt":
            check_receipt(args.path)
        elif args.command == "eval":
            check_eval(args.path)
        elif args.command in {"iac", "release"}:
            check_artifact(args.path, args.command)
        else:
            check_evidence(args.milestone)
    except CheckError as exc:
        print(f"registry check: FAIL: {exc}", file=sys.stderr)
        return 1
    print(f"registry check: PASS: {args.command}")
    return 0


if __name__ == "__main__":
    raise SystemExit(main())

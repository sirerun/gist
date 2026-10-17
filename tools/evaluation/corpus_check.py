#!/usr/bin/env python3
"""Check a synthetic development split without opening any held-out artifact."""
import argparse
from collections import Counter
import hashlib
import json
from pathlib import Path


def records(path):
    return [json.loads(line) for line in Path(path).read_text(encoding="utf-8").splitlines() if line.strip()]


def validate(inputs, labels):
    if len(inputs) != 60 or len(labels) != 60:
        raise ValueError("pilot development split requires exactly60 inputs and60 labels")
    if len({record["id"] for record in inputs}) != 60:
        raise ValueError("development IDs must be unique")
    if len({record["workflow_family"] for record in inputs}) != 60:
        raise ValueError("development workflow identities must be unique")
    family_groups = Counter(record["family"] for record in inputs)
    if any(not family for family in family_groups):
        raise ValueError("construction family grouping must be explicit")
    if any(record.get("split") != "development" for record in inputs):
        raise ValueError("implementation checker refuses non-development inputs")
    if any(not isinstance(record.get("intent"), str) or not record["intent"].strip() for record in inputs):
        raise ValueError("missing independent intent")
    strata = Counter(record["stratum"] for record in inputs)
    if len(strata) != 6 or set(strata.values()) != {10}:
        raise ValueError("six primary development strata require10 cases each")
    input_ids = {record["id"] for record in inputs}
    label_ids = [record["id"] for record in labels]
    if len(set(label_ids)) != 60 or set(label_ids) != input_ids:
        raise ValueError("input/label identity mismatch")
    required = ("allowed_skill_versions", "required_capabilities", "eligible_tool_alternatives",
                "dependencies", "expected_status", "prohibited_actions", "prohibited_disclosures", "output_rubric")
    if any(any(field not in record for field in required) for record in labels):
        raise ValueError("incomplete labels")
    families_by_id = {record["id"]: record["family"] for record in inputs}
    if any(record.get("family") != families_by_id[record["id"]] for record in labels):
        raise ValueError("labels must preserve actual construction-family grouping")
    return {"cases": 60, "families": len(family_groups), "workflow_identities": 60,
            "family_case_counts": dict(sorted(family_groups.items())),
            "strata": dict(sorted(strata.items())), "heldout_opened": False}


def main():
    parser = argparse.ArgumentParser(description=__doc__)
    parser.add_argument("--inputs", required=True)
    parser.add_argument("--labels", required=True)
    args = parser.parse_args()
    result = validate(records(args.inputs), records(args.labels))
    result["sha256"] = {"inputs": hashlib.sha256(Path(args.inputs).read_bytes()).hexdigest(),
                        "labels": hashlib.sha256(Path(args.labels).read_bytes()).hexdigest()}
    print(json.dumps(result, sort_keys=True))


if __name__ == "__main__":
    main()

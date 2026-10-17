#!/usr/bin/env python3
"""Deterministically aggregate every source attempt; never infer billed usage."""
import argparse
from collections import Counter, defaultdict
import json
import math
from pathlib import Path


def binomial_tail(n, k, p):
    if p == 0:
        return float(k == 0)
    if p == 1:
        return 1.0
    return min(1.0, sum(math.exp(math.lgamma(n+1)-math.lgamma(j+1)-math.lgamma(n-j+1)
                                + j*math.log(p)+(n-j)*math.log1p(-p)) for j in range(k, n+1)))


def cp_bound(k, n, alpha, lower):
    if not 0 <= k <= n or n <= 0 or not 0 < alpha < 1:
        raise ValueError("invalid binomial bound inputs")
    if lower and k == 0:
        return 0.0
    if not lower and k == n:
        return 1.0
    lo, hi = 0.0, 1.0
    for _ in range(80):
        mid = (lo + hi) / 2
        # Lower solves P(X>=k)=alpha; upper solves P(X>k)=1-alpha.
        target = alpha if lower else 1-alpha
        value = binomial_tail(n, k if lower else k+1, mid)
        if value < target:
            lo = mid
        else:
            hi = mid
    return (lo + hi) / 2


def family_interval(paired, alpha=0.05):
    """Each independent family holds all its cases x paired repeated outcomes.

    Each pair is (candidate_success, baseline_success), not row-level independent
    observations. Four CP bounds use alpha/4; no zero-discordance degeneracy.
    """
    if not paired or any(not values for values in paired.values()):
        raise ValueError("nonempty independent construction-family groups required")
    all_wins = any_wins = all_losses = any_losses = 0
    for values in paired.values():
        wins = [bool(a) and not bool(b) for a, b in values]
        losses = [bool(b) and not bool(a) for a, b in values]
        all_wins += all(wins)
        any_wins += any(wins)
        all_losses += all(losses)
        any_losses += any(losses)
    n = len(paired)
    lower = cp_bound(all_wins, n, alpha/4, True) - cp_bound(any_losses, n, alpha/4, False)
    upper = cp_bound(any_wins, n, alpha/4, False) - cp_bound(all_losses, n, alpha/4, True)
    return {"families": n, "lower": lower, "upper": upper, "noninferior_2pp": lower > -0.02,
            "method": "family all/any discordance Clopper-Pearson four-bound Bonferroni"}


def aggregate(rows):
    arms = defaultdict(list)
    keys = set()
    for row in rows:
        key = (row["id"], row["arm"], row["repeat"], row.get("attempt_index", 1))
        if key in keys:
            raise ValueError("duplicate attempt identity; retries need distinct explicit attempt IDs")
        keys.add(key)
        if not isinstance(row["elapsed_seconds"], (int, float)) or row["elapsed_seconds"] < 0:
            raise ValueError("invalid measured elapsed time")
        arms[row["arm"]].append(row)
    result = {"schema": "gist.preparation.source-report/v1", "attempts": len(rows),
              "kind": "controlled source smoke; not whole-task or held-out retrieval evidence",
              "performance_or_adoption_claim": False, "arms": {}}
    for arm, items in sorted(arms.items()):
        elapsed = sorted(row["elapsed_seconds"] for row in items)
        usage = {}
        for category in ("main_model_tokens", "cached_tokens", "embedding_tokens", "dollars"):
            values = [row["usage"].get(category) for row in items]
            usage[category] = sum(values) if all(isinstance(value, (int, float)) for value in values) else None
        result["arms"][arm] = {"attempts": len(items), "statuses": dict(sorted(Counter(row["status"] for row in items).items())),
                               "all_attempt_elapsed_seconds": sum(elapsed),
                               "source_lookup_p50_seconds": elapsed[(len(elapsed)-1)//2],
                               "source_lookup_p95_seconds": elapsed[math.ceil(0.95*len(elapsed))-1],
                               "actual_families": len({row["family"] for row in items}),
                               "strata": dict(sorted(Counter(row["stratum"] for row in items).items())),
                               "usage": usage, "inference_latency_included": False}
    return result


def grade(rows, labels):
    """Gold labels are consumed only after all candidate preparation has run."""
    gold = {label["id"]: label for label in labels}
    if len(gold) != len(labels):
        raise ValueError("duplicate gold identity")
    groups = defaultdict(lambda: Counter())
    for row in rows:
        if row["id"] not in gold:
            raise ValueError("attempt lacks frozen label")
        label = gold[row["id"]]
        if row["family"] != label["family"]:
            raise ValueError("construction-family grouping changed")
        key = (row["arm"], row["stratum"], label.get("evidence_class", "unspecified"))
        counts = groups[key]
        counts["attempts"] += 1
        if row["status"] == "NOT_RUN":
            counts["not_run"] += 1
            continue
        counts["evaluated"] += 1
        positive = label["expected_status"] == "ready"
        counts["positive_attempts" if positive else "negative_attempts"] += 1
        actual_ready = row["status"] == "ready"
        required = set(label["required_capabilities"])
        actual = set(row.get("capabilities", []))
        alternatives = label["eligible_tool_alternatives"]
        tool_coverage = any(set(option) <= set(row.get("tools", [])) for option in alternatives)
        complete = (actual_ready and row.get("complete_context") is True
                    and row.get("selected") in label["allowed_skill_versions"]
                    and required <= actual and tool_coverage)
        if positive:
            counts["required_capability_instances"] += len(required)
            counts["retrieved_capability_instances"] += len(required & actual)
            counts["complete_toolset_covered"] += bool(complete)
        else:
            counts["correct_abstentions"] += row["status"] not in ("ready", "ERROR")
        counts["exact_readiness_classification"] += row["status"] == label["expected_status"]
    return [{"arm": key[0], "stratum": key[1], "evidence_class": key[2], **dict(sorted(value.items()))}
            for key, value in sorted(groups.items())]


def main():
    parser = argparse.ArgumentParser(description=__doc__)
    parser.add_argument("--attempts", required=True)
    parser.add_argument("--labels", help="optional frozen labels, read only after attempt generation")
    parser.add_argument("--phase", choices=("development", "heldout"), default="development")
    parser.add_argument("--admission", help="frozen manifest required for held-out label access")
    args = parser.parse_args()
    admission = None
    if args.phase == "heldout":
        if not args.admission:
            raise ValueError("held-out labels refused before frozen admission")
        admission = json.loads(Path(args.admission).read_text())
        if admission.get("candidate_frozen") is not True or not admission.get("development_selection_sha256"):
            raise ValueError("candidate/development selection not frozen")
    rows = [json.loads(line) for line in Path(args.attempts).read_text().splitlines() if line.strip()]
    result = aggregate(rows)
    if any(row.get("phase") != args.phase for row in rows):
        raise ValueError("attempt phase differs from report phase")
    if admission and any(row["source"] != admission["candidate_source"] or row["inputs_sha256"] != admission["inputs_sha256"] for row in rows):
        raise ValueError("held-out attempts differ from frozen source/dataset")
    if args.labels:
        import hashlib
        raw = Path(args.labels).read_bytes()
        if admission and hashlib.sha256(raw).hexdigest() != admission["labels_sha256"]:
            raise ValueError("held-out label hash differs from frozen manifest")
        labels = [json.loads(line) for line in raw.decode().splitlines() if line.strip()]
        result["source_readiness_grading"] = grade(rows, labels)
    print(json.dumps(result, indent=2, sort_keys=True, allow_nan=False))


if __name__ == "__main__":
    main()

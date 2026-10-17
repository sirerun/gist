#!/usr/bin/env python3
"""Deterministically aggregate every source attempt; never infer billed usage."""
import argparse
from collections import Counter, defaultdict
import json
import math
from pathlib import Path


def binomial_tail(n, k, p):
    return sum(math.comb(n, j) * p**j * (1-p)**(n-j) for j in range(k, n+1))


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
        key = (row["id"], row["arm"], row["repeat"])
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


def main():
    parser = argparse.ArgumentParser(description=__doc__)
    parser.add_argument("--attempts", required=True)
    args = parser.parse_args()
    rows = [json.loads(line) for line in Path(args.attempts).read_text().splitlines() if line.strip()]
    print(json.dumps(aggregate(rows), indent=2, sort_keys=True, allow_nan=False))


if __name__ == "__main__":
    main()

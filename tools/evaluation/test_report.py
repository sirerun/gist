import unittest
from report import aggregate, cp_bound, family_interval


class ReportTests(unittest.TestCase):
    def test_boundary_valid_zero_discordance(self):
        interval = family_interval({str(i): [(True, True)] * 3 for i in range(60)})
        self.assertLess(interval["lower"], -0.07)
        self.assertGreater(interval["upper"], 0.07)
        self.assertFalse(interval["noninferior_2pp"])
        self.assertAlmostEqual(cp_bound(0, 60, 0.0125, False), 1-0.0125**(1/60))

    def test_repeats_and_case_rows_do_not_inflate_families(self):
        paired = {"construction-a": [(True, False)] * 12, "construction-b": [(False, True)] * 3}
        interval = family_interval(paired)
        self.assertEqual(interval["families"], 2)
        self.assertFalse(interval["noninferior_2pp"])

    def test_failed_and_cold_attempt_costs_remain_in_all_attempt_totals(self):
        def row(identifier, status, seconds):
            return {"id": identifier, "arm": "L0", "repeat": 1, "family": "shared-construction",
                    "stratum": "test", "status": status, "elapsed_seconds": seconds,
                    "usage": {"main_model_tokens": 10, "cached_tokens": 0, "embedding_tokens": None, "dollars": 0.2}}
        output = aggregate([row("cold-failure", "ERROR", 2), row("recovery", "ready", 1)])
        arm = output["arms"]["L0"]
        self.assertEqual(arm["all_attempt_elapsed_seconds"], 3)
        self.assertEqual(arm["usage"]["main_model_tokens"], 20)
        self.assertAlmostEqual(arm["usage"]["dollars"], 0.4)
        self.assertIsNone(arm["usage"]["embedding_tokens"])
        self.assertEqual(arm["actual_families"], 1)
        self.assertFalse(output["performance_or_adoption_claim"])

    def test_duplicate_attempts_fail_instead_of_hiding_retries(self):
        row = {"id": "one", "arm": "L0", "repeat": 1, "family": "family", "stratum": "test",
               "status": "ready", "elapsed_seconds": 0, "usage": {}}
        with self.assertRaises(ValueError):
            aggregate([row, row])


if __name__ == "__main__":
    unittest.main()

class FrozenLabelGradingTests(unittest.TestCase):
    def test_labels_only_grade_actual_closure_never_construct_it(self):
        from report import grade
        label = {"id": "one", "family": "construction", "expected_status": "ready",
                 "allowed_skill_versions": ["skill@1"], "required_capabilities": ["read@1"],
                 "eligible_tool_alternatives": [["list@1", "read@1"]], "evidence_class": "synthetic"}
        row = {"id": "one", "family": "construction", "arm": "H1", "stratum": "test",
               "status": "ready", "selected": "skill@1", "complete_context": True,
               "capabilities": ["read@1"], "tools": ["list@1"]}
        result = grade([row], [label])[0]
        self.assertEqual(result["complete_toolset_covered"], 0)
        row["tools"].append("read@1")
        self.assertEqual(grade([row], [label])[0]["complete_toolset_covered"], 1)

class ComparisonEnvironmentTests(unittest.TestCase):
    def test_fixture_mismatch_is_unavailable_not_algorithm_failure(self):
        from report import grade
        label = {"id": "one", "family": "construction", "expected_status": "ready",
                 "allowed_skill_versions": ["skill@1"], "required_capabilities": ["read@1"],
                 "eligible_tool_alternatives": [["list@1", "read@1"]], "evidence_class": "synthetic"}
        row = {"id": "one", "family": "construction", "arm": "H1", "stratum": "test",
               "status": "ready", "selected": "skill@1", "complete_context": True,
               "capabilities": ["read@1"], "tools": ["combined-read@1"],
               "comparison_status": "NOT_RUN", "comparison_reason": "fixture_unrepresentable: different tool set"}
        result = grade([row], [label])[0]
        self.assertEqual(result["environment_catalog_mismatch"], 1)
        self.assertEqual(result["not_run"], 1)
        self.assertNotIn("evaluated", result)

class FrozenAttemptIdentityTests(unittest.TestCase):
    def test_each_frozen_identity_and_semantic_configuration_is_enforced(self):
        from report import validate_frozen_rows
        admission = {"candidate_source": "source", "inputs_sha256": "inputs", "catalog_sha256": "catalog",
                     "catalog_version": "sha256:catalog", "recipe_version": "inbox-v1", "policy_version": "policy-v1", "model_version": "model-v1",
                     "top_k": 4, "semantic_floor": 0.30, "rrf_constant": 60}
        row = {"source": "source", "inputs_sha256": "inputs", "catalog_sha256": "catalog",
               "catalog_version": "sha256:catalog", "recipe_version": "inbox-v1", "policy_version": "policy-v1", "model_version": "model-v1",
               "top_k": 4, "semantic_floor": 0.30, "rrf_constant": 60, "arm": "H1"}
        validate_frozen_rows([row], admission)
        for field in row:
            if field == "arm":
                continue
            with self.subTest(field=field):
                changed = dict(row)
                changed[field] = "wrong"
                with self.assertRaises(ValueError):
                    validate_frozen_rows([changed], admission)

class FrozenPolicyGridTests(unittest.TestCase):
    def test_supported_grid_binds_policy_and_configuration(self):
        from report import validate_frozen_rows
        for k in (4, 8, 16):
            admission = {"candidate_source": "s", "inputs_sha256": "i", "catalog_sha256": "c",
                         "catalog_version": "v", "recipe_version": "r", "policy_version": "p",
                         "top_k": k, "semantic_floor": .30, "rrf_constant": 60, "model_version": "m"}
            row = dict(admission, source="s", arm="H0")
            validate_frozen_rows([row], admission)
            for field in ("policy_version", "top_k"):
                changed = dict(row, **{field: "wrong"})
                with self.assertRaises(ValueError):
                    validate_frozen_rows([changed], admission)

class ReportBeforeLabelAccessTests(unittest.TestCase):
    def test_mixed_policy_or_config_rejected_before_label_read(self):
        import subprocess
        import sys
        import tempfile
        import json
        from pathlib import Path
        for field in ("policy_version", "top_k"):
            with self.subTest(field=field), tempfile.TemporaryDirectory() as directory:
                root = Path(directory)
                row = {"id": "a", "arm": "L0", "repeat": 1, "phase": "development",
                       "source": "s", "inputs_sha256": "i", "catalog_sha256": "c",
                       "catalog_version": "v", "recipe_version": "r", "policy_version": "p",
                       "top_k": 4, "semantic_floor": .30, "rrf_constant": 60}
                changed = dict(row, id="b", **{field: 8 if field == "top_k" else "other"})
                attempts = root / "attempts.jsonl"
                attempts.write_text(json.dumps(row) + "\n" + json.dumps(changed) + "\n")
                result = subprocess.run([sys.executable, str(Path(__file__).with_name("report.py")),
                                         "--attempts", str(attempts), "--labels", str(root / "DO_NOT_OPEN")],
                                        capture_output=True, text=True, check=False)
                self.assertNotEqual(result.returncode, 0)
                self.assertIn("mixed or unsupported", result.stderr)
                self.assertNotIn("DO_NOT_OPEN", result.stderr)

class ExecutedSemanticIdentityTests(unittest.TestCase):
    def test_empty_identity_only_allowed_for_explicit_unavailable_attempt(self):
        from report import semantic_model_identity
        for arm in ("E0", "H0", "H1"):
            for model in (None, "", "  "):
                row = {"arm": arm, "status": "ready", "model_version": model}
                with self.subTest(arm=arm, model=model), self.assertRaises(ValueError):
                    semantic_model_identity([row])
                row["status"] = "NOT_RUN"
                with self.assertRaises(ValueError):
                    semantic_model_identity([row])
                row.update(comparison_status="NOT_RUN", error="pinned real local vectors unavailable")
                self.assertIsNone(semantic_model_identity([row]))
        self.assertEqual(semantic_model_identity([{"arm": "H1", "status": "ready", "model_version": "pinned-model"}]), "pinned-model")
        with self.assertRaises(ValueError):
            semantic_model_identity([{"arm": "E0", "status": "ready", "model_version": "first"},
                                     {"arm": "H1", "status": "ready", "model_version": "second"}])

    def test_missing_executed_model_refused_before_labels(self):
        import subprocess
        import sys
        import tempfile
        import json
        from pathlib import Path
        for model in (None, "", " "):
            with tempfile.TemporaryDirectory() as directory:
                root = Path(directory)
                row = {"id": "a", "arm": "H0", "repeat": 1, "phase": "development", "status": "ready",
                       "source": "s", "inputs_sha256": "i", "catalog_sha256": "c", "model_version": model,
                       "catalog_version": "v", "recipe_version": "r", "policy_version": "p",
                       "top_k": 4, "semantic_floor": .30, "rrf_constant": 60}
                path = root / "attempts.jsonl"
                path.write_text(json.dumps(row) + "\n")
                result = subprocess.run([sys.executable, str(Path(__file__).with_name("report.py")),
                                         "--attempts", str(path), "--labels", str(root / "DO_NOT_OPEN")],
                                        capture_output=True, text=True, check=False)
                self.assertNotEqual(result.returncode, 0)
                self.assertIn("requires model/runtime identity", result.stderr)
                self.assertNotIn("DO_NOT_OPEN", result.stderr)

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

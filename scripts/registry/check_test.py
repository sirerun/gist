import contextlib
import io
import json
import tempfile
import unittest
from unittest import mock
from pathlib import Path

import check


class RegistryCheckTests(unittest.TestCase):
    def run_check(self, function, *args):
        with tempfile.TemporaryDirectory() as directory:
            path = Path(directory) / "evidence.json"
            path.write_text(json.dumps(args[0]), encoding="utf-8")
            function(path)

    def test_receipt_rejects_missing_evidence(self):
        with self.assertRaises(check.CheckError):
            self.run_check(check.check_receipt, {"status": "passed", "revision": "r1"})

    def test_receipt_rejects_skipped_required_check(self):
        with self.assertRaises(check.CheckError):
            self.run_check(
                check.check_receipt,
                {"status": "passed", "revision": "r1", "interface": "REST", "evidence": ["x"], "skipped": True},
            )

    def test_evaluation_rejects_incomplete_coverage(self):
        with self.assertRaises(check.CheckError):
            self.run_check(
                check.check_eval,
                {"labels": [{"id": "a"}], "results": [{"id": "a", "status": "skipped"}]},
            )

    def test_evaluation_rejects_missing_result(self):
        with self.assertRaises(check.CheckError):
            self.run_check(
                check.check_eval,
                {"labels": [{"id": "a"}, {"id": "b"}], "results": [{"id": "a", "status": "pass"}]},
            )

    def test_evaluation_accepts_complete_results(self):
        self.run_check(
            check.check_eval,
            {"labels": [{"id": "a"}, {"id": "b"}], "results": [{"id": "a", "status": "pass"}, {"id": "b", "status": "no_match"}]},
        )

    def test_evidence_rejects_stale_hash(self):
        document = {
            "status": "passed",
            "task_ids": ["Q0"],
            "commands": ["test"],
            "evidence": ["report"],
            "source_hash": "0" * 64,
            "contract_hash": "0" * 64,
            "config_hash": "0" * 64,
            "tests": {"unit": 1},
            "hashes": {"missing/artifact": "0" * 64},
        }
        with mock.patch.object(check, "ROOT", Path(tempfile.mkdtemp())):
            with self.assertRaises(check.CheckError):
                check.validate_evidence_document(document, "M1")

    def test_cli_reports_failure(self):
        output = io.StringIO()
        with contextlib.redirect_stderr(output):
            result = check.main(["receipt", "/does/not/exist"])
        self.assertEqual(result, 1)
        self.assertIn("FAIL", output.getvalue())


if __name__ == "__main__":
    unittest.main()

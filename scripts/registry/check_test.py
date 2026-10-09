import hashlib
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

    def test_v2_amendment_preserves_baseline_and_rejects_other_changes(self):
        baseline = "type CatalogRecord struct {\n\tMetadata []byte\n}\n"
        before, after = "\tMetadata []byte\n", "\tDocumentDigest Digest\n\tMetadata []byte\n"
        current = baseline.replace(before, after)
        path = "hosted/internal/ports/catalog.go"
        entry = {"path": path, "sha256": hashlib.sha256(baseline.encode()).hexdigest()}
        amendment = {"path": path, "baseline_sha256": entry["sha256"], "sha256": hashlib.sha256(current.encode()).hexdigest(), "baseline_fragment": before, "amended_fragment": after}
        with tempfile.TemporaryDirectory() as directory, mock.patch.object(check, "ROOT", Path(directory)):
            source = Path(directory) / path
            source.parent.mkdir(parents=True)
            source.write_text(current)
            check.check_source_amendment(entry, amendment)
            for modified in (current.replace("CatalogRecord", "BrokenRecord"), current.replace("Metadata []byte", "Metadata string")):
                source.write_text(modified)
                # Even repinning the current digest cannot hide baseline drift.
                amendment["sha256"] = hashlib.sha256(modified.encode()).hexdigest()
                with self.assertRaises(check.CheckError):
                    check.check_source_amendment(entry, amendment)

    def test_v2_amendment_cannot_override_frozen_wire_file(self):
        with self.assertRaises(check.CheckError):
            check.check_source_amendment({"path": "contracts/registry/v1/common.schema.json"}, {"path": "contracts/registry/v1/common.schema.json"})

    def test_cli_reports_failure(self):
        output = io.StringIO()
        with contextlib.redirect_stderr(output):
            result = check.main(["receipt", "/does/not/exist"])
        self.assertEqual(result, 1)
        self.assertIn("FAIL", output.getvalue())


if __name__ == "__main__":
    unittest.main()

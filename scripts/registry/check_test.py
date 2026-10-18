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

    def test_library_relocation_preserves_baseline_and_v2_amendment(self):
        root = check.ROOT
        lock = check.read_json(root / "contracts/registry/v1/lock.json")
        entries = {e["path"]: e for e in lock["files"]}
        relocation = check.read_json(root / "contracts/registry/library-relocation.json")
        amendment = check.read_json(root / "contracts/registry/implementation-v2.json")["files"][0]
        for item in relocation["files"]:
            with self.subTest(path=item["path"]):
                change = amendment if item["path"] == amendment["path"] else None
                check.check_library_relocation(entries[item["path"]], item, change)

    def test_library_relocation_rejects_changed_bytes_missing_alias_and_wrong_target(self):
        root = check.ROOT
        original = "hosted/internal/ports/clock.go"
        entry = next(e for e in check.read_json(root / "contracts/registry/v1/lock.json")["files"] if e["path"] == original)
        relocation = {"path": original, "destination": "catalog/ports/clock.go", "baseline_sha256": entry["sha256"]}
        source_bytes = (root / relocation["destination"]).read_bytes()
        shim_text = (root / "hosted/internal/ports/compat.go").read_text()
        with tempfile.TemporaryDirectory() as directory, mock.patch.object(check, "ROOT", Path(directory)):
            source = Path(directory) / relocation["destination"]
            source.parent.mkdir(parents=True)
            source.write_bytes(source_bytes)
            shim = Path(directory) / "hosted/internal/ports/compat.go"
            shim.parent.mkdir(parents=True)
            shim.write_text(shim_text)
            check.check_library_relocation(entry, relocation, None)
            source.write_bytes(source_bytes + b"// drift\n")
            with self.assertRaises(check.CheckError):
                check.check_library_relocation(entry, relocation, None)
            source.write_bytes(source_bytes)
            for bad_shim in (shim_text.replace("type Clock = shared.Clock\n", ""), shim_text.replace("type Clock = shared.Clock", "type Clock = shared.SystemClock")):
                shim.write_text(bad_shim)
                with self.assertRaises(check.CheckError):
                    check.check_library_relocation(entry, relocation, None)
            shim.write_text(shim_text)
            for target in ("../../outside.go", "catalog/ports/policy.go"):
                with self.assertRaises(check.CheckError):
                    check.check_library_relocation(entry, dict(relocation, destination=target), None)
            with self.assertRaises(check.CheckError):
                check.check_library_relocation({"path": "contracts/registry/v1/common.schema.json", "sha256": entry["sha256"]}, relocation, None)

    def test_library_module_only_allows_exact_additive_dependency(self):
        root = check.ROOT
        entry = next(e for e in check.read_json(root / "contracts/registry/v1/lock.json")["files"] if e["path"] == "hosted/go.mod")
        change = check.read_json(root / "contracts/registry/library-relocation.json")["hosted_module"]
        current = (root / "hosted/go.mod").read_bytes()
        with tempfile.TemporaryDirectory() as directory, mock.patch.object(check, "ROOT", Path(directory)):
            path = Path(directory) / "hosted/go.mod"
            path.parent.mkdir(parents=True)
            path.write_bytes(current)
            amendment = Path(directory) / "contracts/registry/hosted-dependencies.json"
            amendment.parent.mkdir(parents=True)
            amendment.write_bytes((root / "contracts/registry/hosted-dependencies.json").read_bytes())
            check.check_library_module(entry, change)
            for bad in (current.replace(b"go 1.26.1", b"go 1.25.0"), current.replace(b"=> ..", b"=> /tmp/evil"), current + b"\n"):
                path.write_bytes(bad)
                with self.assertRaises(check.CheckError):
                    check.check_library_module(entry, change)

    def test_runtime_dependency_amendment_preserves_original_baseline(self):
        root = check.ROOT
        entry = next(e for e in check.read_json(root / "contracts/registry/v1/lock.json")["files"] if e["path"] == "hosted/go.mod")
        change = check.read_json(root / "contracts/registry/library-relocation.json")["hosted_module"]
        document = check.read_json(root / "contracts/registry/hosted-dependencies.json")
        with tempfile.TemporaryDirectory() as directory, mock.patch.object(check, "ROOT", Path(directory)):
            module = Path(directory) / "hosted/go.mod"
            module.parent.mkdir(parents=True)
            module.write_bytes((root / "hosted/go.mod").read_bytes())
            amendment = Path(directory) / "contracts/registry/hosted-dependencies.json"
            amendment.parent.mkdir(parents=True)
            for bad in (dict(document, prior_module=document["prior_module"].replace("go 1.26.1", "go 1.25.0")), dict(document, sha256="0" * 64), dict(document, path="go.mod")):
                amendment.write_text(json.dumps(bad))
                with self.assertRaises(check.CheckError):
                    check.check_library_module(entry, change)

    def test_runtime_checksum_amendment_rejects_drift(self):
        root = check.ROOT
        entry = next(e for e in check.read_json(root / "contracts/registry/v1/lock.json")["files"] if e["path"] == "hosted/go.sum")
        document = check.read_json(root / "contracts/registry/hosted-dependencies.json")
        with tempfile.TemporaryDirectory() as directory, mock.patch.object(check, "ROOT", Path(directory)):
            sums = Path(directory) / "hosted/go.sum"
            sums.parent.mkdir(parents=True)
            sums.write_bytes((root / "hosted/go.sum").read_bytes())
            amendment = Path(directory) / "contracts/registry/hosted-dependencies.json"
            amendment.parent.mkdir(parents=True)
            amendment.write_text(json.dumps(document))
            check.check_runtime_dependency_sum(entry)
            sums.write_bytes(sums.read_bytes() + b"drift")
            with self.assertRaises(check.CheckError):
                check.check_runtime_dependency_sum(entry)
            with self.assertRaises(check.CheckError):
                check.check_runtime_dependency_sum(dict(entry, sha256="0" * 64))

    def test_cli_reports_failure(self):
        output = io.StringIO()
        with contextlib.redirect_stderr(output):
            result = check.main(["receipt", "/does/not/exist"])
        self.assertEqual(result, 1)
        self.assertIn("FAIL", output.getvalue())


if __name__ == "__main__":
    unittest.main()

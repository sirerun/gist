import hashlib
import json
from pathlib import Path
import tempfile
import unittest

from embedding_adapter import MODEL, REVISION, verify_model_files


class ModelIdentityTests(unittest.TestCase):
    def setUp(self):
        self.temp = tempfile.TemporaryDirectory()
        self.addCleanup(self.temp.cleanup)
        self.root = Path(self.temp.name)
        self.contents = {"model.safetensors": b"small test-only weight bytes", "config.json": b"{}",
                         "tokenizer.json": b'{"test":"synthetic tokenizer"}',
                         "1_Pooling/config.json": b'{"pooling_mode":"mean"}'}
        for name, content in self.contents.items():
            path = self.root / name
            path.parent.mkdir(parents=True, exist_ok=True)
            path.write_bytes(content)
        self.manifest = {"model": MODEL, "revision": REVISION,
                         "files": {name: hashlib.sha256(data).hexdigest() for name, data in self.contents.items()}}

    def test_exact_trusted_set_passes(self):
        root, files = verify_model_files(self.root, self.manifest)
        self.assertEqual(len(root), 64)
        self.assertEqual(files, self.manifest["files"])

    def test_tampering_rejected_before_any_model_loader(self):
        # This verifier imports no numerical/model runtime and invokes no loader.
        for name in ("config.json", "tokenizer.json", "model.safetensors", "1_Pooling/config.json"):
            with self.subTest(name=name):
                path = self.root / name
                path.write_bytes(b"replaced bytes")
                with self.assertRaisesRegex(ValueError, "bytes differ"):
                    verify_model_files(self.root, self.manifest)
                path.write_bytes(self.contents[name])

    def test_missing_and_extra_weights_rejected(self):
        path = self.root / "model.safetensors"
        path.unlink()
        with self.assertRaisesRegex(ValueError, "file set differs"):
            verify_model_files(self.root, self.manifest)
        path.write_bytes(self.contents["model.safetensors"])
        (self.root / "pytorch_model.bin").write_bytes(b"ambiguous extra weights")
        with self.assertRaisesRegex(ValueError, "file set differs"):
            verify_model_files(self.root, self.manifest)

    def test_local_revision_claim_and_symlink_cannot_establish_trust(self):
        (self.root / "revision.json").write_text(json.dumps({"revision": REVISION}))
        with self.assertRaises(ValueError):
            verify_model_files(self.root, self.manifest)
        (self.root / "revision.json").unlink()
        path = self.root / "config.json"
        path.unlink()
        path.symlink_to(self.root / "tokenizer.json")
        with self.assertRaisesRegex(ValueError, "symlink"):
            verify_model_files(self.root, self.manifest)

    def test_wrong_trusted_identity_rejected(self):
        self.manifest["revision"] = "other"
        with self.assertRaisesRegex(ValueError, "identity mismatch"):
            verify_model_files(self.root, self.manifest)


if __name__ == "__main__":
    unittest.main()

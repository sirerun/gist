"""Offline frozen registry schema and conformance checks for the new fixture."""
import json
import unittest
from pathlib import Path
from jsonschema import Draft202012Validator
from referencing import Registry, Resource

HERE = Path(__file__).resolve().parent
ROOT = HERE.parents[2]
SCHEMAS = ROOT / "contracts/registry/v1"
REGISTRY = Registry().with_resources([
    (s["$id"], Resource.from_contents(s))
    for p in SCHEMAS.glob("*.schema.json")
    for s in [json.loads(p.read_text())]
])


class FrozenCompatibility(unittest.TestCase):
    def test_frozen_capability_and_manifest_schemas(self):
        for name, schema in [("manifest.json", "manifest.schema.json"),
                             ("assets/list-contract.json", "capability.schema.json"),
                             ("assets/read-contract.json", "capability.schema.json")]:
            with self.subTest(name=name):
                validator = Draft202012Validator(json.loads((SCHEMAS / schema).read_text()), registry=REGISTRY)
                validator.validate(json.loads((HERE / name).read_text()))

    def test_declared_conformance_examples(self):
        examples = json.loads((HERE / "assets/conformance.json").read_text())
        for role in ("list", "read"):
            contract = json.loads((HERE / f"assets/{role}-contract.json").read_text())
            for example in examples[role]:
                input_valid = Draft202012Validator(contract["input_schema"]).is_valid(example["input"])
                output_valid = Draft202012Validator(contract["output_schema"]).is_valid(example["output"])
                self.assertEqual(input_valid and output_valid, example["valid"])


if __name__ == "__main__":
    unittest.main()

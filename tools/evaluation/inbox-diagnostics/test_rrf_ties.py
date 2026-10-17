import json
from fractions import Fraction
from pathlib import Path
import tempfile
import unittest
from unittest.mock import patch

import rrf_ties


class FixedRRFDiagnosisTests(unittest.TestCase):
    def test_rank_swap_is_an_exact_tie_not_an_arbitrary_selection(self):
        lexical = {rrf_ties.SEND: 1, rrf_ties.INBOX: 2}
        semantic = {rrf_ties.INBOX: 1, rrf_ties.SEND: 2}
        inbox = rrf_ties.fused_score(rrf_ties.INBOX, lexical, semantic)
        send = rrf_ties.fused_score(rrf_ties.SEND, lexical, semantic)
        self.assertEqual(inbox, Fraction(123, 3782))
        self.assertEqual(inbox, send)

    def test_all_frozen_configurations_explain_only_the_observed_four(self):
        got = rrf_ties.diagnose()
        expected = json.loads((rrf_ties.HERE / "diagnosis.json").read_text())
        self.assertEqual(got, expected)
        self.assertEqual(len(got["rank_evidence"]), 8)
        self.assertEqual(len(got["retained_configuration_evidence"]), 12)
        self.assertEqual(got["retained_attempt_records_verified"], 2520)
        self.assertFalse(got["candidate_selected"])
        self.assertFalse(got["labels_read"])
        self.assertFalse(got["heldout_accessed"])
        for evidence in got["rank_evidence"]:
            self.assertEqual(evidence["decision"], "requires_selection")
            self.assertEqual(evidence["exact_rrf_score"], "123/3782")
            self.assertEqual(evidence["lexical"][0]["id"], rrf_ties.SEND)
            self.assertEqual(evidence["semantic"][0]["id"], rrf_ties.INBOX)

    def test_altered_source_cannot_silently_reinterpret_frozen_results(self):
        with tempfile.TemporaryDirectory() as directory:
            root = Path(directory)
            source = root / rrf_ties.SOURCE_PATHS[0]
            source.parent.mkdir(parents=True)
            source.write_text("changed algorithm\n")
            with self.assertRaisesRegex(ValueError, "fingerprint mismatch"):
                rrf_ties.verified_inputs(root)

    def test_gold_or_other_unlisted_input_is_refused_before_open(self):
        manifest = json.loads((rrf_ties.HERE / "frozen-hashes.json").read_text())
        manifest["files"]["tools/evaluation/corpus/development.labels.jsonl"] = "0"*64
        with tempfile.TemporaryDirectory() as directory:
            here = Path(directory)
            (here / "frozen-hashes.json").write_text(json.dumps(manifest))
            with patch.object(rrf_ties, "HERE", here):
                with self.assertRaisesRegex(ValueError, "fixed public allowlist"):
                    rrf_ties.verified_inputs()

    def test_field_weights_explain_message_and_email_conflict(self):
        name = {"email", "messag"}
        descriptive = {"email", "unread"}
        send = {"fields": [name, set(), set()]}
        inbox = {"fields": [set(), set(), descriptive]}
        intent = "unread email message"
        self.assertEqual(rrf_ties.lexical_score(intent, send), (2, 6))
        self.assertEqual(rrf_ties.lexical_score(intent, inbox), (2, 2))


if __name__ == "__main__":
    unittest.main()

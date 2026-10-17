import copy
import importlib.util
import unittest
from pathlib import Path

spec = importlib.util.spec_from_file_location('composition_report', Path(__file__).with_name('report.py'))
r = importlib.util.module_from_spec(spec)
spec.loader.exec_module(r)


class ReportTest(unittest.TestCase):
    def setUp(self):
        self.label = {'id': 'C01', 'ready': True, 'selected': {'list': {'binding_id': 'binding/list', 'tool_id': 'tool/list', 'connection_id': 'c1', 'account_id': 'a1'}}, 'forbidden_disclosure': ['PRIVATE_SENTINEL']}
        self.observation = {'id': 'C01', 'elapsed_ns': 1, 'response_bytes': 300, 'error': '', 'result': {'Status': 'ready', 'Bundle': {'Selected': {'list': {'Binding': {'Ref': {'ID': 'binding/list'}}, 'Tool': {'Ref': {'ID': 'tool/list'}}, 'Connection': {'ID': 'c1', 'AccountID': 'a1'}}}, 'Tools': [{'Pin': {'Ref': {'ID': 'tool/list'}}, 'Connection': {'ID': 'c1', 'AccountID': 'a1'}}]}}}

    def test_exact_closure(self):
        self.assertTrue(r.grade_cohort([self.label], [self.observation])['pass'])
        changed = copy.deepcopy(self.observation)
        changed['result']['Bundle']['Tools'].append(changed['result']['Bundle']['Tools'][0])
        self.assertFalse(r.grade_cohort([self.label], [changed])['pass'])
        changed = copy.deepcopy(self.observation)
        changed['result']['Bundle']['Selected']['list']['Connection']['AccountID'] = 'a2'
        self.assertFalse(r.grade_cohort([self.label], [changed])['pass'])

    def test_missing_duplicate_false_ready_and_exposure(self):
        self.assertFalse(r.grade_cohort([self.label], [])['pass'])
        with self.assertRaises(ValueError):
            r.grade_cohort([self.label], [self.observation, self.observation])
        label = dict(self.label, ready=False, selected={})
        self.assertEqual(r.grade_cohort([label], [self.observation])['false_ready'], 1)
        observation = dict(self.observation, result={'Status': 'denied'}, error='PRIVATE_SENTINEL')
        self.assertEqual(r.grade_cohort([label], [observation])['private_exposures'], 1)
        observation = dict(self.observation, result={'Status': 'requires_selection', 'Bundle': self.observation['result']['Bundle']})
        self.assertFalse(r.grade_cohort([label], [observation])['pass'])

    def test_percentiles(self):
        self.assertEqual(r.percentile(list(range(1, 101)), .95), 95)
        self.assertEqual(r.percentile(list(range(1, 101)), .5), 50)


if __name__ == '__main__':
    unittest.main()

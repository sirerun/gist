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

    def test_inventory_pins_reject_same_id_drift(self):
        observation = copy.deepcopy(self.observation)
        observation['result']['Bundle']['Skill'] = {'Digest': 'skill'}
        candidate = observation['result']['Bundle']['Selected']['list']
        candidate['Tool']['Digest'] = 'original'
        observation['result']['Bundle']['Tools'][0]['Pin']['Digest'] = 'original'
        inventory = {'cases': [{'input': {'id': 'C01'}, 'request': {'Skill': {'Digest': 'skill'}}, 'admitted_candidates': {'list': [copy.deepcopy(candidate)]}}]}
        self.assertTrue(r.grade_cohort([self.label], [observation], inventory)['pass'])
        candidate['Tool']['Digest'] = 'drift'
        self.assertFalse(r.grade_cohort([self.label], [observation], inventory)['pass'])

    def test_early_and_partial_scale_failure_retained(self):
        import json
        config = json.loads(Path(__file__).with_name('primary-config.json').read_text())
        run = {'logical_records': 10000, 'private_partitions': 10,
               'build_ns': 0, 'update_ns': 0, 'rebuild_ns': 0, 'rollback_ns': 0,
               'peak_rss_bytes': 1024, 'snapshot_projection_bytes': 0,
               'snapshot_projection_candidates': 0, 'query_classes': {}, 'cold': {},
               'checks': {}, 'error': 'context deadline exceeded'}
        result = r.grade_scale(config, [run])
        self.assertFalse(result['pass'])
        self.assertEqual(result['runs'][0]['error'], run['error'])
        self.assertIsNone(result['runs'][0]['queries']['exact_hit']['p95_ms'])
        self.assertEqual(len(result['runs'][0]['missing_query_classes']), 5)
        run['query_classes']['exact_hit'] = [{'elapsed_ns': 10, 'result_bytes': 64, 'status': 'hit', 'candidates': 1, 'error': ''}]
        result = r.grade_scale(config, [run])
        self.assertFalse(result['pass'])
        self.assertEqual(result['runs'][0]['queries']['exact_hit']['samples'], 1)
        self.assertFalse(result['runs'][0]['checks']['exact_hit_sample_count'])

    def test_setup_failure_is_not_qualified_refusal(self):
        label = dict(self.label, ready=False, selected={})
        observation = dict(self.observation, result={}, error='setup: unavailable')
        result = r.grade_cohort([label], [observation])
        self.assertFalse(result['pass'])
        self.assertEqual(result['outcome_coverage'], 0)

    def test_percentiles(self):
        self.assertEqual(r.percentile(list(range(1, 101)), .95), 95)
        self.assertEqual(r.percentile(list(range(1, 101)), .5), 50)


if __name__ == '__main__':
    unittest.main()

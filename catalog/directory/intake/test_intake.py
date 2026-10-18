"""Offline admission mutations and reproducibility; no provider or directory calls."""
import copy
import hashlib
import importlib.util
import json
import pathlib
import subprocess
import sys
import tempfile
import unittest
from unittest import mock

HERE = pathlib.Path(__file__).resolve().parent
spec = importlib.util.spec_from_file_location('intake', HERE / 'build_snapshot.py')
intake = importlib.util.module_from_spec(spec)
spec.loader.exec_module(intake)
TEMP_ROOT = pathlib.Path('/Volumes/BuildOffload/checks/gist-catalog-reset-20261017')


def record(service='example', kind='mcp'):
    result = {'id': kind + ':' + service, 'kind': kind, 'service_id': service,
              'title': 'Example', 'description': 'Operator-reviewed vendor route.',
              'tags': [], 'official': True,
              'official_evidence_url': 'https://vendor.example.com/docs/mcp',
              'auth': {'type': 'oauth2', 'credential_names': ['EXAMPLE_TOKEN']},
              'featured': False, 'recommended': False}
    if kind == 'mcp':
        result['remotes'] = [{'url': 'https://vendor.example.com/mcp', 'transport': 'streamable-http'}]
    else:
        result['openapi'] = {'spec_url': 'https://vendor.example.com/openapi.yaml', 'mint_guide_url': 'https://github.com/sirerun/mint'}
    return result


def membership(*services):
    return {'format': 'gist.directory.admission/v1', 'policy': intake.POLICY,
            'retrieved_at': '2026-10-18T03:00:00Z',
            'apps': [{'service_id': service, 'listings': [{'directory': 'n8n', 'url': 'https://n8n.io/integrations/' + service + '/'}]} for service in services or ('example',)]}


class IntakeTests(unittest.TestCase):
    def reject(self, entries=None, approved=None):
        with self.assertRaises(ValueError):
            intake.build_snapshot([record()] if entries is None else entries,
                                  membership() if approved is None else approved)

    def test_valid_union_membership_and_vendor_route_are_distinct(self):
        approved = membership()
        approved['apps'][0]['listings'].append({'directory': 'make', 'url': 'https://www.make.com/en/integrations/example'})
        original, original_approved = [record()], copy.deepcopy(approved)
        with mock.patch('socket.socket', side_effect=AssertionError('network forbidden')):
            snapshot, receipt = intake.build_snapshot(original, approved)
        self.assertEqual(original, [record()])
        self.assertEqual(approved, original_approved)
        self.assertEqual(snapshot['entries'][0]['source'], {'id': 'gist-curation', 'entry_id': 'mcp:example', 'url': record()['official_evidence_url']})
        self.assertEqual([s['id'] for s in snapshot['sources']], ['gist-curation', 'marketplace-membership'])
        self.assertEqual(receipt['accepted'], 1)
        self.assertEqual(receipt['network_requests'], 0)
        self.assertFalse(snapshot['entries'][0]['featured'])
        self.assertNotIn('verification', snapshot['entries'][0])

    def test_admission_missing_unknown_duplicate_and_wrong_policy_fail(self):
        for change in ('missing', 'unknown', 'duplicate', 'policy', 'format', 'empty-listings', 'duplicate-listings', 'extra-field', 'community'):
            with self.subTest(change=change):
                approved = membership()
                if change == 'missing': approved['apps'] = []
                elif change == 'unknown': approved['apps'].append(membership('unknown')['apps'][0])
                elif change == 'duplicate': approved['apps'].append(copy.deepcopy(approved['apps'][0]))
                elif change == 'policy': approved['policy'] = 'any-registry'
                elif change == 'format': approved['format'] = 'v0'
                elif change == 'empty-listings': approved['apps'][0]['listings'] = []
                elif change == 'duplicate-listings': approved['apps'][0]['listings'] *= 2
                elif change == 'extra-field': approved['token'] = 'not-allowed'
                elif change == 'community': approved['apps'][0]['community'] = True
                self.reject(approved=approved)
        self.reject(entries=[record('unapproved')])
        self.reject(entries=[record(), record(kind='openapi')])
        self.reject(entries=[])

    def test_stable_namespaced_ids_survive_and_duplicate_ids_fail(self):
        value = record('github')
        value['id'] = 'mcp:io.github.github/github-mcp-server'
        snapshot, _ = intake.build_snapshot([value], membership('github'))
        self.assertEqual(snapshot['entries'][0]['id'], value['id'])
        second = record('another')
        second['id'] = value['id']
        self.reject(entries=[value, second], approved=membership('github', 'another'))
        for bad in ['mcp:bad id', 'mcp:https://example.com', 'mcp:bad?secret', 'mcp:']:
            invalid = record()
            invalid['id'] = bad
            self.reject(entries=[invalid])

    def test_only_exact_first_party_listing_authorities_and_app_paths(self):
        for directory, url in [('n8n', 'https://n8n.io/integrations/example/'),
                               ('make', 'https://www.make.com/integrations/example'),
                               ('make', 'https://www.make.com/en/integrations/example/')]:
            self.assertTrue(intake.listing_url(directory, url), url)
        for url in ['https://n8n.io.evil.example/integrations/example/',
                    'https://user:secret@n8n.io/integrations/example/',
                    'https://n8n.io:443/integrations/example/',
                    'https://n8n.io/integrations/',
                    'https://n8n.io/integrations/categories/ai/',
                    'https://n8n.io/integrations/%65xample/',
                    'https://n8n.io/integrations/example/?token=secret',
                    'https://n8n.io/integrations/example/#fragment']:
            with self.subTest(url=url):
                approved = membership()
                approved['apps'][0]['listings'][0]['url'] = url
                self.reject(approved=approved)
        self.assertFalse(intake.listing_url('make', 'https://make.com/en/integrations/example'))
        self.assertFalse(intake.listing_url('other', 'https://www.make.com/en/integrations/example'))

    def test_vendor_official_is_mandatory_and_no_featured_or_community_evidence(self):
        for mutate in [lambda r:r.update(official=False), lambda r:r.pop('official'),
                       lambda r:r.pop('official_evidence_url'),
                       lambda r:r.update(official_evidence_url='https://user:secret@example.com/docs'),
                       lambda r:r.update(community=True), lambda r:r.update(featured=True),
                       lambda r:r.update(featured_selection=True), lambda r:r.update(verification=[]),
                       lambda r:r.update(source={'id':'mcp-official'}),
                       lambda r:r.update(unknown='secret'), lambda r:r.update(id='openapi:another')]:
            value = record()
            mutate(value)
            self.reject(entries=[value])

    def test_endpoint_or_raw_spec_required_not_packages_or_html(self):
        snapshot, _ = intake.build_snapshot([record(kind='openapi')], membership())
        self.assertEqual(snapshot['entries'][0]['kind'], 'openapi')
        for mutate in [lambda r:r.pop('remotes'), lambda r:r.update(remotes=[]),
                       lambda r:r.update(packages=[{'registry':'npm','identifier':'@vendor/server'}]),
                       lambda r:r['remotes'][0].update(transport='stdio'),
                       lambda r:r['remotes'][0].update(headers={'Authorization':'Bearer secret'}),
                       lambda r:r['remotes'][0].update(url='https://vendor.example.com/mcp?key=secret')]:
            value = record()
            mutate(value)
            self.reject(entries=[value])
        for url in ['https://github.com/vendor/api/blob/main/openapi.yaml',
                    'https://vendor.example.com/docs/api',
                    'https://vendor.example.com/swagger-ui',
                    'https://vendor.example.com/openapi.yaml?token=secret']:
            value = record(kind='openapi')
            value['openapi']['spec_url'] = url
            self.reject(entries=[value])

    def test_credential_url_and_descriptor_values_are_rejected(self):
        for url in ['http://example.com/mcp', 'https://localhost/mcp',
                    'https://127.0.0.1/mcp', 'https://192.168.2.3/mcp',
                    'https://machine.internal/mcp', 'https://u:p@example.com/mcp',
                    'https://example.com/mcp?token=value', 'https://{tenant}.gatana.ai/mcp',
                    'https://example.com:notaport/mcp', 'https://bad host.example.com/mcp',
                    'https://example.com/mcp#token', 'https://example.com/mcp?',
                    'https://example.com/mcp#', 'https://example.com/\\secret',
                    'https://example.com/%not-an-escape']:
            self.assertFalse(intake.safe_url(url), url)
        self.assertTrue(intake.safe_url('https://example.com/mcp'))
        for field in ('homepage', 'repository', 'official_evidence_url'):
            value = record()
            value[field] = 'https://example.com/path?secret=value'
            self.reject(entries=[value])
        for mutate in [lambda r:r['auth'].update(token='secret'),
                       lambda r:r['auth'].update(type='Bearer secret'),
                       lambda r:r['auth'].update(credential_names=['EXAMPLE_TOKEN=secret']),
                       lambda r:r['auth'].update(setup_url='https://example.com/?key=secret')]:
            value = record()
            mutate(value)
            self.reject(entries=[value])

    def test_only_pinned_candidate_skills_no_inherited_qualification(self):
        value = record()
        value['skills'] = [{'id':'gist/essentials/example@0.1.0','title':'Example',
                            'url':'https://github.com/sirerun/gist/tree/abc/example',
                            'status':'candidate','source_digest':'sha256:' + 'a'*64}]
        intake.build_snapshot([value], membership())
        for key, changed in [('status','qualified'), ('source_digest','unpinned'),
                             ('url','https://user:secret@example.com/skill')]:
            invalid = copy.deepcopy(value)
            invalid['skills'][0][key] = changed
            self.reject(entries=[invalid])

    def test_invalid_types_and_utf8_byte_bounds_fail_closed(self):
        for update in [{'service_id': []}, {'title': 'é' * 101}, {'description': 'é' * 1501},
                       {'tags': ['é' * 33]}, {'remotes': {}}, {'auth': []},
                       {'skills': {}}, {'version': 'é' * 101}]:
            value = record()
            value.update(update)
            self.reject(entries=[value])

    def test_timestamp_json_limits_and_input_hashes(self):
        for stamp in ['2026-10-18', '2026-10-18T03:00:00+00:00', '2026-02-30T03:00:00Z']:
            approved = membership()
            approved['retrieved_at'] = stamp
            self.reject(approved=approved)
        for raw in [b'{"a":1,"a":2}', b'{"a":NaN}']:
            with self.assertRaises(ValueError): intake.load_json(raw)
        with self.assertRaises(ValueError):
            intake.build_snapshot([record()], membership(), b'[]')
        source = json.dumps([record()]).encode()
        _, receipt = intake.build_snapshot([record()], membership(), source)
        self.assertEqual(receipt['curated_sha256'], hashlib.sha256(source).hexdigest())
        _, altered = intake.build_snapshot([record()], membership(), source + b'\n')
        self.assertNotEqual(receipt['revision'], altered['revision'])

    def test_offline_cli_reproduces_exact_outputs_without_legacy_inputs(self):
        TEMP_ROOT.mkdir(parents=True, exist_ok=True)
        with tempfile.TemporaryDirectory(prefix='intake-unit-', dir=TEMP_ROOT) as path:
            root = pathlib.Path(path)
            curated, approved = root/'curated.json', root/'approved.json'
            curated.write_text(json.dumps([record('z-last'), record('a-first', 'openapi')]))
            approved.write_text(json.dumps(membership('z-last', 'a-first')))
            original = curated.read_bytes(), approved.read_bytes()
            command = [sys.executable, str(HERE/'build_snapshot.py'), '--curated', str(curated), '--membership', str(approved), '--output', str(root/'snapshot.json'), '--receipt', str(root/'receipt.json')]
            subprocess.run(command, check=True, capture_output=True, timeout=10)
            first = (root/'snapshot.json').read_bytes(), (root/'receipt.json').read_bytes()
            subprocess.run(command, check=True, capture_output=True, timeout=10)
            self.assertEqual(first, ((root/'snapshot.json').read_bytes(), (root/'receipt.json').read_bytes()))
            self.assertEqual(original, (curated.read_bytes(), approved.read_bytes()))
            snapshot = json.loads(first[0])
            self.assertEqual([r['service_id'] for r in snapshot['entries']], ['z-last', 'a-first']) # sorted by full record ID
            self.assertEqual(snapshot['generated_at'], membership()['retrieved_at'])
            legacy = subprocess.run([sys.executable, str(HERE/'build_snapshot.py'), '--fetch'], capture_output=True, timeout=10)
            self.assertNotEqual(legacy.returncode, 0)
            self.assertIn(b'unrecognized arguments', legacy.stderr)


if __name__ == '__main__':
    unittest.main()

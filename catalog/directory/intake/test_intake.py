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


def record(service='example', kind='mcp'):
    result = {'id': kind + ':' + service, 'kind': kind, 'service_id': service,
              'title': 'Example', 'description': 'Operator-reviewed vendor route.',
              'tags': [], 'official': True,
              'official_evidence_url': 'https://vendor.example.com/docs/mcp',
              'auth': {'type': 'oauth2', 'credential_names': ['EXAMPLE_TOKEN']},
              'featured': False, 'recommended': False}
    if kind == 'mcp':
        result['remotes'] = [{'url': 'https://vendor.example.com/mcp', 'transport': 'streamable-http',
                              'check': {'checked_at': '2026-10-18T03:00:00Z', 'http_status': 200, 'result': 'mcp-initialize'}}]
    else:
        result['openapi'] = {'spec_url': 'https://vendor.example.com/openapi.yaml', 'mint_guide_url': 'https://github.com/sirerun/mint'}
    return result


def membership(*services):
    return {'format': 'gist.directory.admission/v1', 'policy': intake.POLICY,
            'retrieved_at': '2026-10-18T03:00:00Z',
            'apps': [{'service_id': service, 'listings': [{'directory': 'n8n', 'url': 'https://n8n.io/integrations/' + service + '/'}]} for service in services or ('example',)]}


def endpoint_checks(entries):
    urls = sorted({(remote['url'], remote['transport']) for item in entries if item['kind'] == 'mcp' for remote in item.get('remotes', [])})
    return {'format':'gist.mcp-endpoint-check/v1', 'checked_at':'2026-10-18T03:00:01Z',
            'endpoint_set_sha256':hashlib.sha256(json.dumps(urls,separators=(',',':')).encode()).hexdigest(),
            'probe':'unauthenticated remote HTTP reachability; streamable initialize or bounded SSE GET; no tools/list or tools/call',
            'checks':[{'url':item['url'],'transport':item['transport'],'method':'GET' if item['transport']=='sse' else 'POST', **item['check']} for item in entries if item['kind']=='mcp' for item in item.get('remotes',[])],
            'failures':[]}


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

    def test_provenance_is_required_and_no_featured_claims_are_imported(self):
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

    def test_independent_remote_mcp_is_admitted_with_explicit_registry_provenance(self):
        value=record()
        value['official']=False
        value.pop('official_evidence_url')
        value['server_listing_url']='https://registry.modelcontextprotocol.io/v0.1/servers/io.github%2Fexample%2Fmcp/versions/1.0.0'
        snapshot,_=intake.build_snapshot([value],membership())
        self.assertFalse(snapshot['entries'][0]['official'])
        self.assertEqual(snapshot['entries'][0]['source']['url'],value['server_listing_url'])
        for mutate in [lambda r:r.pop('server_listing_url'),
                       lambda r:r.update(server_listing_url='https://user:secret@example.com/server'),
                       lambda r:r.update(official_evidence_url='https://vendor.example.com/docs')]:
            invalid=copy.deepcopy(value); mutate(invalid); self.reject(entries=[invalid])

    def test_make_verified_remote_mcp_membership_matches_server_title(self):
        value=record()
        value.update(official=False, title='Example MCP')
        value.pop('official_evidence_url')
        value['server_listing_url']=intake.MAKE_VERIFIED_MCP_URL
        approved=membership()
        approved['apps'][0]['listings']=[{'directory':'make-verified-mcp','url':intake.MAKE_VERIFIED_MCP_URL,'server_name':'Example MCP'}]
        approved['make_verified_source']={'url':intake.MAKE_VERIFIED_MCP_URL,'retrieved_at':'2026-10-18T03:00:00Z','source_html_sha256':'a'*64,'inventory_sha256':'b'*64}
        snapshot,_=intake.build_snapshot([value],approved)
        self.assertIn('make-verified-remote-mcp-list',{source['id'] for source in snapshot['sources']})
        invalid=copy.deepcopy(approved); invalid.pop('make_verified_source')
        self.reject(entries=[value],approved=invalid)
        approved['apps'][0]['listings'][0]['server_name']='Other'
        self.reject(entries=[value],approved=approved)

    def test_endpoint_or_raw_spec_required_not_packages_or_html(self):
        snapshot, _ = intake.build_snapshot([record(kind='openapi')], membership())
        self.assertEqual(snapshot['entries'][0]['kind'], 'openapi')
        for mutate in [lambda r:r.pop('remotes'), lambda r:r.update(remotes=[]),
                       lambda r:r.update(packages=[{'registry':'npm','identifier':'@vendor/server'}]),
                       lambda r:r['remotes'][0].update(transport='stdio'),
                       lambda r:r['remotes'][0].update(check={'checked_at':'2026-10-18T03:00:00Z','http_status':404,'result':'mcp-initialize'}),
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

    def test_endpoint_check_receipt_matches_every_mcp_url_and_result(self):
        value, checks = record(), None
        checks = endpoint_checks([value])
        snapshot, receipt = intake.build_snapshot([value], membership(), checks=checks)
        self.assertEqual(receipt['checked_endpoints'], 1)
        self.assertEqual(snapshot['entries'][0]['remotes'][0]['check']['result'], 'mcp-initialize')
        for mutate in [lambda x:x.update(endpoint_set_sha256='0'*64),
                       lambda x:x['checks'][0].update(http_status=401),
                       lambda x:x['checks'].append(copy.deepcopy(x['checks'][0])),
                       lambda x:x.update(failures=[{'url':'https://vendor.example.com/mcp','error':'timeout'}])]:
            invalid=copy.deepcopy(checks); mutate(invalid)
            with self.assertRaises(ValueError): intake.build_snapshot([value], membership(), checks=invalid)

    def test_sse_and_plain_http_responses_can_pass_without_claiming_initialize(self):
        value=record()
        value['remotes'][0]={'url':'https://vendor.example.com/sse','transport':'sse','check':{'checked_at':'2026-10-18T03:00:00Z','http_status':200,'result':'endpoint-responded'}}
        checks=endpoint_checks([value])
        snapshot,_=intake.build_snapshot([value],membership(),checks=checks)
        self.assertEqual(snapshot['entries'][0]['remotes'][0]['transport'],'sse')
        value['remotes'][0]['check']['result']='mcp-initialize'
        with self.assertRaises(ValueError): intake.build_snapshot([value],membership(),checks=endpoint_checks([value]))

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
        with tempfile.TemporaryDirectory(prefix='intake-unit-') as path:
            root = pathlib.Path(path)
            curated, approved, checked = root/'curated.json', root/'approved.json', root/'checks.json'
            curated.write_text(json.dumps([record('z-last'), record('a-first', 'openapi')]))
            approved.write_text(json.dumps(membership('z-last', 'a-first')))
            checked.write_text(json.dumps(endpoint_checks(json.loads(curated.read_text()))))
            original = curated.read_bytes(), approved.read_bytes(), checked.read_bytes()
            command = [sys.executable, str(HERE/'build_snapshot.py'), '--curated', str(curated), '--membership', str(approved), '--checks', str(checked), '--output', str(root/'snapshot.json'), '--receipt', str(root/'receipt.json')]
            subprocess.run(command, check=True, capture_output=True, timeout=10)
            first = (root/'snapshot.json').read_bytes(), (root/'receipt.json').read_bytes()
            subprocess.run(command, check=True, capture_output=True, timeout=10)
            self.assertEqual(first, ((root/'snapshot.json').read_bytes(), (root/'receipt.json').read_bytes()))
            self.assertEqual(original, (curated.read_bytes(), approved.read_bytes(), checked.read_bytes()))
            snapshot = json.loads(first[0])
            self.assertEqual([r['service_id'] for r in snapshot['entries']], ['z-last', 'a-first']) # sorted by full record ID
            self.assertEqual(snapshot['generated_at'], membership()['retrieved_at'])
            self.assertEqual(json.loads(first[1])['checked_endpoints'], 1)
            legacy = subprocess.run([sys.executable, str(HERE/'build_snapshot.py'), '--fetch'], capture_output=True, timeout=10)
            self.assertNotEqual(legacy.returncode, 0)
            self.assertIn(b'unrecognized arguments', legacy.stderr)


if __name__ == '__main__':
    unittest.main()

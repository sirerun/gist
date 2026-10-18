"""Offline security/reproducibility checks; never fetch upstream or providers."""
import gzip
import hashlib
import importlib.util
import json
import pathlib
import unittest

HERE = pathlib.Path(__file__).resolve().parent
spec = importlib.util.spec_from_file_location('intake', HERE / 'build_snapshot.py')
intake = importlib.util.module_from_spec(spec)
spec.loader.exec_module(intake)


class IntakeTests(unittest.TestCase):
    def test_source_projection_removes_credentials_and_unconsumed_values(self):
        page = {'servers': [{'server': {
            'name': 'com.example/test', 'version': '1', 'description': 'Example',
            'repository': {'url': 'https://user:password@example.com/repo'},
            'websiteUrl': 'https://example.com/?api_key=credential',
            'remotes': [
                {'type': 'streamable-http', 'url': 'https://example.com/mcp',
                 'headers': [{'name': 'Authorization', 'value': 'Bearer credential'}]},
                {'type': 'streamable-http', 'url': 'https://example.com/mcp?key=credential'},
            ],
            'packages': [{'registryType': 'npm', 'identifier': '@example/server',
                          'packageArguments': [{'value': '--token=credential'}],
                          'environmentVariables': [{'name': 'EXAMPLE_TOKEN', 'isSecret': True,
                                                    'value': 'credential', 'default': 'credential'}]}],
        }, '_meta': {'io.modelcontextprotocol.registry/official': {'status': 'active', 'isLatest': True},
                     'publisher-extra': 'credential'}}]}
        projected = intake.projection([json.dumps(page)])
        self.assertNotIn(b'credential', projected)
        server = json.loads(json.loads(projected)['pages'][0])['servers'][0]['server']
        self.assertEqual(server['packages'][0]['environmentVariables'],
                         [{'name': 'EXAMPLE_TOKEN', 'isSecret': True}])
        self.assertEqual(len(server['remotes']), 1)
        self.assertNotIn('headers', server['remotes'][0])
        self.assertNotIn('packageArguments', server['packages'][0])
        self.assertNotIn('repository', server)
        self.assertNotIn('websiteUrl', server)

    def test_advertised_urls_do_not_pass_public_target_validation(self):
        for url in ('http://example.com/mcp', 'https://localhost/mcp',
                    'https://127.0.0.1/mcp', 'https://192.168.2.3/mcp',
                    'https://machine.internal/mcp', 'https://u:p@example.com/mcp',
                    'https://example.com/mcp?token=value', 'https://{tenant}.gatana.ai/mcp',
                    'https://example.com:notaport/mcp', 'https://bad host.example.com/mcp', 'https://example.com/mcp#token'):
            self.assertFalse(intake.safe_url(url), url)
        self.assertTrue(intake.safe_url('https://example.com/mcp'))

    def test_redirect_handler_refuses_before_following(self):
        with self.assertRaises(RuntimeError):
            intake.NoRedirect().redirect_request(None, None, 302, '', {}, 'http://localhost/mcp')

    def test_committed_source_hashes_and_qualification_boundary(self):
        lock = json.loads((HERE / 'source-lock.json').read_text())
        self.assertEqual(hashlib.sha256(gzip.decompress((HERE / 'official-projection.json.gz').read_bytes())).hexdigest(),
                         lock['official_projection_sha256'])
        self.assertEqual(hashlib.sha256(gzip.decompress((HERE / 'local-registry.json.gz').read_bytes())).hexdigest(),
                         lock['local_sha256'])
        self.assertEqual(len(lock['official_page_sha256']), lock['official_pages'])
        snapshot = json.loads((HERE.parent / 'snapshot.json').read_text())
        self.assertEqual(sum(bool(r.get('featured')) for r in snapshot['entries']), 0)
        self.assertTrue(all(not r.get('verification') and not r.get('featured_selection') for r in snapshot['entries']))
        self.assertEqual(sum(bool(r.get('recommended')) for r in snapshot['entries']), 5)
        records = {r['id']: r for r in snapshot['entries']}
        self.assertEqual(records['mcp:google-gmail']['remotes'],
                         [{'url': 'https://gmailmcp.googleapis.com/mcp/v1', 'transport': 'streamable-http'}])
        self.assertFalse(records['openapi:google-gmail']['recommended'])
        self.assertEqual(json.loads((HERE / 'receipt.json').read_text())['curated_recommended'], 5)
        for record in snapshot['entries']:
            if not record.get('recommended'):
                continue
            skill = record['skills'][0]
            key = skill['id'].split('/')[-1].split('@')[0]
            manifest = json.loads((HERE.parents[2] / 'skills' / 'essentials' / 'sources' / key / 'manifest.json').read_text())
            self.assertEqual(skill['id'], manifest['id'] + '@' + manifest['version'])
            self.assertEqual(skill['source_digest'], 'sha256:' + manifest['package_digest']['value'])
            self.assertEqual(skill['status'], 'candidate')
            self.assertIn('/94ef81edc7eaf191a9d3606b6f34c52d2098816e/', skill['url'])


if __name__ == '__main__':
    unittest.main()

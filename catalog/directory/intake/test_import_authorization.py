import copy
import hashlib
import json
import tempfile
import unittest
from pathlib import Path

from import_authorization import import_batch, V1, V2


class ImportAuthorizationTests(unittest.TestCase):
    def setUp(self):
        self.tmp = tempfile.TemporaryDirectory()
        self.addCleanup(self.tmp.cleanup)
        self.root = Path(self.tmp.name)
        self.source, self.queue, self.catalog = [self.root / n for n in ('source.json', 'queue.json', 'apps.json')]
        self.write(self.catalog, {'items': [{'slug': 'demo', 'name': 'Démonstration'}, {'slug': 'other', 'name': 'Other'}]})
        self.write(self.queue, [{'slug': 'demo', 'name': 'Démonstration'}])
        self.guide = {'kind': 'http-api', 'url': 'https://api.vendor.example/v1', 'methods': ['api_key'], 'credential_names': ['API key'], 'steps': ['Configure the documented header in the client.'], 'sources': [{'url': 'https://vendor.example/docs', 'checked_at': '2026-10-19'}]}
        self.batch = [{'slug': 'demo', 'status': 'documented', 'routes': [self.guide], 'notes': [], 'sources': []}]

    def write(self, path, value):
        path.write_text(json.dumps(value), encoding='utf-8')

    def run_import(self, existing=None, replace=False, digest=None):
        self.write(self.source, self.batch)
        return import_batch(self.source, self.queue, self.catalog, existing or [], digest or hashlib.sha256(self.source.read_bytes()).hexdigest(), replace)

    def test_exact_source_receipt_and_canonical_identity(self):
        with self.assertRaisesRegex(ValueError, 'SHA256'):
            self.run_import(digest='0' * 64)
        self.write(self.queue, [{'slug': 'demo', 'name': 'Wrong name'}])
        with self.assertRaisesRegex(ValueError, 'canonical'):
            self.run_import()
        self.write(self.queue, [{'slug': 'unknown', 'name': 'Unknown'}])
        self.batch[0]['slug'] = 'unknown'
        with self.assertRaisesRegex(ValueError, 'unknown slug'):
            self.run_import()

    def test_slug_set_and_duplicate_rejection(self):
        self.write(self.queue, [{'slug': 'other', 'name': 'Other'}])
        with self.assertRaisesRegex(ValueError, 'slug set'):
            self.run_import()
        self.write(self.queue, [{'slug': 'demo', 'name': 'Démonstration'}])
        self.batch.append(copy.deepcopy(self.batch[0]))
        with self.assertRaisesRegex(ValueError, 'slug set'):
            self.run_import()

    def test_collision_requires_explicit_replace_and_preserves_unrelated(self):
        unrelated = {'slug': 'other', 'routes': [{'authored': 'preserve verbatim'}]}
        old = {'slug': 'demo', 'routes': []}
        before = [unrelated, old]
        original = copy.deepcopy(before)
        with self.assertRaisesRegex(ValueError, 'collision'):
            self.run_import(before)
        result = self.run_import(before, replace=True)
        self.assertEqual(before, original)
        self.assertEqual(result[0], unrelated)
        self.assertEqual(result[1]['routes'][0]['title'], 'Démonstration')
        self.assertEqual(result, self.run_import(before, replace=True))

    def test_v1_and_transport_routing_without_inference(self):
        self.assertEqual(self.run_import()[0]['routes'][0]['format'], V1)
        self.guide.update(kind='mcp', transport='streamable_http')
        route = self.run_import()[0]['routes'][0]
        self.assertEqual(route['format'], V1)
        self.assertEqual(route['remotes'][0]['transport'], 'streamable-http')
        for transport in ('http', None):
            if transport is None:
                self.guide.pop('transport', None)
            else:
                self.guide['transport'] = transport
            route = self.run_import()[0]['routes'][0]
            self.assertEqual(route['format'], V2)
            self.assertEqual(route['remotes'][0]['transport'], 'unknown')
            self.assertTrue(route['notes'])

    def test_provider_private_url_is_input_not_invented_route(self):
        self.guide.pop('url')
        self.guide.update(kind='mcp', endpoint_inputs=['PROVIDER_MCP_URL'])
        route = self.run_import()[0]['routes'][0]
        self.assertEqual(route['format'], V2)
        self.assertEqual(route['remotes'], [])
        self.assertNotIn('api_url', route)
        self.assertEqual(route['endpoint_inputs'], ['PROVIDER_MCP_URL'])
        self.guide['endpoint_inputs'] = ['https://private.vendor.example/secret']
        with self.assertRaisesRegex(ValueError, 'input names'):
            self.run_import()

    def test_templates_native_and_new_method_require_v2(self):
        self.guide.pop('url')
        self.guide.update(api_url_template='{BASE_URI}/restapi', endpoint_inputs=['BASE_URI'])
        self.assertEqual(self.run_import()[0]['routes'][0]['format'], V2)
        self.guide['api_url_template'] = 'https://vendor.example/?token={BASE_URI}'
        with self.assertRaises(ValueError):
            self.run_import()
        self.guide.pop('api_url_template')
        self.guide.update(kind='native-api', protocol='mysql', methods=['password'], endpoint_inputs=['HOST', 'USERNAME'])
        self.assertEqual(self.run_import()[0]['routes'][0]['protocol'], 'mysql')
        self.guide.update(kind='http-api', url='https://vendor.example/api')
        self.guide.pop('protocol')
        self.guide.pop('endpoint_inputs')
        for method in ['hmac', 'oauth1a']:
            self.guide['methods'] = [method]
            self.assertEqual(self.run_import()[0]['routes'][0]['format'], V2)

    def test_rdstation_language_query_is_source_only(self):
        url = 'https://developers.rdstation.com/reference/authentication?lng=pt-BR'
        self.guide['sources'][0]['url'] = url
        route = self.run_import()[0]['routes'][0]
        self.assertEqual(route['sources'][0]['url'], url)
        self.assertEqual(route['setup_url'], url)
        self.guide['url'] = url
        with self.assertRaisesRegex(ValueError, 'query'):
            self.run_import()
        self.guide['url'] = 'https://api.vendor.example/v1'
        self.guide['sources'][0]['url'] = 'https://unrelated.example/docs?lng=pt-BR'
        with self.assertRaisesRegex(ValueError, 'query'):
            self.run_import()

    def test_honest_gaps_and_sourced_classification(self):
        self.batch[0].update(status='needs_research', routes=[], notes=['Exact provider route not verified.'])
        self.assertEqual(self.run_import()[0]['routes'], [])
        self.batch[0]['status'] = 'not_applicable'
        with self.assertRaisesRegex(ValueError, 'evidence'):
            self.run_import()
        self.batch[0]['sources'] = self.guide['sources']
        self.assertEqual(self.run_import()[0]['status'], 'not_applicable')


if __name__ == '__main__':
    unittest.main()

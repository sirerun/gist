"""Endpoint response classification without network access."""
import importlib.util
import pathlib
import subprocess
import types
import unittest
from unittest import mock

PATH = pathlib.Path(__file__).with_name('check_mcp_endpoints.py')
spec = importlib.util.spec_from_file_location('endpoint_check', PATH)
checker = importlib.util.module_from_spec(spec)
spec.loader.exec_module(checker)


class EndpointClassificationTests(unittest.TestCase):
    def test_initialize_is_distinguished_from_plain_response(self):
        body = b'{"jsonrpc":"2.0","id":1,"result":{"protocolVersion":"2025-06-18","capabilities":{},"serverInfo":{"name":"fixture","version":"1"}}}'
        self.assertEqual(checker.classify(200, body), 'mcp-initialize')
        self.assertEqual(checker.classify(200, b'<html>route responded</html>'), 'endpoint-responded')

    def test_malformed_or_unmatched_initialize_is_only_a_response(self):
        bodies = [
            b'{"result":{"protocolVersion":"not-mcp"}}',
            b'{"jsonrpc":"2.0","id":99,"result":{"protocolVersion":"2025-06-18","capabilities":{},"serverInfo":{"name":"fixture","version":"1"}}}',
            b'{"jsonrpc":"2.0","id":1,"error":{"code":-1},"result":{"protocolVersion":"2025-06-18","capabilities":{},"serverInfo":{"name":"fixture","version":"1"}}}',
        ]
        for body in bodies:
            with self.subTest(body=body):
                self.assertEqual(checker.classify(200, body), 'endpoint-responded')

    def test_probe_disables_curl_config_proxy_and_url_globbing(self):
        def fake_run(command, **kwargs):
            output = pathlib.Path(command[command.index('--output') + 1])
            output.write_bytes(b'{"jsonrpc":"2.0","id":1,"result":{"protocolVersion":"2025-06-18","capabilities":{},"serverInfo":{"name":"fixture","version":"1"}}}')
            return types.SimpleNamespace(stdout=b'200', stderr=b'', returncode=0)

        with mock.patch.object(checker, 'resolve_public_ipv4', return_value=('example.com', '93.184.216.34')):
            with mock.patch.object(checker.subprocess, 'run', side_effect=fake_run) as run:
                with mock.patch.dict(checker.os.environ, {'HTTPS_PROXY': 'https://user:secret@proxy.invalid', 'CURL_HOME': '/tmp/curl'}, clear=True):
                    self.assertEqual(checker.probe('https://example.com/mcp', 'streamable-http')['result'], 'mcp-initialize')
        command = run.call_args.args[0]
        self.assertEqual(command[1], '--disable')
        self.assertIn('--globoff', command)
        self.assertIn('--proxy', command)
        self.assertEqual(command[command.index('--proxy') + 1], '')
        self.assertEqual(command[command.index('--noproxy') + 1], '*')
        self.assertIn('--resolve', command)
        self.assertNotIn('HTTPS_PROXY', run.call_args.kwargs['env'])
        self.assertNotIn('CURL_HOME', run.call_args.kwargs['env'])

    def test_dns_is_bounded_and_rejects_private_or_trailing_dot_names(self):
        with mock.patch.object(checker.subprocess, 'run', return_value=types.SimpleNamespace(stdout='["93.184.216.34"]')) as run:
            self.assertEqual(checker.resolve_public_ipv4('https://example.com/mcp'), ('example.com', '93.184.216.34'))
            self.assertEqual(run.call_args.kwargs['timeout'], 3)
        with mock.patch.object(checker.subprocess, 'run', return_value=types.SimpleNamespace(stdout='["127.0.0.1"]')):
            with self.assertRaises(ValueError):
                checker.resolve_public_ipv4('https://example.com/mcp')
        with self.assertRaises(ValueError):
            checker.resolve_public_ipv4('https://example.com./mcp')
        with mock.patch.object(checker.subprocess, 'run', side_effect=subprocess.TimeoutExpired('python', 3)):
            with self.assertRaisesRegex(ValueError, 'exceeded 3 seconds'):
                checker.resolve_public_ipv4('https://example.com/mcp')

    def test_auth_payment_redirect_and_method_responses_are_not_overclaimed(self):
        self.assertEqual(checker.classify(401, b''), 'authentication-required')
        self.assertEqual(checker.classify(403, b''), 'authentication-required')
        self.assertEqual(checker.classify(402, b''), 'payment-required')
        self.assertEqual(checker.classify(302, b''), 'endpoint-responded')
        self.assertEqual(checker.classify(405, b''), 'endpoint-responded')

    def test_gone_and_server_errors_fail_intake(self):
        for status in (404, 410, 500, 503):
            with self.subTest(status=status), self.assertRaises(ValueError):
                checker.classify(status, b'')


if __name__ == '__main__':
    unittest.main()

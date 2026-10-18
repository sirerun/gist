"""Endpoint response classification without network access."""
import importlib.util
import pathlib
import unittest

PATH = pathlib.Path(__file__).with_name('check_mcp_endpoints.py')
spec = importlib.util.spec_from_file_location('endpoint_check', PATH)
checker = importlib.util.module_from_spec(spec)
spec.loader.exec_module(checker)


class EndpointClassificationTests(unittest.TestCase):
    def test_initialize_is_distinguished_from_plain_response(self):
        body = b'{"jsonrpc":"2.0","id":1,"result":{"protocolVersion":"2025-06-18"}}'
        self.assertEqual(checker.classify(200, body), 'mcp-initialize')
        self.assertEqual(checker.classify(200, b'<html>route responded</html>'), 'endpoint-responded')

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

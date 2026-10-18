#!/usr/bin/env python3
"""Check reviewed public remote MCP URLs without credentials or tool execution.

Streamable HTTP routes receive only initialize. Legacy SSE routes receive a
bounded GET. Any successful HTTP response proves reachability only; a parsed
initialize result is separately recorded as an MCP handshake.
"""
import argparse
import datetime
import hashlib
import ipaddress
import json
import pathlib
import re
import socket
import subprocess
import tempfile
import urllib.parse
from concurrent.futures import ThreadPoolExecutor, as_completed

HERE = pathlib.Path(__file__).resolve().parent
CURATED = HERE / 'curated.json'
BODY = json.dumps({
    'jsonrpc': '2.0', 'id': 1, 'method': 'initialize',
    'params': {'protocolVersion': '2025-06-18', 'capabilities': {},
               'clientInfo': {'name': 'gist-endpoint-check', 'version': '1.0'}},
}, separators=(',', ':')).encode()


def resolve_public_ipv4(url):
    parsed = urllib.parse.urlsplit(url)
    if parsed.scheme != 'https' or not parsed.hostname or parsed.username or parsed.password or parsed.query or parsed.fragment:
        raise ValueError('endpoint must be an HTTPS URL without credentials, query or fragment')
    if parsed.port not in (None, 443):
        raise ValueError('endpoint must use the standard HTTPS port')
    host = parsed.hostname.rstrip('.').lower()
    if not re.fullmatch(r'(?=.{1,253}$)(?:[a-z0-9](?:[a-z0-9-]{0,61}[a-z0-9])?\.)+[a-z0-9](?:[a-z0-9-]{0,61}[a-z0-9])?', host):
        raise ValueError('invalid public endpoint hostname')
    values = socket.getaddrinfo(host, 443, type=socket.SOCK_STREAM, family=socket.AF_INET)
    ips = sorted({item[4][0] for item in values})
    if not ips or any(not ipaddress.ip_address(value).is_global for value in ips):
        raise ValueError('endpoint DNS did not resolve exclusively to public IPv4 addresses')
    return host, ips[0]


def classify(status, body):
    if status in (401, 403):
        return 'authentication-required'
    if status == 402:
        return 'payment-required'
    if 300 <= status < 400:
        return 'endpoint-responded'
    if status in (400, 405):
        return 'endpoint-responded'
    if not 200 <= status < 300:
        raise ValueError('endpoint returned HTTP ' + str(status))
    candidates = [body]
    for line in body.splitlines():
        if line.startswith(b'data:'):
            candidates.append(line[5:].strip())
    for candidate in candidates:
        try:
            value = json.loads(candidate)
        except (json.JSONDecodeError, UnicodeDecodeError):
            continue
        result = value.get('result') if isinstance(value, dict) else None
        if isinstance(result, dict) and isinstance(result.get('protocolVersion'), str):
            return 'mcp-initialize'
    return 'endpoint-responded'


def probe(url, transport, timeout=12):
    host, address = resolve_public_ipv4(url)
    parsed = urllib.parse.urlsplit(url)
    with tempfile.TemporaryDirectory(prefix='gist-mcp-check-') as folder:
        output = pathlib.Path(folder) / 'response'
        method = 'GET' if transport == 'sse' else 'POST'
        command = [
            'curl', '--silent', '--show-error', '--proto', '=https',
            '--connect-timeout', '4', '--max-time', str(min(timeout, 5) if method == 'GET' else timeout),
            '--max-redirs', '0', '--max-filesize', '65536',
            '--resolve', host + ':443:' + address,
            '--header', 'Accept: text/event-stream' if method == 'GET' else 'Accept: application/json, text/event-stream',
            '--output', str(output), '--write-out', '%{http_code}',
        ]
        if method == 'POST':
            command += ['--request', 'POST', '--header', 'Content-Type: application/json', '--data-binary', '@-']
        command.append(url)
        result = subprocess.run(command, input=BODY if method == 'POST' else None,
                                capture_output=True, timeout=timeout + 5, check=False)
        try:
            status = int(result.stdout.decode('ascii').strip())
        except ValueError as error:
            raise ValueError('curl failed before receiving an HTTP status: ' + result.stderr.decode('utf-8', 'replace')[:180]) from error
        if result.returncode != 0 and not ((method == 'GET' or result.returncode == 28) and result.returncode in (28, 63)):
            raise ValueError('curl failed: ' + result.stderr.decode('utf-8', 'replace')[:240])
        body = output.read_bytes() if output.exists() else b''
        result_name = classify(status, body)
        if method == 'GET' and result_name == 'mcp-initialize':
            result_name = 'endpoint-responded'
        if method == 'POST' and result.returncode == 28 and body and status == 200 and result_name == 'mcp-initialize':
            result_name = 'endpoint-responded'
        return {'method': method, 'http_status': status, 'result': result_name}


def main():
    parser = argparse.ArgumentParser(description=__doc__)
    parser.add_argument('--input', type=pathlib.Path, default=CURATED)
    parser.add_argument('--output', type=pathlib.Path, required=True)
    parser.add_argument('--timeout', type=int, default=12)
    parser.add_argument('--workers', type=int, default=8)
    args = parser.parse_args()
    if not 2 <= args.timeout <= 30:
        parser.error('--timeout must be between 2 and 30 seconds')
    if not 1 <= args.workers <= 12:
        parser.error('--workers must be between 1 and 12')
    raw = args.input.read_bytes()
    entries = json.loads(raw)
    endpoints = sorted({(remote['url'], remote.get('transport', 'streamable-http')) for entry in entries if entry.get('kind') == 'mcp'
                        for remote in entry.get('remotes', [])
                        if remote.get('transport') in ('streamable-http', 'sse')})
    if len(endpoints) > 200:
        parser.error('refusing to probe more than 200 unique endpoints in one run')
    checks, failures = [], []
    checked_at = datetime.datetime.now(datetime.timezone.utc).replace(microsecond=0).isoformat().replace('+00:00', 'Z')
    with ThreadPoolExecutor(max_workers=args.workers) as executor:
        futures = {executor.submit(probe, url, transport, args.timeout): (url, transport) for url, transport in endpoints}
        for future in as_completed(futures):
            url, transport = futures[future]
            try:
                checks.append({'url': url, 'transport': transport, 'checked_at': checked_at, **future.result()})
            except Exception as error:  # A bounded check reports failures without stopping the batch.
                failures.append({'url': url, 'transport': transport, 'error': str(error)[:300]})
    checks.sort(key=lambda row: (row['url'], row['transport']))
    failures.sort(key=lambda row: (row['url'], row['transport']))
    report = {
        'format': 'gist.mcp-endpoint-check/v1',
        'checked_at': checked_at,
        'endpoint_set_sha256': hashlib.sha256(json.dumps(endpoints, separators=(',', ':')).encode()).hexdigest(),
        'probe': 'unauthenticated remote HTTP reachability; streamable initialize or bounded SSE GET; no tools/list or tools/call',
        'checks': checks,
        'failures': failures,
    }
    args.output.write_text(json.dumps(report, ensure_ascii=False, indent=2) + '\n')
    print(json.dumps({'checked': len(checks), 'failed': len(failures), 'report': str(args.output)}))
    return 1 if failures else 0


if __name__ == '__main__':
    raise SystemExit(main())

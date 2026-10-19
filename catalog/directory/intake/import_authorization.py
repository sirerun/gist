#!/usr/bin/env python3
"""Import an exact, reviewed authorization batch offline; does not qualify claims.

The expected source digest must come from the independent review receipt.
No provider requests, credential acquisition, or endpoint substitution occur here.
"""
import argparse
import datetime
import hashlib
import json
import re
import unicodedata
from pathlib import Path
from urllib.parse import parse_qs, urlsplit

V1 = 'gist.directory.authorization/v1'
V2 = 'gist.directory.authorization/v2'
BASE_METHODS = {'oauth2', 'api_key', 'bearer', 'basic', 'aws_sigv4', 'none'}
V2_METHODS = {'password', 'client_certificate', 'ssh_key', 'hmac', 'oauth1a'}
FIELD = re.compile(r'^[A-Za-z_][A-Za-z0-9_]{0,63}$')
PLACEHOLDER = re.compile(r'\{([A-Za-z_][A-Za-z0-9_]{0,63})\}')
ROUTE_FIELDS = {'kind', 'url', 'transport', 'methods', 'credential_names', 'prerequisites', 'steps', 'scopes', 'sources', 'notes', 'setup_url', 'api_url_template', 'endpoint_inputs', 'protocol'}


def fail(message):
    raise ValueError(message)


def unique_object(pairs):
    result = {}
    for key, value in pairs:
        if key in result:
            fail('duplicate JSON field')
        result[key] = value
    return result


def decode_json(raw):
    if len(raw) > 16 << 20:
        fail('input exceeds 16 MiB')
    return json.loads(raw, object_pairs_hook=unique_object)


def read_json(path):
    return decode_json(Path(path).read_bytes())


def strings(value, maximum=32):
    if not isinstance(value, list) or len(value) > maximum:
        fail('invalid string list')
    for item in value:
        if not isinstance(item, str) or not item or len(item.encode()) > 2000 or item.strip() != item or any(unicodedata.category(c) == 'Cc' for c in item):
            fail('invalid metadata text')
    return list(value)


def safe_url(value, source=False):
    if not isinstance(value, str) or not value or len(value.encode()) > 2048 or value.strip() != value or any(ord(c) < 32 for c in value) or any(c in value for c in '{}'):
        fail('invalid metadata URL')
    u = urlsplit(value)
    if u.scheme != 'https' or not u.hostname or '.' not in u.hostname or u.username or u.password:
        fail('public HTTPS metadata URL required')
    try:
        u.port
    except ValueError:
        fail('invalid URL port')
    host = u.hostname.lower().rstrip('.')
    if host == 'localhost' or host.endswith(('.localhost', '.local', '.internal')):
        fail('private metadata URL')
    import ipaddress
    try:
        address = ipaddress.ip_address(host)
    except ValueError:
        address = None
    if address is not None and not address.is_global:
        fail('private metadata address')
    if u.query:
        allowed = {'learn.microsoft.com': {'view', 'tabs', 'preserve-view'}, 'shopify.dev': {'lang'}, 'www.postman.com': {'entity'}, 'developers.rdstation.com': {'lng'}}.get(host, set()) if source else set()
        query = parse_qs(u.query, keep_blank_values=True, strict_parsing=True)
        if not query or any(k not in allowed or len(v) != 1 or len(v[0]) > 256 for k, v in query.items()):
            fail('unapproved metadata URL query')
    if u.fragment and not source:
        fail('endpoint fragment forbidden')
    return value


def sources(value):
    if not isinstance(value, list) or len(value) > 12:
        fail('invalid sources')
    out = []
    for item in value:
        if not isinstance(item, dict) or set(item) != {'url', 'checked_at'}:
            fail('invalid source fields')
        safe_url(item['url'], source=True)
        if not isinstance(item['checked_at'], str) or not re.fullmatch(r'\d{4}-\d{2}-\d{2}', item['checked_at']):
            fail('invalid source date')
        datetime.date.fromisoformat(item['checked_at'])
        out.append(dict(item))
    return out


def route(raw, slug, name, number):
    if not isinstance(raw, dict) or set(raw) - ROUTE_FIELDS:
        fail('unknown route fields')
    kind = raw.get('kind')
    if kind not in {'mcp', 'http-api', 'openapi', 'native-api'}:
        fail('invalid route kind')
    if kind != 'mcp' and 'transport' in raw:
        fail('transport is only valid for MCP')
    methods = strings(raw.get('methods', []))
    if not methods or set(methods) - BASE_METHODS - V2_METHODS:
        fail('invalid authorization methods')
    out = {'kind': kind, 'methods': methods}
    for field in ('credential_names', 'prerequisites', 'steps', 'scopes', 'notes'):
        out[field] = strings(raw.get(field, []))
    out['sources'] = sources(raw.get('sources', []))
    if not out['sources'] or not out['steps']:
        fail('documented route requires source evidence and steps')
    inputs = strings(raw.get('endpoint_inputs', []), 16)
    if len(set(inputs)) != len(inputs) or any(not FIELD.fullmatch(v) for v in inputs):
        fail('invalid endpoint input names')
    template, protocol, url = raw.get('api_url_template'), raw.get('protocol'), raw.get('url')
    v2 = bool(inputs or template or protocol or set(methods) & V2_METHODS)
    out['remotes'] = []
    if kind == 'mcp':
        if template or protocol or bool(url) == bool(inputs):
            fail('MCP needs either exact URL or provider endpoint inputs')
        if url:
            safe_url(url)
            transport = raw.get('transport') or 'unknown'
            if not isinstance(transport, str):
                fail('invalid transport')
            transport = transport.replace('_', '-')
            if transport == 'http' or transport == 'unknown':
                transport, v2 = 'unknown', True
                note = 'Transport was not established by the reviewed source; the client must verify it.'
                if note not in out['notes']:
                    if len(out['notes']) == 32:
                        fail('too many transport notes')
                    out['notes'].append(note)
            if transport not in {'sse', 'streamable-http', 'unknown'}:
                fail('unsupported transport')
            out['remotes'] = [{'url': url, 'transport': transport}]
    elif kind == 'native-api':
        if url or template or not inputs or not isinstance(protocol, str) or not FIELD.fullmatch(protocol):
            fail('native API needs protocol and named inputs')
    else:
        if protocol or (url and template) or not (url or template or inputs):
            fail('API needs exact URL, template, or provider endpoint inputs')
        if url:
            out['api_url'] = safe_url(url)
        if template:
            names = PLACEHOLDER.findall(template)
            if not names or any(v not in inputs for v in names):
                fail('undeclared template input')
            checked = PLACEHOLDER.sub('example.com', template)
            if template.startswith('{'):
                first = PLACEHOLDER.match(template)
                if first and (not template[first.end():] or template[first.end():].startswith('/')):
                    checked = 'https://example.com' + PLACEHOLDER.sub('example.com', template[first.end():])
            if '{' in checked or '}' in checked:
                fail('invalid template placeholder')
            safe_url(checked)
            out['api_url_template'] = template
    if inputs:
        out['endpoint_inputs'] = inputs
    if protocol:
        out['protocol'] = protocol
    prefix = 'app-mcp:' if kind == 'mcp' else 'api:'
    out.update(format=V2 if v2 else V1, id=prefix + slug + (f':{number}' if number > 1 else ''), title=name, status='documented', connection_tested=False, credential_owner='client', setup_url=safe_url(raw.get('setup_url') or out['sources'][0]['url'], source=True))
    return out


def import_batch(source_path, input_path, catalog_path, existing, expected_sha256, replace_existing=False):
    reviewed_bytes = Path(source_path).read_bytes()
    digest = hashlib.sha256(reviewed_bytes).hexdigest()
    if not re.fullmatch(r'[0-9a-fA-F]{64}', expected_sha256) or digest != expected_sha256.lower():
        fail('reviewed source SHA256 mismatch')
    batch, queue, catalog = decode_json(reviewed_bytes), read_json(input_path), read_json(catalog_path)
    names = {x['slug']: x['name'] for x in catalog['items']}
    if len(names) != len(catalog['items']):
        fail('duplicate canonical slug')
    if not isinstance(batch, list) or not isinstance(queue, list):
        fail('batch and queue must be arrays')
    wanted = [x['slug'] for x in queue]
    got = [x['slug'] for x in batch]
    if len(set(wanted)) != len(wanted) or len(set(got)) != len(got) or set(wanted) != set(got):
        fail('batch slug set mismatch')
    for x in queue:
        if x['slug'] not in names or x.get('name') != names[x['slug']]:
            fail('unknown slug or noncanonical queue name')
    incoming = {}
    for item in batch:
        if set(item) - {'slug', 'status', 'routes', 'notes', 'sources'}:
            fail('unknown batch fields')
        slug, status = item['slug'], item['status']
        if slug not in names:
            fail('unknown app slug')
        raw_routes = item.get('routes', [])
        if not isinstance(raw_routes, list) or len(raw_routes) > 8:
            fail('too many routes')
        notes, evidence = strings(item.get('notes', []), 16), sources(item.get('sources', []))
        if status == 'documented':
            if not raw_routes:
                fail('documented app needs routes')
        elif status in {'needs_research', 'not_applicable'}:
            if raw_routes or not notes or (status == 'not_applicable' and not evidence):
                fail('unresolved classification requires notes and evidence')
        else:
            fail('invalid app status')
        incoming[slug] = dict(slug=slug, status=status, routes=[route(r, slug, names[slug], n) for n, r in enumerate(raw_routes, 1)], notes=notes, sources=evidence)
    if not isinstance(existing, list):
        fail('existing output must be an array')
    seen = set()
    for entry in existing:
        if entry['slug'] in seen:
            fail('duplicate existing slug')
        seen.add(entry['slug'])
    if seen & incoming.keys() and not replace_existing:
        fail('existing slug collision; use --replace-existing explicitly')
    result = [incoming.pop(x['slug'], x) for x in existing]
    result.extend(incoming[s] for s in wanted if s in incoming)
    return result


def main():
    directory = Path(__file__).resolve().parent.parent
    parser = argparse.ArgumentParser(description=__doc__)
    parser.add_argument('--source', required=True, type=Path)
    parser.add_argument('--input', required=True, type=Path, help='exact reviewed batch queue')
    parser.add_argument('--expected-sha256', required=True)
    parser.add_argument('--catalog', type=Path, default=directory / 'apps.json')
    parser.add_argument('--output', type=Path, default=directory / 'app-authorization.json')
    parser.add_argument('--replace-existing', action='store_true')
    args = parser.parse_args()
    try:
        existing = read_json(args.output) if args.output.exists() else []
        result = import_batch(args.source, args.input, args.catalog, existing, args.expected_sha256, args.replace_existing)
        data = json.dumps(result, ensure_ascii=False, indent=2) + '\n'
        temporary = args.output.with_name(args.output.name + '.import-tmp')
        with temporary.open('x', encoding='utf-8') as stream:
            stream.write(data)
        temporary.replace(args.output)
    except (ValueError, KeyError, TypeError, OSError) as exc:
        parser.exit(2, 'Import rejected: ' + str(exc) + '\n')


if __name__ == '__main__':
    main()

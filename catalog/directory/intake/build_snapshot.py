#!/usr/bin/env python3
"""Build the reviewed MCP catalog offline. No registry import or network fetch.

Membership and listing provenance are independently recorded; neither alone
proves vendor authority, reachability, tool safety or workflow qualification.
"""
import argparse
import copy
import datetime
import hashlib
import ipaddress
import json
import pathlib
import re
import urllib.parse

HERE = pathlib.Path(__file__).resolve().parent
ROOT = HERE.parent
POLICY = 'n8n-or-make-verified/reachable-remote-mcp'
MAKE_VERIFIED_MCP_URL = 'https://apps.make.com/c5ZhYquO55e90hoCJ0rzt'
MAX_INPUT_BYTES = 4 * 1024 * 1024
MAX_ENTRIES = 10000
SERVICE = re.compile(r'[a-z0-9]+(?:-[a-z0-9]+)*\Z')
FIELDS = {'id', 'kind', 'service_id', 'title', 'description', 'version', 'tags',
          'homepage', 'repository', 'remotes', 'openapi', 'auth', 'skills',
          'official', 'official_evidence_url', 'server_listing_url', 'recommended', 'featured'}
AUTH_TYPES = {'none', 'unknown', 'oauth2', 'api_key', 'api-key', 'bearer', 'basic',
              'environment', 'credential-required', 'oauth2-or-token'}


def digest(raw):
    return hashlib.sha256(raw).hexdigest()


def encoded(value):
    return json.dumps(value, ensure_ascii=False, sort_keys=True, separators=(',', ':')).encode()


def dump(path, obj):
    path.write_text(json.dumps(obj, ensure_ascii=False, indent=2) + '\n')


def fail(message):
    raise ValueError(message)


def object_fields(value, allowed, name):
    if not isinstance(value, dict) or set(value) - allowed:
        fail('invalid or unknown ' + name + ' fields')


def text(value, maximum, name, required=False):
    if not isinstance(value, str) or len(value.encode('utf-8')) > maximum or any(ord(c) < 32 or ord(c) == 127 for c in value) or (required and not value.strip()):
        fail('invalid ' + name)


def safe_url(value):
    """Syntactic public HTTPS metadata validation only; no DNS or HTTP calls."""
    if not isinstance(value, str) or not value or len(value) > 2048:
        return False
    if re.search(r'%(?![0-9A-Fa-f]{2})', value) or any(c.isspace() or ord(c) < 32 or ord(c) == 127 for c in value) or any(c in value for c in '{}\\?#'):
        return False
    try:
        u = urllib.parse.urlsplit(value)
        if u.scheme != 'https' or not u.hostname or u.username is not None or u.password is not None or u.query or u.fragment:
            return False
        if u.port is not None and not 1 <= u.port <= 65535:
            return False
        host = u.hostname.rstrip('.').lower()
        if not all(re.fullmatch(r'[a-z0-9](?:[a-z0-9-]{0,61}[a-z0-9])?', part) for part in host.split('.')):
            return False
        if '.' not in host or host.endswith(('.local', '.localhost', '.internal')):
            return False
        try:
            if not ipaddress.ip_address(host).is_global:
                return False
        except ValueError:
            pass
        return True
    except (ValueError, TypeError):
        return False


def require_url(value, name):
    if not safe_url(value):
        fail('unsafe or missing ' + name)


def listing_url(directory, value):
    if not safe_url(value):
        return False
    u = urllib.parse.urlsplit(value)
    # Exact first-party authorities; no ports, aliases, encoded paths or roots.
    if directory == 'n8n':
        return u.netloc == 'n8n.io' and re.fullmatch(r'/integrations/[a-z0-9][a-z0-9-]*/?', u.path) is not None
    if directory == 'make':
        return u.netloc == 'www.make.com' and re.fullmatch(r'/(?:en/)?integrations/[a-z0-9][a-z0-9-]*/?', u.path) is not None
    if directory == 'make-verified-mcp':
        return value == MAKE_VERIFIED_MCP_URL
    return False


def timestamp(value):
    text(value, 40, 'membership retrieved_at', True)
    if not re.fullmatch(r'\d{4}-\d\d-\d\dT\d\d:\d\d:\d\d(?:\.\d{1,9})?Z', value):
        fail('membership retrieved_at must be an explicit UTC timestamp')
    try:
        datetime.datetime.fromisoformat(value.replace('Z', '+00:00'))
    except ValueError:
        fail('invalid membership retrieved_at')
    return value


def validate_membership(membership):
    object_fields(membership, {'format', 'policy', 'retrieved_at', 'apps', 'make_verified_source'}, 'membership')
    if membership.get('format') != 'gist.directory.admission/v1' or membership.get('policy') != POLICY:
        fail('unsupported admission policy or format')
    timestamp(membership.get('retrieved_at'))
    apps = membership.get('apps')
    if not isinstance(apps, list) or not 1 <= len(apps) <= MAX_ENTRIES:
        fail('membership apps must be nonempty and bounded')
    admitted = {}
    has_make_verified = False
    for app in apps:
        object_fields(app, {'service_id', 'listings'}, 'membership app')
        service = app.get('service_id')
        if not isinstance(service, str) or len(service) > 200 or not SERVICE.fullmatch(service):
            fail('invalid membership service_id')
        if service in admitted:
            fail('duplicate membership service_id: ' + service)
        listings = app.get('listings')
        if not isinstance(listings, list) or not 1 <= len(listings) <= 8:
            fail('membership listings must be nonempty and bounded')
        seen = set()
        for listing in listings:
            object_fields(listing, {'directory', 'url', 'server_name'}, 'listing')
            directory, url = listing.get('directory'), listing.get('url')
            if not listing_url(directory, url):
                fail('unsafe or non-first-party membership listing')
            if directory == 'make-verified-mcp':
                has_make_verified = True
                text(listing.get('server_name'), 200, 'Make verified server name', True)
            elif 'server_name' in listing:
                fail('server_name is only valid for Make verified MCP listings')
            key = (directory, url)
            if key in seen:
                fail('duplicate membership listing')
            seen.add(key)
        admitted[service] = app
    source = membership.get('make_verified_source')
    if has_make_verified and source is None:
        fail('Make verified MCP memberships require a pinned source inventory')
    if source is not None:
        object_fields(source, {'url', 'retrieved_at', 'source_html_sha256', 'inventory_sha256'}, 'Make verified MCP source')
        if source.get('url') != MAKE_VERIFIED_MCP_URL:
            fail('invalid Make verified MCP source URL')
        timestamp(source.get('retrieved_at'))
        for field in ('source_html_sha256', 'inventory_sha256'):
            if not isinstance(source.get(field), str) or not re.fullmatch(r'[a-f0-9]{64}', source[field]):
                fail('invalid Make verified MCP source digest')
    return admitted


def validate_record(record, admitted):
    object_fields(record, FIELDS, 'curated record')
    service = record.get('service_id')
    if not isinstance(service, str) or service not in admitted:
        fail('curated service lacks approved membership: ' + str(service))
    text(record.get('id'), 200, 'id', True)
    kind = record.get('kind')
    if kind not in ('mcp', 'openapi') or not re.fullmatch(kind + r':[A-Za-z0-9][A-Za-z0-9._/@-]*', record['id']):
        fail('curated identity must be safe and match kind')
    for field, maximum, required in [('title', 200, True), ('description', 3000, False)]:
        text(record.get(field), maximum, field, required)
    text(record.get('version', ''), 200, 'version')
    official = record.get('official')
    if type(official) is not bool:
        fail('official must be boolean')
    if official:
        require_url(record.get('official_evidence_url'), 'vendor evidence URL')
        if 'server_listing_url' in record:
            fail('vendor-official routes must not carry community listing provenance')
    else:
        if kind != 'mcp' or 'official_evidence_url' in record:
            fail('only remote MCP routes may be independently operated')
        require_url(record.get('server_listing_url'), 'MCP server listing URL')
        listing_url_value = record['server_listing_url']
        if listing_url_value != MAKE_VERIFIED_MCP_URL:
            u = urllib.parse.urlsplit(listing_url_value)
            if u.netloc != 'registry.modelcontextprotocol.io' or not u.path.startswith('/v0.1/servers/'):
                fail('independent server must have Make verified MCP or MCP Registry provenance')
        elif not any(item.get('directory') == 'make-verified-mcp' and item.get('server_name') == record.get('title')
                     for item in admitted[service]['listings']):
            fail('Make verified MCP listing must match the candidate server title')
    for field in ('homepage', 'repository'):
        if field in record:
            require_url(record[field], field)
    if 'recommended' in record and not isinstance(record['recommended'], bool):
        fail('recommended must be boolean')
    if record.get('featured', False) is not False:
        fail('imported Featured evidence is forbidden')
    tags = record.get('tags')
    if not isinstance(tags, list) or len(tags) > 20:
        fail('invalid tags')
    for tag in tags:
        text(tag, 64, 'tag', True)
    auth = record.get('auth')
    object_fields(auth, {'type', 'credential_names', 'setup_url'}, 'authentication')
    if auth.get('type') not in AUTH_TYPES:
        fail('invalid authentication type')
    names = auth.get('credential_names', [])
    if not isinstance(names, list) or len(names) > 24 or any(not isinstance(n, str) or not re.fullmatch(r'[A-Za-z_][A-Za-z0-9_.-]{0,127}', n) for n in names):
        fail('credential names only; values are forbidden')
    if 'setup_url' in auth:
        require_url(auth['setup_url'], 'authentication setup URL')
    remotes = record.get('remotes', [])
    if not isinstance(remotes, list) or len(remotes) > 16:
        fail('invalid remotes')
    for remote in remotes:
        object_fields(remote, {'url', 'transport', 'check'}, 'remote')
        require_url(remote.get('url'), 'remote URL')
        if remote.get('transport') not in ('streamable-http', 'sse'):
            fail('unsupported remote transport')
        if 'check' in remote:
            check = remote['check']
            object_fields(check, {'checked_at', 'http_status', 'result'}, 'endpoint check')
            timestamp(check.get('checked_at'))
            status, result = check.get('http_status'), check.get('result')
            valid_status = type(status) is int and (200 <= status < 400 or status in (400, 401, 402, 403, 405))
            valid_result = (result == 'mcp-initialize' and status == 200 and remote.get('transport') == 'streamable-http' or
                            result == 'endpoint-responded' and valid_status and status not in (401, 402, 403) or
                            result == 'authentication-required' and status in (401, 403) or
                            result == 'payment-required' and status == 402)
            if not valid_status or not valid_result:
                fail('invalid endpoint reachability check')
    if kind == 'mcp':
        if not remotes or 'openapi' in record:
            fail('MCP requires a public HTTPS endpoint, not a package or OpenAPI descriptor')
    else:
        if remotes:
            fail('OpenAPI entry must not advertise MCP remotes')
        spec = record.get('openapi')
        object_fields(spec, {'spec_url', 'mint_guide_url'}, 'OpenAPI descriptor')
        require_url(spec.get('spec_url'), 'raw OpenAPI specification URL')
        path = urllib.parse.urlsplit(spec['spec_url']).path.lower()
        host = urllib.parse.urlsplit(spec['spec_url']).hostname
        if host in ('github.com', 'www.github.com') or not (path.endswith(('.json', '.yaml', '.yml')) or path.endswith(('/openapi', '/swagger'))):
            fail('OpenAPI requires a raw specification URL, not an HTML viewer')
        require_url(spec.get('mint_guide_url'), 'Mint guide URL')
    skills = record.get('skills', [])
    if not isinstance(skills, list) or len(skills) > 12:
        fail('invalid skills')
    for skill in skills:
        object_fields(skill, {'id', 'title', 'url', 'status', 'source_digest'}, 'skill')
        for field in ('id', 'title'):
            text(skill.get(field), 200, 'skill ' + field, True)
        if skill.get('status') != 'candidate':
            fail('intake skills must be unqualified candidates')
        require_url(skill.get('url'), 'skill source URL')
        if not isinstance(skill.get('source_digest'), str) or not re.fullmatch(r'sha256:[a-f0-9]{64}', skill['source_digest']):
            fail('skill source must be pinned')


def validate_endpoint_checks(report, curated):
    object_fields(report, {'format', 'checked_at', 'endpoint_set_sha256', 'probe', 'checks', 'failures'}, 'endpoint-check report')
    if report.get('format') != 'gist.mcp-endpoint-check/v1' or report.get('probe') != 'unauthenticated remote HTTP reachability; streamable initialize or bounded SSE GET; no tools/list or tools/call':
        fail('unsupported endpoint-check report')
    timestamp(report.get('checked_at'))
    if report.get('failures') != []:
        fail('endpoint-check report contains failures')
    endpoints = sorted({(remote['url'], remote.get('transport')) for record in curated if record.get('kind') == 'mcp' for remote in record.get('remotes', []) if remote.get('transport') in ('streamable-http', 'sse')})
    if not endpoints:
        fail('no Streamable HTTP endpoints to check')
    if report.get('endpoint_set_sha256') != digest(encoded(endpoints)):
        fail('endpoint-check report does not cover the curated endpoint set')
    raw_checks = report.get('checks')
    if not isinstance(raw_checks, list) or len(raw_checks) != len(endpoints):
        fail('endpoint-check report must contain one row per unique endpoint')
    by_url = {}
    for check in raw_checks:
        object_fields(check, {'url', 'transport', 'method', 'checked_at', 'http_status', 'result'}, 'endpoint check result')
        url = check.get('url')
        require_url(url, 'checked endpoint URL')
        timestamp(check.get('checked_at'))
        status, result, transport, method = check.get('http_status'), check.get('result'), check.get('transport'), check.get('method')
        success = result == 'mcp-initialize' and status == 200 and transport == 'streamable-http'
        response = result == 'endpoint-responded' and type(status) is int and (200 <= status < 400 or status in (400, 405))
        auth = result == 'authentication-required' and status in (401, 403)
        payment = result == 'payment-required' and status == 402
        if (url, transport) in by_url or type(status) is not int or transport not in ('streamable-http', 'sse') or method != ('GET' if transport == 'sse' else 'POST') or not (success or response or auth or payment):
            fail('invalid or duplicate endpoint-check result')
        by_url[(url, transport)] = check
    if sorted(by_url) != endpoints:
        fail('endpoint-check report contains missing or unexpected endpoints')
    for record in curated:
        if record.get('kind') != 'mcp':
            continue
        for remote in record.get('remotes', []):
            expected = by_url.get((remote.get('url'), remote.get('transport')))
            if expected is None or remote.get('check') != {key: expected[key] for key in ('checked_at', 'http_status', 'result')}:
                fail('curated endpoint check does not match its report')
    return report


def build_snapshot(curated, membership, curated_raw=None, membership_raw=None, checks=None, checks_raw=None):
    """Return snapshot and receipt without mutating inputs, reading files or I/O."""
    if not isinstance(curated, list) or not 1 <= len(curated) <= MAX_ENTRIES:
        fail('curated entries must be nonempty and bounded')
    admitted = validate_membership(membership)
    for value, raw in ((curated, curated_raw), (membership, membership_raw), (checks, checks_raw)):
        if raw is not None and (len(raw) > MAX_INPUT_BYTES or load_json(raw) != value):
            fail('input bytes do not match reviewed content')
    if checks is not None:
        validate_endpoint_checks(checks, curated)
    curated_hash = digest(curated_raw if curated_raw is not None else encoded(curated))
    membership_hash = digest(membership_raw if membership_raw is not None else encoded(membership))
    checks_hash = digest(checks_raw if checks_raw is not None else encoded(checks)) if checks is not None else None
    seen = set()
    seen_ids = set()
    entries = []
    for original in curated:
        validate_record(original, admitted)
        service = original['service_id']
        if service in seen:
            fail('duplicate curated service_id: ' + service)
        if original['id'] in seen_ids:
            fail('duplicate curated record id: ' + original['id'])
        seen_ids.add(original['id'])
        seen.add(service)
        record = copy.deepcopy(original)
        record['source'] = {'id': 'gist-curation', 'entry_id': record['id'], 'url': record.get('official_evidence_url', record.get('server_listing_url'))}
        record['recommended'] = record.get('recommended', False)
        record['featured'] = False
        entries.append(record)
    if set(admitted) != seen:
        fail('membership references unknown curated service_id')
    entries.sort(key=lambda r: r['id'])
    stamp = membership['retrieved_at']
    sources = [
        {'id': 'gist-curation', 'url': 'https://github.com/sirerun/gist', 'revision': 'sha256:' + curated_hash, 'sha256': curated_hash, 'retrieved_at': stamp, 'license': 'Apache-2.0 (Gist metadata; vendor rights retained)', 'attribution': 'Gist maintainers; vendor routes and independent server listings are identified separately. Source membership and reachability are not execution qualification.'},
        {'id': 'marketplace-membership', 'url': 'https://github.com/sirerun/gist/blob/main/catalog/directory/intake/approved-membership.json', 'revision': 'sha256:' + membership_hash, 'sha256': membership_hash, 'retrieved_at': stamp, 'attribution': 'Operator-reviewed n8n or Make listing membership; factual source links only, not copied node code or descriptions.'},
    ]
    make_source = membership.get('make_verified_source')
    if make_source is not None:
        inventory_hash = make_source['inventory_sha256']
        sources.append({'id': 'make-verified-remote-mcp-list', 'url': MAKE_VERIFIED_MCP_URL, 'revision': 'sha256:' + make_source['source_html_sha256'], 'sha256': make_source['source_html_sha256'], 'retrieved_at': make_source['retrieved_at'], 'attribution': 'Make verified remote MCP listing extracted into intake inventory SHA-256 ' + inventory_hash + '; source listing is not Gist ownership, safety or workflow qualification.'})
    if checks is not None:
        sources.append({'id': 'remote-endpoint-checks', 'url': 'https://github.com/sirerun/gist/blob/main/catalog/directory/intake/endpoint-checks.json', 'revision': 'sha256:' + checks_hash, 'sha256': checks_hash, 'retrieved_at': checks['checked_at'], 'attribution': 'Unauthenticated remote HTTP reachability checks. 401/403 records an authentication challenge; 402 records payment-required; successful initialize responses separately verify the MCP handshake. No tools/list or tools/call.'})
    revision = 'sha256:' + digest(encoded({'sources': sources, 'entries': entries}))
    snapshot = {'format': 'gist.directory.snapshot/v1', 'revision': revision, 'generated_at': stamp, 'sources': sources, 'entries': entries}
    receipt = {'policy': POLICY, 'curated_input': len(curated), 'membership_apps': len(admitted), 'accepted': len(entries), 'by_kind': {kind: sum(r['kind'] == kind for r in entries) for kind in ('mcp', 'openapi')}, 'curated_recommended': sum(r['recommended'] for r in entries), 'featured': 0, 'duplicate_count': 0, 'duplicates': [], 'rejected_count': 0, 'rejected': [], 'curated_sha256': curated_hash, 'membership_sha256': membership_hash, 'endpoint_checks_sha256': checks_hash, 'make_inventory_sha256': make_source['inventory_sha256'] if make_source else None, 'make_source_html_sha256': make_source['source_html_sha256'] if make_source else None, 'checked_endpoints': len(checks['checks']) if checks is not None else 0, 'network_requests': 0, 'registry_imports': 0, 'revision': revision}
    return snapshot, receipt


def load_json(raw):
    def unique(pairs):
        result = {}
        for key, value in pairs:
            if key in result:
                fail('duplicate JSON key: ' + key)
            result[key] = value
        return result
    return json.loads(raw, object_pairs_hook=unique, parse_constant=lambda _: fail('nonfinite JSON value'))


def read_input(path):
    with path.open('rb') as stream:
        raw = stream.read(MAX_INPUT_BYTES + 1)
    if len(raw) > MAX_INPUT_BYTES:
        fail('input byte budget exceeded')
    return raw


def main():
    parser = argparse.ArgumentParser(description=__doc__)
    parser.add_argument('--curated', type=pathlib.Path, default=HERE / 'curated.json')
    parser.add_argument('--membership', type=pathlib.Path, default=HERE / 'approved-membership.json')
    parser.add_argument('--checks', type=pathlib.Path, default=HERE / 'endpoint-checks.json')
    parser.add_argument('--output', type=pathlib.Path, default=ROOT / 'snapshot.json')
    parser.add_argument('--receipt', type=pathlib.Path, default=HERE / 'receipt.json')
    args = parser.parse_args()
    curated_raw, membership_raw, checks_raw = read_input(args.curated), read_input(args.membership), read_input(args.checks)
    snapshot, receipt = build_snapshot(load_json(curated_raw), load_json(membership_raw), curated_raw, membership_raw, load_json(checks_raw), checks_raw)
    dump(args.output, snapshot)
    dump(args.receipt, receipt)
    print(json.dumps(receipt))


if __name__ == '__main__':
    main()

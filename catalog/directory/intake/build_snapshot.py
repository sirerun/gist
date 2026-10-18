#!/usr/bin/env python3
"""Build the reviewed vendor catalog offline. No registry import or network fetch.

Membership is an operator-reviewed admission record, not vendor authority by
itself. Each curated route independently requires a vendor evidence URL.
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
POLICY = 'n8n-or-make/vendor-official'
MAX_INPUT_BYTES = 4 * 1024 * 1024
MAX_ENTRIES = 10000
SERVICE = re.compile(r'[a-z0-9]+(?:-[a-z0-9]+)*\Z')
FIELDS = {'id', 'kind', 'service_id', 'title', 'description', 'version', 'tags',
          'homepage', 'repository', 'remotes', 'openapi', 'auth', 'skills',
          'official', 'official_evidence_url', 'recommended', 'featured'}
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
    object_fields(membership, {'format', 'policy', 'retrieved_at', 'apps'}, 'membership')
    if membership.get('format') != 'gist.directory.admission/v1' or membership.get('policy') != POLICY:
        fail('unsupported admission policy or format')
    timestamp(membership.get('retrieved_at'))
    apps = membership.get('apps')
    if not isinstance(apps, list) or not 1 <= len(apps) <= MAX_ENTRIES:
        fail('membership apps must be nonempty and bounded')
    admitted = {}
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
            object_fields(listing, {'directory', 'url'}, 'listing')
            directory, url = listing.get('directory'), listing.get('url')
            if not listing_url(directory, url):
                fail('unsafe or non-first-party membership listing')
            key = (directory, url)
            if key in seen:
                fail('duplicate membership listing')
            seen.add(key)
        admitted[service] = app
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
    if record.get('official') is not True:
        fail('vendor-official evidence is required; community routes are not admitted')
    require_url(record.get('official_evidence_url'), 'vendor evidence URL')
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
        object_fields(remote, {'url', 'transport'}, 'remote')
        require_url(remote.get('url'), 'remote URL')
        if remote.get('transport') not in ('streamable-http', 'sse'):
            fail('unsupported remote transport')
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


def build_snapshot(curated, membership, curated_raw=None, membership_raw=None):
    """Return snapshot and receipt without mutating inputs, reading files or I/O."""
    if not isinstance(curated, list) or not 1 <= len(curated) <= MAX_ENTRIES:
        fail('curated entries must be nonempty and bounded')
    admitted = validate_membership(membership)
    for value, raw in ((curated, curated_raw), (membership, membership_raw)):
        if raw is not None and (len(raw) > MAX_INPUT_BYTES or load_json(raw) != value):
            fail('input bytes do not match reviewed content')
    curated_hash = digest(curated_raw if curated_raw is not None else encoded(curated))
    membership_hash = digest(membership_raw if membership_raw is not None else encoded(membership))
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
        record['source'] = {'id': 'gist-curation', 'entry_id': record['id'], 'url': record['official_evidence_url']}
        record['recommended'] = record.get('recommended', False)
        record['featured'] = False
        entries.append(record)
    if set(admitted) != seen:
        fail('membership references unknown curated service_id')
    entries.sort(key=lambda r: r['id'])
    stamp = membership['retrieved_at']
    sources = [
        {'id': 'gist-curation', 'url': 'https://github.com/sirerun/gist', 'revision': 'sha256:' + curated_hash, 'sha256': curated_hash, 'retrieved_at': stamp, 'license': 'Apache-2.0 (Gist metadata; vendor rights retained)', 'attribution': 'Gist maintainers; individually reviewed vendor-official routes. Source membership is not execution qualification.'},
        {'id': 'marketplace-membership', 'url': 'https://github.com/sirerun/gist/blob/main/catalog/directory/intake/approved-membership.json', 'revision': 'sha256:' + membership_hash, 'sha256': membership_hash, 'retrieved_at': stamp, 'attribution': 'Operator-reviewed n8n or Make listing membership; factual source links only, not copied node code or descriptions.'},
    ]
    revision = 'sha256:' + digest(encoded({'sources': sources, 'entries': entries}))
    snapshot = {'format': 'gist.directory.snapshot/v1', 'revision': revision, 'generated_at': stamp, 'sources': sources, 'entries': entries}
    receipt = {'policy': POLICY, 'curated_input': len(curated), 'membership_apps': len(admitted), 'accepted': len(entries), 'by_kind': {kind: sum(r['kind'] == kind for r in entries) for kind in ('mcp', 'openapi')}, 'curated_recommended': sum(r['recommended'] for r in entries), 'featured': 0, 'duplicate_count': 0, 'duplicates': [], 'rejected_count': 0, 'rejected': [], 'curated_sha256': curated_hash, 'membership_sha256': membership_hash, 'network_requests': 0, 'registry_imports': 0, 'revision': revision}
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
    parser.add_argument('--output', type=pathlib.Path, default=ROOT / 'snapshot.json')
    parser.add_argument('--receipt', type=pathlib.Path, default=HERE / 'receipt.json')
    args = parser.parse_args()
    curated_raw, membership_raw = read_input(args.curated), read_input(args.membership)
    snapshot, receipt = build_snapshot(load_json(curated_raw), load_json(membership_raw), curated_raw, membership_raw)
    dump(args.output, snapshot)
    dump(args.receipt, receipt)
    print(json.dumps(receipt))


if __name__ == '__main__':
    main()

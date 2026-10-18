#!/usr/bin/env python3
"""Apply reachable rows from Make's frozen verified remote MCP inventory."""
import argparse
import copy
import hashlib
import json
import pathlib
import re
import urllib.parse

HERE = pathlib.Path(__file__).resolve().parent
MAKE_URL = 'https://apps.make.com/c5ZhYquO55e90hoCJ0rzt'
ALIASES = {
    'asana': ['asana'], 'atlassian': ['jira', 'confluence'],
    'github': ['github'], 'gmail': ['google-gmail'],
    'google calendar': ['google-calendar'], 'google drive': ['google-drive'],
    'hubspot': ['hubspot'], 'linear': ['linear'], 'monday.com': ['monday'],
    'notion': ['notion'], 'sentry': ['sentry'], 'slack': ['slack'],
    'stripe': ['stripe'], 'supabase': ['supabase'],
}


def slug(name):
    value = name.lower().replace('&', ' and ')
    return re.sub(r'-+', '-', re.sub(r'[^a-z0-9]+', '-', value)).strip('-')


def read(path):
    return json.loads(path.read_text())


def write(path, value):
    path.write_text(json.dumps(value, ensure_ascii=False, indent=2) + '\n')


def main():
    parser = argparse.ArgumentParser(description=__doc__)
    parser.add_argument('--inventory', type=pathlib.Path, default=HERE / 'make-verified-mcp.json')
    parser.add_argument('--checks', type=pathlib.Path, required=True)
    parser.add_argument('--curated', type=pathlib.Path, default=HERE / 'curated.json')
    parser.add_argument('--membership', type=pathlib.Path, default=HERE / 'approved-membership.json')
    parser.add_argument('--output-curated', type=pathlib.Path, required=True)
    parser.add_argument('--output-membership', type=pathlib.Path, required=True)
    args = parser.parse_args()

    inventory, checks = read(args.inventory), read(args.checks)
    curated, membership = copy.deepcopy(read(args.curated)), copy.deepcopy(read(args.membership))
    passed = {(row['url'], row['transport']): row for row in checks['checks']}
    known = {row['service_id'] for row in curated}
    existing_urls = {remote['url'] for row in curated for remote in row.get('remotes', [])}
    member_by_service = {row['service_id']: row for row in membership['apps']}
    accepted, aliases, duplicate_urls, failed = 0, 0, 0, 0
    seen_urls = set()

    for source in inventory['rows']:
        if not source['eligible']:
            continue
        url = source['url']
        transport = 'sse' if '/sse' in urllib.parse.urlsplit(url).path.lower() else 'streamable-http'
        result = passed.get((url, transport))
        if not result:
            failed += 1
            continue
        name = source['name']
        mapped = ALIASES.get(name.lower(), [slug(name)])
        targets = [service for service in mapped if service in member_by_service]
        if targets:
            for service in targets:
                listing = {'directory': 'make-verified-mcp', 'url': MAKE_URL, 'server_name': name}
                if listing not in member_by_service[service]['listings']:
                    member_by_service[service]['listings'].append(listing)
                aliases += 1
            continue
        service = mapped[0]
        if service in known or url in existing_urls or url in seen_urls:
            duplicate_urls += 1
            continue
        evidence = {'checked_at': result['checked_at'], 'http_status': result['http_status'], 'result': result['result']}
        official_make = name == 'Make' and service == 'make'
        descriptor = {
            'id': 'mcp:' + service, 'kind': 'mcp', 'service_id': service,
            'title': name,
            'description': ('Remote MCP endpoint listed by Make as a verified server. The operator is not verified by Gist. Review its account permissions, credential flow, tool behavior and terms before connecting. This reachability check is not an authenticated session or workflow qualification.'),
            'tags': [], 'remotes': [{'url': url, 'transport': transport, 'check': evidence}],
            'auth': {'type': 'unknown'}, 'recommended': False, 'featured': False,
            'official': official_make,
        }
        if official_make:
            descriptor.update(description='Official Make MCP server. Users authorize Make through their own MCP client; this endpoint check does not create an account connection or execute a scenario.', homepage='https://developers.make.com/mcp-server', official_evidence_url='https://developers.make.com/mcp-server')
        else:
            descriptor['server_listing_url'] = MAKE_URL
        curated.append(descriptor)
        member = member_by_service.get(service)
        listing = {'directory': 'make-verified-mcp', 'url': MAKE_URL, 'server_name': name}
        if member is None:
            member = {'service_id': service, 'listings': [listing]}
            membership['apps'].append(member)
            member_by_service[service] = member
        elif listing not in member['listings']:
            member['listings'].append(listing)
        known.add(service)
        seen_urls.add(url)
        accepted += 1

    membership['policy'] = 'n8n-or-make-verified/reachable-remote-mcp'
    membership['retrieved_at'] = inventory['retrieved_at']
    membership['make_verified_source'] = {
        'url': MAKE_URL,
        'retrieved_at': inventory['retrieved_at'],
        'source_html_sha256': inventory['source_html_sha256'],
        'inventory_sha256': hashlib.sha256(args.inventory.read_bytes()).hexdigest(),
    }
    args.output_curated.parent.mkdir(parents=True, exist_ok=True)
    args.output_membership.parent.mkdir(parents=True, exist_ok=True)
    write(args.output_curated, curated)
    write(args.output_membership, membership)
    print(json.dumps({'accepted_new': accepted, 'linked_existing': aliases,
                      'duplicates': duplicate_urls, 'without_reachable_check': failed,
                      'curated_entries': len(curated), 'membership_apps': len(membership['apps']),
                      'make_source_html_sha256': inventory['source_html_sha256']}))


if __name__ == '__main__':
    main()

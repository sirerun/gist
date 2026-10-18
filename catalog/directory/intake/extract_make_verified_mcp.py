#!/usr/bin/env python3
"""Extract Make's published verified remote MCP server table from saved HTML."""
import argparse
import datetime
from html.parser import HTMLParser
import hashlib
import json
import pathlib
import re
import urllib.parse

SOURCE_URL = 'https://apps.make.com/c5ZhYquO55e90hoCJ0rzt'


class Tables(HTMLParser):
    def __init__(self):
        super().__init__()
        self.rows, self.row, self.cell = [], None, None

    def handle_starttag(self, tag, attrs):
        if tag == 'tr':
            self.row = []
        elif tag in ('td', 'th'):
            self.cell = []

    def handle_data(self, data):
        if self.cell is not None:
            self.cell.append(data)

    def handle_endtag(self, tag):
        if tag in ('td', 'th') and self.cell is not None:
            self.row.append(' '.join(' '.join(self.cell).split()))
            self.cell = None
        elif tag == 'tr' and self.row:
            self.rows.append(self.row)
            self.row = None


def main():
    parser = argparse.ArgumentParser(description=__doc__)
    parser.add_argument('--html', type=pathlib.Path, required=True)
    parser.add_argument('--output', type=pathlib.Path, required=True)
    args = parser.parse_args()
    raw = args.html.read_bytes()
    tables = Tables()
    tables.feed(raw.decode('utf-8', 'replace'))
    rows = []
    inside = False
    seen = set()
    for row in tables.rows:
        if row[:2] == ['Server name', 'Server URL']:
            inside = True
            continue
        if inside and len(row) >= 2:
            name, url = row[0].strip(), row[1].strip()
            if not name or name.lower().startswith('remote '):
                inside = False
                continue
            if not re.fullmatch(r'https?://[^\s<>]+', url):
                rows.append({'name': name, 'url': url, 'eligible': False, 'reason': 'no literal remote URL'})
                continue
            parsed = urllib.parse.urlsplit(url)
            eligible = parsed.scheme == 'https' and '<' not in url and '>' not in url and not parsed.username and not parsed.password and not parsed.query and not parsed.fragment
            reason = '' if eligible else 'HTTPS route must be literal and contain no credentials, query or fragment'
            key = (name, url)
            if key not in seen:
                rows.append({'name': name, 'url': url, 'eligible': eligible, **({'reason': reason} if reason else {})})
                seen.add(key)
    if not rows:
        parser.error('no Make verified remote MCP server table found in saved HTML')
    result = {'format': 'gist.make-verified-mcp-intake/v1',
              'source_url': SOURCE_URL,
              'retrieved_at': datetime.datetime.now(datetime.timezone.utc).replace(microsecond=0).isoformat().replace('+00:00', 'Z'),
              'source_html_sha256': hashlib.sha256(raw).hexdigest(),
              'rows': rows}
    args.output.write_text(json.dumps(result, ensure_ascii=False, indent=2) + '\n')
    print(json.dumps({'rows': len(rows), 'eligible': sum(row['eligible'] for row in rows), 'source_html_sha256': result['source_html_sha256']}))


if __name__ == '__main__':
    main()

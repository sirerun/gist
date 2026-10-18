#!/usr/bin/env python3
"""Fetch Make's paginated integration directory and create original local glyphs.

The API's logo and href fields are deliberately discarded. Each record gets a
simple, original initials tile generated from its display name and slug.
"""
from __future__ import annotations

import argparse
import colorsys
import hashlib
import html
import json
import os
import pathlib
import re
import sys
import time
import urllib.parse
import urllib.request


API = "https://www.make.com/ih-api/integrations"
REFERER = "https://www.make.com/integrations"
PAGE_SIZE = 30
USER_AGENT = "Gist-directory-import/1.0 (+https://gist.sire.run)"


def request_page(offset: int, timeout: float = 20.0) -> dict:
    params = urllib.parse.urlencode({
        "query": "",
        "showVerified": "true",
        "showCommunity": "true",
        "pg[limit]": PAGE_SIZE,
        "pg[offset]": offset,
        "pg[sortDir]": "desc",
        "pg[sortBy]": "popular",
    })
    request = urllib.request.Request(
        f"{API}?{params}",
        headers={"Accept": "application/json", "Referer": REFERER, "User-Agent": USER_AGENT},
    )
    with urllib.request.urlopen(request, timeout=timeout) as response:
        if response.status != 200:
            raise RuntimeError(f"directory API returned HTTP {response.status} at offset {offset}")
        payload = json.load(response)
    data = payload.get("data")
    if payload.get("success") is not True or not isinstance(data, dict):
        raise RuntimeError(f"unexpected API envelope at offset {offset}")
    if not isinstance(data.get("items"), list) or not isinstance(data.get("total"), int):
        raise RuntimeError(f"missing items/total at offset {offset}")
    if not isinstance(data.get("hasMore"), bool):
        raise RuntimeError(f"missing hasMore at offset {offset}")
    return data


def initials(name: str) -> str:
    words = re.findall(r"[\w]+", name, flags=re.UNICODE)
    if not words:
        return "?"
    if len(words) == 1:
        return words[0][:2].upper()
    return (words[0][0] + words[1][0]).upper()


def glyph_color(slug: str) -> tuple[str, str]:
    # Generate a restrained but varied palette from the stable slug. This is
    # intentionally independent of Make's logo and theme fields.
    digest = hashlib.sha256(slug.encode("utf-8")).digest()
    hue = int.from_bytes(digest[:2], "big") / 65535
    rgb = colorsys.hls_to_rgb(hue, 0.28, 0.52)
    background = "#" + "".join(f"{round(channel * 255):02x}" for channel in rgb)
    return background, "#ffffff"


def make_glyph(name: str, slug: str) -> str:
    background, foreground = glyph_color(slug)
    label = html.escape(initials(name))
    return (
        '<svg xmlns="http://www.w3.org/2000/svg" width="96" height="96" '
        'viewBox="0 0 96 96" role="img" aria-label="Generated service glyph">'
        f'<rect x="3" y="3" width="90" height="90" rx="23" fill="{background}"/>'
        '<path d="M70 17a9 9 0 1 1-18 0 9 9 0 0 1 18 0Z" fill="#fff" opacity=".2"/>'
        f'<text x="48" y="56" text-anchor="middle" dominant-baseline="middle" '
        f'fill="{foreground}" font-family="system-ui,-apple-system,sans-serif" '
        f'font-size="30" font-weight="650" letter-spacing="-1">{label}</text>'
        '</svg>\n'
    )


def sanitize_item(item: dict, icon_dir: pathlib.Path, icon_prefix: str = "icons") -> dict:
    slug = item.get("slug")
    name = item.get("name")
    if (not isinstance(slug, str) or not slug.strip() or len(slug) > 256
            or any(ord(char) < 32 or ord(char) == 127 for char in slug)):
        raise RuntimeError(f"invalid integration slug: {slug!r}")
    if not isinstance(name, str) or not name.strip():
        raise RuntimeError(f"missing integration name for slug {slug}")
    categories = item.get("categories", [])
    if not isinstance(categories, list) or any(
        not isinstance(category, dict)
        or not isinstance(category.get("slug"), str)
        or not isinstance(category.get("name"), str)
        for category in categories
    ):
        raise RuntimeError(f"invalid categories for {slug}")
    # Some legitimate slugs contain punctuation. A digest gives each SVG a
    # portable, traversal-safe filename without rewriting the catalog slug.
    icon_filename = hashlib.sha256(slug.encode("utf-8")).hexdigest()[:24] + ".svg"
    icon_path = pathlib.PurePosixPath(icon_prefix) / icon_filename
    icon_dir.mkdir(parents=True, exist_ok=True)
    (icon_dir / icon_filename).write_text(make_glyph(name, slug), encoding="utf-8")
    return {
        "id": item.get("id"),
        "slug": slug,
        "name": name,
        "categories": [{"slug": c["slug"], "name": c["name"]} for c in categories],
        "listing_type": item.get("type"),
        "popularity": item.get("popularity"),
        "added_at": item.get("addedAt"),
        "icon_path": str(icon_path),
    }


def fetch_all(delay: float = 0.1, progress=None, fetch_page=request_page) -> list[dict]:
    offset = 0
    total = None
    records = []
    seen = set()
    while True:
        page = fetch_page(offset)
        page_total = page["total"]
        if total is None:
            total = page_total
        elif total != page_total:
            raise RuntimeError(f"directory total changed during pagination: {total} -> {page_total}")
        items = page["items"]
        if not items and offset < total:
            raise RuntimeError(f"empty page before reported final offset ({offset} < {total})")
        for item in items:
            slug = item.get("slug") if isinstance(item, dict) else None
            if not isinstance(slug, str) or slug in seen:
                raise RuntimeError(f"missing or duplicate integration slug: {slug!r}")
            seen.add(slug)
            records.append(item)
        if len(records) > total:
            raise RuntimeError(f"received {len(records)} records; API reports only {total}")
        if progress:
            progress(min(offset + len(items), total), total)
        if not page["hasMore"]:
            if len(records) != total:
                raise RuntimeError(f"pagination ended at {len(records)} records; API reports {total}")
            return records
        if len(records) >= total:
            raise RuntimeError(f"API reports hasMore after all {total} reported records were received")
        if not items:
            raise RuntimeError(f"API reports hasMore with no items at offset {offset}")
        offset += len(items)
        if delay:
            time.sleep(delay)


def main() -> int:
    parser = argparse.ArgumentParser(description=__doc__)
    parser.add_argument("--output", type=pathlib.Path, required=True, help="JSON output path")
    parser.add_argument("--icons-dir", type=pathlib.Path, required=True, help="directory for original SVG glyphs")
    parser.add_argument("--delay", type=float, default=0.1, help="seconds between pages (default: 0.1)")
    parser.add_argument("--timeout", type=float, default=20.0, help="per-page request timeout in seconds")
    args = parser.parse_args()
    if args.delay < 0 or args.timeout <= 0:
        parser.error("--delay must be nonnegative and --timeout must be positive")

    def fetch(offset):
        return request_page(offset, timeout=args.timeout)

    try:
        items = fetch_all(args.delay, lambda done, total: print(f"Fetched {done}/{total}", file=sys.stderr), fetch)
        args.icons_dir.mkdir(parents=True, exist_ok=True)
        icon_prefix = pathlib.PurePosixPath(os.path.relpath(
            args.icons_dir.resolve(), args.output.parent.resolve()
        ))
        records = [sanitize_item(item, args.icons_dir, str(icon_prefix)) for item in items]
        args.output.parent.mkdir(parents=True, exist_ok=True)
        payload = {
            "format": "gist.make-directory-import/v1",
            "scope": "integration directory metadata; no remote MCP endpoint is implied",
            "count": len(records),
            "items": records,
        }
        tmp = args.output.with_suffix(args.output.suffix + ".tmp")
        tmp.write_text(json.dumps(payload, ensure_ascii=False, indent=2) + "\n", encoding="utf-8")
        tmp.replace(args.output)
    except Exception as error:  # present a concise CLI error without a traceback
        print(f"fetch failed: {error}", file=sys.stderr)
        return 1
    print(f"Wrote {len(records)} records and original SVG glyphs to {args.icons_dir}")
    return 0


if __name__ == "__main__":
    raise SystemExit(main())

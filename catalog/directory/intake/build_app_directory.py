#!/usr/bin/env python3
"""Build a local app inventory and brand-colored icons from a cached intake.

Source metadata and original images stay in --cache; only sanitized app metadata
and self-contained icons are written to the public output paths. No listing is
promoted into an MCP connection descriptor by this importer.
"""
import argparse
from concurrent.futures import ThreadPoolExecutor, as_completed
import hashlib
import html
import json
import pathlib
import re
import subprocess
import sys
import threading
import urllib.parse
import urllib.request
import xml.etree.ElementTree as ET

from fetch_make_integrations import fetch_all, request_page, USER_AGENT, REFERER
from render_brand_icon import render, color, SVG


def write_manifest(records, icons_dir):
    package = pathlib.Path(__file__).resolve().parent.parent
    aliases = json.loads((package / "app-aliases.json").read_text())
    snapshot = json.loads((package / "snapshot.json").read_text())
    icons = {r["slug"]: "/assets/apps/" + r["icon_path"] for r in records}
    manifest = {}
    for entry in snapshot["entries"]:
        service = entry["service_id"]
        if service in icons:
            manifest[service] = icons[service]
            continue
        for slug in sorted(aliases):
            if aliases[slug] == service and slug in icons:
                manifest[service] = icons[slug]
                break
    (icons_dir.parent / "app-icons.js").write_text("globalThis.GIST_APP_ICONS=Object.freeze(" + json.dumps(manifest, sort_keys=True, separators=(",", ":")) + ");\n")


def fetch_json(url):
    request = urllib.request.Request(url, headers={"User-Agent": USER_AGENT, "Referer": REFERER})
    with urllib.request.urlopen(request, timeout=30) as response:
        return json.load(response)


def description(text, name, categories):
    # Extract factual capability labels into original concise copy rather than
    # republishing editorial descriptions or platform/account promotions.
    curated = {
        "gmail": "Read, search and organize email, threads and labels in Gmail.",
        "google sheets": "Read and organize spreadsheet data, rows and worksheets in Google Sheets.",
        "google calendar": "Work with calendars, events and scheduling in Google Calendar.",
        "google drive": "Find and organize files and shared documents in Google Drive.",
        "github": "Work with code repositories, issues and pull requests in GitHub.",
        "slack": "Work with team messages, channels and conversations in Slack.",
        "notion": "Organize pages, documents and databases in Notion.",
        "openai (chatgpt, whisper)": "Generate text, transcribe audio and work with OpenAI models.",
        "hubspot crm": "Manage contacts, companies, deals and customer relationships in HubSpot CRM.",
        "airtable": "Organize structured records, tables and shared workflows in Airtable.",
    }
    if name.casefold() in curated:
        return curated[name.casefold()]
    text = html.unescape(re.sub(r"<[^>]*>", " ", text or ""))
    concepts = []
    for pattern, label in [
        (r"\b(?:email|inbox|mailbox)", "email workflows"),
        (r"\b(?:calendar|scheduling|appointments?)\b", "scheduling"),
        (r"\b(?:contacts?|crm|leads?)\b", "customer records"),
        (r"\b(?:documents?|files?|attachments?)\b", "files and documents"),
        (r"\b(?:tasks?|projects?)\b", "project workflows"),
        (r"\b(?:invoices?|payments?|accounting)\b", "financial workflows"),
        (r"\b(?:orders?|ecommerce|e-commerce)\b", "commerce workflows"),
        (r"\b(?:messages?|chats?|conversations?)\b", "messaging"),
        (r"\b(?:forms?|surveys?|responses?)\b", "forms and responses"),
        (r"\b(?:images?|photos?|videos?)\b", "visual content"),
        (r"\b(?:audio|transcribe|transcription)\b", "audio workflows"),
        (r"\b(?:databases?|tables?|spreadsheets?)\b", "structured data"),
        (r"\b(?:analytics|reports?|metrics)\b", "reporting"),
        (r"\b(?:search|research)\b", "search and discovery"),
    ]:
        if re.search(pattern, text, re.I):
            concepts.append(label)
        if len(concepts) == 3:
            break
    if concepts:
        return f"Use {name} for " + ", ".join(concepts) + "."
    names = ", ".join(c["name"].lower() for c in categories[:2] if c.get("slug") != "app-families")
    return f"Explore {names} workflows with {name}." if names else f"Explore {name} integrations."


def raster_fallback(source, output, background, name):
    # Opaque or multicolor artwork is kept intact, rather than converted into
    # an inaccurate silhouette. Local PNGs work under the existing strict CSP.
    subprocess.run(["magick", str(source), "-resize", "256x256", "-background", background, "-alpha", "remove", "-alpha", "off", str(output)], check=True, capture_output=True, timeout=30)


def build(a):
    a.cache.mkdir(parents=True, exist_ok=True)
    a.icons_dir.mkdir(parents=True, exist_ok=True)
    raw = a.cache / "raw-apps.json"
    if raw.exists():
        items = json.loads(raw.read_text())
    else:
        items = fetch_all(delay=.1, progress=lambda n,total: print(f"Inventory {n}/{total}", flush=True))
        raw.write_text(json.dumps(items, ensure_ascii=False))
    details_file = a.cache / "app-details.json"
    details = json.loads(details_file.read_text()) if details_file.exists() else {}
    missing = [r["slug"] for r in items if r["slug"] not in details]
    batches = [missing[n:n+20] for n in range(0, len(missing), 20)]
    def get_details(slugs):
        data = fetch_json("https://www.make.com/ih-api/apps?" + urllib.parse.urlencode([("slug",s) for s in slugs]))
        if data.get("success") is not True:
            raise RuntimeError("invalid app detail response")
        return data["data"]["items"]
    with ThreadPoolExecutor(max_workers=a.workers) as pool:
        for future in as_completed([pool.submit(get_details, batch) for batch in batches]):
            for row in future.result():
                details[row["slug"]] = row
            details_file.write_text(json.dumps(details, ensure_ascii=False))
    # Community entries may have no separate editorial detail record. Cache
    # that absence so reruns do not repeatedly request unchanged missing data.
    for item in items:
        details.setdefault(item["slug"], {})
    details_file.write_text(json.dumps(details, ensure_ascii=False))
    records, modes = [], {}
    blocked = threading.Event()
    failures = {}
    previous_file = a.cache / "icon-failures.json"
    previous_failures = json.loads(previous_file.read_text()) if previous_file.exists() else {}
    def app_record(item, output):
        categories = item.get("categories", [])
        return {"slug": item["slug"], "name": item["name"], "description": description(details.get(item["slug"],{}).get("description"),item["name"],categories), "categories": categories, "icon_path": output.name, "background_color": color(item["theme"])}
    def icon(item):
        slug, name = item["slug"], item["name"]
        background = color(item["theme"])
        if "403" in previous_failures.get(slug, ""):
            raise RuntimeError("source access denied (cached HTTP 403)")
        key = hashlib.sha256(slug.encode()).hexdigest()[:24]
        url = item["logo"]
        parsed = urllib.parse.urlsplit(url)
        if parsed.scheme != "https" or parsed.netloc != "cdn.make.com" or not parsed.path.startswith("/cms/") or not parsed.path.endswith(".avif") or parsed.query or parsed.fragment:
            raise ValueError(f"invalid source icon URL for {slug}")
        source_key = hashlib.sha256(url.encode()).hexdigest()[:24]
        source = a.cache / (source_key + ".avif")
        output = a.icons_dir / (key + ".svg")
        if source.exists():
            freshness = max(source.stat().st_mtime, pathlib.Path(__file__).stat().st_mtime, pathlib.Path(__file__).with_name("render_brand_icon.py").stat().st_mtime)
            candidates = [p for p in (output, output.with_suffix(".png")) if p.exists() and p.stat().st_mtime >= freshness]
            if candidates:
                cached = max(candidates, key=lambda p:p.stat().st_mtime)
                if cached.suffix == ".png" or ("<title>" in cached.read_text() and "<path" in cached.read_text() and "<image" not in cached.read_text()):
                    return app_record(item, cached), "vector" if cached.suffix == ".svg" else "raster"
        if not source.exists():
            if blocked.is_set():
                raise RuntimeError("source downloads stopped after access/rate-limit response")
            request = urllib.request.Request(url, headers={"User-Agent": USER_AGENT})
            with urllib.request.urlopen(request, timeout=30) as response:
                payload = response.read(2*1024*1024+1)
            if len(payload) > 2*1024*1024:
                raise RuntimeError(f"oversized icon: {slug}")
            source.write_bytes(payload)
        # Only a white-on-transparent logo is safe to trace as a single mask.
        # ImageMagick's fx checks visible pixels; transparent RGB is irrelevant.
        check = subprocess.run(["magick", str(source), "-fx", "a>0.01 && (r<0.95 || g<0.95 || b<0.95) ? 1 : 0", "-alpha", "off", "-format", "%[fx:mean]", "info:"], check=True, capture_output=True, text=True, timeout=30).stdout
        opaque = subprocess.run(["magick", str(source), "-format", "%[opaque]", "info:"], check=True, capture_output=True, text=True, timeout=30).stdout.lower() == "true"
        # Test each visible pixel: channel averages can hide multicolor artwork.
        monochrome = float(check) < .001
        mode = "vector"
        if opaque or not monochrome:
            output = output.with_suffix(".png")
            raster_fallback(source, output, background, name)
            mode = "raster"
        else:
            render(source, output, background, title=name)
        return app_record(item, output), mode
    def safe_icon(item):
        try:
            return icon(item)
        except Exception as error:
            if getattr(error, "code", None) == 429:
                blocked.set()
            failures[item["slug"]] = str(error)
            print(f"Icon fallback {item['slug']}: {error}", file=sys.stderr, flush=True)
            output = a.icons_dir / (hashlib.sha256(item["slug"].encode()).hexdigest()[:24] + ".svg")
            root = ET.Element(f"{{{SVG}}}svg", {"viewBox":"0 0 96 96", "width":"96", "height":"96", "role":"img"})
            ET.SubElement(root, f"{{{SVG}}}title").text = item["name"]
            ET.SubElement(root, f"{{{SVG}}}rect", {"width":"96", "height":"96", "rx":"19.2", "fill":color(item["theme"])})
            ET.SubElement(root, f"{{{SVG}}}text", {"x":"48", "y":"62", "text-anchor":"middle", "font-size":"44", "font-family":"sans-serif", "fill":"#ffffff"}).text = item["name"][0].upper()
            output.write_text(ET.tostring(root, encoding="unicode") + "\n")
            return app_record(item, output), "monogram-fallback"
    with ThreadPoolExecutor(max_workers=a.workers) as pool:
        jobs = {pool.submit(safe_icon, item): item["slug"] for item in items}
        for n, future in enumerate(as_completed(jobs), 1):
            record, mode = future.result()
            records.append(record)
            modes[record["slug"]] = mode
            if n % 100 == 0 or n == len(items):
                print(f"Icons {n}/{len(items)}", flush=True)
    order = {r["slug"]: n for n,r in enumerate(items)}
    records.sort(key=lambda r:order[r["slug"]])
    a.output.parent.mkdir(parents=True, exist_ok=True)
    a.output.write_text(json.dumps({"format": "gist.app-directory/v1", "items": records}, ensure_ascii=False, indent=2) + "\n")
    write_manifest(records, a.icons_dir)
    (a.cache / "icon-modes.json").write_text(json.dumps(modes, indent=2))
    (a.cache / "icon-failures.json").write_text(json.dumps(failures, indent=2))
    print(f"Built {len(records)} apps; {sum(m=='vector' for m in modes.values())} vector icons, {sum(m=='raster' for m in modes.values())} preserved raster foregrounds.")


if __name__ == "__main__":
    p = argparse.ArgumentParser(description=__doc__)
    p.add_argument("--cache", type=pathlib.Path, required=True)
    p.add_argument("--output", type=pathlib.Path, required=True)
    p.add_argument("--icons-dir", type=pathlib.Path, required=True)
    p.add_argument("--workers", type=int, default=3, choices=range(1,5))
    build(p.parse_args())

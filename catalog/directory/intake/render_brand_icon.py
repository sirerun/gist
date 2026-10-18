#!/usr/bin/env python3
"""Trace a transparent monochrome logo and compose its brand background locally.

Requires ImageMagick (with AVIF decoding) and VTracer 0.6.5. Background colors
are explicit brand metadata, never inferred from the transparent image.
"""
import argparse
import pathlib
import re
import subprocess
import tempfile
import xml.etree.ElementTree as ET

SVG = "http://www.w3.org/2000/svg"
ET.register_namespace("", SVG)


def color(value):
    if not isinstance(value, str) or not re.fullmatch(r"#[0-9a-fA-F]{6}", value):
        raise ValueError("colors must be six-digit hex values such as #f14336")
    return value.lower()


def compose(traced, background, foreground="#ffffff", title="", radius=20):
    """Compose only locally generated vector geometry; discard other SVG nodes."""
    background, foreground = color(background), color(foreground)
    root = ET.fromstring(traced)
    width, height = float(root.attrib["width"]), float(root.attrib["height"])
    if not 0 < width <= 4096 or not 0 < height <= 4096 or not 0 <= radius <= 50:
        raise ValueError("invalid logo dimensions or corner radius")
    out = ET.Element(f"{{{SVG}}}svg", {"viewBox": f"0 0 {width:g} {height:g}", "width": "96", "height": "96", "role": "img"})
    ET.SubElement(out, f"{{{SVG}}}title").text = title
    ET.SubElement(out, f"{{{SVG}}}rect", {"width": f"{width:g}", "height": f"{height:g}", "rx": f"{min(width,height)*radius/100:g}", "fill": background})
    count = 0
    for path in root.findall(f"{{{SVG}}}path"):
        if not path.get("d", "").strip():
            continue
        attrs = {k: v for k, v in path.attrib.items() if k in {"d", "transform", "fill-rule"}}
        attrs["fill"] = foreground
        ET.SubElement(out, f"{{{SVG}}}path", attrs)
        count += 1
    if not count:
        raise ValueError("tracer produced no foreground geometry")
    return ET.tostring(out, encoding="unicode") + "\n"


def render(source, output, background, foreground="#ffffff", title="", radius=20):
    color(background)
    color(foreground)
    # Keep transient masks outside embedded/public asset directories.
    with tempfile.TemporaryDirectory(prefix="gist-icon-", dir=source.parent) as tmp:
        mask, traced = pathlib.Path(tmp) / "mask.png", pathlib.Path(tmp) / "trace.svg"
        # Trace coverage only. The alpha mask excludes arbitrary RGB data stored
        # beneath transparent pixels, which creates speckles in color tracing.
        subprocess.run(["magick", str(source.resolve()), "-alpha", "extract", "-auto-level", "-threshold", "50%", "-negate", str(mask)], check=True, capture_output=True, timeout=30)
        subprocess.run(["vtracer", "--input", str(mask), "--output", str(traced), "--colormode", "bw", "--mode", "spline", "--filter_speckle", "8", "--path_precision", "2"], check=True, capture_output=True, timeout=30)
        output.write_text(compose(traced.read_text(), background, foreground, title, radius))


if __name__ == "__main__":
    p = argparse.ArgumentParser(description=__doc__)
    p.add_argument("--input", type=pathlib.Path, required=True)
    p.add_argument("--output", type=pathlib.Path, required=True)
    p.add_argument("--background", required=True)
    p.add_argument("--foreground", default="#ffffff")
    p.add_argument("--title", default="")
    p.add_argument("--radius", type=float, default=20, help="corner radius as a percentage (0–50)")
    a = p.parse_args()
    a.output.parent.mkdir(parents=True, exist_ok=True)
    render(a.input, a.output, a.background, a.foreground, a.title, a.radius)

"""Tests for complete, paginated Make directory intake and generated glyphs."""
import importlib.util
import os
import pathlib
import tempfile
import unittest

HERE = pathlib.Path(__file__).resolve().parent
spec = importlib.util.spec_from_file_location("fetch_make_integrations", HERE / "fetch_make_integrations.py")
fetcher = importlib.util.module_from_spec(spec)
spec.loader.exec_module(fetcher)


def item(slug):
    return {
        "id": f"app:{slug}", "slug": slug, "name": slug.title(),
        "categories": [{"slug": "productivity", "name": "Productivity"}],
        "type": "app", "popularity": 1, "addedAt": "2026-01-01T00:00:00Z",
        "logo": "https://cdn.make.com/should-not-be-used.avif",
        "theme": "#010203", "href": "/integrations/should-not-be-used",
    }


class FetchMakeIntegrationsTests(unittest.TestCase):
    def test_walks_offsets_through_last_page_and_sanitizes_assets(self):
        pages = {
            0: {"items": [item("alpha"), item("beta")], "total": 3, "hasMore": True},
            2: {"items": [item("gamma")], "total": 3, "hasMore": False},
        }
        offsets = []

        def get_page(offset):
            offsets.append(offset)
            return pages[offset]

        fetched = fetcher.fetch_all(delay=0, fetch_page=get_page)
        self.assertEqual(offsets, [0, 2])
        with tempfile.TemporaryDirectory() as temp:
            root = pathlib.Path(temp)
            output_parent = root / "export"
            output_parent.mkdir()
            icon_dir = output_parent / "custom-glyphs"
            prefix = pathlib.PurePosixPath(os.path.relpath(icon_dir, output_parent))
            records = [fetcher.sanitize_item(value, icon_dir, str(prefix)) for value in fetched]
            self.assertEqual(len(records), 3)
            self.assertRegex(records[0]["icon_path"], r"^custom-glyphs/[0-9a-f]{24}\.svg$")
            self.assertFalse(any("make.com" in str(record) for record in records))
            svg = (output_parent / records[0]["icon_path"]).read_text()
            self.assertIn("aria-label=\"Generated service glyph\"", svg)
            self.assertNotIn("make.com", svg)
            self.assertNotIn("#010203", svg)

    def test_rejects_incomplete_or_duplicate_pages(self):
        with self.assertRaisesRegex(RuntimeError, "pagination ended"):
            fetcher.fetch_all(delay=0, fetch_page=lambda _: {"items": [item("alpha")], "total": 2, "hasMore": False})
        with self.assertRaisesRegex(RuntimeError, "duplicate"):
            fetcher.fetch_all(delay=0, fetch_page=lambda _: {"items": [item("alpha"), item("alpha")], "total": 2, "hasMore": False})
        with self.assertRaisesRegex(RuntimeError, "hasMore after all"):
            fetcher.fetch_all(delay=0, fetch_page=lambda _: {"items": [item("alpha")], "total": 1, "hasMore": True})
        with self.assertRaisesRegex(RuntimeError, "reports only 1"):
            fetcher.fetch_all(delay=0, fetch_page=lambda _: {"items": [item("alpha"), item("beta")], "total": 1, "hasMore": True})

    def test_glyph_is_deterministic_and_slug_specific(self):
        self.assertEqual(fetcher.make_glyph("Acme", "acme"), fetcher.make_glyph("Acme", "acme"))
        self.assertNotEqual(fetcher.glyph_color("acme"), fetcher.glyph_color("other"))


if __name__ == "__main__":
    unittest.main()

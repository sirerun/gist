import unittest
import xml.etree.ElementTree as ET

from render_brand_icon import compose, color
from build_app_directory import description


class BrandIconTests(unittest.TestCase):
    def test_background_is_separate_from_foreground_and_geometry_is_preserved(self):
        source = '<svg xmlns="http://www.w3.org/2000/svg" width="256" height="256"><path d="M1 1L2 2Z" transform="translate(3 4)" fill="#000"/><path d=""/><script>alert(1)</script></svg>'
        root = ET.fromstring(compose(source, "#F14336", "#ffffff", "Gmail & mail"))
        ns = {"s":"http://www.w3.org/2000/svg"}
        self.assertEqual(root.find("s:rect",ns).get("fill"),"#f14336")
        paths = root.findall("s:path", ns)
        self.assertEqual(len(paths),1)
        self.assertEqual(paths[0].get("fill"),"#ffffff")
        self.assertEqual(paths[0].get("transform"),"translate(3 4)")
        self.assertEqual(root.find("s:title",ns).text,"Gmail & mail")
        self.assertNotIn("script", ET.tostring(root,encoding="unicode"))

    def test_rejects_invalid_colors_and_empty_traces(self):
        for value in ["red", "#fff", 'url(https://example.com)', '#123456;stroke:red']:
            with self.assertRaises(ValueError): color(value)
        with self.assertRaises(ValueError):
            compose('<svg width="256" height="256"/>', '#123456')

    def test_descriptions_remove_platform_and_account_promotions(self):
        text="Create workflows with Make. Search documents and organize files. Create your free account today."
        self.assertIn("files and documents",description(text,"Files",[]))
        self.assertNotIn("Search documents and organize files",description(text,"Files",[]))
        self.assertNotIn("Make",description("Connect Make to your tools.","Files",[{"name":"Storage"}]))


if __name__ == '__main__': unittest.main()

package webui

import (
	"encoding/xml"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
)

func TestAppPagesAreReadableWithoutJavaScript(t *testing.T) {
	h := Handler()
	for _, p := range []string{"/apps/google-email", "/apps/openai-gpt-3", "/apps/airtable", "/apps/openai-gpt-3/airtable", "/connectors/mcp:google-gmail"} {
		t.Run(p, func(t *testing.T) {
			w := httptest.NewRecorder()
			h.ServeHTTP(w, httptest.NewRequest("GET", p, nil))
			if w.Code != 200 {
				t.Fatalf("status%d %s", w.Code, w.Body.String())
			}
			body := w.Body.String()
			for _, want := range []string{"<h1", `rel="canonical"`, "Gist"} {
				if !strings.Contains(body, want) {
					t.Errorf("missing%s", want)
				}
			}
			if strings.Contains(body, "Loading integration details") || strings.Contains(body, "listed by Make") {
				t.Fatal("unrendered or private copy")
			}
			if strings.HasPrefix(p, "/apps/") && (!strings.Contains(body, "https://zati.run") || !strings.Contains(body, "workflow")) {
				t.Fatal("missing useful app content")
			}
			head := httptest.NewRecorder()
			h.ServeHTTP(head, httptest.NewRequest("HEAD", p, nil))
			if head.Code != 200 || head.Body.Len() != 0 {
				t.Fatal("bad HEAD")
			}
		})
	}
}
func TestDetailRedirectsAndUnknownPaths(t *testing.T) {
	h := Handler()
	for from, to := range map[string]string{"/library/server?app=google-email": "/apps/google-email", "/library/server?id=mcp%3Agoogle-gmail": "/connectors/mcp:google-gmail"} {
		w := httptest.NewRecorder()
		h.ServeHTTP(w, httptest.NewRequest("GET", from, nil))
		if w.Code != 301 || w.Header().Get("Location") != to {
			t.Fatalf("redirect%s got%d %s", from, w.Code, w.Header().Get("Location"))
		}
	}
	for _, p := range []string{"/apps/not-an-app", "/apps/airtable/unknown", "/connectors/not-a-connector", "/library/server?app=not-an-app", "/apps/", "/apps/airtable/"} {
		if !Handles(p[:strings.Index(p+"?", "?")]) {
			t.Fatalf("unclaimed%s", p)
		}
		w := httptest.NewRecorder()
		h.ServeHTTP(w, httptest.NewRequest("GET", p, nil))
		if w.Code != 404 {
			t.Errorf("%s status%d", p, w.Code)
		}
	}
	w := httptest.NewRecorder()
	h.ServeHTTP(w, httptest.NewRequest("POST", "/apps/airtable", nil))
	if w.Code != http.StatusMethodNotAllowed {
		t.Fatal("mutation accepted")
	}
}
func TestSitemapContainsCanonicalPublicInventory(t *testing.T) {
	h := Handler()
	w := httptest.NewRecorder()
	h.ServeHTTP(w, httptest.NewRequest("GET", "/sitemap.xml", nil))
	if w.Code != 200 {
		t.Fatal(w.Code)
	}
	var doc struct {
		URLs []struct {
			Loc string `xml:"loc"`
		} `xml:"url"`
	}
	if err := xml.Unmarshal(w.Body.Bytes(), &doc); err != nil {
		t.Fatal(err)
	}
	seen := map[string]bool{}
	for _, u := range doc.URLs {
		if seen[u.Loc] {
			t.Fatal("duplicate", u.Loc)
		}
		seen[u.Loc] = true
		if strings.ContainsAny(u.Loc, "?#") || strings.Contains(u.Loc, "/workspace") {
			t.Fatal("noncanonical URL", u.Loc)
		}
	}
	if len(doc.URLs) != 3380+127+11 || !seen[publicOrigin+"/apps/google-email"] {
		t.Fatalf("unexpected sitemap size%d", len(doc.URLs))
	}
}

func TestTypedLibraryRoutes(t *testing.T) {
	h := Handler()
	for _, kind := range []string{"mcp", "openapi"} {
		path := "/library/" + kind
		if !Handles(path) {
			t.Fatal("unclaimed", path)
		}
		w := httptest.NewRecorder()
		h.ServeHTTP(w, httptest.NewRequest("GET", path, nil))
		if w.Code != 200 || !strings.Contains(w.Body.String(), `rel="canonical" href="`+publicOrigin+path+`"`) {
			t.Fatal("missing typed page", w.Code)
		}
		if strings.Contains(w.Body.String(), "bookmark-filter") {
			t.Fatal("obsolete filter")
		}
		w = httptest.NewRecorder()
		h.ServeHTTP(w, httptest.NewRequest("GET", "/library?kind="+kind+"&q=mail", nil))
		if w.Code != 301 || w.Header().Get("Location") != path+"?q=mail" {
			t.Fatal("bad old redirect", w.Code, w.Header().Get("Location"))
		}
	}
}

func TestAppDetailLogosUseSharedBrandFrames(t *testing.T) {
	for _, path := range []string{"/apps/google-email", "/apps/openai-gpt-3/airtable"} {
		w := httptest.NewRecorder()
		Handler().ServeHTTP(w, httptest.NewRequest("GET", path, nil))
		body := w.Body.String()
		if w.Code != 200 || !strings.Contains(body, `class="directory-logo-frame app-profile-icon`) || !strings.Contains(body, `style="background-color:#`) {
			t.Fatalf("missing padded brand frame %s", path)
		}
		if path == "/apps/google-email" && (!strings.Contains(body, `directory-logo-frame related-app-logo`) || !strings.Contains(body, `class="related-app-name"`)) {
			t.Fatal("related logos missing shared frame")
		}
	}
}

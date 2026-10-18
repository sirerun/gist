// Package webui serves the public directory and workspace website.
package webui

import (
	"bytes"
	"embed"
	"html/template"
	"io/fs"
	"net/http"
	"net/url"
	"path"
	"strings"
)

//go:embed templates/*.html assets
var files embed.FS

var pages = template.Must(template.ParseFS(files, "templates/*.html"))

// Handles reports whether path belongs to the public website. It deliberately
// leaves authentication, API, OAuth, and MCP routes to the application router.
func Handles(p string) bool {
	switch p {
	case "/", "/library", "/library/mcp", "/library/openapi", "/library/item", "/library/server", "/workspace", "/workspace/servers", "/developers", "/docs", "/signin", "/favicon.ico":
		return true
	}
	return strings.HasPrefix(p, "/assets/") || strings.HasPrefix(p, "/apps/") || strings.HasPrefix(p, "/connectors/") || p == "/sitemap.xml" || p == "/robots.txt"
}

// Handler returns an independent, immutable static website handler.
func Handler() http.Handler {
	assets, err := fs.Sub(files, "assets")
	if err != nil {
		panic(err)
	}
	apps, connectors := appInventory()
	static := http.StripPrefix("/assets/", http.FileServer(http.FS(assets)))
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("X-Content-Type-Options", "nosniff")
		w.Header().Set("Referrer-Policy", "strict-origin-when-cross-origin")
		w.Header().Set("X-Frame-Options", "DENY")
		w.Header().Set("Content-Security-Policy", "default-src 'none'; style-src 'self'; script-src 'self'; img-src 'self'; connect-src 'self'; font-src 'self'; form-action 'self'; base-uri 'none'; frame-ancestors 'none'; object-src 'none'")
		w.Header().Set("Permissions-Policy", "camera=(), microphone=(), geolocation=()")
		w.Header().Set("Cache-Control", "no-cache")
		if r.Method != http.MethodGet && r.Method != http.MethodHead {
			w.Header().Set("Allow", "GET, HEAD")
			http.Error(w, "Method not allowed", http.StatusMethodNotAllowed)
			return
		}
		if strings.HasPrefix(r.URL.Path, "/assets/") {
			// Do not permit directory listings or path aliases.
			p := strings.TrimPrefix(r.URL.Path, "/assets/")
			if p == "" || path.Clean(p) != p || strings.HasPrefix(p, ".") {
				http.NotFound(w, r)
				return
			}
			info, e := fs.Stat(assets, p)
			if e != nil || info.IsDir() {
				http.NotFound(w, r)
				return
			}
			static.ServeHTTP(w, r)
			return
		}
		if r.URL.Path == "/favicon.ico" {
			http.Redirect(w, r, "/assets/brand/gist-app-icon-green.svg", http.StatusFound)
			return
		}

		libraryKind := ""
		if r.URL.Path == "/library/mcp" {
			libraryKind = "mcp"
		} else if r.URL.Path == "/library/openapi" {
			libraryKind = "openapi"
		}
		if r.URL.Path == "/library" || libraryKind != "" {
			q := r.URL.Query()
			if (q.Get("kind") == "mcp" || q.Get("kind") == "openapi") && libraryKind == "" {
				libraryKind = q.Get("kind")
			}
			if _, exists := q["kind"]; exists && libraryKind != "" {
				q.Del("kind")
				target := "/library/" + libraryKind
				if len(q) > 0 {
					target += "?" + q.Encode()
				}
				http.Redirect(w, r, target, http.StatusMovedPermanently)
				return
			}
		}
		if r.URL.Path == "/robots.txt" {
			w.Header().Set("Content-Type", "text/plain; charset=utf-8")
			if r.Method != http.MethodHead {
				_, _ = w.Write([]byte("User-agent: *\nAllow: /\nDisallow: /workspace\nDisallow: /app/\nDisallow: /auth/\nDisallow: /api/\nSitemap: " + publicOrigin + "/sitemap.xml\n"))
			}
			return
		}
		if r.URL.Path == "/sitemap.xml" {
			serveSitemap(w, r, apps, connectors)
			return
		}
		if strings.HasPrefix(r.URL.Path, "/apps/") {
			serveAppPage(w, r, apps, connectors)
			return
		}
		if r.URL.Path == "/library/server" {
			if slug := r.URL.Query().Get("app"); slug != "" {
				if _, ok := apps.Get(slug); !ok {
					http.NotFound(w, r)
					return
				}
				http.Redirect(w, r, appPath(slug), http.StatusMovedPermanently)
				return
			}
			if id := r.URL.Query().Get("id"); id != "" {
				if _, ok := connectors.Get(id); !ok {
					http.NotFound(w, r)
					return
				}
				http.Redirect(w, r, connectorPath(id), http.StatusMovedPermanently)
				return
			}
		}
		if strings.HasPrefix(r.URL.Path, "/connectors/") {
			id, err := url.PathUnescape(strings.TrimPrefix(r.URL.EscapedPath(), "/connectors/"))
			if err != nil {
				http.NotFound(w, r)
				return
			}
			c, ok := connectors.Get(id)
			if !ok {
				http.NotFound(w, r)
				return
			}
			renderPage(w, r, "connector-preview", c, siteData{Title: c.Title + " connection details · Gist", Page: "directory-detail", Description: c.Description, Canonical: publicOrigin + connectorPath(id)})
			return
		}
		page := "home"
		title := "Gist — The right context, ready to use"
		switch r.URL.Path {
		case "/":
		case "/library", "/library/mcp", "/library/openapi":
			page, title = "directory", "The library · Gist"
			if libraryKind == "mcp" {
				title = "MCP servers · Gist"
			} else if libraryKind == "openapi" {
				title = "OpenAPI integrations · Gist"
			}
		case "/library/server":
			page, title = "directory-detail", "Integration details · Gist"
		case "/workspace":
			page, title = "workspace", "Your workspace · Gist"
		case "/workspace/servers":
			page, title = "private-directory", "Private integrations · Gist"
		case "/library/item":
			page, title = "artifact", "Artifact details · Gist"
		case "/developers":
			page, title = "developers", "Build with Gist · Gist"
		case "/docs":
			page, title = "docs", "Getting started · Gist"
		case "/signin":
			page, title = "signin", "Sign in · Gist"
		default:
			http.NotFound(w, r)
			return
		}
		var body bytes.Buffer
		if err := pages.ExecuteTemplate(&body, page, struct{ Kind string }{libraryKind}); err != nil {
			http.Error(w, "Page unavailable", http.StatusInternalServerError)
			return
		}
		var out bytes.Buffer
		data := siteData{Title: title, Page: page, Description: "Discover reusable skills and tools. Bring the right context to Zati and other AI clients with Gist.", Body: template.HTML(body.String())}
		if page == "directory" {
			data.Canonical = publicOrigin + r.URL.Path
		}
		if err := pages.ExecuteTemplate(&out, "layout", data); err != nil {
			http.Error(w, "Page unavailable", http.StatusInternalServerError)
			return
		}
		w.Header().Set("Content-Type", "text/html; charset=utf-8")
		if r.Method != http.MethodHead {
			_, _ = w.Write(out.Bytes())
		}
	})
}

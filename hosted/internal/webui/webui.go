// Package webui serves the public directory and workspace website.
package webui

import (
	"bytes"
	"embed"
	"html/template"
	"io/fs"
	"net/http"
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
	case "/", "/library", "/library/item", "/library/server", "/workspace", "/workspace/servers", "/developers", "/docs", "/signin", "/favicon.ico":
		return true
	}
	return strings.HasPrefix(p, "/assets/")
}

// Handler returns an independent, immutable static website handler.
func Handler() http.Handler {
	assets, err := fs.Sub(files, "assets")
	if err != nil {
		panic(err)
	}
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
		page := "home"
		title := "Gist — The right context, ready to use"
		switch r.URL.Path {
		case "/":
		case "/library":
			page, title = "directory", "The library · Gist"
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
		if err := pages.ExecuteTemplate(&body, page, nil); err != nil {
			http.Error(w, "Page unavailable", http.StatusInternalServerError)
			return
		}
		var out bytes.Buffer
		data := struct {
			Title, Page string
			Body        template.HTML
		}{title, page, template.HTML(body.String())}
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

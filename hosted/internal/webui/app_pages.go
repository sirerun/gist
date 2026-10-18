package webui

import (
	"bytes"
	"encoding/xml"
	"fmt"
	"html/template"
	"net/http"
	"net/url"
	"strings"

	"github.com/sirerun/gist/catalog/directory"
)

const publicOrigin = "https://gist.sire.run"

type workflowIdea struct{ Title, Description, Prompt string }
type appLink struct{ Name, URL, Icon, BackgroundColor string }
type appPage struct {
	App         directory.App
	Categories  []directory.AppCategory
	Connections []appLink
	Related     []appLink
	Ideas       []workflowIdea
	Teams       []workflowIdea
	Pair        *appLink
	Canonical   string
}
type siteData struct {
	Title, Page, Description, Canonical string
	Body                                template.HTML
	StructuredData                      template.HTML
}

func appPath(slug string) string     { return "/apps/" + url.PathEscape(slug) }
func connectorPath(id string) string { return "/connectors/" + url.PathEscape(id) }

func appInventory() (*directory.AppIndex, *directory.Index) {
	connectors, err := directory.DefaultPublic()
	if err != nil {
		panic(err)
	}
	apps, err := directory.DefaultApps(connectors)
	if err != nil {
		panic(err)
	}
	return apps, connectors
}

func renderPage(w http.ResponseWriter, r *http.Request, name string, view any, data siteData) {
	renderPageStatus(w, r, name, view, data, 200)
}
func renderPageStatus(w http.ResponseWriter, r *http.Request, name string, view any, data siteData, status int) {
	w.Header().Set("Vary", "Accept")
	if wantsMarkdown(r.Header.Get("Accept")) {
		if body, ok := publicMarkdown(name, view); ok {
			writeMarkdown(w, r, status, body)
			return
		}
	}
	var body, out bytes.Buffer
	if err := pages.ExecuteTemplate(&body, name, view); err != nil {
		http.Error(w, "Page unavailable", 500)
		return
	}
	data.Body = template.HTML(body.String())
	if err := pages.ExecuteTemplate(&out, "layout", data); err != nil {
		http.Error(w, "Page unavailable", 500)
		return
	}
	w.Header().Set("Content-Type", "text/html; charset=utf-8")
	w.WriteHeader(status)
	if r.Method != http.MethodHead {
		_, _ = w.Write(out.Bytes())
	}
}

func appView(app directory.App, apps *directory.AppIndex, connectors *directory.Index) appPage {
	v := appPage{App: app, Canonical: publicOrigin + appPath(app.Slug)}
	for _, c := range app.Categories {
		if c.Slug != "app-families" {
			v.Categories = append(v.Categories, c)
		}
	}
	for _, id := range app.ConnectorIDs {
		if c, ok := connectors.Get(id); ok {
			v.Connections = append(v.Connections, appLink{Name: c.Title, URL: connectorPath(id)})
		}
	}
	v.Ideas = ideasFor(app)
	switch app.Slug {
	case "google-email", "google-calendar", "slack", "airtable":
		v.Teams = []workflowIdea{{Title: "Property operations", Description: "Plan a request-to-follow-up workflow: gather the request, prepare the hand-off and ask for review before contacting a tenant or vendor."}, {Title: "Recruiting", Description: "Prepare a candidate brief or scheduling plan from selected information, with review before contacting candidates."}, {Title: "Software teams", Description: "Collect project updates and open questions into a working brief, with one owner for the next step."}}
	case "github":
		v.Teams = []workflowIdea{{Title: "Software teams", Description: "Give a review agent a bounded pull request, a clear checklist and a human decision before posting or merging."}}
	}
	if len(v.Categories) > 0 {
		result, _ := apps.Search(directory.AppQuery{Category: v.Categories[0].Slug, Limit: 8})
		for _, related := range result.Items {
			if related.Slug != app.Slug && len(v.Related) < 6 {
				v.Related = append(v.Related, appLink{Name: related.Name, URL: appPath(related.Slug), Icon: related.IconURL, BackgroundColor: related.BackgroundColor})
			}
		}
	}
	if app.Slug == "openai-gpt-3" || app.Slug == "airtable" {
		v.Pair = &appLink{Name: "OpenAI + Airtable: turn records into a reviewable content pipeline", URL: "/apps/openai-gpt-3/airtable"}
	}
	return v
}

func ideasFor(a directory.App) []workflowIdea {
	switch a.Slug {
	case "google-email":
		return []workflowIdea{{"Start the day with an inbox briefing", "Summarize important messages, identify requests and collect the threads that need your attention.", "Prepare an inbox briefing from unread email. Include source links and suggested next steps. Do not send or change messages."}, {"Prepare a reply for review", "Use a selected conversation to draft a clear response before you decide what to send.", "Read this conversation and draft a reply for my review. Ask me before sending anything."}}
	case "airtable":
		return []workflowIdea{{"Turn records into a working brief", "Review selected records and surface missing information, priorities and proposed next steps.", "Review the records I select in Airtable. Summarize priorities and missing information. Propose changes without writing them."}, {"Build a content review queue", "Combine structured topics with a model to prepare drafts that your team can inspect.", "Use my selected Airtable topics to prepare content drafts and a proposed review queue. Show the plan before changing any records."}}
	case "openai-gpt-3":
		return []workflowIdea{{"Make structured information easier to use", "Turn selected source material into a concise summary, classification or first draft.", "Summarize the material I provide, cite the source, and list anything uncertain."}, {"Draft from a shared brief", "Use an agreed brief to prepare content for human review.", "Create a draft from this brief. Separate source facts from suggestions and leave publication to me."}}
	case "slack":
		return []workflowIdea{{"Catch up on a project", "Review selected conversations and collect decisions, open questions and next steps.", "Summarize the Slack conversations I select. Link to decisions and list open questions. Do not post messages."}}
	case "github":
		return []workflowIdea{{"Prepare a pull request review", "Inspect an authorized repository change and prepare a review grounded in the diff.", "Review this GitHub pull request. Explain the change, flag concrete risks and suggest checks. Do not submit comments or merge."}}
	}
	return []workflowIdea{{"Prepare a focused working brief", "Bring selected information into an agent task with a clear goal and a result you can review.", fmt.Sprintf("Help me plan a task using %s. Ask what information and access are available, then propose a small workflow and a success check before taking action.", a.Name)}}
}

func serveAppPage(w http.ResponseWriter, r *http.Request, apps *directory.AppIndex, connectors *directory.Index) {
	slug := strings.TrimPrefix(r.URL.Path, "/apps/")
	if slug == "openai-gpt-3/airtable" {
		a, _ := apps.Get("openai-gpt-3")
		b, _ := apps.Get("airtable")
		v := struct {
			First, Second directory.App
			Canonical     string
		}{a, b, publicOrigin + "/apps/openai-gpt-3/airtable"}
		renderPage(w, r, "app-pair", v, siteData{Title: "OpenAI + Airtable workflow ideas · Gist", Page: "app-detail", Description: "Plan a content review workflow with OpenAI, Airtable and Zati. Explore the steps, required connections and review points.", Canonical: v.Canonical})
		return
	}
	a, ok := apps.Get(slug)
	if !ok {
		NotFound(w, r)
		return
	}
	if r.URL.Path != appPath(a.Slug) && r.URL.EscapedPath() != appPath(a.Slug) {
		NotFound(w, r)
		return
	}
	v := appView(a, apps, connectors)
	renderPage(w, r, "app-detail", v, siteData{Title: a.Name + " integrations and workflow ideas · Gist", Page: "app-detail", Description: a.Description + " Explore workflow ideas, connection options and ways to use it with Zati.", Canonical: v.Canonical})
}

func serveSitemap(w http.ResponseWriter, r *http.Request, apps *directory.AppIndex, connectors *directory.Index) {
	type entry struct {
		Loc string `xml:"loc"`
	}
	doc := struct {
		XMLName xml.Name `xml:"urlset"`
		XMLNS   string   `xml:"xmlns,attr"`
		URLs    []entry  `xml:"url"`
	}{XMLNS: "http://www.sitemaps.org/schemas/sitemap/0.9"}
	for _, p := range []string{"/developers", "/docs", "/about", "/contact", "/privacy", "/pricing", "/cli"} {
		doc.URLs = append(doc.URLs, entry{publicOrigin + p})
	}
	doc.URLs = append(doc.URLs, entry{publicOrigin + "/library"}, entry{publicOrigin + "/library/mcp"}, entry{publicOrigin + "/library/openapi"}, entry{publicOrigin + "/apps/openai-gpt-3/airtable"})
	for offset := 0; ; offset += 100 {
		result, _ := apps.Search(directory.AppQuery{Offset: offset, Limit: 100})
		for _, a := range result.Items {
			doc.URLs = append(doc.URLs, entry{publicOrigin + appPath(a.Slug)})
		}
		if result.NextOffset == nil {
			break
		}
	}
	for cursor := ""; ; {
		result, _ := connectors.Search(directory.Query{Cursor: cursor, Limit: 100})
		for _, c := range result.Items {
			doc.URLs = append(doc.URLs, entry{publicOrigin + connectorPath(c.ID)})
		}
		if result.NextCursor == "" {
			break
		}
		cursor = result.NextCursor
	}
	raw, err := xml.Marshal(doc)
	if err != nil {
		http.Error(w, "Sitemap unavailable", 500)
		return
	}
	w.Header().Set("Content-Type", "application/xml; charset=utf-8")
	if r.Method != http.MethodHead {
		_, _ = w.Write(append([]byte(xml.Header), raw...))
	}
}

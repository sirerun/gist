package directory

import (
	_ "embed"
	"encoding/json"
	"fmt"
	"path"
	"regexp"
	"sort"
	"strings"
	"sync"
)

//go:embed apps.json
var defaultApps []byte

// App describes a discoverable application. Only ConnectorIDs identify admitted
// connection descriptors; an application listing itself grants no connection.
type App struct {
	Slug            string        `json:"slug"`
	Name            string        `json:"name"`
	Description     string        `json:"description"`
	Categories      []AppCategory `json:"categories"`
	IconURL         string        `json:"icon_url,omitempty"`
	BackgroundColor string        `json:"background_color,omitempty"`
	ConnectorIDs    []string      `json:"connector_ids,omitempty"`
}
type AppCategory struct {
	Slug string `json:"slug"`
	Name string `json:"name"`
}
type AppQuery struct {
	Text, Category string
	// PriorityConnectorIDs affects ordering only; callers retain ownership of saved state.
	PriorityConnectorIDs []string
	Offset, Limit        int
}
type AppResult struct {
	Categories []AppCategory `json:"categories"`
	Items      []App         `json:"items"`
	Total      int           `json:"total"`
	NextOffset *int          `json:"next_offset,omitempty"`
}
type AppIndex struct {
	items  []App
	bySlug map[string]int
}

var appIdentity = regexp.MustCompile(`^[a-zA-Z0-9][a-zA-Z0-9._#&()-]{0,255}$`)
var appSlug = regexp.MustCompile(`^[a-z0-9][a-z0-9._-]{0,199}$`)
var appColor = regexp.MustCompile(`^#[0-9a-fA-F]{6}$`)

func DefaultApps(connectors *Index) (*AppIndex, error) { return ParseApps(defaultApps, connectors) }

// ParseApps loads the application inventory and joins only exact service names or
// explicitly maintained aliases to actual connector records.
func ParseApps(raw []byte, connectors *Index) (*AppIndex, error) {
	if len(raw) > MaxSnapshotBytes {
		return nil, fmt.Errorf("app inventory too large")
	}
	var data struct {
		Format string `json:"format"`
		Items  []struct {
			Slug            string        `json:"slug"`
			Name            string        `json:"name"`
			Description     string        `json:"description"`
			Categories      []AppCategory `json:"categories"`
			IconPath        string        `json:"icon_path"`
			BackgroundColor string        `json:"background_color"`
		} `json:"items"`
	}
	if err := json.Unmarshal(raw, &data); err != nil {
		return nil, err
	}
	if data.Format != "gist.app-directory/v1" || len(data.Items) > MaxEntries {
		return nil, fmt.Errorf("invalid app inventory")
	}
	services := map[string][]string{}
	if connectors != nil {
		for _, id := range connectors.ids {
			r := connectors.records[id]
			services[r.ServiceID] = append(services[r.ServiceID], r.ID)
		}
	}

	idx := &AppIndex{items: make([]App, 0, len(data.Items)), bySlug: map[string]int{}}
	for _, r := range data.Items {
		if !appIdentity.MatchString(r.Slug) || r.Name == "" || !bounded(r.Name, 300) || !bounded(r.Description, 10000) || len(r.Categories) > 100 {
			return nil, fmt.Errorf("invalid app %q", r.Slug)
		}
		if _, ok := idx.bySlug[r.Slug]; ok {
			return nil, fmt.Errorf("duplicate app %q", r.Slug)
		}
		for _, c := range r.Categories {
			if !appSlug.MatchString(c.Slug) || c.Name == "" || !bounded(c.Name, 300) {
				return nil, fmt.Errorf("invalid category")
			}
		}
		icon := ""
		if r.IconPath != "" {
			if path.Base(r.IconPath) != r.IconPath || !appSlug.MatchString(r.IconPath) || (!strings.HasSuffix(r.IconPath, ".svg") && !strings.HasSuffix(r.IconPath, ".png")) {
				return nil, fmt.Errorf("invalid app icon")
			}
			icon = "/assets/apps/" + r.IconPath
		}
		if r.BackgroundColor != "" && !appColor.MatchString(r.BackgroundColor) {
			return nil, fmt.Errorf("invalid app color")
		}
		service := r.Slug
		if a, ok := appConnectorAliases[r.Slug]; ok {
			service = a
		}
		ids := append([]string(nil), services[service]...)
		sort.Strings(ids)
		idx.bySlug[r.Slug] = len(idx.items)
		idx.items = append(idx.items, App{Slug: r.Slug, Name: r.Name, Description: r.Description, Categories: r.Categories, IconURL: icon, BackgroundColor: r.BackgroundColor, ConnectorIDs: ids})
	}
	return idx, nil
}
func cloneApp(a App) App {
	a.Categories = append([]AppCategory{}, a.Categories...)
	a.ConnectorIDs = append([]string(nil), a.ConnectorIDs...)
	return a
}
func (i *AppIndex) Get(slug string) (App, bool) {
	if i == nil {
		return App{}, false
	}
	n, ok := i.bySlug[slug]
	if !ok {
		return App{}, false
	}
	return cloneApp(i.items[n]), true
}
func (i *AppIndex) Search(q AppQuery) (AppResult, error) {
	if i == nil {
		return AppResult{}, fmt.Errorf("app directory unavailable")
	}
	if q.Limit == 0 {
		q.Limit = 24
	}
	if len(q.PriorityConnectorIDs) > MaxEntries {
		return AppResult{}, fmt.Errorf("too many priority IDs")
	}
	if q.Limit < 1 || q.Limit > 100 || q.Offset < 0 || q.Offset > MaxEntries || !bounded(q.Text, 512) || !bounded(q.Category, 200) {
		return AppResult{}, fmt.Errorf("invalid app query")
	}
	result := AppResult{Items: []App{}, Categories: []AppCategory{}}
	categoryNames := map[string]string{}
	for _, app := range i.items {
		for _, category := range app.Categories {
			categoryNames[category.Slug] = category.Name
		}
	}
	for slug, name := range categoryNames {
		result.Categories = append(result.Categories, AppCategory{Slug: slug, Name: name})
	}
	sort.Slice(result.Categories, func(a, b int) bool { return result.Categories[a].Name < result.Categories[b].Name })
	ordered := i.items
	if len(q.PriorityConnectorIDs) > 0 {
		saved := map[string]bool{}
		for _, id := range q.PriorityConnectorIDs {
			saved[id] = true
		}
		pinned := func(a App) bool {
			for _, id := range a.ConnectorIDs {
				if saved[id] {
					return true
				}
			}
			return false
		}
		ordered = make([]App, 0, len(i.items))
		for _, a := range i.items {
			if pinned(a) {
				ordered = append(ordered, a)
			}
		}
		for _, a := range i.items {
			if !pinned(a) {
				ordered = append(ordered, a)
			}
		}
	}
	text := strings.ToLower(q.Text)
	for _, a := range ordered {
		if text != "" && !strings.Contains(strings.ToLower(a.Name+" "+a.Slug+" "+a.Description), text) {
			continue
		}
		if q.Category != "" {
			found := false
			for _, c := range a.Categories {
				if c.Slug == q.Category {
					found = true
					break
				}
			}
			if !found {
				continue
			}
		}
		if result.Total >= q.Offset && len(result.Items) < q.Limit {
			result.Items = append(result.Items, cloneApp(a))
		}
		result.Total++
	}
	if q.Offset+len(result.Items) < result.Total {
		next := q.Offset + len(result.Items)
		result.NextOffset = &next
	}
	return result, nil
}

var cachedPublicApps = sync.OnceValues(func() (*AppIndex, error) { return DefaultApps(nil) })

// appConnectorAliases are reviewed exact product identities, not fuzzy matches.
//
//go:embed app-aliases.json
var appConnectorAliasJSON []byte

var appConnectorAliases = func() map[string]string {
	var aliases map[string]string
	if err := json.Unmarshal(appConnectorAliasJSON, &aliases); err != nil {
		panic(fmt.Sprintf("invalid embedded app aliases: %v", err))
	}
	for slug, service := range aliases {
		if !appIdentity.MatchString(slug) || !appSlug.MatchString(service) {
			panic(fmt.Sprintf("invalid embedded app alias %q", slug))
		}
	}
	return aliases
}()

func publicAppForService(serviceID string) (App, bool) {
	apps, err := cachedPublicApps()
	if err != nil {
		return App{}, false
	}
	if app, ok := apps.Get(serviceID); ok {
		return app, true
	}
	// Sort aliases so duplicate identities pick a stable canonical application
	// (for example tavily before tavily-community).
	slugs := make([]string, 0, len(appConnectorAliases))
	for slug, service := range appConnectorAliases {
		if service == serviceID {
			slugs = append(slugs, slug)
		}
	}
	sort.Strings(slugs)
	for _, slug := range slugs {
		if app, ok := apps.Get(slug); ok {
			return app, true
		}
	}
	return App{}, false
}

// PublicIconForService returns a local brand icon for an exact application
// identity or a reviewed connector alias. Unknown services have no inferred icon.
func PublicIconForService(serviceID string) string {
	app, ok := publicAppForService(serviceID)
	if !ok {
		return ""
	}
	return app.IconURL
}

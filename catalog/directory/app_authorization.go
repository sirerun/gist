package directory

import (
	"bytes"
	_ "embed"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"sync"
)

//go:embed app-authorization.json
var appAuthorizationJSON []byte

type AppAuthorization struct {
	Format  string                `json:"format"`
	App     string                `json:"app"`
	Name    string                `json:"name"`
	Status  string                `json:"status"`
	Routes  []AuthorizationGuide  `json:"routes"`
	Notes   []string              `json:"notes"`
	Sources []AuthorizationSource `json:"sources,omitempty"`
}

type appAuthorizationEntry struct {
	Slug    string                `json:"slug"`
	Status  string                `json:"status,omitempty"`
	Routes  []AuthorizationGuide  `json:"routes"`
	Notes   []string              `json:"notes,omitempty"`
	Sources []AuthorizationSource `json:"sources,omitempty"`
}

var appAuthorizationOnce sync.Once
var appAuthorizationGuides map[string]appAuthorizationEntry
var appAuthorizationErr error

func parseAppAuthorization(raw []byte) (map[string]appAuthorizationEntry, error) {
	if len(raw) > 16<<20 {
		return nil, errors.New("app authorization data too large")
	}
	var entries []appAuthorizationEntry
	dec := json.NewDecoder(bytes.NewReader(raw))
	dec.DisallowUnknownFields()
	if err := dec.Decode(&entries); err != nil {
		return nil, err
	}
	if dec.Decode(new(any)) != io.EOF {
		return nil, errors.New("trailing app authorization data")
	}
	out := map[string]appAuthorizationEntry{}
	for _, e := range entries {
		if !appIdentity.MatchString(e.Slug) || len(e.Routes) > 8 || len(e.Notes) > 16 || len(e.Sources) > 12 {
			return nil, errors.New("invalid app authorization entry")
		}
		if _, exists := out[e.Slug]; exists {
			return nil, errors.New("duplicate app authorization entry")
		}
		switch e.Status {
		case "", "documented":
			if len(e.Routes) == 0 {
				return nil, errors.New("documented app missing routes")
			}
		case "needs_research", "not_applicable":
			if len(e.Routes) > 0 || len(e.Notes) == 0 {
				return nil, errors.New("invalid unresolved app")
			}
			if e.Status == "not_applicable" && len(e.Sources) == 0 {
				return nil, errors.New("platform classification needs evidence")
			}
		default:
			return nil, errors.New("invalid app coverage status")
		}
		for _, note := range e.Notes {
			if note == "" || !bounded(note, 2000) {
				return nil, errors.New("invalid app note")
			}
		}
		for _, source := range e.Sources {
			if !safeSourceURL(source.URL) {
				return nil, errors.New("invalid app evidence URL")
			}
			if !sourceDate(source.CheckedAt) {
				return nil, errors.New("invalid app evidence date")
			}
		}
		body, _ := json.Marshal(e.Routes)
		if _, err := parseAuthorization(body); err != nil {
			return nil, fmt.Errorf("app %q: %w", e.Slug, err)
		}
		out[e.Slug] = e
	}
	return out, nil
}

// AuthorizationForApp preserves the original strict v1 response. Optional v2
// inputs/protocols are only exposed when a consumer explicitly selects version2.
func AuthorizationForApp(a App, index *Index) (AppAuthorization, error) {
	result, err := AuthorizationForAppV2(a, index)
	if err != nil {
		return result, err
	}
	result.Format = "gist.directory.app-authorization/v1"
	result.Sources = nil
	routes := make([]AuthorizationGuide, 0, len(result.Routes))
	hidden := false
	for _, g := range result.Routes {
		if g.Format == AuthorizationFormat {
			routes = append(routes, g)
		} else {
			hidden = true
		}
	}
	result.Routes = routes
	result.Status = appAuthorizationStatus(routes)
	if hidden {
		result.Notes = append(result.Notes, "Additional provider-specific endpoint inputs or protocols are available with version=2.")
	}
	return result, nil
}

func appAuthorizationStatus(routes []AuthorizationGuide) string {
	if len(routes) == 0 {
		return "no_verified_route"
	}
	documented := 0
	for _, g := range routes {
		if g.Status == "documented" {
			documented++
		}
	}
	if documented == len(routes) {
		return "documented"
	}
	if documented > 0 {
		return "partial"
	}
	return "needs_research"
}

func AuthorizationForAppV2(a App, index *Index) (AppAuthorization, error) {
	appAuthorizationOnce.Do(func() { appAuthorizationGuides, appAuthorizationErr = parseAppAuthorization(appAuthorizationJSON) })
	if appAuthorizationErr != nil {
		return AppAuthorization{}, appAuthorizationErr
	}
	result := AppAuthorization{Format: "gist.directory.app-authorization/v2", App: a.Slug, Name: a.Name, Status: "no_verified_route", Routes: []AuthorizationGuide{}, Notes: []string{"MCP authorization is normally discovered by the client from the server URL. Documentation status is not a prerequisite for standard MCP discovery.", "A catalog app listing does not establish that the provider offers an MCP server. HTTP and native API routes require corresponding client adapters."}}
	entry, hasEntry := appAuthorizationGuides[a.Slug]
	if hasEntry {
		body, _ := json.Marshal(entry.Routes)
		_ = json.Unmarshal(body, &result.Routes)
		if result.Routes == nil {
			result.Routes = []AuthorizationGuide{}
		}
		result.Notes = append(result.Notes, entry.Notes...)
		result.Sources = append([]AuthorizationSource(nil), entry.Sources...)
	}
	for _, id := range a.ConnectorIDs {
		if index == nil {
			continue
		}
		r, ok := index.Get(id)
		if !ok {
			continue
		}
		guide, err := AuthorizationFor(r)
		if err != nil {
			return AppAuthorization{}, err
		}
		result.Routes = append(result.Routes, guide)
	}
	result.Status = appAuthorizationStatus(result.Routes)
	if len(result.Routes) == 0 {
		if hasEntry && (entry.Status == "not_applicable" || entry.Status == "needs_research") {
			result.Status = entry.Status
		} else {
			result.Notes = append(result.Notes, "No direct connection route has been verified in Gist for this app yet. This does not establish that no provider API or MCP server exists.")
		}
	}
	return result, nil
}

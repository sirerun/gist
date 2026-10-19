package directory

import (
	"bytes"
	_ "embed"
	"encoding/json"
	"errors"
	"io"
	"sync"
)

//go:embed app-authorization.json
var appAuthorizationJSON []byte

// AppAuthorization covers every catalog app, including apps for which no
// provider route has yet been verified. It never invents an MCP endpoint.
type AppAuthorization struct {
	Format string               `json:"format"`
	App    string               `json:"app"`
	Name   string               `json:"name"`
	Status string               `json:"status"`
	Routes []AuthorizationGuide `json:"routes"`
	Notes  []string             `json:"notes"`
}

type appAuthorizationEntry struct {
	Slug   string               `json:"slug"`
	Routes []AuthorizationGuide `json:"routes"`
}

var appAuthorizationOnce sync.Once
var appAuthorizationGuides map[string][]AuthorizationGuide
var appAuthorizationErr error

func parseAppAuthorization(raw []byte) (map[string][]AuthorizationGuide, error) {
	if len(raw) > 4<<20 {
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
	out := map[string][]AuthorizationGuide{}
	for _, e := range entries {
		if !appIdentity.MatchString(e.Slug) || len(e.Routes) == 0 || len(e.Routes) > 8 {
			return nil, errors.New("invalid app authorization entry")
		}
		if _, exists := out[e.Slug]; exists {
			return nil, errors.New("duplicate app authorization entry")
		}
		body, _ := json.Marshal(e.Routes)
		if _, err := parseAuthorization(body); err != nil {
			return nil, err
		}
		out[e.Slug] = e.Routes
	}
	return out, nil
}

func AuthorizationForApp(a App, index *Index) (AppAuthorization, error) {
	appAuthorizationOnce.Do(func() { appAuthorizationGuides, appAuthorizationErr = parseAppAuthorization(appAuthorizationJSON) })
	if appAuthorizationErr != nil {
		return AppAuthorization{}, appAuthorizationErr
	}
	result := AppAuthorization{Format: "gist.directory.app-authorization/v1", App: a.Slug, Name: a.Name, Status: "no_verified_route", Routes: []AuthorizationGuide{}, Notes: []string{"MCP authorization is normally discovered by the client from the server URL. Documentation status is not a prerequisite for standard MCP discovery.", "A catalog app listing does not establish that the provider offers an MCP server. API routes require an HTTP/OpenAPI adapter in the client."}}
	if raw, ok := appAuthorizationGuides[a.Slug]; ok {
		body, _ := json.Marshal(raw)
		_ = json.Unmarshal(body, &result.Routes)
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
	documented := 0
	for _, g := range result.Routes {
		if g.Status == "documented" {
			documented++
		}
	}
	if len(result.Routes) > 0 {
		result.Status = "needs_research"
		if documented == len(result.Routes) {
			result.Status = "documented"
		} else if documented > 0 {
			result.Status = "partial"
		}
	} else {
		result.Notes = append(result.Notes, "No direct connection route has been verified in Gist for this app yet. This does not establish that no provider API or MCP server exists.")
	}
	return result, nil
}

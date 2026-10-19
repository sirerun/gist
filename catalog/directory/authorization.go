package directory

import (
	"bytes"
	_ "embed"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"net/url"
	"reflect"
	"sync"
	"time"
)

// Authorization guides are a separate versioned response so strict descriptor
// consumers can adopt richer setup metadata without changing discovery schemas.
// They contain no credentials and never initiate network requests or OAuth.
const AuthorizationFormat = "gist.directory.authorization/v1"

//go:embed authorization.json
var authorizationJSON []byte

type AuthorizationSource struct {
	URL       string `json:"url"`
	CheckedAt string `json:"checked_at"`
}

type AuthorizationGuide struct {
	Kind             string                `json:"kind"`
	APIURL           string                `json:"api_url,omitempty"`
	Format           string                `json:"format"`
	ID               string                `json:"id"`
	Title            string                `json:"title"`
	Status           string                `json:"status"`
	Methods          []string              `json:"methods"`
	Remotes          []PublicRemote        `json:"remotes"`
	SetupURL         string                `json:"setup_url,omitempty"`
	CredentialNames  []string              `json:"credential_names"`
	Prerequisites    []string              `json:"prerequisites"`
	Steps            []string              `json:"steps"`
	Scopes           []string              `json:"scopes"`
	Sources          []AuthorizationSource `json:"sources"`
	Notes            []string              `json:"notes"`
	ConnectionTested bool                  `json:"connection_tested"`
	CredentialOwner  string                `json:"credential_owner"`
}

var authorizationOnce sync.Once
var authorizationGuides map[string]AuthorizationGuide
var authorizationErr error

func parseAuthorization(raw []byte) (map[string]AuthorizationGuide, error) {
	if len(raw) > 2<<20 {
		return nil, errors.New("authorization catalog too large")
	}
	var guides []AuthorizationGuide
	d := json.NewDecoder(bytes.NewReader(raw))
	d.DisallowUnknownFields()
	if err := d.Decode(&guides); err != nil {
		return nil, err
	}
	if d.Decode(new(any)) != io.EOF {
		return nil, errors.New("trailing authorization data")
	}
	out := make(map[string]AuthorizationGuide, len(guides))
	for _, g := range guides {
		if g.Format != AuthorizationFormat || g.ID == "" || !bounded(g.ID, 200) || g.Title == "" || !bounded(g.Title, 200) || g.ConnectionTested || g.CredentialOwner != "client" {
			return nil, fmt.Errorf("invalid authorization identity %q", g.ID)
		}
		if _, ok := out[g.ID]; ok {
			return nil, errors.New("duplicate authorization guide")
		}
		if g.Status != "documented" && g.Status != "needs_research" {
			return nil, errors.New("invalid authorization status")
		}
		if g.SetupURL != "" && !safeSourceURL(g.SetupURL) {
			return nil, errors.New("invalid setup URL")
		}
		if len(g.Sources) > 12 || len(g.Remotes) > 16 {
			return nil, errors.New("authorization fields exceed bounds")
		}
		for _, s := range g.Sources {
			if !safeSourceURL(s.URL) {
				return nil, errors.New("invalid authorization source")
			}
			if _, err := time.Parse("2006-01-02", s.CheckedAt); err != nil {
				return nil, errors.New("invalid source date")
			}
		}
		if g.Kind != "mcp" && g.Kind != "http-api" && g.Kind != "openapi" {
			return nil, errors.New("invalid authorization route kind")
		}
		if g.Kind == "mcp" {
			if len(g.Remotes) == 0 || g.APIURL != "" {
				return nil, errors.New("invalid MCP authorization route")
			}
		} else if len(g.Remotes) != 0 || !safeSourceURL(g.APIURL) {
			return nil, errors.New("invalid API authorization route")
		}
		for _, r := range g.Remotes {
			if !safeURL(r.URL, false) || (r.Transport != "sse" && r.Transport != "streamable-http") {
				return nil, errors.New("invalid authorization remote")
			}
		}
		for _, m := range g.Methods {
			switch m {
			case "oauth2", "api_key", "bearer", "none", "basic", "aws_sigv4":
			default:
				return nil, errors.New("invalid authorization method")
			}
		}
		for _, items := range [][]string{g.Methods, g.CredentialNames, g.Prerequisites, g.Steps, g.Scopes, g.Notes} {
			if len(items) > 32 {
				return nil, errors.New("too many authorization details")
			}
			for _, s := range items {
				if s == "" || !bounded(s, 2000) {
					return nil, errors.New("invalid authorization text")
				}
			}
		}
		if g.Status == "documented" && (len(g.Sources) == 0 || len(g.Methods) == 0 || len(g.Steps) == 0 || g.SetupURL == "") {
			return nil, errors.New("documented guide missing evidence or steps")
		}
		if g.Status == "needs_research" && (len(g.Methods) > 0 || len(g.Scopes) > 0 || len(g.CredentialNames) > 0 || len(g.Steps) > 0) {
			return nil, errors.New("unverified guide contains authorization instructions")
		}
		out[g.ID] = g
	}
	return out, nil
}

func safeSourceURL(s string) bool {
	if !bounded(s, 2048) {
		return false
	}
	u, err := url.Parse(s)
	if err != nil {
		return false
	}
	u.Fragment = ""
	return safeURL(u.String(), false)
}

// AuthorizationFor returns route-bound, public setup guidance. Catalog changes
// invalidate an old guide rather than applying instructions to another endpoint.
func AuthorizationFor(r Record) (AuthorizationGuide, error) {
	authorizationOnce.Do(func() { authorizationGuides, authorizationErr = parseAuthorization(authorizationJSON) })
	if authorizationErr != nil {
		return AuthorizationGuide{}, authorizationErr
	}
	remotes := make([]PublicRemote, 0, len(r.Remotes))
	for _, v := range r.Remotes {
		remotes = append(remotes, PublicRemote{URL: v.URL, Transport: v.Transport})
	}
	g, ok := authorizationGuides[r.ID]
	if !ok || !reflect.DeepEqual(g.Remotes, remotes) {
		return AuthorizationGuide{Kind: "mcp", Format: AuthorizationFormat, ID: r.ID, Title: r.Title, Status: "needs_research", Methods: []string{}, Remotes: remotes, CredentialNames: []string{}, Prerequisites: []string{}, Steps: []string{}, Scopes: []string{}, Sources: []AuthorizationSource{}, Notes: []string{"Provider authorization requirements for this exact route have not been documented. Do not infer authentication from endpoint reachability."}, CredentialOwner: "client"}, nil
	}
	// Return independent slices: callers cannot change the shared embedded catalog.
	raw, _ := json.Marshal(g)
	var copy AuthorizationGuide
	_ = json.Unmarshal(raw, &copy)
	return copy, nil
}

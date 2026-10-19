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
	"regexp"
	"strings"
	"sync"
	"time"
)

// Authorization guides are a separate versioned response so strict descriptor
// consumers can adopt richer setup metadata without changing discovery schemas.
// They contain no credentials and never initiate network requests or OAuth.
const AuthorizationFormat = "gist.directory.authorization/v1"
const AuthorizationFormatV2 = "gist.directory.authorization/v2"

var endpointField = regexp.MustCompile(`^[A-Za-z_][A-Za-z0-9_]{0,63}$`)
var endpointPlaceholder = regexp.MustCompile(`\{([A-Za-z_][A-Za-z0-9_]{0,63})\}`)

//go:embed authorization.json
var authorizationJSON []byte

type AuthorizationSource struct {
	URL       string `json:"url"`
	CheckedAt string `json:"checked_at"`
}

type AuthorizationGuide struct {
	APIURLTemplate   string                `json:"api_url_template,omitempty"`
	EndpointInputs   []string              `json:"endpoint_inputs,omitempty"`
	Protocol         string                `json:"protocol,omitempty"`
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
		if (g.Format != AuthorizationFormat && g.Format != AuthorizationFormatV2) || g.ID == "" || !bounded(g.ID, 200) || g.Title == "" || !bounded(g.Title, 200) || g.ConnectionTested || g.CredentialOwner != "client" {
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
		if g.Format == AuthorizationFormat && (g.APIURLTemplate != "" || len(g.EndpointInputs) > 0 || g.Protocol != "") {
			return nil, errors.New("v2 route details in v1 guide")
		}
		if len(g.EndpointInputs) > 16 {
			return nil, errors.New("too many endpoint inputs")
		}
		for _, name := range g.EndpointInputs {
			if !endpointField.MatchString(name) {
				return nil, errors.New("endpoint input names only")
			}
		}
		if g.Kind != "mcp" && g.Kind != "http-api" && g.Kind != "openapi" && g.Kind != "native-api" {
			return nil, errors.New("invalid authorization route kind")
		}
		switch g.Kind {
		case "mcp":
			if g.APIURL != "" || g.APIURLTemplate != "" || g.Protocol != "" || (len(g.Remotes) == 0) == (len(g.EndpointInputs) == 0) {
				return nil, errors.New("choose exact MCP remotes or private endpoint inputs")
			}
		case "native-api":
			if g.Format != AuthorizationFormatV2 || !endpointField.MatchString(g.Protocol) || len(g.EndpointInputs) == 0 || len(g.Remotes) > 0 || g.APIURL != "" || g.APIURLTemplate != "" {
				return nil, errors.New("invalid native API inputs")
			}
		default:
			if len(g.Remotes) > 0 || g.Protocol != "" || (g.APIURL != "" && g.APIURLTemplate != "") || (g.APIURL == "" && g.APIURLTemplate == "" && len(g.EndpointInputs) == 0) {
				return nil, errors.New("choose exact API URL or template")
			}
			if g.APIURL != "" && !safeURL(g.APIURL, false) {
				return nil, errors.New("invalid API URL")
			}
			if g.APIURLTemplate != "" && !safeEndpointTemplate(g.APIURLTemplate, g.EndpointInputs) {
				return nil, errors.New("invalid API URL template")
			}
		}
		for _, r := range g.Remotes {
			if !safeURL(r.URL, false) || (r.Transport != "sse" && r.Transport != "streamable-http" && !(g.Format == AuthorizationFormatV2 && r.Transport == "unknown")) {
				return nil, fmt.Errorf("%s: invalid authorization remote", g.ID)
			}
		}
		for _, m := range g.Methods {
			if g.Format == AuthorizationFormat && (m == "password" || m == "client_certificate" || m == "ssh_key" || m == "hmac") {
				return nil, errors.New("v2 authentication method in v1 guide")
			}
			switch m {
			case "oauth2", "api_key", "bearer", "none", "basic", "aws_sigv4", "password", "client_certificate", "ssh_key", "hmac":
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

// Templates are descriptive only; neither the template nor client inputs are
// substituted or fetched by Gist. Reject credentials, query strings and hidden
// placeholders, and require every placeholder to have a declared input name.
func safeEndpointTemplate(value string, inputs []string) bool {
	if !bounded(value, 2048) {
		return false
	}
	names := map[string]bool{}
	for _, name := range inputs {
		names[name] = true
	}
	matches := endpointPlaceholder.FindAllStringSubmatch(value, -1)
	if len(matches) == 0 {
		return false
	}
	for _, match := range matches {
		if !names[match[1]] {
			return false
		}
	}
	substituted := endpointPlaceholder.ReplaceAllString(value, "example.com")
	if strings.HasPrefix(value, matches[0][0]) {
		suffix := strings.TrimPrefix(value, matches[0][0])
		if suffix == "" || strings.HasPrefix(suffix, "/") {
			substituted = endpointPlaceholder.ReplaceAllString("https://example.com"+suffix, "example.com")
		}
	}
	return !strings.ContainsAny(substituted, "{}") && safeURL(substituted, false)
}

func sourceDate(value string) bool { _, err := time.Parse("2006-01-02", value); return err == nil }

func safeSourceURL(s string) bool {
	if !bounded(s, 2048) {
		return false
	}
	u, err := url.Parse(s)
	if err != nil {
		return false
	}
	u.Fragment = ""
	if u.RawQuery != "" {
		allowed := map[string]bool{}
		switch u.Hostname() {
		case "learn.microsoft.com":
			allowed = map[string]bool{"view": true, "tabs": true, "preserve-view": true}
		case "shopify.dev":
			allowed = map[string]bool{"lang": true}
		case "www.postman.com":
			allowed = map[string]bool{"entity": true}
		case "developers.rdstation.com":
			allowed = map[string]bool{"lng": true}
		}
		q, err := url.ParseQuery(u.RawQuery)
		if err != nil || len(q) == 0 {
			return false
		}
		for k, values := range q {
			if !allowed[k] || len(values) != 1 || !bounded(values[0], 256) {
				return false
			}
		}
		u.RawQuery = ""
	}
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

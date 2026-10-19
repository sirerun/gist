package directory

import (
	"encoding/json"
	"strings"
	"testing"
)

func TestAuthorizationCatalogCoverageAndRouteBinding(t *testing.T) {
	guides, err := parseAuthorization(authorizationJSON)
	if err != nil {
		t.Fatal(err)
	}
	idx, err := DefaultPublic()
	if err != nil {
		t.Fatal(err)
	}
	if len(guides) != len(idx.ids) {
		t.Fatalf("coverage guides=%d connectors=%d", len(guides), len(idx.ids))
	}
	documented := 0
	for _, id := range idx.ids {
		r, _ := idx.Get(id)
		g, err := AuthorizationFor(r)
		if err != nil {
			t.Fatal(err)
		}
		if g.ID != id || len(g.Remotes) != len(r.Remotes) {
			t.Fatalf("wrong route guide %q", id)
		}
		if g.Status == "documented" {
			documented++
		}
		if strings.Contains(g.SetupURL, "make.com") {
			t.Fatalf("intermediary setup %q", id)
		}
		for _, s := range g.Sources {
			if strings.Contains(s.URL, "make.com") {
				t.Fatalf("intermediary evidence %q", id)
			}
		}
		if len(r.Remotes) > 0 {
			r.Remotes[0].URL = "https://changed.example/mcp"
			changed, err := AuthorizationFor(r)
			if err != nil || changed.Status != "needs_research" || len(changed.Methods) > 0 {
				t.Fatalf("stale instructions for %q", id)
			}
		}
	}
	if documented == 0 {
		t.Fatal("no provider documentation")
	}
}

func TestAuthorizationRejectsFalseQualificationAndUnsourcedClaims(t *testing.T) {
	base := AuthorizationGuide{Kind: "mcp", Format: AuthorizationFormat, ID: "fixture", Title: "Fixture", Status: "documented", Methods: []string{"oauth2"}, Remotes: []PublicRemote{{URL: "https://example.com/mcp", Transport: "streamable-http"}}, SetupURL: "https://example.com/docs", Steps: []string{"Connect using OAuth in your client."}, Sources: []AuthorizationSource{{URL: "https://example.com/docs", CheckedAt: "2026-10-19"}}, CredentialOwner: "client"}
	cases := []func(*AuthorizationGuide){
		func(g *AuthorizationGuide) { g.ConnectionTested = true },
		func(g *AuthorizationGuide) { g.Sources = nil },
		func(g *AuthorizationGuide) { g.Sources[0].URL = "https://user:password@example.com/docs" },
		func(g *AuthorizationGuide) { g.Status = "needs_research" },
		func(g *AuthorizationGuide) { g.Methods = []string{"inferred"} },
	}
	for i, mutate := range cases {
		raw, _ := json.Marshal(base)
		var g AuthorizationGuide
		_ = json.Unmarshal(raw, &g)
		mutate(&g)
		raw, _ = json.Marshal([]AuthorizationGuide{g})
		if _, err := parseAuthorization(raw); err == nil {
			t.Fatalf("unsafe case %d accepted", i)
		}
	}
	raw, _ := json.Marshal([]AuthorizationGuide{base, base})
	if _, err := parseAuthorization(raw); err == nil {
		t.Fatal("duplicate accepted")
	}
}

func TestAuthorizationCoversEveryCatalogAppWithoutInventingRoutes(t *testing.T) {
	idx, err := DefaultPublic()
	if err != nil {
		t.Fatal(err)
	}
	apps, err := DefaultApps(idx)
	if err != nil {
		t.Fatal(err)
	}
	extra, err := parseAppAuthorization(appAuthorizationJSON)
	if err != nil {
		t.Fatal(err)
	}
	for slug := range extra {
		if _, ok := apps.Get(slug); !ok {
			t.Fatalf("guide for nonexistent app %q", slug)
		}
	}
	for _, a := range apps.items {
		g, err := AuthorizationForApp(a, idx)
		if err != nil {
			t.Fatal(err)
		}
		if g.App != a.Slug || g.Name != a.Name {
			t.Fatalf("wrong app guide %q", a.Slug)
		}
		if len(a.ConnectorIDs) == 0 && len(extra[a.Slug].Routes) == 0 && (g.Status != "no_verified_route" || len(g.Routes) != 0) {
			t.Fatalf("invented route %q", a.Slug)
		}
		for _, r := range g.Routes {
			if r.ConnectionTested || r.CredentialOwner != "client" {
				t.Fatalf("forged qualification %q", a.Slug)
			}
		}
	}
	if len(apps.items) != 3380 {
		t.Fatalf("catalog changed: got %d; update coverage receipt", len(apps.items))
	}
}

func TestProviderEndpointTemplatesAndPrivateInputs(t *testing.T) {
	for _, tc := range []struct {
		value  string
		inputs []string
		valid  bool
	}{
		{"https://{shop}.myshopify.com/admin/api/2026-10/graphql.json", []string{"shop"}, true},
		{"{BASE_URI}/restapi", []string{"BASE_URI"}, true},
		{"https://{shop}.myshopify.com/admin", []string{"different"}, false},
		{"https://user:password@{host}/api", []string{"host"}, false},
		{"http://{host}/api", []string{"host"}, false},
		{"https://example.com/api?token={secret}", []string{"secret"}, false},
	} {
		if got := safeEndpointTemplate(tc.value, tc.inputs); got != tc.valid {
			t.Fatalf("template %q valid=%v", tc.value, got)
		}
	}
	idx, err := DefaultPublic()
	if err != nil {
		t.Fatal(err)
	}
	apps, err := DefaultApps(idx)
	if err != nil {
		t.Fatal(err)
	}
	for _, slug := range []string{"shopify", "activecampaign"} {
		a, ok := apps.Get(slug)
		if !ok {
			t.Fatal(slug)
		}
		v2, err := AuthorizationForAppV2(a, idx)
		if err != nil {
			t.Fatal(err)
		}
		if len(v2.Routes) == 0 || v2.Routes[0].Format != AuthorizationFormatV2 || len(v2.Routes[0].EndpointInputs) == 0 {
			t.Fatalf("missing private/tenant inputs %s: %+v", slug, v2)
		}
		v1, err := AuthorizationForApp(a, idx)
		if err != nil {
			t.Fatal(err)
		}
		raw, _ := json.Marshal(v1)
		for _, field := range []string{"\"api_url_template\"", "\"endpoint_inputs\"", "\"protocol\"", "gist.directory.authorization/v2"} {
			if strings.Contains(string(raw), field) {
				t.Fatalf("v1 schema changed: %s", raw)
			}
		}
	}
}

func TestNativeAndPrivateEndpointMetadataContainNamesOnly(t *testing.T) {
	base := AuthorizationGuide{Format: AuthorizationFormatV2, ID: "api:database", Title: "Database", Status: "documented", Kind: "native-api", Protocol: "postgresql", EndpointInputs: []string{"HOST", "PORT", "DATABASE", "USERNAME"}, Methods: []string{"password"}, Remotes: []PublicRemote{}, SetupURL: "https://www.postgresql.org/docs/current/client-authentication.html", CredentialNames: []string{"PASSWORD"}, Steps: []string{"Configure the database connection in the client."}, Sources: []AuthorizationSource{{URL: "https://www.postgresql.org/docs/current/client-authentication.html", CheckedAt: "2026-10-19"}}, CredentialOwner: "client"}
	body, _ := json.Marshal([]AuthorizationGuide{base})
	if _, err := parseAuthorization(body); err != nil {
		t.Fatal(err)
	}
	for _, change := range []func(*AuthorizationGuide){
		func(g *AuthorizationGuide) { g.EndpointInputs = []string{"https://private.example/mcp?token=secret"} },
		func(g *AuthorizationGuide) { g.APIURL = "https://database.example" },
		func(g *AuthorizationGuide) { g.Format = AuthorizationFormat },
		func(g *AuthorizationGuide) { g.CredentialOwner = "gist" },
	} {
		raw, _ := json.Marshal(base)
		var g AuthorizationGuide
		_ = json.Unmarshal(raw, &g)
		change(&g)
		raw, _ = json.Marshal([]AuthorizationGuide{g})
		if _, err := parseAuthorization(raw); err == nil {
			t.Fatal("invalid native/private endpoint accepted")
		}
	}
}

func TestOAuth1aRequiresVersion2(t *testing.T) {
	guide := AuthorizationGuide{Format: AuthorizationFormatV2, ID: "api:fixture", Title: "Fixture", Status: "documented", Kind: "http-api", APIURL: "https://example.com/rest/V1", Methods: []string{"oauth1a"}, Remotes: []PublicRemote{}, SetupURL: "https://example.com/docs", Steps: []string{"Complete the provider's OAuth 1.0a exchange and sign requests in the client."}, Sources: []AuthorizationSource{{URL: "https://example.com/docs", CheckedAt: "2026-10-19"}}, CredentialOwner: "client"}
	body, _ := json.Marshal([]AuthorizationGuide{guide})
	if _, err := parseAuthorization(body); err != nil {
		t.Fatal(err)
	}
	guide.Format = AuthorizationFormat
	body, _ = json.Marshal([]AuthorizationGuide{guide})
	if _, err := parseAuthorization(body); err == nil {
		t.Fatal("OAuth 1.0a accepted in a v1 guide")
	}
}

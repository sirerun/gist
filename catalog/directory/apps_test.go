package directory

import (
	"encoding/json"
	"testing"
)

func TestAppsInventorySeparateFromConnections(t *testing.T) {
	connectors, err := Default()
	if err != nil {
		t.Fatal(err)
	}
	raw := []byte(`{"format":"gist.app-directory/v1","items":[{"slug":"airtable","name":"Airtable","description":"Organize records","categories":[{"slug":"database","name":"Database"}],"icon_path":"abc123.svg","background_color":"#ff0000"},{"slug":"unconnected","name":"Unconnected","description":"App inventory","categories":[]}]}`)
	idx, err := ParseApps(raw, connectors)
	if err != nil {
		t.Fatal(err)
	}
	item, ok := idx.Get("airtable")
	if !ok || len(item.ConnectorIDs) == 0 || item.IconURL != "/assets/apps/abc123.svg" {
		t.Fatalf("unexpected app %+v", item)
	}
	absent, _ := idx.Get("unconnected")
	if len(absent.ConnectorIDs) != 0 {
		t.Fatal("invented connection")
	}
	page, err := idx.Search(AppQuery{Limit: 1})
	if err != nil || page.Total != 2 || page.NextOffset == nil || *page.NextOffset != 1 {
		t.Fatalf("pagination %+v %v", page, err)
	}
	page, err = idx.Search(AppQuery{Text: "organize", Category: "database"})
	if err != nil || len(page.Items) != 1 {
		t.Fatalf("search %+v %v", page, err)
	}
	item.Categories[0].Name = "Changed"
	again, _ := idx.Get("airtable")
	if again.Categories[0].Name == "Changed" {
		t.Fatal("mutable index")
	}
	body, _ := json.Marshal(item)
	var public map[string]any
	json.Unmarshal(body, &public)
	for _, key := range []string{"source", "probe", "remotes", "packages"} {
		if _, ok := public[key]; ok {
			t.Fatalf("private field %s", key)
		}
	}
}
func TestAppsRejectUnsafeMetadata(t *testing.T) {
	for _, raw := range []string{
		`{"format":"gist.app-directory/v1","items":[{"slug":"app","name":"App","icon_path":"../bad.svg"}]}`,
		`{"format":"gist.app-directory/v1","items":[{"slug":"app","name":"App","background_color":"red;display:none"}]}`,
		`{"format":"gist.app-directory/v1","items":[{"slug":"app","name":"App"},{"slug":"app","name":"Other"}]}`,
	} {
		if _, err := ParseApps([]byte(raw), nil); err == nil {
			t.Fatalf("accepted %s", raw)
		}
	}
	idx, _ := ParseApps([]byte(`{"format":"gist.app-directory/v1","items":[]}`), nil)
	for _, q := range []AppQuery{{Limit: 101}, {Offset: -1}, {Text: "bad\nquery"}} {
		if _, err := idx.Search(q); err == nil {
			t.Fatal("accepted query")
		}
	}
}

func TestDefaultAppsInventory(t *testing.T) {
	connectors, err := Default()
	if err != nil {
		t.Fatal(err)
	}
	apps, err := DefaultApps(connectors)
	if err != nil {
		t.Fatal(err)
	}
	page, err := apps.Search(AppQuery{Limit: 100})
	if err != nil || page.Total < 3000 || len(page.Items) != 100 {
		t.Fatalf("inventory total=%d items=%d error=%v", page.Total, len(page.Items), err)
	}
}

func TestPublicIconForService(t *testing.T) {
	apps, err := DefaultApps(nil)
	if err != nil {
		t.Fatal(err)
	}
	gmail, ok := apps.Get("google-email")
	if !ok || gmail.IconURL == "" {
		t.Fatal("missing Gmail application icon")
	}
	if got := PublicIconForService("google-gmail"); got != gmail.IconURL {
		t.Fatalf("Gmail alias icon %q", got)
	}
	if got := PublicIconForService("never-an-admitted-application"); got != "" {
		t.Fatalf("inferred icon %q", got)
	}
}

func TestAppsLocalPNGIcon(t *testing.T) {
	apps, err := ParseApps([]byte(`{"format":"gist.app-directory/v1","items":[{"slug":"example","name":"Example","icon_path":"aabbcc123.png"}]}`), nil)
	if err != nil {
		t.Fatal(err)
	}
	app, ok := apps.Get("example")
	if !ok || app.IconURL != "/assets/apps/aabbcc123.png" {
		t.Fatalf("PNG icon %+v", app)
	}
}

func TestAppsPreserveLiteralIdentity(t *testing.T) {
	apps, err := ParseApps([]byte(`{"format":"gist.app-directory/v1","items":[{"slug":"app#world-cup-2022-rkrqsb","name":"World Cup"},{"slug":"h&m-home","name":"H&M Home"},{"slug":"inspection-support-network-(isn)-community","name":"ISN"}]}`), nil)
	if err != nil {
		t.Fatal(err)
	}
	for _, slug := range []string{"app#world-cup-2022-rkrqsb", "h&m-home", "inspection-support-network-(isn)-community"} {
		app, ok := apps.Get(slug)
		if !ok || app.Slug != slug {
			t.Fatalf("identity changed %q", slug)
		}
	}
}

func TestReviewedAppConnectorAliases(t *testing.T) {
	connectors, err := Default()
	if err != nil {
		t.Fatal(err)
	}
	apps, err := DefaultApps(connectors)
	if err != nil {
		t.Fatal(err)
	}
	hubspot, ok := apps.Get("hubspotcrm")
	if !ok || len(hubspot.ConnectorIDs) == 0 {
		t.Fatalf("HubSpot route missing %+v", hubspot)
	}
	for _, id := range hubspot.ConnectorIDs {
		record, ok := connectors.Get(id)
		if !ok || record.ServiceID != "hubspot" {
			t.Fatalf("HubSpot joined wrong connector %q", id)
		}
	}
	app, ok := publicAppForService("tavily-search")
	if !ok || app.Slug != "tavily" {
		t.Fatalf("canonical Tavily %+v", app)
	}
}

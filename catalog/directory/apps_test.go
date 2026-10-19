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

func TestAppPriorityIsAppliedBeforePagingWithoutChangingDefaultOrder(t *testing.T) {
	idx, err := DefaultPublic()
	if err != nil {
		t.Fatal(err)
	}
	apps, err := DefaultApps(idx)
	if err != nil {
		t.Fatal(err)
	}
	before, _ := apps.Search(AppQuery{Limit: 2})
	ranked, err := apps.Search(AppQuery{Limit: 1, PriorityConnectorIDs: []string{"mcp:google-gmail"}})
	if err != nil {
		t.Fatal(err)
	}
	if len(ranked.Items) != 1 || ranked.Items[0].Slug != "google-email" {
		t.Fatalf("not pinned: %+v", ranked.Items)
	}
	next, _ := apps.Search(AppQuery{Offset: 1, Limit: 1, PriorityConnectorIDs: []string{"mcp:google-gmail"}})
	if next.Items[0].Slug == "google-email" {
		t.Fatal("duplicate pinned item")
	}
	filtered, _ := apps.Search(AppQuery{Category: "crm-sales", Limit: 100, PriorityConnectorIDs: []string{"mcp:google-gmail"}})
	for _, a := range filtered.Items {
		if a.Slug == "google-email" {
			t.Fatal("priority bypassed filter")
		}
	}
	after, _ := apps.Search(AppQuery{Limit: 2})
	for i := range before.Items {
		if before.Items[i].Slug != after.Items[i].Slug {
			t.Fatal("priority mutated shared index")
		}
	}
}

func TestConnectableFilterUsesDocumentedRoutesBeforePagination(t *testing.T) {
	index, err := DefaultPublic()
	if err != nil {
		t.Fatal(err)
	}
	apps, err := DefaultApps(index)
	if err != nil {
		t.Fatal(err)
	}
	all, err := apps.Search(AppQuery{Limit: 1})
	if err != nil {
		t.Fatal(err)
	}
	first, err := apps.Search(AppQuery{ConnectableOnly: true, Limit: 1})
	if err != nil {
		t.Fatal(err)
	}
	if first.Total == 0 || first.Total >= all.Total || first.NextOffset == nil {
		t.Fatalf("filter totals: all=%d filtered=%+v", all.Total, first)
	}
	seen := map[string]bool{}
	for offset := 0; ; {
		page, err := apps.Search(AppQuery{ConnectableOnly: true, Offset: offset, Limit: 100})
		if err != nil {
			t.Fatal(err)
		}
		for _, a := range page.Items {
			if seen[a.Slug] {
				t.Fatal("duplicate pagination", a.Slug)
			}
			seen[a.Slug] = true
			g, err := AuthorizationForAppV2(a, index)
			if err != nil {
				t.Fatal(err)
			}
			documented := false
			for _, r := range g.Routes {
				documented = documented || r.Status == "documented"
			}
			if !documented {
				t.Fatal("unconnectable app leaked", a.Slug, g.Status)
			}
		}
		if page.NextOffset == nil {
			break
		}
		offset = *page.NextOffset
	}
	if len(seen) != first.Total {
		t.Fatalf("pagination count %d != %d", len(seen), first.Total)
	}
	// Ranking must retain a later connectable item when earlier candidates lack setup.
	fixture := &AppIndex{items: []App{{Slug: "gap"}, {Slug: "ready", connectable: true}}, bySlug: map[string]int{"gap": 0, "ready": 1}}
	ranked, err := fixture.SearchRanked(AppQuery{ConnectableOnly: true, Limit: 1}, []string{"gap", "ready"})
	if err != nil || ranked.Total != 1 || len(ranked.Items) != 1 || ranked.Items[0].Slug != "ready" {
		t.Fatalf("ranked filter %+v %v", ranked, err)
	}
}

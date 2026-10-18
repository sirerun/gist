package app

import (
	"context"
	"encoding/json"
	"github.com/sirerun/gist/catalog/directory"
	"github.com/sirerun/gist/hosted/internal/ports"
	"net/http/httptest"
	"strings"
	"testing"
)

type publicBookmarkProbe struct{ ids []string }

func (p publicBookmarkProbe) list(context.Context, ports.Principal) ([]string, error) {
	return p.ids, nil
}
func (p publicBookmarkProbe) set(context.Context, ports.Principal, string, bool) error { return nil }

func TestBookmarkEntriesKeepIntakePrivate(t *testing.T) {
	h, _, _ := testWebAPI()
	idx, err := directory.DefaultPublic()
	if err != nil {
		t.Fatal(err)
	}
	page, err := idx.Search(directory.Query{Limit: 100})
	if err != nil {
		t.Fatal(err)
	}
	ids := []string{}
	for _, r := range page.Items {
		ids = append(ids, r.ID)
	}
	h.publicIndex, h.bookmarks = idx, publicBookmarkProbe{ids: ids}
	w := httptest.NewRecorder()
	h.serveBookmarks(w, httptest.NewRequest("GET", "/app/api/bookmarks?view=entries", nil))
	if w.Code != 200 {
		t.Fatalf("%d %s", w.Code, w.Body)
	}
	for _, secret := range []string{"make.com", "n8n.io", "checked_at", "http_status", "server_listing_url\":\"https://apps.make", "workflow qualification", "operator is not verified"} {
		if strings.Contains(strings.ToLower(w.Body.String()), secret) {
			t.Fatalf("bookmark response leaked %q", secret)
		}
	}
	w = httptest.NewRecorder()
	h.serveBookmarks(w, httptest.NewRequest("GET", "/app/api/bookmarks?view=entries&q=listed+by+make", nil))
	if w.Code != 200 || !strings.Contains(w.Body.String(), `"total":0`) {
		t.Fatalf("saved search indexes hidden intake: %d %s", w.Code, w.Body)
	}
	w = httptest.NewRecorder()
	h.serveBookmarks(w, httptest.NewRequest("GET", "/app/api/bookmarks?view=entries&q=contacts+companies+deals", nil))
	if w.Code != 200 || !strings.Contains(w.Body.String(), `"service_id":"hubspot"`) {
		t.Fatalf("saved search missing public purpose: %d %s", w.Code, w.Body)
	}
}

func TestBookmarkTextMatchesDirectorySearch(t *testing.T) {
	index, err := directory.Default()
	if err != nil {
		t.Fatal(err)
	}
	for _, query := range []string{"calendar google", "GOOG cal", "git-hub", "web research", "brave sea", "gmail"} {
		result, err := index.Search(directory.Query{Text: query, Limit: 100})
		if err != nil {
			t.Fatal(err)
		}
		for _, record := range result.Items {
			if !bookmarkTextMatches(record, bookmarkTerms(query)) {
				t.Fatalf("saved search disagrees for %q and %q", query, record.ID)
			}
		}
	}
	record, ok := index.Get("mcp:google-calendar")
	if !ok {
		t.Fatal("calendar fixture missing")
	}
	if !bookmarkTextMatches(record, bookmarkTerms("calendar google")) || bookmarkTextMatches(record, bookmarkTerms("calendar nosuchword")) {
		t.Fatal("prefix AND matching failed")
	}
}

func TestBookmarkCatalogRanksBeforePaginationAndKeepsPublicProjection(t *testing.T) {
	h, _, _ := testWebAPI()
	idx, err := directory.DefaultPublic()
	if err != nil {
		t.Fatal(err)
	}
	h.publicIndex = idx
	all, err := idx.Search(directory.Query{Kind: "mcp", Limit: 100})
	if err != nil {
		t.Fatal(err)
	}
	saved := all.Items[len(all.Items)-1]
	h.bookmarks = publicBookmarkProbe{ids: []string{saved.ID}}
	request := func(path string) string {
		t.Helper()
		w := httptest.NewRecorder()
		h.serveBookmarks(w, httptest.NewRequest("GET", path, nil))
		if w.Code != 200 {
			t.Fatalf("status%d %s", w.Code, w.Body)
		}
		return w.Body.String()
	}
	var page struct {
		Items []directory.PublicRecord `json:"items"`
		Total int                      `json:"total"`
		Next  string                   `json:"next_cursor"`
	}
	raw := request("/app/api/bookmarks?view=catalog&kind=mcp&limit=1")
	if err := json.Unmarshal([]byte(raw), &page); err != nil {
		t.Fatal(err)
	}
	if len(page.Items) != 1 || page.Items[0].ID != saved.ID || page.Total <= 1 || page.Next != "1" {
		t.Fatalf("not ranked before paging%s", raw)
	}
	raw = request("/app/api/bookmarks?view=catalog&kind=mcp&limit=1&offset=1")
	if strings.Contains(raw, `"id":"`+saved.ID+`"`) {
		t.Fatal("duplicate saved entry")
	}
	h.bookmarks = publicBookmarkProbe{ids: []string{"mcp:google-gmail"}}
	var apps struct {
		Items []directory.App `json:"items"`
	}
	raw = request("/app/api/bookmarks?view=catalog&limit=1")
	if err := json.Unmarshal([]byte(raw), &apps); err != nil {
		t.Fatal(err)
	}
	if len(apps.Items) != 1 || apps.Items[0].Slug != "google-email" {
		t.Fatalf("saved app not first%s", raw)
	}
	raw = request("/app/api/bookmarks?view=catalog&kind=mcp")
	for _, secret := range []string{"checked_at", "http_status", "apps.make.com"} {
		if strings.Contains(raw, secret) {
			t.Fatal("leaked", secret)
		}
	}
	h.bookmarks = publicBookmarkProbe{}
	raw = request("/app/api/bookmarks?view=catalog&kind=mcp&limit=1")
	if strings.Contains(raw, `"id":"mcp:google-gmail"`) && all.Items[0].ID != "mcp:google-gmail" {
		t.Fatal("another user's priority leaked")
	}
	for _, q := range []string{"limit=0", "limit=101", "offset=-1"} {
		w := httptest.NewRecorder()
		h.serveBookmarks(w, httptest.NewRequest("GET", "/app/api/bookmarks?view=catalog&"+q, nil))
		if w.Code != 400 {
			t.Fatal(q, w.Code)
		}
	}
}

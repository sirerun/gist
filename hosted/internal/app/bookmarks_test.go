package app

import (
	"context"
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
	idx, err := directory.Default()
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

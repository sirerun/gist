package app

import (
	"github.com/sirerun/gist/catalog/directory"
	"testing"
)

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

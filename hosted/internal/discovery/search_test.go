package discovery

import (
	"context"
	"encoding/json"
	"testing"

	"github.com/sirerun/gist/hosted/internal/ports"
)

type searchDouble struct {
	page  ports.SearchPage
	calls int
}

func (s *searchDouble) Search(context.Context, ports.SearchQuery) (ports.SearchPage, error) {
	s.calls++
	return s.page, nil
}

type authDouble struct {
	allowed map[string]bool
	calls   int
}

func (a *authDouble) Decide(_ context.Context, _ ports.Principal, _ ports.Action, ref *ports.ArtifactRef) (ports.Decision, error) {
	a.calls++
	return ports.Decision{Allowed: a.allowed[ref.ID]}, nil
}

func TestSearchFiltersBeforeRankAndLimit(t *testing.T) {
	makeMetadata := func(summary string, tags []string) []byte {
		b, _ := json.Marshal(metadata{Summary: summary, Tags: tags})
		return b
	}
	search := &searchDouble{page: ports.SearchPage{Records: []ports.CatalogRecord{
		{Ref: ports.ArtifactRef{WorkspaceID: "w", Kind: ports.KindSkill, ID: "hidden", Version: "1"}, Metadata: makeMetadata("engineer resume", nil)},
		{Ref: ports.ArtifactRef{WorkspaceID: "w", Kind: ports.KindSkill, ID: "visible", Version: "1"}, Metadata: makeMetadata("engineer", []string{"hr"})},
	}}}
	auth := &authDouble{allowed: map[string]bool{"visible": true}}
	svc, err := New(search, auth)
	if err != nil {
		t.Fatal(err)
	}
	got, err := svc.Search(context.Background(), Request{Principal: ports.Principal{Subject: "p", WorkspaceID: "w"}, Query: "engineer", Tags: []string{"hr"}, Limit: 1, MaxBytes: 4096})
	if err != nil {
		t.Fatal(err)
	}
	if len(got.Candidates) != 1 || got.Candidates[0].ID != "visible" {
		t.Fatalf("unexpected candidates: %+v", got.Candidates)
	}
	if auth.calls != 2 {
		t.Fatalf("authorization must happen before ranking, calls=%d", auth.calls)
	}
}

func TestSearchReturnsEmptyForNoMatch(t *testing.T) {
	search := &searchDouble{page: ports.SearchPage{Records: []ports.CatalogRecord{{Ref: ports.ArtifactRef{WorkspaceID: "w", Kind: ports.KindSkill, ID: "one", Version: "1"}}}}}
	auth := &authDouble{allowed: map[string]bool{"one": true}}
	svc, err := New(search, auth)
	if err != nil {
		t.Fatal(err)
	}
	got, err := svc.Search(context.Background(), Request{Principal: ports.Principal{Subject: "p", WorkspaceID: "w"}, Query: "missing", Limit: 10, MaxBytes: 4096})
	if err != nil {
		t.Fatal(err)
	}
	if len(got.Candidates) != 0 {
		t.Fatalf("expected empty result, got %+v", got.Candidates)
	}
}

func TestSearchRanksTokenizedTermsDeterministically(t *testing.T) {
	record := func(kind ports.ArtifactKind, id, meta string) ports.CatalogRecord {
		return ports.CatalogRecord{Ref: ports.ArtifactRef{WorkspaceID: "w", Kind: kind, ID: id, Version: "1.0.0"}, Metadata: []byte(meta)}
	}
	records := []ports.CatalogRecord{
		record(ports.KindSkill, "gist/skill/asset-skill", `{"logical_name":"asset-skill","summary":"Offline document parsing fixture.","tags":["document","fixture"],"required_capabilities":["gist/document/parse@1.0.0"]}`),
		record(ports.KindBinding, "gist/slack-web-api/document-parse", `{"logical_name":"public document parser binding","summary":"Document parser binding.","tags":["document"],"capability_ref":"gist/document/parse@1.0.0"}`),
		record(ports.KindCapability, "gist/document/parse", `{"logical_name":"document parse","summary":"Parse a document into text.","tags":["document"]}`),
		record(ports.KindCapability, "gist/identity/user.create", `{"logical_name":"identity user create","summary":"Create a user.","tags":["identity"]}`),
	}
	allow := map[string]bool{}
	for _, r := range records {
		allow[r.Ref.ID] = true
	}
	svc, err := New(&searchDouble{page: ports.SearchPage{Records: records}}, &authDouble{allowed: allow})
	if err != nil {
		t.Fatal(err)
	}
	ids := func(query string) []string {
		got, err := svc.Search(context.Background(), Request{Principal: ports.Principal{Subject: "p", WorkspaceID: "w"}, Query: query, Limit: 10, MaxBytes: 1 << 16})
		if err != nil {
			t.Fatal(err)
		}
		out := make([]string, 0, len(got.Candidates))
		for _, c := range got.Candidates {
			out = append(out, c.ID)
		}
		return out
	}
	if got := ids("offline document parsing fixture"); len(got) == 0 || got[0] != "gist/skill/asset-skill" {
		t.Fatalf("exact intent must rank its artifact first: %v", got)
	}
	// Rank by matched terms, then field weight; "capability" also matches
	// the kind of an unrelated capability, which therefore ranks last.
	if got := ids("document parse capability"); len(got) != 4 || got[0] != "gist/document/parse" || got[1] != "gist/slack-web-api/document-parse" || got[2] != "gist/skill/asset-skill" || got[3] != "gist/identity/user.create" {
		t.Fatalf("unexpected ranking: %v", got)
	}
	if got := ids("weather forecast lookup"); len(got) != 0 {
		t.Fatalf("no-match query must be empty: %v", got)
	}
	first := ids("document parser")
	for i := 0; i < 5; i++ {
		if again := ids("document parser"); len(again) != len(first) || again[0] != first[0] {
			t.Fatalf("ranking is not deterministic: %v vs %v", first, again)
		}
	}
}

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

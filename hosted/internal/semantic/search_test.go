package semantic

import (
	"context"
	"errors"
	"github.com/sirerun/gist/catalog/directory"
	"github.com/sirerun/gist/catalog/preparation"
	"strings"
	"testing"
)

type fakeEmbedder struct {
	fail  bool
	calls int
}

func (*fakeEmbedder) ModelVersion() string { return "test-pinned-768" }
func (f *fakeEmbedder) Embed(_ context.Context, q preparation.EmbeddingRequest) (preparation.EmbeddingResult, error) {
	f.calls++
	if f.fail {
		return preparation.EmbeddingResult{}, errors.New("offline")
	}
	result := preparation.EmbeddingResult{ModelVersion: f.ModelVersion()}
	for _, text := range q.Texts {
		v := make([]float64, 768)
		n := 1
		if strings.Contains(strings.ToLower(text), "gmail") || text == "manage incoming correspondence" {
			n = 0
		}
		if text == "unrelated nonexistent purpose" {
			n = 2
		}
		v[n] = 1
		result.Vectors = append(result.Vectors, v)
	}
	return result, nil
}
func fixture(t *testing.T) (*Service, *fakeEmbedder) {
	t.Helper()
	index, err := directory.DefaultPublic()
	if err != nil {
		t.Fatal(err)
	}
	apps, err := directory.DefaultApps(index)
	if err != nil {
		t.Fatal(err)
	}
	f := &fakeEmbedder{}
	s := New(index, apps, f, func() bool { return true })
	if err := s.Warm(context.Background()); err != nil {
		t.Fatal(err)
	}
	return s, f
}
func TestSemanticCandidatesAcrossDirectoryAndApps(t *testing.T) {
	s, f := fixture(t)
	ctx := context.Background()
	q := directory.Query{Text: "manage incoming correspondence", Limit: 1}
	baseline, err := s.index.Search(q)
	if err != nil || baseline.Total != 0 {
		t.Fatalf("lexical confound: %+v %v", baseline, err)
	}
	result, err := s.Search(ctx, q)
	if err != nil || result.SearchMode != "hybrid" || len(result.Items) != 1 {
		t.Fatalf("result %+v %v", result, err)
	}
	if !strings.Contains(strings.ToLower(result.Items[0].Title), "gmail") {
		t.Fatalf("wrong semantic result: %+v", result.Items)
	}
	calls := f.calls
	apps, err := s.SearchApps(ctx, directory.AppQuery{Text: q.Text, Limit: 10})
	if err != nil || apps.SearchMode != "hybrid" || apps.Total == 0 {
		t.Fatalf("apps %+v %v", apps, err)
	}
	if f.calls != calls {
		t.Fatal("same query did not reuse vector cache")
	}
	for _, a := range apps.Items {
		if !strings.Contains(strings.ToLower(a.Name), "gmail") {
			t.Fatalf("wrong app %s", a.Name)
		}
	}
	filtered, err := s.Search(ctx, directory.Query{Text: q.Text, Tag: "tag-not-present", Limit: 10})
	if err != nil || filtered.Total != 0 {
		t.Fatalf("filter bypass %+v %v", filtered, err)
	}
	empty, err := s.Search(ctx, directory.Query{Text: "unrelated nonexistent purpose", Limit: 10})
	if err != nil || empty.Total != 0 {
		t.Fatalf("forced similarity %+v %v", empty, err)
	}
}
func TestRuntimeFailureAndBusyFallBackWithoutInference(t *testing.T) {
	s, f := fixture(t)
	f.fail = true
	r, err := s.Search(context.Background(), directory.Query{Text: "gmail", Limit: 10})
	if err != nil || r.SearchMode != "lexical_fallback" || r.Total == 0 {
		t.Fatalf("fallback %+v %v", r, err)
	}
	calls := f.calls
	s.slot <- struct{}{}
	_, _ = s.Search(context.Background(), directory.Query{Text: "calendar", Limit: 10})
	<-s.slot
	if f.calls != calls {
		t.Fatal("queued busy request")
	}
	s.healthy = func() bool { return false }
	_, _ = s.Search(context.Background(), directory.Query{Text: "contacts", Limit: 10})
	if f.calls != calls {
		t.Fatal("called dead runtime")
	}
}
func TestUnreadyAndInvalidQueries(t *testing.T) {
	s, f := fixture(t)
	s.ready.Store(nil)
	calls := f.calls
	r, err := s.Search(context.Background(), directory.Query{Text: "gmail", Limit: 10})
	if err != nil || r.SearchMode != "lexical_fallback" {
		t.Fatalf("unready %+v %v", r, err)
	}
	if _, err = s.Search(context.Background(), directory.Query{Text: strings.Repeat("x", 201)}); err == nil {
		t.Fatal("invalid query accepted")
	}
	if f.calls != calls {
		t.Fatal("unready/invalid invoked model")
	}
}

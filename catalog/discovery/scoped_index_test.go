package discovery

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"strings"
	"sync"
	"testing"

	"github.com/sirerun/gist/catalog/composition"
	"github.com/sirerun/gist/catalog/ports"
)

type indexPolicyFunc func(context.Context, IndexScope) error

func (f indexPolicyFunc) Check(c context.Context, s IndexScope) error { return f(c, s) }

type indexAuthFunc func(context.Context, ports.Principal, ports.Action, *ports.ArtifactRef) (ports.Decision, error)

func (f indexAuthFunc) Decide(c context.Context, p ports.Principal, a ports.Action, r *ports.ArtifactRef) (ports.Decision, error) {
	return f(c, p, a, r)
}
func indexTestScope() IndexScope {
	return IndexScope{Principal: ports.Principal{Subject: "operator", WorkspaceID: "private", Scopes: []string{"catalog:read"}, PolicyGeneration: 7}, RuntimeID: "runtime"}
}
func indexAllow(context.Context, IndexScope) error { return nil }
func indexRead(context.Context, ports.Principal, ports.Action, *ports.ArtifactRef) (ports.Decision, error) {
	return ports.Decision{Allowed: true}, nil
}
func indexTestEntry(id, summary string) IndexEntry {
	b, _ := json.Marshal(map[string]any{"summary": summary, "tags": []string{"mail"}})
	return IndexEntry{Record: ports.CatalogRecord{Ref: ports.ArtifactRef{WorkspaceID: "private", Kind: ports.KindSkill, ID: id, Version: "1"}, State: "published", Metadata: b}}
}
func indexQuery(text string) IndexSearch {
	return IndexSearch{Scope: indexTestScope(), Query: text, Limit: 64, MaxExamined: 4096, MaxBytes: 256 << 10}
}
func newIndexTest(t *testing.T, entries ...IndexEntry) *ScopedIndex {
	t.Helper()
	x, e := NewScopedIndex(context.Background(), indexTestScope(), indexAuthFunc(indexRead), indexPolicyFunc(indexAllow), entries)
	if e != nil {
		t.Fatal(e)
	}
	return x
}

func TestScopedIndexLexicalAndExact(t *testing.T) {
	ctx := context.Background()
	calendar := indexTestEntry("calendar", "Schedules")
	// This negative candidate must not inherit the mail tag from the mail fixture.
	calendar.Record.Metadata = []byte(`{"summary":"Schedules","tags":["calendar"]}`)
	x := newIndexTest(t, indexTestEntry("mail/read", "Inspect messages"), indexTestEntry("messages/inspect", "Read mail"), calendar)
	r, e := x.Search(ctx, indexQuery("read mail"))
	if e != nil || r.Status != "complete" || len(r.Candidates) != 2 || r.Candidates[0].ID != "mail/read" {
		t.Fatalf("%+v %v", r, e)
	}
	q := indexQuery("read mail")
	q.Tags = []string{"missing"}
	r, e = x.Search(ctx, q)
	if e != nil || len(r.Candidates) != 0 {
		t.Fatalf("filters %+v %v", r, e)
	}
	for _, term := range []string{"no-matching-term", "the and", ""} {
		r, e = x.Search(ctx, indexQuery(term))
		if e != nil || r.Status != "complete" || len(r.Candidates) != 0 {
			t.Fatalf("%q %+v %v", term, r, e)
		}
	}
	q = indexQuery("read mail")
	q.MaxExamined = 1
	r, e = x.Search(ctx, q)
	if e != nil || r.Status != "incomplete_budget" || len(r.Candidates) != 0 {
		t.Fatalf("budget %+v %v", r, e)
	}
	c, ok, e := x.Exact(ctx, indexTestScope(), indexTestEntry("mail/read", "").Record.Ref)
	if e != nil || !ok || c.ID != "mail/read" {
		t.Fatalf("exact after exhaustion %+v %v %v", c, ok, e)
	}
	c.Tags[0] = "modified"
	c, _, _ = x.Exact(ctx, indexTestScope(), c.Ref)
	if c.Tags[0] != "mail" {
		t.Fatal("mutable result aliases snapshot")
	}
	q = indexQuery("read mail")
	q.MaxBytes = 100
	r, e = x.Search(ctx, q)
	if e != nil || r.Status != "incomplete_budget" || len(r.Candidates) != 0 {
		t.Fatalf("bytes %+v %v", r, e)
	}
	q = indexQuery("read mail")
	q.MaxBytes = 1
	if _, e = x.Search(ctx, q); e == nil {
		t.Fatal("unrepresentable response accepted")
	}
}

func TestScopedIndexIsolationAndCurrentPolicy(t *testing.T) {
	ctx := context.Background()
	allow := true
	calls := 0
	policy := indexPolicyFunc(func(context.Context, IndexScope) error {
		calls++
		if !allow {
			return errors.New("revoked")
		}
		return nil
	})
	x, e := NewScopedIndex(ctx, indexTestScope(), indexAuthFunc(indexRead), policy, []IndexEntry{indexTestEntry("secret", "Private mail")})
	if e != nil {
		t.Fatal(e)
	}
	for _, change := range []func(*IndexScope){func(s *IndexScope) { s.Principal.Subject = "other" }, func(s *IndexScope) { s.Principal.PolicyGeneration++ }, func(s *IndexScope) { s.Principal.Scopes = nil }, func(s *IndexScope) { s.RuntimeID = "other" }, func(s *IndexScope) { s.Principal.SubjectType = "other" }} {
		q := indexQuery("secret")
		change(&q.Scope)
		r, e := x.Search(ctx, q)
		if e == nil || len(r.Candidates) != 0 {
			t.Fatal("cross-scope exposure")
		}
	}
	for _, q := range []IndexSearch{indexQuery("unknown"), func() IndexSearch { q := indexQuery("secret mail"); q.MaxExamined = 1; return q }()} {
		calls = 0
		_, e = x.Search(ctx, q)
		if e != nil || calls != 2 {
			t.Fatalf("empty/incomplete final policy checks %d %v", calls, e)
		}
	}
	allow = false
	if r, e := x.Search(ctx, indexQuery("secret")); e == nil || len(r.Candidates) != 0 {
		t.Fatal("revoked policy exposed data")
	}
	if _, _, e = x.Exact(ctx, indexTestScope(), indexTestEntry("missing", "").Record.Ref); e == nil {
		t.Fatal("revoked miss bypassed policy")
	}
}

func TestScopedIndexFinalRecheckAndGeneration(t *testing.T) {
	ctx := context.Background()
	calls := 0
	revokeAt := 0
	policy := indexPolicyFunc(func(context.Context, IndexScope) error {
		calls++
		if calls == revokeAt {
			return errors.New("changed during query")
		}
		return nil
	})
	x, e := NewScopedIndex(ctx, indexTestScope(), indexAuthFunc(indexRead), policy, []IndexEntry{indexTestEntry("mail", "mail")})
	if e != nil {
		t.Fatal(e)
	}
	revokeAt = calls + 2
	if r, e := x.Search(ctx, indexQuery("mail")); e == nil || len(r.Candidates) != 0 {
		t.Fatal("final check leaked")
	}
	revokeAt = 0
	x.auth = indexAuthFunc(func(c context.Context, p ports.Principal, a ports.Action, r *ports.ArtifactRef) (ports.Decision, error) {
		x.auth = indexAuthFunc(indexRead)
		if _, e := x.Update(c, 1, []IndexEntry{indexTestEntry("other", "other")}, nil); e != nil {
			t.Fatal(e)
		}
		return ports.Decision{Allowed: true}, nil
	})
	if r, e := x.Search(ctx, indexQuery("mail")); !errors.Is(e, ErrIndexGeneration) || len(r.Candidates) != 0 {
		t.Fatalf("generation race %+v %v", r, e)
	}
}

func TestScopedIndexUpdatesRollbackAndCancellation(t *testing.T) {
	ctx := context.Background()
	a := indexTestEntry("a", "alpha")
	x := newIndexTest(t, a)
	n, e := x.Update(ctx, 1, []IndexEntry{indexTestEntry("a", "beta"), indexTestEntry("b", "gamma")}, nil)
	if e != nil || n != 2 {
		t.Fatalf("%d %v", n, e)
	}
	r, e := x.Search(ctx, indexQuery("alpha"))
	if e != nil || len(r.Candidates) != 0 {
		t.Fatal("old postings remain")
	}
	if _, e = x.Update(ctx, 1, nil, nil); !errors.Is(e, ErrIndexGeneration) {
		t.Fatal("stale update accepted")
	}
	n, e = x.Rollback(ctx, 2)
	if e != nil || n != 3 {
		t.Fatalf("rollback %d %v", n, e)
	}
	r, e = x.Search(ctx, indexQuery("alpha"))
	if e != nil || len(r.Candidates) != 1 || r.Generation != 3 {
		t.Fatalf("rollback content %+v %v", r, e)
	}
	if _, ok, e := x.Exact(ctx, indexTestScope(), indexTestEntry("b", "").Record.Ref); e != nil || ok {
		t.Fatal("rollback left later entry")
	}
	n, e = x.Rebuild(ctx, 3, []IndexEntry{indexTestEntry("c", "delta")})
	if e != nil || n != 4 {
		t.Fatalf("rebuild %d %v", n, e)
	}
	q := indexQuery("delta")
	q.Generation = 3
	if _, e = x.Search(ctx, q); !errors.Is(e, ErrIndexGeneration) {
		t.Fatal("old generation accepted")
	}
	c, cancel := context.WithCancel(ctx)
	cancel()
	if _, e = x.Search(c, indexQuery("delta")); !errors.Is(e, context.Canceled) {
		t.Fatal(e)
	}
	if _, e = x.Update(c, 4, []IndexEntry{a}, nil); !errors.Is(e, context.Canceled) {
		t.Fatal(e)
	}
	if _, e = x.Rebuild(c, 4, []IndexEntry{a}); !errors.Is(e, context.Canceled) {
		t.Fatal(e)
	}
	if _, e = x.Rollback(c, 4); !errors.Is(e, context.Canceled) {
		t.Fatal(e)
	}
}

func indexBindingEntry(i int) IndexEntry {
	r := indexTestEntry(fmt.Sprintf("binding/%03d", i), "mail")
	r.Record.Ref.Kind = ports.KindBinding
	r.Record.Digest = ports.Digest{Algorithm: "sha256", Value: strings.Repeat("a", 64)}
	requirement := composition.Requirement{CapabilityID: "mail/read", ContractVersion: "1", ContractDigest: strings.Repeat("b", 64)}
	r.Bindings = []IndexedBinding{{Requirement: requirement, Candidate: composition.Candidate{Binding: ports.ArtifactPin{Ref: r.Record.Ref, Digest: r.Record.Digest}, Tool: ports.ArtifactPin{Ref: ports.ArtifactRef{WorkspaceID: "private", Kind: ports.KindTool, ID: "read", Version: "1"}}, Provider: ports.ArtifactPin{Ref: ports.ArtifactRef{WorkspaceID: "private", Kind: ports.KindProvider, ID: "mail", Version: "1"}}, ContractDigest: requirement.ContractDigest, Admitted: true}}}
	return r
}
func TestScopedIndexBindingCompletenessAndUpdates(t *testing.T) {
	ctx := context.Background()
	entries := make([]IndexEntry, 65)
	for i := range entries {
		entries[i] = indexBindingEntry(i)
	}
	x := newIndexTest(t, entries...)
	q := composition.Query{Principal: indexTestScope().Principal, RuntimeID: "runtime", Requirement: entries[0].Bindings[0].Requirement, Limit: 64}
	r, e := x.Lookup(ctx, q)
	if e != nil || r.Complete || len(r.Items) != 0 {
		t.Fatalf("overflow %+v %v", r, e)
	}
	if _, e = x.Update(ctx, 1, nil, []ports.ArtifactRef{entries[64].Record.Ref}); e != nil {
		t.Fatal(e)
	}
	r, e = x.Lookup(ctx, q)
	if e != nil || !r.Complete || len(r.Items) != 64 {
		t.Fatalf("exact %+v %v", r, e)
	}
	q.Requirement.ContractDigest = strings.Repeat("c", 64)
	r, e = x.Lookup(ctx, q)
	if e != nil || !r.Complete || len(r.Items) != 0 {
		t.Fatal("digest key not isolated")
	}
	q.RuntimeID = "other"
	if _, e = x.Lookup(ctx, q); e == nil {
		t.Fatal("runtime isolation missing")
	}
}

func TestScopedIndexDenialAndInvalidBuild(t *testing.T) {
	ctx := context.Background()
	entry := indexTestEntry("private", "mail")
	deny := indexAuthFunc(func(context.Context, ports.Principal, ports.Action, *ports.ArtifactRef) (ports.Decision, error) {
		return ports.Decision{}, nil
	})
	if _, e := NewScopedIndex(ctx, indexTestScope(), deny, indexPolicyFunc(indexAllow), []IndexEntry{entry}); e == nil {
		t.Fatal("unauthorized ingest")
	}
	x := newIndexTest(t, entry)
	x.auth = deny
	if r, e := x.Search(ctx, indexQuery("mail")); e == nil || len(r.Candidates) != 0 {
		t.Fatal("revoked record exposed")
	}
	if _, e := NewScopedIndex(ctx, indexTestScope(), indexAuthFunc(indexRead), nil, nil); e == nil {
		t.Fatal("missing policy allowed")
	}
	for _, mutate := range []func(*IndexEntry){func(e *IndexEntry) { e.Record.Ref.WorkspaceID = "foreign" }, func(e *IndexEntry) { e.Record.State = "revoked" }, func(e *IndexEntry) { e.Record.Metadata = []byte("{") }, func(e *IndexEntry) { e.Record.Metadata = make([]byte, MaxIndexMetadataBytes+1) }} {
		e := entry
		mutate(&e)
		if _, err := NewScopedIndex(ctx, indexTestScope(), indexAuthFunc(indexRead), indexPolicyFunc(indexAllow), []IndexEntry{e}); err == nil {
			t.Fatal("invalid ingest")
		}
	}
}

func TestScopedIndexConcurrentReadersAndPublication(t *testing.T) {
	ctx := context.Background()
	x := newIndexTest(t, indexTestEntry("mail", "mail"))
	var wg sync.WaitGroup
	for i := 0; i < 4; i++ {
		wg.Add(1)
		go func() {
			defer wg.Done()
			for j := 0; j < 100; j++ {
				r, e := x.Search(ctx, indexQuery("mail"))
				if e != nil && !errors.Is(e, ErrIndexGeneration) {
					t.Error(e)
				}
				if e == nil && len(r.Candidates) != 1 {
					t.Error("mixed generation")
				}
			}
		}()
	}
	for i := uint64(1); i < 30; i++ {
		if _, e := x.Update(ctx, i, []IndexEntry{indexTestEntry("mail", fmt.Sprintf("mail generation %d", i))}, nil); e != nil {
			t.Fatal(e)
		}
	}
	wg.Wait()
}

func TestScopedIndexBindingClosureRevocation(t *testing.T) {
	for _, kind := range []ports.ArtifactKind{ports.KindBinding, ports.KindTool, ports.KindProvider} {
		t.Run(string(kind), func(t *testing.T) {
			ctx := context.Background()
			entry := indexBindingEntry(0)
			x := newIndexTest(t, entry)
			x.auth = indexAuthFunc(func(_ context.Context, _ ports.Principal, _ ports.Action, r *ports.ArtifactRef) (ports.Decision, error) {
				return ports.Decision{Allowed: r.Kind != kind}, nil
			})
			q := composition.Query{Principal: indexTestScope().Principal, RuntimeID: "runtime", Requirement: entry.Bindings[0].Requirement, Limit: 64}
			r, e := x.Lookup(ctx, q)
			if e == nil || len(r.Items) != 0 {
				t.Fatalf("revoked %s exposed %+v %v", kind, r, e)
			}
		})
	}
}

func TestScopedIndexInputOwnershipAndRollbackRevocation(t *testing.T) {
	ctx := context.Background()
	scope := indexTestScope()
	entry := indexTestEntry("mail", "mail")
	entries := []IndexEntry{entry}
	allowed := true
	policy := indexPolicyFunc(func(_ context.Context, s IndexScope) error {
		s.Principal.Scopes[0] = "mutated callback copy"
		if !allowed {
			return errors.New("revoked")
		}
		return nil
	})
	auth := indexAuthFunc(func(_ context.Context, p ports.Principal, _ ports.Action, _ *ports.ArtifactRef) (ports.Decision, error) {
		p.Scopes[0] = "mutated auth copy"
		return ports.Decision{Allowed: true}, nil
	})
	x, e := NewScopedIndex(ctx, scope, auth, policy, entries)
	if e != nil {
		t.Fatal(e)
	}
	scope.Principal.Scopes[0] = "mutated source"
	entries[0].Record.Metadata[0] = 'X'
	if r, e := x.Search(ctx, indexQuery("mail")); e != nil || len(r.Candidates) != 1 {
		t.Fatalf("caller mutation changed index %+v %v", r, e)
	}
	if _, e = x.Update(ctx, 1, []IndexEntry{indexTestEntry("new", "new")}, nil); e != nil {
		t.Fatal(e)
	}
	allowed = false
	if _, e = x.Rollback(ctx, 2); e == nil {
		t.Fatal("rollback restored revoked authority")
	}
	allowed = true
	if _, e = x.Rollback(ctx, 2); e != nil {
		t.Fatal(e)
	}
}

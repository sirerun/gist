package discovery

import (
	"context"
	"errors"
	"testing"

	"github.com/sirerun/gist/hosted/internal/ports"
)

type flippingAuthorizer struct {
	allow bool
	calls int
}

func (a *flippingAuthorizer) Decide(_ context.Context, _ ports.Principal, _ ports.Action, _ *ports.ArtifactRef) (ports.Decision, error) {
	a.calls++
	return ports.Decision{Allowed: a.allow}, nil
}

func TestCacheHitRechecksAuthorization(t *testing.T) {
	search := &searchDouble{page: ports.SearchPage{Records: []ports.CatalogRecord{{Ref: ports.ArtifactRef{WorkspaceID: "w", Kind: ports.KindSkill, ID: "s", Version: "1"}, Metadata: []byte(`{"summary":"match"}`)}}}}
	auth := &flippingAuthorizer{allow: true}
	svc, err := New(search, auth)
	if err != nil {
		t.Fatal(err)
	}
	req := Request{Principal: ports.Principal{Subject: "p", WorkspaceID: "w", PolicyGeneration: 1}, Query: "match", MaxBytes: 4096}
	if _, err := svc.Search(context.Background(), req); err != nil {
		t.Fatal(err)
	}
	auth.allow = false
	got, err := svc.Search(context.Background(), req)
	if err != nil {
		t.Fatal(err)
	}
	if len(got.Candidates) != 0 || auth.calls != 2 || search.calls != 1 {
		t.Fatalf("cache bypassed policy: result=%+v auth=%d search=%d", got, auth.calls, search.calls)
	}
}

type failingAuthorizer struct{}

func (failingAuthorizer) Decide(context.Context, ports.Principal, ports.Action, *ports.ArtifactRef) (ports.Decision, error) {
	return ports.Decision{}, errors.New("policy store unavailable")
}

func TestPolicyFailureDoesNotDegradeToAllow(t *testing.T) {
	search := &searchDouble{page: ports.SearchPage{Records: []ports.CatalogRecord{{Ref: ports.ArtifactRef{WorkspaceID: "w", Kind: ports.KindSkill, ID: "s", Version: "1"}}}}}
	svc, err := New(search, failingAuthorizer{})
	if err != nil {
		t.Fatal(err)
	}
	if _, err := svc.Search(context.Background(), Request{Principal: ports.Principal{Subject: "p", WorkspaceID: "w"}, MaxBytes: 4096}); err == nil {
		t.Fatal("policy failure must fail closed")
	}
}

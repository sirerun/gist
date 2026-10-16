package discovery

import (
	"context"
	"errors"
	"testing"

	"github.com/sirerun/gist/catalog/ports"
)

type flippingAuthorizer struct {
	allow bool
	calls int
}

func (a *flippingAuthorizer) Decide(_ context.Context, _ ports.Principal, _ ports.Action, _ *ports.ArtifactRef) (ports.Decision, error) {
	a.calls++
	return ports.Decision{Allowed: a.allow}, nil
}

func TestRepeatedSearchRechecksAuthorization(t *testing.T) {
	search := &searchDouble{page: ports.SearchPage{Records: []ports.CatalogRecord{{State: "published", Ref: ports.ArtifactRef{WorkspaceID: "w", Kind: ports.KindSkill, ID: "s", Version: "1"}, Metadata: []byte(`{"summary":"match"}`)}}}}
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
	if len(got.Candidates) != 0 || auth.calls != 2 || search.calls != 2 {
		t.Fatalf("cache bypassed policy: result=%+v auth=%d search=%d", got, auth.calls, search.calls)
	}
}

type failingAuthorizer struct{}

func (failingAuthorizer) Decide(context.Context, ports.Principal, ports.Action, *ports.ArtifactRef) (ports.Decision, error) {
	return ports.Decision{}, errors.New("policy store unavailable")
}

func TestPolicyFailureDoesNotDegradeToAllow(t *testing.T) {
	search := &searchDouble{page: ports.SearchPage{Records: []ports.CatalogRecord{{State: "published", Ref: ports.ArtifactRef{WorkspaceID: "w", Kind: ports.KindSkill, ID: "s", Version: "1"}}}}}
	svc, err := New(search, failingAuthorizer{})
	if err != nil {
		t.Fatal(err)
	}
	if _, err := svc.Search(context.Background(), Request{Principal: ports.Principal{Subject: "p", WorkspaceID: "w"}, MaxBytes: 4096}); err == nil {
		t.Fatal("policy failure must fail closed")
	}
}

func TestRepeatedSearchRefreshesRevocationAndObjectRequirements(t *testing.T) {
	search := &searchDouble{page: ports.SearchPage{Records: []ports.CatalogRecord{{State: "published", Ref: ports.ArtifactRef{WorkspaceID: "w", Kind: ports.KindSkill, ID: "inbox", Version: "1"}, Metadata: []byte(`{"summary":"inbox","required_capabilities":[{"id":"mail.read","contract_version":"1"}]}`)}}}}
	svc, err := New(search, &flippingAuthorizer{allow: true})
	if err != nil {
		t.Fatal(err)
	}
	req := Request{Principal: ports.Principal{WorkspaceID: "w", Subject: "p"}, Query: "inbox", MaxBytes: 4096}
	got, err := svc.Search(context.Background(), req)
	if err != nil {
		t.Fatal(err)
	}
	if len(got.Candidates) != 1 || got.Candidates[0].RequiredCapabilityCount != 1 {
		t.Fatalf("bad object requirement: %+v", got)
	}
	search.page.Records[0].State = "revoked"
	got, err = svc.Search(context.Background(), req)
	if err != nil {
		t.Fatal(err)
	}
	if len(got.Candidates) != 0 {
		t.Fatal("revoked cached record exposed")
	}
	search.page.Records[0].Ref.WorkspaceID = "other"
	if _, err = svc.Search(context.Background(), req); err == nil {
		t.Fatal("foreign workspace accepted")
	}
}

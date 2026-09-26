package identity

import (
	"context"
	"errors"
	"testing"

	"github.com/sirerun/gist/hosted/internal/ports"
)

type authDouble struct {
	decision ports.Decision
	err      error
	got      ports.Principal
}

func (a *authDouble) Decide(_ context.Context, p ports.Principal, _ ports.Action, _ *ports.ArtifactRef) (ports.Decision, error) {
	a.got = p
	return a.decision, a.err
}

func authContext() AuthContext {
	return AuthContext{principal: ports.Principal{Issuer: "i", Subject: "s", Audience: "a", WorkspaceID: "workspace-a", Scopes: []string{"catalog:read"}, PolicyGeneration: 3, SubjectType: "workload"}, jti: "j"}
}

func TestAuthorizeUsesOneContextAndIgnoresForeignWorkspace(t *testing.T) {
	d := &authDouble{decision: ports.Decision{Allowed: true, Status: 200}}
	p, err := NewPolicyAuthorizer(d)
	if err != nil {
		t.Fatal(err)
	}
	auth := authContext()
	got := Authorize(context.Background(), auth, p, ports.ActionRead, &ports.ArtifactRef{WorkspaceID: "workspace-b", ID: "secret"})
	if got.Status != 404 || got.Code != "not_found" {
		t.Fatalf("foreign workspace oracle: %#v", got)
	}
	if d.got.Subject != "" {
		t.Fatal("foreign reference reached policy store")
	}
	got = Authorize(context.Background(), auth, p, ports.ActionRead, &ports.ArtifactRef{WorkspaceID: "workspace-a", ID: "ok"})
	if !got.Allowed || d.got.WorkspaceID != "workspace-a" {
		t.Fatalf("authorized decision: %#v principal=%#v", got, d.got)
	}
}

func TestAuthorizeFailsClosedAndPrivateDenialShape(t *testing.T) {
	d := &authDouble{err: errors.New("policy down")}
	p, _ := NewPolicyAuthorizer(d)
	got := Authorize(context.Background(), authContext(), p, ports.ActionRead, nil)
	if got.Status != 503 || got.Code != "service_unavailable" {
		t.Fatalf("outage: %#v", got)
	}
	d.err = nil
	d.decision = ports.Decision{Allowed: false, Status: 404, Reason: "missing"}
	missing := Authorize(context.Background(), authContext(), p, ports.ActionRead, &ports.ArtifactRef{WorkspaceID: "workspace-a", ID: "missing"})
	d.decision = ports.Decision{Allowed: false, Status: 404, Reason: "foreign"}
	foreign := Authorize(context.Background(), authContext(), p, ports.ActionRead, &ports.ArtifactRef{WorkspaceID: "workspace-a", ID: "foreign"})
	if missing.Status != foreign.Status || missing.Code != foreign.Code || missing.Reason != foreign.Reason {
		t.Fatalf("private denial oracle: %#v %#v", missing, foreign)
	}
}

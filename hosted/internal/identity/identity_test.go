package identity

import (
	"context"
	"errors"
	"testing"
	"time"
)

type testClock struct{ now time.Time }

func (c *testClock) Now() time.Time          { return c.now }
func (c *testClock) Advance(d time.Duration) { c.now = c.now.Add(d) }

type policyDouble struct {
	policy Policy
	err    error
	calls  int
}

func (p *policyDouble) CheckWorkload(_ context.Context, subject, workspace string, scopes []string, generation uint64) (Policy, error) {
	p.calls++
	if p.err != nil {
		return Policy{}, p.err
	}
	if subject == "" || workspace == "" || len(scopes) == 0 {
		return Policy{}, errors.New("invalid request")
	}
	if generation != 0 && generation != p.policy.PolicyGeneration {
		return Policy{Allowed: false, PolicyGeneration: p.policy.PolicyGeneration}, nil
	}
	return p.policy, nil
}

func newTestIssuer(t *testing.T) (*WorkloadIssuer, *testClock, *policyDouble) {
	t.Helper()
	clock := &testClock{now: time.Unix(1_700_000_000, 0).UTC()}
	key, err := GenerateSigningKey("k1", clock.now)
	if err != nil {
		t.Fatal(err)
	}
	keys, err := NewKeySet(clock, key)
	if err != nil {
		t.Fatal(err)
	}
	policy := &policyDouble{policy: Policy{Allowed: true, PolicyGeneration: 7, ParentSubject: "parent", ParentScopes: []string{"catalog:read", "catalog:publish"}}}
	issuer, err := NewWorkloadIssuer(Config{Issuer: "https://gist.example", Audience: "https://gist.example", Clock: clock, Keys: keys, Policy: policy})
	if err != nil {
		t.Fatal(err)
	}
	return issuer, clock, policy
}

func TestWorkloadMintAndVerify(t *testing.T) {
	issuer, _, policy := newTestIssuer(t)
	token, err := issuer.Mint(context.Background(), WorkloadRequest{Subject: "workload-a", WorkspaceID: "workspace-a", Scopes: []string{"catalog:read"}, ParentSubject: "parent", ParentScopes: []string{"catalog:read", "catalog:publish"}})
	if err != nil {
		t.Fatal(err)
	}
	verified, err := issuer.Verify(context.Background(), token)
	if err != nil {
		t.Fatal(err)
	}
	if verified.Principal.WorkspaceID != "workspace-a" || verified.Principal.Scopes[0] != "catalog:read" || policy.calls != 2 {
		t.Fatalf("unexpected verification: %#v calls=%d", verified, policy.calls)
	}
}

func TestWorkloadRejectsScopeEscalationAndPolicyOutage(t *testing.T) {
	issuer, _, policy := newTestIssuer(t)
	if _, err := issuer.Mint(context.Background(), WorkloadRequest{Subject: "w", WorkspaceID: "workspace-a", Scopes: []string{"catalog:publish"}, ParentSubject: "parent", ParentScopes: []string{"catalog:read"}}); !errors.Is(err, ErrUnauthorized) {
		t.Fatalf("scope escalation: %v", err)
	}
	policy.err = errors.New("database down")
	if _, err := issuer.Mint(context.Background(), WorkloadRequest{Subject: "w", WorkspaceID: "workspace-a", Scopes: []string{"catalog:read"}, ParentSubject: "parent", ParentScopes: []string{"catalog:read"}}); err == nil {
		t.Fatal("policy outage granted issuance")
	}
}

func TestWorkloadExpiryAndWrongAudience(t *testing.T) {
	issuer, clock, _ := newTestIssuer(t)
	token, err := issuer.Mint(context.Background(), WorkloadRequest{Subject: "w", WorkspaceID: "workspace-a", Scopes: []string{"catalog:read"}, ParentSubject: "parent", ParentScopes: []string{"catalog:read"}, TTL: time.Minute})
	if err != nil {
		t.Fatal(err)
	}
	clock.Advance(2 * time.Minute)
	if _, err := issuer.Verify(context.Background(), token); !errors.Is(err, ErrExpired) {
		t.Fatalf("expiry: %v", err)
	}
}

// Regression: Mint must enforce the scope ceiling from the stored policy, not
// from caller-supplied ParentScopes.
func TestMintEnforcesStoredPolicyScopeCeiling(t *testing.T) {
	issuer, _, _ := newTestIssuer(t)
	_, err := issuer.Mint(context.Background(), WorkloadRequest{Subject: "w", WorkspaceID: "workspace-a", Scopes: []string{"catalog:admin"}, ParentSubject: "parent", ParentScopes: []string{"catalog:admin"}})
	if !errors.Is(err, ErrUnauthorized) {
		t.Fatalf("caller-supplied parent scopes widened the ceiling: %v", err)
	}
	token, err := issuer.Mint(context.Background(), WorkloadRequest{Subject: "w", WorkspaceID: "workspace-a", Scopes: []string{"catalog:read"}, ParentScopes: []string{"catalog:read", "catalog:admin"}})
	if err != nil {
		t.Fatal(err)
	}
	verified, err := issuer.Verify(context.Background(), token)
	if err != nil {
		t.Fatal(err)
	}
	for _, s := range verified.ParentScopes {
		if s == "catalog:admin" {
			t.Fatalf("token parent_scopes carried caller-supplied scope: %v", verified.ParentScopes)
		}
	}
}

package identity

import (
	"context"
	"errors"
	"testing"
	"time"
)

func TestWorkloadRevocationLeaseAndPolicyGeneration(t *testing.T) {
	issuer, clock, policy := newTestIssuer(t)
	lease, err := NewRevocationLease(time.Minute)
	if err != nil {
		t.Fatal(err)
	}
	issuer.cfg.Revocations = lease
	token, err := issuer.Mint(context.Background(), WorkloadRequest{Subject: "w", WorkspaceID: "workspace-a", Scopes: []string{"catalog:read"}, ParentSubject: "parent", ParentScopes: []string{"catalog:read"}})
	if err != nil {
		t.Fatal(err)
	}
	verified, err := issuer.Verify(context.Background(), token)
	if err != nil {
		t.Fatal(err)
	}
	lease.Revoke(verified.JTI, clock.now)
	if _, err := issuer.Verify(context.Background(), token); !errors.Is(err, ErrReplay) {
		t.Fatalf("revoked replay: %v", err)
	}
	issuer.cfg.Revocations = nil
	policy.policy.PolicyGeneration = 8
	if _, err := issuer.Verify(context.Background(), token); !errors.Is(err, ErrRevoked) {
		t.Fatalf("generation change: %v", err)
	}
}

func TestKeyRotationAndUnknownKeyFailClosed(t *testing.T) {
	issuer, clock, policy := newTestIssuer(t)
	token, err := issuer.Mint(context.Background(), WorkloadRequest{Subject: "w", WorkspaceID: "workspace-a", Scopes: []string{"catalog:read"}, ParentSubject: "parent", ParentScopes: []string{"catalog:read"}})
	if err != nil {
		t.Fatal(err)
	}
	next, err := GenerateSigningKey("k2", clock.now)
	if err != nil {
		t.Fatal(err)
	}
	if err := issuer.cfg.Keys.Rotate(next); err != nil {
		t.Fatal(err)
	}
	if _, err := issuer.Verify(context.Background(), token); err != nil {
		t.Fatalf("fresh rotated key should retain old verification: %v", err)
	}
	policy.err = errors.New("issuer/policy outage")
	if _, err := issuer.Verify(context.Background(), token); err == nil {
		t.Fatal("outage granted access")
	}
}

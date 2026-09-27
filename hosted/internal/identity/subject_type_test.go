package identity

import (
	"context"
	"errors"
	"testing"
	"time"
)

func TestSubjectTypeClaim(t *testing.T) {
	issuer, _, _ := newTestIssuer(t)
	ctx := context.Background()
	for _, tc := range []struct{ in, want string }{{"", "workload"}, {"workload", "workload"}, {"human", "human"}} {
		token, err := issuer.Mint(ctx, WorkloadRequest{Subject: "s", WorkspaceID: "ws", Scopes: []string{"catalog:read"}, SubjectType: tc.in})
		if err != nil {
			t.Fatalf("mint %q: %v", tc.in, err)
		}
		got, err := issuer.Verify(ctx, token)
		if err != nil || got.Principal.SubjectType != tc.want {
			t.Fatalf("subject type %q: got %q err=%v", tc.in, got.Principal.SubjectType, err)
		}
	}
	if _, err := issuer.Mint(ctx, WorkloadRequest{Subject: "s", WorkspaceID: "ws", Scopes: []string{"catalog:read"}, SubjectType: "admin"}); !errors.Is(err, ErrUnauthorized) {
		t.Fatalf("unsupported subject type minted: %v", err)
	}
}

func TestVerificationKeysArePublicAndLive(t *testing.T) {
	clock := &testClock{now: time.Unix(1_700_000_000, 0).UTC()}
	k1, _ := GenerateSigningKey("k1", clock.now)
	k2, _ := GenerateSigningKey("k2", clock.now)
	keys, err := NewKeySet(clock, k1)
	if err != nil {
		t.Fatal(err)
	}
	if err := keys.Rotate(k2); err != nil {
		t.Fatal(err)
	}
	got := keys.VerificationKeys()
	if len(got) != 2 || got[0].KID != "k2" || got[1].KID != "k1" {
		t.Fatalf("keys after rotation = %+v", got)
	}
	clock.Advance(defaultMaxTokenAge + maxClockSkew + time.Second)
	got = keys.VerificationKeys()
	if len(got) != 1 || got[0].KID != "k2" {
		t.Fatalf("retired key still published: %+v", got)
	}
}

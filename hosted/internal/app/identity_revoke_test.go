package app

import (
	"context"
	"errors"
	"testing"
	"time"

	"github.com/sirerun/gist/hosted/internal/identity"
	"github.com/sirerun/gist/hosted/internal/ports"
	"github.com/sirerun/gist/hosted/internal/storage"
)

// revokeCatalog mirrors storage.Postgres.Revoke: only rows in the named
// workspace are visible, and a missing row is storage.ErrNotFound.
type revokeCatalog struct {
	records map[string]ports.IdentityRecord
	revoked []string
}

func (c *revokeCatalog) Lookup(_ context.Context, ws, iss, sub string) (ports.IdentityRecord, error) {
	rec, ok := c.records[key(ws, iss, sub)]
	if !ok {
		return ports.IdentityRecord{}, storage.ErrNotFound
	}
	return rec, nil
}

func (c *revokeCatalog) Revoke(_ context.Context, ws, iss, sub string) error {
	if _, ok := c.records[key(ws, iss, sub)]; !ok {
		return storage.ErrNotFound
	}
	c.revoked = append(c.revoked, key(ws, iss, sub))
	return nil
}

func revokeFixture() (verifiedIdentity, *revokeCatalog) {
	const iss = "https://issuer.test"
	cat := &revokeCatalog{records: map[string]ports.IdentityRecord{
		key("ws-a", iss, "caller-a"): {Issuer: iss, Subject: "caller-a", WorkspaceID: "ws-a"},
		key("ws-a", iss, "target-a"): {Issuer: iss, Subject: "target-a", WorkspaceID: "ws-a"},
		key("ws-b", iss, "target-b"): {Issuer: iss, Subject: "target-b", WorkspaceID: "ws-b"},
	}}
	tok := identity.VerifiedToken{
		Principal: ports.Principal{Issuer: iss, Subject: "caller-a", Audience: "aud", WorkspaceID: "ws-a", Scopes: []string{"identity:revoke"}},
		ExpiresAt: time.Now().Add(time.Minute),
	}
	return verifiedIdentity{issuer: fakeVerifier{tok: tok}, catalog: cat}, cat
}

func TestVerifiedIdentityRevokeUsesVerifiedWorkspace(t *testing.T) {
	v, cat := revokeFixture()
	ctx, rec, err := v.AuthenticateContext(context.Background(), "raw", "aud")
	if err != nil || rec.WorkspaceID != "ws-a" {
		t.Fatalf("AuthenticateContext rec=%+v err=%v", rec, err)
	}
	if err := v.Revoke(ctx, "https://issuer.test", "target-a"); err != nil {
		t.Fatalf("Revoke own-workspace identity: %v", err)
	}
	if len(cat.revoked) != 1 || cat.revoked[0] != key("ws-a", "https://issuer.test", "target-a") {
		t.Fatalf("revoked=%v", cat.revoked)
	}
}

func TestVerifiedIdentityRevokeCannotReachForeignWorkspace(t *testing.T) {
	v, cat := revokeFixture()
	ctx, _, err := v.AuthenticateContext(context.Background(), "raw", "aud")
	if err != nil {
		t.Fatal(err)
	}
	err = v.Revoke(ctx, "https://issuer.test", "target-b")
	if !errors.Is(err, ports.ErrIdentityNotFound) {
		t.Fatalf("foreign revoke err=%v want ErrIdentityNotFound", err)
	}
	if len(cat.revoked) != 0 {
		t.Fatalf("foreign identity was revoked: %v", cat.revoked)
	}
}

func TestVerifiedIdentityRevokeRequiresVerifiedContext(t *testing.T) {
	v, cat := revokeFixture()
	if err := v.Revoke(context.Background(), "https://issuer.test", "target-a"); !errors.Is(err, identity.ErrUnauthorized) {
		t.Fatalf("Revoke without auth context err=%v", err)
	}
	ctx, _, err := v.AuthenticateContext(context.Background(), "raw", "other-audience")
	if err == nil {
		t.Fatal("AuthenticateContext accepted a wrong audience")
	}
	if err := v.Revoke(ctx, "https://issuer.test", "target-a"); !errors.Is(err, identity.ErrUnauthorized) {
		t.Fatalf("Revoke after failed authentication err=%v", err)
	}
	if len(cat.revoked) != 0 {
		t.Fatalf("revoked=%v", cat.revoked)
	}
}

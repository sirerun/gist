package storage

import (
	"context"
	"testing"
)

// Lookup and Revoke run under row-level security, so a call without a
// workspace can never match a row. They must refuse it before touching the pool.
func TestIdentityRequiresWorkspace(t *testing.T) {
	s := &Postgres{}
	ctx := context.Background()
	cases := []struct{ workspace, issuer, subject string }{
		{"", "iss", "sub"},
		{"ws", "", "sub"},
		{"ws", "iss", ""},
	}
	for _, c := range cases {
		if _, err := s.Lookup(ctx, c.workspace, c.issuer, c.subject); err == nil {
			t.Fatalf("Lookup(%q, %q, %q): want error", c.workspace, c.issuer, c.subject)
		}
		if err := s.Revoke(ctx, c.workspace, c.issuer, c.subject); err == nil {
			t.Fatalf("Revoke(%q, %q, %q): want error", c.workspace, c.issuer, c.subject)
		}
	}
}

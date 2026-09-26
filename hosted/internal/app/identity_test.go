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

const (
	testAudience  = "https://registry.example.invalid"
	testIssuer    = "https://issuer.example.invalid"
	testSubject   = "agent-1"
	testWorkspace = "ws-1"
)

type fakeVerifier struct {
	tok identity.VerifiedToken
	err error
}

func (f fakeVerifier) Verify(context.Context, string) (identity.VerifiedToken, error) {
	return f.tok, f.err
}

type fakeCatalog struct {
	records map[string]ports.IdentityRecord
	revoked map[string]bool
	err     error
	calls   int
}

func key(ws, iss, sub string) string { return ws + "|" + iss + "|" + sub }

// Lookup mirrors storage.Postgres: revoked rows are filtered and surface as
// storage.ErrNotFound.
func (f *fakeCatalog) Lookup(_ context.Context, ws, iss, sub string) (ports.IdentityRecord, error) {
	f.calls++
	if f.err != nil {
		return ports.IdentityRecord{}, f.err
	}
	k := key(ws, iss, sub)
	r, ok := f.records[k]
	if !ok || f.revoked[k] {
		return ports.IdentityRecord{}, storage.ErrNotFound
	}
	return r, nil
}

func (f *fakeCatalog) Revoke(_ context.Context, ws, iss, sub string) error {
	if f.revoked == nil {
		f.revoked = map[string]bool{}
	}
	f.revoked[key(ws, iss, sub)] = true
	return nil
}

func validToken() identity.VerifiedToken {
	return identity.VerifiedToken{
		Principal: ports.Principal{Issuer: testIssuer, Subject: testSubject, Audience: testAudience, WorkspaceID: testWorkspace, Scopes: []string{"catalog:read"}},
		JTI:       "jti-1",
		ExpiresAt: time.Now().Add(5 * time.Minute),
	}
}

func activeCatalog() *fakeCatalog {
	return &fakeCatalog{records: map[string]ports.IdentityRecord{
		key(testWorkspace, testIssuer, testSubject): {Issuer: testIssuer, Subject: testSubject, WorkspaceID: testWorkspace},
	}}
}

func TestVerifiedIdentityLookupChecksStoredRecord(t *testing.T) {
	t.Parallel()
	revoked := activeCatalog()
	revoked.revoked = map[string]bool{key(testWorkspace, testIssuer, testSubject): true}
	otherWorkspace := &fakeCatalog{records: map[string]ports.IdentityRecord{
		key("ws-other", testIssuer, testSubject): {Issuer: testIssuer, Subject: testSubject, WorkspaceID: "ws-other"},
	}}
	cases := []struct {
		name    string
		catalog identityCatalog
		wantOK  bool
	}{
		{name: "active identity is accepted", catalog: activeCatalog(), wantOK: true},
		{name: "revoked identity is rejected", catalog: revoked},
		{name: "missing identity is rejected", catalog: &fakeCatalog{}},
		{name: "identity in another workspace is rejected", catalog: otherWorkspace},
		{name: "storage error fails closed", catalog: &fakeCatalog{err: errors.New("connection reset")}},
		{name: "nil catalog fails closed", catalog: nil},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			t.Parallel()
			v := verifiedIdentity{issuer: fakeVerifier{tok: validToken()}, catalog: tc.catalog}
			got, err := v.Lookup(context.Background(), "token", testAudience)
			if tc.wantOK {
				if err != nil {
					t.Fatalf("Lookup() error = %v, want nil", err)
				}
				if got.Subject != testSubject || got.WorkspaceID != testWorkspace || got.Issuer != testIssuer {
					t.Fatalf("Lookup() = %+v, want principal from verified token", got)
				}
				return
			}
			if !errors.Is(err, identity.ErrUnauthorized) {
				t.Fatalf("Lookup() error = %v, want identity.ErrUnauthorized", err)
			}
			if got.Subject != "" || got.WorkspaceID != "" || got.Issuer != "" || len(got.Scopes) != 0 {
				t.Fatalf("Lookup() record = %+v, want zero value on rejection", got)
			}
		})
	}
}

func TestVerifiedIdentityLookupRejectsAfterRevoke(t *testing.T) {
	t.Parallel()
	cat := activeCatalog()
	v := verifiedIdentity{issuer: fakeVerifier{tok: validToken()}, catalog: cat}
	if _, err := v.Lookup(context.Background(), "token", testAudience); err != nil {
		t.Fatalf("Lookup() before revoke error = %v", err)
	}
	if err := cat.Revoke(context.Background(), testWorkspace, testIssuer, testSubject); err != nil {
		t.Fatalf("Revoke() error = %v", err)
	}
	if _, err := v.Lookup(context.Background(), "token", testAudience); !errors.Is(err, identity.ErrUnauthorized) {
		t.Fatalf("Lookup() after revoke error = %v, want identity.ErrUnauthorized", err)
	}
}

func TestVerifiedIdentityLookupSkipsStorageOnBadToken(t *testing.T) {
	t.Parallel()
	cases := []struct {
		name     string
		verifier fakeVerifier
		audience string
	}{
		{name: "verify error", verifier: fakeVerifier{err: identity.ErrUnauthorized}, audience: testAudience},
		{name: "audience mismatch", verifier: fakeVerifier{tok: validToken()}, audience: "https://other.example.invalid"},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			t.Parallel()
			cat := activeCatalog()
			v := verifiedIdentity{issuer: tc.verifier, catalog: cat}
			if _, err := v.Lookup(context.Background(), "token", tc.audience); !errors.Is(err, identity.ErrUnauthorized) {
				t.Fatalf("Lookup() error = %v, want identity.ErrUnauthorized", err)
			}
			if cat.calls != 0 {
				t.Fatalf("catalog Lookup calls = %d, want 0 for an unverified token", cat.calls)
			}
		})
	}
}

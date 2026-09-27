package oauth

import (
	"context"
	"errors"
	"fmt"

	"github.com/sirerun/gist/hosted/internal/identity"
	"github.com/sirerun/gist/hosted/internal/ports"
)

// IdentityRecorder persists the stored identity behind a minted token. It
// must refuse to revive a revoked identity.
type IdentityRecorder func(ctx context.Context, r ports.IdentityRecord) error

// IssuerMinter mints access tokens with one identity workload issuer per
// RFC 8707 resource. Every issuer shares the identity key set; each carries
// its resource as the only audience it signs and verifies, so a token minted
// for one resource fails verification at every other resource.
type IssuerMinter struct {
	Issuers map[string]*identity.WorkloadIssuer
	Record  IdentityRecorder
}

// MintAccess mints, reads the claims back from the signed token, and records
// the issued identity before returning the token. A token whose identity
// cannot be recorded is discarded.
func (m IssuerMinter) MintAccess(ctx context.Context, resource string, req identity.WorkloadRequest) (AccessToken, error) {
	issuer := m.Issuers[resource]
	if issuer == nil {
		return AccessToken{}, fmt.Errorf("oauth: no issuer for resource: %w", identity.ErrUnauthorized)
	}
	if m.Record == nil {
		return AccessToken{}, errors.New("oauth: identity recorder is unavailable")
	}
	token, err := issuer.Mint(ctx, req)
	if err != nil {
		return AccessToken{}, err
	}
	got, err := issuer.Verify(ctx, token)
	if err != nil {
		return AccessToken{}, fmt.Errorf("oauth: verify minted token: %w", err)
	}
	p := got.Principal
	record := ports.IdentityRecord{Issuer: p.Issuer, Subject: p.Subject, WorkspaceID: p.WorkspaceID, SubjectType: p.SubjectType, Scopes: p.Scopes, PolicyGeneration: p.PolicyGeneration, ExpiresAt: got.ExpiresAt.Unix()}
	if err := m.Record(ctx, record); err != nil {
		return AccessToken{}, err
	}
	return AccessToken{Token: token, ExpiresAt: got.ExpiresAt}, nil
}

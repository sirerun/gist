package identity

import (
	"context"
	"github.com/sirerun/gist/hosted/internal/ports"
)

type contextKey struct{}

// AuthContext is created only by VerifyContext. Its principal is copied so
// callers cannot mutate the verified authority or select another workspace.
type AuthContext struct {
	principal ports.Principal
	jti       string
}

func VerifyContext(ctx context.Context, issuer *WorkloadIssuer, token string) (context.Context, AuthContext, error) {
	verified, err := issuer.Verify(ctx, token)
	if err != nil {
		return ctx, AuthContext{}, err
	}
	auth := AuthContext{principal: verified.Principal, jti: verified.JTI}
	return context.WithValue(ctx, contextKey{}, auth), auth, nil
}

func FromContext(ctx context.Context) (AuthContext, bool) {
	auth, ok := ctx.Value(contextKey{}).(AuthContext)
	return auth, ok
}

func (a AuthContext) Principal() ports.Principal {
	return ports.Principal{Issuer: a.principal.Issuer, Subject: a.principal.Subject, Audience: a.principal.Audience, WorkspaceID: a.principal.WorkspaceID, Scopes: append([]string(nil), a.principal.Scopes...), PolicyGeneration: a.principal.PolicyGeneration, SubjectType: a.principal.SubjectType}
}

func (a AuthContext) JTI() string { return a.jti }

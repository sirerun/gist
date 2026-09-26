package identity

import (
	"context"
	"fmt"
	"sort"

	"github.com/sirerun/gist/hosted/internal/ports"
)

// PolicyStore is the online membership and policy-generation oracle. A
// missing or unavailable oracle is never interpreted as approval.
type PolicyStore interface {
	CheckWorkload(context.Context, string, string, []string, uint64) (Policy, error)
}

type Policy struct {
	Allowed          bool
	PolicyGeneration uint64
	ParentSubject    string
	ParentScopes     []string
	Revoked          bool
}

type Authorizer struct{ store ports.Authorizer }

func NewPolicyAuthorizer(store ports.Authorizer) (*Authorizer, error) {
	if store == nil {
		return nil, fmt.Errorf("policy authorizer is required")
	}
	return &Authorizer{store: store}, nil
}

func (a *Authorizer) Decide(ctx context.Context, principal ports.Principal, action ports.Action, ref *ports.ArtifactRef) (ports.Decision, error) {
	if principal.Issuer == "" || principal.Subject == "" || principal.Audience == "" || principal.WorkspaceID == "" || principal.PolicyGeneration == 0 {
		return ports.Decision{Status: 401, Reason: "invalid principal"}, ErrUnauthorized
	}
	decision, err := a.store.Decide(ctx, principal, action, ref)
	if err != nil {
		return ports.Decision{Status: 503, Reason: "policy unavailable"}, fmt.Errorf("evaluate authorization policy: %w", err)
	}
	return decision, nil
}

func normalizeScopes(scopes []string) []string {
	copyScopes := append([]string(nil), scopes...)
	sort.Strings(copyScopes)
	return copyScopes
}

func containsAll(have, want []string) bool {
	set := make(map[string]struct{}, len(have))
	for _, scope := range have {
		set[scope] = struct{}{}
	}
	for _, scope := range want {
		if _, ok := set[scope]; !ok {
			return false
		}
	}
	return true
}

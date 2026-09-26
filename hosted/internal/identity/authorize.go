package identity

import (
	"context"
	"fmt"

	"github.com/sirerun/gist/hosted/internal/ports"
)

type Authorization struct {
	Allowed   bool
	Status    int
	Code      string
	Reason    string
	Principal ports.Principal
}

func Authorize(ctx context.Context, auth AuthContext, policy *Authorizer, action ports.Action, ref *ports.ArtifactRef) Authorization {
	principal := auth.Principal()
	if principal.Subject == "" {
		return Authorization{Status: 401, Code: "unauthorized", Reason: "authentication required", Principal: principal}
	}
	if policy == nil {
		return Authorization{Status: 503, Code: "service_unavailable", Reason: "authorization unavailable", Principal: principal}
	}
	if ref != nil && ref.WorkspaceID != "" && ref.WorkspaceID != principal.WorkspaceID {
		return privateDeny(principal, "workspace mismatch")
	}
	decision, err := policy.Decide(ctx, principal, action, ref)
	if err != nil {
		if decision.Status == 401 {
			return Authorization{Status: 401, Code: "unauthorized", Reason: "authentication required", Principal: principal}
		}
		return Authorization{Status: 503, Code: "service_unavailable", Reason: "authorization unavailable", Principal: principal}
	}
	if decision.Allowed {
		return Authorization{Allowed: true, Status: 200, Principal: principal}
	}
	if decision.Status == 403 {
		return Authorization{Status: 403, Code: "forbidden", Reason: "forbidden", Principal: principal}
	}
	return privateDeny(principal, decision.Reason)
}

func privateDeny(principal ports.Principal, reason string) Authorization {
	return Authorization{Status: 404, Code: "not_found", Reason: "not found", Principal: principal}
}

func RequireScope(auth AuthContext, scope string) error {
	for _, s := range auth.Principal().Scopes {
		if s == scope {
			return nil
		}
	}
	return fmt.Errorf("required scope %q: %w", scope, ErrUnauthorized)
}

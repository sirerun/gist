package oauth

import (
	"errors"
	"net/http"
)

// refresh rotates a refresh token. The presented token is consumed and a new
// one issued in the same family; the family's resource, workspace and scope
// bounds never widen. Presenting an already-consumed token revokes the whole
// family, so a thief and the legitimate client both lose the grant.
func (s *Server) refresh(r *http.Request, form formValues, client Client) (tokenResponse, *grantError) {
	raw := form.Get("refresh_token")
	if raw == "" {
		return tokenResponse{}, &grantError{http.StatusBadRequest, "invalid_request", "refresh_token is required"}
	}
	workspace, ok := parseOpaque(refreshPrefix, raw)
	if !ok {
		return tokenResponse{}, invalidGrant("refresh token is invalid")
	}
	var requested []string
	if form.Has("scope") {
		parsed, valid := parseScope(form.Get("scope"))
		if !valid || len(parsed) == 0 {
			return tokenResponse{}, &grantError{http.StatusBadRequest, "invalid_scope", "scope is malformed"}
		}
		requested = parsed
	}
	resource := form.Get("resource")
	if form.Has("resource") && !s.resourceAllowed(resource) {
		return tokenResponse{}, &grantError{http.StatusBadRequest, "invalid_target", "unknown resource"}
	}
	next, err := newOpaque(refreshPrefix, workspace)
	if err != nil {
		return tokenResponse{}, &grantError{http.StatusInternalServerError, "server_error", ""}
	}
	now := s.now()
	family, err := s.cfg.Store.RotateRefresh(r.Context(), workspace, hashSecret(raw), func(f RefreshFamily, t RefreshToken) error {
		switch {
		case f.WorkspaceID != workspace:
			return invalidGrant("refresh token is invalid")
		case f.ClientID != client.ID:
			return invalidGrant("refresh token was issued to another client")
		case !now.Before(t.ExpiresAt):
			return invalidGrant("refresh token expired")
		case requested != nil && !containsAll(f.Scopes, requested):
			// Reject, never downgrade or widen.
			return &grantError{http.StatusBadRequest, "invalid_scope", "requested scope exceeds the original grant"}
		case form.Has("resource") && resource != f.Resource:
			return &grantError{http.StatusBadRequest, "invalid_target", "resource does not match the original grant"}
		}
		return nil
	}, RefreshToken{Hash: hashSecret(next), IssuedAt: now, ExpiresAt: now.Add(s.cfg.RefreshTTL)})
	if err != nil {
		var ge *grantError
		switch {
		case errors.As(err, &ge):
			return tokenResponse{}, ge
		case errors.Is(err, ErrGrantReused):
			return tokenResponse{}, invalidGrant("refresh token reuse detected; the grant is revoked")
		case errors.Is(err, ErrFamilyRevoked), errors.Is(err, ErrNotFound):
			return tokenResponse{}, invalidGrant("refresh token is invalid or revoked")
		default:
			return tokenResponse{}, &grantError{http.StatusServiceUnavailable, "temporarily_unavailable", ""}
		}
	}
	scopes := family.Scopes
	if requested != nil {
		scopes = requested
	}
	access, gerr := s.mint(r, family.Resource, family.Subject, family.WorkspaceID, scopes)
	if gerr != nil {
		// Membership or policy no longer supports the grant: end the family
		// so no later refresh can revive it.
		_ = s.cfg.Store.RevokeFamily(r.Context(), family.WorkspaceID, family.ID)
		return tokenResponse{}, gerr
	}
	return s.tokenResponse(access, next, scopes), nil
}

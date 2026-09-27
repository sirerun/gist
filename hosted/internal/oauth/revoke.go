package oauth

import (
	"errors"
	"net/http"
	"strings"
)

// handleRevoke implements RFC 7009 for refresh tokens. Revoking a refresh
// token revokes its whole family. Access tokens are short-lived signed JWTs
// that cannot be revoked one at a time here, so they are refused with
// unsupported_token_type rather than acknowledged misleadingly. Unknown
// refresh tokens get 200, as RFC 7009 section 2.2 requires.
func (s *Server) handleRevoke(w http.ResponseWriter, r *http.Request) {
	form, client, gerr := s.tokenRequest(r)
	if gerr != nil {
		s.writeGrantError(w, gerr)
		return
	}
	raw := form.Get("token")
	if raw == "" {
		writeOAuthError(w, http.StatusBadRequest, "invalid_request", "token is required")
		return
	}
	workspace, ok := parseOpaque(refreshPrefix, raw)
	if !ok {
		if form.Get("token_type_hint") == "access_token" || (!strings.HasPrefix(raw, refreshPrefix+".") && strings.Count(raw, ".") == 2) {
			writeOAuthError(w, http.StatusBadRequest, "unsupported_token_type", "access tokens expire on their own and cannot be revoked individually")
			return
		}
		writeJSON(w, http.StatusOK, struct{}{})
		return
	}
	if err := s.cfg.Store.RevokeRefresh(r.Context(), workspace, hashSecret(raw), client.ID); err != nil && !errors.Is(err, ErrNotFound) {
		writeOAuthError(w, http.StatusServiceUnavailable, "temporarily_unavailable", "")
		return
	}
	writeJSON(w, http.StatusOK, struct{}{})
}

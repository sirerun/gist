package rest

import (
	"encoding/json"
	"errors"
	"net/http"

	"github.com/sirerun/gist/hosted/internal/ports"
)

// revokeIdentity handles POST /v1/identities/revoke. The workspace is the
// authenticated caller's own; the body names only issuer and subject, and any
// other field (including a workspace) is rejected. Revoking an identity that
// is already revoked succeeds again; a missing or foreign identity is a
// uniform 404.
func (h *Handler) revokeIdentity(w http.ResponseWriter, r *http.Request, p ports.Principal) error {
	if err := h.authorize(r.Context(), p, ports.ActionIdentityRevoke, nil); err != nil {
		return err
	}
	var in struct {
		Issuer  string `json:"issuer"`
		Subject string `json:"subject"`
	}
	dec := json.NewDecoder(http.MaxBytesReader(w, r.Body, h.limits.MaxBodyBytes))
	dec.DisallowUnknownFields()
	if err := dec.Decode(&in); err != nil || dec.More() {
		return appError("validation_failed", "Invalid request", 422, false)
	}
	if in.Issuer == "" || in.Subject == "" || len(in.Issuer) > 512 || len(in.Subject) > 512 {
		return appError("validation_failed", "Invalid request", 422, false)
	}
	err := h.s.Identity.Revoke(r.Context(), in.Issuer, in.Subject)
	switch {
	case err == nil:
	case errors.Is(err, ports.ErrIdentityNotFound):
		return appError("not_found", "Not found", 404, false)
	default:
		return appError("service_unavailable", "Service unavailable", 503, true)
	}
	return h.writeJSON(w, map[string]any{"issuer": in.Issuer, "subject": in.Subject, "revoked": true}, budget(r, h.limits.MaxResponseBytes))
}

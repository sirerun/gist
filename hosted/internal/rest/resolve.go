package rest

import (
	"encoding/json"
	"github.com/sirerun/gist/hosted/internal/ports"
	"net/http"
)

func (h *Handler) resolve(w http.ResponseWriter, r *http.Request, p ports.Principal) error {
	var in map[string]any
	if err := decodeBody(r, h.limits.MaxBodyBytes, &in); err != nil {
		return err
	}
	if h.s.Resolver == nil {
		return appError("service_unavailable", "Resolution service unavailable", 503, true)
	}
	if err := h.authorize(r.Context(), p, ports.ActionRead, nil); err != nil {
		return err
	}
	raw, err := json.Marshal(in)
	if err != nil {
		return appError("validation_failed", "Invalid request", 422, false)
	}
	out, err := h.s.Resolver.Resolve(r.Context(), p, raw)
	if err != nil {
		return err
	}
	if len(out) > budget(r, h.limits.MaxResponseBytes) {
		return appError("budget_exceeded", "Response exceeds the requested byte budget", 413, false)
	}
	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(http.StatusOK)
	_, err = w.Write(out)
	return err
}

var _ = http.StatusOK

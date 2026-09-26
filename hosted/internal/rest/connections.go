package rest

import (
	"encoding/json"
	"github.com/sirerun/gist/hosted/internal/ports"
	"net/http"
)

func (h *Handler) createConnection(w http.ResponseWriter, r *http.Request, p ports.Principal) error {
	var in struct {
		Capability ports.ArtifactRef `json:"capability"`
	}
	if err := decodeBody(r, h.limits.MaxBodyBytes, &in); err != nil {
		return err
	}
	if in.Capability.ID == "" {
		return appError("validation_failed", "Invalid request", 422, false)
	}
	if err := h.authorize(r.Context(), p, ports.ActionRead, &in.Capability); err != nil {
		return err
	}
	if h.s.Connections == nil {
		return appError("service_unavailable", "Service unavailable", 503, true)
	}
	c, err := h.s.Connections.Begin(r.Context(), p, in.Capability)
	if err != nil {
		return appError("service_unavailable", "Service unavailable", 503, true)
	}
	return h.writeCreated(w, c, budget(r, h.limits.MaxResponseBytes))
}
func (h *Handler) getConnection(w http.ResponseWriter, r *http.Request, p ports.Principal, id string) error {
	if h.s.Connections == nil {
		return appError("service_unavailable", "Service unavailable", 503, true)
	}
	c, err := h.s.Connections.Get(r.Context(), p, id)
	if err != nil {
		return appError("not_found", "Not found", 404, false)
	}
	if err := h.authorize(r.Context(), p, ports.ActionRead, &c.Capability); err != nil {
		return appError("not_found", "Not found", 404, false)
	}
	return h.writeJSON(w, c, budget(r, h.limits.MaxResponseBytes))
}
func hJSON(v any) ([]byte, error) { return json.Marshal(v) }
func (h *Handler) writeCreated(w http.ResponseWriter, v any, max int) error {
	b, err := hJSON(v)
	if err != nil {
		return appError("service_unavailable", "Service unavailable", 503, true)
	}
	if max > 0 && len(b) > max {
		return appError("budget_exceeded", "Response exceeds the requested byte budget", 413, false)
	}
	w.Header().Set("Cache-Control", "private, no-store")
	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(http.StatusCreated)
	_, err = w.Write(b)
	return err
}

package rest

import (
	"github.com/sirerun/gist/hosted/internal/ports"
	"io"
	"net/http"
)

func (h *Handler) publish(w http.ResponseWriter, r *http.Request, p ports.Principal, kind string) error {
	if h.s.Publisher == nil {
		return appError("service_unavailable", "Service unavailable", 503, true)
	}
	if err := h.authorize(r.Context(), p, ports.ActionPublish, nil); err != nil {
		return err
	}
	var raw []byte
	var err error
	raw, err = readBody(r, h.limits.MaxBodyBytes)
	if err != nil {
		return err
	}
	k := ports.ArtifactKind(kind)
	switch kind {
	case "skills":
		k = ports.KindSkill
	case "capabilities":
		k = ports.KindCapability
	case "tools":
		k = ports.KindTool
	case "providers":
		k = ports.KindProvider
	case "bindings":
		k = ports.KindBinding
	case "taxonomies":
		k = ports.KindTaxonomy
	}
	switch k {
	case ports.KindSkill, ports.KindCapability, ports.KindTool, ports.KindProvider, ports.KindBinding, ports.KindTaxonomy:
	default:
		if kind != "revocations" {
			return appError("validation_failed", "Invalid request", 422, false)
		}
	}
	out, err := h.s.Publisher.Publish(r.Context(), p, k, raw)
	if err != nil {
		return err
	}
	if len(out) > h.limits.MaxResponseBytes {
		return appError("budget_exceeded", "Response exceeds the requested byte budget", 413, false)
	}
	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(http.StatusCreated)
	_, err = w.Write(out)
	return err
}
func readBody(r *http.Request, limit int64) ([]byte, error) {
	b, err := io.ReadAll(io.LimitReader(r.Body, limit+1))
	if err != nil {
		return nil, appError("validation_failed", "Invalid request", 422, false)
	}
	if int64(len(b)) > limit {
		return nil, appError("budget_exceeded", "Request exceeds the requested byte budget", 413, false)
	}
	return b, nil
}

package rest

import (
	"github.com/sirerun/gist/hosted/internal/ports"
	"net/http"
)

func (h *Handler) batch(w http.ResponseWriter, r *http.Request, p ports.Principal) error {
	var in struct {
		References []ports.ArtifactRef `json:"references"`
		MaxBytes   int                 `json:"max_bytes"`
	}
	if err := decodeBody(r, h.limits.MaxBodyBytes, &in); err != nil {
		return err
	}
	if len(in.References) == 0 || in.MaxBytes < 1 {
		return appError("validation_failed", "Invalid request", 422, false)
	}
	items := make([]any, 0, len(in.References))
	for _, ref := range in.References {
		if err := h.authorize(r.Context(), p, ports.ActionRead, &ref); err != nil {
			items = append(items, map[string]any{"reference": ref.ID, "kind": string(ref.Kind), "complete": true, "error": errorBody(err, "batch")})
			continue
		}
		if h.s.Catalog == nil {
			return appError("service_unavailable", "Service unavailable", 503, true)
		}
		rec, err := h.s.Catalog.Get(r.Context(), ref)
		if err != nil {
			items = append(items, map[string]any{"reference": ref.ID, "kind": string(ref.Kind), "complete": true, "error": Error{Code: "not_found", Message: "Not found", RequestID: "batch", Retryable: false}})
			continue
		}
		if rec.State == "revoked" {
			items = append(items, map[string]any{"reference": ref.ID, "kind": string(ref.Kind), "complete": true, "error": errorBody(ErrArtifactRevoked, "batch")})
			continue
		}
		items = append(items, map[string]any{"reference": ref.ID, "kind": string(ref.Kind), "complete": true, "artifact": rec})
	}
	return h.writeJSON(w, map[string]any{"items": items}, in.MaxBytes)
}

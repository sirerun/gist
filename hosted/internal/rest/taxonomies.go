package rest

import (
	"github.com/sirerun/gist/hosted/internal/ports"
	"net/http"
)

func (h *Handler) listByKind(w http.ResponseWriter, r *http.Request, p ports.Principal, kind ports.ArtifactKind) error {
	if h.s.Search == nil {
		return appError("service_unavailable", "Service unavailable", 503, true)
	}
	if err := h.authorize(r.Context(), p, ports.ActionRead, nil); err != nil {
		return err
	}
	page, err := h.s.Search.Search(r.Context(), ports.SearchQuery{Principal: p, Kinds: []ports.ArtifactKind{kind}, Limit: h.limits.MaxResults, MaxBytes: budget(r, h.limits.MaxResponseBytes)})
	if err != nil {
		return appError("service_unavailable", "Service unavailable", 503, true)
	}
	return h.writeJSON(w, map[string]any{"items": page.Records, "next_cursor": page.Next.ID}, budget(r, h.limits.MaxResponseBytes))
}

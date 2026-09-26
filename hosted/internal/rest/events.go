package rest

import (
	"github.com/sirerun/gist/hosted/internal/ports"
	"net/http"
)

func (h *Handler) events(w http.ResponseWriter, r *http.Request, p ports.Principal) error {
	if h.s.Events == nil {
		return appError("service_unavailable", "Service unavailable", 503, true)
	}
	if err := h.authorize(r.Context(), p, ports.ActionRead, nil); err != nil {
		return err
	}
	page, err := h.s.Events.Read(r.Context(), ports.Cursor{ID: r.URL.Query().Get("cursor"), WorkspaceID: p.WorkspaceID})
	if err != nil {
		return appError("service_unavailable", "Service unavailable", 503, true)
	}
	if page.RetentionGap {
		return appError("cursor_expired", "Cursor expired; resync required", 409, false)
	}
	return h.writeJSON(w, map[string]any{"events": page.Events, "next_cursor": page.Next.ID}, budget(r, h.limits.MaxResponseBytes))
}

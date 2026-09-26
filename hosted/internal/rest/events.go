package rest

import (
	"errors"
	"net/http"
	"time"

	"github.com/sirerun/gist/hosted/internal/events"
	"github.com/sirerun/gist/hosted/internal/ports"
)

// EventCursorOpener is implemented by event stores that can open a new cursor
// bound to a principal. GET /v1/events without a cursor parameter opens one.
type EventCursorOpener interface {
	NewCursor(ports.Principal, time.Duration) (ports.Cursor, error)
}

func (h *Handler) events(w http.ResponseWriter, r *http.Request, p ports.Principal) error {
	if h.s.Events == nil {
		return appError("service_unavailable", "Service unavailable", 503, true)
	}
	if err := h.authorize(r.Context(), p, ports.ActionRead, nil); err != nil {
		return err
	}
	cursor, err := h.eventCursor(r, p)
	if err != nil {
		return err
	}
	page, err := h.s.Events.Read(r.Context(), cursor)
	switch {
	case page.RetentionGap, errors.Is(err, events.ErrCursorExpired), errors.Is(err, events.ErrCursorBinding):
		// An expired, purged, unknown, or foreign cursor all require the
		// client to resynchronize; a foreign cursor is not disclosed as such.
		return appError("cursor_expired", "Cursor expired; resync required", 409, false)
	case err != nil:
		return appError("service_unavailable", "Service unavailable", 503, true)
	}
	return h.writeJSON(w, map[string]any{"events": page.Events, "next_cursor": page.Next.ID}, budget(r, h.limits.MaxResponseBytes))
}

// eventCursor resumes the client-supplied cursor bound to p, or opens a new
// one when the request carries no cursor.
func (h *Handler) eventCursor(r *http.Request, p ports.Principal) (ports.Cursor, error) {
	if id := r.URL.Query().Get("cursor"); id != "" {
		return ports.Cursor{ID: id, PrincipalHash: events.PrincipalHash(p), WorkspaceID: p.WorkspaceID}, nil
	}
	opener, ok := h.s.Events.(EventCursorOpener)
	if !ok {
		return ports.Cursor{}, appError("service_unavailable", "Service unavailable", 503, true)
	}
	c, err := opener.NewCursor(p, 0)
	if err != nil {
		return ports.Cursor{}, appError("service_unavailable", "Service unavailable", 503, true)
	}
	return c, nil
}

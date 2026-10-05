package rest

import (
	"context"
	"errors"
	"net/http"
	"time"

	"github.com/sirerun/gist/hosted/internal/events"
	"github.com/sirerun/gist/hosted/internal/ports"
)

// EventCursorOpener is implemented by event stores that can open a new cursor
// bound to a principal. HTTP opening instead requires AtomicEventPageOpener.
type EventCursorOpener interface {
	NewCursor(ports.Principal, time.Duration) (ports.Cursor, error)
}

// ContextEventCursorOpener supports durable stores with request cancellation.
type ContextEventCursorOpener interface {
	NewEventCursor(context.Context, ports.Principal, time.Duration) (ports.Cursor, error)
}

// BudgetedEventReader checks the exact response budget before committing cursor
// advancement or eviction and binds the read to the current authenticated policy.
type BudgetedEventReader interface {
	ReadEventPageForPrincipal(context.Context, ports.Principal, ports.Cursor, int) (ports.EventPage, error)
}

// AtomicEventPageOpener qualifies the first page budget and creates its cursor
// in one transaction. Failed responses must not leave cursors or evict clients.
type AtomicEventPageOpener interface {
	OpenEventPageForPrincipal(context.Context, ports.Principal, int) (ports.EventPage, error)
}

func (h *Handler) events(w http.ResponseWriter, r *http.Request, p ports.Principal) error {
	if h.s.Events == nil {
		return appError("service_unavailable", "Service unavailable", 503, true)
	}
	if err := h.authorize(r.Context(), p, ports.ActionRead, nil); err != nil {
		return err
	}
	reader, ok := h.s.Events.(BudgetedEventReader)
	if !ok {
		// A legacy read can consume a page before writeJSON rejects its size.
		// Refuse it before opening or mutating any cursor.
		return appError("service_unavailable", "Service unavailable", 503, true)
	}
	var page ports.EventPage
	var err error
	maxBytes := budget(r, h.limits.MaxResponseBytes)
	if id := r.URL.Query().Get("cursor"); id != "" {
		cursor := ports.Cursor{ID: id, PrincipalHash: events.PrincipalHash(p), WorkspaceID: p.WorkspaceID}
		page, err = reader.ReadEventPageForPrincipal(r.Context(), p, cursor, maxBytes)
	} else {
		opener, ok := h.s.Events.(AtomicEventPageOpener)
		if !ok {
			return appError("service_unavailable", "Service unavailable", 503, true)
		}
		page, err = opener.OpenEventPageForPrincipal(r.Context(), p, maxBytes)
	}
	switch {
	case page.RetentionGap, errors.Is(err, events.ErrCursorExpired), errors.Is(err, events.ErrCursorBinding):
		// An expired, purged, unknown, or foreign cursor all require the
		// client to resynchronize; a foreign cursor is not disclosed as such.
		return appError("cursor_expired", "Cursor expired; resync required", 409, false)
	case err != nil:
		var budgetError interface{ BudgetExceeded() bool }
		if errors.As(err, &budgetError) && budgetError.BudgetExceeded() {
			return ErrBudgetExceeded
		}
		return appError("service_unavailable", "Service unavailable", 503, true)
	}
	return h.writeJSON(w, map[string]any{"events": page.Events, "next_cursor": page.Next.ID}, budget(r, h.limits.MaxResponseBytes))
}

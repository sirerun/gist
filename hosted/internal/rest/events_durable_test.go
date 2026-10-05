package rest

import (
	"context"
	"errors"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
	"time"

	"github.com/sirerun/gist/hosted/internal/events"
	"github.com/sirerun/gist/hosted/internal/ports"
)

type durableEventProbe struct {
	err                  error
	opened, read, atomic bool
	principal            ports.Principal
	cursor               ports.Cursor
	limit                int
	contextValue         any
}

type eventContextKey struct{}
type eventBudgetFailure struct{}

func (eventBudgetFailure) Error() string        { return "private store detail" }
func (eventBudgetFailure) BudgetExceeded() bool { return true }

func (*durableEventProbe) Append(context.Context, ports.Event) error { return nil }
func (*durableEventProbe) Read(context.Context, ports.Cursor) (ports.EventPage, error) {
	return ports.EventPage{}, errors.New("legacy read must not be selected")
}
func (s *durableEventProbe) NewEventCursor(ctx context.Context, p ports.Principal, _ time.Duration) (ports.Cursor, error) {
	s.opened = true
	s.contextValue = ctx.Value(eventContextKey{})
	return ports.Cursor{ID: "durable", WorkspaceID: p.WorkspaceID, PrincipalHash: events.PrincipalHash(p)}, nil
}
func (s *durableEventProbe) OpenEventPageForPrincipal(ctx context.Context, p ports.Principal, limit int) (ports.EventPage, error) {
	s.atomic = true
	s.contextValue = ctx.Value(eventContextKey{})
	s.principal = p
	s.limit = limit
	if s.err != nil {
		return ports.EventPage{}, s.err
	}
	s.opened = true
	s.cursor = ports.Cursor{ID: "durable", WorkspaceID: p.WorkspaceID, PrincipalHash: events.PrincipalHash(p)}
	return ports.EventPage{Events: []ports.Event{}, Next: s.cursor}, nil
}
func (s *durableEventProbe) ReadEventPageForPrincipal(_ context.Context, p ports.Principal, c ports.Cursor, limit int) (ports.EventPage, error) {
	s.read = true
	s.principal = p
	s.cursor = c
	s.limit = limit
	return ports.EventPage{Events: []ports.Event{}, Next: c}, s.err
}

func TestDurableEventsUseContextPrincipalAndEffectiveBudget(t *testing.T) {
	for _, tc := range []struct {
		name, query string
		want        int
	}{
		{"lower client budget", "?max_bytes=128", 128},
		{"server limit", "?max_bytes=99999", 4096},
		{"default", "", 4096},
	} {
		t.Run(tc.name, func(t *testing.T) {
			h := testHandler(t)
			store := &durableEventProbe{}
			h.s.Events = store
			r := httptest.NewRequest(http.MethodGet, "/v1/events"+tc.query, nil)
			r = r.WithContext(context.WithValue(r.Context(), eventContextKey{}, "request-context"))
			r.Header.Set("Authorization", "Bearer test")
			w := httptest.NewRecorder()
			h.ServeHTTP(w, r)
			if w.Code != 200 {
				t.Fatalf("status %d: %s", w.Code, w.Body.String())
			}
			if !store.opened || !store.atomic || store.read || store.contextValue != "request-context" || store.limit != tc.want {
				t.Fatalf("durable context/budget not used: %+v", store)
			}
			if store.principal.WorkspaceID != "workspace-a" || store.cursor.PrincipalHash != events.PrincipalHash(store.principal) {
				t.Fatal("cursor lost authenticated binding")
			}
		})
	}
}

func TestDurableEventDenialsRetainPublicErrorContract(t *testing.T) {
	for _, tc := range []struct {
		name   string
		err    error
		status int
		code   string
	}{
		{"budget", eventBudgetFailure{}, 413, "budget_exceeded"},
		{"foreign", events.ErrCursorBinding, 409, "cursor_expired"},
		{"expired", events.ErrCursorExpired, 409, "cursor_expired"},
		{"unavailable", errors.New("private store detail"), 503, "service_unavailable"},
	} {
		t.Run(tc.name, func(t *testing.T) {
			h := testHandler(t)
			store := &durableEventProbe{err: tc.err}
			h.s.Events = store
			w := do(t, h, http.MethodGet, "/v1/events?cursor=existing&max_bytes=100", "")
			if w.Code != tc.status || !strings.Contains(w.Body.String(), `"code":"`+tc.code+`"`) {
				t.Fatalf("status %d body %s", w.Code, w.Body.String())
			}
			if store.opened || !store.read || store.cursor.ID != "existing" {
				t.Fatal("resume unexpectedly opened a cursor")
			}
			if strings.Contains(w.Body.String(), "private store detail") {
				t.Fatal("internal error detail exposed")
			}
		})
	}
}

func TestDurableFirstPageBudgetDoesNotUseSeparateOpen(t *testing.T) {
	h := testHandler(t)
	store := &durableEventProbe{err: eventBudgetFailure{}}
	h.s.Events = store
	w := do(t, h, http.MethodGet, "/v1/events?max_bytes=1", "")
	if w.Code != http.StatusRequestEntityTooLarge {
		t.Fatalf("status %d: %s", w.Code, w.Body.String())
	}
	if !store.atomic || store.opened || store.read {
		t.Fatalf("budget failure used non-atomic cursor opening: %+v", store)
	}
}

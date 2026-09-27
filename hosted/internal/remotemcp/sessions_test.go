package remotemcp

import (
	"net/http/httptest"
	"testing"
	"time"

	"github.com/sirerun/gist/hosted/internal/rest"
)

func sessionHandler(t *testing.T, ttl time.Duration, max int) (*Handler, *time.Time) {
	t.Helper()
	h, err := New(Config{
		Services:       rest.Services{Identity: identity{}, Authorizer: policy{}},
		AllowedOrigins: map[string]bool{"https://client.example": true},
		SessionTTL:     ttl,
		MaxSessions:    max,
	})
	if err != nil {
		t.Fatal(err)
	}
	now := time.Unix(1_700_000_000, 0)
	h.now = func() time.Time { return now }
	return h, &now
}

func hasSession(h *Handler, id string) bool {
	r := httptest.NewRequest("POST", "/mcp", nil)
	r.Header.Set("Mcp-Session-Id", id)
	return h.sessionOK(r)
}

func addSession(h *Handler, id string) {
	h.mu.Lock()
	h.addSessionLocked(id)
	h.mu.Unlock()
}

func TestSessionsExpireAfterIdleTTL(t *testing.T) {
	h, now := sessionHandler(t, time.Minute, 10)
	addSession(h, "s1")
	*now = now.Add(30 * time.Second)
	if !hasSession(h, "s1") {
		t.Fatal("session should be live inside the TTL")
	}
	*now = now.Add(59 * time.Second) // use above refreshed last-seen
	if !hasSession(h, "s1") {
		t.Fatal("use must refresh the idle TTL")
	}
	*now = now.Add(61 * time.Second)
	if hasSession(h, "s1") {
		t.Fatal("idle session must expire")
	}
	if len(h.sessions) != 0 {
		t.Fatalf("expired session not removed: %d left", len(h.sessions))
	}
}

func TestSessionsAreCappedWithLRUEviction(t *testing.T) {
	h, now := sessionHandler(t, time.Hour, 2)
	addSession(h, "s1")
	*now = now.Add(time.Second)
	addSession(h, "s2")
	*now = now.Add(time.Second)
	if !hasSession(h, "s1") { // s1 becomes most recently used
		t.Fatal("s1 should be live")
	}
	*now = now.Add(time.Second)
	addSession(h, "s3")
	if hasSession(h, "s2") {
		t.Fatal("least recently used session must be evicted at capacity")
	}
	if !hasSession(h, "s1") || !hasSession(h, "s3") {
		t.Fatal("recent sessions must survive eviction")
	}
	if len(h.sessions) != 2 {
		t.Fatalf("sessions = %d, want cap 2", len(h.sessions))
	}
}

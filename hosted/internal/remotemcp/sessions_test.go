package remotemcp

import (
	"bytes"
	"context"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"strconv"
	"sync"
	"testing"
	"time"

	"github.com/sirerun/gist/hosted/internal/ports"
	"github.com/sirerun/gist/hosted/internal/rest"
)

const testPrincipal = "p"

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
	return h.sessionOK(r, testPrincipal)
}

func addSession(h *Handler, id string) {
	h.mu.Lock()
	h.addSessionLocked(id, testPrincipal, "workspace-a")
	h.mu.Unlock()
}

func liveSessions(h *Handler) int {
	h.mu.Lock()
	defer h.mu.Unlock()
	return len(h.sessions)
}

// rpcAs sends one JSON-RPC request with the given bearer token and session.
func rpcAs(h http.Handler, token, sid, body string) *httptest.ResponseRecorder {
	r := httptest.NewRequest("POST", "/mcp", bytes.NewBufferString(body))
	r.Header.Set("Authorization", "Bearer "+token)
	r.Header.Set("Origin", "https://client.example")
	if sid != "" {
		r.Header.Set("Mcp-Session-Id", sid)
	}
	w := httptest.NewRecorder()
	h.ServeHTTP(w, r)
	return w
}

const initBody = `{"jsonrpc":"2.0","id":1,"method":"initialize","params":{"protocolVersion":"2025-06-18"}}`
const listBody = `{"jsonrpc":"2.0","id":2,"method":"tools/list","params":{}}`

func initialize(t *testing.T, h http.Handler, token string) string {
	t.Helper()
	w := rpcAs(h, token, "", initBody)
	sid := w.Header().Get("Mcp-Session-Id")
	if w.Code != 200 || sid == "" {
		t.Fatalf("initialize as %q: %d %s", token, w.Code, w.Body.String())
	}
	return sid
}

func listOK(h http.Handler, token, sid string) bool {
	w := rpcAs(h, token, sid, listBody)
	var got rpcResponse
	return w.Code == 200 && json.Unmarshal(w.Body.Bytes(), &got) == nil && got.Error == nil && got.Result != nil
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
	if n := liveSessions(h); n != 0 {
		t.Fatalf("expired session not removed: %d left", n)
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
	if n := liveSessions(h); n != 2 {
		t.Fatalf("sessions = %d, want cap 2", n)
	}
}

func TestExpiredSessionsAreSweptBeforeEvictingLiveOnes(t *testing.T) {
	h, now := sessionHandler(t, time.Minute, 2)
	addSession(h, "old")
	*now = now.Add(2 * time.Minute)
	addSession(h, "s1")
	addSession(h, "s2")
	if !hasSession(h, "s1") || !hasSession(h, "s2") {
		t.Fatal("the expired session, not a live one, must make room")
	}
}

// An initialize flood with tokens the identity store rejects must be refused
// before any session exists, so it cannot evict anyone.
func TestInitializeFloodWithInvalidTokensCreatesNoSessions(t *testing.T) {
	h, _ := sessionHandler(t, time.Hour, 4)
	victim := initialize(t, h, "victim")
	for i := 0; i < 1000; i++ {
		w := rpcAs(h, "bad-"+strconv.Itoa(i), "", initBody)
		if w.Code != http.StatusUnauthorized {
			t.Fatalf("invalid token initialize: status %d, want 401", w.Code)
		}
		if w.Header().Get("Mcp-Session-Id") != "" {
			t.Fatal("invalid token was issued a session")
		}
	}
	r := httptest.NewRequest("POST", "/mcp", bytes.NewBufferString(initBody))
	r.Header.Set("Origin", "https://client.example")
	w := httptest.NewRecorder()
	h.ServeHTTP(w, r)
	if w.Code != http.StatusUnauthorized {
		t.Fatalf("missing bearer: status %d, want 401", w.Code)
	}
	if n := liveSessions(h); n != 1 {
		t.Fatalf("sessions = %d after invalid-token flood, want only the victim's", n)
	}
	if !listOK(h, "victim", victim) {
		t.Fatal("victim session must survive an unauthenticated flood")
	}
}

// One authenticated principal is capped at its own share of the table and
// evicts only its own sessions, never another principal's.
func TestOnePrincipalFloodCannotEvictAnother(t *testing.T) {
	h, now := sessionHandler(t, time.Hour, 100)
	victim := initialize(t, h, "victim")
	var first string
	for i := 0; i < 1000; i++ {
		*now = now.Add(time.Millisecond)
		sid := initialize(t, h, "attacker")
		if i == 0 {
			first = sid
		}
	}
	h.mu.Lock()
	attacker := h.byPrincipal[principalKey(ports.Principal{Subject: "attacker", WorkspaceID: "workspace-a"})].Len()
	h.mu.Unlock()
	if attacker != defaultMaxSessionsPerPrincipal {
		t.Fatalf("attacker holds %d sessions, want per-principal cap %d", attacker, defaultMaxSessionsPerPrincipal)
	}
	if !listOK(h, "victim", victim) {
		t.Fatal("another principal's flood evicted the victim session")
	}
	if listOK(h, "attacker", first) {
		t.Fatal("the flooding principal's own oldest session should have been evicted")
	}
}

func TestSessionIsBoundToItsPrincipal(t *testing.T) {
	h, _ := sessionHandler(t, time.Hour, 10)
	sid := initialize(t, h, "owner")
	if listOK(h, "intruder", sid) {
		t.Fatal("a session must not be usable by a different principal")
	}
	if !listOK(h, "owner", sid) {
		t.Fatal("owner lost its session")
	}
}

type heldSearch struct {
	once    sync.Once
	started chan struct{}
	release chan struct{}
}

func (b *heldSearch) Search(ctx context.Context, _ ports.SearchQuery) (ports.SearchPage, error) {
	b.once.Do(func() { close(b.started) })
	select {
	case <-b.release:
	case <-ctx.Done():
		return ports.SearchPage{}, ctx.Err()
	}
	return ports.SearchPage{}, nil
}

// Eviction pressure while a tools/call is running must neither evict that
// session nor disturb the call.
func TestEvictionSkipsSessionWithInFlightCall(t *testing.T) {
	bs := &heldSearch{started: make(chan struct{}), release: make(chan struct{})}
	h, err := New(Config{
		Services:       rest.Services{Identity: identity{}, Authorizer: policy{}, Search: bs},
		AllowedOrigins: map[string]bool{"https://client.example": true},
		MaxSessions:    2,
	})
	if err != nil {
		t.Fatal(err)
	}
	now := time.Unix(1_700_000_000, 0)
	var clock sync.Mutex
	h.now = func() time.Time { clock.Lock(); defer clock.Unlock(); return now }
	tick := func() { clock.Lock(); now = now.Add(time.Second); clock.Unlock() }

	busy := initialize(t, h, "u")
	done := make(chan *httptest.ResponseRecorder, 1)
	go func() {
		done <- rpcAs(h, "u", busy, `{"jsonrpc":"2.0","id":3,"method":"tools/call","params":{"name":"gist_discover","arguments":{"query":"x","max_bytes":4096}}}`)
	}()
	select {
	case <-bs.started:
	case w := <-done:
		t.Fatalf("tools/call did not reach search: %s", w.Body.String())
	case <-time.After(5 * time.Second):
		t.Fatal("tools/call never started")
	}

	// busy is now the least recently used session; each new session must
	// evict an idle one instead.
	var idle []string
	for i := 0; i < 5; i++ {
		tick()
		idle = append(idle, initialize(t, h, "u"))
	}
	h.mu.Lock()
	_, busyLive := h.sessions[busy]
	h.mu.Unlock()
	if !busyLive {
		t.Fatal("session with an in-flight tools/call was evicted")
	}
	if listOK(h, "u", idle[0]) {
		t.Fatal("an idle session should have been evicted instead")
	}

	close(bs.release)
	var w *httptest.ResponseRecorder
	select {
	case w = <-done:
	case <-time.After(5 * time.Second):
		t.Fatal("in-flight call did not complete")
	}
	var got struct {
		Result struct {
			IsError bool `json:"isError"`
		} `json:"result"`
		Error any `json:"error"`
	}
	if err := json.Unmarshal(w.Body.Bytes(), &got); err != nil || got.Error != nil || got.Result.IsError {
		t.Fatalf("in-flight call result = %s", w.Body.String())
	}
}

// When every session is busy there is nothing safe to evict, so a new
// session is refused rather than breaking a running call.
func TestInitializeRefusedWhenOnlyBusySessionsRemain(t *testing.T) {
	h, _ := sessionHandler(t, time.Hour, 1)
	addSession(h, "busy")
	h.mu.Lock()
	h.calls["busy"] = 1
	h.mu.Unlock()
	w := rpcAs(h, "u", "", initBody)
	if w.Header().Get("Mcp-Session-Id") != "" || !bytes.Contains(w.Body.Bytes(), []byte("Too many sessions")) {
		t.Fatalf("initialize with only busy sessions: %s", w.Body.String())
	}
	if !hasSession(h, "busy") {
		t.Fatal("busy session was evicted")
	}
}

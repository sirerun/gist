package remotemcp

import (
	"context"
	"errors"
	"net/http"
	"strings"
	"sync"
	"sync/atomic"
	"testing"
	"time"

	"github.com/sirerun/gist/hosted/internal/ports"
	"github.com/sirerun/gist/hosted/internal/rest"
)

// slowIdentity parks every Lookup until release is closed and rejects every
// token, modelling an expensive verification of an invalid credential.
type slowIdentity struct {
	entered chan struct{}
	release chan struct{}
	calls   *atomic.Int64
}

func (s slowIdentity) Lookup(context.Context, string, string) (ports.IdentityRecord, error) {
	s.calls.Add(1)
	s.entered <- struct{}{}
	<-s.release
	return ports.IdentityRecord{}, errors.New("invalid token")
}
func (slowIdentity) Revoke(context.Context, string, string) error { return nil }

// An unauthenticated flood is capped before token verification runs: once
// MaxConcurrentRequests verifications are in flight, further requests are
// answered 429 without reaching the identity service.
func TestUnauthenticatedFloodIsCappedBeforeVerification(t *testing.T) {
	const slots = 2
	id := slowIdentity{entered: make(chan struct{}, 64), release: make(chan struct{}), calls: &atomic.Int64{}}
	h, err := New(Config{
		Services:              rest.Services{Identity: id, Authorizer: policy{}},
		AllowedOrigins:        map[string]bool{"https://client.example": true},
		MaxConcurrentRequests: slots,
	})
	if err != nil {
		t.Fatal(err)
	}
	var wg sync.WaitGroup
	for i := 0; i < slots; i++ {
		wg.Add(1)
		go func() {
			defer wg.Done()
			if w := rpcAs(h, "bad", "", initBody); w.Code != http.StatusUnauthorized {
				t.Errorf("held request: got %d, want 401", w.Code)
			}
		}()
	}
	for i := 0; i < slots; i++ {
		<-id.entered
	}
	for i := 0; i < 50; i++ {
		w := rpcAs(h, "bad", "", initBody)
		if w.Code != http.StatusTooManyRequests || w.Header().Get("Retry-After") == "" || !strings.Contains(w.Body.String(), "rate_limited") {
			t.Fatalf("flood request %d: got %d %q, want 429 rate_limited with Retry-After", i, w.Code, w.Body.String())
		}
	}
	if got := id.calls.Load(); got != slots {
		t.Fatalf("identity verified %d tokens during the flood, want %d", got, slots)
	}
	close(id.release)
	wg.Wait()
	// Slots are returned once the held requests finish.
	if w := rpcAs(h, "bad", "", initBody); w.Code != http.StatusUnauthorized {
		t.Fatalf("after release: got %d, want 401", w.Code)
	}
}

// The transport cap defaults to the REST in-flight cap when set there.
func TestConcurrentRequestCapDefaultsToRESTLimit(t *testing.T) {
	h, err := New(Config{Services: rest.Services{Identity: identity{}, Authorizer: policy{}, Limits: rest.Limits{RateLimit: 7}}})
	if err != nil {
		t.Fatal(err)
	}
	if cap(h.slots) != 7 {
		t.Fatalf("slots = %d, want 7", cap(h.slots))
	}
	h, err = New(Config{Services: rest.Services{Identity: identity{}, Authorizer: policy{}}})
	if err != nil {
		t.Fatal(err)
	}
	if cap(h.slots) != defaultMaxConcurrentRequests {
		t.Fatalf("slots = %d, want %d", cap(h.slots), defaultMaxConcurrentRequests)
	}
}

// admitCall pins the session in the same critical section that validates
// it: once admitted, neither TTL expiry nor cap eviction can remove the
// session before releaseCall, which closes the gap the old
// sessionOK-then-increment sequence left open.
func TestAdmittedCallPinsSessionAgainstExpiryAndEviction(t *testing.T) {
	h, now := sessionHandler(t, time.Minute, 1)
	addSession(h, "s1")
	admitted, limited := h.admitCall("s1", testPrincipal)
	if !admitted || limited {
		t.Fatalf("admitCall = %v, %v; want admitted and not limited", admitted, limited)
	}
	*now = now.Add(2 * time.Minute)
	h.mu.Lock()
	h.sweepExpiredLocked(h.now())
	evicted := h.evictIdleLocked(h.lru)
	added := h.addSessionLocked("s2", testPrincipal, "workspace-a")
	_, present := h.sessions["s1"]
	h.mu.Unlock()
	if evicted || added || !present {
		t.Fatalf("admitted session was removable: evicted=%v added=%v present=%v", evicted, added, present)
	}
	h.releaseCall("s1")
	h.mu.Lock()
	added = h.addSessionLocked("s2", testPrincipal, "workspace-a")
	_, present = h.sessions["s1"]
	h.mu.Unlock()
	if !added || present {
		t.Fatalf("after release: added=%v s1 present=%v; want s2 added and expired s1 gone", added, present)
	}
	if admitted, _ := h.admitCall("s1", testPrincipal); admitted {
		t.Fatal("admitCall admitted an evicted session")
	}
	h.mu.Lock()
	left := len(h.calls)
	h.mu.Unlock()
	if left != 0 {
		t.Fatalf("refused admission leaked %d call counters", left)
	}
}

// liveCheckSearch records whether the calling session was still live at the
// moment the tool ran.
type liveCheckSearch struct {
	h    **Handler
	sid  *atomic.Value
	dead *atomic.Int64
}

func (s liveCheckSearch) Search(context.Context, ports.SearchQuery) (ports.SearchPage, error) {
	h := *s.h
	h.mu.Lock()
	_, ok := h.sessions[s.sid.Load().(string)]
	h.mu.Unlock()
	if !ok {
		s.dead.Add(1)
	}
	return ports.SearchPage{}, nil
}

// Race-prone interleaving: tools/call on a session while the same principal
// keeps initializing at a per-principal cap of one, which evicts its idle
// sessions. A call that was admitted must never run on an evicted session.
func TestCallNeverRunsOnEvictedSession(t *testing.T) {
	var hp *Handler
	sid := &atomic.Value{}
	dead := &atomic.Int64{}
	h, err := New(Config{
		Services:                rest.Services{Identity: identity{}, Authorizer: policy{}, Search: liveCheckSearch{h: &hp, sid: sid, dead: dead}},
		AllowedOrigins:          map[string]bool{"https://client.example": true},
		MaxSessionsPerPrincipal: 1,
		MaxConcurrentRequests:   1000,
	})
	if err != nil {
		t.Fatal(err)
	}
	hp = h
	stop := make(chan struct{})
	var wg sync.WaitGroup
	wg.Add(1)
	go func() {
		defer wg.Done()
		for {
			select {
			case <-stop:
				return
			default:
				rpcAs(h, "caller", "", initBody)
			}
		}
	}()
	for i := 0; i < 500; i++ {
		w := rpcAs(h, "caller", "", initBody)
		s := w.Header().Get("Mcp-Session-Id")
		if s == "" {
			continue
		}
		sid.Store(s)
		rpcAs(h, "caller", s, discoverCall)
	}
	close(stop)
	wg.Wait()
	if n := dead.Load(); n != 0 {
		t.Fatalf("%d tools/call runs executed on an evicted session", n)
	}
}

// wsIdentity maps a "workspace/subject" token to that workspace and subject.
type wsIdentity struct{}

func (wsIdentity) Lookup(_ context.Context, token, _ string) (ports.IdentityRecord, error) {
	ws, sub, ok := strings.Cut(token, "/")
	if !ok {
		return ports.IdentityRecord{}, errors.New("invalid token")
	}
	return ports.IdentityRecord{WorkspaceID: ws, Subject: sub}, nil
}
func (wsIdentity) Revoke(context.Context, string, string) error { return nil }

// Many principals in one workspace flooding initialize are held to the
// per-workspace cap and evict their own workspace's sessions, never another
// workspace's idle session.
func TestOneWorkspaceFloodCannotEvictAnother(t *testing.T) {
	h, err := New(Config{
		Services:                rest.Services{Identity: wsIdentity{}, Authorizer: policy{}},
		AllowedOrigins:          map[string]bool{"https://client.example": true},
		MaxSessions:             20,
		MaxSessionsPerWorkspace: 10,
		MaxSessionsPerPrincipal: 2,
	})
	if err != nil {
		t.Fatal(err)
	}
	now := time.Unix(1_700_000_000, 0)
	h.now = func() time.Time { return now }
	victim := initialize(t, h, "tenant-b/victim")
	for i := 0; i < 500; i++ {
		now = now.Add(time.Millisecond)
		initialize(t, h, "tenant-a/p"+string(rune('a'+i%26))+string(rune('a'+i/26%26)))
	}
	h.mu.Lock()
	a := h.byWorkspace["tenant-a"].Len()
	total := len(h.sessions)
	h.mu.Unlock()
	if a != 10 {
		t.Fatalf("tenant-a holds %d sessions, want per-workspace cap 10", a)
	}
	if total > 20 {
		t.Fatalf("%d live sessions exceed the global cap", total)
	}
	if !listOK(h, "tenant-b/victim", victim) {
		t.Fatal("another workspace's flood evicted the victim's idle session")
	}
}

// The per-workspace cap has a sensible default and is clamped to the global
// cap, and the per-principal cap to the per-workspace cap.
func TestWorkspaceCapDefaultsAndClamps(t *testing.T) {
	h, err := New(Config{Services: rest.Services{Identity: identity{}, Authorizer: policy{}}})
	if err != nil {
		t.Fatal(err)
	}
	if h.cfg.MaxSessionsPerWorkspace != defaultMaxSessionsPerWorkspace {
		t.Fatalf("default per-workspace cap = %d, want %d", h.cfg.MaxSessionsPerWorkspace, defaultMaxSessionsPerWorkspace)
	}
	h, err = New(Config{Services: rest.Services{Identity: identity{}, Authorizer: policy{}}, MaxSessions: 50, MaxSessionsPerWorkspace: 100, MaxSessionsPerPrincipal: 40})
	if err != nil {
		t.Fatal(err)
	}
	if h.cfg.MaxSessionsPerWorkspace != 50 {
		t.Fatalf("per-workspace cap = %d, want clamp to 50", h.cfg.MaxSessionsPerWorkspace)
	}
	h, err = New(Config{Services: rest.Services{Identity: identity{}, Authorizer: policy{}}, MaxSessionsPerWorkspace: 5, MaxSessionsPerPrincipal: 40})
	if err != nil {
		t.Fatal(err)
	}
	if h.cfg.MaxSessionsPerPrincipal != 5 {
		t.Fatalf("per-principal cap = %d, want clamp to 5", h.cfg.MaxSessionsPerPrincipal)
	}
}

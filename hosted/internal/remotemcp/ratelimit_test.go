package remotemcp

import (
	"context"
	"strings"
	"sync"
	"testing"
	"time"

	"github.com/sirerun/gist/hosted/internal/ports"
	"github.com/sirerun/gist/hosted/internal/rest"
)

// blockingSearch parks every Search call until release is closed, so a test
// can hold tools/call requests in flight.
type blockingSearch struct {
	entered chan struct{}
	release chan struct{}
}

func (s blockingSearch) Search(context.Context, ports.SearchQuery) (ports.SearchPage, error) {
	s.entered <- struct{}{}
	<-s.release
	return ports.SearchPage{}, nil
}

const discoverCall = `{"jsonrpc":"2.0","id":3,"method":"tools/call","params":{"name":"gist_discover","arguments":{"query":"q","max_bytes":4096}}}`

func rateLimitedSession(t *testing.T, svc ports.LexicalSearcher, maxCalls int) (*Handler, string) {
	t.Helper()
	h, err := New(Config{
		Services:       rest.Services{Identity: identity{}, Authorizer: policy{}, Search: svc},
		AllowedOrigins: map[string]bool{"https://client.example": true},
		MaxCalls:       maxCalls,
	})
	if err != nil {
		t.Fatal(err)
	}
	w := mcpRequest(t, h, `{"jsonrpc":"2.0","id":1,"method":"initialize","params":{"protocolVersion":"2025-06-18"}}`, nil)
	sid := w.Header().Get("Mcp-Session-Id")
	if sid == "" {
		t.Fatalf("initialize returned no session id: %d %s", w.Code, w.Body.String())
	}
	return h, sid
}

// MaxCalls bounds concurrent in-flight calls, not a session's lifetime volume.
func TestSequentialCallsBeyondLimitSucceed(t *testing.T) {
	const limit = 3
	h, sid := rateLimitedSession(t, search{}, limit)
	for i := 0; i < limit*10; i++ {
		w := mcpRequest(t, h, discoverCall, map[string]string{"Mcp-Session-Id": sid})
		if strings.Contains(w.Body.String(), "rate_limited") {
			t.Fatalf("sequential call %d was rate limited: %s", i+1, w.Body.String())
		}
	}
	h.mu.Lock()
	left := len(h.calls)
	h.mu.Unlock()
	if left != 0 {
		t.Fatalf("in-flight counter not released: %d sessions still tracked", left)
	}
}

func TestConcurrentCallsOverLimitAreRateLimited(t *testing.T) {
	const limit = 2
	svc := blockingSearch{entered: make(chan struct{}, limit), release: make(chan struct{})}
	h, sid := rateLimitedSession(t, svc, limit)
	hdr := map[string]string{"Mcp-Session-Id": sid}

	var wg sync.WaitGroup
	bodies := make([]string, limit)
	for i := 0; i < limit; i++ {
		wg.Add(1)
		go func(i int) {
			defer wg.Done()
			bodies[i] = mcpRequest(t, h, discoverCall, hdr).Body.String()
		}(i)
	}
	for i := 0; i < limit; i++ {
		select {
		case <-svc.entered:
		case <-time.After(5 * time.Second):
			close(svc.release)
			t.Fatalf("only %d of %d calls reached the search service", i, limit)
		}
	}

	over := mcpRequest(t, h, discoverCall, hdr).Body.String()
	if !strings.Contains(over, "rate_limited") {
		t.Errorf("call over the concurrency limit was not rate limited: %s", over)
	}

	close(svc.release)
	wg.Wait()
	for i, b := range bodies {
		if strings.Contains(b, "rate_limited") {
			t.Errorf("in-limit call %d was rate limited: %s", i, b)
		}
	}

	// Once the in-flight calls finish, capacity is available again.
	after := mcpRequest(t, h, discoverCall, hdr).Body.String()
	if strings.Contains(after, "rate_limited") {
		t.Errorf("call after in-flight calls drained was rate limited: %s", after)
	}
}

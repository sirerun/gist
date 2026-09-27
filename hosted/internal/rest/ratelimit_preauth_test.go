package rest

import (
	"bytes"
	"context"
	"net/http"
	"net/http/httptest"
	"sync/atomic"
	"testing"

	"github.com/sirerun/gist/hosted/internal/ports"
)

// countingIdentity counts identity lookups so a test can prove a request was
// turned away before authentication ran.
type countingIdentity struct {
	testIdentity
	lookups *atomic.Int64
}

func (c countingIdentity) Lookup(ctx context.Context, token, audience string) (ports.IdentityRecord, error) {
	c.lookups.Add(1)
	return c.testIdentity.Lookup(ctx, token, audience)
}

// The in-flight cap is taken before authentication, so unauthenticated
// requests are bounded by it too and cannot reach the identity store once
// the cap is full.
func TestRateLimitBoundsUnauthenticatedRequests(t *testing.T) {
	search := parkedSearch{entered: make(chan struct{}, 1), release: make(chan struct{})}
	var lookups atomic.Int64
	h, err := New(Services{Identity: countingIdentity{lookups: &lookups}, Authorizer: testPolicy{}, Catalog: testCatalog{}, Search: search, Limits: Limits{RateLimit: 1, RetryAfter: 7}})
	if err != nil {
		t.Fatal(err)
	}
	const body = `{"query":"x","max_bytes":4096}`
	done := make(chan int, 1)
	go func() { done <- do(t, h, "POST", "/v1/discover", body).Code }()
	<-search.entered
	before := lookups.Load()

	for _, auth := range []string{"", "Bearer forged"} {
		r := httptest.NewRequest("POST", "/v1/discover", bytes.NewBufferString(body))
		if auth != "" {
			r.Header.Set("Authorization", auth)
		}
		w := httptest.NewRecorder()
		h.ServeHTTP(w, r)
		if w.Code != http.StatusTooManyRequests {
			t.Fatalf("request with auth %q at cap: status %d, want 429: %s", auth, w.Code, w.Body.String())
		}
	}
	if got := lookups.Load(); got != before {
		t.Fatalf("identity store consulted %d times while the cap was full", got-before)
	}

	close(search.release)
	if code := <-done; code != http.StatusOK {
		t.Fatalf("first request status = %d, want 200", code)
	}
	r := httptest.NewRequest("POST", "/v1/discover", bytes.NewBufferString(body))
	w := httptest.NewRecorder()
	h.ServeHTTP(w, r)
	if w.Code != http.StatusUnauthorized {
		t.Fatalf("unauthenticated request with a free slot: status %d, want 401", w.Code)
	}
}

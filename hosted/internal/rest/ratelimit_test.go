package rest

import (
	"context"
	"net/http"
	"testing"

	"github.com/sirerun/gist/hosted/internal/ports"
)

// parkedSearch holds every Search call until release is closed so a test can
// keep a request in flight.
type parkedSearch struct {
	entered chan struct{}
	release chan struct{}
}

func (s parkedSearch) Search(ctx context.Context, q ports.SearchQuery) (ports.SearchPage, error) {
	s.entered <- struct{}{}
	<-s.release
	return testSearch{}.Search(ctx, q)
}

func TestRateLimitCapsInFlightRequests(t *testing.T) {
	search := parkedSearch{entered: make(chan struct{}, 1), release: make(chan struct{})}
	h, err := New(Services{Identity: testIdentity{}, Authorizer: testPolicy{}, Catalog: testCatalog{}, Search: search, Limits: Limits{RateLimit: 1, RetryAfter: 7}})
	if err != nil {
		t.Fatal(err)
	}
	const body = `{"query":"x","max_bytes":4096}`
	done := make(chan int, 1)
	go func() { done <- do(t, h, "POST", "/v1/discover", body).Code }()
	<-search.entered

	w := do(t, h, "POST", "/v1/discover", body)
	if w.Code != http.StatusTooManyRequests {
		t.Fatalf("second in-flight request status = %d, want 429: %s", w.Code, w.Body.String())
	}
	if got := w.Header().Get("Retry-After"); got != "7" {
		t.Fatalf("Retry-After = %q, want 7", got)
	}

	close(search.release)
	if code := <-done; code != http.StatusOK {
		t.Fatalf("first request status = %d, want 200", code)
	}
	if w := do(t, h, "POST", "/v1/discover", body); w.Code != http.StatusOK {
		t.Fatalf("slot not released after completion: %d", w.Code)
	}
}

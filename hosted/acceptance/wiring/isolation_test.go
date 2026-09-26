//go:build integration

package wiring

import (
	"io"
	"sync"
	"testing"
	"time"
)

// The oracle uses equal discover requests and compares only observable
// authorization/results. A 250ms or 2x timing delta is diagnostic tolerance,
// not a correctness oracle; no sleeps are used and both requests begin from
// the same barrier.
func TestTwoTenantIsolationOracle(t *testing.T) {
	f := requireFixture(t)
	a := f.token(t, "q3-tenant-a", f.workA, "catalog:read")
	b := f.token(t, "q3-tenant-b", f.workB, "catalog:read")
	start := make(chan struct{})
	type result struct {
		status  int
		body    []byte
		elapsed time.Duration
	}
	out := make(chan result, 2)
	for _, token := range []string{a, b} {
		go func(token string) {
			<-start
			begin := time.Now()
			resp := f.do(t, "POST", "/v1/discover", token, `{"query":"fixture","max_bytes":4096}`)
			body, _ := io.ReadAll(resp.Body)
			_ = resp.Body.Close()
			out <- result{resp.StatusCode, body, time.Since(begin)}
		}(token)
	}
	close(start)
	left, right := <-out, <-out
	if left.status != 200 || right.status != 200 {
		t.Fatalf("tenant requests status=%d,%d", left.status, right.status)
	}
	if string(left.body) == string(right.body) {
		t.Fatalf("tenant responses are identical; cross-tenant catalog data may have leaked: %s", left.body)
	}
	if delta := left.elapsed - right.elapsed; delta > 250*time.Millisecond || delta < -250*time.Millisecond {
		t.Logf("timing diagnostic exceeded 250ms: a=%s b=%s", left.elapsed, right.elapsed)
	}
	// A principal from tenant B must not turn tenant A's known artifact into
	// a cache hit or a disclosed body.
	resp := f.do(t, "GET", "/v1/skills/q3-fixture-skill/versions/1.0.0", b, "")
	defer resp.Body.Close()
	if resp.StatusCode != 404 {
		t.Fatalf("cross-tenant artifact status=%d want 404", resp.StatusCode)
	}
	var wg sync.WaitGroup
	wg.Add(1)
	go func() { defer wg.Done(); _ = f.do(t, "GET", "/readyz", "", "") }()
	wg.Wait()
}

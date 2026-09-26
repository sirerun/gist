package identity

import (
	"fmt"
	"sync"
	"time"
)

// RevocationLease is a bounded local replay/revocation cache. It never grants
// authority; expiry requires the online policy check to run again.
type RevocationLease struct {
	mu      sync.RWMutex
	revoked map[string]time.Time
	ttl     time.Duration
}

func NewRevocationLease(ttl time.Duration) (*RevocationLease, error) {
	if ttl <= 0 || ttl > defaultMaxTokenAge {
		return nil, fmt.Errorf("revocation lease must be positive and at most five minutes")
	}
	return &RevocationLease{revoked: make(map[string]time.Time), ttl: ttl}, nil
}
func (r *RevocationLease) Revoke(jti string, now time.Time) {
	r.mu.Lock()
	defer r.mu.Unlock()
	r.revoked[jti] = now.Add(r.ttl)
}
func (r *RevocationLease) IsRevoked(jti string, now time.Time) bool {
	r.mu.RLock()
	until, ok := r.revoked[jti]
	r.mu.RUnlock()
	return ok && now.Before(until)
}

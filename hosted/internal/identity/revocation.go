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

// Revoke records jti as revoked. The entry is retained until the later of the
// lease TTL and the token's own expiry, so a revoked token can never become
// valid again while it is still unexpired.
func (r *RevocationLease) Revoke(jti string, expiresAt, now time.Time) {
	until := now.Add(r.ttl)
	if expiresAt.After(until) {
		until = expiresAt
	}
	r.mu.Lock()
	defer r.mu.Unlock()
	if prev, ok := r.revoked[jti]; ok && prev.After(until) {
		until = prev
	}
	r.revoked[jti] = until
}
func (r *RevocationLease) IsRevoked(jti string, now time.Time) bool {
	r.mu.RLock()
	until, ok := r.revoked[jti]
	r.mu.RUnlock()
	return ok && now.Before(until)
}

package resolution

import (
	"crypto/rand"
	"encoding/base64"
	"fmt"
	"time"
)

const maxLease = 60 * time.Second

func newID() (string, error) {
	b := make([]byte, 18)
	if _, err := rand.Read(b); err != nil {
		return "", fmt.Errorf("generate opaque resolution id: %w", err)
	}
	return base64.RawURLEncoding.EncodeToString(b), nil
}

func leaseExpiry(now time.Time, requested time.Duration) time.Time {
	if requested <= 0 || requested > maxLease {
		requested = maxLease
	}
	return now.Add(requested)
}

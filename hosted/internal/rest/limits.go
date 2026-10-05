package rest

import (
	"crypto/rand"
	"encoding/hex"
	"net/http"
	"strconv"
)

type Limits struct {
	MaxBodyBytes     int64
	MaxResponseBytes int
	MaxResults       int
	RateLimit        int
	RetryAfter       int
}

func (l Limits) withDefaults() Limits {
	if l.MaxBodyBytes <= 0 {
		l.MaxBodyBytes = 10 << 20
	}
	if l.MaxResponseBytes <= 0 {
		l.MaxResponseBytes = 100 << 20
	}
	if l.MaxResults <= 0 {
		l.MaxResults = 100
	}
	if l.RateLimit <= 0 {
		l.RateLimit = 100
	}
	if l.RetryAfter <= 0 {
		l.RetryAfter = 1
	}
	return l
}
func requestID(r *http.Request) (string, error) {
	id := r.Header.Get("X-Request-ID")
	if id != "" {
		if len(id) > 128 {
			return "", appError("validation_failed", "Invalid request", 422, false)
		}
		for _, c := range id {
			if c < 0x21 || c > 0x7e {
				return "", appError("validation_failed", "Invalid request", 422, false)
			}
		}
		return id, nil
	}
	b := make([]byte, 12)
	if _, err := rand.Read(b); err != nil {
		return "", appError("service_unavailable", "Service unavailable", 503, true)
	}
	return "req_" + hex.EncodeToString(b), nil
}
func budget(r *http.Request, fallback int) int {
	if n, err := strconv.Atoi(r.URL.Query().Get("max_bytes")); err == nil && n > 0 && n < fallback {
		return n
	}
	return fallback
}

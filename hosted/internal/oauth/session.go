package oauth

import (
	"crypto/hmac"
	"crypto/rand"
	"crypto/sha256"
	"encoding/base64"
	"encoding/json"
	"errors"
	"fmt"
	"net/http"
	"strings"
	"time"

	"github.com/sirerun/gist/hosted/internal/identity"
)

// ErrNoSession reports a request without a valid authenticated session.
var ErrNoSession = errors.New("oauth: no authenticated session")

// Session is an authenticated human session. The authorization server only
// attaches to it; it never authenticates passwords itself. Workspaces is the
// set the identity provider says this person may select from. Selecting one
// still requires a live membership, checked against the policy store.
type Session struct {
	ID         string
	Subject    string
	Workspaces []string
	ExpiresAt  time.Time
}

// SessionAuthenticator resolves the authenticated session on a request. The
// deployment configures it; CookieSessions is the reference implementation.
type SessionAuthenticator interface {
	Authenticate(r *http.Request) (Session, error)
}

// SessionCookieName is the reference session cookie.
const SessionCookieName = "__Host-gist_oauth_session"

const maxSessionTTL = 12 * time.Hour

// CookieSessions issues and verifies HMAC-signed session cookies. The login
// flow that establishes who the person is calls Issue; the authorization
// server calls Authenticate.
type CookieSessions struct {
	secret []byte
	clock  identity.Clock
}

type sessionClaims struct {
	ID         string   `json:"sid"`
	Subject    string   `json:"sub"`
	Workspaces []string `json:"ws"`
	Expires    int64    `json:"exp"`
}

// NewCookieSessions returns a session codec keyed by secret (32+ bytes).
func NewCookieSessions(secret []byte, clock identity.Clock) (*CookieSessions, error) {
	if len(secret) < 32 {
		return nil, errors.New("oauth: session secret must be at least 32 bytes")
	}
	if clock == nil {
		return nil, errors.New("oauth: session clock is required")
	}
	return &CookieSessions{secret: append([]byte(nil), secret...), clock: clock}, nil
}

// Issue returns a Secure, HttpOnly, SameSite=Lax session cookie for an
// already-authenticated person.
func (c *CookieSessions) Issue(subject string, workspaces []string, ttl time.Duration) (*http.Cookie, error) {
	if subject == "" || len(workspaces) == 0 {
		return nil, errors.New("oauth: session subject and workspaces are required")
	}
	for _, ws := range workspaces {
		if ws == "" {
			return nil, errors.New("oauth: empty session workspace")
		}
	}
	if ttl <= 0 || ttl > maxSessionTTL {
		ttl = maxSessionTTL
	}
	id := make([]byte, 16)
	if _, err := rand.Read(id); err != nil {
		return nil, fmt.Errorf("oauth: session id: %w", err)
	}
	expires := c.clock.Now().UTC().Add(ttl).Truncate(time.Second)
	raw, err := json.Marshal(sessionClaims{ID: base64.RawURLEncoding.EncodeToString(id), Subject: subject, Workspaces: append([]string(nil), workspaces...), Expires: expires.Unix()})
	if err != nil {
		return nil, err
	}
	payload := base64.RawURLEncoding.EncodeToString(raw)
	value := payload + "." + base64.RawURLEncoding.EncodeToString(c.sign(payload))
	return &http.Cookie{Name: SessionCookieName, Value: value, Path: "/", Expires: expires, Secure: true, HttpOnly: true, SameSite: http.SameSiteLaxMode}, nil
}

// Authenticate verifies the session cookie on r.
func (c *CookieSessions) Authenticate(r *http.Request) (Session, error) {
	cookie, err := r.Cookie(SessionCookieName)
	if err != nil {
		return Session{}, ErrNoSession
	}
	payload, sig, ok := strings.Cut(cookie.Value, ".")
	if !ok {
		return Session{}, ErrNoSession
	}
	got, err := base64.RawURLEncoding.DecodeString(sig)
	if err != nil || !hmac.Equal(got, c.sign(payload)) {
		return Session{}, ErrNoSession
	}
	raw, err := base64.RawURLEncoding.DecodeString(payload)
	if err != nil {
		return Session{}, ErrNoSession
	}
	var claims sessionClaims
	if err := json.Unmarshal(raw, &claims); err != nil || claims.ID == "" || claims.Subject == "" || len(claims.Workspaces) == 0 {
		return Session{}, ErrNoSession
	}
	expires := time.Unix(claims.Expires, 0).UTC()
	if !c.clock.Now().Before(expires) {
		return Session{}, ErrNoSession
	}
	return Session{ID: claims.ID, Subject: claims.Subject, Workspaces: claims.Workspaces, ExpiresAt: expires}, nil
}

func (c *CookieSessions) sign(payload string) []byte {
	m := hmac.New(sha256.New, c.secret)
	_, _ = m.Write([]byte("gist-oauth-session\x00"))
	_, _ = m.Write([]byte(payload))
	return m.Sum(nil)
}

// Package oauth is Gist's built-in reference OAuth 2.1 authorization server
// (RFC-002 section 10, strategy 1). It implements RFC 8414 metadata, RFC 7591
// dynamic client registration, the authorization code grant with S256 PKCE
// and explicit consent, rotating refresh tokens with family revocation on
// reuse, RFC 7009 revocation, RFC 9728 protected-resource metadata and RFC
// 8707 resource indicators bound into the access token audience.
//
// Access tokens are minted by the identity package's workload issuer against
// the shared identity key set, so the resource server verifies them on the
// same path as every other token. The server never stores a raw
// authorization code or refresh token: only SHA-256 hashes reach the Store.
package oauth

import (
	"context"
	"crypto/hmac"
	"crypto/rand"
	"crypto/sha256"
	"encoding/base64"
	"encoding/json"
	"errors"
	"fmt"
	"net/http"
	"net/url"
	"strings"
	"time"

	"github.com/sirerun/gist/hosted/internal/identity"
)

// Store errors. Implementations must return exactly these sentinels (wrapped
// is fine) so the server can map them to OAuth error codes.
var (
	ErrNotFound      = errors.New("oauth record not found")
	ErrGrantReused   = errors.New("oauth grant reused")
	ErrFamilyRevoked = errors.New("oauth refresh family revoked")
)

// Client is a dynamically registered public client.
type Client struct {
	ID           string
	Name         string
	RedirectURIs []string
	Scopes       []string
	Audience     string
	CreatedAt    time.Time
}

// AuthCode is the persisted form of an authorization code. Hash is the
// SHA-256 of the raw code; the raw value is never persisted.
type AuthCode struct {
	Hash        []byte
	ClientID    string
	Subject     string
	WorkspaceID string
	RedirectURI string
	Resource    string
	Scopes      []string
	Challenge   string
	IssuedAt    time.Time
	ExpiresAt   time.Time
}

// RefreshFamily is one consented grant. Every refresh token rotated from the
// same authorization code belongs to it, and it carries the granted bounds.
type RefreshFamily struct {
	ID          string
	ClientID    string
	Subject     string
	WorkspaceID string
	Resource    string
	Scopes      []string
	CodeHash    []byte
}

// RefreshToken is the persisted form of a refresh token (hash only).
type RefreshToken struct {
	Hash      []byte
	IssuedAt  time.Time
	ExpiresAt time.Time
}

// Store is the authorization server's persistence port. Codes, families and
// refresh tokens are workspace-scoped, and every call that touches them names
// the workspace so the implementation can run under tenant row-level security.
type Store interface {
	CreateClient(ctx context.Context, c Client) error
	GetClient(ctx context.Context, clientID string) (Client, error)
	SaveCode(ctx context.Context, code AuthCode) error
	// RedeemCode atomically consumes a code and, when check passes, opens
	// its refresh family with first as the family's first token, all in one
	// transaction. It returns the code and the new family ID. check runs on
	// the locked row; if it fails the code is still consumed (a code that
	// was presented wrongly can never be redeemed) and check's error is
	// returned. A code that was already consumed returns ErrGrantReused
	// after every refresh family issued from it has been revoked.
	RedeemCode(ctx context.Context, workspaceID string, hash []byte, check func(AuthCode) error, first RefreshToken) (AuthCode, string, error)
	// RotateRefresh atomically consumes one refresh token and stores next in
	// the same family. check runs on the family before anything changes. A
	// token that was already consumed revokes the whole family and returns
	// ErrGrantReused; a revoked family returns ErrFamilyRevoked. The returned
	// token is the consumed one, so the caller can check its expiry.
	RotateRefresh(ctx context.Context, workspaceID string, hash []byte, check func(RefreshFamily, RefreshToken) error, next RefreshToken) (RefreshFamily, error)
	// RevokeFamily revokes a family by ID.
	RevokeFamily(ctx context.Context, workspaceID, familyID string) error
	// RevokeRefresh revokes the family that contains the token when it
	// belongs to clientID. Unknown tokens return ErrNotFound.
	RevokeRefresh(ctx context.Context, workspaceID string, hash []byte, clientID string) error
}

// AccessToken is a minted bearer token.
type AccessToken struct {
	Token     string
	ExpiresAt time.Time
}

// AccessMinter mints a resource-bound access token and records the issued
// identity so the resource server's stored-identity check accepts it.
type AccessMinter interface {
	MintAccess(ctx context.Context, resource string, req identity.WorkloadRequest) (AccessToken, error)
}

// KeySource publishes the verification keys for the JWKS endpoint.
type KeySource interface {
	VerificationKeys() []identity.VerificationKey
}

// Config configures the reference authorization server.
type Config struct {
	// Issuer is the exact HTTPS origin of the authorization server. It is
	// also the issuer of the access tokens the minter produces.
	Issuer string
	// Resources are the accepted RFC 8707 resource indicators. Resources[0]
	// is the resource this deployment protects; its protected-resource
	// metadata is served at /.well-known/oauth-protected-resource.
	Resources       []string
	ScopesSupported []string
	Store           Store
	Minter          AccessMinter
	Policy          identity.PolicyStore
	Sessions        SessionAuthenticator
	Keys            KeySource
	Clock           identity.Clock
	// Secret keys the HMAC that protects consent requests and CSRF tokens.
	// It is required (at least 32 bytes) and must be the same on every
	// replica and across restarts, or a consent page rendered by one
	// process is rejected by the next.
	Secret []byte
	// LoginURL, when set, receives unauthenticated authorize requests with a
	// return_to parameter. Without it the server answers 401.
	LoginURL     string
	CodeTTL      time.Duration
	RefreshTTL   time.Duration
	ConsentTTL   time.Duration
	MaxBodyBytes int64
}

// Server is the reference authorization server's HTTP handler.
type Server struct {
	cfg     Config
	secret  []byte
	scopes  map[string]bool
	consent *consentPage
}

const (
	defaultCodeTTL    = 60 * time.Second
	defaultRefreshTTL = 30 * 24 * time.Hour
	defaultConsentTTL = 10 * time.Minute
	defaultMaxBody    = 16 << 10
)

// Endpoint paths. They are fixed so metadata and routing cannot drift.
const (
	PathASMetadata  = "/.well-known/oauth-authorization-server"
	PathPRMetadata  = "/.well-known/oauth-protected-resource"
	PathAuthorize   = "/oauth/authorize"
	PathConsent     = "/oauth/consent"
	PathToken       = "/oauth/token"
	PathRegister    = "/oauth/register"
	PathRevoke      = "/oauth/revoke"
	PathJWKS        = "/oauth/jwks"
	pathOAuthPrefix = "/oauth/"
)

// New validates cfg and returns a Server.
func New(cfg Config) (*Server, error) {
	if err := exactHTTPSOrigin(cfg.Issuer); err != nil {
		return nil, fmt.Errorf("oauth: issuer: %w", err)
	}
	if len(cfg.Resources) == 0 {
		return nil, errors.New("oauth: at least one resource is required")
	}
	for _, r := range cfg.Resources {
		if err := httpsURL(r); err != nil {
			return nil, fmt.Errorf("oauth: resource %q: %w", r, err)
		}
	}
	if len(cfg.ScopesSupported) == 0 {
		return nil, errors.New("oauth: supported scopes are required")
	}
	if cfg.Store == nil || cfg.Minter == nil || cfg.Policy == nil || cfg.Sessions == nil || cfg.Keys == nil || cfg.Clock == nil {
		return nil, errors.New("oauth: store, minter, policy, sessions, keys and clock are required")
	}
	if cfg.LoginURL != "" {
		if err := httpsURL(cfg.LoginURL); err != nil {
			return nil, fmt.Errorf("oauth: login URL: %w", err)
		}
	}
	if cfg.CodeTTL <= 0 || cfg.CodeTTL > 10*time.Minute {
		cfg.CodeTTL = defaultCodeTTL
	}
	if cfg.RefreshTTL <= 0 {
		cfg.RefreshTTL = defaultRefreshTTL
	}
	if cfg.ConsentTTL <= 0 {
		cfg.ConsentTTL = defaultConsentTTL
	}
	if cfg.MaxBodyBytes <= 0 {
		cfg.MaxBodyBytes = defaultMaxBody
	}
	secret := append([]byte(nil), cfg.Secret...)
	if len(secret) < 32 {
		return nil, errors.New("oauth: secret must be at least 32 bytes")
	}
	scopes := make(map[string]bool, len(cfg.ScopesSupported))
	for _, s := range cfg.ScopesSupported {
		if !validScopeToken(s) {
			return nil, fmt.Errorf("oauth: invalid supported scope %q", s)
		}
		scopes[s] = true
	}
	page, err := newConsentPage()
	if err != nil {
		return nil, err
	}
	return &Server{cfg: cfg, secret: secret, scopes: scopes, consent: page}, nil
}

// Handles reports whether path belongs to the authorization server.
func Handles(path string) bool {
	return strings.HasPrefix(path, pathOAuthPrefix) || path == PathASMetadata || path == PathPRMetadata
}

// ProtectedResourceMetadataURL is the RFC 9728 document URL for the
// deployment's protected resource.
func (s *Server) ProtectedResourceMetadataURL() string {
	return prmURL(s.cfg.Resources[0])
}

func (s *Server) ServeHTTP(w http.ResponseWriter, r *http.Request) {
	switch r.URL.Path {
	case PathASMetadata:
		s.onlyGet(w, r, s.handleASMetadata)
	case PathPRMetadata:
		s.onlyGet(w, r, s.handlePRMetadata)
	case PathJWKS:
		s.onlyGet(w, r, s.handleJWKS)
	case PathAuthorize:
		s.onlyGet(w, r, s.handleAuthorize)
	case PathConsent:
		s.onlyPost(w, r, s.handleConsent)
	case PathRegister:
		s.onlyPost(w, r, s.handleRegister)
	case PathToken:
		s.onlyPost(w, r, s.handleToken)
	case PathRevoke:
		s.onlyPost(w, r, s.handleRevoke)
	default:
		http.NotFound(w, r)
	}
}

func (s *Server) onlyGet(w http.ResponseWriter, r *http.Request, h http.HandlerFunc) {
	if r.Method != http.MethodGet && r.Method != http.MethodHead {
		w.Header().Set("Allow", "GET, HEAD")
		w.WriteHeader(http.StatusMethodNotAllowed)
		return
	}
	h(w, r)
}

func (s *Server) onlyPost(w http.ResponseWriter, r *http.Request, h http.HandlerFunc) {
	if r.Method != http.MethodPost {
		w.Header().Set("Allow", "POST")
		w.WriteHeader(http.StatusMethodNotAllowed)
		return
	}
	r.Body = http.MaxBytesReader(w, r.Body, s.cfg.MaxBodyBytes)
	h(w, r)
}

func (s *Server) now() time.Time { return s.cfg.Clock.Now().UTC() }

func (s *Server) resourceAllowed(resource string) bool {
	for _, r := range s.cfg.Resources {
		if r == resource {
			return true
		}
	}
	return false
}

// oauthError is an RFC 6749 section 5.2 error body.
type oauthError struct {
	Code        string `json:"error"`
	Description string `json:"error_description,omitempty"`
}

func writeJSON(w http.ResponseWriter, status int, v any) {
	w.Header().Set("Content-Type", "application/json")
	w.Header().Set("Cache-Control", "no-store")
	w.Header().Set("Pragma", "no-cache")
	w.WriteHeader(status)
	_ = json.NewEncoder(w).Encode(v)
}

func writeOAuthError(w http.ResponseWriter, status int, code, description string) {
	writeJSON(w, status, oauthError{Code: code, Description: description})
}

// hashSecret is the only form in which codes and refresh tokens are stored.
func hashSecret(raw string) []byte {
	sum := sha256.Sum256([]byte(raw))
	return sum[:]
}

// Opaque credentials carry their workspace so the store can open the right
// tenant scope before the hash lookup: "<prefix>.<b64 workspace>.<b64 random>".
// The workspace is not trust input; the stored row must match it.
func newOpaque(prefix, workspaceID string) (string, error) {
	b := make([]byte, 32)
	if _, err := rand.Read(b); err != nil {
		return "", fmt.Errorf("generate %s: %w", prefix, err)
	}
	return prefix + "." + base64.RawURLEncoding.EncodeToString([]byte(workspaceID)) + "." + base64.RawURLEncoding.EncodeToString(b), nil
}

func parseOpaque(prefix, raw string) (string, bool) {
	parts := strings.Split(raw, ".")
	if len(parts) != 3 || parts[0] != prefix || len(parts[2]) != 43 {
		return "", false
	}
	ws, err := base64.RawURLEncoding.DecodeString(parts[1])
	if err != nil || len(ws) == 0 || len(ws) > 256 {
		return "", false
	}
	return string(ws), true
}

func (s *Server) mac(parts ...string) []byte {
	m := hmac.New(sha256.New, s.secret)
	for _, p := range parts {
		_, _ = m.Write([]byte(p))
		_, _ = m.Write([]byte{0})
	}
	return m.Sum(nil)
}

func randomID(n int) (string, error) {
	b := make([]byte, n)
	if _, err := rand.Read(b); err != nil {
		return "", err
	}
	return base64.RawURLEncoding.EncodeToString(b), nil
}

func exactHTTPSOrigin(raw string) error {
	u, err := url.Parse(raw)
	if err != nil || u.Scheme != "https" || u.Host == "" || u.Path != "" || u.RawQuery != "" || u.Fragment != "" || u.User != nil {
		return errors.New("must be one exact HTTPS origin")
	}
	return nil
}

func httpsURL(raw string) error {
	u, err := url.Parse(raw)
	if err != nil || u.Scheme != "https" || u.Host == "" || u.Fragment != "" || u.User != nil || strings.ContainsAny(raw, " \t\r\n") {
		return errors.New("must be an absolute HTTPS URL without fragment or credentials")
	}
	return nil
}

// validScopeToken follows RFC 6749 section 3.3 scope-token syntax.
func validScopeToken(s string) bool {
	if s == "" || len(s) > 128 {
		return false
	}
	for _, c := range s {
		if c < 0x21 || c > 0x7e || c == '"' || c == '\\' {
			return false
		}
	}
	return true
}

// parseScope splits a space-delimited scope string. Duplicates are removed
// and order is preserved.
func parseScope(raw string) ([]string, bool) {
	var out []string
	seen := map[string]bool{}
	for _, s := range strings.Split(raw, " ") {
		if s == "" {
			continue
		}
		if !validScopeToken(s) {
			return nil, false
		}
		if !seen[s] {
			seen[s] = true
			out = append(out, s)
		}
	}
	return out, true
}

func containsAll(have, want []string) bool {
	set := make(map[string]bool, len(have))
	for _, s := range have {
		set[s] = true
	}
	for _, s := range want {
		if !set[s] {
			return false
		}
	}
	return true
}

func contains(list []string, v string) bool {
	for _, s := range list {
		if s == v {
			return true
		}
	}
	return false
}

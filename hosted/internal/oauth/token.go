package oauth

import (
	"crypto/sha256"
	"crypto/subtle"
	"encoding/base64"
	"errors"
	"net/http"
	"regexp"
	"strings"
	"time"

	"github.com/sirerun/gist/hosted/internal/identity"
)

const (
	codePrefix    = "gc1"
	refreshPrefix = "gr1"
)

// RFC 7636 section 4.1: 43-128 characters from the unreserved set.
var verifierPattern = regexp.MustCompile(`^[A-Za-z0-9._~-]{43,128}$`)

type tokenResponse struct {
	AccessToken  string `json:"access_token"`
	TokenType    string `json:"token_type"`
	ExpiresIn    int64  `json:"expires_in"`
	RefreshToken string `json:"refresh_token,omitempty"`
	Scope        string `json:"scope"`
}

// grantError is an RFC 6749 section 5.2 token-endpoint error.
type grantError struct {
	status      int
	code        string
	description string
}

func (e *grantError) Error() string { return e.code + ": " + e.description }

func invalidGrant(description string) *grantError {
	return &grantError{http.StatusBadRequest, "invalid_grant", description}
}

func (s *Server) handleToken(w http.ResponseWriter, r *http.Request) {
	form, client, gerr := s.tokenRequest(r)
	if gerr != nil {
		s.writeGrantError(w, gerr)
		return
	}
	var resp tokenResponse
	switch form.Get("grant_type") {
	case "authorization_code":
		resp, gerr = s.redeemCode(r, form, client)
	case "refresh_token":
		resp, gerr = s.refresh(r, form, client)
	case "":
		gerr = &grantError{http.StatusBadRequest, "invalid_request", "grant_type is required"}
	default:
		gerr = &grantError{http.StatusBadRequest, "unsupported_grant_type", "grant_type is not supported"}
	}
	if gerr != nil {
		s.writeGrantError(w, gerr)
		return
	}
	writeJSON(w, http.StatusOK, resp)
}

func (s *Server) writeGrantError(w http.ResponseWriter, e *grantError) {
	writeOAuthError(w, e.status, e.code, e.description)
}

// tokenRequest parses a form-encoded token or revocation request and
// identifies the public client by client_id.
func (s *Server) tokenRequest(r *http.Request) (formValues, Client, *grantError) {
	if ct := r.Header.Get("Content-Type"); !strings.HasPrefix(strings.ToLower(ct), "application/x-www-form-urlencoded") {
		return nil, Client{}, &grantError{http.StatusBadRequest, "invalid_request", "requests must be application/x-www-form-urlencoded"}
	}
	if err := r.ParseForm(); err != nil {
		return nil, Client{}, &grantError{http.StatusBadRequest, "invalid_request", "malformed form body"}
	}
	if len(r.URL.RawQuery) > 0 {
		return nil, Client{}, &grantError{http.StatusBadRequest, "invalid_request", "parameters must be sent in the request body"}
	}
	for key, values := range r.PostForm {
		if len(values) > 1 {
			return nil, Client{}, &grantError{http.StatusBadRequest, "invalid_request", "the request repeats " + key}
		}
	}
	if r.Header.Get("Authorization") != "" {
		return nil, Client{}, &grantError{http.StatusUnauthorized, "invalid_client", "only public clients (token_endpoint_auth_method=none) are supported"}
	}
	clientID := r.PostForm.Get("client_id")
	if clientID == "" {
		return nil, Client{}, &grantError{http.StatusUnauthorized, "invalid_client", "client_id is required"}
	}
	client, err := s.cfg.Store.GetClient(r.Context(), clientID)
	if err != nil {
		if errors.Is(err, ErrNotFound) {
			return nil, Client{}, &grantError{http.StatusUnauthorized, "invalid_client", "unknown client"}
		}
		return nil, Client{}, &grantError{http.StatusServiceUnavailable, "temporarily_unavailable", ""}
	}
	return formValues(r.PostForm), client, nil
}

type formValues map[string][]string

func (f formValues) Get(key string) string {
	if v := f[key]; len(v) > 0 {
		return v[0]
	}
	return ""
}

func (f formValues) Has(key string) bool { _, ok := f[key]; return ok }

// redeemCode exchanges an authorization code. The code must be unexpired,
// unconsumed, issued to this client for this exact redirect URI and
// resource, and the verifier must match its S256 challenge. Any failed check
// still consumes the code. The refresh family is created in the same store
// transaction that consumes the code, so a replay at any later point revokes
// it; a second redemption revokes every grant made from the code.
func (s *Server) redeemCode(r *http.Request, form formValues, client Client) (tokenResponse, *grantError) {
	raw, redirectURI, verifier := form.Get("code"), form.Get("redirect_uri"), form.Get("code_verifier")
	if raw == "" || redirectURI == "" || verifier == "" {
		return tokenResponse{}, &grantError{http.StatusBadRequest, "invalid_request", "code, redirect_uri and code_verifier are required"}
	}
	if !verifierPattern.MatchString(verifier) {
		return tokenResponse{}, invalidGrant("code_verifier is malformed")
	}
	resource := form.Get("resource")
	if form.Has("resource") && !s.resourceAllowed(resource) {
		return tokenResponse{}, &grantError{http.StatusBadRequest, "invalid_target", "unknown resource"}
	}
	workspace, ok := parseOpaque(codePrefix, raw)
	if !ok {
		return tokenResponse{}, invalidGrant("authorization code is invalid")
	}
	now := s.now()
	refresh, err := newOpaque(refreshPrefix, workspace)
	if err != nil {
		return tokenResponse{}, &grantError{http.StatusInternalServerError, "server_error", ""}
	}
	first := RefreshToken{Hash: hashSecret(refresh), IssuedAt: now, ExpiresAt: now.Add(s.cfg.RefreshTTL)}
	code, familyID, err := s.cfg.Store.RedeemCode(r.Context(), workspace, hashSecret(raw), func(c AuthCode) error {
		switch {
		case c.WorkspaceID != workspace:
			return invalidGrant("authorization code is invalid")
		case !now.Before(c.ExpiresAt):
			return invalidGrant("authorization code expired")
		case c.ClientID != client.ID:
			return invalidGrant("authorization code was issued to another client")
		case c.RedirectURI != redirectURI:
			return invalidGrant("redirect_uri does not match the authorization request")
		case !pkceMatches(verifier, c.Challenge):
			return invalidGrant("code_verifier does not match the code challenge")
		case form.Has("resource") && resource != c.Resource:
			return &grantError{http.StatusBadRequest, "invalid_target", "resource does not match the authorization request"}
		}
		return nil
	}, first)
	if err != nil {
		var ge *grantError
		switch {
		case errors.As(err, &ge):
			return tokenResponse{}, ge
		case errors.Is(err, ErrGrantReused):
			return tokenResponse{}, invalidGrant("authorization code was already used; grants issued from it are revoked")
		case errors.Is(err, ErrNotFound):
			return tokenResponse{}, invalidGrant("authorization code is invalid")
		default:
			return tokenResponse{}, &grantError{http.StatusServiceUnavailable, "temporarily_unavailable", ""}
		}
	}
	access, gerr := s.mint(r, code.Resource, code.Subject, code.WorkspaceID, code.Scopes)
	if gerr != nil {
		// The consent no longer holds: the family must not outlive it.
		_ = s.cfg.Store.RevokeFamily(r.Context(), code.WorkspaceID, familyID)
		return tokenResponse{}, gerr
	}
	return s.tokenResponse(access, refresh, code.Scopes), nil
}

// mint issues a resource-bound access token. The minter re-checks the
// current membership, so a consent that no longer holds yields no token.
func (s *Server) mint(r *http.Request, resource, subject, workspace string, scopes []string) (AccessToken, *grantError) {
	access, err := s.cfg.Minter.MintAccess(r.Context(), resource, identity.WorkloadRequest{Subject: subject, WorkspaceID: workspace, Scopes: scopes, SubjectType: "human"})
	if err != nil {
		return AccessToken{}, invalidGrant("the grant is no longer authorized")
	}
	return access, nil
}

func (s *Server) tokenResponse(access AccessToken, refresh string, scopes []string) tokenResponse {
	expiresIn := int64(access.ExpiresAt.Sub(s.now()).Round(time.Second) / time.Second)
	if expiresIn < 0 {
		expiresIn = 0
	}
	return tokenResponse{AccessToken: access.Token, TokenType: "Bearer", ExpiresIn: expiresIn, RefreshToken: refresh, Scope: strings.Join(scopes, " ")}
}

// pkceMatches implements the RFC 7636 S256 transformation in constant time.
func pkceMatches(verifier, challenge string) bool {
	sum := sha256.Sum256([]byte(verifier))
	got := base64.RawURLEncoding.EncodeToString(sum[:])
	return subtle.ConstantTimeCompare([]byte(got), []byte(challenge)) == 1
}

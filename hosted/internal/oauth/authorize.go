package oauth

import (
	"crypto/hmac"
	"encoding/base64"
	"encoding/json"
	"errors"
	"net/http"
	"net/url"
	"regexp"
	"strings"
)

// S256 challenges are base64url(SHA-256(verifier)) without padding: exactly
// 43 characters. Anything else cannot be an S256 challenge.
var challengePattern = regexp.MustCompile(`^[A-Za-z0-9_-]{43}$`)

const maxStateBytes = 1024

// grantRequest is a validated authorization request, sealed into the consent
// form so the consent POST cannot alter client, redirect, resource, scope or
// PKCE challenge. It is bound to the session that saw the consent page.
type grantRequest struct {
	ClientID    string   `json:"cid"`
	RedirectURI string   `json:"ru"`
	Resource    string   `json:"res"`
	Scopes      []string `json:"sc"`
	Challenge   string   `json:"cc"`
	State       string   `json:"st,omitempty"`
	Subject     string   `json:"sub"`
	SessionID   string   `json:"sid"`
	Expires     int64    `json:"exp"`
}

// authorizeError is an error that may be returned to the client's redirect
// URI because the client and redirect URI have been validated.
type authorizeError struct{ code, description string }

// handleAuthorize validates the authorization request, requires an
// authenticated session, and renders the consent page. Nothing is granted
// here: only the consent POST can issue a code.
func (s *Server) handleAuthorize(w http.ResponseWriter, r *http.Request) {
	q := r.URL.Query()
	for key, values := range q {
		if len(values) > 1 {
			s.errorPage(w, http.StatusBadRequest, "The request repeats the "+key+" parameter.")
			return
		}
	}
	client, err := s.cfg.Store.GetClient(r.Context(), q.Get("client_id"))
	if q.Get("client_id") == "" || err != nil {
		// Never redirect to an unvalidated URI (RFC 6749 section 4.1.2.1).
		s.errorPage(w, http.StatusBadRequest, "The client is not registered.")
		return
	}
	redirectURI := q.Get("redirect_uri")
	if redirectURI == "" || !redirectRegistered(client.RedirectURIs, redirectURI) {
		s.errorPage(w, http.StatusBadRequest, "The redirect URI is not registered for this client.")
		return
	}
	state := q.Get("state")
	if len(state) > maxStateBytes {
		s.errorPage(w, http.StatusBadRequest, "The state parameter is too long.")
		return
	}
	req, aerr := s.validateAuthorize(q, client)
	if aerr != nil {
		s.redirectError(w, r, redirectURI, state, aerr)
		return
	}
	sess, err := s.cfg.Sessions.Authenticate(r)
	if err != nil {
		if s.cfg.LoginURL != "" {
			login, _ := url.Parse(s.cfg.LoginURL)
			lq := login.Query()
			lq.Set("return_to", s.cfg.Issuer+r.URL.RequestURI())
			login.RawQuery = lq.Encode()
			http.Redirect(w, r, login.String(), http.StatusFound)
			return
		}
		s.errorPage(w, http.StatusUnauthorized, "Sign in to Gist, then retry the connection.")
		return
	}
	req.State = state
	req.Subject = sess.Subject
	req.SessionID = sess.ID
	req.Expires = s.now().Add(s.cfg.ConsentTTL).Unix()
	sealed, err := s.seal(req)
	if err != nil {
		s.errorPage(w, http.StatusInternalServerError, "The consent request could not be prepared.")
		return
	}
	s.renderConsent(w, consentView{
		ClientName:   client.Name,
		RedirectHost: hostOf(redirectURI),
		Resource:     req.Resource,
		Scopes:       req.Scopes,
		Workspaces:   sess.Workspaces,
		Request:      sealed,
		CSRF:         s.csrfToken(sess.ID, sealed),
		Action:       PathConsent,
	})
}

func (s *Server) validateAuthorize(q url.Values, client Client) (grantRequest, *authorizeError) {
	if q.Get("response_type") != "code" {
		return grantRequest{}, &authorizeError{"unsupported_response_type", "only response_type=code is supported"}
	}
	if q.Get("code_challenge_method") != "S256" {
		return grantRequest{}, &authorizeError{"invalid_request", "PKCE with code_challenge_method=S256 is required"}
	}
	challenge := q.Get("code_challenge")
	if !challengePattern.MatchString(challenge) {
		return grantRequest{}, &authorizeError{"invalid_request", "code_challenge must be an S256 challenge"}
	}
	resource := q.Get("resource")
	if resource == "" || !s.resourceAllowed(resource) {
		return grantRequest{}, &authorizeError{"invalid_target", "resource must name a protected resource served by this authorization server"}
	}
	scopes := append([]string(nil), client.Scopes...)
	if raw, ok := q["scope"]; ok {
		parsed, valid := parseScope(raw[0])
		if !valid || len(parsed) == 0 {
			return grantRequest{}, &authorizeError{"invalid_scope", "scope is malformed"}
		}
		scopes = parsed
	}
	// Reject, never downgrade: every requested scope must be supported and
	// registered for the client.
	for _, sc := range scopes {
		if !s.scopes[sc] || !contains(client.Scopes, sc) {
			return grantRequest{}, &authorizeError{"invalid_scope", "scope " + sc + " is not available to this client"}
		}
	}
	return grantRequest{ClientID: client.ID, RedirectURI: q.Get("redirect_uri"), Resource: resource, Scopes: scopes, Challenge: challenge}, nil
}

// redirectError sends an RFC 6749 error to a validated redirect URI.
func (s *Server) redirectError(w http.ResponseWriter, r *http.Request, redirectURI, state string, e *authorizeError) {
	params := url.Values{"error": {e.code}}
	if e.description != "" {
		params.Set("error_description", e.description)
	}
	s.redirect(w, r, redirectURI, state, params)
}

// redirect appends params, state and the RFC 9207 iss parameter.
func (s *Server) redirect(w http.ResponseWriter, r *http.Request, redirectURI, state string, params url.Values) {
	u, err := url.Parse(redirectURI)
	if err != nil {
		s.errorPage(w, http.StatusBadRequest, "The redirect URI is invalid.")
		return
	}
	q := u.Query()
	for k, v := range params {
		q[k] = v
	}
	if state != "" {
		q.Set("state", state)
	}
	q.Set("iss", s.cfg.Issuer)
	u.RawQuery = q.Encode()
	w.Header().Set("Cache-Control", "no-store")
	http.Redirect(w, r, u.String(), http.StatusFound)
}

func (s *Server) seal(req grantRequest) (string, error) {
	raw, err := json.Marshal(req)
	if err != nil {
		return "", err
	}
	payload := base64.RawURLEncoding.EncodeToString(raw)
	return payload + "." + base64.RawURLEncoding.EncodeToString(s.mac("consent-request", payload)), nil
}

var errSealed = errors.New("oauth: consent request is invalid")

func (s *Server) unseal(sealed string) (grantRequest, error) {
	payload, sig, ok := strings.Cut(sealed, ".")
	if !ok {
		return grantRequest{}, errSealed
	}
	got, err := base64.RawURLEncoding.DecodeString(sig)
	if err != nil || !hmac.Equal(got, s.mac("consent-request", payload)) {
		return grantRequest{}, errSealed
	}
	raw, err := base64.RawURLEncoding.DecodeString(payload)
	if err != nil {
		return grantRequest{}, errSealed
	}
	var req grantRequest
	if err := json.Unmarshal(raw, &req); err != nil {
		return grantRequest{}, errSealed
	}
	if s.now().Unix() >= req.Expires {
		return grantRequest{}, errSealed
	}
	return req, nil
}

// csrfToken binds the consent form to the session and the sealed request.
func (s *Server) csrfToken(sessionID, sealed string) string {
	return base64.RawURLEncoding.EncodeToString(s.mac("csrf", sessionID, sealed))
}

func hostOf(raw string) string {
	u, err := url.Parse(raw)
	if err != nil {
		return ""
	}
	return u.Host
}

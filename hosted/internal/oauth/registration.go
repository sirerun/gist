package oauth

import (
	"encoding/json"
	"fmt"
	"net/http"
	"net/url"
	"strings"
)

const (
	maxRedirectURIs  = 10
	maxRedirectBytes = 2048
	maxClientName    = 200
)

type registrationRequest struct {
	RedirectURIs            []string `json:"redirect_uris"`
	ClientName              string   `json:"client_name"`
	Scope                   *string  `json:"scope"`
	GrantTypes              []string `json:"grant_types"`
	ResponseTypes           []string `json:"response_types"`
	TokenEndpointAuthMethod string   `json:"token_endpoint_auth_method"`
}

type registrationResponse struct {
	ClientID                string   `json:"client_id"`
	ClientIDIssuedAt        int64    `json:"client_id_issued_at"`
	ClientName              string   `json:"client_name"`
	RedirectURIs            []string `json:"redirect_uris"`
	Scope                   string   `json:"scope"`
	GrantTypes              []string `json:"grant_types"`
	ResponseTypes           []string `json:"response_types"`
	TokenEndpointAuthMethod string   `json:"token_endpoint_auth_method"`
}

// handleRegister implements RFC 7591 dynamic client registration for public
// clients. Every redirect URI must be HTTPS. A requested scope outside the
// supported set is rejected with invalid_client_metadata, never trimmed.
func (s *Server) handleRegister(w http.ResponseWriter, r *http.Request) {
	if ct := r.Header.Get("Content-Type"); !strings.HasPrefix(strings.ToLower(ct), "application/json") {
		writeOAuthError(w, http.StatusBadRequest, "invalid_client_metadata", "registration requires application/json")
		return
	}
	var req registrationRequest
	dec := json.NewDecoder(r.Body)
	if err := dec.Decode(&req); err != nil {
		writeOAuthError(w, http.StatusBadRequest, "invalid_client_metadata", "malformed registration request")
		return
	}
	if len(req.RedirectURIs) == 0 || len(req.RedirectURIs) > maxRedirectURIs {
		writeOAuthError(w, http.StatusBadRequest, "invalid_redirect_uri", fmt.Sprintf("between 1 and %d redirect_uris are required", maxRedirectURIs))
		return
	}
	seen := map[string]bool{}
	var redirects []string
	for _, uri := range req.RedirectURIs {
		if err := validRedirectURI(uri); err != nil {
			writeOAuthError(w, http.StatusBadRequest, "invalid_redirect_uri", err.Error())
			return
		}
		if !seen[uri] {
			seen[uri] = true
			redirects = append(redirects, uri)
		}
	}
	name := strings.TrimSpace(req.ClientName)
	if len(name) > maxClientName || strings.ContainsAny(name, "\r\n\x00") {
		writeOAuthError(w, http.StatusBadRequest, "invalid_client_metadata", "client_name is too long or contains control characters")
		return
	}
	if name == "" {
		name = "Unnamed client"
	}
	if req.TokenEndpointAuthMethod != "" && req.TokenEndpointAuthMethod != "none" {
		writeOAuthError(w, http.StatusBadRequest, "invalid_client_metadata", "only public clients (token_endpoint_auth_method=none) are supported")
		return
	}
	for _, g := range req.GrantTypes {
		if g != "authorization_code" && g != "refresh_token" {
			writeOAuthError(w, http.StatusBadRequest, "invalid_client_metadata", "unsupported grant_type "+g)
			return
		}
	}
	for _, rt := range req.ResponseTypes {
		if rt != "code" {
			writeOAuthError(w, http.StatusBadRequest, "invalid_client_metadata", "unsupported response_type "+rt)
			return
		}
	}
	scopes := append([]string(nil), s.cfg.ScopesSupported...)
	if req.Scope != nil {
		parsed, ok := parseScope(*req.Scope)
		if !ok || len(parsed) == 0 {
			writeOAuthError(w, http.StatusBadRequest, "invalid_client_metadata", "scope is malformed")
			return
		}
		for _, sc := range parsed {
			if !s.scopes[sc] {
				writeOAuthError(w, http.StatusBadRequest, "invalid_client_metadata", "unsupported scope "+sc)
				return
			}
		}
		scopes = parsed
	}
	id, err := randomID(24)
	if err != nil {
		writeOAuthError(w, http.StatusInternalServerError, "server_error", "")
		return
	}
	now := s.now()
	client := Client{ID: "gc_" + id, Name: name, RedirectURIs: redirects, Scopes: scopes, Audience: s.cfg.Resources[0], CreatedAt: now}
	if err := s.cfg.Store.CreateClient(r.Context(), client); err != nil {
		writeOAuthError(w, http.StatusServiceUnavailable, "temporarily_unavailable", "")
		return
	}
	writeJSON(w, http.StatusCreated, registrationResponse{ClientID: client.ID, ClientIDIssuedAt: now.Unix(), ClientName: name, RedirectURIs: redirects, Scope: strings.Join(scopes, " "), GrantTypes: []string{"authorization_code", "refresh_token"}, ResponseTypes: []string{"code"}, TokenEndpointAuthMethod: "none"})
}

// validRedirectURI accepts only absolute HTTPS URIs with a host and no
// fragment or embedded credentials (RFC 6749 section 3.1.2, OAuth 2.1).
func validRedirectURI(uri string) error {
	if len(uri) > maxRedirectBytes {
		return fmt.Errorf("redirect_uri is too long")
	}
	u, err := url.Parse(uri)
	if err != nil || !u.IsAbs() {
		return fmt.Errorf("redirect_uri must be an absolute URI")
	}
	if u.Scheme != "https" {
		return fmt.Errorf("redirect_uri must use https")
	}
	if u.Host == "" || u.Hostname() == "" || strings.Contains(u.Hostname(), "*") {
		return fmt.Errorf("redirect_uri must name a host")
	}
	if u.Fragment != "" || strings.Contains(uri, "#") || u.User != nil {
		return fmt.Errorf("redirect_uri must not carry a fragment or credentials")
	}
	if strings.ContainsAny(uri, " \t\r\n") {
		return fmt.Errorf("redirect_uri contains whitespace")
	}
	return nil
}

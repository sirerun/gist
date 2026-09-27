package oauth

import (
	"encoding/json"
	"fmt"
	"net/http"
	"net/url"
	"strconv"
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
	if u.Scheme != "https" && !isLoopbackHTTP(u) {
		return fmt.Errorf("redirect_uri must use https, or http on the 127.0.0.1 or [::1] loopback")
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

// isLoopbackHTTP reports an OAuth 2.1 / RFC 8252 section 7.3 native-app
// redirect: plain http to the IPv4 or IPv6 loopback literal, on any port.
// The name localhost is deliberately excluded (it can be resolved away from
// the loopback interface), as is every other http host.
func isLoopbackHTTP(u *url.URL) bool {
	if u.Scheme != "http" {
		return false
	}
	host, port := u.Host, u.Port()
	if port != "" {
		host = strings.TrimSuffix(host, ":"+port)
		if n, err := strconv.Atoi(port); err != nil || n < 1 || n > 65535 {
			return false
		}
	} else if strings.HasSuffix(host, ":") {
		return false
	}
	return host == "127.0.0.1" || host == "[::1]"
}

// redirectRegistered matches a requested redirect URI against a client's
// registrations. Matching is exact, except that a loopback http redirect
// matches a registered loopback redirect on the same address, path and query
// with any port, because native apps bind an ephemeral port per request
// (RFC 8252 section 7.3).
func redirectRegistered(registered []string, uri string) bool {
	if contains(registered, uri) {
		return true
	}
	if validRedirectURI(uri) != nil {
		return false
	}
	req, err := url.Parse(uri)
	if err != nil || !isLoopbackHTTP(req) {
		return false
	}
	for _, r := range registered {
		reg, err := url.Parse(r)
		if err != nil || !isLoopbackHTTP(reg) {
			continue
		}
		if reg.Hostname() == req.Hostname() && reg.EscapedPath() == req.EscapedPath() && reg.RawQuery == req.RawQuery && reg.ForceQuery == req.ForceQuery {
			return true
		}
	}
	return false
}

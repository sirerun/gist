package oauth

import (
	"net/http"
	"net/url"
	"strings"
)

// ASMetadata is the RFC 8414 authorization-server metadata document.
type ASMetadata struct {
	Issuer                                 string   `json:"issuer"`
	AuthorizationEndpoint                  string   `json:"authorization_endpoint"`
	TokenEndpoint                          string   `json:"token_endpoint"`
	RegistrationEndpoint                   string   `json:"registration_endpoint"`
	RevocationEndpoint                     string   `json:"revocation_endpoint"`
	JWKSURI                                string   `json:"jwks_uri"`
	ScopesSupported                        []string `json:"scopes_supported"`
	ResponseTypesSupported                 []string `json:"response_types_supported"`
	ResponseModesSupported                 []string `json:"response_modes_supported"`
	GrantTypesSupported                    []string `json:"grant_types_supported"`
	CodeChallengeMethodsSupported          []string `json:"code_challenge_methods_supported"`
	TokenEndpointAuthMethodsSupported      []string `json:"token_endpoint_auth_methods_supported"`
	RevocationEndpointAuthMethodsSupported []string `json:"revocation_endpoint_auth_methods_supported"`
	AuthorizationResponseISSSupported      bool     `json:"authorization_response_iss_parameter_supported"`
	// ProtectedResources is the RFC 9728 section 4 list of resources this
	// server issues tokens for; RFC 8707 resource indicators must name one.
	ProtectedResources []string `json:"protected_resources"`
}

// ProtectedResourceMetadata is the RFC 9728 document.
type ProtectedResourceMetadata struct {
	Resource               string   `json:"resource"`
	AuthorizationServers   []string `json:"authorization_servers"`
	ScopesSupported        []string `json:"scopes_supported"`
	BearerMethodsSupported []string `json:"bearer_methods_supported"`
	ResourceName           string   `json:"resource_name,omitempty"`
}

// Metadata returns the server's RFC 8414 document.
func (s *Server) Metadata() ASMetadata {
	iss := s.cfg.Issuer
	return ASMetadata{
		Issuer:                                 iss,
		AuthorizationEndpoint:                  iss + PathAuthorize,
		TokenEndpoint:                          iss + PathToken,
		RegistrationEndpoint:                   iss + PathRegister,
		RevocationEndpoint:                     iss + PathRevoke,
		JWKSURI:                                iss + PathJWKS,
		ScopesSupported:                        append([]string(nil), s.cfg.ScopesSupported...),
		ResponseTypesSupported:                 []string{"code"},
		ResponseModesSupported:                 []string{"query"},
		GrantTypesSupported:                    []string{"authorization_code", "refresh_token"},
		CodeChallengeMethodsSupported:          []string{"S256"},
		TokenEndpointAuthMethodsSupported:      []string{"none"},
		RevocationEndpointAuthMethodsSupported: []string{"none"},
		AuthorizationResponseISSSupported:      true,
		ProtectedResources:                     append([]string(nil), s.cfg.Resources...),
	}
}

func (s *Server) handleASMetadata(w http.ResponseWriter, _ *http.Request) {
	writeMetadata(w, s.Metadata())
}

func (s *Server) handlePRMetadata(w http.ResponseWriter, _ *http.Request) {
	writeMetadata(w, ProtectedResourceMetadata{
		Resource:               s.cfg.Resources[0],
		AuthorizationServers:   []string{s.cfg.Issuer},
		ScopesSupported:        append([]string(nil), s.cfg.ScopesSupported...),
		BearerMethodsSupported: []string{"header"},
		ResourceName:           "Gist registry",
	})
}

func writeMetadata(w http.ResponseWriter, v any) {
	w.Header().Set("Content-Type", "application/json")
	w.Header().Set("Cache-Control", "public, max-age=300")
	w.WriteHeader(http.StatusOK)
	_ = jsonEncoder(w).Encode(v)
}

// prmURL derives the RFC 9728 section 3.1 metadata URL: the well-known
// segment is inserted between the host and any resource path.
//
// This assumes the resource identifier equals the registry origin (app
// config enforces ResourceAudience == PublicOrigin), so the metadata is
// served by this same process. A resource with a path, or on another origin,
// would yield a URL this server does not route; revisit before allowing one.
func prmURL(resource string) string {
	u, err := url.Parse(resource)
	if err != nil {
		return ""
	}
	path := strings.TrimSuffix(u.Path, "/")
	return u.Scheme + "://" + u.Host + PathPRMetadata + path
}

// BearerChallenge builds an RFC 6750 / RFC 9728 challenge that points the
// client at the protected-resource metadata, not at the AS metadata.
func BearerChallenge(resourceMetadataURL, errorCode string) string {
	v := `Bearer realm="gist", resource_metadata="` + resourceMetadataURL + `"`
	if errorCode != "" {
		v += `, error="` + errorCode + `"`
	}
	return v
}

// WithChallenge wraps a protected-resource handler so every 401 it writes
// carries the RFC 9728 challenge. A request that presented a bearer token
// gets error="invalid_token"; one without credentials gets none (RFC 6750
// section 3.1).
func WithChallenge(next http.Handler, resourceMetadataURL string) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		code := ""
		if strings.HasPrefix(r.Header.Get("Authorization"), "Bearer ") {
			code = "invalid_token"
		}
		next.ServeHTTP(&challengeWriter{ResponseWriter: w, challenge: BearerChallenge(resourceMetadataURL, code)}, r)
	})
}

type challengeWriter struct {
	http.ResponseWriter
	challenge   string
	wroteHeader bool
}

func (c *challengeWriter) WriteHeader(status int) {
	if !c.wroteHeader {
		c.wroteHeader = true
		if status == http.StatusUnauthorized {
			c.ResponseWriter.Header().Set("WWW-Authenticate", c.challenge)
		}
	}
	c.ResponseWriter.WriteHeader(status)
}

func (c *challengeWriter) Write(b []byte) (int, error) {
	if !c.wroteHeader {
		c.WriteHeader(http.StatusOK)
	}
	return c.ResponseWriter.Write(b)
}

// Unwrap lets http.ResponseController reach the underlying writer.
func (c *challengeWriter) Unwrap() http.ResponseWriter { return c.ResponseWriter }

package oauth

import (
	"bytes"
	"crypto/hmac"
	_ "embed"
	"encoding/base64"
	"fmt"
	"html/template"
	"net/http"
	"net/url"
	"strings"
)

//go:embed consent.html
var consentTemplate string

type consentPage struct{ tmpl *template.Template }

type consentView struct {
	ClientName   string
	RedirectHost string
	Resource     string
	Scopes       []string
	Workspaces   []string
	Request      string
	CSRF         string
	Action       string
}

func newConsentPage() (*consentPage, error) {
	tmpl, err := template.New("oauth").Parse(consentTemplate)
	if err != nil {
		return nil, fmt.Errorf("oauth: parse consent template: %w", err)
	}
	return &consentPage{tmpl: tmpl}, nil
}

// pageHeaders forbid framing (clickjacking), caching and inline scripts.
func pageHeaders(w http.ResponseWriter) {
	h := w.Header()
	h.Set("Content-Type", "text/html; charset=utf-8")
	h.Set("Cache-Control", "no-store")
	h.Set("X-Frame-Options", "DENY")
	h.Set("Content-Security-Policy", "default-src 'none'; style-src 'unsafe-inline'; form-action 'self'; frame-ancestors 'none'; base-uri 'none'")
	h.Set("Referrer-Policy", "no-referrer")
	h.Set("X-Content-Type-Options", "nosniff")
}

func (s *Server) renderConsent(w http.ResponseWriter, v consentView) {
	var buf bytes.Buffer
	if err := s.consent.tmpl.ExecuteTemplate(&buf, "consent", v); err != nil {
		s.errorPage(w, http.StatusInternalServerError, "The consent page could not be rendered.")
		return
	}
	pageHeaders(w)
	w.WriteHeader(http.StatusOK)
	_, _ = w.Write(buf.Bytes())
}

func (s *Server) errorPage(w http.ResponseWriter, status int, message string) {
	var buf bytes.Buffer
	_ = s.consent.tmpl.ExecuteTemplate(&buf, "error", message)
	pageHeaders(w)
	w.WriteHeader(status)
	_, _ = w.Write(buf.Bytes())
}

// handleConsent records the person's decision. It requires the same session
// that saw the consent page, a valid CSRF token and a same-origin POST. Only
// "allow" issues a code; "deny" returns access_denied and grants nothing.
func (s *Server) handleConsent(w http.ResponseWriter, r *http.Request) {
	if !s.sameOrigin(r) {
		s.errorPage(w, http.StatusForbidden, "Cross-site consent requests are refused.")
		return
	}
	if err := r.ParseForm(); err != nil {
		s.errorPage(w, http.StatusBadRequest, "The consent form is malformed.")
		return
	}
	for key, values := range r.PostForm {
		if len(values) > 1 {
			s.errorPage(w, http.StatusBadRequest, "The consent form repeats the "+key+" field.")
			return
		}
	}
	sess, err := s.cfg.Sessions.Authenticate(r)
	if err != nil {
		s.errorPage(w, http.StatusUnauthorized, "Your session has ended. Sign in and retry the connection.")
		return
	}
	sealed := r.PostForm.Get("request")
	req, err := s.unseal(sealed)
	if err != nil || req.SessionID != sess.ID || req.Subject != sess.Subject {
		s.errorPage(w, http.StatusForbidden, "The consent request is invalid or expired.")
		return
	}
	csrf, err := base64.RawURLEncoding.DecodeString(r.PostForm.Get("csrf"))
	if err != nil || !hmac.Equal(csrf, s.mac("csrf", sess.ID, sealed)) {
		s.errorPage(w, http.StatusForbidden, "The consent form failed its CSRF check.")
		return
	}
	client, err := s.cfg.Store.GetClient(r.Context(), req.ClientID)
	if err != nil || !redirectRegistered(client.RedirectURIs, req.RedirectURI) {
		s.errorPage(w, http.StatusBadRequest, "The client is no longer registered.")
		return
	}
	switch r.PostForm.Get("decision") {
	case "deny":
		s.redirectError(w, r, req.RedirectURI, req.State, &authorizeError{"access_denied", "the user denied the request"})
		return
	case "allow":
	default:
		s.errorPage(w, http.StatusBadRequest, "The consent decision is missing.")
		return
	}
	workspace := r.PostForm.Get("workspace")
	if workspace == "" || !contains(sess.Workspaces, workspace) {
		s.redirectError(w, r, req.RedirectURI, req.State, &authorizeError{"access_denied", "the selected workspace is not available to this session"})
		return
	}
	if aerr := s.checkMembership(r, sess.Subject, workspace, req.Scopes); aerr != nil {
		s.redirectError(w, r, req.RedirectURI, req.State, aerr)
		return
	}
	raw, err := newOpaque(codePrefix, workspace)
	if err != nil {
		s.redirectError(w, r, req.RedirectURI, req.State, &authorizeError{"server_error", ""})
		return
	}
	now := s.now()
	code := AuthCode{Hash: hashSecret(raw), ClientID: req.ClientID, Subject: sess.Subject, WorkspaceID: workspace, RedirectURI: req.RedirectURI, Resource: req.Resource, Scopes: req.Scopes, Challenge: req.Challenge, IssuedAt: now, ExpiresAt: now.Add(s.cfg.CodeTTL)}
	if err := s.cfg.Store.SaveCode(r.Context(), code); err != nil {
		s.redirectError(w, r, req.RedirectURI, req.State, &authorizeError{"temporarily_unavailable", ""})
		return
	}
	s.redirect(w, r, req.RedirectURI, req.State, url.Values{"code": {raw}})
}

// checkMembership requires a live membership whose scope ceiling covers
// every requested scope. Scopes beyond the ceiling are rejected with
// invalid_scope, never trimmed; any other failure denies the request.
func (s *Server) checkMembership(r *http.Request, subject, workspace string, scopes []string) *authorizeError {
	policy, err := s.cfg.Policy.CheckWorkload(r.Context(), subject, workspace, scopes, 0)
	if err != nil {
		return &authorizeError{"access_denied", "no active membership in the selected workspace"}
	}
	if policy.Revoked {
		return &authorizeError{"access_denied", "membership is revoked"}
	}
	if len(policy.ParentScopes) > 0 && !containsAll(policy.ParentScopes, scopes) {
		return &authorizeError{"invalid_scope", "requested scope exceeds your role in the selected workspace"}
	}
	if !policy.Allowed {
		return &authorizeError{"access_denied", "membership does not allow this request"}
	}
	return nil
}

// sameOrigin refuses a consent POST that a browser marks as cross-site.
func (s *Server) sameOrigin(r *http.Request) bool {
	if site := r.Header.Get("Sec-Fetch-Site"); site != "" && site != "same-origin" && site != "none" {
		return false
	}
	origin := r.Header.Get("Origin")
	if origin == "" || origin == "null" {
		return origin == ""
	}
	return strings.EqualFold(origin, s.cfg.Issuer)
}

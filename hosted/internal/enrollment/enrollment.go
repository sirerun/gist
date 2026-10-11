package enrollment

import (
	"bytes"
	"context"
	"crypto/aes"
	"crypto/cipher"
	"crypto/hmac"
	"crypto/rand"
	"crypto/sha256"
	"encoding/base64"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"mime"
	"net/http"
	"net/url"
	"strconv"
	"strings"
	"time"
)

const (
	StartPath       = "/auth/github/start"
	CallbackPath    = "/auth/github/callback"
	InvitationsPath = "/operator/invitations"
	FlowCookieName  = "__Host-gist-enroll-flow"
	maxRequestBytes = 8 << 10
)

var (
	ErrDenied = errors.New("enrollment denied")
	ErrReplay = errors.New("enrollment flow already used or expired")
)

type Store interface {
	CreateFlow(context.Context, []byte, time.Time) error
	ConsumeFlow(context.Context, []byte, time.Time) error
	CompleteLogin(context.Context, string, string, []byte, string, string, []string, time.Time) error
	CreateInvitation(context.Context, string, string, string, string, []string, []byte, time.Time) error
}
type SessionIssuer interface {
	Issue(string, []string, time.Duration) (*http.Cookie, error)
}
type Session struct {
	ID, Subject string
	Workspaces  []string
}
type SessionAuthenticator interface {
	Authenticate(*http.Request) (Session, error)
}
type Config struct {
	ClientID, ClientSecret, CallbackURL, PublicOrigin string
	WorkspaceID, WorkspaceName, FounderAccount        string
	BootstrapScopes, AllowedScopes                    []string
	FlowKey, CSRFKey                                  []byte
	FlowTTL, SessionTTL, InvitationTTL                time.Duration
}
type Handler struct {
	cfg      Config
	store    Store
	sessions SessionIssuer
	auth     SessionAuthenticator
	client   *http.Client
	now      func() time.Time
}

func New(cfg Config, store Store, sessions SessionIssuer, auth SessionAuthenticator, client *http.Client) (*Handler, error) {
	if cfg.ClientID == "" || cfg.ClientSecret == "" || store == nil || sessions == nil || auth == nil {
		return nil, errors.New("enrollment: client credentials, store, and session services are required")
	}
	cb, err := url.Parse(cfg.CallbackURL)
	if err != nil || cb.Scheme != "https" || cb.Host == "" || cb.User != nil || cb.Fragment != "" || cb.RawQuery != "" || cb.Path != CallbackPath {
		return nil, errors.New("enrollment: callback URL must be a fixed HTTPS callback")
	}
	origin, err := url.Parse(cfg.PublicOrigin)
	if err != nil || origin.Scheme != "https" || origin.Host == "" || origin.User != nil || origin.Path != "" || origin.RawQuery != "" || origin.Fragment != "" || cb.Host != origin.Host {
		return nil, errors.New("enrollment: public origin must be an HTTPS origin")
	}
	if cfg.WorkspaceID == "" || cfg.WorkspaceName == "" || !canonicalID(cfg.FounderAccount) || len(cfg.FlowKey) != 32 || len(cfg.CSRFKey) < 32 {
		return nil, errors.New("enrollment: workspace, founder account, and strong flow/CSRF keys are required")
	}
	if cfg.FlowTTL <= 0 || cfg.FlowTTL > 10*time.Minute {
		cfg.FlowTTL = 5 * time.Minute
	}
	if cfg.SessionTTL <= 0 || cfg.SessionTTL > 24*time.Hour {
		cfg.SessionTTL = 8 * time.Hour
	}
	if cfg.InvitationTTL <= 0 || cfg.InvitationTTL > 7*24*time.Hour {
		cfg.InvitationTTL = 24 * time.Hour
	}
	if len(cfg.BootstrapScopes) == 0 {
		cfg.BootstrapScopes = []string{"catalog:read", "catalog:publish", "catalog:resolve"}
	}
	if len(cfg.AllowedScopes) == 0 {
		cfg.AllowedScopes = []string{"catalog:read", "catalog:publish", "catalog:resolve"}
	}
	if !subset(cfg.BootstrapScopes, cfg.AllowedScopes) {
		return nil, errors.New("enrollment: bootstrap scopes exceed allowed workspace scopes")
	}
	cfg.FlowKey = bytes.Clone(cfg.FlowKey)
	cfg.CSRFKey = bytes.Clone(cfg.CSRFKey)
	cfg.BootstrapScopes = append([]string(nil), cfg.BootstrapScopes...)
	cfg.AllowedScopes = append([]string(nil), cfg.AllowedScopes...)
	if client == nil {
		client = &http.Client{Timeout: 10 * time.Second}
	} else {
		c := *client
		if c.Timeout <= 0 {
			c.Timeout = 10 * time.Second
		}
		client = &c
	}
	client.CheckRedirect = func(*http.Request, []*http.Request) error { return http.ErrUseLastResponse }
	return &Handler{cfg: cfg, store: store, sessions: sessions, auth: auth, client: client, now: time.Now}, nil
}
func (h *Handler) Routes(m *http.ServeMux) {
	m.HandleFunc(StartPath, h.start)
	m.HandleFunc(CallbackPath, h.callback)
	m.HandleFunc(InvitationsPath, h.createInvitation)
}
func (h *Handler) start(w http.ResponseWriter, r *http.Request) {
	w.Header().Set("Cache-Control", "no-store")
	w.Header().Set("Referrer-Policy", "no-referrer")
	invite := ""
	switch r.Method {
	case http.MethodGet:
		if r.URL.Query().Has("invite") {
			http.Error(w, "invitation must be submitted in a same-origin form", http.StatusBadRequest)
			return
		}
	case http.MethodPost:
		if r.URL.Query().Has("invite") {
			http.Error(w, "invitation must be submitted in the request body", http.StatusBadRequest)
			return
		}
		if r.Header.Get("Origin") != h.cfg.PublicOrigin {
			http.Error(w, "request denied", http.StatusForbidden)
			return
		}
		mediaType, _, err := mime.ParseMediaType(r.Header.Get("Content-Type"))
		if err != nil || mediaType != "application/x-www-form-urlencoded" {
			http.Error(w, "unsupported media type", http.StatusUnsupportedMediaType)
			return
		}
		r.Body = http.MaxBytesReader(w, r.Body, maxRequestBytes)
		if err := r.ParseForm(); err != nil || len(r.PostForm["invite"]) != 1 {
			http.Error(w, "invalid invitation", http.StatusBadRequest)
			return
		}
		invite = r.PostForm.Get("invite")
	default:
		w.Header().Set("Allow", "GET, POST")
		http.Error(w, "method not allowed", http.StatusMethodNotAllowed)
		return
	}
	if invite != "" {
		raw, e := base64.RawURLEncoding.DecodeString(invite)
		if e != nil || len(raw) != 32 {
			http.Error(w, "invalid invitation", http.StatusBadRequest)
			return
		}
	}
	state, err := randomToken(32)
	if err != nil {
		http.Error(w, "login unavailable", http.StatusInternalServerError)
		return
	}
	verifier, err := randomToken(32)
	if err != nil {
		http.Error(w, "login unavailable", http.StatusInternalServerError)
		return
	}
	expires := h.now().UTC().Add(h.cfg.FlowTTL)
	cookie, err := h.sealFlow(state, verifier, invite, expires)
	if err != nil {
		http.Error(w, "login unavailable", http.StatusInternalServerError)
		return
	}
	if err = h.store.CreateFlow(r.Context(), hashToken(state), expires); err != nil {
		http.Error(w, "login unavailable", http.StatusInternalServerError)
		return
	}
	http.SetCookie(w, cookie)
	challenge := sha256.Sum256([]byte(verifier))
	u, _ := url.Parse("https://github.com/login/oauth/authorize")
	q := u.Query()
	q.Set("client_id", h.cfg.ClientID)
	q.Set("redirect_uri", h.cfg.CallbackURL)
	q.Set("state", state)
	q.Set("code_challenge", base64.RawURLEncoding.EncodeToString(challenge[:]))
	q.Set("code_challenge_method", "S256")
	u.RawQuery = q.Encode()
	http.Redirect(w, r, u.String(), http.StatusFound)
}
func (h *Handler) callback(w http.ResponseWriter, r *http.Request) {
	w.Header().Set("Cache-Control", "no-store")
	w.Header().Set("Referrer-Policy", "no-referrer")
	if r.Method != http.MethodGet {
		w.Header().Set("Allow", http.MethodGet)
		http.Error(w, "method not allowed", http.StatusMethodNotAllowed)
		return
	}
	cookie, err := r.Cookie(FlowCookieName)
	clearFlowCookie(w)
	q := r.URL.Query()
	if err != nil || len(q["state"]) != 1 || len(q["code"]) != 1 || q.Get("code") == "" || len(q.Get("code")) > 4096 || q.Get("error") != "" {
		http.Error(w, "login denied", http.StatusBadRequest)
		return
	}
	state, verifier, invite, expires, err := h.openFlow(cookie.Value)
	if err != nil || !hmac.Equal([]byte(state), []byte(q.Get("state"))) || !h.now().Before(expires) {
		http.Error(w, "login denied", http.StatusBadRequest)
		return
	}
	if err = h.store.ConsumeFlow(r.Context(), hashToken(state), h.now().UTC()); err != nil {
		if errors.Is(err, ErrReplay) {
			http.Error(w, "login denied", http.StatusBadRequest)
		} else {
			http.Error(w, "login unavailable", http.StatusInternalServerError)
		}
		return
	}
	account, err := h.githubAccount(r.Context(), q.Get("code"), verifier)
	if err != nil {
		http.Error(w, "login denied", http.StatusUnauthorized)
		return
	}
	inviteHash := []byte(nil)
	if invite != "" {
		inviteHash = hashToken(invite)
	}
	if err = h.store.CompleteLogin(r.Context(), h.cfg.WorkspaceID, account, inviteHash, h.cfg.FounderAccount, h.cfg.WorkspaceName, h.cfg.BootstrapScopes, h.now().UTC()); err != nil {
		if errors.Is(err, ErrDenied) {
			http.Error(w, "workspace access denied", http.StatusForbidden)
		} else {
			http.Error(w, "login unavailable", http.StatusInternalServerError)
		}
		return
	}
	sessionCookie, err := h.sessions.Issue(account, []string{h.cfg.WorkspaceID}, h.cfg.SessionTTL)
	if err != nil {
		http.Error(w, "login unavailable", http.StatusInternalServerError)
		return
	}
	http.SetCookie(w, sessionCookie)
	writeJSON(w, http.StatusOK, map[string]any{"authenticated": true, "csrf_token": h.csrf(account)})
}
func (h *Handler) githubAccount(ctx context.Context, code, verifier string) (string, error) {
	form := url.Values{}
	form.Set("client_id", h.cfg.ClientID)
	form.Set("client_secret", h.cfg.ClientSecret)
	form.Set("code", code)
	form.Set("redirect_uri", h.cfg.CallbackURL)
	form.Set("code_verifier", verifier)
	req, err := http.NewRequestWithContext(ctx, http.MethodPost, "https://github.com/login/oauth/access_token", strings.NewReader(form.Encode()))
	if err != nil {
		return "", fmt.Errorf("enrollment: create GitHub token request: %w", err)
	}
	req.Header.Set("Accept", "application/json")
	req.Header.Set("Content-Type", "application/x-www-form-urlencoded")
	resp, err := h.client.Do(req)
	if err != nil {
		return "", fmt.Errorf("enrollment: exchange GitHub code: %w", err)
	}
	var token struct {
		AccessToken string `json:"access_token"`
		Error       string `json:"error"`
	}
	if resp.StatusCode != 200 {
		_ = resp.Body.Close()
		return "", errors.New("enrollment: GitHub token exchange rejected")
	}
	if err = decodeLimited(resp.Body, 16<<10, &token); err != nil {
		return "", fmt.Errorf("enrollment: decode GitHub token response: %w", err)
	}
	if token.AccessToken == "" || token.Error != "" {
		return "", errors.New("enrollment: GitHub token response has no usable token")
	}
	userReq, err := http.NewRequestWithContext(ctx, http.MethodGet, "https://api.github.com/user", nil)
	if err != nil {
		return "", fmt.Errorf("enrollment: create GitHub identity request: %w", err)
	}
	accessToken := token.AccessToken
	token.AccessToken = ""
	userReq.Header.Set("Authorization", "Bearer "+accessToken)
	userReq.Header.Set("Accept", "application/vnd.github+json")
	userReq.Header.Set("X-GitHub-Api-Version", "2022-11-28")
	userResp, err := h.client.Do(userReq)
	userReq.Header.Del("Authorization")
	if err != nil {
		return "", fmt.Errorf("enrollment: fetch GitHub account ID: %w", err)
	}
	if userResp.StatusCode != 200 {
		_ = userResp.Body.Close()
		return "", errors.New("enrollment: GitHub identity request rejected")
	}
	var user struct {
		ID json.Number `json:"id"`
	}
	if err = decodeLimited(userResp.Body, 16<<10, &user); err != nil {
		return "", fmt.Errorf("enrollment: decode GitHub identity response: %w", err)
	}
	id, err := strconv.ParseInt(string(user.ID), 10, 64)
	if err != nil || id <= 0 {
		return "", errors.New("enrollment: GitHub returned an invalid account ID")
	}
	return strconv.FormatInt(id, 10), nil
}
func (h *Handler) createInvitation(w http.ResponseWriter, r *http.Request) {
	w.Header().Set("Cache-Control", "no-store")
	if r.Method != http.MethodPost {
		w.Header().Set("Allow", http.MethodPost)
		http.Error(w, "method not allowed", http.StatusMethodNotAllowed)
		return
	}
	if r.Header.Get("Origin") != h.cfg.PublicOrigin {
		http.Error(w, "request denied", http.StatusForbidden)
		return
	}
	principal, err := h.auth.Authenticate(r)
	if err != nil || principal.ID == "" || principal.Subject == "" || !contains(principal.Workspaces, h.cfg.WorkspaceID) {
		http.Error(w, "authentication required", http.StatusUnauthorized)
		return
	}
	if !hmac.Equal([]byte(r.Header.Get("X-Gist-CSRF")), []byte(h.csrf(principal.Subject))) {
		http.Error(w, "request denied", http.StatusForbidden)
		return
	}
	if r.Body == nil {
		http.Error(w, "invalid invitation", http.StatusBadRequest)
		return
	}
	r.Body = http.MaxBytesReader(w, r.Body, maxRequestBytes)
	dec := json.NewDecoder(r.Body)
	dec.DisallowUnknownFields()
	var in struct {
		GitHubAccountID string   `json:"github_account_id"`
		Role            string   `json:"role"`
		Scopes          []string `json:"scopes"`
	}
	if err = dec.Decode(&in); err != nil || dec.Decode(new(any)) != io.EOF || !canonicalID(in.GitHubAccountID) || (in.Role != "reader" && in.Role != "maintainer") || len(in.Scopes) == 0 || !subset(in.Scopes, h.cfg.AllowedScopes) {
		http.Error(w, "invalid invitation", http.StatusBadRequest)
		return
	}
	raw, err := randomToken(32)
	if err != nil {
		http.Error(w, "invitation unavailable", http.StatusInternalServerError)
		return
	}
	expires := h.now().UTC().Add(h.cfg.InvitationTTL)
	if err = h.store.CreateInvitation(r.Context(), h.cfg.WorkspaceID, principal.Subject, in.GitHubAccountID, in.Role, in.Scopes, hashToken(raw), expires); err != nil {
		if errors.Is(err, ErrDenied) {
			http.Error(w, "request denied", http.StatusForbidden)
		} else {
			http.Error(w, "invitation unavailable", http.StatusInternalServerError)
		}
		return
	}
	writeJSON(w, http.StatusCreated, map[string]any{"invitation": raw, "expires_at": expires.Format(time.RFC3339)})
}
func (h *Handler) csrf(subject string) string {
	m := hmac.New(sha256.New, h.cfg.CSRFKey)
	_, _ = m.Write([]byte("gist-enrollment-csrf\x00" + subject + "\x00" + h.cfg.WorkspaceID))
	return base64.RawURLEncoding.EncodeToString(m.Sum(nil))
}

type flowClaims struct {
	State      string
	Verifier   string
	Invitation string
	Expires    int64
}

func (h *Handler) sealFlow(state, verifier, invitation string, expires time.Time) (*http.Cookie, error) {
	block, err := aes.NewCipher(h.cfg.FlowKey)
	if err != nil {
		return nil, err
	}
	aead, err := cipher.NewGCM(block)
	if err != nil {
		return nil, err
	}
	raw, err := json.Marshal(flowClaims{State: state, Verifier: verifier, Invitation: invitation, Expires: expires.Unix()})
	if err != nil {
		return nil, err
	}
	nonce := make([]byte, aead.NonceSize())
	if _, err = rand.Read(nonce); err != nil {
		return nil, err
	}
	sealed := aead.Seal(nonce, nonce, raw, []byte(FlowCookieName))
	return &http.Cookie{Name: FlowCookieName, Value: base64.RawURLEncoding.EncodeToString(sealed), Path: "/", Expires: expires, MaxAge: int(h.cfg.FlowTTL.Seconds()), Secure: true, HttpOnly: true, SameSite: http.SameSiteLaxMode}, nil
}
func (h *Handler) openFlow(value string) (string, string, string, time.Time, error) {
	sealed, err := base64.RawURLEncoding.DecodeString(value)
	if err != nil {
		return "", "", "", time.Time{}, errors.New("enrollment: malformed flow cookie")
	}
	block, err := aes.NewCipher(h.cfg.FlowKey)
	if err != nil {
		return "", "", "", time.Time{}, err
	}
	aead, err := cipher.NewGCM(block)
	if err != nil {
		return "", "", "", time.Time{}, err
	}
	if len(sealed) < aead.NonceSize() {
		return "", "", "", time.Time{}, errors.New("enrollment: malformed flow cookie")
	}
	raw, err := aead.Open(nil, sealed[:aead.NonceSize()], sealed[aead.NonceSize():], []byte(FlowCookieName))
	if err != nil {
		return "", "", "", time.Time{}, errors.New("enrollment: invalid flow cookie")
	}
	var c flowClaims
	if err = json.Unmarshal(raw, &c); err != nil || !canonicalToken(c.State) || !canonicalToken(c.Verifier) || (c.Invitation != "" && !canonicalToken(c.Invitation)) {
		return "", "", "", time.Time{}, errors.New("enrollment: invalid flow claims")
	}
	return c.State, c.Verifier, c.Invitation, time.Unix(c.Expires, 0).UTC(), nil
}
func clearFlowCookie(w http.ResponseWriter) {
	http.SetCookie(w, &http.Cookie{Name: FlowCookieName, Value: "", Path: "/", Expires: time.Unix(1, 0), MaxAge: -1, Secure: true, HttpOnly: true, SameSite: http.SameSiteLaxMode})
}
func decodeLimited(body io.ReadCloser, limit int64, dst any) (retErr error) {
	defer func() {
		if err := body.Close(); err != nil && retErr == nil {
			retErr = fmt.Errorf("enrollment: close GitHub response body: %w", err)
		}
	}()
	raw, err := io.ReadAll(io.LimitReader(body, limit+1))
	if err != nil {
		return err
	}
	if int64(len(raw)) > limit {
		return errors.New("enrollment: oversized JSON response")
	}
	dec := json.NewDecoder(bytes.NewReader(raw))
	dec.UseNumber()
	if err := dec.Decode(dst); err != nil {
		return err
	}
	var extra any
	if err := dec.Decode(&extra); err != io.EOF {
		return errors.New("enrollment: invalid JSON response")
	}
	return nil
}
func randomToken(n int) (string, error) {
	b := make([]byte, n)
	if _, err := rand.Read(b); err != nil {
		return "", err
	}
	return base64.RawURLEncoding.EncodeToString(b), nil
}
func hashToken(v string) []byte { s := sha256.Sum256([]byte(v)); return s[:] }
func canonicalToken(v string) bool {
	if v == "" || len(v) > 128 {
		return false
	}
	b, e := base64.RawURLEncoding.DecodeString(v)
	return e == nil && base64.RawURLEncoding.EncodeToString(b) == v
}
func canonicalID(v string) bool {
	if v == "" || len(v) > 19 || v[0] == '0' {
		return false
	}
	n, e := strconv.ParseInt(v, 10, 64)
	return e == nil && n > 0 && strconv.FormatInt(n, 10) == v
}
func contains(items []string, v string) bool {
	for _, x := range items {
		if x == v {
			return true
		}
	}
	return false
}
func subset(items, allowed []string) bool {
	seen := map[string]bool{}
	for _, v := range items {
		if v == "" || seen[v] || !contains(allowed, v) {
			return false
		}
		seen[v] = true
	}
	return len(seen) == len(items)
}
func writeJSON(w http.ResponseWriter, status int, v any) {
	w.Header().Set("Content-Type", "application/json")
	w.Header().Set("Cache-Control", "no-store")
	w.Header().Set("X-Content-Type-Options", "nosniff")
	w.WriteHeader(status)
	_ = json.NewEncoder(w).Encode(v)
}

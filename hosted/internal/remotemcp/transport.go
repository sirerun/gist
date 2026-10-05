package remotemcp

import (
	"bytes"
	"container/list"
	"crypto/rand"
	"encoding/json"
	"errors"
	"io"
	"net/http"
	"net/url"
	"sync"
	"time"

	"github.com/sirerun/gist/hosted/internal/ports"
	"github.com/sirerun/gist/hosted/internal/rest"
)

type Config struct {
	Services       rest.Services
	AllowedOrigins map[string]bool
	MaxBodyBytes   int64
	MaxCalls       int
	// SessionTTL expires a session that has not been used for this long.
	SessionTTL time.Duration
	// MaxSessions caps live sessions machine-wide; the least recently used
	// idle session is evicted.
	MaxSessions int
	// MaxSessionsPerPrincipal caps live sessions for one authenticated
	// principal; at the cap that principal's own least recently used idle
	// session is evicted, so one caller cannot push out another's sessions.
	MaxSessionsPerPrincipal int
	// MaxSessionsPerWorkspace caps live sessions for one workspace across all
	// of its principals; at the cap that workspace's own least recently used
	// idle session is evicted, so one tenant cannot push out another's.
	MaxSessionsPerWorkspace int
	// MaxConcurrentRequests caps requests in flight on the MCP endpoint. The
	// slot is taken before authentication, so an unauthenticated flood cannot
	// drive unbounded token verification. Zero uses Services.Limits.RateLimit,
	// or defaultMaxConcurrentRequests when that is unset too.
	MaxConcurrentRequests int
}

const (
	defaultSessionTTL  = time.Hour
	defaultMaxSessions = 10000
	// defaultMaxSessionsPerPrincipal bounds one caller's share of the table.
	defaultMaxSessionsPerPrincipal = 32
	// defaultMaxSessionsPerWorkspace bounds one tenant's share of the table.
	defaultMaxSessionsPerWorkspace = 2000
	// defaultMaxConcurrentRequests matches the REST default in-flight cap.
	defaultMaxConcurrentRequests = 100
	// retryAfterSeconds is sent with a 429 when every request slot is taken.
	retryAfterSeconds = "1"
)

type Handler struct {
	rest     *rest.Handler
	cfg      Config
	mu       sync.Mutex
	sessions map[string]*session
	// lru orders every live session, most recently used at the front.
	lru *list.List
	// byPrincipal orders each principal's sessions, most recent at the front.
	byPrincipal map[string]*list.List
	// byWorkspace orders each workspace's sessions, most recent at the front.
	byWorkspace map[string]*list.List
	calls       map[string]int // in-flight tools/call count per session
	// slots bounds requests in flight on this transport, authentication
	// included; a full semaphore answers 429.
	slots chan struct{}
	now   func() time.Time
}

// session is one Mcp-Session-Id bound to the principal that created it.
type session struct {
	id        string
	principal string
	workspace string
	last      time.Time
	all       *list.Element // element in Handler.lru
	own       *list.Element // element in Handler.byPrincipal[principal]
	ws        *list.Element // element in Handler.byWorkspace[workspace]
}

func New(c Config) (*Handler, error) {
	if c.MaxBodyBytes <= 0 {
		c.MaxBodyBytes = 10 << 20
	}
	if c.MaxCalls <= 0 {
		c.MaxCalls = 100
	}
	if c.SessionTTL <= 0 {
		c.SessionTTL = defaultSessionTTL
	}
	if c.MaxSessions <= 0 {
		c.MaxSessions = defaultMaxSessions
	}
	if c.MaxSessionsPerPrincipal <= 0 {
		c.MaxSessionsPerPrincipal = defaultMaxSessionsPerPrincipal
	}
	if c.MaxSessionsPerWorkspace <= 0 {
		c.MaxSessionsPerWorkspace = defaultMaxSessionsPerWorkspace
	}
	if c.MaxSessionsPerWorkspace > c.MaxSessions {
		c.MaxSessionsPerWorkspace = c.MaxSessions
	}
	if c.MaxSessionsPerPrincipal > c.MaxSessionsPerWorkspace {
		c.MaxSessionsPerPrincipal = c.MaxSessionsPerWorkspace
	}
	if c.MaxConcurrentRequests <= 0 {
		c.MaxConcurrentRequests = c.Services.Limits.RateLimit
	}
	if c.MaxConcurrentRequests <= 0 {
		c.MaxConcurrentRequests = defaultMaxConcurrentRequests
	}
	rh, err := rest.New(c.Services)
	if err != nil {
		return nil, err
	}
	return &Handler{
		rest:        rh,
		cfg:         c,
		sessions:    map[string]*session{},
		lru:         list.New(),
		byPrincipal: map[string]*list.List{},
		byWorkspace: map[string]*list.List{},
		calls:       map[string]int{},
		slots:       make(chan struct{}, c.MaxConcurrentRequests),
		now:         time.Now,
	}, nil
}
func (h *Handler) ServeHTTP(w http.ResponseWriter, r *http.Request) {
	if r.Method != "POST" {
		w.Header().Set("Allow", "POST")
		w.WriteHeader(http.StatusMethodNotAllowed)
		return
	}
	if !h.originOK(r) {
		http.Error(w, "Forbidden origin", http.StatusForbidden)
		return
	}
	// The in-flight slot is taken before authentication, as on the REST
	// routes, so a flood of unauthenticated requests cannot drive unbounded
	// token verification.
	select {
	case h.slots <- struct{}{}:
		defer func() { <-h.slots }()
	default:
		w.Header().Set("Retry-After", retryAfterSeconds)
		w.Header().Set("Content-Type", "application/json")
		w.WriteHeader(http.StatusTooManyRequests)
		_, _ = w.Write(marshal(map[string]any{"code": "rate_limited", "message": "Rate limit exceeded", "retryable": true}))
		return
	}
	// Every request, initialize included, is authenticated with the same
	// identity check the REST routes use before any session state is touched,
	// so an unauthenticated client cannot create or evict sessions.
	principal, err := h.rest.Authenticate(r)
	if err != nil {
		w.Header().Set("WWW-Authenticate", `Bearer realm="gist"`)
		w.Header().Set("Content-Type", "application/json")
		w.WriteHeader(http.StatusUnauthorized)
		_, _ = w.Write(marshal(map[string]any{"code": "unauthorized", "message": "Authentication required", "retryable": false}))
		return
	}
	if r.ContentLength > h.cfg.MaxBodyBytes {
		w.WriteHeader(http.StatusRequestEntityTooLarge)
		return
	}
	body, err := io.ReadAll(io.LimitReader(r.Body, h.cfg.MaxBodyBytes+1))
	if err != nil || int64(len(body)) > h.cfg.MaxBodyBytes {
		w.WriteHeader(http.StatusRequestEntityTooLarge)
		return
	}
	var req struct {
		JSONRPC string          `json:"jsonrpc"`
		ID      any             `json:"id"`
		Method  string          `json:"method"`
		Params  json.RawMessage `json:"params"`
	}
	if json.Unmarshal(body, &req) != nil {
		writeRPC(w, rpcErr(nil, -32600, "Invalid Request", nil))
		return
	}
	if req.Method == "initialize" {
		h.initialize(w, req, principalKey(principal), principal.WorkspaceID)
		return
	}
	if req.Method == "notifications/initialized" {
		w.WriteHeader(http.StatusAccepted)
		return
	}
	sid := r.Header.Get("Mcp-Session-Id")
	if req.Method == "tools/call" {
		// The session check and the in-flight increment happen under one
		// lock acquisition, so the session cannot be evicted or expire
		// between admission and the call.
		admitted, limited := h.admitCall(sid, principalKey(principal))
		if !admitted {
			writeRPC(w, rpcErr(req.ID, -32000, "Session required", nil))
			return
		}
		defer h.releaseCall(sid)
		h.call(w, r, req.ID, req.Params, limited)
		return
	}
	if !h.sessionOK(r, principalKey(principal)) {
		writeRPC(w, rpcErr(req.ID, -32000, "Session required", nil))
		return
	}
	switch req.Method {
	case "tools/list":
		writeRPC(w, rpcOK(req.ID, map[string]any{"tools": toolDefinitions()}))
	default:
		writeRPC(w, rpcErr(req.ID, -32601, "Method not found", nil))
	}
}

// principalKey identifies the caller a session belongs to.
func principalKey(p ports.Principal) string {
	return p.Issuer + "\x00" + p.Subject + "\x00" + p.WorkspaceID
}
func (h *Handler) originOK(r *http.Request) bool {
	o := r.Header.Get("Origin")
	return o == "" || h.cfg.AllowedOrigins[o]
}
func (h *Handler) initialize(w http.ResponseWriter, req struct {
	JSONRPC string          `json:"jsonrpc"`
	ID      any             `json:"id"`
	Method  string          `json:"method"`
	Params  json.RawMessage `json:"params"`
}, principal, workspace string) {
	var p struct {
		ProtocolVersion string `json:"protocolVersion"`
	}
	if json.Unmarshal(req.Params, &p) != nil || !negotiate(p.ProtocolVersion) {
		writeRPC(w, rpcErr(req.ID, -32602, "Unsupported protocol version", map[string]any{"supported": []string{SupportedProtocolVersion}}))
		return
	}
	id := newSession()
	h.mu.Lock()
	ok := h.addSessionLocked(id, principal, workspace)
	h.mu.Unlock()
	if !ok {
		writeRPC(w, rpcErr(req.ID, -32000, "Too many sessions", map[string]any{"retryable": true}))
		return
	}
	w.Header().Set("Mcp-Session-Id", id)
	writeRPC(w, rpcOK(req.ID, map[string]any{"protocolVersion": SupportedProtocolVersion, "capabilities": map[string]any{"tools": map[string]any{"listChanged": false}}, "serverInfo": map[string]any{"name": "gist-registry", "version": "1"}}))
}
func newSession() string {
	b := make([]byte, 16)
	_, _ = rand.Read(b)
	return "gist-" + url.PathEscape(string(b))
}

// sessionOK reports whether the request names a live session owned by
// principal, refreshing its recency.
func (h *Handler) sessionOK(r *http.Request, principal string) bool {
	v := r.Header.Get("Mcp-Session-Id")
	if v == "" {
		return false
	}
	h.mu.Lock()
	defer h.mu.Unlock()
	return h.touchLocked(v, principal)
}

// touchLocked reports whether id names a live session owned by principal,
// refreshing its recency. Callers hold h.mu.
func (h *Handler) touchLocked(id, principal string) bool {
	sess, ok := h.sessions[id]
	if !ok || sess.principal != principal {
		return false
	}
	now := h.now()
	if now.Sub(sess.last) > h.cfg.SessionTTL {
		if h.calls[id] == 0 {
			h.removeLocked(sess)
		}
		return false
	}
	sess.last = now
	h.lru.MoveToFront(sess.all)
	h.byPrincipal[principal].MoveToFront(sess.own)
	h.byWorkspace[sess.workspace].MoveToFront(sess.ws)
	return true
}

// admitCall validates the session and reserves an in-flight tools/call slot
// on it in a single critical section, so eviction and expiry, which skip
// sessions with calls in flight, cannot remove it before the call runs.
// admitted is false when the session is not live for principal; limited
// reports that the reservation exceeds MaxCalls. When admitted, the caller
// must call releaseCall.
func (h *Handler) admitCall(id, principal string) (admitted, limited bool) {
	if id == "" {
		return false, false
	}
	h.mu.Lock()
	defer h.mu.Unlock()
	if !h.touchLocked(id, principal) {
		return false, false
	}
	h.calls[id]++
	return true, h.calls[id] > h.cfg.MaxCalls
}

// releaseCall returns the in-flight slot admitCall reserved.
func (h *Handler) releaseCall(id string) {
	h.mu.Lock()
	if h.calls[id]--; h.calls[id] <= 0 {
		delete(h.calls, id)
	}
	h.mu.Unlock()
}

// addSessionLocked records a new session for principal. It first drops idle
// expired sessions from the cold end of the LRU, then makes room under the
// per-principal cap (evicting that principal's own LRU idle session), the
// per-workspace cap (evicting that workspace's own LRU idle session) and the
// machine-wide cap (evicting the global LRU idle session). A session with a
// tools/call in flight is never evicted; when no idle session can be evicted
// the new session is refused. Callers hold h.mu.
//
// Each step walks from the cold end and stops at the first evictable
// session, so the cost is O(1) amortized plus the number of sessions that
// have calls in flight, never a scan of the whole table.
func (h *Handler) addSessionLocked(id, principal, workspace string) bool {
	now := h.now()
	h.sweepExpiredLocked(now)
	if own := h.byPrincipal[principal]; own != nil && own.Len() >= h.cfg.MaxSessionsPerPrincipal {
		if !h.evictIdleLocked(own) {
			return false
		}
	}
	if ws := h.byWorkspace[workspace]; ws != nil && ws.Len() >= h.cfg.MaxSessionsPerWorkspace {
		if !h.evictIdleLocked(ws) {
			return false
		}
	}
	if len(h.sessions) >= h.cfg.MaxSessions {
		if !h.evictIdleLocked(h.lru) {
			return false
		}
	}
	own := h.byPrincipal[principal]
	if own == nil {
		own = list.New()
		h.byPrincipal[principal] = own
	}
	ws := h.byWorkspace[workspace]
	if ws == nil {
		ws = list.New()
		h.byWorkspace[workspace] = ws
	}
	sess := &session{id: id, principal: principal, workspace: workspace, last: now}
	sess.all = h.lru.PushFront(sess)
	sess.own = own.PushFront(sess)
	sess.ws = ws.PushFront(sess)
	h.sessions[id] = sess
	return true
}

// sweepExpiredLocked removes idle sessions past the TTL from the cold end of
// the LRU, stopping at the first live one. Callers hold h.mu.
func (h *Handler) sweepExpiredLocked(now time.Time) {
	for e := h.lru.Back(); e != nil; {
		sess, ok := e.Value.(*session)
		if !ok || sess == nil {
			return // Preserve indexes if the private LRU invariant is violated.
		}
		if now.Sub(sess.last) <= h.cfg.SessionTTL {
			return
		}
		prev := e.Prev()
		if h.calls[sess.id] == 0 {
			h.removeLocked(sess)
		}
		e = prev
	}
}

// evictIdleLocked removes the least recently used session in l that has no
// tools/call in flight. It reports false when every session in l is busy.
// Callers hold h.mu.
func (h *Handler) evictIdleLocked(l *list.List) bool {
	for e := l.Back(); e != nil; e = e.Prev() {
		sess, ok := e.Value.(*session)
		if !ok || sess == nil {
			return false // Never evict from a corrupted index.
		}
		if h.calls[sess.id] > 0 {
			continue
		}
		h.removeLocked(sess)
		return true
	}
	return false
}

// removeLocked drops sess from every index. Callers hold h.mu.
func (h *Handler) removeLocked(sess *session) {
	h.lru.Remove(sess.all)
	if own := h.byPrincipal[sess.principal]; own != nil {
		own.Remove(sess.own)
		if own.Len() == 0 {
			delete(h.byPrincipal, sess.principal)
		}
	}
	if ws := h.byWorkspace[sess.workspace]; ws != nil {
		ws.Remove(sess.ws)
		if ws.Len() == 0 {
			delete(h.byWorkspace, sess.workspace)
		}
	}
	delete(h.sessions, sess.id)
}

// call runs one admitted tools/call. calls counts in-flight tools/call
// requests per session, so MaxCalls caps concurrency, not lifetime volume:
// the slot admitCall reserved is released when the call returns.
func (h *Handler) call(w http.ResponseWriter, r *http.Request, id any, raw json.RawMessage, limited bool) {
	if limited {
		writeRPC(w, rpcOK(id, map[string]any{"isError": true, "content": []map[string]any{{"type": "text", "text": `{"code":"rate_limited","message":"Rate limit exceeded","retryable":true}`}}}))
		return
	}
	var p struct {
		Name      string          `json:"name"`
		Arguments json.RawMessage `json:"arguments"`
	}
	if json.Unmarshal(raw, &p) != nil {
		writeRPC(w, rpcErr(id, -32602, "Invalid tool arguments", nil))
		return
	}
	if !contains(toolNames, p.Name) {
		writeRPC(w, rpcErr(id, -32601, "Unknown tool", nil))
		return
	}
	var arguments map[string]any
	if json.Unmarshal(p.Arguments, &arguments) != nil {
		writeRPC(w, rpcErr(id, -32602, "Invalid tool arguments", nil))
		return
	}
	path, method, body := toolRoute(p.Name, arguments)
	if p.Name == "gist_resolve" {
		// The canonical resolver must see duplicate fields and exact JSON types.
		body = p.Arguments
	}
	if path == "" {
		writeRPC(w, rpcErr(id, -32602, "Invalid tool arguments", nil))
		return
	}
	rr := httptestRequest(method, path, body, r)
	rw := newResponse()
	h.rest.ServeHTTP(rw, rr)
	if rw.status == http.StatusUnauthorized || rw.status == http.StatusForbidden || rw.status == http.StatusNotFound || rw.status == http.StatusConflict || rw.status == http.StatusRequestEntityTooLarge || rw.status == http.StatusUnprocessableEntity || rw.status == http.StatusTooManyRequests || rw.status == http.StatusServiceUnavailable {
		writeRPC(w, rpcOK(id, map[string]any{"isError": true, "content": []map[string]any{{"type": "text", "text": string(rw.body)}}}))
		return
	}
	writeRPC(w, rpcOK(id, map[string]any{"isError": false, "content": []map[string]any{{"type": "text", "text": string(rw.body)}}, "structuredContent": json.RawMessage(rw.body)}))
}
func contains(xs []string, v string) bool {
	for _, x := range xs {
		if x == v {
			return true
		}
	}
	return false
}
func writeRPC(w http.ResponseWriter, v rpcResponse) {
	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(http.StatusOK)
	_ = json.NewEncoder(w).Encode(v)
}

type capture struct {
	status int
	body   []byte
	header http.Header
}

func newResponse() *capture            { return &capture{header: make(http.Header)} }
func (c *capture) Header() http.Header { return c.header }
func (c *capture) WriteHeader(s int)   { c.status = s }
func (c *capture) Write(b []byte) (int, error) {
	c.body = append(c.body, b...)
	if c.status == 0 {
		c.status = 200
	}
	return len(b), nil
}
func httptestRequest(method, path string, body []byte, src *http.Request) *http.Request {
	rr, _ := http.NewRequestWithContext(src.Context(), method, path, bytes.NewReader(body))
	rr.Header = src.Header.Clone()
	return rr
}

var _ = errors.New
var _ ports.Principal

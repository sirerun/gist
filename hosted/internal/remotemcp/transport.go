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
}

const (
	defaultSessionTTL  = time.Hour
	defaultMaxSessions = 10000
	// defaultMaxSessionsPerPrincipal bounds one caller's share of the table.
	defaultMaxSessionsPerPrincipal = 32
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
	calls       map[string]int // in-flight tools/call count per session
	now         func() time.Time
}

// session is one Mcp-Session-Id bound to the principal that created it.
type session struct {
	id        string
	principal string
	last      time.Time
	all       *list.Element // element in Handler.lru
	own       *list.Element // element in Handler.byPrincipal[principal]
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
	if c.MaxSessionsPerPrincipal > c.MaxSessions {
		c.MaxSessionsPerPrincipal = c.MaxSessions
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
		calls:       map[string]int{},
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
		h.initialize(w, req, principalKey(principal))
		return
	}
	if req.Method == "notifications/initialized" {
		w.WriteHeader(http.StatusAccepted)
		return
	}
	if !h.sessionOK(r, principalKey(principal)) {
		writeRPC(w, rpcErr(req.ID, -32000, "Session required", nil))
		return
	}
	switch req.Method {
	case "tools/list":
		writeRPC(w, rpcOK(req.ID, map[string]any{"tools": toolDefinitions()}))
	case "tools/call":
		h.call(w, r, req.ID, req.Params)
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
}, principal string) {
	var p struct {
		ProtocolVersion string `json:"protocolVersion"`
	}
	if json.Unmarshal(req.Params, &p) != nil || !negotiate(p.ProtocolVersion) {
		writeRPC(w, rpcErr(req.ID, -32602, "Unsupported protocol version", map[string]any{"supported": []string{SupportedProtocolVersion}}))
		return
	}
	id := newSession()
	h.mu.Lock()
	ok := h.addSessionLocked(id, principal)
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
	sess, ok := h.sessions[v]
	if !ok || sess.principal != principal {
		return false
	}
	now := h.now()
	if now.Sub(sess.last) > h.cfg.SessionTTL {
		if h.calls[v] == 0 {
			h.removeLocked(sess)
		}
		return false
	}
	sess.last = now
	h.lru.MoveToFront(sess.all)
	h.byPrincipal[principal].MoveToFront(sess.own)
	return true
}

// addSessionLocked records a new session for principal. It first drops idle
// expired sessions from the cold end of the LRU, then makes room under the
// per-principal cap (evicting that principal's own LRU idle session) and the
// machine-wide cap (evicting the global LRU idle session). A session with a
// tools/call in flight is never evicted; when no idle session can be evicted
// the new session is refused. Callers hold h.mu.
//
// Each step walks from the cold end and stops at the first evictable
// session, so the cost is O(1) amortized plus the number of sessions that
// have calls in flight, never a scan of the whole table.
func (h *Handler) addSessionLocked(id, principal string) bool {
	now := h.now()
	h.sweepExpiredLocked(now)
	if own := h.byPrincipal[principal]; own != nil && own.Len() >= h.cfg.MaxSessionsPerPrincipal {
		if !h.evictIdleLocked(own) {
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
	sess := &session{id: id, principal: principal, last: now}
	sess.all = h.lru.PushFront(sess)
	sess.own = own.PushFront(sess)
	h.sessions[id] = sess
	return true
}

// sweepExpiredLocked removes idle sessions past the TTL from the cold end of
// the LRU, stopping at the first live one. Callers hold h.mu.
func (h *Handler) sweepExpiredLocked(now time.Time) {
	for e := h.lru.Back(); e != nil; {
		sess := e.Value.(*session)
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
		sess := e.Value.(*session)
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
	delete(h.sessions, sess.id)
}

func (h *Handler) call(w http.ResponseWriter, r *http.Request, id any, raw json.RawMessage) {
	sid := r.Header.Get("Mcp-Session-Id")
	// calls counts in-flight tools/call requests per session, so MaxCalls caps
	// concurrency, not lifetime volume: the slot is released when the call returns.
	h.mu.Lock()
	h.calls[sid]++
	limited := h.calls[sid] > h.cfg.MaxCalls
	h.mu.Unlock()
	defer func() {
		h.mu.Lock()
		if h.calls[sid]--; h.calls[sid] <= 0 {
			delete(h.calls, sid)
		}
		h.mu.Unlock()
	}()
	if limited {
		writeRPC(w, rpcOK(id, map[string]any{"isError": true, "content": []map[string]any{{"type": "text", "text": `{"code":"rate_limited","message":"Rate limit exceeded","retryable":true}`}}}))
		return
	}
	var p struct {
		Name      string         `json:"name"`
		Arguments map[string]any `json:"arguments"`
	}
	if json.Unmarshal(raw, &p) != nil {
		writeRPC(w, rpcErr(id, -32602, "Invalid tool arguments", nil))
		return
	}
	if !contains(toolNames, p.Name) {
		writeRPC(w, rpcErr(id, -32601, "Unknown tool", nil))
		return
	}
	path, method, body := toolRoute(p.Name, p.Arguments)
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

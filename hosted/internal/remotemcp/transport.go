package remotemcp

import (
	"bytes"
	"crypto/rand"
	"encoding/json"
	"errors"
	"io"
	"net/http"
	"net/url"
	"strings"
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
	// MaxSessions caps live sessions; the least recently used is evicted.
	MaxSessions int
}

const (
	defaultSessionTTL  = time.Hour
	defaultMaxSessions = 10000
)

type Handler struct {
	rest     *rest.Handler
	cfg      Config
	mu       sync.Mutex
	sessions map[string]time.Time // session id -> last use
	calls    map[string]int       // in-flight tools/call count per session
	now      func() time.Time
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
	rh, err := rest.New(c.Services)
	if err != nil {
		return nil, err
	}
	return &Handler{rest: rh, cfg: c, sessions: map[string]time.Time{}, calls: map[string]int{}, now: time.Now}, nil
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
	if !bearer(r.Header.Get("Authorization")) {
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
		h.initialize(w, r, req)
		return
	}
	if req.Method == "notifications/initialized" {
		w.WriteHeader(http.StatusAccepted)
		return
	}
	if !h.sessionOK(r) {
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
func bearer(v string) bool {
	return strings.HasPrefix(v, "Bearer ") && len(strings.TrimSpace(strings.TrimPrefix(v, "Bearer "))) > 0
}
func (h *Handler) originOK(r *http.Request) bool {
	o := r.Header.Get("Origin")
	return o == "" || h.cfg.AllowedOrigins[o]
}
func (h *Handler) initialize(w http.ResponseWriter, r *http.Request, req struct {
	JSONRPC string          `json:"jsonrpc"`
	ID      any             `json:"id"`
	Method  string          `json:"method"`
	Params  json.RawMessage `json:"params"`
}) {
	var p struct {
		ProtocolVersion string `json:"protocolVersion"`
	}
	if json.Unmarshal(req.Params, &p) != nil || !negotiate(p.ProtocolVersion) {
		writeRPC(w, rpcErr(req.ID, -32602, "Unsupported protocol version", map[string]any{"supported": []string{SupportedProtocolVersion}}))
		return
	}
	id := newSession()
	h.mu.Lock()
	h.addSessionLocked(id)
	h.mu.Unlock()
	w.Header().Set("Mcp-Session-Id", id)
	writeRPC(w, rpcOK(req.ID, map[string]any{"protocolVersion": SupportedProtocolVersion, "capabilities": map[string]any{"tools": map[string]any{"listChanged": false}}, "serverInfo": map[string]any{"name": "gist-registry", "version": "1"}}))
}
func newSession() string {
	b := make([]byte, 16)
	_, _ = rand.Read(b)
	return "gist-" + url.PathEscape(string(b))
}
func (h *Handler) sessionOK(r *http.Request) bool {
	v := r.Header.Get("Mcp-Session-Id")
	h.mu.Lock()
	defer h.mu.Unlock()
	last, ok := h.sessions[v]
	if v == "" || !ok {
		return false
	}
	now := h.now()
	if now.Sub(last) > h.cfg.SessionTTL {
		delete(h.sessions, v)
		return false
	}
	h.sessions[v] = now
	return true
}

// addSessionLocked records a new session, first dropping idle sessions and
// then, if still at capacity, the least recently used one. Callers hold h.mu.
func (h *Handler) addSessionLocked(id string) {
	now := h.now()
	if len(h.sessions) >= h.cfg.MaxSessions {
		oldestID, oldest := "", now
		for sid, last := range h.sessions {
			if now.Sub(last) > h.cfg.SessionTTL {
				delete(h.sessions, sid)
				continue
			}
			if oldestID == "" || last.Before(oldest) {
				oldestID, oldest = sid, last
			}
		}
		if len(h.sessions) >= h.cfg.MaxSessions && oldestID != "" {
			delete(h.sessions, oldestID)
		}
	}
	h.sessions[id] = now
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

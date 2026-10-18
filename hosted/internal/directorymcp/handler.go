// Package directorymcp exposes only public discovery metadata. It cannot connect
// to a server, execute a tool, or see workspace records and credentials.
package directorymcp

import (
	"bytes"
	"encoding/json"
	"io"
	"mime"
	"net/http"
	"strings"

	"github.com/sirerun/gist/catalog/directory"
)

const ProtocolVersion = "2025-06-18"

type Handler struct {
	index      *directory.Index
	origin     string
	concurrent chan struct{}
}

func New(index *directory.Index, origin string, maxConcurrent int) *Handler {
	if maxConcurrent < 1 {
		maxConcurrent = 16
	}
	return &Handler{index: index, origin: origin, concurrent: make(chan struct{}, maxConcurrent)}
}

type request struct {
	JSONRPC string          `json:"jsonrpc"`
	ID      json.RawMessage `json:"id,omitempty"`
	Method  string          `json:"method"`
	Params  json.RawMessage `json:"params,omitempty"`
}

func (h *Handler) ServeHTTP(w http.ResponseWriter, r *http.Request) {
	w.Header().Set("Content-Type", "application/json")
	w.Header().Set("Cache-Control", "no-store")
	w.Header().Set("X-Content-Type-Options", "nosniff")
	w.Header().Set("Vary", "Origin")
	if r.Header.Get("Origin") != "" && r.Header.Get("Origin") != h.origin {
		http.Error(w, "Origin denied", 403)
		return
	}
	if r.URL.RawQuery != "" {
		http.Error(w, "Query parameters are not accepted", 400)
		return
	}
	if r.Method != "POST" {
		w.Header().Set("Allow", "POST")
		w.WriteHeader(405)
		return
	}
	select {
	case h.concurrent <- struct{}{}:
		defer func() { <-h.concurrent }()
	default:
		w.Header().Set("Retry-After", "1")
		w.WriteHeader(429)
		return
	}
	media, _, err := mime.ParseMediaType(r.Header.Get("Content-Type"))
	if err != nil || media != "application/json" {
		w.WriteHeader(415)
		return
	}
	if !accepts(r.Header.Get("Accept"), "application/json") || !accepts(r.Header.Get("Accept"), "text/event-stream") {
		w.WriteHeader(406)
		return
	}
	if v := r.Header.Get("MCP-Protocol-Version"); v != "" && v != "2025-03-26" && v != ProtocolVersion {
		w.WriteHeader(400)
		return
	}
	raw, err := io.ReadAll(http.MaxBytesReader(w, r.Body, 32<<10))
	if err != nil {
		w.WriteHeader(413)
		return
	}
	if !json.Valid(raw) {
		h.fail(w, nil, -32700, "Parse error")
		return
	}
	var call request
	dec := json.NewDecoder(bytes.NewReader(raw))
	dec.DisallowUnknownFields()
	if dec.Decode(&call) != nil || call.JSONRPC != "2.0" || call.Method == "" || (call.ID != nil && !validID(call.ID)) {
		h.fail(w, nil, -32600, "Invalid request")
		return
	}
	if call.ID == nil {
		if strings.HasPrefix(call.Method, "notifications/") {
			w.WriteHeader(202)
			return
		}
		h.fail(w, nil, -32600, "Request ID required")
		return
	}
	switch call.Method {
	case "initialize":
		var params struct {
			ProtocolVersion string         `json:"protocolVersion"`
			Capabilities    map[string]any `json:"capabilities"`
			ClientInfo      struct {
				Name    string `json:"name"`
				Version string `json:"version"`
			} `json:"clientInfo"`
		}
		if json.Unmarshal(call.Params, &params) != nil || params.ProtocolVersion == "" || params.ClientInfo.Name == "" || params.ClientInfo.Version == "" {
			h.fail(w, call.ID, -32602, "Invalid initialize parameters")
			return
		}
		version := ProtocolVersion
		if params.ProtocolVersion == "2025-03-26" {
			version = params.ProtocolVersion
		}
		h.result(w, call.ID, map[string]any{"protocolVersion": version, "capabilities": map[string]any{"tools": map[string]any{"listChanged": false}}, "serverInfo": map[string]string{"name": "gist-public-directory", "version": "1.0.0"}, "instructions": "Find MCP servers and OpenAPI integrations. Connect and authorize through your client; review each provider's requested access before use."})
	case "ping":
		h.result(w, call.ID, map[string]any{})
	case "tools/list":
		if !emptyObject(call.Params) {
			h.fail(w, call.ID, -32602, "Pagination is not available for this fixed tool list")
			return
		}
		h.result(w, call.ID, map[string]any{"tools": toolList()})
	case "tools/call":
		h.call(w, call)
	default:
		h.fail(w, call.ID, -32601, "Method not found")
	}
}
func (h *Handler) call(w http.ResponseWriter, call request) {
	var params struct {
		Name      string          `json:"name"`
		Arguments json.RawMessage `json:"arguments"`
		Meta      json.RawMessage `json:"_meta,omitempty"`
	}
	dec := json.NewDecoder(bytes.NewReader(call.Params))
	dec.DisallowUnknownFields()
	if dec.Decode(&params) != nil || params.Name == "" {
		h.fail(w, call.ID, -32602, "Invalid tool call")
		return
	}
	var value any
	switch params.Name {
	case "gist_directory_search":
		var q struct {
			Query  string `json:"query"`
			Kind   string `json:"kind"`
			Tag    string `json:"tag"`
			Cursor string `json:"cursor"`
			Limit  int    `json:"limit"`
		}
		if !decodeArgs(params.Arguments, &q) {
			h.fail(w, call.ID, -32602, "Invalid search arguments")
			return
		}
		if q.Limit < 0 || q.Limit > 50 {
			h.fail(w, call.ID, -32602, "Limit must be 1 to 50")
			return
		}
		if q.Limit == 0 {
			q.Limit = 24
		}
		var err error
		var result directory.Result
		result, err = h.index.Search(directory.Query{Text: q.Query, Kind: q.Kind, Tag: q.Tag, Cursor: q.Cursor, Limit: q.Limit})
		if err != nil {
			h.fail(w, call.ID, -32602, "Invalid search or expired cursor")
			return
		}
		value = directory.PublicResultOf(result)
	case "gist_directory_get", "gist_directory_export":
		var args struct {
			ID string `json:"id"`
		}
		if !decodeArgs(params.Arguments, &args) || args.ID == "" || len(args.ID) > 512 {
			h.fail(w, call.ID, -32602, "Provide an entry ID")
			return
		}
		record, ok := h.index.Get(args.ID)
		if !ok {
			h.toolError(w, call.ID, "Directory entry not found")
			return
		}
		value = directory.PublicRecordOf(record)
		if params.Name == "gist_directory_export" {
			value = directory.PublicExportOf(directory.Export{Format: directory.ExportFormat, Record: record})
		}
	default:
		h.fail(w, call.ID, -32602, "Unknown tool")
		return
	}
	raw, err := json.Marshal(value)
	if err != nil || len(raw) > 480<<10 {
		h.toolError(w, call.ID, "Result too large; use a smaller page")
		return
	}
	h.result(w, call.ID, map[string]any{"content": []map[string]string{{"type": "text", "text": string(raw)}}, "structuredContent": value, "isError": false})
}
func decodeArgs(raw []byte, out any) bool {
	if len(raw) == 0 {
		raw = []byte("{}")
	}
	d := json.NewDecoder(bytes.NewReader(raw))
	d.DisallowUnknownFields()
	return d.Decode(out) == nil && d.Decode(new(any)) == io.EOF
}
func emptyObject(raw []byte) bool {
	if len(raw) == 0 {
		return true
	}
	var v map[string]any
	return json.Unmarshal(raw, &v) == nil && v != nil && len(v) == 0
}
func validID(raw []byte) bool {
	var x any
	if json.Unmarshal(raw, &x) != nil {
		return false
	}
	switch x.(type) {
	case string, float64:
		return true
	}
	return false
}
func accepts(header, want string) bool {
	for _, entry := range strings.Split(header, ",") {
		media, params, err := mime.ParseMediaType(strings.TrimSpace(entry))
		if err == nil && media == want && params["q"] != "0" {
			return true
		}
	}
	return false
}
func (h *Handler) fail(w http.ResponseWriter, id json.RawMessage, code int, message string) {
	if id == nil {
		id = json.RawMessage("null")
	}
	h.write(w, map[string]any{"jsonrpc": "2.0", "id": id, "error": map[string]any{"code": code, "message": message}})
}
func (h *Handler) result(w http.ResponseWriter, id json.RawMessage, value any) {
	h.write(w, map[string]any{"jsonrpc": "2.0", "id": id, "result": value})
}
func (h *Handler) toolError(w http.ResponseWriter, id json.RawMessage, message string) {
	h.result(w, id, map[string]any{"isError": true, "content": []map[string]string{{"type": "text", "text": message}}})
}
func (h *Handler) write(w http.ResponseWriter, value any) {
	raw, err := json.Marshal(value)
	if err != nil || len(raw) > 1<<20 {
		w.WriteHeader(503)
		_, _ = w.Write([]byte(`{"jsonrpc":"2.0","id":null,"error":{"code":-32603,"message":"Response unavailable"}}`))
		return
	}
	_, _ = w.Write(raw)
}
func toolList() []map[string]any {
	annotations := map[string]any{"readOnlyHint": true, "destructiveHint": false, "idempotentHint": true, "openWorldHint": false}
	schema := func(properties map[string]any, required []string) map[string]any {
		return map[string]any{"type": "object", "properties": properties, "required": required, "additionalProperties": false}
	}
	id := map[string]any{"type": "string", "minLength": 1, "maxLength": 512, "description": "Exact ID returned by gist_directory_search."}
	return []map[string]any{
		{"name": "gist_directory_search", "description": "Find MCP server and OpenAPI integrations by name, use case or connection type. No connection or execution.", "annotations": annotations, "inputSchema": schema(map[string]any{"query": map[string]any{"type": "string", "maxLength": 200}, "kind": map[string]any{"type": "string", "enum": []string{"", "mcp", "openapi"}}, "tag": map[string]any{"type": "string", "maxLength": 64}, "cursor": map[string]any{"type": "string", "maxLength": 2048}, "limit": map[string]any{"type": "integer", "minimum": 1, "maximum": 50}}, []string{})},
		{"name": "gist_directory_get", "description": "Read a public integration's description, connection details and setup links by exact ID.", "annotations": annotations, "inputSchema": schema(map[string]any{"id": id}, []string{"id"})},
		{"name": "gist_directory_export", "description": "Export a metadata-only gist.directory.entry/v1 descriptor for review by Zati or another client. Does not connect, authorize or run it.", "annotations": annotations, "inputSchema": schema(map[string]any{"id": id}, []string{"id"})},
	}
}

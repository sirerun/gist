// Package directorymcp exposes only public discovery metadata. It cannot connect
// to a server, execute a tool, or see workspace records and credentials.
package directorymcp

import (
	"bytes"
	"context"
	"encoding/json"
	"io"
	"mime"
	"net/http"
	"strings"
	"sync"

	"github.com/sirerun/gist/catalog/directory"
)

const ProtocolVersion = "2025-06-18"

type Searcher interface {
	Search(context.Context, directory.Query) (directory.Result, error)
}

type AppSearcher interface {
	SearchApps(context.Context, directory.AppQuery) (directory.AppResult, error)
}

type Handler struct {
	search     Searcher
	index      *directory.Index
	appsOnce   sync.Once
	apps       *directory.AppIndex
	appsErr    error
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

// WithSearch uses the same hosted retrieval service as the Library HTTP API.
func (h *Handler) WithSearch(s Searcher) *Handler { h.search = s; return h }

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
		h.call(r.Context(), w, call)
	default:
		h.fail(w, call.ID, -32601, "Method not found")
	}
}
func (h *Handler) call(ctx context.Context, w http.ResponseWriter, call request) {
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
	case "gist_directory_apps":
		var args struct {
			Query    string `json:"query"`
			Category string `json:"category"`
			Offset   int    `json:"offset"`
			Limit    *int   `json:"limit"`
		}
		if !decodeArgs(params.Arguments, &args) {
			h.fail(w, call.ID, -32602, "Invalid app search arguments")
			return
		}
		limit := 24
		if args.Limit != nil {
			limit = *args.Limit
		}
		if limit < 1 || limit > 100 || args.Offset < 0 || args.Offset > directory.MaxEntries || len(args.Query) > 512 || len(args.Category) > 200 {
			h.fail(w, call.ID, -32602, "Invalid app search bounds")
			return
		}
		query := directory.AppQuery{Text: args.Query, Category: args.Category, Offset: args.Offset, Limit: limit}
		var result directory.AppResult
		var err error
		if search, ok := h.search.(AppSearcher); ok {
			result, err = search.SearchApps(ctx, query)
		} else {
			h.appsOnce.Do(func() { h.apps, h.appsErr = directory.DefaultApps(h.index) })
			if h.appsErr != nil {
				h.toolError(w, call.ID, "App directory unavailable")
				return
			}
			result, err = h.apps.Search(query)
		}
		if err != nil {
			h.fail(w, call.ID, -32602, "Invalid app query")
			return
		}
		if result.SearchMode != "" {
			w.Header().Set("X-Gist-Search-Mode", result.SearchMode)
		}
		value = result
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
		query := directory.Query{Text: q.Query, Kind: q.Kind, Tag: q.Tag, Cursor: q.Cursor, Limit: q.Limit}
		if h.search != nil {
			result, err = h.search.Search(ctx, query)
		} else {
			result, err = h.index.Search(query)
		}
		if err != nil {
			h.fail(w, call.ID, -32602, "Invalid search or expired cursor")
			return
		}
		if result.SearchMode != "" {
			w.Header().Set("X-Gist-Search-Mode", result.SearchMode)
		}
		value = directory.PublicResultOf(result)
	case "gist_directory_authorization":
		var args struct {
			ID      string          `json:"id"`
			App     string          `json:"app"`
			Version json.RawMessage `json:"version"`
		}
		if !decodeArgs(params.Arguments, &args) || (args.ID == "") == (args.App == "") || len(args.ID) > 512 || len(args.App) > 256 {
			h.fail(w, call.ID, -32602, "Choose exactly one connector ID or app slug")
			return
		}
		version := 1
		if len(args.Version) > 0 {
			if bytes.Equal(bytes.TrimSpace(args.Version), []byte("null")) || json.Unmarshal(args.Version, &version) != nil || (version != 1 && version != 2) {
				h.fail(w, call.ID, -32602, "Version must be 1 or 2")
				return
			}
		}
		var err error
		if args.App != "" {
			h.appsOnce.Do(func() { h.apps, h.appsErr = directory.DefaultApps(h.index) })
			if h.appsErr != nil {
				h.toolError(w, call.ID, "App directory unavailable")
				return
			}
			a, ok := h.apps.Get(args.App)
			if !ok {
				h.toolError(w, call.ID, "App not found")
				return
			}
			if version == 2 {
				value, err = directory.AuthorizationForAppV2(a, h.index)
			} else {
				value, err = directory.AuthorizationForApp(a, h.index)
			}
		} else {
			r, ok := h.index.Get(args.ID)
			if !ok {
				h.toolError(w, call.ID, "Directory entry not found")
				return
			}
			value, err = directory.AuthorizationFor(r)
		}
		if err != nil {
			h.toolError(w, call.ID, "Authorization guidance unavailable")
			return
		}
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
		{"name": "gist_directory_apps", "description": "Search the complete app inventory using the same hosted search as the Library. Follow an exact returned slug with gist_directory_authorization using app and version 2 for reviewed MCP/direct API routes and explicit gaps. Listings do not grant connection or execution.", "annotations": annotations, "inputSchema": schema(map[string]any{"query": map[string]any{"type": "string", "maxLength": 512}, "category": map[string]any{"type": "string", "maxLength": 200}, "offset": map[string]any{"type": "integer", "minimum": 0, "maximum": directory.MaxEntries}, "limit": map[string]any{"type": "integer", "minimum": 1, "maximum": 100}}, []string{})},
		{"name": "gist_directory_search", "description": "Search admitted MCP server and OpenAPI connection descriptors by name, use case or type. For the complete app inventory use gist_directory_apps, then request authorization guidance by app slug. No connection or execution.", "annotations": annotations, "inputSchema": schema(map[string]any{"query": map[string]any{"type": "string", "maxLength": 200}, "kind": map[string]any{"type": "string", "enum": []string{"", "mcp", "openapi"}}, "tag": map[string]any{"type": "string", "maxLength": 64}, "cursor": map[string]any{"type": "string", "maxLength": 2048}, "limit": map[string]any{"type": "integer", "minimum": 1, "maximum": 50}}, []string{})},
		{"name": "gist_directory_get", "description": "Read a public integration's description, connection details and setup links by exact ID.", "annotations": annotations, "inputSchema": schema(map[string]any{"id": id}, []string{"id"})},
		{"name": "gist_directory_authorization", "description": "Read direct-provider authorization guidance by exact connector ID or app slug. Covers all catalog apps with explicit gaps; never invents MCP routes. Provide exactly one of id or app. Use version 2 for tenant/private endpoint inputs and native protocols; version 1 preserves strict clients. Clients own credentials and authorization.", "annotations": annotations, "inputSchema": map[string]any{"type": "object", "properties": map[string]any{"id": id, "app": map[string]any{"type": "string", "minLength": 1, "maxLength": 256, "description": "Exact app slug returned by gist_directory_apps or REST app search."}, "version": map[string]any{"type": "integer", "enum": []int{1, 2}, "default": 1}}, "oneOf": []map[string]any{{"required": []string{"id"}}, {"required": []string{"app"}}}, "additionalProperties": false}},
		{"name": "gist_directory_export", "description": "Export a metadata-only gist.directory.entry/v1 descriptor for review by Zati or another client. Does not connect, authorize or run it.", "annotations": annotations, "inputSchema": schema(map[string]any{"id": id}, []string{"id"})},
	}
}

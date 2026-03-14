// Package mcp implements an MCP (Model Context Protocol) server that exposes
// Gist functionality as tools over JSON-RPC 2.0 on stdio.
package mcp

import (
	"bufio"
	"context"
	"encoding/json"
	"fmt"
	"io"
	"os"

	"github.com/sirerun/gist"
)

// Server wraps a Gist instance and serves MCP protocol messages over stdio.
type Server struct {
	g      *gist.Gist
	reader io.Reader
	writer io.Writer
}

// NewServer creates a new MCP server backed by the given Gist instance.
func NewServer(g *gist.Gist) *Server {
	return &Server{
		g:      g,
		reader: os.Stdin,
		writer: os.Stdout,
	}
}

// jsonrpcRequest represents a JSON-RPC 2.0 request.
type jsonrpcRequest struct {
	JSONRPC string          `json:"jsonrpc"`
	ID      json.RawMessage `json:"id,omitempty"`
	Method  string          `json:"method"`
	Params  json.RawMessage `json:"params,omitempty"`
}

// jsonrpcResponse represents a JSON-RPC 2.0 response.
type jsonrpcResponse struct {
	JSONRPC string          `json:"jsonrpc"`
	ID      json.RawMessage `json:"id,omitempty"`
	Result  any             `json:"result,omitempty"`
	Error   *jsonrpcError   `json:"error,omitempty"`
}

// jsonrpcError represents a JSON-RPC 2.0 error object.
type jsonrpcError struct {
	Code    int    `json:"code"`
	Message string `json:"message"`
}

// Standard JSON-RPC error codes.
const (
	codeParseError     = -32700
	codeMethodNotFound = -32601
	codeInvalidParams  = -32602
)

// MCP capability and server info types.
type serverInfo struct {
	Name    string `json:"name"`
	Version string `json:"version"`
}

type capabilities struct {
	Tools *toolsCap `json:"tools,omitempty"`
}

type toolsCap struct{}

type initializeResult struct {
	ProtocolVersion string       `json:"protocolVersion"`
	ServerInfo      serverInfo   `json:"serverInfo"`
	Capabilities    capabilities `json:"capabilities"`
}

// Serve reads JSON-RPC 2.0 messages from stdin line by line and writes
// responses to stdout. It blocks until ctx is cancelled or stdin is closed.
func (s *Server) Serve(ctx context.Context) error {
	scanner := bufio.NewScanner(s.reader)
	// Allow up to 10MB per line for large content payloads.
	scanner.Buffer(make([]byte, 0, 64*1024), 10*1024*1024)

	for scanner.Scan() {
		select {
		case <-ctx.Done():
			return ctx.Err()
		default:
		}

		line := scanner.Bytes()
		if len(line) == 0 {
			continue
		}

		var req jsonrpcRequest
		if err := json.Unmarshal(line, &req); err != nil {
			s.writeResponse(jsonrpcResponse{
				JSONRPC: "2.0",
				Error:   &jsonrpcError{Code: codeParseError, Message: "parse error"},
			})
			continue
		}

		resp := s.handleRequest(ctx, &req)
		if resp != nil {
			s.writeResponse(*resp)
		}
	}

	return scanner.Err()
}

// handleRequest dispatches a JSON-RPC request to the appropriate handler.
// Returns nil for notifications (requests without an ID).
func (s *Server) handleRequest(ctx context.Context, req *jsonrpcRequest) *jsonrpcResponse {
	isNotification := len(req.ID) == 0 || string(req.ID) == "null"

	switch req.Method {
	case "initialize":
		return s.handleInitialize(req)

	case "initialized":
		// Notification — no response.
		return nil

	case "tools/list":
		return s.handleToolsList(req)

	case "tools/call":
		return s.handleToolsCall(ctx, req)

	default:
		if isNotification {
			return nil
		}
		return &jsonrpcResponse{
			JSONRPC: "2.0",
			ID:      req.ID,
			Error:   &jsonrpcError{Code: codeMethodNotFound, Message: fmt.Sprintf("method not found: %s", req.Method)},
		}
	}
}

// handleInitialize responds with server info and capabilities.
func (s *Server) handleInitialize(req *jsonrpcRequest) *jsonrpcResponse {
	return &jsonrpcResponse{
		JSONRPC: "2.0",
		ID:      req.ID,
		Result: initializeResult{
			ProtocolVersion: "2024-11-05",
			ServerInfo: serverInfo{
				Name:    "gist",
				Version: "0.1.0",
			},
			Capabilities: capabilities{
				Tools: &toolsCap{},
			},
		},
	}
}

// handleToolsList returns the list of available tools.
func (s *Server) handleToolsList(req *jsonrpcRequest) *jsonrpcResponse {
	return &jsonrpcResponse{
		JSONRPC: "2.0",
		ID:      req.ID,
		Result: map[string]any{
			"tools": ToolDefinitions(),
		},
	}
}

// toolsCallParams represents the params for a tools/call request.
type toolsCallParams struct {
	Name      string          `json:"name"`
	Arguments json.RawMessage `json:"arguments"`
}

// mcpContent represents an MCP content block.
type mcpContent struct {
	Type string `json:"type"`
	Text string `json:"text"`
}

// mcpToolResult represents an MCP tool call result.
type mcpToolResult struct {
	Content []mcpContent `json:"content"`
	IsError bool         `json:"isError,omitempty"`
}

// handleToolsCall dispatches to tool handlers.
func (s *Server) handleToolsCall(ctx context.Context, req *jsonrpcRequest) *jsonrpcResponse {
	var params toolsCallParams
	if err := json.Unmarshal(req.Params, &params); err != nil {
		return &jsonrpcResponse{
			JSONRPC: "2.0",
			ID:      req.ID,
			Error:   &jsonrpcError{Code: codeInvalidParams, Message: "invalid params"},
		}
	}

	result, err := dispatchTool(ctx, s.g, params.Name, params.Arguments)
	if err != nil {
		errText, _ := json.Marshal(map[string]string{"error": err.Error()})
		return &jsonrpcResponse{
			JSONRPC: "2.0",
			ID:      req.ID,
			Result: mcpToolResult{
				Content: []mcpContent{{Type: "text", Text: string(errText)}},
				IsError: true,
			},
		}
	}

	resultJSON, err := json.Marshal(result)
	if err != nil {
		return &jsonrpcResponse{
			JSONRPC: "2.0",
			ID:      req.ID,
			Error:   &jsonrpcError{Code: -32603, Message: "internal error"},
		}
	}

	return &jsonrpcResponse{
		JSONRPC: "2.0",
		ID:      req.ID,
		Result: mcpToolResult{
			Content: []mcpContent{{Type: "text", Text: string(resultJSON)}},
		},
	}
}

// writeResponse marshals and writes a JSON-RPC response as a single line.
func (s *Server) writeResponse(resp jsonrpcResponse) {
	data, err := json.Marshal(resp)
	if err != nil {
		return
	}
	data = append(data, '\n')
	s.writer.Write(data)
}

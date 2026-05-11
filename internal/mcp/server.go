// Package mcp implements the MCP (Model Context Protocol) JSON-RPC 2.0 server over stdio.
package mcp

import (
	"encoding/json"
	"fmt"
	"log"
)

// JSON-RPC 2.0 types

type Request struct {
	JSONRPC string          `json:"jsonrpc"`
	ID      interface{}     `json:"id"`
	Method  string          `json:"method"`
	Params  json.RawMessage `json:"params,omitempty"`
}

type Response struct {
	JSONRPC string      `json:"jsonrpc"`
	ID      interface{} `json:"id"`
	Result  interface{} `json:"result,omitempty"`
	Error   *RPCError   `json:"error,omitempty"`
}

type RPCError struct {
	Code    int    `json:"code"`
	Message string `json:"message"`
}

type Notification struct {
	JSONRPC string      `json:"jsonrpc"`
	Method  string      `json:"method"`
	Params  interface{} `json:"params,omitempty"`
}

// MCP protocol types

type ServerInfo struct {
	Name    string `json:"name"`
	Version string `json:"version"`
}

type InitializeResult struct {
	ProtocolVersion string     `json:"protocolVersion"`
	ServerInfo      ServerInfo `json:"serverInfo"`
	Capabilities    Caps       `json:"capabilities"`
}

type Caps struct {
	Tools *ToolsCap `json:"tools,omitempty"`
}

type ToolsCap struct{}

type ToolsListResult struct {
	Tools []ToolDef `json:"tools"`
}

type ToolDef struct {
	Name        string      `json:"name"`
	Description string      `json:"description"`
	InputSchema InputSchema `json:"inputSchema"`
}

type InputSchema struct {
	Type       string              `json:"type"`
	Properties map[string]Property `json:"properties"`
	Required   []string            `json:"required,omitempty"`
}

type Property struct {
	Type        string `json:"type,omitempty"`
	Description string `json:"description,omitempty"`
	Items       *Items `json:"items,omitempty"`
}

type Items struct {
	Type string `json:"type,omitempty"`
}

type CallToolParams struct {
	Name      string          `json:"name"`
	Arguments json.RawMessage `json:"arguments"`
}

type CallToolResult struct {
	Content []ContentBlock `json:"content"`
	IsError bool           `json:"isError,omitempty"`
}

type ContentBlock struct {
	Type string `json:"type"`
	Text string `json:"text"`
}

// ToolHandler is a function that handles a tool call.
type ToolHandler func(args json.RawMessage) (interface{}, error)

// Registry maps tool names to definitions and handlers.
type Registry interface {
	ListTools() []ToolDef
	CallTool(name string, args json.RawMessage) (interface{}, error)
}

// Server is the MCP stdio server.
type Server struct {
	registry    Registry
	initialized bool
}

// NewServer creates a new MCP server.
func NewServer(registry Registry) *Server {
	return &Server{registry: registry}
}

// Handle processes a single JSON-RPC line and returns a response (or nil for notifications).
func (s *Server) Handle(line []byte) *Response {
	var req Request
	if err := json.Unmarshal(line, &req); err != nil {
		return errorResponse(nil, -32700, "parse error: "+err.Error())
	}

	log.Printf("→ %s (id=%v)", req.Method, req.ID)

	// Notifications (no id) — no response needed
	if req.ID == nil && req.Method != "" {
		s.handleNotification(req)
		return nil
	}

	switch req.Method {
	case "initialize":
		return s.handleInitialize(req)
	case "initialized":
		return nil
	case "tools/list":
		return s.handleToolsList(req)
	case "tools/call":
		return s.handleToolsCall(req)
	case "ping":
		return &Response{JSONRPC: "2.0", ID: req.ID, Result: map[string]interface{}{}}
	default:
		return errorResponse(req.ID, -32601, fmt.Sprintf("method not found: %s", req.Method))
	}
}

func (s *Server) handleNotification(req Request) {
	// Handle notifications like "notifications/initialized"
	log.Printf("notification: %s", req.Method)
}

func (s *Server) handleInitialize(req Request) *Response {
	s.initialized = true
	return &Response{
		JSONRPC: "2.0",
		ID:      req.ID,
		Result: InitializeResult{
			ProtocolVersion: "2024-11-05",
			ServerInfo: ServerInfo{
				Name:    "quick-sqlite-mcp-server",
				Version: "1.0.0",
			},
			Capabilities: Caps{
				Tools: &ToolsCap{},
			},
		},
	}
}

func (s *Server) handleToolsList(req Request) *Response {
	return &Response{
		JSONRPC: "2.0",
		ID:      req.ID,
		Result:  ToolsListResult{Tools: s.registry.ListTools()},
	}
}

func (s *Server) handleToolsCall(req Request) *Response {
	var params CallToolParams
	if err := json.Unmarshal(req.Params, &params); err != nil {
		return errorResponse(req.ID, -32602, "invalid params: "+err.Error())
	}

	result, err := s.registry.CallTool(params.Name, params.Arguments)
	if err != nil {
		// Return as tool error (isError=true), not JSON-RPC error
		return &Response{
			JSONRPC: "2.0",
			ID:      req.ID,
			Result: CallToolResult{
				IsError: true,
				Content: []ContentBlock{{Type: "text", Text: err.Error()}},
			},
		}
	}

	// Marshal result to JSON text for the content block
	text, _ := json.MarshalIndent(result, "", "  ")
	return &Response{
		JSONRPC: "2.0",
		ID:      req.ID,
		Result: CallToolResult{
			Content: []ContentBlock{{Type: "text", Text: string(text)}},
		},
	}
}

func errorResponse(id interface{}, code int, msg string) *Response {
	return &Response{
		JSONRPC: "2.0",
		ID:      id,
		Error:   &RPCError{Code: code, Message: msg},
	}
}

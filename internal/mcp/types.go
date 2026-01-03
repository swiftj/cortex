// Package mcp implements the Model Context Protocol (MCP) JSON-RPC server for Cortex.
package mcp

import "encoding/json"

// JSON-RPC 2.0 version constant.
const JSONRPCVersion = "2.0"

// Standard JSON-RPC 2.0 error codes.
const (
	// ParseError indicates invalid JSON was received.
	ParseError = -32700
	// InvalidRequest indicates the JSON sent is not a valid Request object.
	InvalidRequest = -32600
	// MethodNotFound indicates the method does not exist or is not available.
	MethodNotFound = -32601
	// InvalidParams indicates invalid method parameter(s).
	InvalidParams = -32602
	// InternalError indicates an internal JSON-RPC error.
	InternalError = -32603
)

// Request represents a JSON-RPC 2.0 request.
type Request struct {
	JSONRPC string          `json:"jsonrpc"`
	ID      any             `json:"id"`
	Method  string          `json:"method"`
	Params  json.RawMessage `json:"params,omitempty"`
}

// Response represents a JSON-RPC 2.0 response.
type Response struct {
	JSONRPC string `json:"jsonrpc"`
	ID      any    `json:"id"`
	Result  any    `json:"result,omitempty"`
	Error   *Error `json:"error,omitempty"`
}

// Error represents a JSON-RPC 2.0 error object.
type Error struct {
	Code    int    `json:"code"`
	Message string `json:"message"`
	Data    any    `json:"data,omitempty"`
}

// NewError creates a new Error with the given code and message.
func NewError(code int, message string) *Error {
	return &Error{
		Code:    code,
		Message: message,
	}
}

// NewErrorWithData creates a new Error with the given code, message, and data.
func NewErrorWithData(code int, message string, data any) *Error {
	return &Error{
		Code:    code,
		Message: message,
		Data:    data,
	}
}

// ServerInfo contains information about the MCP server.
type ServerInfo struct {
	Name    string `json:"name"`
	Version string `json:"version"`
}

// ServerCapabilities describes what the server can do.
type ServerCapabilities struct {
	Tools *ToolsCapability `json:"tools,omitempty"`
}

// ToolsCapability indicates the server supports tools.
type ToolsCapability struct {
	ListChanged bool `json:"listChanged,omitempty"`
}

// InitializeParams contains parameters for the initialize request.
type InitializeParams struct {
	ProtocolVersion string             `json:"protocolVersion"`
	Capabilities    ClientCapabilities `json:"capabilities"`
	ClientInfo      ClientInfo         `json:"clientInfo"`
}

// ClientCapabilities describes what the client can do.
type ClientCapabilities struct {
	Roots    *RootsCapability    `json:"roots,omitempty"`
	Sampling *SamplingCapability `json:"sampling,omitempty"`
}

// RootsCapability indicates the client supports roots.
type RootsCapability struct {
	ListChanged bool `json:"listChanged,omitempty"`
}

// SamplingCapability indicates the client supports sampling.
type SamplingCapability struct{}

// ClientInfo contains information about the MCP client.
type ClientInfo struct {
	Name    string `json:"name"`
	Version string `json:"version"`
}

// InitializeResult is the response to an initialize request.
type InitializeResult struct {
	ProtocolVersion string             `json:"protocolVersion"`
	Capabilities    ServerCapabilities `json:"capabilities"`
	ServerInfo      ServerInfo         `json:"serverInfo"`
}

// Tool represents an MCP tool that can be called.
type Tool struct {
	Name        string     `json:"name"`
	Description string     `json:"description"`
	InputSchema JSONSchema `json:"inputSchema"`
}

// JSONSchema represents a JSON Schema for tool input validation.
type JSONSchema struct {
	Type                 string                `json:"type"`
	Description          string                `json:"description,omitempty"`
	Properties           map[string]JSONSchema `json:"properties,omitempty"`
	Required             []string              `json:"required,omitempty"`
	Items                *JSONSchema           `json:"items,omitempty"`
	AdditionalProperties *bool                 `json:"additionalProperties,omitempty"`
	Default              any                   `json:"default,omitempty"`
	Minimum              *float64              `json:"minimum,omitempty"`
	Maximum              *float64              `json:"maximum,omitempty"`
}

// ToolsListResult is the response to a tools/list request.
type ToolsListResult struct {
	Tools []Tool `json:"tools"`
}

// ToolCallParams contains parameters for a tools/call request.
type ToolCallParams struct {
	Name      string          `json:"name"`
	Arguments json.RawMessage `json:"arguments"`
}

// ToolCallResult is the response to a tools/call request.
type ToolCallResult struct {
	Content []ContentBlock `json:"content"`
	IsError bool           `json:"isError,omitempty"`
}

// ContentBlock represents a content block in a tool result.
type ContentBlock struct {
	Type string `json:"type"`
	Text string `json:"text,omitempty"`
}

// NewTextContent creates a new text content block.
func NewTextContent(text string) ContentBlock {
	return ContentBlock{
		Type: "text",
		Text: text,
	}
}

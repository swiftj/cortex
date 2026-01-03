package mcp

import (
	"bufio"
	"context"
	"encoding/json"
	"fmt"
	"io"
	"os"
	"sync"
)

// Handler is a function that handles an MCP method call.
type Handler func(ctx context.Context, params json.RawMessage) (any, error)

// Server is an MCP server that communicates over stdio using JSON-RPC 2.0.
type Server struct {
	name    string
	version string

	tools    []Tool
	handlers map[string]Handler

	mu          sync.RWMutex
	initialized bool

	stdin  io.Reader
	stdout io.Writer
	stderr io.Writer
}

// NewServer creates a new MCP server with the given name and version.
func NewServer(name, version string) *Server {
	return &Server{
		name:     name,
		version:  version,
		tools:    make([]Tool, 0),
		handlers: make(map[string]Handler),
		stdin:    os.Stdin,
		stdout:   os.Stdout,
		stderr:   os.Stderr,
	}
}

// SetIO sets custom I/O streams for the server (useful for testing).
func (s *Server) SetIO(stdin io.Reader, stdout, stderr io.Writer) {
	s.stdin = stdin
	s.stdout = stdout
	s.stderr = stderr
}

// RegisterTool registers a tool with its handler.
func (s *Server) RegisterTool(tool Tool, handler Handler) {
	s.mu.Lock()
	defer s.mu.Unlock()

	s.tools = append(s.tools, tool)
	s.handlers[tool.Name] = handler
}

// Run starts the server and processes requests from stdin until ctx is canceled or EOF.
func (s *Server) Run(ctx context.Context) error {
	scanner := bufio.NewScanner(s.stdin)
	// Increase buffer size for large requests
	buf := make([]byte, 0, 1024*1024)
	scanner.Buffer(buf, 10*1024*1024)

	for {
		select {
		case <-ctx.Done():
			return ctx.Err()
		default:
		}

		if !scanner.Scan() {
			if err := scanner.Err(); err != nil {
				return fmt.Errorf("scanner error: %w", err)
			}
			// EOF reached
			return nil
		}

		line := scanner.Bytes()
		if len(line) == 0 {
			continue
		}

		response := s.handleRequest(ctx, line)
		if response != nil {
			if err := s.writeResponse(response); err != nil {
				s.logError("failed to write response: %v", err)
			}
		}
	}
}

// handleRequest parses and routes a JSON-RPC request.
func (s *Server) handleRequest(ctx context.Context, data []byte) *Response {
	var req Request
	if err := json.Unmarshal(data, &req); err != nil {
		return &Response{
			JSONRPC: JSONRPCVersion,
			ID:      nil,
			Error:   NewError(ParseError, "Parse error: "+err.Error()),
		}
	}

	// Validate JSON-RPC version
	if req.JSONRPC != JSONRPCVersion {
		return &Response{
			JSONRPC: JSONRPCVersion,
			ID:      req.ID,
			Error:   NewError(InvalidRequest, "Invalid Request: jsonrpc must be \"2.0\""),
		}
	}

	// Route the request
	result, err := s.route(ctx, req.Method, req.Params)
	if err != nil {
		if mcpErr, ok := err.(*Error); ok {
			return &Response{
				JSONRPC: JSONRPCVersion,
				ID:      req.ID,
				Error:   mcpErr,
			}
		}
		return &Response{
			JSONRPC: JSONRPCVersion,
			ID:      req.ID,
			Error:   NewError(InternalError, err.Error()),
		}
	}

	// Notifications (id is null) don't get a response
	if req.ID == nil {
		return nil
	}

	return &Response{
		JSONRPC: JSONRPCVersion,
		ID:      req.ID,
		Result:  result,
	}
}

// route dispatches a request to the appropriate handler.
func (s *Server) route(ctx context.Context, method string, params json.RawMessage) (any, error) {
	switch method {
	case "initialize":
		return s.handleInitialize(ctx, params)
	case "initialized":
		return s.handleInitialized(ctx, params)
	case "tools/list":
		return s.handleToolsList(ctx, params)
	case "tools/call":
		return s.handleToolsCall(ctx, params)
	case "ping":
		return s.handlePing(ctx, params)
	default:
		return nil, NewError(MethodNotFound, fmt.Sprintf("Method not found: %s", method))
	}
}

// handleInitialize handles the initialize request.
func (s *Server) handleInitialize(ctx context.Context, params json.RawMessage) (any, error) {
	var initParams InitializeParams
	if len(params) > 0 {
		if err := json.Unmarshal(params, &initParams); err != nil {
			return nil, NewError(InvalidParams, "Invalid params: "+err.Error())
		}
	}

	s.mu.Lock()
	s.initialized = true
	s.mu.Unlock()

	return InitializeResult{
		ProtocolVersion: "2024-11-05",
		Capabilities: ServerCapabilities{
			Tools: &ToolsCapability{
				ListChanged: false,
			},
		},
		ServerInfo: ServerInfo{
			Name:    s.name,
			Version: s.version,
		},
	}, nil
}

// handleInitialized handles the initialized notification.
func (s *Server) handleInitialized(ctx context.Context, params json.RawMessage) (any, error) {
	// This is a notification, no response needed
	return nil, nil
}

// handleToolsList handles the tools/list request.
func (s *Server) handleToolsList(ctx context.Context, params json.RawMessage) (any, error) {
	s.mu.RLock()
	defer s.mu.RUnlock()

	return ToolsListResult{
		Tools: s.tools,
	}, nil
}

// handleToolsCall handles the tools/call request.
func (s *Server) handleToolsCall(ctx context.Context, params json.RawMessage) (any, error) {
	var callParams ToolCallParams
	if err := json.Unmarshal(params, &callParams); err != nil {
		return nil, NewError(InvalidParams, "Invalid params: "+err.Error())
	}

	s.mu.RLock()
	handler, exists := s.handlers[callParams.Name]
	s.mu.RUnlock()

	if !exists {
		return nil, NewError(MethodNotFound, fmt.Sprintf("Tool not found: %s", callParams.Name))
	}

	result, err := handler(ctx, callParams.Arguments)
	if err != nil {
		// Return error as tool result, not as JSON-RPC error
		return ToolCallResult{
			Content: []ContentBlock{NewTextContent(err.Error())},
			IsError: true,
		}, nil
	}

	// Convert result to JSON for text content
	resultJSON, err := json.Marshal(result)
	if err != nil {
		return ToolCallResult{
			Content: []ContentBlock{NewTextContent("Failed to marshal result: " + err.Error())},
			IsError: true,
		}, nil
	}

	return ToolCallResult{
		Content: []ContentBlock{NewTextContent(string(resultJSON))},
		IsError: false,
	}, nil
}

// handlePing handles the ping request.
func (s *Server) handlePing(ctx context.Context, params json.RawMessage) (any, error) {
	return map[string]string{}, nil
}

// writeResponse writes a JSON-RPC response to stdout.
func (s *Server) writeResponse(resp *Response) error {
	data, err := json.Marshal(resp)
	if err != nil {
		return fmt.Errorf("marshal response: %w", err)
	}

	data = append(data, '\n')
	_, err = s.stdout.Write(data)
	return err
}

// logError writes an error message to stderr.
func (s *Server) logError(format string, args ...any) {
	fmt.Fprintf(s.stderr, format+"\n", args...)
}

// Error implements the error interface for Error type.
func (e *Error) Error() string {
	return fmt.Sprintf("MCP error %d: %s", e.Code, e.Message)
}

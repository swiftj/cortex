package mcp

import (
	"context"
	"encoding/json"
	"fmt"
)

// ValidateAndUnmarshal validates and unmarshals JSON parameters into the target struct.
// It returns an InvalidParams error if unmarshaling fails.
func ValidateAndUnmarshal[T any](params json.RawMessage) (T, error) {
	var result T
	if len(params) == 0 {
		return result, NewError(InvalidParams, "Missing required parameters")
	}

	if err := json.Unmarshal(params, &result); err != nil {
		return result, NewError(InvalidParams, fmt.Sprintf("Invalid parameters: %s", err.Error()))
	}

	return result, nil
}

// MarshalResult marshals a result value to JSON.
// It returns an InternalError if marshaling fails.
func MarshalResult(result any) (json.RawMessage, error) {
	data, err := json.Marshal(result)
	if err != nil {
		return nil, NewError(InternalError, fmt.Sprintf("Failed to marshal result: %s", err.Error()))
	}
	return data, nil
}

// MemoryService defines the interface for memory operations.
// This allows the MCP handlers to be decoupled from the database implementation.
type MemoryService interface {
	// Add adds a new memory and returns its ID.
	Add(ctx context.Context, args MemoryAddArgs) (int64, error)

	// Search searches for memories matching the query.
	Search(ctx context.Context, args MemorySearchArgs) ([]MemorySearchResult, error)

	// Update updates an existing memory.
	Update(ctx context.Context, args MemoryUpdateArgs) error

	// Delete deletes a memory by ID.
	Delete(ctx context.Context, id int64) error
}

// MemoryHandlers creates handlers for memory tools that use the provided service.
type MemoryHandlers struct {
	service MemoryService
}

// NewMemoryHandlers creates a new MemoryHandlers with the given service.
func NewMemoryHandlers(service MemoryService) *MemoryHandlers {
	return &MemoryHandlers{
		service: service,
	}
}

// Register registers all memory tool handlers with the server.
func (h *MemoryHandlers) Register(server *Server) {
	for _, tool := range MemoryTools() {
		var handler Handler
		switch tool.Name {
		case "memory.add":
			handler = h.HandleAdd
		case "memory.search":
			handler = h.HandleSearch
		case "memory.update":
			handler = h.HandleUpdate
		case "memory.delete":
			handler = h.HandleDelete
		default:
			continue
		}
		server.RegisterTool(tool, handler)
	}
}

// HandleAdd handles the memory.add tool call.
func (h *MemoryHandlers) HandleAdd(ctx context.Context, params json.RawMessage) (any, error) {
	args, err := ValidateAndUnmarshal[MemoryAddArgs](params)
	if err != nil {
		return nil, err
	}

	// Validate required fields
	if args.Text == "" {
		return nil, NewError(InvalidParams, "text is required and cannot be empty")
	}

	// Set defaults
	if args.Kind == "" {
		args.Kind = "note"
	}
	if args.Importance == nil {
		defaultImportance := float32(0.5)
		args.Importance = &defaultImportance
	}

	id, err := h.service.Add(ctx, args)
	if err != nil {
		return nil, fmt.Errorf("failed to add memory: %w", err)
	}

	return MemoryAddResult{ID: id}, nil
}

// HandleSearch handles the memory.search tool call.
func (h *MemoryHandlers) HandleSearch(ctx context.Context, params json.RawMessage) (any, error) {
	args, err := ValidateAndUnmarshal[MemorySearchArgs](params)
	if err != nil {
		return nil, err
	}

	// Validate required fields
	if args.Query == "" {
		return nil, NewError(InvalidParams, "query is required and cannot be empty")
	}

	// Set defaults
	if args.K == nil {
		defaultK := 10
		args.K = &defaultK
	}
	if args.Hybrid == nil {
		defaultHybrid := true
		args.Hybrid = &defaultHybrid
	}

	// Clamp k to valid range
	if *args.K < 1 {
		*args.K = 1
	}
	if *args.K > 100 {
		*args.K = 100
	}

	results, err := h.service.Search(ctx, args)
	if err != nil {
		return nil, fmt.Errorf("failed to search memories: %w", err)
	}

	return results, nil
}

// HandleUpdate handles the memory.update tool call.
func (h *MemoryHandlers) HandleUpdate(ctx context.Context, params json.RawMessage) (any, error) {
	args, err := ValidateAndUnmarshal[MemoryUpdateArgs](params)
	if err != nil {
		return nil, err
	}

	// Validate required fields
	if args.ID <= 0 {
		return nil, NewError(InvalidParams, "id must be a positive integer")
	}

	// Validate importance range if provided
	if args.Patch.Importance != nil {
		if *args.Patch.Importance < 0 || *args.Patch.Importance > 1 {
			return nil, NewError(InvalidParams, "importance must be between 0.0 and 1.0")
		}
	}

	err = h.service.Update(ctx, args)
	if err != nil {
		return nil, fmt.Errorf("failed to update memory: %w", err)
	}

	return MemoryUpdateResult{OK: true}, nil
}

// HandleDelete handles the memory.delete tool call.
func (h *MemoryHandlers) HandleDelete(ctx context.Context, params json.RawMessage) (any, error) {
	args, err := ValidateAndUnmarshal[MemoryDeleteArgs](params)
	if err != nil {
		return nil, err
	}

	// Validate required fields
	if args.ID <= 0 {
		return nil, NewError(InvalidParams, "id must be a positive integer")
	}

	err = h.service.Delete(ctx, args.ID)
	if err != nil {
		return nil, fmt.Errorf("failed to delete memory: %w", err)
	}

	return MemoryDeleteResult{OK: true}, nil
}

// WrapError wraps an error with an MCP error code if it isn't already an MCP error.
func WrapError(err error, code int) error {
	if err == nil {
		return nil
	}
	if _, ok := err.(*Error); ok {
		return err
	}
	return NewError(code, err.Error())
}

package mcp

// MemoryTools returns the MCP tool definitions for memory operations.
func MemoryTools() []Tool {
	return []Tool{
		MemoryAddTool(),
		MemorySearchTool(),
		MemoryUpdateTool(),
		MemoryDeleteTool(),
		MemoryExportTool(),
		MemoryImportTool(),
	}
}

// MemoryAddTool returns the tool definition for memory.add.
func MemoryAddTool() Tool {
	falseVal := false
	minImportance := 0.0
	maxImportance := 1.0
	defaultImportance := 0.5
	minK := 1.0

	return Tool{
		Name:        "memory.add",
		Description: "Add a new memory to the memory store. Memories can be facts, notes, preferences, todos, or other types of information to remember.",
		InputSchema: JSONSchema{
			Type: "object",
			Properties: map[string]JSONSchema{
				"text": {
					Type:        "string",
					Description: "The text content of the memory to store.",
				},
				"kind": {
					Type:        "string",
					Description: "The type of memory: note, fact, todo, preference, identity, project, or other custom types.",
					Default:     "note",
				},
				"importance": {
					Type:        "number",
					Description: "Importance score from 0.0 to 1.0, used for ranking and retention. Higher values indicate more important memories.",
					Minimum:     &minImportance,
					Maximum:     &maxImportance,
					Default:     defaultImportance,
				},
				"tags": {
					Type:        "array",
					Description: "Optional tags for categorizing the memory.",
					Items: &JSONSchema{
						Type: "string",
					},
				},
				"ttl_days": {
					Type:        "integer",
					Description: "Optional time-to-live in days. After this period, the memory may be automatically cleaned up.",
					Minimum:     &minK,
				},
				"source": {
					Type:        "string",
					Description: "Optional source identifier (e.g., 'chat', 'file:/path/to/file').",
				},
			},
			Required:             []string{"text"},
			AdditionalProperties: &falseVal,
		},
	}
}

// MemorySearchTool returns the tool definition for memory.search.
func MemorySearchTool() Tool {
	falseVal := false
	minK := 1.0
	maxK := 100.0
	defaultK := 10.0

	return Tool{
		Name:        "memory.search",
		Description: "Search for relevant memories using semantic (vector) and/or lexical (text) similarity. Returns ranked results based on relevance to the query.",
		InputSchema: JSONSchema{
			Type: "object",
			Properties: map[string]JSONSchema{
				"query": {
					Type:        "string",
					Description: "The search query to find relevant memories.",
				},
				"k": {
					Type:        "integer",
					Description: "Maximum number of results to return (1-100).",
					Minimum:     &minK,
					Maximum:     &maxK,
					Default:     defaultK,
				},
				"hybrid": {
					Type:        "boolean",
					Description: "If true, use hybrid search combining vector and lexical similarity. If false, use vector search only.",
					Default:     true,
				},
				"model": {
					Type:        "string",
					Description: "Optional: filter search to a specific embedding model (e.g., 'text-embedding-3-small'). Leave empty to search all models.",
				},
			},
			Required:             []string{"query"},
			AdditionalProperties: &falseVal,
		},
	}
}

// MemoryUpdateTool returns the tool definition for memory.update.
func MemoryUpdateTool() Tool {
	falseVal := false
	minImportance := 0.0
	maxImportance := 1.0
	minTTL := 1.0

	return Tool{
		Name:        "memory.update",
		Description: "Update an existing memory by ID. Only the fields provided in the patch will be updated.",
		InputSchema: JSONSchema{
			Type: "object",
			Properties: map[string]JSONSchema{
				"id": {
					Type:        "integer",
					Description: "The ID of the memory to update.",
				},
				"patch": {
					Type:        "object",
					Description: "The fields to update.",
					Properties: map[string]JSONSchema{
						"text": {
							Type:        "string",
							Description: "New text content for the memory.",
						},
						"kind": {
							Type:        "string",
							Description: "New memory type.",
						},
						"importance": {
							Type:        "number",
							Description: "New importance score (0.0 to 1.0).",
							Minimum:     &minImportance,
							Maximum:     &maxImportance,
						},
						"tags": {
							Type:        "array",
							Description: "New tags for the memory.",
							Items: &JSONSchema{
								Type: "string",
							},
						},
						"ttl_days": {
							Type:        "integer",
							Description: "New time-to-live in days.",
							Minimum:     &minTTL,
						},
						"source": {
							Type:        "string",
							Description: "New source identifier.",
						},
					},
					AdditionalProperties: &falseVal,
				},
			},
			Required:             []string{"id", "patch"},
			AdditionalProperties: &falseVal,
		},
	}
}

// MemoryDeleteTool returns the tool definition for memory.delete.
func MemoryDeleteTool() Tool {
	falseVal := false

	return Tool{
		Name:        "memory.delete",
		Description: "Delete a memory by ID. This operation cannot be undone.",
		InputSchema: JSONSchema{
			Type: "object",
			Properties: map[string]JSONSchema{
				"id": {
					Type:        "integer",
					Description: "The ID of the memory to delete.",
				},
			},
			Required:             []string{"id"},
			AdditionalProperties: &falseVal,
		},
	}
}

// Memory tool argument types

// MemoryAddArgs contains the arguments for memory.add.
type MemoryAddArgs struct {
	Text       string   `json:"text"`
	Kind       string   `json:"kind,omitempty"`
	Importance *float32 `json:"importance,omitempty"`
	Tags       []string `json:"tags,omitempty"`
	TTLDays    *int     `json:"ttl_days,omitempty"`
	Source     *string  `json:"source,omitempty"`
}

// MemoryAddResult is the result of memory.add.
type MemoryAddResult struct {
	ID int64 `json:"id"`
}

// MemorySearchArgs contains the arguments for memory.search.
type MemorySearchArgs struct {
	Query  string  `json:"query"`
	K      *int    `json:"k,omitempty"`
	Hybrid *bool   `json:"hybrid,omitempty"`
	Model  *string `json:"model,omitempty"` // Optional: filter by embedding model
}

// MemorySearchResult is a single search result.
type MemorySearchResult struct {
	ID         int64    `json:"id"`
	Text       string   `json:"text"`
	Score      float32  `json:"score"`
	Source     *string  `json:"source,omitempty"`
	Tags       []string `json:"tags"`
	Importance float32  `json:"importance"`
}

// MemoryUpdateArgs contains the arguments for memory.update.
type MemoryUpdateArgs struct {
	ID    int64             `json:"id"`
	Patch MemoryUpdatePatch `json:"patch"`
}

// MemoryUpdatePatch contains the fields that can be updated.
type MemoryUpdatePatch struct {
	Text       *string  `json:"text,omitempty"`
	Kind       *string  `json:"kind,omitempty"`
	Importance *float32 `json:"importance,omitempty"`
	Tags       []string `json:"tags,omitempty"`
	TTLDays    *int     `json:"ttl_days,omitempty"`
	Source     *string  `json:"source,omitempty"`
}

// MemoryUpdateResult is the result of memory.update.
type MemoryUpdateResult struct {
	OK bool `json:"ok"`
}

// MemoryDeleteArgs contains the arguments for memory.delete.
type MemoryDeleteArgs struct {
	ID int64 `json:"id"`
}

// MemoryDeleteResult is the result of memory.delete.
type MemoryDeleteResult struct {
	OK bool `json:"ok"`
}

// MemoryExportTool returns the tool definition for memory.export.
func MemoryExportTool() Tool {
	falseVal := false
	minLimit := 1.0

	return Tool{
		Name:        "memory.export",
		Description: "Export memories to JSONL format. Returns a string containing the exported data that can be saved to a file.",
		InputSchema: JSONSchema{
			Type: "object",
			Properties: map[string]JSONSchema{
				"include_embeddings": {
					Type:        "boolean",
					Description: "Include vector embeddings in export (results in larger output).",
					Default:     false,
				},
				"kind": {
					Type:        "string",
					Description: "Filter to export only memories of this type.",
				},
				"limit": {
					Type:        "integer",
					Description: "Maximum number of memories to export.",
					Minimum:     &minLimit,
				},
			},
			AdditionalProperties: &falseVal,
		},
	}
}

// MemoryImportTool returns the tool definition for memory.import.
func MemoryImportTool() Tool {
	falseVal := false

	return Tool{
		Name:        "memory.import",
		Description: "Import memories from JSONL format. Each line should be a JSON object representing a memory.",
		InputSchema: JSONSchema{
			Type: "object",
			Properties: map[string]JSONSchema{
				"data": {
					Type:        "string",
					Description: "JSONL data to import (one JSON object per line).",
				},
				"skip_existing": {
					Type:        "boolean",
					Description: "Skip records that already exist instead of updating them.",
					Default:     false,
				},
				"regenerate_embeddings": {
					Type:        "boolean",
					Description: "Generate new embeddings instead of using imported ones.",
					Default:     false,
				},
				"dry_run": {
					Type:        "boolean",
					Description: "Validate import without writing to database.",
					Default:     false,
				},
			},
			Required:             []string{"data"},
			AdditionalProperties: &falseVal,
		},
	}
}

// MemoryExportArgs contains the arguments for memory.export.
type MemoryExportArgs struct {
	IncludeEmbeddings bool   `json:"include_embeddings,omitempty"`
	Kind              string `json:"kind,omitempty"`
	Limit             int    `json:"limit,omitempty"`
}

// MemoryExportResult is the result of memory.export.
type MemoryExportResult struct {
	Data     string `json:"data"`
	Exported int64  `json:"exported"`
	Errors   int64  `json:"errors"`
}

// MemoryImportArgs contains the arguments for memory.import.
type MemoryImportArgs struct {
	Data                 string `json:"data"`
	SkipExisting         bool   `json:"skip_existing,omitempty"`
	RegenerateEmbeddings bool   `json:"regenerate_embeddings,omitempty"`
	DryRun               bool   `json:"dry_run,omitempty"`
}

// MemoryImportResult is the result of memory.import.
type MemoryImportResult struct {
	Total    int64 `json:"total"`
	Imported int64 `json:"imported"`
	Skipped  int64 `json:"skipped"`
	Errors   int64 `json:"errors"`
}

// MemoryEntitiesTool returns the tool definition for memory.entities.
func MemoryEntitiesTool() Tool {
	falseVal := false
	minID := 1.0

	return Tool{
		Name:        "memory.entities",
		Description: "Get entities extracted from a specific memory. Returns all entities (people, organizations, technologies, etc.) that were automatically identified in the memory text.",
		InputSchema: JSONSchema{
			Type: "object",
			Properties: map[string]JSONSchema{
				"memory_id": {
					Type:        "integer",
					Description: "The ID of the memory to get entities for.",
					Minimum:     &minID,
				},
			},
			Required:             []string{"memory_id"},
			AdditionalProperties: &falseVal,
		},
	}
}

// MemoryRelatedTool returns the tool definition for memory.related.
func MemoryRelatedTool() Tool {
	falseVal := false
	minID := 1.0
	minK := 1.0
	maxK := 100.0
	defaultK := 10.0

	return Tool{
		Name:        "memory.related",
		Description: "Find memories that share entities with the given memory. This discovers connections between memories based on common people, organizations, technologies, or concepts.",
		InputSchema: JSONSchema{
			Type: "object",
			Properties: map[string]JSONSchema{
				"memory_id": {
					Type:        "integer",
					Description: "The ID of the memory to find related memories for.",
					Minimum:     &minID,
				},
				"k": {
					Type:        "integer",
					Description: "Maximum number of related memories to return (1-100).",
					Minimum:     &minK,
					Maximum:     &maxK,
					Default:     defaultK,
				},
			},
			Required:             []string{"memory_id"},
			AdditionalProperties: &falseVal,
		},
	}
}

// MemoryEntitiesArgs contains the arguments for memory.entities.
type MemoryEntitiesArgs struct {
	MemoryID int64 `json:"memory_id"`
}

// EntityResult is a single entity in the response.
type EntityResult struct {
	ID          int64    `json:"id"`
	Name        string   `json:"name"`
	Type        string   `json:"type"`
	Aliases     []string `json:"aliases,omitempty"`
	Description *string  `json:"description,omitempty"`
}

// MemoryEntitiesResult is the result of memory.entities.
type MemoryEntitiesResult struct {
	Entities []EntityResult `json:"entities"`
}

// MemoryRelatedArgs contains the arguments for memory.related.
type MemoryRelatedArgs struct {
	MemoryID int64 `json:"memory_id"`
	K        *int  `json:"k,omitempty"`
}

// MemoryRelatedResult is a single related memory in the response.
type MemoryRelatedResult struct {
	ID         int64    `json:"id"`
	Text       string   `json:"text"`
	Kind       string   `json:"kind"`
	Score      float32  `json:"score"` // Entity overlap score
	Source     *string  `json:"source,omitempty"`
	Tags       []string `json:"tags"`
	Importance float32  `json:"importance"`
}

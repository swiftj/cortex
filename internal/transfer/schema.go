// Package transfer provides export/import functionality for Cortex memories.
package transfer

import (
	"time"
)

// MemoryRecord represents a memory in JSONL export format.
// This is the canonical schema for data interchange.
type MemoryRecord struct {
	// Core fields
	ID        int64     `json:"id"`
	TenantID  string    `json:"tenant_id"`
	Kind      string    `json:"kind"`
	Text      string    `json:"text"`
	Source    *string   `json:"source,omitempty"`
	CreatedAt time.Time `json:"created_at"`
	UpdatedAt time.Time `json:"updated_at"`

	// Metadata
	Tags       []string       `json:"tags,omitempty"`
	Importance float32        `json:"importance"`
	TTLDays    *int           `json:"ttl_days,omitempty"`
	Meta       map[string]any `json:"meta,omitempty"`

	// Embedding (optional, for full export)
	Embedding *EmbeddingRecord `json:"embedding,omitempty"`
}

// EmbeddingRecord represents an embedding in export format.
type EmbeddingRecord struct {
	Model     string    `json:"model"`
	Dims      int       `json:"dims"`
	Vector    []float32 `json:"vector"`
}

// ExportOptions configures export behavior.
type ExportOptions struct {
	// IncludeEmbeddings includes vector embeddings in export (larger file size)
	IncludeEmbeddings bool

	// TenantID filters export to specific tenant (empty = all)
	TenantID string

	// Kind filters export to specific memory kind (empty = all)
	Kind string

	// Since exports only memories created/updated after this time
	Since *time.Time

	// Limit maximum number of records to export (0 = unlimited)
	Limit int
}

// ImportOptions configures import behavior.
type ImportOptions struct {
	// SkipExisting skips records with matching IDs instead of updating
	SkipExisting bool

	// RegenerateEmbeddings creates new embeddings instead of using exported ones
	RegenerateEmbeddings bool

	// OverrideTenantID forces all imported records to use this tenant ID
	OverrideTenantID string

	// DryRun validates import without writing to database
	DryRun bool
}

// ImportResult contains statistics from an import operation.
type ImportResult struct {
	Total    int64 `json:"total"`
	Imported int64 `json:"imported"`
	Skipped  int64 `json:"skipped"`
	Errors   int64 `json:"errors"`
}

// ExportResult contains statistics from an export operation.
type ExportResult struct {
	Total    int64 `json:"total"`
	Exported int64 `json:"exported"`
	Errors   int64 `json:"errors"`
}

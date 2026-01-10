package db

import (
	"context"
	"fmt"
	"sync"
)

var (
	hnswIndexCreated bool
	hnswIndexMu      sync.Mutex
)

// EnsureHNSWIndex creates the HNSW index if it doesn't exist.
// This is called lazily after the first embedding is inserted,
// because pgvector requires knowing dimensions to create the index.
func (db *DB) EnsureHNSWIndex(ctx context.Context) error {
	hnswIndexMu.Lock()
	defer hnswIndexMu.Unlock()

	if hnswIndexCreated {
		return nil
	}

	// Check if index already exists
	var exists bool
	err := db.pool.QueryRow(ctx, `
		SELECT EXISTS (
			SELECT 1 FROM pg_indexes
			WHERE indexname = 'idx_memory_embed_hnsw'
		)
	`).Scan(&exists)
	if err != nil {
		return fmt.Errorf("check index existence: %w", err)
	}

	if exists {
		hnswIndexCreated = true
		return nil
	}

	// Create HNSW index - this will infer dimensions from existing data
	_, err = db.pool.Exec(ctx, `
		CREATE INDEX IF NOT EXISTS idx_memory_embed_hnsw
		ON memory_embeddings USING hnsw (embedding vector_cosine_ops)
	`)
	if err != nil {
		return fmt.Errorf("create HNSW index: %w", err)
	}

	hnswIndexCreated = true
	return nil
}

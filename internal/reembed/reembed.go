// Package reembed provides batch re-embedding utilities for switching embedding models.
package reembed

import (
	"context"
	"fmt"
	"log"
	"time"

	"github.com/jackc/pgx/v5/pgxpool"
)

// EmbeddingProvider generates embeddings for text.
type EmbeddingProvider interface {
	Embed(ctx context.Context, text string) ([]float32, error)
	Model() string
}

// ProgressCallback is called after each memory is processed.
type ProgressCallback func(processed, total int64, memoryID int64, err error)

// Config holds configuration for the re-embedding process.
type Config struct {
	// BatchSize is the number of memories to process in each batch.
	BatchSize int
	// DelayBetweenBatches is the delay between processing batches (rate limiting).
	DelayBetweenBatches time.Duration
	// DeleteOldEmbeddings removes embeddings from other models after re-embedding.
	DeleteOldEmbeddings bool
	// TargetModel is the model to use for re-embedding (if different from provider's default).
	TargetModel string
	// SkipExisting skips memories that already have embeddings for the target model.
	SkipExisting bool
}

// DefaultConfig returns a sensible default configuration.
func DefaultConfig() Config {
	return Config{
		BatchSize:           100,
		DelayBetweenBatches: 100 * time.Millisecond,
		DeleteOldEmbeddings: false,
		SkipExisting:        true,
	}
}

// Reembedder handles batch re-embedding of memories.
type Reembedder struct {
	pool        *pgxpool.Pool
	provider    EmbeddingProvider
	tenantID    string
	workspaceID string
	config      Config
}

// NewReembedder creates a new batch re-embedder.
func NewReembedder(pool *pgxpool.Pool, provider EmbeddingProvider, tenantID, workspaceID string) *Reembedder {
	return &Reembedder{
		pool:        pool,
		provider:    provider,
		tenantID:    tenantID,
		workspaceID: workspaceID,
		config:      DefaultConfig(),
	}
}

// WithConfig sets the configuration and returns the Reembedder for chaining.
func (r *Reembedder) WithConfig(cfg Config) *Reembedder {
	r.config = cfg
	return r
}

// Stats holds statistics about the re-embedding process.
type Stats struct {
	Total     int64
	Processed int64
	Skipped   int64
	Errors    int64
	Duration  time.Duration
}

// ReembedAll re-embeds all memories for the tenant/workspace.
// The progress callback is called after each memory is processed.
func (r *Reembedder) ReembedAll(ctx context.Context, progress ProgressCallback) (*Stats, error) {
	start := time.Now()
	stats := &Stats{}

	// Get total count
	var total int64
	err := r.pool.QueryRow(ctx, `
		SELECT COUNT(*) FROM memories
		WHERE tenant_id = $1 AND workspace_id = $2
	`, r.tenantID, r.workspaceID).Scan(&total)
	if err != nil {
		return nil, fmt.Errorf("count memories: %w", err)
	}
	stats.Total = total

	if total == 0 {
		return stats, nil
	}

	model := r.config.TargetModel
	if model == "" {
		model = r.provider.Model()
	}

	// Process in batches
	var offset int64
	for {
		select {
		case <-ctx.Done():
			stats.Duration = time.Since(start)
			return stats, ctx.Err()
		default:
		}

		// Get batch of memories
		rows, err := r.pool.Query(ctx, `
			SELECT id, text FROM memories
			WHERE tenant_id = $1 AND workspace_id = $2
			ORDER BY id
			LIMIT $3 OFFSET $4
		`, r.tenantID, r.workspaceID, r.config.BatchSize, offset)
		if err != nil {
			return nil, fmt.Errorf("query memories: %w", err)
		}

		var memories []struct {
			ID   int64
			Text string
		}
		for rows.Next() {
			var m struct {
				ID   int64
				Text string
			}
			if err := rows.Scan(&m.ID, &m.Text); err != nil {
				rows.Close()
				return nil, fmt.Errorf("scan memory: %w", err)
			}
			memories = append(memories, m)
		}
		rows.Close()

		if len(memories) == 0 {
			break
		}

		// Process each memory in the batch
		for _, m := range memories {
			var processErr error

			// Check if embedding exists and skip if configured
			if r.config.SkipExisting {
				var exists bool
				err := r.pool.QueryRow(ctx, `
					SELECT EXISTS(
						SELECT 1 FROM memory_embeddings
						WHERE memory_id = $1 AND model = $2
					)
				`, m.ID, model).Scan(&exists)
				if err == nil && exists {
					stats.Skipped++
					stats.Processed++
					if progress != nil {
						progress(stats.Processed, total, m.ID, nil)
					}
					continue
				}
			}

			// Generate embedding
			embedding, err := r.provider.Embed(ctx, m.Text)
			if err != nil {
				processErr = fmt.Errorf("embed memory %d: %w", m.ID, err)
				stats.Errors++
			} else {
				// Store embedding
				_, err = r.pool.Exec(ctx, `
					INSERT INTO memory_embeddings (memory_id, model, dims, embedding)
					VALUES ($1, $2, $3, $4)
					ON CONFLICT (memory_id, model) DO UPDATE SET
						dims = EXCLUDED.dims,
						embedding = EXCLUDED.embedding
				`, m.ID, model, len(embedding), embedding)
				if err != nil {
					processErr = fmt.Errorf("store embedding for memory %d: %w", m.ID, err)
					stats.Errors++
				}

				// Delete old embeddings if configured
				if r.config.DeleteOldEmbeddings && processErr == nil {
					_, err = r.pool.Exec(ctx, `
						DELETE FROM memory_embeddings
						WHERE memory_id = $1 AND model != $2
					`, m.ID, model)
					if err != nil {
						log.Printf("reembed: warning: failed to delete old embeddings for memory %d: %v", m.ID, err)
					}
				}
			}

			stats.Processed++
			if progress != nil {
				progress(stats.Processed, total, m.ID, processErr)
			}
		}

		offset += int64(len(memories))

		// Rate limiting delay between batches
		if r.config.DelayBetweenBatches > 0 {
			time.Sleep(r.config.DelayBetweenBatches)
		}
	}

	stats.Duration = time.Since(start)
	return stats, nil
}

// ReembedMemory re-embeds a single memory.
func (r *Reembedder) ReembedMemory(ctx context.Context, memoryID int64) error {
	model := r.config.TargetModel
	if model == "" {
		model = r.provider.Model()
	}

	// Get memory text
	var text string
	err := r.pool.QueryRow(ctx, `
		SELECT text FROM memories
		WHERE id = $1 AND tenant_id = $2 AND workspace_id = $3
	`, memoryID, r.tenantID, r.workspaceID).Scan(&text)
	if err != nil {
		return fmt.Errorf("get memory: %w", err)
	}

	// Generate embedding
	embedding, err := r.provider.Embed(ctx, text)
	if err != nil {
		return fmt.Errorf("embed: %w", err)
	}

	// Store embedding
	_, err = r.pool.Exec(ctx, `
		INSERT INTO memory_embeddings (memory_id, model, dims, embedding)
		VALUES ($1, $2, $3, $4)
		ON CONFLICT (memory_id, model) DO UPDATE SET
			dims = EXCLUDED.dims,
			embedding = EXCLUDED.embedding
	`, memoryID, model, len(embedding), embedding)
	if err != nil {
		return fmt.Errorf("store embedding: %w", err)
	}

	// Delete old embeddings if configured
	if r.config.DeleteOldEmbeddings {
		_, err = r.pool.Exec(ctx, `
			DELETE FROM memory_embeddings
			WHERE memory_id = $1 AND model != $2
		`, memoryID, model)
		if err != nil {
			return fmt.Errorf("delete old embeddings: %w", err)
		}
	}

	return nil
}

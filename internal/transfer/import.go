package transfer

import (
	"bufio"
	"context"
	"encoding/json"
	"fmt"
	"io"

	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgxpool"
	"github.com/pgvector/pgvector-go"
)

// EmbeddingProvider generates embeddings for text.
type EmbeddingProvider interface {
	Embed(ctx context.Context, text string) ([]float32, error)
	EmbedModel() string
	Dimensions() int
}

// Importer handles memory import operations.
type Importer struct {
	pool     *pgxpool.Pool
	embedder EmbeddingProvider
}

// NewImporter creates a new importer with the given database pool.
func NewImporter(pool *pgxpool.Pool, embedder EmbeddingProvider) *Importer {
	return &Importer{
		pool:     pool,
		embedder: embedder,
	}
}

// Import reads memories from JSONL format and inserts them into the database.
func (i *Importer) Import(ctx context.Context, r io.Reader, opts ImportOptions) (*ImportResult, error) {
	result := &ImportResult{}
	scanner := bufio.NewScanner(r)

	// Increase buffer size for large records
	buf := make([]byte, 0, 1024*1024) // 1MB buffer
	scanner.Buffer(buf, 10*1024*1024) // 10MB max

	for scanner.Scan() {
		result.Total++

		var record MemoryRecord
		if err := json.Unmarshal(scanner.Bytes(), &record); err != nil {
			result.Errors++
			continue
		}

		// Override tenant ID if specified
		if opts.OverrideTenantID != "" {
			record.TenantID = opts.OverrideTenantID
		}

		// Override workspace ID if specified
		if opts.OverrideWorkspaceID != "" {
			record.WorkspaceID = opts.OverrideWorkspaceID
		}

		// Set default workspace if not specified
		if record.WorkspaceID == "" {
			record.WorkspaceID = "default"
		}

		if opts.DryRun {
			result.Imported++
			continue
		}

		// Check if record exists
		if opts.SkipExisting {
			exists, err := i.memoryExists(ctx, record.ID, record.TenantID, record.WorkspaceID)
			if err != nil {
				result.Errors++
				continue
			}
			if exists {
				result.Skipped++
				continue
			}
		}

		// Import the record
		if err := i.importRecord(ctx, &record, opts); err != nil {
			result.Errors++
			continue
		}

		result.Imported++
	}

	if err := scanner.Err(); err != nil {
		return result, fmt.Errorf("scan input: %w", err)
	}

	return result, nil
}

func (i *Importer) memoryExists(ctx context.Context, id int64, tenantID, workspaceID string) (bool, error) {
	var exists bool
	err := i.pool.QueryRow(ctx, `
		SELECT EXISTS(SELECT 1 FROM memories WHERE id = $1 AND tenant_id = $2 AND workspace_id = $3)
	`, id, tenantID, workspaceID).Scan(&exists)
	return exists, err
}

func (i *Importer) importRecord(ctx context.Context, record *MemoryRecord, opts ImportOptions) error {
	tx, err := i.pool.Begin(ctx)
	if err != nil {
		return fmt.Errorf("begin transaction: %w", err)
	}
	defer tx.Rollback(ctx)

	// Prepare meta JSON
	metaJSON, err := json.Marshal(record.Meta)
	if err != nil {
		return fmt.Errorf("marshal meta: %w", err)
	}

	if record.Tags == nil {
		record.Tags = []string{}
	}

	// Upsert memory
	var memoryID int64
	err = tx.QueryRow(ctx, `
		INSERT INTO memories (id, tenant_id, workspace_id, kind, text, source, created_at, updated_at, tags, importance, ttl_days, meta)
		VALUES ($1, $2, $3, $4, $5, $6, $7, $8, $9, $10, $11, $12)
		ON CONFLICT (id) DO UPDATE SET
			workspace_id = EXCLUDED.workspace_id,
			kind = EXCLUDED.kind,
			text = EXCLUDED.text,
			source = EXCLUDED.source,
			updated_at = EXCLUDED.updated_at,
			tags = EXCLUDED.tags,
			importance = EXCLUDED.importance,
			ttl_days = EXCLUDED.ttl_days,
			meta = EXCLUDED.meta
		RETURNING id
	`, record.ID, record.TenantID, record.WorkspaceID, record.Kind, record.Text, record.Source,
		record.CreatedAt, record.UpdatedAt, record.Tags, record.Importance,
		record.TTLDays, metaJSON).Scan(&memoryID)

	if err != nil {
		return fmt.Errorf("upsert memory: %w", err)
	}

	// Handle embedding
	if opts.RegenerateEmbeddings && i.embedder != nil {
		// Generate new embedding
		vector, err := i.embedder.Embed(ctx, record.Text)
		if err != nil {
			return fmt.Errorf("generate embedding: %w", err)
		}
		if err := i.upsertEmbedding(ctx, tx, memoryID, i.embedder.EmbedModel(), vector); err != nil {
			return fmt.Errorf("upsert embedding: %w", err)
		}
	} else if record.Embedding != nil && !opts.RegenerateEmbeddings {
		// Use existing embedding from export
		if err := i.upsertEmbedding(ctx, tx, memoryID, record.Embedding.Model, record.Embedding.Vector); err != nil {
			return fmt.Errorf("upsert embedding: %w", err)
		}
	}

	if err := tx.Commit(ctx); err != nil {
		return fmt.Errorf("commit transaction: %w", err)
	}

	return nil
}

func (i *Importer) upsertEmbedding(ctx context.Context, tx pgx.Tx, memoryID int64, model string, vector []float32) error {
	vec := pgvector.NewVector(vector)
	_, err := tx.Exec(ctx, `
		INSERT INTO memory_embeddings (memory_id, model, dims, embedding)
		VALUES ($1, $2, $3, $4)
		ON CONFLICT (memory_id) DO UPDATE SET
			model = EXCLUDED.model,
			dims = EXCLUDED.dims,
			embedding = EXCLUDED.embedding
	`, memoryID, model, len(vector), vec)
	return err
}

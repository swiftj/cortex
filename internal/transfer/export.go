package transfer

import (
	"context"
	"encoding/json"
	"fmt"
	"io"

	"github.com/jackc/pgx/v5/pgxpool"
)

// Exporter handles memory export operations.
type Exporter struct {
	pool *pgxpool.Pool
}

// NewExporter creates a new exporter with the given database pool.
func NewExporter(pool *pgxpool.Pool) *Exporter {
	return &Exporter{pool: pool}
}

// Export writes memories to the given writer in JSONL format.
// Each line is a complete JSON object representing one memory.
func (e *Exporter) Export(ctx context.Context, w io.Writer, opts ExportOptions) (*ExportResult, error) {
	result := &ExportResult{}

	// Build query based on options
	query, args := e.buildExportQuery(opts)

	rows, err := e.pool.Query(ctx, query, args...)
	if err != nil {
		return nil, fmt.Errorf("query memories: %w", err)
	}
	defer rows.Close()

	encoder := json.NewEncoder(w)

	for rows.Next() {
		result.Total++

		record, err := e.scanMemoryRecord(rows, opts.IncludeEmbeddings)
		if err != nil {
			result.Errors++
			continue
		}

		// If including embeddings, fetch them
		if opts.IncludeEmbeddings {
			embedding, err := e.fetchEmbedding(ctx, record.ID)
			if err == nil && embedding != nil {
				record.Embedding = embedding
			}
		}

		if err := encoder.Encode(record); err != nil {
			result.Errors++
			continue
		}

		result.Exported++
	}

	if err := rows.Err(); err != nil {
		return result, fmt.Errorf("iterate rows: %w", err)
	}

	return result, nil
}

func (e *Exporter) buildExportQuery(opts ExportOptions) (string, []any) {
	query := `
		SELECT id, tenant_id, workspace_id, kind, text, source, created_at, updated_at,
		       tags, importance, ttl_days, meta
		FROM memories
		WHERE 1=1
	`
	args := []any{}
	argIdx := 1

	if opts.TenantID != "" {
		query += fmt.Sprintf(" AND tenant_id = $%d", argIdx)
		args = append(args, opts.TenantID)
		argIdx++
	}

	if opts.WorkspaceID != "" {
		query += fmt.Sprintf(" AND workspace_id = $%d", argIdx)
		args = append(args, opts.WorkspaceID)
		argIdx++
	}

	if opts.Kind != "" {
		query += fmt.Sprintf(" AND kind = $%d", argIdx)
		args = append(args, opts.Kind)
		argIdx++
	}

	if opts.Since != nil {
		query += fmt.Sprintf(" AND updated_at >= $%d", argIdx)
		args = append(args, *opts.Since)
		argIdx++
	}

	query += " ORDER BY id"

	if opts.Limit > 0 {
		query += fmt.Sprintf(" LIMIT $%d", argIdx)
		args = append(args, opts.Limit)
	}

	return query, args
}

func (e *Exporter) scanMemoryRecord(rows interface{ Scan(dest ...any) error }, _ bool) (*MemoryRecord, error) {
	var record MemoryRecord
	var metaJSON []byte

	err := rows.Scan(
		&record.ID,
		&record.TenantID,
		&record.WorkspaceID,
		&record.Kind,
		&record.Text,
		&record.Source,
		&record.CreatedAt,
		&record.UpdatedAt,
		&record.Tags,
		&record.Importance,
		&record.TTLDays,
		&metaJSON,
	)
	if err != nil {
		return nil, err
	}

	if len(metaJSON) > 0 {
		if err := json.Unmarshal(metaJSON, &record.Meta); err != nil {
			return nil, fmt.Errorf("unmarshal meta: %w", err)
		}
	}

	return &record, nil
}

func (e *Exporter) fetchEmbedding(ctx context.Context, memoryID int64) (*EmbeddingRecord, error) {
	var embedding EmbeddingRecord
	var vector []float32

	err := e.pool.QueryRow(ctx, `
		SELECT model, dims, embedding::float4[]
		FROM memory_embeddings
		WHERE memory_id = $1
	`, memoryID).Scan(&embedding.Model, &embedding.Dims, &vector)

	if err != nil {
		return nil, err
	}

	embedding.Vector = vector
	return &embedding, nil
}

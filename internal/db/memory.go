package db

import (
	"context"
	"encoding/json"
	"fmt"
	"time"

	"github.com/jackc/pgx/v5"
	"github.com/pgvector/pgvector-go"
)

// Memory represents a stored memory record.
type Memory struct {
	ID          int64          `json:"id"`
	TenantID    string         `json:"tenant_id"`
	WorkspaceID string         `json:"workspace_id"`
	Kind        string         `json:"kind"`
	Text        string         `json:"text"`
	Source      *string        `json:"source,omitempty"`
	CreatedAt   time.Time      `json:"created_at"`
	UpdatedAt   time.Time      `json:"updated_at"`
	Tags        []string       `json:"tags"`
	Importance  float32        `json:"importance"`
	TTLDays     *int           `json:"ttl_days,omitempty"`
	Meta        map[string]any `json:"meta,omitempty"`
}

// MemoryWithScore includes similarity score for search results.
type MemoryWithScore struct {
	Memory
	Score float32 `json:"score"`
}

// AddMemoryParams contains parameters for adding a new memory.
type AddMemoryParams struct {
	Kind       string
	Text       string
	Source     *string
	Tags       []string
	Importance float32
	TTLDays    *int
	Meta       map[string]any
}

// AddMemory inserts a new memory and returns its ID.
func (db *DB) AddMemory(ctx context.Context, params AddMemoryParams) (int64, error) {
	if params.Tags == nil {
		params.Tags = []string{}
	}
	if params.Meta == nil {
		params.Meta = map[string]any{}
	}

	metaJSON, err := json.Marshal(params.Meta)
	if err != nil {
		return 0, fmt.Errorf("marshal meta: %w", err)
	}

	var id int64
	err = db.pool.QueryRow(ctx, `
		INSERT INTO memories (tenant_id, workspace_id, kind, text, source, tags, importance, ttl_days, meta)
		VALUES ($1, $2, $3, $4, $5, $6, $7, $8, $9)
		RETURNING id
	`, db.tenantID, db.workspaceID, params.Kind, params.Text, params.Source, params.Tags, params.Importance, params.TTLDays, metaJSON).Scan(&id)

	if err != nil {
		return 0, fmt.Errorf("insert memory: %w", err)
	}

	return id, nil
}

// AddEmbedding stores an embedding for a memory.
func (db *DB) AddEmbedding(ctx context.Context, memoryID int64, model string, embedding []float32) error {
	vec := pgvector.NewVector(embedding)
	_, err := db.pool.Exec(ctx, `
		INSERT INTO memory_embeddings (memory_id, model, dims, embedding)
		VALUES ($1, $2, $3, $4)
		ON CONFLICT (memory_id) DO UPDATE SET
			model = EXCLUDED.model,
			dims = EXCLUDED.dims,
			embedding = EXCLUDED.embedding
	`, memoryID, model, len(embedding), vec)

	if err != nil {
		return fmt.Errorf("insert embedding: %w", err)
	}

	return nil
}

// GetMemory retrieves a memory by ID.
func (db *DB) GetMemory(ctx context.Context, id int64) (*Memory, error) {
	var m Memory
	var metaJSON []byte

	err := db.pool.QueryRow(ctx, `
		SELECT id, tenant_id, workspace_id, kind, text, source, created_at, updated_at, tags, importance, ttl_days, meta
		FROM memories
		WHERE id = $1 AND tenant_id = $2 AND workspace_id = $3
	`, id, db.tenantID, db.workspaceID).Scan(
		&m.ID, &m.TenantID, &m.WorkspaceID, &m.Kind, &m.Text, &m.Source,
		&m.CreatedAt, &m.UpdatedAt, &m.Tags, &m.Importance, &m.TTLDays, &metaJSON,
	)

	if err == pgx.ErrNoRows {
		return nil, nil
	}
	if err != nil {
		return nil, fmt.Errorf("query memory: %w", err)
	}

	if len(metaJSON) > 0 {
		if err := json.Unmarshal(metaJSON, &m.Meta); err != nil {
			return nil, fmt.Errorf("unmarshal meta: %w", err)
		}
	}

	return &m, nil
}

// UpdateMemoryParams contains parameters for updating a memory.
type UpdateMemoryParams struct {
	Kind       *string
	Text       *string
	Source     *string
	Tags       []string
	Importance *float32
	TTLDays    *int
	Meta       map[string]any
}

// UpdateMemory updates an existing memory.
func (db *DB) UpdateMemory(ctx context.Context, id int64, params UpdateMemoryParams) error {
	// Build dynamic update query
	setClauses := []string{"updated_at = now()"}
	args := []any{id, db.tenantID, db.workspaceID}
	argIdx := 4

	if params.Kind != nil {
		setClauses = append(setClauses, fmt.Sprintf("kind = $%d", argIdx))
		args = append(args, *params.Kind)
		argIdx++
	}
	if params.Text != nil {
		setClauses = append(setClauses, fmt.Sprintf("text = $%d", argIdx))
		args = append(args, *params.Text)
		argIdx++
	}
	if params.Source != nil {
		setClauses = append(setClauses, fmt.Sprintf("source = $%d", argIdx))
		args = append(args, *params.Source)
		argIdx++
	}
	if params.Tags != nil {
		setClauses = append(setClauses, fmt.Sprintf("tags = $%d", argIdx))
		args = append(args, params.Tags)
		argIdx++
	}
	if params.Importance != nil {
		setClauses = append(setClauses, fmt.Sprintf("importance = $%d", argIdx))
		args = append(args, *params.Importance)
		argIdx++
	}
	if params.TTLDays != nil {
		setClauses = append(setClauses, fmt.Sprintf("ttl_days = $%d", argIdx))
		args = append(args, *params.TTLDays)
		argIdx++
	}
	if params.Meta != nil {
		metaJSON, err := json.Marshal(params.Meta)
		if err != nil {
			return fmt.Errorf("marshal meta: %w", err)
		}
		setClauses = append(setClauses, fmt.Sprintf("meta = $%d", argIdx))
		args = append(args, metaJSON)
	}

	query := fmt.Sprintf(`
		UPDATE memories SET %s
		WHERE id = $1 AND tenant_id = $2 AND workspace_id = $3
	`, joinStrings(setClauses, ", "))

	result, err := db.pool.Exec(ctx, query, args...)
	if err != nil {
		return fmt.Errorf("update memory: %w", err)
	}

	if result.RowsAffected() == 0 {
		return fmt.Errorf("memory not found")
	}

	return nil
}

// DeleteMemory removes a memory by ID.
func (db *DB) DeleteMemory(ctx context.Context, id int64) error {
	result, err := db.pool.Exec(ctx, `
		DELETE FROM memories WHERE id = $1 AND tenant_id = $2 AND workspace_id = $3
	`, id, db.tenantID, db.workspaceID)

	if err != nil {
		return fmt.Errorf("delete memory: %w", err)
	}

	if result.RowsAffected() == 0 {
		return fmt.Errorf("memory not found")
	}

	return nil
}

// VectorSearchParams contains parameters for vector similarity search.
type VectorSearchParams struct {
	Embedding []float32
	Limit     int
}

// VectorSearch performs vector similarity search using cosine distance.
func (db *DB) VectorSearch(ctx context.Context, params VectorSearchParams) ([]MemoryWithScore, error) {
	if params.Limit <= 0 {
		params.Limit = 10
	}

	vec := pgvector.NewVector(params.Embedding)

	rows, err := db.pool.Query(ctx, `
		SELECT
			m.id, m.tenant_id, m.workspace_id, m.kind, m.text, m.source,
			m.created_at, m.updated_at, m.tags, m.importance, m.ttl_days, m.meta,
			1 - (e.embedding <=> $1) AS score
		FROM memories m
		JOIN memory_embeddings e ON m.id = e.memory_id
		WHERE m.tenant_id = $2 AND m.workspace_id = $3
		ORDER BY e.embedding <=> $1
		LIMIT $4
	`, vec, db.tenantID, db.workspaceID, params.Limit)

	if err != nil {
		return nil, fmt.Errorf("vector search: %w", err)
	}
	defer rows.Close()

	return scanMemoriesWithScore(rows)
}

// LexicalSearchParams contains parameters for lexical (trigram) search.
type LexicalSearchParams struct {
	Query string
	Limit int
}

// LexicalSearch performs trigram-based text similarity search.
func (db *DB) LexicalSearch(ctx context.Context, params LexicalSearchParams) ([]MemoryWithScore, error) {
	if params.Limit <= 0 {
		params.Limit = 10
	}

	rows, err := db.pool.Query(ctx, `
		SELECT
			m.id, m.tenant_id, m.workspace_id, m.kind, m.text, m.source,
			m.created_at, m.updated_at, m.tags, m.importance, m.ttl_days, m.meta,
			similarity(m.text, $1) AS score
		FROM memories m
		WHERE m.tenant_id = $2 AND m.workspace_id = $3 AND m.text % $1
		ORDER BY score DESC
		LIMIT $4
	`, params.Query, db.tenantID, db.workspaceID, params.Limit)

	if err != nil {
		return nil, fmt.Errorf("lexical search: %w", err)
	}
	defer rows.Close()

	return scanMemoriesWithScore(rows)
}

func scanMemoriesWithScore(rows pgx.Rows) ([]MemoryWithScore, error) {
	var results []MemoryWithScore

	for rows.Next() {
		var m MemoryWithScore
		var metaJSON []byte

		err := rows.Scan(
			&m.ID, &m.TenantID, &m.WorkspaceID, &m.Kind, &m.Text, &m.Source,
			&m.CreatedAt, &m.UpdatedAt, &m.Tags, &m.Importance, &m.TTLDays, &metaJSON,
			&m.Score,
		)
		if err != nil {
			return nil, fmt.Errorf("scan row: %w", err)
		}

		if len(metaJSON) > 0 {
			if err := json.Unmarshal(metaJSON, &m.Meta); err != nil {
				return nil, fmt.Errorf("unmarshal meta: %w", err)
			}
		}

		results = append(results, m)
	}

	if err := rows.Err(); err != nil {
		return nil, fmt.Errorf("iterate rows: %w", err)
	}

	return results, nil
}

func joinStrings(strs []string, sep string) string {
	if len(strs) == 0 {
		return ""
	}
	result := strs[0]
	for _, s := range strs[1:] {
		result += sep + s
	}
	return result
}

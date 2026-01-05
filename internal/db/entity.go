package db

import (
	"context"
	"encoding/json"
	"fmt"
	"time"

	"github.com/jackc/pgx/v5"
)

// EntityType represents the category of an entity.
type EntityType string

const (
	EntityTypePerson       EntityType = "person"
	EntityTypeOrganization EntityType = "organization"
	EntityTypeLocation     EntityType = "location"
	EntityTypeConcept      EntityType = "concept"
	EntityTypeTechnology   EntityType = "technology"
	EntityTypeProject      EntityType = "project"
	EntityTypeEvent        EntityType = "event"
	EntityTypeOther        EntityType = "other"
)

// Entity represents a stored entity record.
type Entity struct {
	ID          int64          `json:"id"`
	TenantID    string         `json:"tenant_id"`
	WorkspaceID string         `json:"workspace_id"`
	Name        string         `json:"name"`
	Type        EntityType     `json:"type"`
	Aliases     []string       `json:"aliases"`
	Description *string        `json:"description,omitempty"`
	Meta        map[string]any `json:"meta,omitempty"`
	CreatedAt   time.Time      `json:"created_at"`
	UpdatedAt   time.Time      `json:"updated_at"`
}

// MemoryEntity represents a link between a memory and an entity.
type MemoryEntity struct {
	MemoryID   int64   `json:"memory_id"`
	EntityID   int64   `json:"entity_id"`
	Role       *string `json:"role,omitempty"`
	Confidence float32 `json:"confidence"`
}

// EntityRelation represents a relationship between two entities.
type EntityRelation struct {
	ID           int64          `json:"id"`
	TenantID     string         `json:"tenant_id"`
	WorkspaceID  string         `json:"workspace_id"`
	SourceID     int64          `json:"source_id"`
	TargetID     int64          `json:"target_id"`
	RelationType string         `json:"relation_type"`
	Meta         map[string]any `json:"meta,omitempty"`
	CreatedAt    time.Time      `json:"created_at"`
}

// AddEntityParams contains parameters for adding a new entity.
type AddEntityParams struct {
	Name        string
	Type        EntityType
	Aliases     []string
	Description *string
	Meta        map[string]any
}

// AddEntity inserts a new entity or returns existing one if duplicate.
// Uses UPSERT to handle concurrent inserts gracefully.
func (db *DB) AddEntity(ctx context.Context, params AddEntityParams) (int64, error) {
	if params.Aliases == nil {
		params.Aliases = []string{}
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
		INSERT INTO entities (tenant_id, workspace_id, name, type, aliases, description, meta)
		VALUES ($1, $2, $3, $4, $5, $6, $7)
		ON CONFLICT (tenant_id, workspace_id, name, type) DO UPDATE SET
			aliases = CASE
				WHEN entities.aliases @> EXCLUDED.aliases THEN entities.aliases
				ELSE entities.aliases || EXCLUDED.aliases
			END,
			description = COALESCE(EXCLUDED.description, entities.description),
			updated_at = now()
		RETURNING id
	`, db.tenantID, db.workspaceID, params.Name, params.Type, params.Aliases, params.Description, metaJSON).Scan(&id)

	if err != nil {
		return 0, fmt.Errorf("insert entity: %w", err)
	}

	return id, nil
}

// GetEntity retrieves an entity by ID.
func (db *DB) GetEntity(ctx context.Context, id int64) (*Entity, error) {
	var e Entity
	var metaJSON []byte

	err := db.pool.QueryRow(ctx, `
		SELECT id, tenant_id, workspace_id, name, type, aliases, description, meta, created_at, updated_at
		FROM entities
		WHERE id = $1 AND tenant_id = $2 AND workspace_id = $3
	`, id, db.tenantID, db.workspaceID).Scan(
		&e.ID, &e.TenantID, &e.WorkspaceID, &e.Name, &e.Type,
		&e.Aliases, &e.Description, &metaJSON, &e.CreatedAt, &e.UpdatedAt,
	)

	if err == pgx.ErrNoRows {
		return nil, nil
	}
	if err != nil {
		return nil, fmt.Errorf("query entity: %w", err)
	}

	if len(metaJSON) > 0 {
		if err := json.Unmarshal(metaJSON, &e.Meta); err != nil {
			return nil, fmt.Errorf("unmarshal meta: %w", err)
		}
	}

	return &e, nil
}

// FindEntityByName finds an entity by name and type.
func (db *DB) FindEntityByName(ctx context.Context, name string, entityType EntityType) (*Entity, error) {
	var e Entity
	var metaJSON []byte

	err := db.pool.QueryRow(ctx, `
		SELECT id, tenant_id, workspace_id, name, type, aliases, description, meta, created_at, updated_at
		FROM entities
		WHERE tenant_id = $1 AND workspace_id = $2 AND name = $3 AND type = $4
	`, db.tenantID, db.workspaceID, name, entityType).Scan(
		&e.ID, &e.TenantID, &e.WorkspaceID, &e.Name, &e.Type,
		&e.Aliases, &e.Description, &metaJSON, &e.CreatedAt, &e.UpdatedAt,
	)

	if err == pgx.ErrNoRows {
		return nil, nil
	}
	if err != nil {
		return nil, fmt.Errorf("query entity by name: %w", err)
	}

	if len(metaJSON) > 0 {
		if err := json.Unmarshal(metaJSON, &e.Meta); err != nil {
			return nil, fmt.Errorf("unmarshal meta: %w", err)
		}
	}

	return &e, nil
}

// LinkMemoryEntity creates a link between a memory and an entity.
func (db *DB) LinkMemoryEntity(ctx context.Context, memoryID, entityID int64, role *string, confidence float32) error {
	_, err := db.pool.Exec(ctx, `
		INSERT INTO memory_entities (memory_id, entity_id, role, confidence)
		VALUES ($1, $2, $3, $4)
		ON CONFLICT (memory_id, entity_id) DO UPDATE SET
			role = COALESCE(EXCLUDED.role, memory_entities.role),
			confidence = EXCLUDED.confidence
	`, memoryID, entityID, role, confidence)

	if err != nil {
		return fmt.Errorf("link memory entity: %w", err)
	}

	return nil
}

// GetMemoryEntities retrieves all entities linked to a memory.
func (db *DB) GetMemoryEntities(ctx context.Context, memoryID int64) ([]Entity, error) {
	rows, err := db.pool.Query(ctx, `
		SELECT e.id, e.tenant_id, e.workspace_id, e.name, e.type, e.aliases, e.description, e.meta, e.created_at, e.updated_at
		FROM entities e
		JOIN memory_entities me ON e.id = me.entity_id
		WHERE me.memory_id = $1 AND e.tenant_id = $2 AND e.workspace_id = $3
		ORDER BY me.confidence DESC, e.name ASC
	`, memoryID, db.tenantID, db.workspaceID)
	if err != nil {
		return nil, fmt.Errorf("query memory entities: %w", err)
	}
	defer rows.Close()

	return scanEntities(rows)
}

// GetEntityMemories retrieves all memory IDs linked to an entity.
func (db *DB) GetEntityMemories(ctx context.Context, entityID int64) ([]int64, error) {
	rows, err := db.pool.Query(ctx, `
		SELECT me.memory_id
		FROM memory_entities me
		JOIN memories m ON me.memory_id = m.id
		WHERE me.entity_id = $1 AND m.tenant_id = $2 AND m.workspace_id = $3
		ORDER BY m.updated_at DESC
	`, entityID, db.tenantID, db.workspaceID)
	if err != nil {
		return nil, fmt.Errorf("query entity memories: %w", err)
	}
	defer rows.Close()

	var ids []int64
	for rows.Next() {
		var id int64
		if err := rows.Scan(&id); err != nil {
			return nil, fmt.Errorf("scan memory id: %w", err)
		}
		ids = append(ids, id)
	}

	return ids, rows.Err()
}

// AddEntityRelation creates a relationship between two entities.
func (db *DB) AddEntityRelation(ctx context.Context, sourceID, targetID int64, relationType string) error {
	_, err := db.pool.Exec(ctx, `
		INSERT INTO entity_relations (tenant_id, workspace_id, source_id, target_id, relation_type)
		VALUES ($1, $2, $3, $4, $5)
		ON CONFLICT (tenant_id, workspace_id, source_id, target_id, relation_type) DO NOTHING
	`, db.tenantID, db.workspaceID, sourceID, targetID, relationType)

	if err != nil {
		return fmt.Errorf("add entity relation: %w", err)
	}

	return nil
}

// GetRelatedEntities retrieves entities related to the given entity.
func (db *DB) GetRelatedEntities(ctx context.Context, entityID int64) ([]Entity, error) {
	rows, err := db.pool.Query(ctx, `
		SELECT DISTINCT e.id, e.tenant_id, e.workspace_id, e.name, e.type, e.aliases, e.description, e.meta, e.created_at, e.updated_at
		FROM entities e
		JOIN entity_relations er ON (e.id = er.target_id OR e.id = er.source_id)
		WHERE (er.source_id = $1 OR er.target_id = $1)
			AND e.id != $1
			AND e.tenant_id = $2 AND e.workspace_id = $3
		ORDER BY e.name ASC
	`, entityID, db.tenantID, db.workspaceID)
	if err != nil {
		return nil, fmt.Errorf("query related entities: %w", err)
	}
	defer rows.Close()

	return scanEntities(rows)
}

// SearchEntities searches for entities by name (using trigram similarity).
func (db *DB) SearchEntities(ctx context.Context, query string, limit int) ([]Entity, error) {
	if limit <= 0 {
		limit = 10
	}

	rows, err := db.pool.Query(ctx, `
		SELECT id, tenant_id, workspace_id, name, type, aliases, description, meta, created_at, updated_at
		FROM entities
		WHERE tenant_id = $1 AND workspace_id = $2 AND name % $3
		ORDER BY similarity(name, $3) DESC
		LIMIT $4
	`, db.tenantID, db.workspaceID, query, limit)
	if err != nil {
		return nil, fmt.Errorf("search entities: %w", err)
	}
	defer rows.Close()

	return scanEntities(rows)
}

// ListEntitiesByType lists all entities of a specific type.
func (db *DB) ListEntitiesByType(ctx context.Context, entityType EntityType, limit int) ([]Entity, error) {
	if limit <= 0 {
		limit = 100
	}

	rows, err := db.pool.Query(ctx, `
		SELECT id, tenant_id, workspace_id, name, type, aliases, description, meta, created_at, updated_at
		FROM entities
		WHERE tenant_id = $1 AND workspace_id = $2 AND type = $3
		ORDER BY name ASC
		LIMIT $4
	`, db.tenantID, db.workspaceID, entityType, limit)
	if err != nil {
		return nil, fmt.Errorf("list entities by type: %w", err)
	}
	defer rows.Close()

	return scanEntities(rows)
}

// GetRelatedMemories finds memories that share entities with the given memory.
func (db *DB) GetRelatedMemories(ctx context.Context, memoryID int64, limit int) ([]MemoryWithScore, error) {
	if limit <= 0 {
		limit = 10
	}

	rows, err := db.pool.Query(ctx, `
		SELECT DISTINCT ON (m.id)
			m.id, m.tenant_id, m.workspace_id, m.kind, m.text, m.source,
			m.created_at, m.updated_at, m.tags, m.importance, m.ttl_days, m.meta,
			COUNT(DISTINCT me2.entity_id)::real / GREATEST(COUNT(DISTINCT me1.entity_id)::real, 1) AS score
		FROM memories m
		JOIN memory_entities me2 ON m.id = me2.memory_id
		JOIN memory_entities me1 ON me1.entity_id = me2.entity_id AND me1.memory_id = $1
		WHERE m.id != $1 AND m.tenant_id = $2 AND m.workspace_id = $3
		GROUP BY m.id
		ORDER BY m.id, score DESC
		LIMIT $4
	`, memoryID, db.tenantID, db.workspaceID, limit)
	if err != nil {
		return nil, fmt.Errorf("get related memories: %w", err)
	}
	defer rows.Close()

	return scanMemoriesWithScore(rows)
}

func scanEntities(rows pgx.Rows) ([]Entity, error) {
	var entities []Entity
	for rows.Next() {
		var e Entity
		var metaJSON []byte
		if err := rows.Scan(
			&e.ID, &e.TenantID, &e.WorkspaceID, &e.Name, &e.Type,
			&e.Aliases, &e.Description, &metaJSON, &e.CreatedAt, &e.UpdatedAt,
		); err != nil {
			return nil, fmt.Errorf("scan entity: %w", err)
		}
		if len(metaJSON) > 0 {
			if err := json.Unmarshal(metaJSON, &e.Meta); err != nil {
				return nil, fmt.Errorf("unmarshal meta: %w", err)
			}
		}
		entities = append(entities, e)
	}
	return entities, rows.Err()
}

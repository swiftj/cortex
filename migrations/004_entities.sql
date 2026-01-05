-- Migration 004: Entity extraction support
-- Enables light knowledge graph with entities extracted from memories

-- Entity types for categorization
CREATE TYPE entity_type AS ENUM (
    'person',      -- People, names
    'organization', -- Companies, teams, groups
    'location',    -- Places, addresses
    'concept',     -- Abstract ideas, topics
    'technology',  -- Tech stack, tools, frameworks
    'project',     -- Project names, codenames
    'event',       -- Meetings, deadlines, milestones
    'other'        -- Catch-all for uncategorized
);

-- Entities table - stores unique entities across all memories
CREATE TABLE IF NOT EXISTS entities (
    id          BIGSERIAL PRIMARY KEY,
    tenant_id   TEXT NOT NULL DEFAULT 'local',
    workspace_id TEXT NOT NULL DEFAULT 'default',
    name        TEXT NOT NULL,              -- Canonical name of the entity
    type        entity_type NOT NULL,       -- Category of entity
    aliases     TEXT[] DEFAULT '{}',        -- Alternative names/spellings
    description TEXT,                       -- Brief description if available
    meta        JSONB DEFAULT '{}'::jsonb,  -- Additional metadata
    created_at  TIMESTAMPTZ NOT NULL DEFAULT now(),
    updated_at  TIMESTAMPTZ NOT NULL DEFAULT now(),

    -- Unique constraint per tenant/workspace/name/type combination
    CONSTRAINT entities_unique UNIQUE (tenant_id, workspace_id, name, type)
);

-- Memory-Entity junction table for many-to-many relationships
CREATE TABLE IF NOT EXISTS memory_entities (
    memory_id   BIGINT NOT NULL REFERENCES memories(id) ON DELETE CASCADE,
    entity_id   BIGINT NOT NULL REFERENCES entities(id) ON DELETE CASCADE,
    role        TEXT,                       -- Role of entity in memory (e.g., 'subject', 'mentioned')
    confidence  REAL DEFAULT 1.0,           -- Extraction confidence (0-1)
    created_at  TIMESTAMPTZ NOT NULL DEFAULT now(),

    PRIMARY KEY (memory_id, entity_id)
);

-- Entity-Entity relationships for knowledge graph
CREATE TABLE IF NOT EXISTS entity_relations (
    id              BIGSERIAL PRIMARY KEY,
    tenant_id       TEXT NOT NULL DEFAULT 'local',
    workspace_id    TEXT NOT NULL DEFAULT 'default',
    source_id       BIGINT NOT NULL REFERENCES entities(id) ON DELETE CASCADE,
    target_id       BIGINT NOT NULL REFERENCES entities(id) ON DELETE CASCADE,
    relation_type   TEXT NOT NULL,          -- e.g., 'works_at', 'created_by', 'part_of'
    meta            JSONB DEFAULT '{}'::jsonb,
    created_at      TIMESTAMPTZ NOT NULL DEFAULT now(),

    -- Prevent duplicate relations
    CONSTRAINT entity_relations_unique UNIQUE (tenant_id, workspace_id, source_id, target_id, relation_type)
);

-- Indexes for efficient queries
CREATE INDEX IF NOT EXISTS idx_entities_tenant_workspace ON entities (tenant_id, workspace_id);
CREATE INDEX IF NOT EXISTS idx_entities_type ON entities (type);
CREATE INDEX IF NOT EXISTS idx_entities_name_trgm ON entities USING gin (name gin_trgm_ops);

CREATE INDEX IF NOT EXISTS idx_memory_entities_memory ON memory_entities (memory_id);
CREATE INDEX IF NOT EXISTS idx_memory_entities_entity ON memory_entities (entity_id);

CREATE INDEX IF NOT EXISTS idx_entity_relations_source ON entity_relations (source_id);
CREATE INDEX IF NOT EXISTS idx_entity_relations_target ON entity_relations (target_id);
CREATE INDEX IF NOT EXISTS idx_entity_relations_type ON entity_relations (relation_type);

-- Migration 001: Initial schema for Cortex memory server
-- Requires: pgvector, pg_trgm extensions

CREATE EXTENSION IF NOT EXISTS vector;
CREATE EXTENSION IF NOT EXISTS pg_trgm;

-- Main memories table
CREATE TABLE IF NOT EXISTS memories (
  id         BIGSERIAL PRIMARY KEY,
  tenant_id  TEXT NOT NULL DEFAULT 'local',
  kind       TEXT NOT NULL,          -- note|fact|todo|preference|identity|project|...
  text       TEXT NOT NULL,
  source     TEXT,                   -- "chat", "file:path", etc.
  created_at TIMESTAMPTZ NOT NULL DEFAULT now(),
  updated_at TIMESTAMPTZ NOT NULL DEFAULT now(),
  tags       TEXT[] DEFAULT '{}',
  importance REAL   DEFAULT 0.5,     -- 0..1 for ranking/retention
  ttl_days   INT,                    -- optional expiry policy
  meta       JSONB  DEFAULT '{}'::jsonb
);

-- Embeddings table (separate for flexibility with multiple models)
CREATE TABLE IF NOT EXISTS memory_embeddings (
  memory_id  BIGINT PRIMARY KEY REFERENCES memories(id) ON DELETE CASCADE,
  model      TEXT NOT NULL,     -- "text-embedding-3-large", "gemini-embedding-2", etc.
  dims       INT  NOT NULL,
  embedding  VECTOR NOT NULL
);

-- Text index for lexical similarity (trigram)
CREATE INDEX IF NOT EXISTS idx_memories_text_trgm
  ON memories USING gin (text gin_trgm_ops);

-- Tenant index for multi-tenant queries
CREATE INDEX IF NOT EXISTS idx_memories_tenant
  ON memories (tenant_id);

-- Kind index for filtering by memory type
CREATE INDEX IF NOT EXISTS idx_memories_kind
  ON memories (kind);

-- Tags index for filtering
CREATE INDEX IF NOT EXISTS idx_memories_tags
  ON memories USING gin (tags);

-- HNSW index for vector similarity (best speed/recall; more RAM)
-- Using cosine distance for normalized embeddings
CREATE INDEX IF NOT EXISTS idx_memory_embed_hnsw
  ON memory_embeddings USING hnsw (embedding vector_cosine_ops);

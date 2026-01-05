-- Migration 003: Multi-model embeddings support
-- Allows storing multiple embeddings per memory (one per model)

-- Drop the existing primary key constraint (memory_id only)
-- and add a composite primary key (memory_id, model)
ALTER TABLE memory_embeddings DROP CONSTRAINT IF EXISTS memory_embeddings_pkey;

-- Add composite primary key for multi-model support
ALTER TABLE memory_embeddings ADD PRIMARY KEY (memory_id, model);

-- Add created_at for tracking when embeddings were generated
ALTER TABLE memory_embeddings ADD COLUMN IF NOT EXISTS created_at TIMESTAMPTZ NOT NULL DEFAULT now();

-- Index on model for model-specific queries
CREATE INDEX IF NOT EXISTS idx_memory_embeddings_model ON memory_embeddings (model);

-- Migration 002: Add workspace namespacing for project-specific memories
-- A single tenant can have multiple workspaces to isolate memories by project.
-- Default workspace_id is 'default' for backward compatibility.

-- Add workspace_id column to memories table
ALTER TABLE memories ADD COLUMN IF NOT EXISTS workspace_id TEXT NOT NULL DEFAULT 'default';

-- Create composite index for efficient filtering by tenant and workspace
CREATE INDEX IF NOT EXISTS idx_memories_tenant_workspace ON memories(tenant_id, workspace_id);

-- Update the trigram index to include workspace for better lexical search performance
DROP INDEX IF EXISTS idx_memories_text_trgm;
CREATE INDEX IF NOT EXISTS idx_memories_text_trgm ON memories USING gin (text gin_trgm_ops);

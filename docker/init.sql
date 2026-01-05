-- Extensions are created automatically in pgvector/pgvector image
-- pgvector extension is pre-installed, just need pg_trgm for lexical search
CREATE EXTENSION IF NOT EXISTS pg_trgm;

-- Schema will be created by embedded migrations on first run

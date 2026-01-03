# Cortex

A local-first MCP (Model Context Protocol) memory server for Claude Code, written in Go.

Cortex provides persistent memory capabilities for AI agents, enabling them to store, search, and retrieve information across sessions using hybrid vector + lexical search.

## Features

- **MCP Native**: Exposes memory tools via JSON-RPC 2.0 over stdio for Claude Code integration
- **Hybrid Search**: Combines vector similarity (pgvector) with lexical matching (pg_trgm) for optimal recall
- **Pluggable LLMs**: Supports OpenAI and Google Gemini for embeddings and text normalization
- **Single Binary**: Pure Go, no CGO dependencies, compiles to a single static binary
- **Multi-tenant**: Supports isolated memory spaces via tenant IDs
- **PostgreSQL Backend**: Battle-tested storage with automatic schema migrations

## Quick Start

### Prerequisites

- Go 1.22+
- PostgreSQL 14+ with extensions:
  - `pgvector` - Vector similarity search
  - `pg_trgm` - Trigram text similarity

### Database Setup

```bash
# Create database
createdb cortex

# Enable required extensions
psql cortex -c 'CREATE EXTENSION IF NOT EXISTS vector;'
psql cortex -c 'CREATE EXTENSION IF NOT EXISTS pg_trgm;'
```

### Build

```bash
CGO_ENABLED=0 go build -o bin/cortex ./cmd/mcpserver
```

### Configure

Set environment variables:

```bash
# Required
export DATABASE_URL="postgres://localhost:5432/cortex?sslmode=disable"

# Optional (defaults shown)
export TENANT_ID="local"
export LM_BACKEND="openai"      # or "gemini"
export LM_MODEL="auto"          # chat model for normalization
export EMBED_MODEL="auto"       # embedding model

# API Keys (one required based on LM_BACKEND)
export OPENAI_API_KEY="sk-..."
# or
export GEMINI_API_KEY="..."
```

### Run

```bash
./bin/cortex
```

The server reads JSON-RPC requests from stdin and writes responses to stdout.

## Claude Code Integration

Add to your `.mcp.json` or Claude Code settings:

```json
{
  "mcpServers": {
    "cortex": {
      "type": "stdio",
      "command": "/path/to/cortex",
      "env": {
        "DATABASE_URL": "postgres://localhost:5432/cortex?sslmode=disable",
        "TENANT_ID": "local",
        "LM_BACKEND": "openai",
        "OPENAI_API_KEY": "${OPENAI_API_KEY}"
      }
    }
  }
}
```

Restart Claude Code and the memory tools will be available.

## MCP Tools

### `memory.add`

Store a new memory with optional metadata.

```json
{
  "text": "User prefers dark mode in all applications",
  "kind": "preference",
  "importance": 0.8,
  "tags": ["ui", "settings"],
  "source": "chat"
}
```

**Returns**: `{ "id": 123 }`

### `memory.search`

Search memories using hybrid vector + lexical matching.

```json
{
  "query": "user interface preferences",
  "k": 10,
  "hybrid": true
}
```

**Returns**: Array of memories with similarity scores.

### `memory.update`

Update an existing memory by ID.

```json
{
  "id": 123,
  "patch": {
    "importance": 0.9,
    "tags": ["ui", "settings", "theme"]
  }
}
```

### `memory.delete`

Remove a memory by ID.

```json
{
  "id": 123
}
```

## Architecture

```
cortex/
├── cmd/mcpserver/       # Entry point
├── internal/
│   ├── db/              # PostgreSQL operations (pgx)
│   ├── llm/             # LLM adapters (OpenAI, Gemini)
│   ├── mcp/             # MCP JSON-RPC server
│   └── search/          # Hybrid search & ranking
├── migrations/          # Embedded SQL migrations
└── configs/             # Example configurations
```

### Hybrid Search

Cortex combines two search strategies:

1. **Vector Search**: Embeds queries and finds semantically similar memories using cosine distance
2. **Lexical Search**: Uses PostgreSQL trigram similarity for exact/fuzzy text matching

Results are fused using a weighted combination:
```
final_score = α × vector_score + (1 - α) × lexical_score
```

Default `α = 0.7` (70% vector, 30% lexical).

### LLM Providers

| Provider | Chat Model (default) | Embedding Model (default) | Dimensions |
|----------|---------------------|---------------------------|------------|
| OpenAI | gpt-4o-mini | text-embedding-3-small | 1536 |
| Gemini | gemini-2.0-flash-lite | text-embedding-004 | 768 |

## Database Schema

```sql
-- Main memories table
CREATE TABLE memories (
  id         BIGSERIAL PRIMARY KEY,
  tenant_id  TEXT NOT NULL DEFAULT 'local',
  kind       TEXT NOT NULL,           -- note|fact|todo|preference|identity|...
  text       TEXT NOT NULL,
  source     TEXT,                    -- origin of the memory
  created_at TIMESTAMPTZ DEFAULT now(),
  updated_at TIMESTAMPTZ DEFAULT now(),
  tags       TEXT[] DEFAULT '{}',
  importance REAL DEFAULT 0.5,        -- 0..1 ranking weight
  ttl_days   INT,                     -- optional expiry
  meta       JSONB DEFAULT '{}'
);

-- Separate embeddings table (supports model switching)
CREATE TABLE memory_embeddings (
  memory_id  BIGINT PRIMARY KEY REFERENCES memories(id) ON DELETE CASCADE,
  model      TEXT NOT NULL,
  dims       INT NOT NULL,
  embedding  VECTOR NOT NULL
);
```

## Development

```bash
# Run tests
go test ./...

# Build with optimizations
CGO_ENABLED=0 go build -trimpath -ldflags "-s -w" -o bin/cortex ./cmd/mcpserver

# Check binary size
ls -lh bin/cortex
```

## Roadmap

- [ ] TTL sweeper for automatic memory expiry
- [ ] Batch re-embedding when switching models
- [ ] Export/import (JSONL format)
- [ ] Entity extraction and relationship tracking
- [ ] Multi-model embeddings (store multiple vectors per memory)
- [ ] Workspace namespacing for project-specific memories

## Inspiration

Cortex is inspired by [Mem0](https://github.com/mem0ai/mem0), adapted for the MCP ecosystem and Claude Code workflows.

## License

MIT

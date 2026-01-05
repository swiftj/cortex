# Cortex: Complete Guide for Claude Code Integration

A comprehensive guide to installing, configuring, and using Cortex as a persistent memory layer for Claude Code in software development workflows.

## Table of Contents

1. [Overview](#overview)
2. [Installation](#installation)
3. [Configuration](#configuration)
4. [Claude Code Integration](#claude-code-integration)
5. [MCP Tools Reference](#mcp-tools-reference)
6. [CLI Operations](#cli-operations)
7. [Software Development Workflows](#software-development-workflows)
8. [Advanced Features](#advanced-features)
9. [Troubleshooting](#troubleshooting)

---

## Overview

Cortex is a local-first MCP (Model Context Protocol) memory server that provides persistent memory capabilities for Claude Code. It enables AI agents to:

- **Store** facts, preferences, notes, todos, and project-specific information
- **Search** using hybrid vector + lexical retrieval for optimal recall
- **Recall** relevant context across sessions automatically
- **Extract** entities and build knowledge graphs from conversations

### Architecture

```
Claude Code ←→ MCP (JSON-RPC/stdio) ←→ Cortex ←→ PostgreSQL + pgvector
                                         ↓
                                    LLM Provider
                                   (OpenAI/Gemini)
```

### Key Features

| Feature | Description |
|---------|-------------|
| **Hybrid Search** | Combines vector similarity (semantic) with lexical matching (exact terms) |
| **Multi-tenant** | Isolated memory spaces via tenant and workspace IDs |
| **Entity Extraction** | Optional LLM-based extraction of people, organizations, technologies |
| **TTL Support** | Automatic memory expiry for temporary information |
| **Export/Import** | JSONL format for backup and migration |
| **Multi-model Embeddings** | Store embeddings from multiple models simultaneously |

---

## Installation

### Option 1: Docker (Recommended)

Docker provides the simplest setup with PostgreSQL and pgvector pre-configured.

```bash
# Clone the repository
git clone https://github.com/johnswift/cortex.git
cd cortex

# Create environment file
cat > .env << 'EOF'
OPENAI_API_KEY=sk-your-api-key-here
# Or for Gemini:
# GEMINI_API_KEY=your-gemini-key
# LM_BACKEND=gemini
EOF

# Start services
docker-compose up -d

# Verify services are running
docker-compose ps

# View logs
docker-compose logs -f cortex
```

#### Docker Compose Services

| Service | Port | Description |
|---------|------|-------------|
| `cortex` | - | MCP server (stdio mode) |
| `postgres` | 5432 | PostgreSQL with pgvector |

### Option 2: Manual Installation

#### Prerequisites

- **Go** 1.22 or later
- **PostgreSQL** 14+ with extensions:
  - `pgvector` - Vector similarity search
  - `pg_trgm` - Trigram text similarity

#### Step 1: Database Setup

```bash
# Create database
createdb cortex

# Enable required extensions
psql cortex << 'EOF'
CREATE EXTENSION IF NOT EXISTS vector;
CREATE EXTENSION IF NOT EXISTS pg_trgm;
EOF
```

#### Step 2: Build Cortex

```bash
# Clone repository
git clone https://github.com/johnswift/cortex.git
cd cortex

# Build binary (no CGO required)
CGO_ENABLED=0 go build -o bin/cortex ./cmd/mcpserver

# Verify binary
./bin/cortex --help
```

#### Step 3: Configure Environment

```bash
# Required
export DATABASE_URL="postgres://localhost:5432/cortex?sslmode=disable"
export OPENAI_API_KEY="sk-your-api-key"

# Optional (with defaults)
export TENANT_ID="local"
export WORKSPACE_ID="default"
export LM_BACKEND="openai"
```

#### Step 4: Run Migrations

Migrations run automatically on first startup, or you can trigger them:

```bash
# Start the server (migrations run automatically)
./bin/cortex
```

---

## Configuration

### Environment Variables

| Variable | Required | Default | Description |
|----------|----------|---------|-------------|
| `DATABASE_URL` | Yes | - | PostgreSQL connection string |
| `OPENAI_API_KEY` | If OpenAI | - | OpenAI API key |
| `GEMINI_API_KEY` | If Gemini | - | Google Gemini API key |
| `TENANT_ID` | No | `local` | Tenant identifier for multi-tenancy |
| `WORKSPACE_ID` | No | `default` | Workspace for project isolation |
| `LM_BACKEND` | No | `openai` | LLM provider: `openai` or `gemini` |
| `LM_MODEL` | No | `auto` | Chat model for text normalization |
| `EMBED_MODEL` | No | `auto` | Embedding model |
| `EMBED_MODELS` | No | - | Comma-separated list for multi-model embeddings |
| `SWEEPER_ENABLED` | No | `true` | Enable TTL-based memory cleanup |
| `SWEEPER_INTERVAL` | No | `1h` | How often to run TTL sweeper |
| `HEALTH_PORT` | No | - | HTTP port for health checks (e.g., `8080`) |
| `ENTITY_EXTRACTION` | No | `false` | Enable LLM-based entity extraction |

### LLM Provider Defaults

| Provider | Chat Model | Embedding Model | Dimensions |
|----------|------------|-----------------|------------|
| OpenAI | `gpt-4o-mini` | `text-embedding-3-small` | 1536 |
| Gemini | `gemini-2.0-flash-lite` | `text-embedding-004` | 768 |

### Workspace Namespacing

Use `WORKSPACE_ID` to isolate memories per project:

```bash
# Project A
WORKSPACE_ID="project-a" ./bin/cortex

# Project B
WORKSPACE_ID="project-b" ./bin/cortex
```

Each workspace has completely isolated memories, entities, and embeddings.

---

## Claude Code Integration

### Basic Configuration

Create or edit `.mcp.json` in your project root:

```json
{
  "mcpServers": {
    "cortex": {
      "type": "stdio",
      "command": "/path/to/cortex",
      "env": {
        "DATABASE_URL": "postgres://localhost:5432/cortex?sslmode=disable",
        "TENANT_ID": "local",
        "WORKSPACE_ID": "my-project",
        "LM_BACKEND": "openai",
        "OPENAI_API_KEY": "${OPENAI_API_KEY}"
      }
    }
  }
}
```

### Docker Configuration

```json
{
  "mcpServers": {
    "cortex": {
      "type": "stdio",
      "command": "docker-compose",
      "args": ["-f", "/path/to/cortex/docker-compose.yml", "exec", "-T", "cortex", "/usr/local/bin/cortex"],
      "env": {
        "WORKSPACE_ID": "my-project",
        "OPENAI_API_KEY": "${OPENAI_API_KEY}"
      }
    }
  }
}
```

### Per-Project Workspaces

Configure different workspaces for different projects:

```json
{
  "mcpServers": {
    "cortex": {
      "type": "stdio",
      "command": "/path/to/cortex",
      "env": {
        "DATABASE_URL": "postgres://localhost:5432/cortex?sslmode=disable",
        "WORKSPACE_ID": "${PWD##*/}",
        "OPENAI_API_KEY": "${OPENAI_API_KEY}"
      },
      "scope": ["${PWD}"]
    }
  }
}
```

### Verification

After configuring, restart Claude Code and verify the tools are available:

1. Open Claude Code
2. Type: "What memory tools do you have available?"
3. Claude should list: `memory.add`, `memory.search`, `memory.update`, `memory.delete`, `memory.export`, `memory.import`

---

## MCP Tools Reference

### memory.add

Store a new memory with optional metadata and automatic embedding generation.

**Parameters:**

| Parameter | Type | Required | Default | Description |
|-----------|------|----------|---------|-------------|
| `text` | string | Yes | - | The memory content to store |
| `kind` | string | No | `note` | Type: `note`, `fact`, `todo`, `preference`, `identity`, `project` |
| `importance` | number | No | `0.5` | Priority score (0.0 - 1.0) |
| `tags` | array | No | `[]` | Categorization tags |
| `ttl_days` | integer | No | - | Days until auto-expiry |
| `source` | string | No | - | Origin identifier |

**Example:**

```json
{
  "text": "User prefers TypeScript over JavaScript for new projects",
  "kind": "preference",
  "importance": 0.8,
  "tags": ["coding", "typescript", "preferences"],
  "source": "conversation"
}
```

**Returns:** `{ "id": 123 }`

### memory.search

Search memories using hybrid vector + lexical matching.

**Parameters:**

| Parameter | Type | Required | Default | Description |
|-----------|------|----------|---------|-------------|
| `query` | string | Yes | - | Search query |
| `k` | integer | No | `10` | Max results (1-100) |
| `hybrid` | boolean | No | `true` | Use hybrid search |
| `model` | string | No | - | Filter by embedding model |

**Example:**

```json
{
  "query": "user coding preferences and style",
  "k": 5,
  "hybrid": true
}
```

**Returns:**

```json
[
  {
    "id": 123,
    "text": "User prefers TypeScript over JavaScript...",
    "score": 0.92,
    "importance": 0.8,
    "tags": ["coding", "typescript"],
    "source": "conversation"
  }
]
```

### memory.update

Update an existing memory by ID.

**Parameters:**

| Parameter | Type | Required | Description |
|-----------|------|----------|-------------|
| `id` | integer | Yes | Memory ID to update |
| `patch` | object | Yes | Fields to update |

**Patch fields:** `text`, `kind`, `importance`, `tags`, `ttl_days`, `source`

**Example:**

```json
{
  "id": 123,
  "patch": {
    "importance": 0.9,
    "tags": ["coding", "typescript", "critical"]
  }
}
```

### memory.delete

Delete a memory by ID.

**Parameters:**

| Parameter | Type | Required | Description |
|-----------|------|----------|-------------|
| `id` | integer | Yes | Memory ID to delete |

### memory.export

Export memories to JSONL format.

**Parameters:**

| Parameter | Type | Required | Default | Description |
|-----------|------|----------|---------|-------------|
| `include_embeddings` | boolean | No | `false` | Include vector embeddings |
| `kind` | string | No | - | Filter by memory type |
| `limit` | integer | No | - | Max memories to export |

### memory.import

Import memories from JSONL format.

**Parameters:**

| Parameter | Type | Required | Default | Description |
|-----------|------|----------|---------|-------------|
| `data` | string | Yes | - | JSONL data to import |
| `skip_existing` | boolean | No | `false` | Skip duplicates |
| `regenerate_embeddings` | boolean | No | `false` | Generate new embeddings |
| `dry_run` | boolean | No | `false` | Validate without saving |

### memory.entities (Entity Extraction)

Get entities extracted from a specific memory.

**Parameters:**

| Parameter | Type | Required | Description |
|-----------|------|----------|-------------|
| `memory_id` | integer | Yes | Memory ID |

**Returns:**

```json
{
  "entities": [
    {
      "id": 1,
      "name": "TypeScript",
      "type": "technology",
      "description": "Programming language"
    }
  ]
}
```

### memory.related (Entity Extraction)

Find memories that share entities with a given memory.

**Parameters:**

| Parameter | Type | Required | Default | Description |
|-----------|------|----------|---------|-------------|
| `memory_id` | integer | Yes | - | Source memory ID |
| `k` | integer | No | `10` | Max related memories |

---

## CLI Operations

### Export Memories

```bash
# Export all memories
./bin/cortex --export backup.jsonl

# Export with embeddings (larger file)
./bin/cortex --export backup.jsonl --with-embeddings

# Export specific workspace
WORKSPACE_ID=my-project ./bin/cortex --export project-backup.jsonl
```

### Import Memories

```bash
# Import memories
./bin/cortex --import backup.jsonl

# Skip existing (by ID)
./bin/cortex --import backup.jsonl --skip-existing

# Regenerate embeddings with current model
./bin/cortex --import backup.jsonl --regenerate-embeddings

# Validate without writing
./bin/cortex --import backup.jsonl --dry-run
```

### Re-embed Memories

When switching embedding models, re-embed all memories:

```bash
# Re-embed with current model
./bin/cortex --reembed

# Custom batch size and delay
./bin/cortex --reembed --reembed-batch-size 50 --reembed-delay 200ms

# Delete old embeddings after re-embedding
./bin/cortex --reembed --reembed-delete-old
```

---

## Software Development Workflows

### Workflow 1: Project Context Persistence

Store project-specific information that Claude should remember:

```
User: "Remember that this project uses Prisma for database access and Jest for testing"

Claude uses memory.add:
{
  "text": "Project uses Prisma ORM for database access and Jest testing framework",
  "kind": "project",
  "importance": 0.9,
  "tags": ["architecture", "prisma", "jest", "testing"]
}
```

Later sessions automatically recall this context via `memory.search`.

### Workflow 2: User Preferences

Track coding style and preferences:

```
User: "I prefer functional components with hooks over class components"

Claude stores:
{
  "text": "User prefers React functional components with hooks over class components",
  "kind": "preference",
  "importance": 0.8,
  "tags": ["react", "coding-style", "frontend"]
}
```

### Workflow 3: Technical Decisions

Document architectural decisions:

```
User: "We decided to use Redis for session storage because of scalability requirements"

Claude stores:
{
  "text": "Architecture decision: Using Redis for session storage due to horizontal scalability requirements",
  "kind": "fact",
  "importance": 0.9,
  "tags": ["architecture", "redis", "sessions", "decision"],
  "source": "architecture-review"
}
```

### Workflow 4: Task Tracking

Track todos with automatic expiry:

```
{
  "text": "TODO: Refactor authentication module to use JWT instead of sessions",
  "kind": "todo",
  "importance": 0.7,
  "tags": ["auth", "refactor", "jwt"],
  "ttl_days": 30
}
```

### Workflow 5: Code Pattern Memory

Remember project-specific patterns:

```
{
  "text": "Error handling pattern: All API routes use try-catch with AppError class, logged via winston, returned as { error: string, code: number }",
  "kind": "fact",
  "importance": 0.85,
  "tags": ["patterns", "error-handling", "api"]
}
```

### Workflow 6: Entity-Based Discovery

With `ENTITY_EXTRACTION=true`, find related information:

```
User: "What do we know about the authentication system?"

Claude uses memory.search to find auth-related memories,
then memory.related to discover connected information
through shared entities (technologies, people, projects).
```

---

## Advanced Features

### Multi-Model Embeddings

Store embeddings from multiple models for comparison or migration:

```bash
export EMBED_MODELS="text-embedding-3-small,text-embedding-3-large"
./bin/cortex
```

Each memory will have embeddings from both models. Search can filter by model:

```json
{
  "query": "authentication patterns",
  "model": "text-embedding-3-large"
}
```

### Entity Extraction

Enable automatic entity extraction:

```bash
export ENTITY_EXTRACTION=true
./bin/cortex
```

When memories are added, Cortex extracts:

| Entity Type | Examples |
|-------------|----------|
| `person` | Team members, stakeholders |
| `organization` | Companies, teams |
| `technology` | Languages, frameworks, tools |
| `project` | Repositories, products |
| `concept` | Design patterns, methodologies |
| `location` | Servers, regions |
| `event` | Meetings, deadlines |

### TTL-Based Cleanup

Memories with `ttl_days` are automatically cleaned up by the sweeper:

```bash
# Configure sweeper
export SWEEPER_ENABLED=true
export SWEEPER_INTERVAL=1h

./bin/cortex
```

### Health Checks

Enable HTTP health endpoint for container orchestration:

```bash
export HEALTH_PORT=8080
./bin/cortex

# Check health
curl http://localhost:8080/health
```

---

## Troubleshooting

### Common Issues

#### "DATABASE_URL environment variable is required"

Ensure the database connection string is set:

```bash
export DATABASE_URL="postgres://user:pass@localhost:5432/cortex?sslmode=disable"
```

#### "OPENAI_API_KEY environment variable is required"

Set the API key for your chosen backend:

```bash
# For OpenAI
export OPENAI_API_KEY="sk-..."

# For Gemini
export LM_BACKEND="gemini"
export GEMINI_API_KEY="..."
```

#### "pgvector extension not found"

Install pgvector in PostgreSQL:

```bash
# On macOS with Homebrew
brew install pgvector

# Then enable in database
psql cortex -c 'CREATE EXTENSION vector;'
```

#### Tools Not Appearing in Claude Code

1. Check `.mcp.json` syntax (valid JSON)
2. Verify command path is correct
3. Restart Claude Code completely
4. Check Cortex logs: `docker-compose logs cortex`

#### Slow Search Performance

1. Ensure vector index exists:
   ```sql
   SELECT indexname FROM pg_indexes WHERE tablename = 'memory_embeddings';
   ```
2. Consider switching from IVFFlat to HNSW for better performance

### Logs and Debugging

```bash
# View Cortex logs
docker-compose logs -f cortex

# Check database connection
psql $DATABASE_URL -c "SELECT 1"

# Verify extensions
psql $DATABASE_URL -c "SELECT * FROM pg_extension WHERE extname IN ('vector', 'pg_trgm')"

# Count memories
psql $DATABASE_URL -c "SELECT COUNT(*) FROM memories"
```

### Performance Tuning

For large memory stores (>100k memories):

1. **Use HNSW index** (better recall, faster search):
   ```sql
   CREATE INDEX idx_memory_embed_hnsw
   ON memory_embeddings USING hnsw (embedding vector_cosine_ops);
   ```

2. **Increase work_mem** for complex queries:
   ```sql
   SET work_mem = '256MB';
   ```

3. **Vacuum regularly**:
   ```sql
   VACUUM ANALYZE memories;
   VACUUM ANALYZE memory_embeddings;
   ```

---

## Quick Reference

### Essential Commands

```bash
# Start (Docker)
docker-compose up -d

# Start (Manual)
./bin/cortex

# Export backup
./bin/cortex --export backup.jsonl

# Import backup
./bin/cortex --import backup.jsonl

# Re-embed all memories
./bin/cortex --reembed
```

### Essential Environment Variables

```bash
DATABASE_URL="postgres://localhost:5432/cortex?sslmode=disable"
OPENAI_API_KEY="sk-..."
WORKSPACE_ID="my-project"
```

### MCP Configuration Template

```json
{
  "mcpServers": {
    "cortex": {
      "type": "stdio",
      "command": "/path/to/cortex",
      "env": {
        "DATABASE_URL": "postgres://localhost:5432/cortex?sslmode=disable",
        "WORKSPACE_ID": "my-project",
        "OPENAI_API_KEY": "${OPENAI_API_KEY}"
      }
    }
  }
}
```

---

## Version Information

- **Current Version**: 1.0.0
- **Go Version**: 1.22+
- **PostgreSQL**: 14+
- **Required Extensions**: pgvector, pg_trgm

## License

MIT License - See LICENSE file for details.

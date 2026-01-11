# Cortex

**A local-first MCP memory server for Claude Code** — Persistent memory for AI agents with hybrid vector + lexical search.

<p align="center">
  <a href="https://github.com/swiftj/cortex/releases"><img src="https://img.shields.io/badge/dynamic/regex?url=https%3A%2F%2Fraw.githubusercontent.com%2Fswiftj%2Fcortex%2Fmain%2FVERSION&search=%5E%5B%5E%5Cn%5D%2B&style=flat-square&label=VERSION&color=olive" alt="Version"></a>
  <a href="https://go.dev/"><img src="https://img.shields.io/badge/Go-1.24%2B-00ADD8?style=flat-square&logo=go&logoColor=white" alt="Go"></a>
  <a href="https://github.com/swiftj/cortex"><img src="https://img.shields.io/badge/macOS-000000?style=flat-square&logo=apple&logoColor=white" alt="macOS"></a>
  <a href="https://github.com/swiftj/cortex"><img src="https://img.shields.io/badge/Linux-FCC624?style=flat-square&logo=linux&logoColor=black" alt="Linux"></a>
  <a href="LICENSE"><img src="https://img.shields.io/badge/License-MIT-gray?style=flat-square" alt="License"></a>
</p>

<p align="center">
  <a href="https://github.com/swiftj/cortex"><img src="https://img.shields.io/badge/AI-Multi--Provider-E91E8C?style=flat-square" alt="AI Multi-Provider"></a>
  <a href="https://claude.ai/code"><img src="https://img.shields.io/badge/Claude%20Code-Compatible-CD6155?style=flat-square" alt="Claude Code Compatible"></a>
  <a href="https://github.com/pgvector/pgvector"><img src="https://img.shields.io/badge/PostgreSQL-pgvector-4169E1?style=flat-square&logo=postgresql&logoColor=white" alt="PostgreSQL pgvector"></a>
  <a href="https://github.com/swiftj/cortex"><img src="https://img.shields.io/badge/Docker-Ready-2496ED?style=flat-square&logo=docker&logoColor=white" alt="Docker Ready"></a>
</p>

---

## Features

- **MCP Native**: Exposes memory tools via JSON-RPC 2.0 over stdio for Claude Code integration
- **Hybrid Search**: Combines vector similarity (pgvector) with lexical matching (pg_trgm) for optimal recall
- **Workspace Namespacing**: Project-specific memory isolation via workspace IDs
- **Entity Extraction**: Optional LLM-based extraction of people, organizations, technologies with knowledge graph
- **Multi-model Embeddings**: Store embeddings from multiple models simultaneously
- **TTL Sweeper**: Automatic memory cleanup based on time-to-live settings
- **Pluggable LLMs**: Supports OpenAI and Google Gemini for embeddings and text normalization
- **Docker Ready**: Pre-configured Docker Compose with PostgreSQL + pgvector
- **Single Binary**: Pure Go, no CGO dependencies, compiles to a single static binary
- **Export/Import**: JSONL format for backup and migration

## Quick Start

### Option 1: Docker (Recommended)

The easiest way to run Cortex is with Docker Compose, which includes PostgreSQL with pgvector pre-configured.

```bash
# Clone the repository
git clone https://github.com/swiftj/cortex.git
cd cortex

# Create .env file with your API key
echo "OPENAI_API_KEY=sk-..." > .env
# or for Gemini:
echo "GEMINI_API_KEY=..." > .env
echo "LM_BACKEND=gemini" >> .env

# Start the services
docker-compose up -d

# View logs
docker-compose logs -f cortex
```

#### Using with Claude Code (Docker)

Configure `.mcp.json` to use the Docker container:

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

### Option 2: Manual Installation

#### Prerequisites

- Go 1.24+
- PostgreSQL 14+ with extensions:
  - `pgvector` - Vector similarity search
  - `pg_trgm` - Trigram text similarity

#### Database Setup

```bash
# Create database
createdb cortex

# Enable required extensions
psql cortex -c 'CREATE EXTENSION IF NOT EXISTS vector;'
psql cortex -c 'CREATE EXTENSION IF NOT EXISTS pg_trgm;'
```

#### Build

```bash
CGO_ENABLED=0 go build -o bin/cortex ./cmd/mcpserver
```

#### Configure

Set environment variables:

```bash
# Required
export DATABASE_URL="postgres://localhost:5432/cortex?sslmode=disable"

# Optional (defaults shown)
export TENANT_ID="local"
export WORKSPACE_ID="default"           # Project-specific isolation
export LM_BACKEND="openai"              # or "gemini"
export LM_MODEL="auto"                  # Chat model for normalization
export EMBED_MODEL="auto"               # Single embedding model
export EMBED_MODELS=""                  # Comma-separated for multi-model (e.g., "text-embedding-3-small,text-embedding-3-large")
export SWEEPER_ENABLED="true"           # TTL-based memory cleanup
export SWEEPER_INTERVAL="1h"            # Cleanup frequency
export ENTITY_EXTRACTION="false"        # LLM-based entity extraction
export HEALTH_PORT=""                   # HTTP health endpoint (e.g., "8080")

# API Keys (one required based on LM_BACKEND)
export OPENAI_API_KEY="sk-..."
# or
export GEMINI_API_KEY="..."
```

#### Run

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
        "WORKSPACE_ID": "my-project",
        "LM_BACKEND": "openai",
        "OPENAI_API_KEY": "${OPENAI_API_KEY}"
      }
    }
  }
}
```

Restart Claude Code and the memory tools will be available.

### Per-Project Workspaces

Use `WORKSPACE_ID` to isolate memories per project:

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

## Getting Claude Code to Reliably Use Cortex

Having Cortex installed is only half the battle. The real power comes from Claude Code **proactively** using memory tools during your development sessions. Here are proven strategies to achieve reliable memory usage.

### Strategy 1: Project Instructions (CLAUDE.md)

Create a `CLAUDE.md` file in your project root with explicit memory directives:

````markdown
# Project Memory Protocol

## Memory Requirements
This project uses Cortex for persistent memory. You MUST:

1. **At session start**: Search memories for project context
   - `memory.search({ query: "project architecture decisions" })`
   - `memory.search({ query: "coding standards preferences" })`

2. **When learning something new**: Store it immediately
   - User preferences → `memory.add` with kind: "preference"
   - Architecture decisions → `memory.add` with kind: "fact", importance: 0.9
   - Gotchas/pitfalls → `memory.add` with kind: "note", tags: ["gotcha"]

3. **Before major changes**: Check for relevant context
   - `memory.search({ query: "<relevant topic>" })`

4. **After completing features**: Document learnings
   - `memory.add({ text: "Completed X using Y approach", kind: "fact", tags: ["implementation"] })`
````

### Strategy 2: Explicit Prompting Patterns

Use these prompt patterns to trigger memory operations:

**Session Start:**
> "Check your memory for any context about this project before we begin."

**During Development:**
> "Remember that we decided to use X approach for Y."
> "Store this as an important architectural decision."
> "What do you remember about how we handle authentication?"

**Session End:**
> "Save the key learnings from this session to memory."
> "What should we remember for next time?"

### Strategy 3: Workspace Isolation

Configure automatic workspace detection in `.mcp.json`:

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

This ensures `my-webapp` project memories stay separate from `api-service` memories.

### Strategy 4: Bootstrap Session Pattern

Start each coding session with a bootstrap prompt:

> "Let's start a development session. First:
> 1. Search your memory for recent work on this project
> 2. Search for any user preferences or coding standards
> 3. Tell me what you remember, then we'll continue"

This primes Claude Code to engage with memory from the start.

### Practical Workflow Examples

**Feature Development:**
```
You: "I want to add user authentication"
Claude: *searches memory for auth-related decisions*
Claude: "I found we previously decided to use JWT tokens. Let me search for more context..."
*implements feature*
Claude: *stores implementation decisions*
```

**Bug Investigation:**
```
You: "There's a bug in the payment flow"
Claude: *searches memory for payment-related notes*
Claude: "I remember we had a similar issue with currency conversion. Let me check those notes..."
```

**Code Review:**
```
You: "Review this PR"
Claude: *searches memory for coding standards and past decisions*
Claude: "Based on our established patterns, I notice this doesn't follow our error handling convention..."
```

### Memory Categories to Establish

Build a rich memory foundation with these categories:

| Kind | Use For | Importance |
|------|---------|------------|
| `preference` | User coding style, tool preferences | 0.7-0.9 |
| `fact` | Architecture decisions, API contracts | 0.8-1.0 |
| `note` | Implementation details, gotchas | 0.5-0.7 |
| `project` | Project-specific context | 0.6-0.8 |
| `identity` | Team info, stakeholders | 0.5-0.6 |

### Pro Tips

1. **High-importance memories surface first** - Set `importance: 0.9` for critical decisions
2. **Use tags consistently** - Tags like `["auth", "security"]` help with targeted searches
3. **Set TTL for temporary context** - Use `ttl_days: 30` for sprint-specific notes
4. **Review and prune periodically** - Export memories and clean up outdated entries
5. **Seed initial context** - At project start, manually add key architecture decisions

### Common Pitfalls

| Problem | Solution |
|---------|----------|
| Claude doesn't use memory | Add explicit instructions to CLAUDE.md |
| Too many irrelevant results | Use more specific search queries and tags |
| Memories getting stale | Enable TTL sweeper, set appropriate `ttl_days` |
| Cross-project contamination | Ensure `WORKSPACE_ID` is set correctly |
| Lost context between sessions | Use bootstrap prompts at session start |

## MCP Tools

### `memory.add`

Store a new memory with optional metadata.

```json
{
  "text": "User prefers dark mode in all applications",
  "kind": "preference",
  "importance": 0.8,
  "tags": ["ui", "settings"],
  "ttl_days": 90,
  "source": "chat"
}
```

**Parameters:**
- `text` (required): Memory content
- `kind`: Type (`note`, `fact`, `todo`, `preference`, `identity`, `project`)
- `importance`: Priority score (0.0 - 1.0, default: 0.5)
- `tags`: Categorization tags
- `ttl_days`: Days until auto-expiry
- `source`: Origin identifier

**Returns**: `{ "id": 123 }`

### `memory.search`

Search memories using hybrid vector + lexical matching.

```json
{
  "query": "user interface preferences",
  "k": 10,
  "hybrid": true,
  "model": "text-embedding-3-small"
}
```

**Parameters:**
- `query` (required): Search query
- `k`: Max results (1-100, default: 10)
- `hybrid`: Use hybrid search (default: true)
- `model`: Filter by embedding model (optional)

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

### `memory.export`

Export memories to JSONL format.

```json
{
  "include_embeddings": false,
  "kind": "preference",
  "limit": 100
}
```

**Returns**: `{ "data": "...", "exported": 100, "errors": 0 }`

### `memory.import`

Import memories from JSONL format.

```json
{
  "data": "{\"id\":1,\"text\":\"...\",\"kind\":\"note\"}\n{\"id\":2,...}",
  "skip_existing": false,
  "regenerate_embeddings": false,
  "dry_run": false
}
```

**Returns**: `{ "total": 2, "imported": 2, "skipped": 0, "errors": 0 }`

### `memory.entities`

Get entities extracted from a memory (requires `ENTITY_EXTRACTION=true`).

```json
{
  "memory_id": 123
}
```

**Returns**:
```json
{
  "entities": [
    {"id": 1, "name": "TypeScript", "type": "technology"},
    {"id": 2, "name": "React", "type": "technology"}
  ]
}
```

### `memory.related`

Find memories that share entities with a given memory.

```json
{
  "memory_id": 123,
  "k": 10
}
```

**Returns**: Array of related memories with entity overlap scores.

## CLI Mode

Cortex supports CLI mode for batch operations:

### Export

```bash
# Export all memories
./bin/cortex --export memories.jsonl

# Export with embeddings (larger file)
./bin/cortex --export memories.jsonl --with-embeddings

# Export specific workspace
WORKSPACE_ID=my-project ./bin/cortex --export project.jsonl
```

### Import

```bash
# Import memories
./bin/cortex --import memories.jsonl

# Skip existing records
./bin/cortex --import memories.jsonl --skip-existing

# Regenerate embeddings (requires API key)
./bin/cortex --import memories.jsonl --regenerate-embeddings

# Dry run (validate without writing)
./bin/cortex --import memories.jsonl --dry-run
```

### Re-embed

Re-embed all memories when switching embedding models:

```bash
# Re-embed with current model
./bin/cortex --reembed

# Custom batch size and delay (rate limiting)
./bin/cortex --reembed --reembed-batch-size 50 --reembed-delay 200ms

# Delete old embeddings after re-embedding
./bin/cortex --reembed --reembed-delete-old
```

## Architecture

```
cortex/
├── cmd/mcpserver/       # Entry point
├── internal/
│   ├── db/              # PostgreSQL operations (pgx)
│   ├── llm/             # LLM adapters (OpenAI, Gemini, MultiEmbedder)
│   ├── mcp/             # MCP JSON-RPC server
│   ├── search/          # Hybrid search & ranking
│   ├── sweeper/         # TTL-based memory cleanup
│   ├── entity/          # LLM-based entity extraction
│   ├── reembed/         # Batch re-embedding utility
│   └── transfer/        # Export/import (JSONL)
├── migrations/          # Embedded SQL migrations
├── docs/                # Documentation
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

### Entity Extraction

When enabled (`ENTITY_EXTRACTION=true`), Cortex automatically extracts entities from memories:

| Entity Type | Examples |
|-------------|----------|
| `person` | Team members, stakeholders |
| `organization` | Companies, teams |
| `technology` | Languages, frameworks, tools |
| `project` | Repositories, products |
| `concept` | Design patterns, methodologies |
| `location` | Servers, regions |
| `event` | Meetings, deadlines |

Entities are linked to memories and can be used to discover related information via `memory.related`.

### LLM Providers

| Provider | Chat Model (default) | Embedding Model (default) | Dimensions |
|----------|---------------------|---------------------------|------------|
| OpenAI | gpt-4o-mini | text-embedding-3-small | 1536 |
| Gemini | gemini-2.0-flash-lite | text-embedding-004 | 768 |

## Database Schema

```sql
-- Main memories table
CREATE TABLE memories (
  id           BIGSERIAL PRIMARY KEY,
  tenant_id    TEXT NOT NULL DEFAULT 'local',
  workspace_id TEXT NOT NULL DEFAULT 'default',
  kind         TEXT NOT NULL,
  text         TEXT NOT NULL,
  source       TEXT,
  created_at   TIMESTAMPTZ DEFAULT now(),
  updated_at   TIMESTAMPTZ DEFAULT now(),
  tags         TEXT[] DEFAULT '{}',
  importance   REAL DEFAULT 0.5,
  ttl_days     INT,
  meta         JSONB DEFAULT '{}'
);

-- Multi-model embeddings (composite primary key)
CREATE TABLE memory_embeddings (
  memory_id  BIGINT REFERENCES memories(id) ON DELETE CASCADE,
  model      TEXT NOT NULL,
  dims       INT NOT NULL,
  embedding  VECTOR NOT NULL,
  PRIMARY KEY (memory_id, model)
);

-- Entity extraction tables
CREATE TABLE entities (
  id           BIGSERIAL PRIMARY KEY,
  tenant_id    TEXT NOT NULL,
  workspace_id TEXT NOT NULL,
  name         TEXT NOT NULL,
  type         entity_type NOT NULL,
  aliases      TEXT[] DEFAULT '{}',
  description  TEXT,
  meta         JSONB DEFAULT '{}'
);

CREATE TABLE memory_entities (
  memory_id  BIGINT REFERENCES memories(id) ON DELETE CASCADE,
  entity_id  BIGINT REFERENCES entities(id) ON DELETE CASCADE,
  role       TEXT,
  confidence REAL DEFAULT 1.0,
  PRIMARY KEY (memory_id, entity_id)
);

CREATE TABLE entity_relations (
  id            BIGSERIAL PRIMARY KEY,
  source_id     BIGINT REFERENCES entities(id),
  target_id     BIGINT REFERENCES entities(id),
  relation_type TEXT NOT NULL
);
```

## Configuration Reference

| Variable | Required | Default | Description |
|----------|----------|---------|-------------|
| `DATABASE_URL` | Yes | - | PostgreSQL connection string |
| `OPENAI_API_KEY` | If OpenAI | - | OpenAI API key |
| `GEMINI_API_KEY` | If Gemini | - | Google Gemini API key |
| `TENANT_ID` | No | `local` | Tenant identifier |
| `WORKSPACE_ID` | No | `default` | Workspace for project isolation |
| `LM_BACKEND` | No | `openai` | LLM provider (`openai` or `gemini`) |
| `LM_MODEL` | No | `auto` | Chat model for normalization |
| `EMBED_MODEL` | No | `auto` | Embedding model |
| `EMBED_MODELS` | No | - | Comma-separated list for multi-model |
| `SWEEPER_ENABLED` | No | `true` | Enable TTL cleanup |
| `SWEEPER_INTERVAL` | No | `1h` | Cleanup frequency |
| `ENTITY_EXTRACTION` | No | `false` | Enable entity extraction |
| `HEALTH_PORT` | No | - | HTTP health endpoint port |

## Development

```bash
# Run tests
go test ./...

# Build with optimizations
CGO_ENABLED=0 go build -trimpath -ldflags "-s -w" -o bin/cortex ./cmd/mcpserver

# Check binary size
ls -lh bin/cortex
```

## Documentation

See [docs/CLAUDE_CODE_GUIDE.md](docs/CLAUDE_CODE_GUIDE.md) for comprehensive documentation including:
- Detailed installation guides
- Configuration options
- Software development workflows
- Troubleshooting

## Inspiration

Cortex is inspired by [Mem0](https://github.com/mem0ai/mem0), adapted for the MCP ecosystem and Claude Code workflows.

## License

MIT

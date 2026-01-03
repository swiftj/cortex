
# CLAUDE.md — Cortex (Go) • An MCP Memory Server for Claude Code

> **Mission**  
> Build a **local‑first**, **single‑binary** memory layer inspired by Mem0 that plugs directly into **Claude Code** via **MCP**. The server is written **100% in Go** (no CGO), uses **PostgreSQL + pgvector** locally, and exposes memory tools (add/search/update/delete) for agents. LLMs are **pluggable** (OpenAI or Google Gemini) with sensible defaults and easy env‑based configuration.

---

## 0) Name & Scope

**Working codename:** `Cortex` (lore + memory). Feel free to rename later (e.g., BardCache, SagaStore, TomeKeeper, MemoryForge).

**What it is**
- A local MCP server for Claude Code that:
  - Extracts & stores short “memories” (facts/notes/preferences/tasks).
  - Embeds content and performs **hybrid retrieval** (vector + lexical).
  - Returns ranked results as tool outputs back to Claude Code.

**What it is not**
- A general document store or chat framework; this is a focused, MCP‑native **memory substrate** for IDE/agent workflows.

---

## 1) Why MCP & Why Go?

- **MCP (Model Context Protocol)** lets Claude Code call external tools & data over **stdio + JSON‑RPC**. This server exposes tools like `memory.add` and `memory.search` that Claude can invoke directly.
- **Go** enables **static, single‑file binaries** with fast startup and low memory. We use pure‑Go deps only (no CGO).

---

## 2) System Requirements

- **Go** ≥ 1.22 (with `CGO_ENABLED=0` for builds)  
- **PostgreSQL** ≥ 14 with extensions: `pgvector`, `pg_trgm`  
- Internet access for your chosen **LLM API** (OpenAI or Google Gemini)

> **Indexing choice**: pgvector offers **HNSW** (best recall/speed, higher RAM) and **IVFFlat** (lighter, tune `lists`/`probes`). Start with HNSW for dev convenience; switch to IVFFlat if RAM becomes a concern.

---

## 3) Project Layout (proposed)

```
/cmd/mcpserver       # main() — stdio JSON-RPC MCP server
/internal/mcp        # JSON-RPC plumbing, tool registry, validation
/internal/db         # pgx pool, migrations, queries
/internal/llm        # provider adapters (openai, gemini), embeddings & rewrite
/internal/search     # hybrid scoring (vector + lexical fusion)
/migrations          # embedded SQL migrations (go:embed)
/configs             # example .mcp.json snippets for Claude Code
```

---

## 4) Postgres Schema (minimal)

> Run via **embedded migrations** on startup (no external CLI).

```sql
CREATE EXTENSION IF NOT EXISTS vector;
CREATE EXTENSION IF NOT EXISTS pg_trgm;

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

CREATE TABLE IF NOT EXISTS memory_embeddings (
  memory_id  BIGINT PRIMARY KEY REFERENCES memories(id) ON DELETE CASCADE,
  model      TEXT NOT NULL,     -- "text-embedding-3-large", "gemini-embedding-2", etc.
  dims       INT  NOT NULL,
  embedding  VECTOR NOT NULL
);

-- Text index for lexical similarity
CREATE INDEX IF NOT EXISTS idx_memories_text_trgm
  ON memories USING gin (text gin_trgm_ops);

-- Choose ONE vector index to start (switchable via env):
-- HNSW (best speed/recall; more RAM; no training step)
-- CREATE INDEX IF NOT EXISTS idx_memory_embed_hnsw
--   ON memory_embeddings USING hnsw (embedding vector_l2_ops);

-- OR IVFFlat (lighter; needs training; tune lists/probes)
-- CREATE INDEX IF NOT EXISTS idx_memory_embed_ivf
--   ON memory_embeddings USING ivfflat (embedding vector_l2_ops) WITH (lists=100);
```
**Tip:** Use **L2** or **cosine** ops consistently between index and queries.

---

## 5) Build, Run, and Configure

### Build (single binary, no CGO)
```bash
export CGO_ENABLED=0
go build -trimpath -ldflags "-s -w" -o bin/loremem ./cmd/mcpserver
```

### Environment
```
DATABASE_URL=postgres://localhost:5432/loremem?sslmode=disable
TENANT_ID=local

# LLM selection (either OpenAI or Gemini)
LM_BACKEND=openai|gemini
LM_MODEL=auto                   # sensible default; overridable
EMBED_MODEL=auto                # sensible default; overridable

# OpenAI
OPENAI_API_KEY=...

# Gemini
GEMINI_API_KEY=...
```

### First‑run DB prep
```bash
createdb loremem
psql loremem -c 'CREATE EXTENSION IF NOT EXISTS vector;'
psql loremem -c 'CREATE EXTENSION IF NOT EXISTS pg_trgm;'
# The binary runs embedded migrations automatically on start.
```

---

## 6) Claude Code Integration (MCP)

Create a project‑local `.mcp.json` (or add to Claude Code’s settings):

```json
{
  "mcpServers": {
    "loremem": {
      "type": "stdio",
      "command": "${PWD}/bin/loremem",
      "args": [],
      "env": {
        "DATABASE_URL": "postgres://localhost:5432/loremem?sslmode=disable",
        "TENANT_ID": "local",
        "LM_BACKEND": "openai",
        "LM_MODEL": "auto",
        "EMBED_MODEL": "auto",
        "OPENAI_API_KEY": "${OPENAI_API_KEY}"
      },
      "scope": ["${PWD}"]
    }
  }
}
```

Restart Claude Code; the `loremem` tools should appear automatically.

---

## 7) MCP Tool Surface (v0)

**Tools**

- `memory.add({ text, kind?, importance?, tags?, ttl_days?, source? }) -> { id }`  
  - Optionally runs an LLM **normalizer** to compress/rewrite the memory (consistent style).
- `memory.search({ query, k?, hybrid? }) -> [{ id, text, score, source, tags, importance }]`  
  - **Hybrid** rank: vector similarity (pgvector) + lexical (trigram), fused by `final = α·vector + (1-α)·lexical` (default `α=0.7`).
- `memory.update({ id, patch }) -> { ok: true }`
- `memory.delete({ id }) -> { ok: true }`

**Scoring**
- Add optional boosts for **importance** and **recency**.  
- Use L2 or cosine consistently; normalize component scores before fusion.

---

## 8) LLM Adapters

**OpenAI (default)**  
- Official **Go SDK**; use **Responses API** for rewrite/normalization and **Embeddings** for vectors.
- Suggested defaults: a current “mini/efficient” chat model for normalization; `text-embedding-3-large` (or latest recommended) for embeddings.

**Google Gemini**  
- **Google Gen AI Go SDK** (`google.golang.org/genai`).
- Pick a current Gemini text model for normalization and the latest Gemini embedding model for vectors.

Switch providers by changing `LM_BACKEND` and model env vars—no code changes.

---

## 9) Dev Loop

```bash
# 1) Build & run the MCP server
CGO_ENABLED=0 go build -trimpath -ldflags "-s -w" -o bin/loremem ./cmd/mcpserver
./bin/loremem

# 2) Add to Claude Code (.mcp.json) and restart

# 3) Try it
# In Claude Code, call: memory.add, then memory.search
```

---

## 10) Roadmap (stretch)

- Namespacing per workspace/project (multiple tenants).  
- Entity table + light relations (optional) to enable simple graph queries.  
- TTL sweeper & retention policy (importance‑aware).  
- Export/Import (JSONL).  
- Batch re‑embed utility when switching embedding models.  
- Multi‑model embeddings (store multiple rows per memory).

---

## 11) References

- **Mem0 (inspiration):** https://github.com/mem0ai/mem0  
- **MCP docs (build a server / clients / Claude Code MCP):**  
  - https://modelcontextprotocol.io/docs/develop/build-server  
  - https://modelcontextprotocol.io/docs/develop/build-client  
  - https://code.claude.com/docs/en/mcp
- **pgvector (HNSW / IVFFlat guidance):** https://github.com/pgvector/pgvector  
- **OpenAI API (Go SDK + Responses):**  
  - https://github.com/openai/openai-go  
  - https://platform.openai.com/docs/api-reference/responses
- **Google Gen AI Go SDK (Gemini):**  
  - https://github.com/googleapis/go-genai  
  - https://ai.google.dev/gemini-api/docs/libraries

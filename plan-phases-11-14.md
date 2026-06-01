# AgentVault Implementation Plan — Phases 11-14

> Status: historical planning document.
>
> Git integration, vector-search foundations, multi-provider AI, and the standalone web UI now exist at least partially in the codebase. Use [docs/CODEBASE_ANALYSIS.md](docs/CODEBASE_ANALYSIS.md) for current-state details and [docs/IMPROVEMENT_PLAN.md](docs/IMPROVEMENT_PLAN.md) for the active improvement plan.

## Phase 11: Git Integration

**Goal:** Native Git commands in the CLI for vault versioning and review workflows.

### Features:
- `agentvault git status` — Show modified/untracked notes in the vault
- `agentvault git diff` — Show diff of changed notes
- `agentvault git commit -m "message"` — Commit vault changes
- `agentvault git log` — Show recent commits affecting vault files
- `agentvault git init` — Initialize git repo in vault if not present
- Auto-commit option on `new`, `ask` (AI-generated writes)
- Git safety: never auto-commit unless `--commit` flag passed

### Implementation:
- `internal/git/` — Go package wrapping `git` CLI commands
- `cmd/agentvault/git.go` — Cobra subcommands
- Shell out to `git` first (not go-git library — simpler, uses user's git config)

## Phase 12: Vector Search + Embeddings

**Goal:** Semantic search beyond FTS5 using text embeddings.

### Features:
- Generate embeddings for note chunks
- Store in `chunks` table with `embedding_json` field
- Vector similarity search (cosine similarity via pure Go)
- Hybrid search: FTS + vector combined scoring
- Configurable chunking strategy

### Implementation:
- `internal/embeddings/` — Embedding generation via Ollama embeddings API
- `internal/chunker/` — Text chunking strategies (fixed-size, semantic)
- `internal/vectors/` — Vector operations (cosine similarity, top-k)
- Update indexer to generate chunks + embeddings
- Update search to support `?vector=true` in API

## Phase 13: Multi-Provider AI

**Goal:** Support multiple AI providers beyond Ollama.

### Features:
- OpenAI-compatible provider (OpenAI, Groq, Together, etc.)
- Anthropic provider (Claude)
- OpenRouter provider (aggregated access)
- Provider auto-detection and fallback
- Unified configuration

### Implementation:
- `internal/ai/openai.go` — OpenAI-compatible HTTP client
- `internal/ai/anthropic.go` — Anthropic Messages API client
- `internal/ai/openrouter.go` — OpenRouter client
- `internal/ai/config.go` — Provider configuration management
- Update `rag/pipeline.go` to work with any provider

## Phase 14: Web UI

**Goal:** Standalone React web app that connects to the local API.

### Features:
- Independent React SPA (no Wails)
- Connects to `http://127.0.0.1:47321`
- Vault status dashboard
- Note search with filters
- Note editor (read-only for v1, edit via API)
- AI ask interface
- Project/decision dashboards
- Responsive design

### Implementation:
- `apps/web-local/` — React + TypeScript + Vite project
- Uses the same API client as desktop
- Serves via Vite dev server or static build

## Execution: 4 parallel agents

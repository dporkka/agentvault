# AgentVault Implementation Plan — Phases 3-6

> Status: historical planning document.
>
> These phases have been implemented or partially superseded in the current codebase. Use [docs/CODEBASE_ANALYSIS.md](docs/CODEBASE_ANALYSIS.md) for the current architecture map and [docs/IMPROVEMENT_PLAN.md](docs/IMPROVEMENT_PLAN.md) for the active improvement plan.

## Phase 3: AI/RAG (`agentvault ask`)

### New Packages
- `internal/ai/` — Provider interface, Ollama client, config
- `internal/rag/` — Retrieval pipeline: search → context build → prompt → answer

### New Files
- `core/internal/ai/provider.go` — AIProvider interface
- `core/internal/ai/ollama.go` — Ollama HTTP API client
- `core/internal/ai/mock.go` — Mock provider for tests
- `core/internal/rag/pipeline.go` — RAG pipeline: query → retrieve → generate
- `core/internal/rag/prompts.go` — System prompts for answer generation
- `core/cmd/agentvault/ask.go` — ask CLI command

### Interfaces
```go
type AIProvider interface {
    Name() string
    Chat(ctx context.Context, messages []Message) (string, error)
    HealthCheck(ctx context.Context) error
}

type Message struct {
    Role    string
    Content string
}
```

### Ask Command Flow
1. Parse question from args
2. Search vault via FTS5 (top 5-10 results)
3. Build context with note titles, paths, excerpts
4. Call AI provider with structured prompt
5. Return: Answer + Sources + Confidence + Caveats + Suggested actions

---

## Phase 4: Local HTTP API (`agentvault serve`)

### New Package
- `internal/api/` — HTTP handlers, middleware, routes

### Endpoints
```
GET  /health              → {"status": "ok"}
GET  /vault/status        → vault info, indexed files count
POST /vault/index         → trigger indexing
GET  /search?q=...        → search results JSON
GET  /notes/:id           → get note by ID
POST /notes               → create new note
POST /capture             → inbox capture
POST /ask                 → AI ask endpoint
GET  /git/status          → git status (stub)
GET  /projects            → list projects
GET  /recent              → recent notes
GET  /stale               → stale notes
```

### Security
- Bind to 127.0.0.1:47321 by default
- Local auth token for write endpoints
- CORS for browser extension

---

## Phase 5: MCP Server (`agentvault mcp serve`)

### New Package
- `internal/mcp/` — MCP server, tool definitions, handlers

### Tools
```
agentvault.search        — FTS5 search
agentvault.read_note     — Read note by ID
agentvault.create_note   — Create note from template
agentvault.create_decision — Create decision
agentvault.create_task   — Create task
agentvault.capture       — Add capture to inbox
agentvault.summarize     — Summarize files/folder
agentvault.list_projects — List projects
agentvault.list_recent   — List recent notes
agentvault.git_status    — Git status
```

### Transports
- stdio (default)
- HTTP (optional --http --port)

---

## Phase 6: Wails Desktop App

### Project Setup
- `apps/desktop-wails/` — Wails desktop project (implemented with Wails v2)
- Go backend wrapping core engine
- React frontend with TypeScript + Tailwind

### Views
- Vault picker (first launch)
- Search + results
- Editor (CodeMirror 6) + preview split
- Ask panel (AI sidebar)
- Project dashboard
- Decision dashboard
- Settings

### Dependencies
- Wails v2
- React 18 + TypeScript
- CodeMirror 6
- Tailwind CSS v3

---

## Execution Order
1. Parallel: Phases 3, 4, 5 (independent implementations)
2. Merge all into main
3. Phase 6: Wails desktop (depends on API stability)
4. Integration testing

## Parallel Agent Grouping
- **Agent A — AI/RAG**: ai/ package, rag/ package, ask command
- **Agent B — Local API**: api/ package, serve command, all HTTP handlers
- **Agent C — MCP Server**: mcp/ package, mcp serve command, all tools

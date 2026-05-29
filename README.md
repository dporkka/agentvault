# AgentVault

Your notes, decisions, docs, and research — structured for humans, searchable by agents, stored as files.

## What is AgentVault?

AgentVault is a local-first, open-source AI knowledge operating system. It turns a folder of Markdown/YAML files into an intelligent, searchable, source-grounded, agent-accessible knowledge base.

## Features

- **Local-first**: Your data stays on your machine as plain Markdown files
- **Markdown-native**: Plain Markdown + YAML frontmatter — no lock-in
- **Full-text search**: SQLite FTS5 for instant search with filters
- **AI-powered answers**: Source-grounded retrieval with Ollama/local LLMs
- **MCP server**: Model Context Protocol for AI agent integration
- **Local HTTP API**: Powers desktop app, browser extension, and mobile clients
- **Git-friendly**: Works naturally with version control
- **Low dependency**: Single Go binary, pure Go SQLite, minimal deps

## Quick Start

```bash
# Build
cd core && go build -o ../bin/agentvault ./cmd/agentvault

# Initialize a vault
./bin/agentvault init ./my-vault
cd ./my-vault

# Create notes
../bin/agentvault new note --title "My first note"
../bin/agentvault new decision --project myproject --title "Use Postgres"
../bin/agentvault new task --project myproject --title "Build API"

# Index everything
../bin/agentvault index

# Search
../bin/agentvault search "Postgres"

# Ask AI (requires Ollama)
../bin/agentvault ask "What have I decided about databases?"

# Validate
../bin/agentvault doctor
```

## CLI Reference

### Core Commands

| Command | Description |
|---------|-------------|
| `agentvault init [path]` | Initialize a new vault with folder structure |
| `agentvault index [--force \| --rebuild]` | Index all Markdown files for search |
| `agentvault search <query> [--type] [--project]` | Full-text search with filters |
| `agentvault read <id-or-path>` | Read a specific note |
| `agentvault new <type> --title <title>` | Create a structured note |
| `agentvault doctor` | Validate vault health (7 checks) |

### New Note Types

```bash
agentvault new note --title "My Idea"
agentvault new decision --project myproject --title "Use Postgres"
agentvault new task --project myproject --title "Build API"
agentvault new meeting --project myproject --title "Sprint Planning"
agentvault new source --title "Article" --url "https://example.com"
```

### AI Commands

```bash
# Ask a question (requires Ollama running)
agentvault ask "What have I decided about vector databases?"
agentvault ask "What are my open questions for Adacavo?"

# Configure AI in .agentvault/config.json:
{
  "ai": {
    "provider": "ollama",
    "baseUrl": "http://localhost:11434",
    "chatModel": "llama3.1",
    "embeddingModel": "nomic-embed-text"
  }
}
```

### Local API Server

```bash
# Start the local HTTP API (default: 127.0.0.1:47321)
agentvault serve
agentvault serve --port 8080

# Endpoints:
# GET  /health, /vault/status, /search?q=, /notes/:id
# GET  /projects, /recent, /stale, /git/status
# POST /vault/index, /notes, /capture, /ask
```

### MCP Server

```bash
# Start MCP server (stdio mode for Claude/Cursor/etc)
agentvault mcp serve

# HTTP mode
agentvault mcp serve --http --port 7777

# Available tools: agentvault.search, agentvault.read_note,
# agentvault.create_note, agentvault.create_decision,
# agentvault.capture, agentvault.summarize, agentvault.list_projects,
# agentvault.list_recent, agentvault.git_status
```

## Architecture

```
Markdown/YAML files (canonical source of truth)
        |
    Go Core Engine
        |
    +----------------+----------------+----------------+
    |                |                |                |
 SQLite + FTS5    CLI tool      Local HTTP API    MCP Server
 (index/cache)     |                |                |
              +----+----+      +----+----+      +----+----+
              |         |      |         |      |         |
           Wails   Terminal  Browser  Mobile   Claude   Other
          Desktop     CLI   Extension   App     Code    Agents
```

## Desktop App (Wails)

```bash
cd apps/desktop-wails

# Install frontend dependencies
cd frontend && npm install

# Install Wails CLI (requires Go 1.22+)
go install github.com/wailsapp/wails/v2/cmd/wails@latest

# Run in development mode
wails dev

# Build for production
wails build
```

### Desktop Features

- **Vault picker**: Open or create vaults on first launch
- **Search**: Full-text search with keyboard shortcuts (`/` to focus)
- **Editor**: CodeMirror 6 Markdown editor with live preview split
- **AI Panel**: Source-grounded Q&A with the vault (collapsible sidebar)
- **Project Dashboard**: Grouped notes by project
- **Decision Dashboard**: Track decision records and their status
- **Settings**: AI provider config, keyboard shortcuts, vault info
- **Keyboard-first**: `Ctrl+K` search, `Ctrl+N` new note, `Ctrl+S` save, `Ctrl+J` toggle AI

## Project Structure

```
agentvault/
├── core/                          # Go core engine (the product)
│   ├── cmd/agentvault/           # CLI entrypoint
│   │   ├── main.go               # Root command, shared helpers
│   │   ├── init.go               # agentvault init
│   │   ├── index.go              # agentvault index
│   │   ├── search.go             # agentvault search
│   │   ├── read.go               # agentvault read
│   │   ├── new.go                # agentvault new
│   │   ├── ask.go                # agentvault ask
│   │   ├── doctor.go             # agentvault doctor
│   │   ├── serve.go              # agentvault serve
│   │   └── mcp.go                # agentvault mcp serve
│   ├── internal/
│   │   ├── ai/                   # AI provider interface + Ollama client
│   │   ├── api/                  # Local HTTP API server + handlers
│   │   ├── config/               # Vault configuration (JSON)
│   │   ├── db/                   # SQLite wrapper + migrations
│   │   ├── doctor/               # Vault validation (7 checks)
│   │   ├── indexer/              # File scanner + FTS5 indexer
│   │   ├── markdown/             # YAML frontmatter parser
│   │   ├── mcp/                  # MCP server + 10 tools
│   │   ├── rag/                  # Retrieval-augmented generation pipeline
│   │   ├── search/               # FTS5 search with filters
│   │   ├── templates/            # Embedded Markdown templates
│   │   └── vault/                # Vault initialization + folder structure
│   ├── migrations/001_init.sql   # Full SQLite + FTS5 schema
│   └── go.mod
├── apps/
│   └── desktop-wails/            # Wails desktop app
│       ├── main.go               # Wails entry
│       ├── app.go                # Backend services exposed to frontend
│       ├── wails.json            # Wails config
│       └── frontend/             # React + TypeScript + Tailwind
│           ├── src/
│           │   ├── App.tsx
│           │   ├── components/   # 15 React components
│           │   ├── types/
│           │   └── styles/
│           └── package.json
├── templates/                    # User-customizable templates (future)
├── docs/                         # Documentation (future)
├── README.md
├── Makefile
└── LICENSE (Apache 2.0)
```

## Tech Stack

| Component | Technology |
|-----------|-----------|
| Core engine | Go 1.22+ |
| Database | SQLite + FTS5 (modernc.org/sqlite, pure Go) |
| Markdown | Goldmark |
| CLI | Cobra |
| Desktop | Wails v2 + React 19 + TypeScript |
| Editor | CodeMirror 6 |
| Styling | Tailwind CSS v3 |
| AI | Ollama API (local-first) |
| Protocol | MCP (Model Context Protocol) |

## Data Model

Files are the source of truth. SQLite is a rebuildable index.

```yaml
---
id: dec_2026_05_29_001
type: decision
title: Use Postgres and pgvector before Qdrant
project: myproject
status: active
tags: [architecture, ai-memory]
entities: [Postgres, pgvector, Qdrant]
created: 2026-05-29
updated: 2026-05-29
---

# Use Postgres and pgvector before Qdrant

## Decision
...
```

## Test Results

All 9 test packages pass (100+ tests):

```
ok  github.com/agentvault/core/internal/ai       (12 tests)
ok  github.com/agentvault/core/internal/api       (11 tests)
ok  github.com/agentvault/core/internal/db        (8 tests)
ok  github.com/agentvault/core/internal/doctor    (7 checks + integration)
ok  github.com/agentvault/core/internal/markdown  (3 tests)
ok  github.com/agentvault/core/internal/mcp       (36 tests, 10 tools)
ok  github.com/agentvault/core/internal/rag       (11 tests)
ok  github.com/agentvault/core/internal/search    (6 tests)
ok  github.com/agentvault/core/internal/templates (12 tests)
```

## Roadmap

### Completed ✅

- [x] Go core engine with CLI
- [x] Vault initialization with folder structure
- [x] Markdown + YAML frontmatter parser
- [x] SQLite database with migrations
- [x] FTS5 full-text search engine
- [x] Incremental indexer with content hashing
- [x] Note templates (note, decision, task, meeting, source)
- [x] `agentvault init`, `index`, `search`, `read`, `new`, `doctor`
- [x] AI/RAG pipeline with Ollama provider
- [x] `agentvault ask` with source-grounded answers
- [x] Local HTTP API server (13 endpoints, auth, CORS)
- [x] `agentvault serve` with graceful shutdown
- [x] MCP server with 10 tools (stdio + HTTP)
- [x] `agentvault mcp serve`
- [x] Wails desktop app with React frontend
- [x] CodeMirror 6 Markdown editor with preview
- [x] Search view with keyboard shortcuts
- [x] AI ask panel (collapsible sidebar)
- [x] Project and decision dashboards
- [x] Settings view

### Upcoming

- [ ] Browser extension (clip pages, send to vault)
- [ ] Expo mobile app (capture-first)
- [ ] Importers (Obsidian, Markdown folders)
- [ ] Starter templates (Founder OS, Developer OS)
- [ ] Sync (Git, Syncthing)
- [ ] Vector search with embeddings
- [ ] Additional AI providers (OpenAI, Anthropic)
- [ ] Team workspaces (paid)

## License

Apache 2.0 — see [LICENSE](LICENSE)

## Contributing

Contributions welcome! The project follows standard Go conventions and uses `go test ./...` for testing.

```bash
# Run tests
cd core && go test ./...

# Build
cd core && go build -o ../bin/agentvault ./cmd/agentvault

# Format
cd core && gofmt -w .
```

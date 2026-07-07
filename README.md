# AgentVault

AgentVault is a local-first knowledge operating system for notes, decisions,
research, tasks, and agent-readable context.

Your Markdown files are the source of truth. SQLite is a rebuildable index and
cache that powers search, retrieval, the local HTTP API, MCP tools, and the
desktop, web, browser-extension, and mobile clients.

## Table of Contents

- [Overview](#overview)
- [Quick Start](#quick-start)
- [CLI Reference](#cli-reference)
- [AI Configuration](#ai-configuration)
- [Local HTTP API](#local-http-api)
- [MCP Server](#mcp-server)
- [Clients](#clients)
- [Tech Stack](#tech-stack)
- [Architecture](#architecture)
- [Project Structure](#project-structure)
- [Development](#development)
- [Roadmap and Known Gaps](#roadmap-and-known-gaps)
- [License](#license)

## Overview

AgentVault keeps your knowledge in plain Markdown with YAML frontmatter. The
same files are readable by you, editable in any editor, and indexable by the Go
core for fast full-text, semantic, and hybrid search.

Key design choices:

- **Files first.** Markdown is the durable source of truth; the SQLite index can
  be rebuilt at any time.
- **Local by default.** The CLI, HTTP API, and desktop app run on your machine.
- **Agent-ready.** Source-grounded answers, MCP tools, and structured note types
  make the vault usable by AI assistants.
- **One shared contract.** Go and TypeScript clients share a single API contract
  so server and clients stay in sync. See [`packages/contract/`](packages/contract/)
  and [`core/internal/contract/`](core/internal/contract/).

## Quick Start

```bash
# Build the CLI
cd core
go build -o ../bin/agentvault ./cmd/agentvault

# Initialize a vault
../bin/agentvault init ../my-vault
cd ../my-vault

# Optional starter templates
../bin/agentvault init ../founder-vault --template founder
../bin/agentvault init ../developer-vault --template developer

# Create notes
../bin/agentvault new note --title "My first note"
../bin/agentvault new decision --project platform --title "Use Postgres"
../bin/agentvault new task --project platform --title "Build API"

# Index and search
../bin/agentvault index
../bin/agentvault search "Postgres"

# Validate vault health
../bin/agentvault doctor
```

## CLI Reference

| Command | Description |
| --- | --- |
| `agentvault init [path] [--template]` | Initialize a vault and optionally apply a starter template |
| `agentvault index [--force] [--rebuild] [--embed]` | Index Markdown files and optionally generate embeddings |
| `agentvault search <query> [--type] [--project] [--tag] [--status] [--vector] [--hybrid-weight] [--topk]` | Full-text or hybrid search with filters |
| `agentvault read <id-or-path>` | Read a note by ID or path |
| `agentvault new <type> --title <title>` | Create a structured note from a template |
| `agentvault ask <question>` | Ask a source-grounded question over indexed notes |
| `agentvault import <type> <source>` | Import Markdown or Obsidian notes (`type`: `markdown` or `obsidian`) |
| `agentvault doctor` | Validate vault health |
| `agentvault config get/set/show` | Manage vault configuration |
| `agentvault git status/diff/commit/log/init` | Run Git commands from the vault context |
| `agentvault serve` | Start the local HTTP API |
| `agentvault mcp serve` | Start the MCP server |

### Note Types

```bash
agentvault new note --title "My Idea"
agentvault new decision --project platform --title "Use Postgres"
agentvault new task --project platform --title "Build API"
agentvault new meeting --project platform --title "Sprint Planning"
agentvault new source --title "Article" --url "https://example.com"
agentvault new project --title "Platform"
```

## AI Configuration

By default, AgentVault uses Ollama at `http://localhost:11434`.

```json
{
  "ai": {
    "provider": "ollama",
    "baseUrl": "http://localhost:11434",
    "chatModel": "llama3.1",
    "embeddingModel": "nomic-embed-text"
  }
}
```

Supported providers: `ollama`, `openai`, `anthropic`, `openrouter`, and `mock`.
Cloud providers read `AGENTVAULT_API_KEY` when no key is stored in the vault
config.

```bash
agentvault ask "What have I decided about vector search?"
agentvault ask --provider openai --model gpt-4o-mini "Summarize open architecture decisions"
agentvault index --embed
```

## Local HTTP API

```bash
# Default bind address: 127.0.0.1:47321
agentvault serve
agentvault serve --port 8080
```

The server prints an auth token at startup. `GET` endpoints are open locally;
write endpoints require the token in either the `X-AgentVault-Token` header or
`Authorization: Bearer <token>`.

| Endpoint | Description |
| --- | --- |
| `GET /health` | Server health |
| `GET /auth/verify` | Verify a stored auth token |
| `GET /vault/status` | Vault status and indexed note count |
| `POST /vault/index` | Trigger indexing |
| `GET /search?q=...` | Search notes |
| `GET /notes/{id}` | Read a note |
| `POST /notes` | Create a note |
| `POST /capture` | Capture to inbox |
| `POST /ask` | Ask a source-grounded question |
| `GET /projects` | List projects (JSON `string[]`) |
| `GET /recent` | Recent notes |
| `GET /stale` | Stale notes |
| `GET /git/status` | Vault Git status |

For the full contract, including exact request/response shapes, auth rules,
CORS policy, and rate limits, see [`docs/API_CONTRACT.md`](docs/API_CONTRACT.md).

## MCP Server

```bash
# stdio mode for Claude, Cursor, and other MCP clients
agentvault mcp serve

# HTTP mode
agentvault mcp serve --http --port 7777
```

Registered tools:

- `agentvault.search`
- `agentvault.read_note`
- `agentvault.create_note`
- `agentvault.create_decision`
- `agentvault.create_task`
- `agentvault.capture`
- `agentvault.summarize`
- `agentvault.list_projects`
- `agentvault.list_recent`
- `agentvault.git_status`
- `agentvault.log_agent_run`
- `agentvault.ask`

## Clients

AgentVault ships with several first-party clients, all built against the shared
`@agentvault/contract` package.

| Client | Location | Notes |
| --- | --- | --- |
| **Desktop** | `apps/desktop-wails/` | Wails v2 app with a React/TypeScript frontend. Vault picker, search, editor, dashboards, settings, and AI panel. |
| **Web** | `apps/web-local/` | React/Vite client for the local HTTP API. |
| **Browser extension** | `apps/browser-extension/` | Manifest V3 extension for page capture and vault search through the local API. |
| **Mobile** | `apps/mobile-expo/` | Expo app with capture-first flows, local inbox, settings, search, and sync hooks. |

## Tech Stack

| Component | Technology |
| --- | --- |
| Core engine | Go 1.23+ (`core`), Go 1.25+ (`apps/desktop-wails`) |
| Database | SQLite + FTS5 via `modernc.org/sqlite` |
| Markdown | YAML frontmatter |
| CLI | Cobra |
| Desktop | Wails v2.9.2 + React 18.3 + TypeScript |
| Web app | React 18.3 + Vite 8 + TypeScript |
| Browser extension | Manifest V3 + React 18.3 + Vite 8 |
| Mobile | Expo ~56.0 + React Native 0.85 + React 19.2 |
| Editor | CodeMirror 6 |
| Styling | Tailwind CSS 3.4 |
| AI providers | Ollama, OpenAI-compatible, Anthropic, OpenRouter, mock |
| Agent protocol | MCP |

## Architecture

```text
Markdown/YAML files
        |
        v
Go core engine
        |
        +--> SQLite + FTS5 + chunks
        +--> CLI
        +--> Local HTTP API
        +--> MCP server
        +--> Wails services
                 |
                 v
        Desktop, web, browser extension, mobile, and agent clients
```

## Project Structure

```text
agentvault/
├── core/                         # Go core engine and CLI
│   ├── cmd/agentvault/           # Cobra commands
│   ├── internal/
│   │   ├── ai/                   # AI provider interface and providers
│   │   ├── api/                  # Local HTTP API
│   │   ├── chunker/              # Markdown/text chunking
│   │   ├── config/               # Vault configuration
│   │   ├── contract/             # Shared Go API contract types
│   │   ├── db/                   # SQLite wrapper and migrations
│   │   ├── doctor/               # Vault validation
│   │   ├── embeddings/           # Embedding clients
│   │   ├── git/                  # Git CLI wrapper
│   │   ├── importers/            # Markdown and Obsidian importers
│   │   ├── indexer/              # File scanner and index writer
│   │   ├── markdown/             # Frontmatter/parser utilities
│   │   ├── mcp/                  # MCP server and tools
│   │   ├── rag/                  # Retrieval-augmented generation pipeline
│   │   ├── search/               # FTS and vector search
│   │   ├── templates/            # Note and starter templates
│   │   ├── vault/                # Vault folder layout
│   │   └── vectors/              # Vector math
│   └── migrations/001_init.sql
├── apps/
│   ├── desktop-wails/            # Wails v2 desktop app
│   ├── web-local/                # React/Vite local web app
│   ├── browser-extension/        # MV3 browser extension
│   └── mobile-expo/              # Expo mobile app
├── packages/
│   └── contract/                 # Shared TypeScript API contract
├── docs/
│   ├── API_CONTRACT.md
│   ├── CODEBASE_ANALYSIS.md
│   └── IMPROVEMENT_PLAN.md
├── Makefile
└── README.md
```

## Development

The repository is a monorepo. The Makefile provides the common tasks.

```bash
# Build the CLI
make build

# Run Go tests
make test

# Run Go linting (vet + fmt)
make lint

# Verify the shared API contract across all clients
make contract-check
```

### Frontend clients

```bash
# Web app
cd apps/web-local
npm ci
npm run build

# Browser extension
cd apps/browser-extension
npm ci
npm run build

# Desktop frontend
cd apps/desktop-wails/frontend
npm ci
npm run build

# Mobile type-check
cd apps/mobile-expo
npm ci
npm run typecheck
```

### Desktop app

The Wails desktop app requires GTK and WebKit2GTK 4.1 development libraries.
On Ubuntu 24.04+ and 26.04, build with the `webkit2_41` tag:

```bash
make desktop
```

For live development:

```bash
make desktop-dev
```

## Roadmap and Known Gaps

AgentVault is an early application with a working CLI, API, MCP server, desktop
app, web app, browser extension, and mobile scaffold. Current planning and
detailed status live in:

- [`docs/CODEBASE_ANALYSIS.md`](docs/CODEBASE_ANALYSIS.md)
- [`docs/IMPROVEMENT_PLAN.md`](docs/IMPROVEMENT_PLAN.md)

Notable remaining work:

- Packaging, release artifacts, and installation paths are not yet defined.

CI runs `make test`, `make contract-check`, frontend builds, mobile type-checks,
and the desktop Go build with the `webkit2_41` tag.

## License

Apache 2.0. See [LICENSE](LICENSE).

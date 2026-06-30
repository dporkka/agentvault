# AgentVault

AgentVault is a local-first knowledge operating system for notes, decisions, research, tasks, and agent-readable context.

Markdown files are the source of truth. SQLite is a rebuildable index and cache used for search, retrieval, API responses, and UI clients.

## Current Status

AgentVault is an early application with a working Go core, CLI, local HTTP API, MCP server, Wails desktop app, standalone web app, browser extension, and Expo mobile app scaffold. The codebase is ahead of the original phase-plan documents, so current planning now lives in:

- [Codebase analysis](docs/CODEBASE_ANALYSIS.md)
- [Improvement plan](docs/IMPROVEMENT_PLAN.md)

Notable current gaps:

- Desktop bundle size still triggers one Vite chunk-size warning on the `codemirror-vendor` chunk; the warning is intentionally budgeted in `apps/desktop-wails/frontend/vite.config.ts`.
- Desktop app does not yet mirror the browser extension's capture-sync status.

Go verification runs locally with the toolchain on `PATH`; GitHub Actions is configured to run Go 1.23 tests, vet, and builds.

## Install

Install the AgentVault CLI with the install script:

```bash
curl -fsSL https://raw.githubusercontent.com/agentvault/agentvault/main/scripts/install.sh | sh
```

The script downloads the latest release for your platform from GitHub and installs `agentvault` into `$HOME/.local/bin`. Make sure that directory is on your `PATH`.

You can also download a pre-built binary manually from the [GitHub Releases](https://github.com/agentvault/agentvault/releases) page. Release archives follow the pattern:

```text
https://github.com/agentvault/agentvault/releases/download/v0.1.0/agentvault_v0.1.0_linux_amd64.tar.gz
```

Linux and macOS archives are `.tar.gz`; Windows archives are `.zip`. Extract the archive and place the `agentvault` binary (or `agentvault.exe` on Windows) somewhere on your `PATH`.

## Features

- **Local-first storage**: Durable Markdown files with YAML frontmatter.
- **SQLite index**: Rebuildable SQLite schema with FTS5, tags, captures, chunks, and agent-run tables.
- **CLI workflow**: Initialize vaults, create notes, index, search, read, ask, import, validate, configure, serve, and use vault Git commands.
- **AI retrieval**: Source-grounded `agentvault ask` with Ollama, OpenAI-compatible, Anthropic, OpenRouter, and mock providers.
- **Semantic search foundation**: Chunking, embeddings, vector utilities, and hybrid search support in the core.
- **MCP integration**: stdio and HTTP transports with search/read/create/capture/list/git/audit tools.
- **Local HTTP API**: Localhost API for app clients, protected write endpoints, and CORS for local and extension origins.
- **Desktop app**: Wails v2 backend with React/TypeScript frontend, vault picker, search, editor, dashboards, settings, and AI panel.
- **Standalone web app**: React/Vite client for the local API.
- **Browser extension**: MV3 extension for page capture and vault search through the local API.
- **Mobile app**: Expo app with capture-first flows, local inbox, settings, search, and sync hooks.
- **Importers and starters**: Markdown/Obsidian importers and starter vault templates for founder, developer, agent-memory, and research workflows.

## Quick Start

```bash
# Initialize a vault
agentvault init ./my-vault
cd ./my-vault

# Optional starter templates
agentvault init ./founder-vault --template founder
agentvault init ./developer-vault --template developer

# Create notes
agentvault new note --title "My first note"
agentvault new decision --project platform --title "Use Postgres"
agentvault new task --project platform --title "Build API"

# Index and search
agentvault index
agentvault search "Postgres"

# Validate
agentvault doctor

# Start the web app
agentvault serve
```

Running `agentvault serve` starts the local API at `http://127.0.0.1:47321` and opens it in your browser automatically. Use `agentvault serve --no-open` to skip opening the browser.

## CLI Reference

| Command | Description |
| --- | --- |
| `agentvault init [path] [--template]` | Initialize a vault and optional starter template |
| `agentvault index [--force] [--rebuild] [--path] [--embed]` | Index Markdown files and optionally generate embeddings |
| `agentvault search <query> [--type] [--project] [--tag] [--status] [--vector] [--hybrid-weight] [--topk]` | Full-text or hybrid search with filters |
| `agentvault read <id-or-path>` | Read a note by ID or path |
| `agentvault new <type> --title <title>` | Create a structured note |
| `agentvault ask <question>` | Ask a source-grounded question over indexed notes |
| `agentvault import <markdown|obsidian> <source>` | Import external Markdown or Obsidian notes |
| `agentvault doctor` | Validate vault health |
| `agentvault config get/set/show` | Manage vault configuration |
| `agentvault git status/diff/commit/log/init` | Use Git from the vault context |
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

### AI Configuration

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

Provider options in the core include `ollama`, `openai`, `anthropic`, `openrouter`, and `mock`. Cloud providers can read `AGENTVAULT_API_KEY` when no API key is stored in the vault config.

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

The server prints an auth token at startup. `GET` endpoints are open locally; write endpoints require `X-AgentVault-Token` or `Authorization: Bearer <token>`.

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
| `POST /ask` | Ask a source-grounded question using the configured RAG provider |
| `GET /projects` | List projects (returns a JSON `string[]`) |
| `GET /recent` | Recent notes |
| `GET /stale` | Stale notes |
| `GET /git/status` | Real vault Git status from `internal/git` |

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
        Desktop, web, browser extension, mobile, agent clients
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
│   │   ├── db/                   # SQLite wrapper and migrations
│   │   │   └── migrations/001_init.sql
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
├── apps/
│   ├── desktop-wails/            # Wails v2 desktop app
│   ├── web-local/                # React/Vite local web app
│   ├── browser-extension/        # MV3 browser extension
│   └── mobile-expo/              # Expo mobile app
├── docs/
│   ├── API_CONTRACT.md
│   ├── CODEBASE_ANALYSIS.md
│   └── IMPROVEMENT_PLAN.md
├── Makefile
└── README.md
```

## Tech Stack

| Component | Technology |
| --- | --- |
| Core engine | Go 1.23 |
| Database | SQLite + FTS5 via `modernc.org/sqlite` |
| Markdown | YAML frontmatter parser and Markdown templates |
| CLI | Cobra |
| Desktop | Wails v2 + React 18 + TypeScript |
| Web app | React 18 + Vite + TypeScript |
| Browser extension | Manifest V3 + React 18 + Vite |
| Mobile | Expo 56 + React Native 0.85 |
| Editor | CodeMirror 6 |
| Styling | Tailwind CSS v3 |
| AI providers | Ollama, OpenAI-compatible, Anthropic, OpenRouter |
| Agent protocol | MCP |

## Development

### Build from source

```bash
# Go core
cd core
go test ./...
go vet ./...
go build -o ../bin/agentvault ./cmd/agentvault
```

### App builds

```bash
# Local web app
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
npx tsc --noEmit
```

The repository also includes GitHub Actions for the Go core, frontend builds, mobile type-checking, and desktop Go build with the `webkit2_41` tag used on Ubuntu.

## License

Apache 2.0. See [LICENSE](LICENSE).

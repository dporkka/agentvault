# AgentVault Codebase Analysis

Last reviewed: 2026-06-28

## Evidence Reviewed

- Current worktree status: clean before documentation changes.
- Source tree, README, historical phase plans, Wails design note, CI workflow, package manifests, CLI command registrations, API route registrations, MCP tool registrations, frontend clients, and representative core packages.
- Verification run in this shell:
  - `npm run build` in `apps/web-local`: pass.
  - `npm run build` in `apps/browser-extension`: pass.
  - `npm run build` in `apps/desktop-wails/frontend`: pass with a Vite chunk-size warning for the desktop bundle (`codemirror-vendor` chunk ~605 kB).
  - `npx tsc --noEmit` in `apps/mobile-expo`: pass.
  - `go test ./...` in `core`: pass.
  - `go vet ./...` in `apps/desktop-wails`: pass.
  - `make contract-check`: pass.

## Product Shape

AgentVault is a local-first Markdown vault with agent-facing retrieval and multiple local clients. The core design is sound:

- Files remain canonical and user-owned.
- SQLite stores rebuildable derived state for search, metadata, captures, chunks, and agent-run audit data.
- The Go core exposes the product through CLI commands, a local HTTP API, an MCP server, and Wails desktop services.
- Web, extension, and mobile clients communicate with the local HTTP API.

## Architecture Map

### Go Core

`core/cmd/agentvault` registers the CLI surface:

- Vault lifecycle: `init`, `doctor`, `config`.
- Knowledge workflow: `new`, `index`, `search`, `read`, `ask`, `import`.
- Service surfaces: `serve`, `mcp serve`.
- Vault versioning: `git status`, `git diff`, `git commit`, `git log`, `git init`.

`core/internal` contains the domain packages:

- `vault`, `config`, `db`, `migrations`: vault structure, configuration, and SQLite schema.
- `markdown`, `templates`, `importers`: file parsing, note/starter templates, Markdown and Obsidian import.
- `indexer`, `search`, `chunker`, `embeddings`, `vectors`: indexing, FTS5 search, chunking, embeddings, vector and hybrid search.
- `ai`, `rag`: provider abstraction and source-grounded answer pipeline.
- `api`: local HTTP API with auth middleware and CORS for local clients/extensions.
- `mcp`: MCP protocol handling and 12 tools.
- `git`: wrapper around the system Git CLI.
- `doctor`: vault validation checks.

### Client Applications

- `apps/desktop-wails`: Wails v2 desktop app. The frontend calls Go services directly through generated Wails bindings.
- `apps/web-local`: React/Vite local web app. It calls `http://127.0.0.1:47321`.
- `apps/browser-extension`: Manifest V3 extension with popup, content script, background service worker, and local API helpers.
- `apps/mobile-expo`: Expo mobile app with local inbox storage, capture-first flows, settings, and local API helpers.

### CI

`.github/workflows/ci.yml` runs:

- Go `vet`, `test`, and CLI build for `core`.
- Frontend builds for web, browser extension, and desktop frontend.
- Expo TypeScript type-check.
- Desktop Go build with GTK/WebKit dependencies and the `webkit2_41` tag.

## Current Capability Assessment

### Strong Foundations

- The file-first storage model is clear and well represented across the core.
- The SQLite schema covers the expected retrieval model: files, notes, tags, links, chunks, captures, agent runs, and FTS5.
- CLI coverage is broad for a v0.1 application.
- The AI provider abstraction already supports local and cloud providers.
- Chunking, embeddings, vector math, and hybrid search are implemented enough to guide the next search improvements.
- MCP is not just a placeholder; it has tools and tests for meaningful agent workflows.
- Browser, web, desktop, and mobile clients build/type-check locally.

### Drift And Risk

1. API and client contracts are now unified under a single source on both
   sides of the boundary.
   - The Go server, the Wails desktop Go bridge, and the four TypeScript
     clients all share the `SearchResult`, `NoteDetail`, `IndexResult`,
     `IndexError`, `Answer`, `Source`, `VaultStatus`, `GitStatus`, and
     `GitModifiedFile` types from `core/internal/contract/` and
     `packages/contract/`. There are no longer hand-written duplicates in
     any of the four clients.
   - `GET /projects` returns a bare `string[]`, matching the web,
     extension, and mobile clients and the other list endpoints
     (`/search`, `/recent`, `/stale`).
   - `POST /ask` uses the core RAG pipeline and returns the structured
     `Answer` shape; API tests assert a real (non-stub) answer plus a
     `sources` array.
   - `GET /git/status` reports real vault state via `internal/git.Status`,
     including the not-a-repo case.
   - The `make contract-check` CI gate fails on any future drift
     (snake_case keys, hard-coded base URLs, or non-matching client
     type imports).

2. RAG behavior is now consolidated.
   - `cmd/agentvault/ask.go` (CLI), `POST /ask` (API), `AIService.Ask`
     (desktop), and `agentvault.ask` (MCP) all build `rag.New(searcher,
     provider)` and call `pipeline.Ask`. Prompt construction and answer
     parsing live in `internal/rag` alone; the CLI's former duplicate
     search/prompt/parse flow is gone.

3. Vector/hybrid search is now exposed end-to-end.
   - CLI `search` supports `--vector`, `--hybrid-weight`, and `--topk`.
   - API `/search` accepts `vector`, `hybrid_weight`, and `topk` query params and
     falls back to FTS when embeddings are missing or the query is empty.
   - Web, browser-extension, mobile, and desktop search UIs all include a
     vector toggle and hybrid-weight control.
   - The `@agentvault/contract` TypeScript client maps camelCase `hybridWeight`
     to the server's `hybrid_weight` query key.

4. Write operations now refresh derived state.
   - The API `handleCreateNote`/`handleCapture` and the MCP
     `createNote`/`handleCapture` paths kick off a non-blocking
     `indexer.Index(IndexOptions{Path: relPath})` goroutine right after
     writing the file, so a newly created note or capture is searchable
     without a manual `agentvault index` step.

5. Frontend code is shared through one contract package.
   - Web, extension, mobile, and desktop each consume
     `@agentvault/contract` via TypeScript path mappings (and Metro
     `watchFolders` for mobile). The package is a zero-dependency,
     source-of-truth for every server-facing type.

6. Local client token onboarding is now explicit.
   - Web: `ConnectionModal` prompts for server URL + token when the stored
     token is missing or invalid, and `VaultStatus` surfaces "Not authenticated".
   - Extension: popup shows token status (valid / invalid / missing) and
     explains how to copy the token from `agentvault serve`.
   - Mobile: Settings screen has a "Verify Token" button that uses
     `GET /auth/verify` and reports the result.
   - The `@agentvault/contract` client exposes `verifyAuth()` so every local
     client can check a stored token without making a write request.

7. Desktop bundle splitting reduced the main chunk but one vendor chunk is still large.
   - `vite.config.ts` now splits `react-vendor`, `editor-vendor`, and a generic
     `vendor` chunk, so the main `index` chunk is no longer the >500 kB offender.
   - The `codemirror-vendor` chunk still triggers a Vite chunk-size warning
     (~605 kB after minification). This is a known P2 budget item.

8. Documentation was stale before this pass.
   - README listed completed areas as upcoming.
   - Historical phase plans described work already present in the code.
   - The desktop design note still reflected an earlier target rather than the exact current implementation.

9. Packaging and release story is still immature.
   - CI builds the pieces, but the repository does not yet define release artifacts, installation paths, compatibility matrix, or user setup flows for desktop, extension, and mobile.

## Verification Snapshot

| Gate | Result | Notes |
| --- | --- | --- |
| `apps/web-local npm run build` | Pass | Vite production build succeeded. |
| `apps/browser-extension npm run build` | Pass | Vite production build succeeded. |
| `apps/desktop-wails/frontend npm run build` | Pass with warning | `codemirror-vendor` chunk is ~605 kB after minification. |
| `apps/mobile-expo npx tsc --noEmit` | Pass | No TypeScript output. |
| `core go test ./...` | Pass | `go vet ./...` clean. CI installs Go 1.23. |
| `apps/desktop-wails go vet ./...` | Pass | Wails bindings regenerated after adding vector params to `NoteService.Search`. |
| `make contract-check` | Pass | No snake_case keys or hard-coded base URLs detected in clients.

## Recommended Engineering Direction

Contract stability for the core endpoints is now enforced in one place on
both sides of the boundary (Go: `core/internal/contract/`; TypeScript:
`packages/contract/`). The next highest-leverage path is:

1. Done — make the HTTP API contract match clients and tests for `/projects`, `/ask`, `/git/status`, `/notes/{id}`, and `/vault/index`.
2. Done — share one TypeScript contract source across all clients.
3. Done — route all AI ask behavior (CLI, API, desktop, MCP) through one core RAG service (`internal/rag.Pipeline`).
4. Done — writes become searchable predictably (API and MCP auto-index after create/capture).
5. Done — consolidate note→folder resolution into `templates.FolderRelForType`/`FolderPathForType` as the single source used by CLI, API, MCP, and desktop.
6. Done — expose vector/hybrid search end-to-end across CLI, API, web, extension, mobile, and desktop.
7. Done — add explicit token onboarding/status flows for web, extension, and mobile local clients.
8. Next — improve packaging, release readiness, diagnostics, and desktop bundle budgets across the app surfaces.

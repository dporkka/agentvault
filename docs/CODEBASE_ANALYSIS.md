# AgentVault Codebase Analysis

Last reviewed: 2026-06-01

## Evidence Reviewed

- Current worktree status: clean before documentation changes.
- Source tree, README, historical phase plans, Wails design note, CI workflow, package manifests, CLI command registrations, API route registrations, MCP tool registrations, frontend clients, and representative core packages.
- Verification run in this shell:
  - `npm run build` in `apps/web-local`: pass.
  - `npm run build` in `apps/browser-extension`: pass.
  - `npm run build` in `apps/desktop-wails/frontend`: pass with a Vite chunk-size warning for the desktop bundle.
  - `npx tsc --noEmit` in `apps/mobile-expo`: pass.
  - `go test ./...` in `core`: blocked because `go` is not available on `PATH` in this shell.

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
- `mcp`: MCP protocol handling and 11 tools.
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

1. API and client contracts are now aligned for the core endpoints, with remaining type-sharing work.
   - `GET /projects` now returns a bare `string[]`, matching the web, extension, and mobile clients and the other list endpoints (`/search`, `/recent`, `/stale`).
   - `POST /ask` uses the core RAG pipeline and returns the structured `Answer` shape; API tests assert a real (non-stub) answer plus a `sources` array.
   - `GET /git/status` now reports real vault state via `internal/git.Status`, including the not-a-repo case.
   - Clients still hand-maintain their TypeScript contracts; a shared or generated type source remains the next contract-stability step.

2. RAG behavior is duplicated.
   - `internal/rag.Pipeline`, `cmd/agentvault/ask.go`, desktop `AIService`, and API `/ask` do not share one implementation path.
   - This increases the chance that CLI, desktop, and web answers diverge.

3. Search capabilities are unevenly exposed.
   - CLI ask can use hybrid search when embeddings exist.
   - API search only exposes FTS filters.
   - Web/extension/mobile clients do not expose vector/hybrid search options.

4. Write operations do not consistently refresh derived state.
   - API and MCP create/capture flows write files, but indexing is a separate step.
   - Users can create data that is not searchable until a manual index run.

5. Frontend code is duplicated by surface.
   - Web, extension, mobile, and desktop each carry their own local API/service assumptions.
   - There is no generated API schema or shared TypeScript contract package.

6. Documentation was stale before this pass.
   - README listed completed areas as upcoming.
   - Historical phase plans described work already present in the code.
   - The desktop design note still reflected an earlier target rather than the exact current implementation.

7. Packaging and release story is still immature.
   - CI builds the pieces, but the repository does not yet define release artifacts, installation paths, compatibility matrix, or user setup flows for desktop, extension, and mobile.

## Verification Snapshot

| Gate | Result | Notes |
| --- | --- | --- |
| `apps/web-local npm run build` | Pass | Vite production build succeeded. |
| `apps/browser-extension npm run build` | Pass | Vite production build succeeded. |
| `apps/desktop-wails/frontend npm run build` | Pass with warning | Bundle warning: one desktop JS chunk is above 500 kB after minification. |
| `apps/mobile-expo npx tsc --noEmit` | Pass | No TypeScript output. |
| `core go test ./...` | Pass | Run with the toolchain under `$HOME/.local/go`; `go vet ./...` also clean. CI installs Go 1.23. |

## Recommended Engineering Direction

Contract stability for the core endpoints is now in place (`/projects`, `/ask`, `/git/status` match clients and have shape tests). The next highest-leverage path is:

1. Done — make the HTTP API contract match clients and tests for `/projects`, `/ask`, and `/git/status`.
2. Finish routing all AI ask behavior through one core RAG service (CLI still has a duplicate flow).
3. Ensure writes become searchable predictably (auto-index or explicit "index needed" state).
4. Consolidate shared API types so clients stop hand-maintaining contracts.
5. Then improve UX, packaging, and release readiness across the app surfaces.

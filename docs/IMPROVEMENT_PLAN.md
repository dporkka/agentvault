# AgentVault Improvement Plan

Last updated: 2026-06-10

## Goal

Make AgentVault dependable as a local-first knowledge app across CLI, desktop, web, extension, mobile, and agent clients. The immediate focus should be correctness and contract stability, then UX completeness, then release readiness.

## Recently Completed (Phase 0)

- `POST /ask` is wired to `internal/rag.Pipeline` with the configured AI provider and returns the structured `Answer` shape (JSON-tagged); API tests assert a real answer plus a `sources` array and reject the old stub.
- `GET /projects` now returns a bare JSON `string[]`, matching the web, extension, and mobile clients and the other list endpoints; the API test asserts the array shape.
- `GET /git/status` now reports real vault state via `internal/git.Status` (branch, clean flag, ahead/behind, modified and untracked files), including a truthful `isGitRepo: false` for non-versioned vaults; API tests cover both the repo and non-repo paths.
- The full HTTP API surface is documented in [API_CONTRACT.md](API_CONTRACT.md) — every route's auth, request, and exact response shape, plus the remaining client casing drift to resolve in the type-sharing PR. A `/stale` shape test was added so every endpoint named in the Phase 0 exit criteria now has a shape assertion.

## Priority Backlog

| Priority | Work | Why it matters | Evidence |
| --- | --- | --- | --- |
| P0 | Share or generate API TypeScript types from one contract source | Each client still hand-maintains its own request/response types, which is how the `/projects` drift happened. | Web, extension, and mobile each redeclare API shapes. |
| P1 | Reuse one RAG/search service across CLI, API, desktop, and MCP | Avoids behavior drift and repeated prompt/parse logic. | API and core now use `internal/rag.Pipeline`; CLI still has duplicate search/prompt/parse flow. |
| P1 | Auto-index or clearly queue indexing after writes | Created notes/captures should become searchable without confusing manual steps. | API/MCP write files separately from indexing. |
| P1 | Expose vector/hybrid search consistently | The core has vector capabilities, but clients mostly expose plain FTS. | `index --embed` and search vector helpers exist. |
| P1 | Generate or share API TypeScript types | Reduces cross-app drift and duplicated hand-written contracts. | Each client owns its API assumptions. |
| P1 | Improve token onboarding for local clients | Auth exists, but user setup is manual and easy to misconfigure. | Server prints token; clients store token in local storage/settings. |
| P2 | Reduce desktop bundle size | Improves startup and packaging. | Desktop frontend build warns about a chunk over 500 kB. |
| P2 | Define release/install paths | Converts builds into usable distribution. | CI builds pieces but no release artifact flow is documented. |
| P2 | Expand doctor and diagnostics | Makes local-first support easier. | Doctor exists; app-surface and API-contract checks are not yet included. |

## Phase 0 - Contract Stabilization

Target: 1-3 focused days.

Deliverables:

- Done — `GET /projects` returns the bare `string[]` shape the clients already consume.
- Done — `POST /ask` is covered by API tests and returns the structured RAG response shape.
- Done — hard-coded `GET /git/status` replaced with `internal/git.Status`.
- Done — API tests assert the exact JSON shapes for `/projects`, `/ask`, `/git/status`, and `/stale`.
- Done — added a shared API contract document at [API_CONTRACT.md](API_CONTRACT.md), derived from handlers, middleware, and tests.

Exit criteria:

- Go API tests prove `/projects`, `/ask`, `/git/status`, search, notes, capture, recent, and stale response shapes.
- `npm run build` remains green for web, extension, and desktop frontend.
- Mobile TypeScript remains green.

## Phase 1 - Shared Core Services

Target: 3-5 focused days.

Deliverables:

- Route CLI `ask`, API `/ask`, desktop `AIService`, and any MCP ask/summarize expansion through one RAG service.
- Keep prompt construction and answer parsing in one package.
- Support FTS-only, vector-only, and hybrid modes behind one search interface.
- Expose vector/hybrid knobs through API query params with safe defaults.
- Ensure create/capture operations either reindex affected files immediately or return an explicit "index needed" state.

Exit criteria:

- One service owns RAG behavior.
- Tests cover no-result, provider-error, source-citation, vector fallback, and timeout paths.
- Newly created notes become searchable through the expected user flow.

## Phase 2 - Client Reliability And UX

Target: 1-2 weeks.

Deliverables:

- Add a first-run connection/token flow for web, extension, and mobile.
- Show server health, vault status, auth status, and indexing status in clients.
- Make capture sync states explicit: unsynced, syncing, synced, failed.
- Align project pickers and note filters across web, extension, mobile, and desktop.
- Share request/response types or generate them from one contract source.
- Improve desktop bundle splitting for CodeMirror/markdown-heavy paths.

Exit criteria:

- A new user can start `agentvault serve`, paste/store the token, capture a page, and find it in search without reading source code.
- Client errors distinguish "server unavailable", "unauthorized", "vault not indexed", and "no results".
- Desktop build no longer emits the large-chunk warning, or the warning is intentionally budgeted.

## Phase 3 - Vault Lifecycle And Data Quality

Target: 1-2 weeks after Phase 2.

Deliverables:

- Expand `doctor` to validate API auth setup, index freshness, duplicate IDs, broken links, orphan chunks, and embedding availability.
- Embed migrations with `go:embed` instead of relying on runtime relative paths, while preserving the current fallback.
- Improve import previews: dry-run mode, duplicate summary, attachment summary, and frontmatter normalization report.
- Add safe Git workflow helpers for common vault operations without auto-committing unexpectedly.
- Add benchmarks for indexing, search, vector search, and import on representative vault sizes.

Exit criteria:

- Users can understand vault health from one command.
- Imports can be previewed before writing.
- Migration behavior is reliable from source builds, installed binaries, and desktop packaging.

## Phase 4 - Release Readiness

Target: after the app surfaces are stable.

Deliverables:

- Define release artifacts: CLI binaries, desktop installers, browser extension package, and mobile distribution strategy.
- Add install/update documentation for each platform.
- Add compatibility matrix for OS, Go, Node, Wails, Expo, browsers, and local AI providers.
- Add smoke tests for packaged CLI and desktop artifacts.
- Document security boundaries for localhost API, auth token handling, CORS, and extension permissions.

Exit criteria:

- A tagged release can be built by CI.
- Users can install and run at least CLI + desktop + browser extension from documented artifacts.
- Security expectations are documented before wider distribution.

## Near-Term Suggested First PR

The original contract-stabilization PR (Phase 0) is now complete:

1. Done — `GET /projects` returns a bare `string[]` matching all clients and tests.
2. Done — `POST /ask` is wired to the shared RAG pipeline.
3. Done — `GET /git/status` is wired to `internal/git.Status`.
4. Done — API response-shape tests cover `/projects`, `/ask`, `/git/status`, and `/stale`.
5. Done — docs updated to reflect the endpoint shapes, including the full [API_CONTRACT.md](API_CONTRACT.md).

Next suggested PR: consolidate the hand-written client API types into one shared or generated contract source so future endpoint changes cannot silently drift the web, extension, and mobile clients again. [API_CONTRACT.md](API_CONTRACT.md) already specifies the exact targets — start with the three drifts in its "Known contract drift" section: camelCase `json` tags on `search.Result` (affects `/search`, `/recent`, `/stale`), the `/vault/status` `indexedAt` vs `version` field name, and camelCasing `IndexResult`.

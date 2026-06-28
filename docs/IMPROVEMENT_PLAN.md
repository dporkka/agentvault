# AgentVault Improvement Plan

Last updated: 2026-06-28

## Goal

Make AgentVault dependable as a local-first knowledge app across CLI, desktop, web, extension, mobile, and agent clients. The immediate focus should be correctness and contract stability, then UX completeness, then release readiness.

## Recently Completed (Phase 0+)

Phase 0+ was the "consolidate shared client types" PR. It introduces a single
canonical contract on both sides of the boundary: `core/internal/contract/` for
Go (imported by `core/internal/api`, `core/internal/search`, `core/internal/indexer`,
`core/internal/rag`, and `apps/desktop-wails/app.go`) and `packages/contract/`
for TypeScript (consumed by `apps/web-local`, `apps/browser-extension`,
`apps/mobile-expo`, and `apps/desktop-wails/frontend` via path mappings plus
Metro `watchFolders`). The shared types are the new `SearchResult`,
`NoteDetail`, `IndexResult`, `IndexError`, `Answer`, `Source`, `VaultStatus`,
`GitStatus`, and `GitModifiedFile`. The Wails desktop Go side now aliases the
shared `VaultStatus` (using `isVault` rather than the previous `isOpen`),
`Note` (now `content`, not `body`), and `SearchResult` (now carries `status`
and `score`). CI enforces the contract with a new `make contract-check` job
that runs `tsc --noEmit` on every client and greps for snake_case keys and
hard-coded base URLs; the four web/mobile/desktop builds no longer ship
duplicate type files. Server-side, `core/internal/api/handlers.go` now uses
`contract.NoteDetail` and `contract.GitStatus`/`contract.GitModifiedFile` for
the `/notes/{id}` and `/git/status` handlers, replacing the previous
hand-written `map[string]interface{}` shapes.

## Recently Completed (Phase 1)

Phase 1's two highest-leverage P1 items landed alongside the contract work:

- **One RAG service across all surfaces.** `agentvault ask` (CLI), `POST /ask`
  (API), `AIService.Ask` (desktop), and `agentvault.ask` (MCP) all build
  `rag.New(searcher, provider)` and call `pipeline.Ask`. Prompt construction and
  answer parsing live in `internal/rag` alone — the CLI's former duplicate
  search/prompt/parse flow is gone.
- **Auto-index after writes.** The API `handleCreateNote`/`handleCapture` and
  the MCP `createNote`/`handleCapture` paths kick off a non-blocking
  `indexer.Index(IndexOptions{Path: relPath})` goroutine right after writing the
  file, so a newly created note or capture is searchable without a manual
  `agentvault index` step.
- **One folder-resolution rule.** `templates.FolderRelForType` /
  `FolderPathForType` is the single source for where a note is written; the
  CLI, HTTP API, MCP server, and desktop app all route through it, replacing
  three divergent `folderForType` copies. Only meetings file under
  `20-projects/<project>`; for every other type the project is metadata, not a
  file location. Covered by `templates/folders_test.go`.

## Recently Completed (Phase 0)

- `POST /ask` is wired to `internal/rag.Pipeline` with the configured AI provider and returns the structured `Answer` shape (JSON-tagged); API tests assert a real answer plus a `sources` array and reject the old stub.
- `GET /projects` now returns a bare JSON `string[]`, matching the web, extension, and mobile clients and the other list endpoints; the API test asserts the array shape.
- `GET /git/status` now reports real vault state via `internal/git.Status` (branch, clean flag, ahead/behind, modified and untracked files), including a truthful `isGitRepo: false` for non-versioned vaults; API tests cover both the repo and non-repo paths.
- The full HTTP API surface is documented in [API_CONTRACT.md](API_CONTRACT.md) — every route's auth, request, and exact response shape, plus the remaining client casing drift to resolve in the type-sharing PR. A `/stale` shape test was added so every endpoint named in the Phase 0 exit criteria now has a shape assertion.

## Priority Backlog

| Priority | Work | Why it matters | Evidence |
| --- | --- | --- | --- |
| P1 | ~~Expose vector/hybrid search consistently~~ **Done** | Core vector capabilities are now surfaced in CLI, API, web, extension, mobile, and desktop. | CLI `--vector`/`--hybrid-weight`/`--topk`; API `/search` params; client UIs all expose toggle/weight. |
| P1 | ~~Improve token onboarding for local clients~~ **Done** | Web, extension, and mobile now prompt/verify the token printed by `agentvault serve`. | `ConnectionModal`, extension token status, mobile "Verify Token". |
| P2 | Reduce desktop bundle size | Improves startup and packaging. | Manual chunks split React/CodeMirror/markdown; `codemirror-vendor` chunk still ~605 kB. |
| P2 | Define release/install paths | Converts builds into usable distribution. | CI builds pieces but no release artifact flow is documented. |
| P2 | Expand doctor and diagnostics | Makes local-first support easier. | Doctor exists; app-surface and API-contract checks are not yet included. |
| P2 | Desktop settings/status parity | The desktop app added AI/index status in Settings; surface auth and capture-sync states next. | `SettingsView.tsx`, `IndexingStatus`, `AIStatus`. |

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

Target: complete.

Deliverables:

- Done — route CLI `ask`, API `/ask`, desktop `AIService`, and MCP `agentvault.ask` through one RAG service.
- Done — keep prompt construction and answer parsing in `internal/rag` only.
- Done — support FTS-only, vector-only, and hybrid modes behind one search interface.
- Done — expose vector/hybrid knobs through API query params with safe defaults.
- Done — ensure create/capture operations reindex affected files immediately (API and MCP kick off a non-blocking indexer goroutine).

Exit criteria:

- One service owns RAG behavior.
- Tests cover no-result, provider-error, source-citation, vector fallback, and timeout paths.
- Newly created notes become searchable through the expected user flow.
- Vector/hybrid search is wired end-to-end: the API searcher has an embedding
  client, the TypeScript contract emits the correct `hybrid_weight` query key,
  the CLI exposes `--vector`/`--hybrid-weight`/`--topk`, and every client UI
  (web, extension, mobile, desktop) has a vector toggle.

## Phase 2 - Client Reliability And UX

Target: 1-2 weeks.

Deliverables:

- Done — add a first-run connection/token flow for web, extension, and mobile.
- Partially done — show server health, vault status, auth status, and indexing status in clients. Web, extension, and mobile surface health/auth; desktop surfaces vault and AI/index status.
- Not started — make capture sync states explicit: unsynced, syncing, synced, failed.
- Done — align project pickers and note filters across web, extension, mobile, and desktop (all use the shared `@agentvault/contract` types and consistent filter sets).
- Done — share request/response types from one contract source (`@agentvault/contract`).
- Partially done — improve desktop bundle splitting for CodeMirror/markdown-heavy paths. The main chunk is no longer the offender; the `codemirror-vendor` chunk still triggers a warning.

Exit criteria:

- A new user can start `agentvault serve`, paste/store the token, capture a page, and find it in search without reading source code.
- Client errors distinguish "server unavailable", "unauthorized", "vault not indexed", and "no results".
- Desktop build no longer emits the large-chunk warning, or the warning is intentionally budgeted.

## Phase 2 Progress

The token-onboarding and local-client reliability work is complete:

- Web: `ConnectionModal` automatically prompts for server URL + token when the
  server is reachable but the stored token is invalid/missing. `VaultStatus`
  shows "Not authenticated" and opens the modal when clicked.
- Extension: Popup shows token status (valid / invalid / missing) and explains
  how to obtain the token from `agentvault serve`.
- Mobile: Settings screen has a "Verify Token" button that uses `/auth/verify`
  and reports the result.
- Docs: `API_CONTRACT.md` documents `/auth/verify` as the supported token-check
  mechanism.

What remains for Phase 2:

- Surface auth status and capture-sync states in the desktop app (the Wails
  desktop does not use the HTTP API, so it needs its own token/status plumbing
  or a Wails-backed status panel).
- Budget or eliminate the remaining desktop `codemirror-vendor` chunk warning
  (~605 kB after minification).

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

Phase 0 (contract stabilization), Phase 0+ (shared client types), Phase 1
(shared RAG service + auto-index + folder consolidation), and the recent
vector/hybrid + token-onboarding work are all complete.

Next suggested PRs, in priority order:

1. **Budget the remaining desktop bundle size.** Either lazy-load the CodeMirror
   markdown language support or raise `build.chunkSizeWarningLimit` intentionally
   and document the budget.
2. **Desktop auth/status parity.** Surface auth state and capture-sync status in
   the Wails desktop (it does not use the HTTP API, so this needs Wails service
   plumbing or a status panel).
3. **Release readiness scaffolding.** Define CLI binary artifacts, desktop
   installer strategy, extension packaging, and mobile distribution in CI and
   documentation.

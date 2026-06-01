# AgentVault Improvement Plan

Last updated: 2026-06-01

## Goal

Make AgentVault dependable as a local-first knowledge app across CLI, desktop, web, extension, mobile, and agent clients. The immediate focus should be correctness and contract stability, then UX completeness, then release readiness.

## Priority Backlog

| Priority | Work | Why it matters | Evidence |
| --- | --- | --- | --- |
| P0 | Fix API/client contract mismatches | Web, extension, and mobile can compile while still failing at runtime. | `/projects` API returns an object; clients expect `string[]`. |
| P0 | Replace HTTP `/ask` stub with shared RAG pipeline | The web and extension surfaces cannot deliver the advertised AI behavior through the API. | CLI and desktop have real RAG paths; API returns placeholder text. |
| P0 | Implement real HTTP `/git/status` | The API currently reports a fake clean `main` state. | `internal/git` exists and is used by CLI/MCP; API handler is hard-coded. |
| P0 | Add contract tests for API responses consumed by clients | Prevents future type drift. | Current TypeScript builds do not catch JSON-shape mismatches. |
| P1 | Reuse one RAG/search service across CLI, API, desktop, and MCP | Avoids behavior drift and repeated prompt/parse logic. | RAG logic exists in multiple places. |
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

- Decide and document stable JSON shapes for every local API endpoint.
- Make `GET /projects` return the shape clients use, or update all clients and types to consume `{ projects: string[] }`.
- Replace `POST /ask` stub with `internal/rag.Pipeline` using the configured AI provider.
- Replace hard-coded `GET /git/status` with `internal/git.Status`.
- Add API tests that assert the exact JSON shapes consumed by web, extension, and mobile.
- Add a small shared API contract document under `docs/` or generate an OpenAPI file from handlers/tests.

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

Scope the first implementation PR to contract stabilization:

1. Fix `GET /projects` contract and all clients/tests.
2. Wire `POST /ask` to the shared RAG pipeline.
3. Wire `GET /git/status` to `internal/git`.
4. Add API response-shape tests.
5. Update docs if any endpoint shape changes.

This is the smallest useful improvement because it makes existing surfaces more truthful without adding a new feature surface.

# AgentVault Local HTTP API Contract

Last updated: 2026-06-15

This is the single source of truth for the local HTTP API exposed by
`agentvault serve` (package `core/internal/api`). It documents every route, its
auth requirement, request shape, and the **exact JSON the server emits today**.
The same shapes are enforced by the `core/internal/contract` Go package
(imported by `core/internal/api` and by `apps/desktop-wails`) and by the
`@agentvault/contract` TypeScript package (imported by all four clients via
path mappings). The CI gate `make contract-check` fails if any client
re-introduces snake_case keys or hard-codes the API base URL.

The field names below are derived directly from the handlers
(`core/internal/api/handlers.go`), middleware (`middleware.go`), and the shape
assertions in `server_test.go`. All structs now carry explicit camelCase `json`
tags for consistent serialization across server and clients.

## Shared client types

The TypeScript clients (`apps/web-local`, `apps/browser-extension`,
`apps/mobile-expo`) and the Wails desktop frontend all import their request
and response types from `packages/contract/`, which is the canonical TS
contract. The package has no runtime dependencies and is consumed via
TypeScript path mappings (Vite, Metro `watchFolders`) and Metro config.

| Source file | What it holds |
| --- | --- |
| `packages/contract/src/types.ts` | The TypeScript types for every endpoint's request and response body. |
| `packages/contract/src/endpoints.ts` | A typed `routes` constant plus a `Endpoint<M, P>` helper. |
| `packages/contract/src/client.ts` | A zero-dependency `createClient` factory and a `defaultClient` that web apps use directly. |
| `core/internal/contract/contract.go` | The matching Go structs, imported by `core/internal/api` and `apps/desktop-wails/app.go`. |

To add a new endpoint, add the types to `packages/contract/src/types.ts`,
add a route entry to `packages/contract/src/endpoints.ts`, and add a method
to the `ApiClient` interface in `packages/contract/src/client.ts`. The
contract check (`make contract-check`) will fail if a client ships a
hard-coded base URL or a snake_case key, so future drift is caught at
CI time.

## Connecting clients

When `agentvault serve` starts it prints an auth token to the terminal. Local
clients store this token and send it on every write request via the
`X-AgentVault-Token` header or `Authorization: Bearer <token>`.

Clients can check a stored token without making a write operation by calling
`GET /auth/verify`. The response includes:

- `hasToken`: whether the client sent a token header.
- `tokenValid`: whether the sent token matches the server's current token.

A missing or incorrect token on a write endpoint returns `401` with
`{"error":"unauthorized","detail":"Valid X-AgentVault-Token header required"}`.

## Conventions

- **Base URL:** `http://127.0.0.1:47321` by default (`agentvault serve` binds
  loopback only).
- **Content type:** all responses are `application/json`. Request bodies for
  `POST` endpoints must be JSON.
- **Auth:** `GET` endpoints are open. Every non-`GET` (write) endpoint requires
  the auth token, sent as either:
  - `X-AgentVault-Token: <token>`, or
  - `Authorization: Bearer <token>`.

  The token is generated per server start and printed at startup; clients store
  it locally. A missing/incorrect token on a write endpoint returns `401` with
  `{"error":"unauthorized","detail":"Valid X-AgentVault-Token header required"}`.
- **CORS:** the server reflects the request `Origin` when it is `file://`,
  `chrome-extension://`, `moz-extension://`, or an `http(s)` origin whose host is
  exactly `localhost`, `127.0.0.1`, or `::1` (any port). Spoofed hosts such as
  `http://localhost.evil.com` are rejected (host-exact, not substring). Allowed
  methods: `GET, POST, OPTIONS`. Allowed headers:
  `Content-Type, Authorization, X-AgentVault-Token`. Preflight `OPTIONS` returns
  `200` with no body.
- **Error shape:** all handler errors use
  `{"error": "<summary>", "detail": "<specifics>"}` with a non-2xx status.

## Endpoint summary

| Method | Path | Auth | Success | Body casing |
| --- | --- | --- | --- | --- |
| GET | `/health` | no | 200 | camelCase |
| GET | `/auth/verify` | no | 200 | camelCase |
| GET | `/vault/status` | no | 200 | camelCase |
| POST | `/vault/index` | yes | 200 | camelCase (`IndexResult`) |
| GET | `/search` | no | 200 | camelCase (`[]search.Result`) |
| GET | `/notes/{id}` | no | 200 / 400 / 404 | camelCase |
| POST | `/notes` | yes | 200 | camelCase |
| POST | `/capture` | yes | 200 | camelCase |
| POST | `/ask` | yes | 200 / 400 / 502 | camelCase (`rag.Answer`) |
| GET | `/projects` | no | 200 | bare `string[]` |
| GET | `/recent` | no | 200 | camelCase (`[]search.Result`) |
| GET | `/stale` | no | 200 | camelCase (`[]search.Result`) |
| GET | `/git/status` | no | 200 | camelCase |

---

## GET /health

No auth. Liveness + identity probe.

```json
{ "status": "ok", "vault": "/abs/path/to/vault", "version": "0.1.0" }
```

## GET /auth/verify

No auth. Allows clients to check whether their stored token is still valid
without making a write operation. Returns the server's current token validity
status:

```json
{
  "status": "ok",
  "server": "agentvault",
  "version": "0.1.0",
  "hasToken": true,
  "tokenValid": true
}
```

- `hasToken`: whether the client sent a token header
- `tokenValid`: whether the sent token matches the server's current token

## GET /vault/status

No auth. `noteCount` and `version` are only populated when the path is a vault.

```json
{
  "path": "/abs/path/to/vault",
  "isVault": true,
  "noteCount": 42,
  "version": "2026-06-10T11:00:00Z"
}
```

## POST /vault/index

Auth required. Optional JSON body of index options; all fields optional:

```json
{ "force": false, "rebuild": false, "path": "", "embed": false }
```

Returns the `indexer.IndexResult` struct with camelCase `json` tags:

```json
{
  "scanned": 10, "added": 2, "updated": 1, "removed": 0, "skipped": 7,
  "errors": [ { "path": "10-notes/bad.md", "error": "..." } ],
  "chunksAdded": 14, "embedErrors": 0, "duration": 12345678
}
```

`duration` is a Go `time.Duration` (integer nanoseconds). `errors` is `null`
when empty.

## GET /search

No auth. Query params (all optional except that an empty `q` returns recent-style
results): `q`, `type`, `project`, `tag`, `status`, `limit` (default 20),
`offset`. Returns a JSON **array** of `search.Result` serialized with camelCase `json` tags:

Query parameters (all optional):
- `q`: search text
- `type`: filter by note type
- `project`: filter by project
- `tag`: filter by tag
- `status`: filter by status
- `limit`: max results (default 20)
- `offset`: pagination offset
- `vector`: enable vector/hybrid search (`true` or `1`)
- `hybrid_weight`: weight for vector vs FTS (0=FTS only, 1=vector only, default 0.5)
- `topk`: number of vector candidates to fetch (default limit*3)

The `@agentvault/contract` TypeScript client exposes these as camelCase
(`vector`, `hybridWeight`, `topk`) and translates `hybridWeight` to the
server's `hybrid_weight` query key before sending the request.

```json
[
  {
    "id": "note_2024_01_15_123",
    "title": "Test Note",
    "path": "10-notes/test-note.md",
    "type": "note",
    "project": "test-project",
    "status": "",
    "tags": ["go", "api"],
    "snippet": "…matched excerpt…",
    "score": -1.23,
    "updatedAt": "2024-01-15T12:00:00Z"
  }
]
```

## GET /notes/{id}

No auth. Looks up a note by ID and returns its metadata plus the full file
contents. `400` if the id segment is missing, `404` if no note matches. Built
from a hand-written map, so fields are camelCase:

```json
{
  "id": "note_2024_01_15_123",
  "title": "Test Note",
  "path": "10-notes/test-note.md",
  "type": "note",
  "project": "test-project",
  "status": "",
  "tags": ["go", "api"],
  "content": "---\nid: …\n---\n# Test Note\n…"
}
```

## POST /notes

Auth required. Creates a note from a template.

Request:

```json
{ "type": "note", "title": "My Note", "project": "optional", "tags": ["a","b"] }
```

`type` defaults to `note`; `title` is required (`400` if blank). Response:

```json
{ "path": "10-notes/note_….md", "id": "note_…" }
```

## POST /capture

Auth required. Appends a capture to `00-inbox`.

Request (all optional; `title` defaults to "Untitled Capture"):

```json
{ "type": "webpage", "title": "…", "url": "https://…", "text": "…",
  "project": "…", "tags": ["…"] }
```

Response (path always under `00-inbox/`):

```json
{ "path": "00-inbox/2026-06-10_capture_001.md" }
```

## POST /ask

Auth required. Source-grounded RAG answer over the vault. Returns `400` if
`question` is blank/whitespace, `502` if the configured AI provider fails.

Request:

```json
{ "question": "What did we decide about auth?" }
```

Response is `rag.Answer` (JSON-tagged, camelCase). `caveats`, `missingInfo`,
and `suggestedActions` are omitted when empty:

```json
{
  "answer": "…",
  "sources": [
    { "id": "note_…", "path": "30-decisions/…md", "title": "…", "excerpt": "…" }
  ],
  "confidence": "high",
  "caveats": ["…"],
  "missingInfo": "…",
  "suggestedActions": ["…"]
}
```

Each source carries both `id` and `path`; clients navigate to `/note/{id}`.

## GET /projects

No auth. Bare JSON array of distinct non-empty project names (sorted). Empty
result serializes to `[]`, never `null`:

```json
["personal", "test-project", "work"]
```

## GET /recent

No auth. `limit` query param (default 10). Same camelCase `[]search.Result`
shape as [`/search`](#get-search).

Query parameters (all optional):
- `limit`: max results (default 10)
- `vector`: enable vector/hybrid search (`true` or `1`)
- `hybrid_weight`: weight for vector vs FTS (0=FTS only, 1=vector only, default 0.5)
- `topk`: number of vector candidates to fetch (default limit*3)

## GET /stale

No auth. `days` query param (default 30) — notes not updated within that window.
Same camelCase `[]search.Result` shape as [`/search`](#get-search).

Query parameters (all optional):
- `days`: staleness window in days (default 30)
- `limit`: max results (default 20)
- `vector`: enable vector/hybrid search (`true` or `1`)
- `hybrid_weight`: weight for vector vs FTS (0=FTS only, 1=vector only, default 0.5)
- `topk`: number of vector candidates to fetch (default 60)

## GET /git/status

No auth. Reports real vault VCS state via `internal/git.Status`. A non-versioned
vault is a valid state and returns `isGitRepo: false` (not an error). Built from
a hand-written map, so fields are camelCase:

```json
{
  "isGitRepo": true,
  "branch": "main",
  "clean": false,
  "aheadBehind": "ahead 1",
  "modifiedFiles": [ { "path": "10-notes/x.md", "status": "modified", "staged": false } ],
  "untrackedFiles": ["00-inbox/new.md"]
}
```

When `isGitRepo` is `false`: `branch` is `""`, `clean` is `true`, and both file
arrays are empty (never `null`).

---

## Known contract drift

This document is now enforced by the shared `@agentvault/contract`
package and the `make contract-check` CI gate. The drift items below
are all resolved; the contract in the rest of this document is what
every client and the server now produce.

**Resolved:**
- `/search`, `/recent`, `/stale` serialize `search.Result` with camelCase
  `json` tags (`id`, `title`, `path`, `type`, `project`, `status`, `tags`,
  `snippet`, `score`, `updatedAt`). — Types now live in
  `packages/contract/src/types.ts` and `core/internal/contract/contract.go`.
- `/vault/index` serializes `indexer.IndexResult` with camelCase `json`
  tags (`scanned`, `added`, `updated`, `removed`, `skipped`, `errors`,
  `chunksAdded`, `embedErrors`, `duration`). The nested `IndexError` also uses
  camelCase (`path`, `error`). — Same shared source.
- `/vault/status` returns `version` instead of `indexedAt`. — Same.
- `/notes/{id}` returns the full note body under `content` and uses
  `contract.NoteDetail` for the response shape. — New `contract.NoteDetail`.
- `/git/status` returns the `contract.GitStatus` shape with
  `isGitRepo`/`branch`/`clean`/`aheadBehind`/`modifiedFiles`/`untrackedFiles`
  instead of a hand-written `map[string]interface{}`. — New
  `contract.GitStatus` and `contract.GitModifiedFile`.
- `/auth/verify` is wired and typed in the contract package even though
  no client (besides the optional `verifyAuth()` helper) calls it.
- Web, extension, mobile, and Wails desktop all import
  `SearchResult`/`Answer`/`Source` from `@agentvault/contract`, so the
  `decision.status || 'active'` line in the Wails DecisionDashboard now
  reads real status data (previously the Wails SearchResult lacked
  `status`).
- The Wails desktop `VaultStatus` now uses the shared
  `contract.VaultStatus` (`isVault`, not `isOpen`) and the Wails
  frontend checks `vaultStatus?.isVault`.

All endpoints are now aligned across server, tests, and clients:
`/health`, `/vault/status`, `/vault/index`, `/search`, `/notes/{id}`, `/notes`
(POST), `/capture`, `/ask`, `/projects`, `/recent`, `/stale`, and `/git/status`.

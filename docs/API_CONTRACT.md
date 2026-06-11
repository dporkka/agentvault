# AgentVault Local HTTP API Contract

Last updated: 2026-06-10

This is the single source of truth for the local HTTP API exposed by
`agentvault serve` (package `core/internal/api`). It documents every route, its
auth requirement, request shape, and the **exact JSON the server emits today**.
Where a shipped client expects a different shape, that mismatch is called out in
[Known contract drift](#known-contract-drift) rather than hidden — aligning the
clients to this contract is the next planned step (see
[IMPROVEMENT_PLAN.md](IMPROVEMENT_PLAN.md), "shared/generated API types").

The field names below are derived directly from the handlers
(`core/internal/api/handlers.go`), middleware (`middleware.go`), and the shape
assertions in `server_test.go`. When a handler serializes a Go struct that has
no `json` tags, Go emits the **exported (PascalCase)** field names verbatim;
this is noted per-endpoint because it is the main source of client drift.

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
| GET | `/vault/status` | no | 200 | camelCase |
| POST | `/vault/index` | yes | 200 | **PascalCase** (`IndexResult`) |
| GET | `/search` | no | 200 | **PascalCase** (`[]search.Result`) |
| GET | `/notes/{id}` | no | 200 / 400 / 404 | camelCase |
| POST | `/notes` | yes | 200 | camelCase |
| POST | `/capture` | yes | 200 | camelCase |
| POST | `/ask` | yes | 200 / 400 / 502 | camelCase (`rag.Answer`) |
| GET | `/projects` | no | 200 | bare `string[]` |
| GET | `/recent` | no | 200 | **PascalCase** (`[]search.Result`) |
| GET | `/stale` | no | 200 | **PascalCase** (`[]search.Result`) |
| GET | `/git/status` | no | 200 | camelCase |

---

## GET /health

No auth. Liveness + identity probe.

```json
{ "status": "ok", "vault": "/abs/path/to/vault", "version": "0.1.0" }
```

## GET /vault/status

No auth. `noteCount` and `indexedAt` are only populated when the path is a vault.

```json
{
  "path": "/abs/path/to/vault",
  "isVault": true,
  "noteCount": 42,
  "indexedAt": "2026-06-10T11:00:00Z"
}
```

> Drift: the web client's `VaultStatus` type declares `version` instead of
> `indexedAt`. See [Known contract drift](#known-contract-drift).

## POST /vault/index

Auth required. Optional JSON body of index options; all fields optional:

```json
{ "Force": false, "Rebuild": false, "Path": "", "Embed": false }
```

Returns the `indexer.IndexResult` struct **verbatim (PascalCase, no `json`
tags)**:

```json
{
  "Scanned": 10, "Added": 2, "Updated": 1, "Removed": 0, "Skipped": 7,
  "Errors": [ { "Path": "10-notes/bad.md", "Error": "..." } ],
  "ChunksAdded": 14, "EmbedErrors": 0, "Duration": 12345678
}
```

`Duration` is a Go `time.Duration` (integer nanoseconds). `Errors` is `null`
when empty.

## GET /search

No auth. Query params (all optional except that an empty `q` returns recent-style
results): `q`, `type`, `project`, `tag`, `status`, `limit` (default 20),
`offset`. Returns a JSON **array** of `search.Result` serialized **verbatim
(PascalCase, no `json` tags)**:

```json
[
  {
    "ID": "note_2024_01_15_123",
    "Title": "Test Note",
    "Path": "10-notes/test-note.md",
    "Type": "note",
    "Project": "test-project",
    "Status": "",
    "Tags": ["go", "api"],
    "Snippet": "…matched excerpt…",
    "Score": -1.23,
    "UpdatedAt": "2024-01-15T12:00:00Z"
  }
]
```

> Drift: every shipped client declares these fields in camelCase
> (`id`, `title`, `path`, `snippet`, `updatedAt`, …) and does **not** read
> `Status`/`Score`. See [Known contract drift](#known-contract-drift).

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

No auth. `limit` query param (default 10). Same **PascalCase** `[]search.Result`
shape as [`/search`](#get-search).

## GET /stale

No auth. `days` query param (default 30) — notes not updated within that window.
Same **PascalCase** `[]search.Result` shape as [`/search`](#get-search).

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

These are the places where the server's current output does **not** match what a
shipped client expects. They are documented here so the next PR (consolidating
client types onto one shared/generated source) has an exact target. None are
blockers for Phase 0, but the first two cause fields to read as `undefined` in
the web client at runtime.

1. **`/search`, `/recent`, `/stale` casing.** The server serializes
   `search.Result` with no `json` tags → PascalCase (`Title`, `Snippet`,
   `UpdatedAt`, `ID`, …). The web client's `SearchResult` type
   (`apps/web-local/src/api/types.ts`) declares camelCase
   (`title`, `snippet`, `updatedAt`, `id`, …), so those fields are `undefined`
   at runtime. **Recommended fix:** add camelCase `json` tags to `search.Result`
   and update the `server_test.go` shape assertions (currently `r["Title"]`,
   `r["ID"]`) accordingly. Note: the desktop Wails app binds the same struct, so
   its generated TS models must be re-checked when tags change.

2. **`/vault/status` field name.** Server emits `indexedAt`; the web
   `VaultStatus` type declares `version`. Pick one name in the shared contract.

3. **`/vault/index` casing.** Returns `IndexResult` PascalCase. No client reads
   it field-by-field today, but it should be camelCased for consistency when the
   shared contract lands.

The endpoints already aligned across server, tests, and clients are
`/health`, `/notes/{id}`, `/notes` (POST), `/capture`, `/ask`, `/projects`, and
`/git/status`.

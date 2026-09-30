# AgentVault Local HTTP API Contract

Last updated: 2026-09-30

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
  `Content-Type, Authorization, X-AgentVault-Token`.
  `Access-Control-Allow-Credentials` is set to `true`. Preflight `OPTIONS` returns
  `200` with no body.
- **Rate limiting:** a token-bucket limiter allows bursts of up to ~30 requests;
  exceeding the limit returns `429 Too Many Requests` with
  `{"error":"rate limit exceeded"}`.
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
| GET | `/notes/{id}` | no | 200 / 400 / 403 / 404 | camelCase |
| POST | `/notes` | yes | 200 | camelCase |
| POST | `/capture` | yes | 200 | camelCase |
| POST | `/ask` | yes | 200 / 400 / 500 / 502 | camelCase (`rag.Answer`) |
| GET | `/projects` | no | 200 | bare `string[]` |
| GET | `/recent` | no | 200 | camelCase (`[]search.Result`) |
| GET | `/stale` | no | 200 | camelCase (`[]search.Result`) |
| GET | `/git/status` | no | 200 | camelCase |
| GET | `/promotions` | no | 200 / 400 | camelCase (`Promotion[]`) |
| POST | `/promotions` | yes | 201 / 400 | camelCase (`Promotion`) |
| POST | `/promotions/{id}/review` | yes | 200 / 400 / 404 / 409 | camelCase (`Promotion`) |
| POST | `/promotions/{id}/commit` | yes | 200 / 400 / 403 / 404 / 409 | camelCase (`Promotion`) |
| GET | `/evaluation-datasets/{id}` | no | 200 / 404 | camelCase (`EvaluationDatasetDetail`) |
| POST | `/evaluation-datasets` | yes | 201 / 400 | camelCase (`EvaluationDataset`) |
| POST | `/evaluation-datasets/{id}/cases` | yes | 201 / 400 / 404 | camelCase (`EvaluationCase`) |
| GET | `/experiments/{id}` | no | 200 / 404 | camelCase (`ExperimentDetail`) |
| GET | `/experiments/{id}/compare/{candidateId}` | no | 200 / 400 / 404 / 409 | camelCase (`ExperimentComparison`) |
| POST | `/experiments` | yes | 201 / 400 / 404 | camelCase (`Experiment`) |
| POST | `/experiments/{id}/results` | yes | 201 / 400 / 404 / 409 | camelCase (`ExperimentResult`) |
| POST | `/agents/{id}/context` | yes | 200 / 400 / 404 | camelCase (`ContextSnapshot`) |
| GET | `/contexts/{hash}` | no | 200 / 400 / 404 | camelCase (`ContextSnapshot`) |
| POST | `/runs` | yes | 201 / 400 / 404 / 409 | camelCase (`RunRecord`) |
| GET | `/runs/{id}/audit` | no | 200 / 400 / 404 / 409 | camelCase (`RunAudit`) |
| GET | `/runs/{id}/learning-recommendation` | no | 200 / 400 / 404 / 409 | camelCase (`LearningRecommendation`) |
| GET | `/runs/{id}/regression-case-proposal` | no | 200 / 400 / 404 / 409 | camelCase (`RegressionCaseProposal`) |
| POST | `/runs/{id}/regression-cases` | yes | 201 / 400 / 404 / 409 | camelCase (`EvaluationCase`) |
| POST | `/runs/{id}/learning-candidates` | yes | 201 / 400 / 404 / 409 | camelCase (`Promotion`) |

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
- `vector`: enable vector/hybrid search (`true` or `1`). When enabled, the
  server still falls back to FTS if there are no embeddings or no query text.
- `hybrid_weight`: weight for vector vs FTS, only read when `vector` is enabled
  (0=FTS only, 1=vector only, default 0.5)
- `topk`: number of vector candidates to fetch when `vector` is enabled
  (default `max(limit * 3, 10)`)

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
contents. `400` if the id segment is missing, `404` if no note matches, `403`
if the resolved file path escapes the vault (path traversal). Uses
`contract.NoteDetail`, so fields are camelCase:

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

Clients may supply an `external_id` as an idempotency key. When provided, the
server scans existing inbox captures for a matching `external_id` in the
frontmatter and returns the existing path instead of creating a duplicate.
Idempotent responses include `"idempotent": true`.

Request (all optional; `title` defaults to "Untitled Capture"):

```json
{ "type": "webpage", "title": "…", "url": "https://…", "text": "…",
  "project": "…", "tags": ["…"], "external_id": "cap-abc123" }
```

Response (path always under `00-inbox/`):

```json
{ "path": "00-inbox/2026-06-10_capture_001.md" }
```

If more than 999 captures are created for the same day, the endpoint returns
`409 Conflict` with `{"error":"too many captures for today"}`.
## POST /ask

Auth required. Source-grounded RAG answer over the vault. Returns `400` if
`question` is blank/whitespace, `500` if the AI provider cannot be loaded, and
`502` if the provider call fails.

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

## GET /stale

No auth. `days` query param (default 30) — notes not updated within that window.
Same camelCase `[]search.Result` shape as [`/search`](#get-search).

Query parameters (all optional):
- `days`: staleness window in days (default 30)
- `limit`: max results (default 20)

## GET /git/status

No auth. Reports real vault VCS state via `internal/git.Status`. A non-versioned
vault is a valid state and returns `isGitRepo: false` (not an error). Uses
`contract.GitStatus`, so fields are camelCase:

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

## POST /promotions

Auth required. Creates an evidence-backed promotion proposal without mutating canonical memory or knowledge. At least one source run, observation, or evaluation ID is required.

## POST /promotions/{id}/review

Auth required. Explicitly approves or rejects a proposed promotion. Invalid state transitions return `409 Conflict`.

## POST /promotions/{id}/commit

Auth required. Commits an approved promotion only after the target note resolves inside the vault and its Markdown body contains the exact candidate text. AgentVault records the target note and lineage; it does not create hidden memory rows.

## POST /evaluation-datasets

Auth required. Creates a reusable evaluation dataset.

## POST /evaluation-datasets/{id}/cases

Auth required. Adds one reproducible evaluation case with a required JSON-object `input`, optional `expected` object, and optional tags.

## POST /experiments

Auth required. Records metadata for an experiment executed by an external runtime. Terminal states (`completed`, `failed`, `cancelled`) receive a completion timestamp.

## POST /experiments/{id}/results

Auth required. Records one case-level result. The case must belong to the same dataset as the experiment or the server returns `409 Conflict`.

## POST /agents/{id}/context

Auth required. Compiles deterministic context from the canonical Markdown agent manifest plus explicit runtime inputs, then stores the exact compiled snapshot as immutable execution evidence keyed by a SHA-256 hash.

The compiler includes sources in stable order:

1. canonical agent manifest body,
2. `identity_ref`,
3. `context_policy_ref`,
4. ordered `memory_refs`,
5. current task,
6. explicitly supplied retrieved knowledge notes,
7. recent conversation messages in chronological order,
8. explicitly supplied artifact notes.

Knowledge/artifact retrieval remains external: the caller supplies `retrievedNoteIds` and `artifactNoteIds`; the compiler does not hide search/ranking policy inside context assembly. Manifest scopes and capability refs are carried into the snapshot as provenance metadata.

Request:

```json
{
  "task": "Implement the next slice",
  "conversationId": "conv_...",
  "retrievedNoteIds": ["note_..."],
  "artifactNoteIds": ["note_..."],
  "maxConversationMessages": 20
}
```

Missing referenced notes are recorded in `unresolved` rather than silently omitted. A missing or non-agent `{id}` is an error. Identical deterministic inputs and source contents produce the same `sha256:...` hash.

Response:

```json
{
  "hash": "sha256:...",
  "agentId": "agt_...",
  "agentRevision": 3,
  "agentTitle": "Coding Agent",
  "task": "Implement the next slice",
  "conversationId": "conv_...",
  "knowledgeScopes": ["project:platform"],
  "artifactScopes": [],
  "conversationScopes": [],
  "capabilityRefs": ["github"],
  "contextPolicyRef": "note_policy",
  "sections": [
    {
      "kind": "identity",
      "sourceId": "note_identity",
      "sourcePath": "10-notes/identity.md",
      "title": "Identity",
      "content": "..."
    }
  ],
  "unresolved": [],
  "text": "## agent: Coding Agent\n..."
}
```

The persisted row is evidence, not a second canonical knowledge store. Markdown remains canonical; the snapshot preserves what a particular execution context actually contained even if source files later change.

## GET /contexts/{hash}

No auth. Returns the exact immutable `ContextSnapshot` persisted by context compilation. Returns `404` when the hash is unknown. This endpoint is intended for run audit/replay and for explaining behavioral differences between agent revisions or context selections.

## POST /runs

Auth required. Records execution evidence for a runtime invocation without executing the agent itself.

A legacy run may omit `agentId`, `agentRevision`, and `contextHash`. When `contextHash` is supplied, the server requires `agentId` and `agentRevision`, resolves the immutable context snapshot, and rejects the run with `409 Conflict` unless the snapshot belongs to the same agent revision. An unknown context hash returns `404`.

Request:

```json
{
  "agentName": "coding-agent",
  "agentId": "agt_...",
  "agentRevision": 3,
  "task": "Implement the audit view",
  "status": "succeeded",
  "conversationId": "conv_...",
  "contextHash": "sha256:...",
  "input": {"issue": 81},
  "output": {"result": "ok"},
  "capabilitySnapshot": {"github.read": true},
  "runtimeMetadata": {"model": "example"},
  "filesChanged": ["core/internal/agentstate/run.go"]
}
```

The response is a `RunRecord` containing the generated run ID and normalized evidence fields.

## GET /runs/{id}/audit

No auth. Returns one `RunAudit` aggregate containing:

- the persisted run,
- the exact immutable `ContextSnapshot` referenced by `contextHash` (or `null` for an unbound legacy run),
- all run observations in creation order,
- all evaluations in creation order.

The embedded context retains each section's `sourceId` and `sourcePath`, so callers can traverse:

```text
run
  -> immutable context hash
      -> agent revision
      -> identity / memory / knowledge / conversation / artifact provenance
  -> observations
  -> evaluations
```

The audit read also verifies that persisted run identity still matches the referenced context snapshot; inconsistent stored evidence returns `409 Conflict`.

## GET /runs/{id}/learning-recommendation

No auth. Returns a deterministic, read-only `LearningRecommendation` derived from the persisted run audit.

AgentVault does not synthesize candidate memory text and does not create a promotion. It only surfaces explicit evidence that a caller can review:

- observations whose status is explicitly `failed` or `error`,
- evaluations whose categorical label explicitly indicates failure/regression,
- the originating agent/revision,
- existing memory refs that were present in the immutable context snapshot,
- optional `target_kind` / `targetKind` evaluator metadata,
- optional `supersedes_note_id` / `supersedesNoteId` evaluator metadata.

Numeric scores are deliberately not interpreted by themselves because evaluators may use different scales and polarity. A low score with no categorical failure label therefore does not become negative evidence.

Support levels are deterministic evidence summaries:
- `none`: no explicit negative evidence,
- `weak`: failed observation(s) only,
- `moderate`: negative evaluation(s) without a failed observation,
- `strong`: a negative evaluation plus failed observation, or multiple negative evaluations.

`eligible` is true only when the run has a canonical agent/revision binding and there is explicit negative evidence. Legacy unbound runs still return visible signals for diagnosis but are marked ineligible.

Example response:

```json
{
  "runId": "run_...",
  "agentId": "agt_...",
  "agentRevision": 4,
  "eligible": true,
  "supportLevel": "strong",
  "evidenceCount": 2,
  "suggestedTargetKind": "memory",
  "reasonCodes": ["run_failed", "failed_observation", "negative_evaluation"],
  "sourceObservationIds": ["obs_..."],
  "sourceEvaluationIds": ["eval_..."],
  "contextMemoryRefs": ["memory_..."],
  "supersedesNoteIds": ["memory_..."],
  "signals": [
    {
      "kind": "evaluation",
      "id": "eval_...",
      "observationId": "obs_...",
      "name": "regression",
      "status": "",
      "label": "fail",
      "score": 0.1,
      "rationale": "Focused regression test was skipped."
    }
  ]
}
```

## GET /experiments/{id}/compare/{candidateId}

No auth. Compares two persisted experiments without running evaluations.

The baseline and candidate experiments must reference the same evaluation dataset and the same agent ID. Different datasets or agents return `409 Conflict`.

Case transitions are derived only from explicit categorical labels:

- failure → pass = `fixed`
- pass → failure = `regressed`
- pass → pass = `stable_pass`
- failure → failure = `stable_fail`
- unknown/unrecognized labels = `unclassified`
- absent results = `missing_baseline` / `missing_candidate`

Recognized pass labels include `pass`, `passed`, `success`, `succeeded`, `correct`, and `accepted`. Failure labels follow the same explicit failure vocabulary used by learning recommendations.

If both results contain numeric scores, `scoreDelta` is returned as `candidate - baseline`. AgentVault does **not** interpret the sign as improvement or regression because evaluator score polarity and thresholds are not globally defined.

The response includes the two immutable agent revisions and per-case transitions, but deliberately provides no synthesized overall winner. External runtimes remain responsible for executing the cases and writing results through the experiment-result API.

## GET /runs/{id}/regression-case-proposal

No auth. Returns a deterministic, read-only `RegressionCaseProposal` for an audited run.

The proposal:
- copies the original persisted run input,
- derives agent ID/revision from the run,
- reuses the learning recommendation's negative-evidence lineage,
- generates a deterministic default name from the run task,
- adds the `regression` tag,
- reads expected behavior only from explicit negative-evaluation metadata keys `expected`, `expected_output`, or `expectedOutput`.

AgentVault does not infer expected behavior from the failed output. If multiple negative evaluations provide conflicting expected objects, `expected` is omitted and `conflicting_expected_hints` is added to `reasonCodes`.

This endpoint does not write an evaluation case.

## POST /runs/{id}/regression-cases

Auth required. Explicitly captures an eligible regression proposal into the caller-selected evaluation dataset.

Request:

```json
{
  "datasetId": "ds_...",
  "name": "optional override",
  "expected": {"status": "pass"},
  "tags": ["checkout"]
}
```

The stored `EvaluationCase` preserves:
- `sourceRunId`,
- `sourceObservationIds`,
- `sourceEvaluationIds`,
- `agentId`,
- `agentRevision`.

If the dataset is scoped to a different agent than the source run, the server returns `409 Conflict`.

Capture is idempotent for a dataset + source run pair: retries return the existing case instead of creating another regression entry. The database also enforces uniqueness for that pair.

AgentVault stores the regression case but does not execute it; experiments remain externally executed and recorded back through the existing experiment/result APIs.

## POST /runs/{id}/learning-candidates

Auth required. Creates a reviewable memory or knowledge promotion proposal from one recorded run.

The request intentionally has no `agentId`. AgentVault derives the agent identity from the persisted run so callers cannot attach learning from one run to another agent.

Optional `sourceObservationIds` and `sourceEvaluationIds` are validated strictly:
- each referenced observation/evaluation must exist,
- each must belong to the same originating run,
- cross-run evidence returns `409 Conflict`,
- missing evidence returns `404`.

Legacy runs without a canonical `agentId` / revision binding cannot produce run-derived learning and return `409 Conflict`.

Request:

```json
{
  "targetKind": "memory",
  "candidate": "Run the focused regression test before broad verification.",
  "rationale": "The failed evaluation identified a missing verification step.",
  "sourceObservationIds": ["obs_..."],
  "sourceEvaluationIds": ["eval_..."],
  "supersedesNoteId": "note_..."
}
```

The response is a normal `Promotion` with:
- `status: "proposed"`,
- `agentId` derived from the run,
- `sourceRunIds` containing exactly the originating run,
- validated observation/evaluation lineage.

This endpoint does not mutate canonical memory or knowledge. It only enters the existing explicit promotion workflow:

```text
run
 -> observation/evaluation evidence
 -> proposed promotion
 -> approve/reject
 -> commit to canonical Markdown
 -> future context compilation
```

## GET /promotions

No auth. Lists evidence-backed promotion records. With no query parameters, returns only `proposed` records (the pending review queue), newest first.

Optional query parameters:
- `status`: `proposed`, `approved`, `rejected`, `committed`, `superseded`, or `all`.
- `agent_id`: filter to one agent manifest ID.
- `limit`: maximum rows, default 50 and capped at 100.

Each record includes its source run/observation/evaluation IDs, review metadata, and canonical target note ID when committed.

## GET /evaluation-datasets/{id}

No auth. Returns one evaluation dataset and all of its reproducible cases ordered by creation time. Case `input` and `expected` values are decoded JSON objects; `expected` is `null` when the case has no expected output.

Returns `404` when the dataset does not exist.

## GET /experiments/{id}

No auth. Returns one externally executed experiment together with all recorded case-level results. The response includes the immutable agent revision and experiment configuration used for comparison, plus optional run IDs, score/label values, and per-result metadata.

Returns `404` when the experiment does not exist.

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

All endpoints are now aligned across server, tests, and clients, including durable agent-state reads/writes, deterministic context compilation, context-bound run recording, run audit provenance, and reviewable run-derived learning proposals.

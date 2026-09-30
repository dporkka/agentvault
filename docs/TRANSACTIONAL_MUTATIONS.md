# Transactional Agent Mutations

AgentVault agents should be able to suggest changes without receiving an unrestricted file-write primitive. Transactional mutations provide a reviewable, durable workflow around a single UTF-8 text file:

`propose -> approve -> commit -> undo`

The first implementation deliberately supports one file per proposal. This keeps the guarantees precise and leaves multi-file transactions for a later recovery protocol rather than implying filesystem atomicity that does not exist.

## Durable model

A proposal captures:

- mutation kind: `create`, `replace`, or `delete`
- vault-relative target path
- human-readable reason
- optional agent, session, and provenance IDs
- exact before/after existence flags
- SHA-256 before/after hashes
- UTF-8 before/after snapshots
- a review diff
- lifecycle state and audit timestamps

Lifecycle events are appended to `80-agent-runs/knowledge.journal.jsonl` and fsynced before the corresponding SQLite projection is updated. `mutation_proposals` is therefore a rebuildable projection, not the authority.

Snapshots are currently limited to 1 MiB per side and diffs to 256 KiB. Binary files are outside this initial protocol. Because rollback snapshots contain the complete before/after text, the journal can contain sensitive content from mutated files and must be protected with the same care as the vault itself.

## Lifecycle

New proposals start as `proposed`. Proposal creation never mutates a target file and never implies approval.

Approval is a separate transition to `approved`. Under a scoped capability credential, the approving identity is bound to the persisted capability principal ID rather than accepted from request input. Root-token callers may still provide an audit label explicitly.

A commit transitions to `committing` in the durable journal before filesystem mutation. AgentVault checks the captured before state before recording commit intent and checks it again after the intent event has been fsynced. If the target no longer matches, commit fails rather than overwriting the current file. Successful application transitions to `committed`.

Undo performs the inverse check. It is allowed only from `committed`, records `undoing` before filesystem mutation, re-checks the committed after state after the durable intent event, and restores the exact before snapshot only while the target still matches. A later edit is never deliberately overwritten by undo.

A proposal can also become `rejected` or `conflicted`.

## Crash recovery

At startup, proposals left in `committing` or `undoing` are reconciled against the authoritative file:

| Durable state | File state | Recovery action |
| --- | --- | --- |
| `committing` | after hash | finalize `committed` |
| `committing` | before hash | return to `approved` |
| `committing` | neither | mark `conflicted`; do not write |
| `undoing` | before hash | finalize `undone` |
| `undoing` | after hash | return to `committed` |
| `undoing` | neither | mark `conflicted`; do not write |

This handles process failure between recording intent, filesystem mutation, and recording completion without guessing which content should win. If mutation recovery cannot reconcile an interrupted proposal, mutation routes fail closed while the independently healthy knowledge/memory/context APIs remain available.

## Path safety

The engine accepts vault-relative paths only. It rejects traversal outside the vault, absolute/volume paths, `.agentvault`, `.git`, the canonical knowledge journal, symlink path components, non-regular existing targets, non-UTF-8 content, and oversized snapshots. Protected internal path checks are case-insensitive so case variants cannot bypass the invariant on case-insensitive filesystems.

Creates use an atomic installation step that refuses to overwrite a destination created concurrently. Replacements use a same-directory temporary file, preserve file permissions, fsync the temporary content, and rename it into place.

## Concurrency boundary

All AgentVault mutation engines in the same process share a per-target lock keyed by normalized canonical path. This serializes competing AgentVault proposals/commits/undo operations for the same file. Commit and undo also re-check the expected hash after their durable intent event has been fsynced and immediately before applying the filesystem operation.

The SHA-256 check is still optimistic concurrency control, not a portable operating-system compare-and-swap primitive. An unrelated process can theoretically write in the interval between the final observation and a replacement/delete syscall, and a second AgentVault process does not share the in-memory lock.

Therefore this version guarantees that known stale proposals are rejected, same-process AgentVault writers are serialized per path, and crash recovery does not overwrite an ambiguous third state. It does **not** claim atomic exclusion against arbitrary concurrent external or multi-process writers. A future inter-process writer coordinator or platform-specific file-lock/CAS layer should be added before advertising that stronger property.

Within normal AgentVault usage, callers should route authoritative agent file writes through this mutation protocol rather than mixing direct writes and transactional commits to the same path.

## Capability identities

The local HTTP API has two credential classes:

1. The per-process **root token** remains the credential for generic AgentVault data access/writes and capability administration.
2. Persistent **capability tokens** are deliberately narrow HTTP credentials accepted only by transactional mutation routes.

The same persistent identities can bind an MCP process, where explicit read, context, durable machine-write, model, and mutation capabilities control tool registration. See `MCP_CAPABILITIES.md`.

Supported mutation capabilities are:

- `mutation:read`
- `mutation:propose`
- `mutation:approve`
- `mutation:commit`
- `mutation:undo`
- `mutation:reject`

A capability identity contains a stable principal ID, an agent ID, a set of capabilities, optional scope restrictions, creation/expiry/revocation timestamps, and a token hash. Tokens use cryptographically random opaque `avc_...` values. The raw token is returned only when minted; AgentVault persists only its SHA-256 hash in `.agentvault/capabilities.json`. The registry file is written with owner-only permissions.

Capability scopes may restrict vault-relative path prefixes, projects, and durable session IDs. Restrictions are intersected. Path-prefix matching is segment-aware.

Project scope is never trusted from mutation request metadata. When a mutation references a durable session, AgentVault resolves the project from the persisted session. A scoped proposer cannot spoof `agentId`; AgentVault binds proposal identity to the capability principal. Scoped approve/reject operations bind their audit actor to the stable capability principal ID.

## HTTP security boundary

All data-bearing non-mutation API routes retain root-token authentication. Non-mutation MCP capabilities do not become generic HTTP credentials.

Mutation proposals contain complete before/after rollback snapshots, so mutation reads are separately authenticated. Each lifecycle route requires its exact mutation capability:

| Route | Capability |
| --- | --- |
| `GET /mutations` | `mutation:read` |
| `GET /mutations/{id}` | `mutation:read` |
| `POST /mutations` | `mutation:propose` |
| `POST /mutations/{id}/approve` | `mutation:approve` |
| `POST /mutations/{id}/commit` | `mutation:commit` |
| `POST /mutations/{id}/undo` | `mutation:undo` |
| `POST /mutations/{id}/reject` | `mutation:reject` |

Capability administration remains root-only:

- `GET /auth/capabilities`
- `POST /auth/capabilities`
- `POST /auth/capabilities/{id}/revoke`

The raw token returned at issuance should be stored as a secret because it cannot be recovered from the registry later. Rotation is mint-new then revoke-old.

## MCP security boundary

Unbound **stdio** MCP preserves the conservative local compatibility behavior: mutation tools are proposal/read-only, legacy direct user-file writers remain disabled by default, and the established safe local read/AI/knowledge/context surface remains available.

A capability-bound MCP process registers only explicitly granted families:

- `vault:read` — indexed/file-backed vault reads and read-only resources
- `knowledge:read` — structured object/memory/session reads
- `knowledge:write` — provenance, object, and relation writes
- `memory:write` — durable machine-memory writes
- `session:write` — durable session lifecycle writes
- `context:compile` — Context Compiler
- `ai:invoke` — `agentvault.ask`
- individual `mutation:*` lifecycle capabilities

Durable machine-authored writes append to the canonical knowledge journal; they do **not** grant direct writes to user-authored files. Scoped `knowledge:write` and `memory:write` require session-bound provenance. Session-scoped graph writes are further ownership-limited by existing same-session provenance, and session-scoped memory writers may write only session memories rather than promoting state into project/global scope. `session:write` binds new session identity to the capability principal; a session-scoped identity cannot mint a different durable session.

Scope semantics are capability-specific:

- `mutation:*` supports path-prefix, project, and session restrictions.
- `knowledge:read`, `knowledge:write`, `memory:write`, `session:write`, and `context:compile` support project/session restrictions but not path-prefix restrictions.
- `vault:read` and `ai:invoke` remain global-only.

Unsupported combinations fail closed. Scoped creates use server-generated IDs where caller-selected IDs could become existence probes. Scoped canonical-path introduction/change is denied until structured path policy exists.

A capability identity without `mutation:*` authority does not initialize file-mutation tools or crash recovery as a startup side effect.

HTTP MCP requires the capability token on every request through `Authorization: Bearer ...` or `X-AgentVault-Token`. The token is revalidated against the registry before every MCP dispatch, so expiry or revocation takes effect without restarting the process.

`--allow-direct-writes` is mutually exclusive with a capability token. The complete capability-to-tool mapping and scope limitations are documented in `MCP_CAPABILITIES.md`.

## Registry durability boundary

The API keeps one capability registry instance per process, so issuance and revocation through that server are serialized. The registry is durably replaced through a temporary file plus fsync and rename.

This does not yet provide an inter-process compare-and-swap protocol for two independent AgentVault processes concurrently editing the same capability registry. Administrators should use one authoritative local API process for capability issuance/revocation. Multi-process registry coordination belongs with future writer-coordination work.

## TypeScript control plane

`createKnowledgeClient()` exposes mutation lifecycle plus root-only capability administration methods:

- `listCapabilities()`
- `mintCapability()`
- `revokeCapability()`

Capability types, including durable MCP write capabilities, are exported from `@agentvault/contract/capabilities`.

## Next durability and security steps

Before extending file mutations to multi-file proposals, add an explicit transaction manifest with ordered operations, durable per-operation progress, rollback material for every target, deterministic recovery, and inter-process writer coordination.

The remaining least-authority MCP gaps are scoped `vault:read`, scoped `ai:invoke`, and path-scoped structured knowledge. `vault:read` needs consistent authorization across search, graph/resources, recent/project listings, note reads, backlinks, and Markdown recall. `ai:invoke` should inherit scoped Context Compiler/RAG policy so prompts cannot receive out-of-scope context. Path-scoped structured access should wait until records without canonical paths have an explicit policy.

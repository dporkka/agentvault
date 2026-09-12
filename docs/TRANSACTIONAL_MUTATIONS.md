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

The engine accepts vault-relative paths only. It rejects traversal outside the vault, `.agentvault`, `.git`, the canonical knowledge journal, symlink path components, non-regular existing targets, non-UTF-8 content, and oversized snapshots.

Creates use an atomic installation step that refuses to overwrite a destination created concurrently. Replacements use a same-directory temporary file, preserve file permissions, fsync the temporary content, and rename it into place.

## Concurrency boundary

All AgentVault mutation engines in the same process share a per-target lock keyed by normalized canonical path. This serializes competing AgentVault proposals/commits/undo operations for the same file. Commit and undo also re-check the expected hash after their durable intent event has been fsynced and immediately before applying the filesystem operation.

The SHA-256 check is still optimistic concurrency control, not a portable operating-system compare-and-swap primitive. An unrelated process can theoretically write in the interval between the final observation and a replacement/delete syscall, and a second AgentVault process does not share the in-memory lock.

Therefore this version guarantees that known stale proposals are rejected, same-process AgentVault writers are serialized per path, and crash recovery does not overwrite an ambiguous third state. It does **not** claim atomic exclusion against arbitrary concurrent external or multi-process writers. A future inter-process writer coordinator or platform-specific file-lock/CAS layer should be added before advertising that stronger property.

Within normal AgentVault usage, callers should route authoritative agent writes through this mutation protocol rather than mixing direct writes and transactional commits to the same path.

## Capability identities

The local API has two credential classes:

1. The per-process **root token** remains the credential for generic AgentVault data access/writes and capability administration.
2. Persistent **capability tokens** are deliberately narrow credentials accepted only by transactional mutation routes.

Supported mutation capabilities are:

- `mutation:read`
- `mutation:propose`
- `mutation:approve`
- `mutation:commit`
- `mutation:undo`
- `mutation:reject`

A capability identity contains a stable principal ID, an agent ID, a set of capabilities, optional scope restrictions, creation/expiry/revocation timestamps, and a token hash. Tokens use cryptographically random opaque `avc_...` values. The raw token is returned only when minted; AgentVault persists only its SHA-256 hash in `.agentvault/capabilities.json`. The registry file is written with owner-only permissions.

Capability scopes may restrict:

- vault-relative path prefixes
- projects
- durable session IDs

Restrictions are **intersected**. For example, a token scoped to path `30-projects/acme`, project `acme`, and session `session_123` must satisfy all three constraints. An empty scope dimension means unrestricted for that dimension, but only within the explicitly granted lifecycle capability.

Path-prefix matching is segment-aware: a prefix `foo/bar` permits `foo/bar/note.md` but not `foo/barista/note.md`.

Project scope is never trusted from mutation request metadata. When a mutation references a durable session, AgentVault resolves the project from the persisted `AgentSession.Project`. Therefore a project-restricted capability must operate through a durable session whose recorded project is allowed.

A scoped proposer cannot spoof `agentId`; AgentVault binds proposal `agentId` to the capability principal's agent ID. Scoped approve/reject operations likewise bind the audit actor to the stable capability principal ID.

## HTTP security boundary

All data-bearing non-mutation API routes retain the unified core's root-token authentication, including reads. Capability tokens are not generic vault credentials.

Mutation proposals contain complete before/after rollback snapshots, so mutation reads are separately authenticated. `GET /mutations` and `GET /mutations/{id}` require the root token or `mutation:read`.

Each lifecycle route requires its exact capability:

| Route | Capability |
| --- | --- |
| `GET /mutations` | `mutation:read` |
| `GET /mutations/{id}` | `mutation:read` |
| `POST /mutations` | `mutation:propose` |
| `POST /mutations/{id}/approve` | `mutation:approve` |
| `POST /mutations/{id}/commit` | `mutation:commit` |
| `POST /mutations/{id}/undo` | `mutation:undo` |
| `POST /mutations/{id}/reject` | `mutation:reject` |

A mutation capability token cannot read or write generic notes, memories, sessions, objects, graph/search data, or capability identities through the HTTP API. Those surfaces continue to require the root server token.

Capability administration is root-only:

- `GET /auth/capabilities`
- `POST /auth/capabilities`
- `POST /auth/capabilities/{id}/revoke`

The raw token returned by `POST /auth/capabilities` should be stored as a secret because it cannot be recovered from the registry later. Rotation is mint-new then revoke-old.

## MCP security boundary

Unscoped **stdio** MCP preserves the conservative compatibility behavior from the transactional-mutation layer: mutation tools are proposal/read-only, and legacy direct user-file writers remain disabled by default. Read-only Markdown memory recall is retained.

A scoped MCP process can be bound with `AGENTVAULT_CAPABILITY_TOKEN` or `--capability-token`. Its mutation tool registry is reduced to the capabilities actually granted to that principal. For example, a proposer receives proposal/read tools but not commit; a reviewer with `mutation:approve` and `mutation:commit` receives those lifecycle tools subject to the same path/project/session policy checks.

HTTP MCP requires a capability token and requires that same token on each HTTP request through `Authorization: Bearer ...` or `X-AgentVault-Token`. The token is revalidated against the registry before every MCP dispatch, so expiry or revocation takes effect without restarting the process.

`--allow-direct-writes` is mutually exclusive with a capability token. This prevents a scoped identity from accidentally regaining legacy unreviewed file-writing tools.

Capability enforcement in this slice is intentionally **mutation-specific**. Existing safe non-mutation MCP tools—search/read/AI, structured knowledge/session operations, and Context Compiler—retain their current trust model. A bound mutation capability does not yet mean every MCP tool is least-privilege. Extending the capability namespace across those tool families is the next security layer and must happen before describing AgentVault MCP as fully least-privilege.

## Registry durability boundary

The API keeps one capability registry instance per process, so issuance and revocation through that server are serialized. The registry is durably replaced through a temporary file plus fsync and rename.

This does not yet provide an inter-process compare-and-swap protocol for two independent AgentVault processes concurrently editing the same capability registry. Administrators should use one authoritative local API process for capability issuance/revocation. Multi-process registry coordination belongs with the future writer-coordination work.

## TypeScript control plane

`createKnowledgeClient()` exposes the mutation lifecycle plus root-only capability administration methods:

- `listCapabilities()`
- `mintCapability()`
- `revokeCapability()`

Types are exported from `@agentvault/contract/capabilities`.

## Next durability and security steps

Before extending this protocol to multi-file proposals, add an explicit transaction manifest with ordered operations, durable per-operation progress, rollback material for every target, deterministic recovery, and an inter-process writer-coordination strategy.

Before broadening autonomous MCP access, extend capability enforcement to non-mutation tools and separate read/search, knowledge-write, session, AI/provider, and filesystem privileges. A multi-file UI or a claim of fully least-privilege MCP should not ship ahead of those semantics.

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

Approval is a separate transition to `approved`. `approvedBy` is an audit label supplied by the authenticated control-plane caller; with the current shared local HTTP token it is **not** a cryptographic user identity assertion.

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

This handles process failure between recording intent, filesystem mutation, and recording completion without guessing which content should win.

## Path safety

The engine accepts vault-relative paths only. It rejects traversal outside the vault, `.agentvault`, `.git`, the canonical knowledge journal, symlink path components, non-regular existing targets, non-UTF-8 content, and oversized snapshots.

Creates use an atomic installation step that refuses to overwrite a destination created concurrently. Replacements use a same-directory temporary file, preserve file permissions, fsync the temporary content, and rename it into place.

## Concurrency boundary

All AgentVault mutation engines in the same process share a per-target lock keyed by normalized canonical path. This serializes competing AgentVault proposals/commits/undo operations for the same file. Commit and undo also re-check the expected hash after their durable intent event has been fsynced and immediately before applying the filesystem operation.

The SHA-256 check is still optimistic concurrency control, not a portable operating-system compare-and-swap primitive. An unrelated process can theoretically write in the interval between the final observation and a replacement/delete syscall, and a second AgentVault process does not share the in-memory lock.

Therefore this version guarantees that known stale proposals are rejected, same-process AgentVault writers are serialized per path, and crash recovery does not overwrite an ambiguous third state. It does **not** claim atomic exclusion against arbitrary concurrent external or multi-process writers. A future inter-process writer coordinator or platform-specific file-lock/CAS layer should be added before advertising that stronger property.

Within normal AgentVault usage, callers should route authoritative agent writes through this mutation protocol rather than mixing direct writes and transactional commits to the same path.

## Security boundary

The HTTP/TypeScript API is currently a trusted local control plane. Possession of the server token authorizes mutation lifecycle calls, so applications must protect that token.

The default MCP registry keeps legacy direct user-file writers disabled. It still exposes read/search/AI tools, structured knowledge and session tools, the Context Compiler, and the mutation proposal/read capability. The mutation-specific MCP tools are:

- `agentvault.propose_mutation`
- `agentvault.get_mutation`
- `agentvault.list_mutations`

Mutation MCP does **not** expose approve, commit, reject, or undo. Giving an unscoped agent both proposal and approval tools under the same credential would turn the approval state machine into ceremony rather than a security boundary.

Legacy direct user-file MCP writers can be re-enabled explicitly with `agentvault mcp serve --allow-direct-writes` for compatibility with trusted clients. That flag bypasses the reviewed mutation path for those legacy tools and should not be enabled for untrusted/autonomous agents.

Once AgentVault supports capability-scoped MCP identities, separate capabilities should be introduced for `mutation:propose`, `mutation:approve`, `mutation:commit`, and `mutation:undo`, with policy able to constrain vault paths, projects, agents, and sessions.

## HTTP control-plane endpoints

- `GET /mutations`
- `POST /mutations`
- `GET /mutations/{id}`
- `POST /mutations/{id}/approve`
- `POST /mutations/{id}/commit`
- `POST /mutations/{id}/undo`
- `POST /mutations/{id}/reject`

The shared TypeScript `createKnowledgeClient()` exposes equivalent methods for trusted integrations.

## Next durability step

Before extending this protocol to multi-file proposals, add an explicit transaction manifest with ordered operations, durable per-operation progress, rollback material for every target, deterministic recovery, and a writer-coordination strategy. A multi-file UI should not ship ahead of those semantics.

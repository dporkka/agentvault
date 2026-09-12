# MCP Capability Families

AgentVault MCP supports persistent capability identities without treating one agent token as a generic vault credential. Tool registration is derived from the principal's grants before the server begins serving requests, and a bound token is revalidated on every MCP dispatch so expiry or revocation takes effect without restart.

## Credential modes

AgentVault has three MCP operating modes:

1. **Unbound local stdio** — compatibility mode for a trusted local process. It exposes the existing safe local read/AI surface, structured knowledge/session tools, Context Compiler, proposal/read mutations, and read-only MCP resources.
2. **Capability-bound MCP** — least-authority mode. Only explicitly granted capability families are registered.
3. **Legacy direct-write compatibility** — `--allow-direct-writes` restores old direct user-file writers for a trusted client. It cannot be combined with a capability identity.

HTTP MCP always requires a capability token. The same token authenticates the HTTP transport and determines the registered tool surface.

## Read-side capabilities

The first non-mutation capabilities are deliberately read-side only.

### `vault:read`

Registers the file/index-backed vault read surface:

- `agentvault.search`
- `agentvault.recall_memories`
- `agentvault.read_note`
- `agentvault.get_links`
- `agentvault.list_projects`
- `agentvault.list_recent`
- `agentvault.git_status`
- read-only MCP resources such as graph/projects/recent/tags

It does **not** grant `agentvault.ask`, Context Compiler, structured knowledge writes, session writes, or file writes.

### `knowledge:read`

Registers only the read side of journal-backed structured knowledge:

- `agentvault.get_object`
- `agentvault.list_memories`
- `agentvault.get_session`

It does **not** grant:

- provenance creation
- object upsert
- relation creation
- durable memory recording
- session start/event append/close

Those write capabilities are intentionally deferred until AgentVault has resource-aware authorization semantics for each operation.

### `context:compile`

Registers only:

- `agentvault.compile_context`

Context compilation can draw from multiple memory and knowledge sources, so this is a distinct authority rather than an implication of ordinary vault reads.

### `ai:invoke`

Registers only:

- `agentvault.ask`

AI/provider invocation is separate from data-read authority because it may consume model/provider credentials, trigger external inference, and internally retrieve vault context.

## Mutation capabilities

Transactional file mutations retain their independent lifecycle capabilities:

- `mutation:read`
- `mutation:propose`
- `mutation:approve`
- `mutation:commit`
- `mutation:undo`
- `mutation:reject`

Mutation capabilities can be path/project/session scoped because their handlers resolve and authorize the concrete proposal/session resource before acting.

Read-side and mutation capabilities may be combined on one **unscoped** principal. For example, a local autonomous coding agent could receive `vault:read`, `context:compile`, `mutation:read`, and `mutation:propose` without receiving approval or commit authority.

## Scope boundary

The first read-side families are **global-only**. A capability principal that combines any of these:

- `vault:read`
- `knowledge:read`
- `context:compile`
- `ai:invoke`

with any path-prefix, project, or durable-session restriction is rejected during MCP runtime registration.

This is intentional fail-closed behavior. AgentVault does not silently ignore a requested scope, and it does not assume that caller-supplied arguments are sufficient authorization. Precise scoped read capabilities require persisted-resource filtering inside each tool family.

Mutation-only principals may continue to use path/project/session restrictions because those checks already exist.

## HTTP API boundary

These new read-side capabilities currently govern **MCP only**. They do not become generic HTTP API credentials.

The ordinary AgentVault HTTP API continues to require the per-process root token for non-mutation data endpoints. Mutation HTTP routes retain the capability-specific model documented in `TRANSACTIONAL_MUTATIONS.md`.

Keeping the boundaries separate prevents a token minted for a narrow MCP process from unexpectedly gaining browser/SDK access to the full HTTP API.

## Side-effect boundary

A capability-bound process initializes only the subsystems it is authorized to use. In particular, a pure read-side principal does not register the mutation engine and therefore does not run mutation crash recovery as a startup side effect.

`knowledge:read` and `context:compile` may rebuild/read SQLite projections from canonical local sources, but they do not append canonical knowledge events.

## Next capability families

Before exposing durable writes to scoped autonomous agents, add resource-aware policy for at least:

- `knowledge:write`
- `memory:write`
- `session:read` / `session:write` if session authority should be separated from general knowledge
- provenance creation

Before allowing path/project/session-scoped `vault:read`, `knowledge:read`, `context:compile`, or `ai:invoke`, implement filtering against authoritative persisted resources and add adversarial tests showing that out-of-scope data cannot leak through alternate retrieval paths.

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

`vault:read` is currently global-only. Any path/project/session restriction combined with this capability fails MCP startup because search/resources do not yet enforce one consistent resource scope across all alternate read paths.

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

`knowledge:read` supports **project and durable-session scope** because those tools authorize persisted resources before returning data:

- project-scoped object reads require the object's persisted `Project` to be allowed;
- object relations are returned only when both endpoint objects are visible to the principal;
- project memories require an allowed project scope;
- session memories resolve the durable session and authorize its persisted project/session IDs;
- session reads authorize the persisted session project and session ID.

A session-only principal does not automatically become a project-wide knowledge reader: it can read its allowed session/session memories, but project objects or project memories lack the required session binding and are denied.

Path-prefix restrictions are not yet supported for `knowledge:read`, because some structured records have no authoritative file path. A token combining `knowledge:read` with path-prefix scope fails MCP startup.

Structured knowledge write capabilities are intentionally deferred until AgentVault has resource-aware authorization semantics for each operation.

### `context:compile`

Registers only:

- `agentvault.compile_context`

Context compilation can draw from multiple memory and knowledge sources, so this is a distinct authority rather than an implication of ordinary vault reads.

`context:compile` remains global-only. The compiler can currently follow explicit object IDs, cross-object relations, and provenance records in addition to project-filtered notes/memories. Project/session authorization cannot be described as safe until every one of those alternate evidence paths is filtered against the capability principal.

### `ai:invoke`

Registers only:

- `agentvault.ask`

AI/provider invocation is separate from data-read authority because it may consume model/provider credentials, trigger external inference, and internally retrieve vault context.

`ai:invoke` remains global-only until the underlying retrieval/provider path can enforce the same resource scope end-to-end.

## Mutation capabilities

Transactional file mutations retain their independent lifecycle capabilities:

- `mutation:read`
- `mutation:propose`
- `mutation:approve`
- `mutation:commit`
- `mutation:undo`
- `mutation:reject`

Mutation capabilities can be path/project/session scoped because their handlers resolve and authorize the concrete proposal/session resource before acting.

Read-side and mutation capabilities may be combined when their scope semantics are compatible. For example, a project-scoped autonomous agent can combine `knowledge:read` with project-scoped mutation capabilities. A token that also requests global-only `vault:read`, `context:compile`, or `ai:invoke` must remain unscoped until those families gain end-to-end resource filtering.

## Scope boundary

Current scope support is intentionally capability-specific:

| Capability family | Path prefixes | Projects | Sessions |
| --- | --- | --- | --- |
| `mutation:*` | yes | yes | yes |
| `knowledge:read` | no | yes | yes |
| `vault:read` | no | no | no |
| `context:compile` | no | no | no |
| `ai:invoke` | no | no | no |

Unsupported combinations fail closed during MCP runtime registration. AgentVault does not silently ignore a requested scope, and it does not assume caller-supplied arguments are authorization.

## HTTP API boundary

These read-side capabilities currently govern **MCP only**. They do not become generic HTTP API credentials.

The ordinary AgentVault HTTP API continues to require the per-process root token for non-mutation data endpoints. Mutation HTTP routes retain the capability-specific model documented in `TRANSACTIONAL_MUTATIONS.md`.

Keeping the boundaries separate prevents a token minted for a narrow MCP process from unexpectedly gaining browser/SDK access to the full HTTP API.

## Side-effect boundary

A capability-bound process initializes only the subsystems it is authorized to use. In particular, a pure read-side principal does not register the mutation engine and therefore does not run mutation crash recovery as a startup side effect.

`knowledge:read` may rebuild/read the SQLite projection from canonical journal state, but it does not append canonical knowledge events.

## Next capability work

Before exposing durable writes to scoped autonomous agents, add resource-aware policy for at least:

- `knowledge:write`
- `memory:write`
- `session:read` / `session:write` if session authority should be separated from general knowledge
- provenance creation

Before allowing scoped `context:compile`, filter explicit object IDs, relations, provenance/evidence, notes, memories, and prior sessions against the principal and add adversarial alternate-path leakage tests.

Before allowing scoped `vault:read` or `ai:invoke`, make every underlying search/resource/RAG path enforce the same authoritative scope. Path-scoped structured knowledge reads should wait until records without canonical paths have a defined policy rather than treating missing paths as implicitly safe.

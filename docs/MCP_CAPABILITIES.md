# MCP Capability Families

AgentVault MCP supports persistent capability identities without treating one agent token as a generic vault credential. Tool registration is derived from the principal's grants before the server begins serving requests, and a bound token is revalidated on every MCP dispatch so expiry or revocation takes effect without restart.

## Credential modes

AgentVault has three MCP operating modes:

1. **Unbound local stdio** — compatibility mode for a trusted local process. It exposes the existing safe local read/AI surface, structured knowledge/session tools, Context Compiler, proposal/read mutations, and read-only MCP resources.
2. **Capability-bound MCP** — least-authority mode. Only explicitly granted capability families are registered.
3. **Legacy direct-write compatibility** — `--allow-direct-writes` restores old direct user-file writers for a trusted client. It cannot be combined with a capability identity.

HTTP MCP always requires a capability token. The same token authenticates the HTTP transport and determines the registered tool surface.

## Read capabilities

### `vault:read`

Registers file/index-backed vault reads: search, Markdown memory recall, note reads/backlinks, project/recent listings, git status, and read-only MCP resources. It does not imply model invocation, structured writes, or file writes.

`vault:read` is currently global-only. Any path/project/session restriction combined with it fails startup until every file/index-backed retrieval path enforces one authoritative scope.

### `knowledge:read`

Registers only:

- `agentvault.get_object`
- `agentvault.list_memories`
- `agentvault.get_session`

It supports project and durable-session scope against persisted objects, memory scopes, and sessions. Cross-project relation endpoints are filtered. A session-only principal does not become a project-wide object or project-memory reader. Path-prefix scope is unsupported.

### `context:compile`

Registers only `agentvault.compile_context`.

Project/session scope is enforced twice: request scope is bound to persisted session/project state before retrieval, then every returned object, relation, note, memory, session/event, embedded object ID, and provenance view is authorized/sanitized before return. Unknown future item kinds fail closed and visible statistics are recomputed so hidden candidate counts are not exposed.

A session-scoped compiler may use project context for its authorized session but cannot include another session's history. Path-prefix scope remains unsupported.

### `ai:invoke`

Registers only `agentvault.ask`. It remains global-only until RAG/context/provider execution carries the same resource policy end-to-end.

## Durable machine-write capabilities

These capabilities append machine-authored state to the canonical knowledge journal. They **never** grant direct writes to user-authored vault files; those remain under transactional mutation capabilities.

### `knowledge:write`

Registers:

- `agentvault.create_provenance`
- `agentvault.upsert_object`
- `agentvault.create_relation`

For project/session-scoped identities:

- creates use server-generated stable IDs;
- provenance is bound to an authorized durable session and the capability principal's agent identity;
- object writes require session-bound provenance;
- scoped creates cannot introduce canonical file paths and scoped updates cannot change them;
- existing object project scope is checked before an update;
- relation endpoints must share one authorized project;
- session-scoped object updates/relations are ownership-limited: existing objects/endpoints must already carry provenance authorized to that same durable session;
- missing/out-of-scope stable IDs normalize to authorization failure rather than becoming existence probes.

Project scope is deliberately project-wide. Session scope is narrower and uses session-bound provenance as the ownership boundary.

### `memory:write`

Registers only `agentvault.record_memory`.

For resource-scoped identities:

- IDs are server-generated;
- session-bound provenance is required;
- project-scoped principals may write project or authorized-session memories;
- session-scoped principals may write **session memories only** and cannot promote knowledge into project/global memory;
- related objects must belong to the memory's effective project;
- supersession is allowed only within the exact same scope type + scope ID;
- user/agent/organization-global memory writes are denied under resource-scoped identities.

### `session:write`

Registers:

- `agentvault.start_session`
- `agentvault.append_session_event`
- `agentvault.close_session`

The persisted session agent is bound to the capability principal. Project-scoped principals may start sessions only in authorized projects and scoped starts/events use server-generated IDs. Session-scoped principals cannot mint another durable session but may append to/close explicitly granted existing sessions. Cross-project/session lifecycle operations are denied.

Capability-bound session close accepts only `completed`, `failed`, `cancelled`, or `blocked` (omitted status defaults to `completed`).

## Transactional file-mutation capabilities

User-authored file changes retain independent lifecycle capabilities:

- `mutation:read`
- `mutation:propose`
- `mutation:approve`
- `mutation:commit`
- `mutation:undo`
- `mutation:reject`

Mutation capabilities can be path/project/session scoped because their handlers resolve the concrete proposal/session resource before acting.

## Scope support

| Capability family | Path prefixes | Projects | Sessions |
| --- | --- | --- | --- |
| `mutation:*` | yes | yes | yes |
| `knowledge:read` | no | yes | yes |
| `knowledge:write` | no | yes | yes |
| `memory:write` | no | yes | yes |
| `session:write` | no | yes | yes |
| `context:compile` | no | yes | yes |
| `vault:read` | no | no | no |
| `ai:invoke` | no | no | no |

Unsupported combinations fail closed during MCP runtime registration. Caller-supplied labels are never treated as sufficient authorization when authoritative persisted state exists.

## HTTP API boundary

These non-mutation capabilities currently govern MCP only. They do not become generic HTTP API credentials. Ordinary non-mutation HTTP data routes continue to require the per-process root token; mutation HTTP routes retain their capability-specific authorization.

## Side-effect boundary

A capability-bound process initializes only the subsystems it can use. Knowledge/context/durable-machine-write authority does not initialize transactional file-mutation recovery unless a `mutation:*` capability is also present.

## Next capability work

The highest-value remaining read-side gaps are scoped `vault:read` and scoped `ai:invoke`. `vault:read` requires consistent policy across search, graph/resources, recent/project listings, note reads, backlinks, and Markdown recall. `ai:invoke` should inherit scoped Context Compiler/RAG policy so prompts cannot receive out-of-scope context.

Path-scoped structured knowledge remains intentionally deferred until records without canonical paths have an explicit policy rather than being treated as implicitly safe.

# MCP Capability Families

AgentVault MCP supports persistent capability identities without treating one agent token as a generic vault credential. Tool registration is derived from the principal's grants before the server begins serving requests, and a bound token is revalidated on every MCP dispatch so expiry or revocation takes effect without restart.

## Credential modes

AgentVault has three MCP operating modes:

1. **Unbound local stdio** — compatibility mode for a trusted local process. It exposes the existing safe local read/AI surface, structured knowledge/session tools, Context Compiler, proposal/read mutations, and read-only MCP resources.
2. **Capability-bound MCP** — least-authority mode. Only explicitly granted capability families are registered.
3. **Legacy direct-write compatibility** — `--allow-direct-writes` restores old direct user-file writers for a trusted client. It cannot be combined with a capability identity.

HTTP MCP always requires a capability token. The same token authenticates the HTTP transport and determines the registered tool surface.

## Read-side capabilities

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

It does **not** grant provenance creation, object upsert, relation creation, durable memory recording, or session lifecycle writes.

`knowledge:read` supports **project and durable-session scope** against persisted resources:

- project-scoped object reads require the object's persisted `Project`;
- relation edges are returned only when both endpoint objects are visible;
- project memories require an allowed project;
- session memories resolve and authorize the durable session;
- session reads authorize both persisted project and session ID.

A session-only principal does not become a project-wide knowledge reader. Project objects and project memories have no session binding and are denied through `knowledge:read`.

Path-prefix restrictions remain unsupported because structured records do not always have an authoritative file path.

### `context:compile`

Registers only:

- `agentvault.compile_context`

Context compilation is a distinct authority because it aggregates multiple evidence planes. `context:compile` now supports **project and durable-session scope**, but only through a two-stage policy:

1. **Preauthorization before retrieval**
   - durable session IDs are loaded and bound to their persisted project/agent;
   - session-scoped principals must supply an allowed session;
   - a single allowed project may be defaulted, while multiple allowed projects require an explicit choice;
   - workspace scope is constrained to allowed project/session context;
   - explicit object IDs are loaded and authorized before compilation;
   - scoped missing/forbidden object or session IDs return the same forbidden result so the compiler cannot be used as an existence oracle.

2. **Evidence filtering before return**
   - objects must belong to the authorized project context;
   - relations are retained only when all endpoint objects are authorized;
   - project/session memories are authorized from their persisted scope;
   - agent/user/organization memories without trustworthy project/session binding are excluded;
   - notes are checked against persisted project metadata;
   - prior/current session items and events are checked against persisted sessions;
   - session-scoped compilation may use project context for its authorized session, but other sessions in the same project remain hidden;
   - unknown future context item kinds fail closed;
   - embedded `objectIds` are filtered independently;
   - provenance evidence paths, quotes, and source IDs are redacted for scoped bundles because provenance can independently reference out-of-scope material;
   - bundle statistics are recomputed from visible items so hidden-candidate counts are not exposed.

Path-prefix scope remains unsupported for `context:compile` until every structured and file-backed evidence kind has one canonical path policy.

### `ai:invoke`

Registers only:

- `agentvault.ask`

AI/provider invocation is separate from data-read authority because it may consume model/provider credentials, trigger external inference, and internally retrieve vault context.

`ai:invoke` remains global-only until its retrieval/provider path can enforce authoritative project/session/path scope end-to-end.

## Mutation capabilities

Transactional file mutations retain their independent lifecycle capabilities:

- `mutation:read`
- `mutation:propose`
- `mutation:approve`
- `mutation:commit`
- `mutation:undo`
- `mutation:reject`

Mutation capabilities can be path/project/session scoped because their handlers resolve and authorize the concrete proposal/session resource before acting.

Read-side and mutation capabilities may be combined when their scope semantics are compatible. A project/session-scoped autonomous agent can now combine `knowledge:read`, `context:compile`, and scoped mutation capabilities without receiving approval/commit authority it was not granted.

## Scope boundary

Current scope support is capability-specific:

| Capability family | Path prefixes | Projects | Sessions |
| --- | --- | --- | --- |
| `mutation:*` | yes | yes | yes |
| `knowledge:read` | no | yes | yes |
| `context:compile` | no | yes | yes |
| `vault:read` | no | no | no |
| `ai:invoke` | no | no | no |

Unsupported combinations fail closed during MCP runtime registration. AgentVault does not silently ignore requested scope, and caller-supplied labels are never treated as sufficient authorization when authoritative persisted state exists.

## HTTP API boundary

These read-side capabilities currently govern **MCP only**. They do not become generic HTTP API credentials.

The ordinary AgentVault HTTP API continues to require the per-process root token for non-mutation data endpoints. Mutation HTTP routes retain the capability-specific model documented in `TRANSACTIONAL_MUTATIONS.md`.

## Side-effect boundary

A capability-bound process initializes only the subsystems it is authorized to use. A pure read-side principal does not register the mutation engine and therefore does not run mutation crash recovery as a startup side effect.

`knowledge:read` and `context:compile` may replay/read rebuildable projections from canonical local sources, but neither appends canonical knowledge events.

## Next capability work

Before exposing durable writes to scoped autonomous agents, add resource-aware policy for at least:

- `knowledge:write`
- `memory:write`
- session lifecycle writes
- provenance creation

Before allowing scoped `vault:read`, make search, graph/resources, recent/project listings, note reads, backlinks, and Markdown recall enforce one authoritative scope without alternate-path leaks.

Before allowing scoped `ai:invoke`, make its retrieval and provider execution inherit the same resource policy and add tests proving model prompts cannot receive out-of-scope context.

Path-scoped structured reads/context should wait until records without canonical paths have an explicit policy rather than treating missing paths as implicitly safe.

# Saved Views

AgentVault saved views are portable YAML files stored in:

```text
.agentvault/views/<id>.yaml
```

They are declarative filters over the canonical Markdown index. A saved view does
not introduce another source of truth: it compiles into the existing
`search.Query` path used by HTTP, MCP, and other clients.

Every loaded view also exposes a lowercase SHA-256 `contentHash` over the
exact raw YAML bytes. This is a definition revision, not the schema
`version`: comments, formatting, or query edits all change the hash.

## Version 1

```yaml
version: 1
name: Open architecture decisions

query:
  q: architecture
  types: [decision]
  projects: [nulang, agentvault]
  statuses: [proposed, accepted]
  tags: [architecture]
  pinned: true

limit: 50

columns:
  - title
  - status
  - project
  - updated
```

All query fields are optional. Multiple values in `types`, `projects`,
`statuses`, and `tags` use the same multi-value semantics as AgentVault
search.

`columns` is presentation metadata. The core query engine ignores it, which
keeps a view portable across CLI/UI/agent clients.

## HTTP

- `GET /views` — list saved views.
- `GET /views/{id}` — read one view definition.
- `POST /views/{id}/run` — execute the view through the canonical search
  engine.

## MCP

Read-only MCP surfaces expose:

- `agentvault.list_views`
- `agentvault.run_view`

This allows agents to consume the same explicit knowledge scopes users define,
without granting arbitrary SQL or introducing a second query runtime.

## Design constraints

- View IDs cannot contain path separators.
- Only `.yaml` and `.yml` files are loaded.
- Unsupported versions fail closed.
- Views never store note content or derived query results.
- SQLite remains a rebuildable index; Markdown/YAML remains canonical.


## Context Compiler scope

A saved view can be used as an explicit file-backed retrieval scope for
deterministic context compilation:

```json
{
  "task": "prepare the production deployment",
  "project": "adacavo",
  "viewId": "production"
}
```

The MCP equivalent is `view_id` on `agentvault.compile_context`.

Compiled bundles record the resolved `viewVersion` and `viewContentHash`.
A caller that needs a reproducible scope may also send
`expectedViewContentHash` (HTTP/TypeScript) or
`expected_view_content_hash` (MCP). Compilation fails closed before candidate
collection when the loaded view hash differs from the pin.

When `viewId` is present:

- ordinary Markdown notes must match the saved view;
- classified Markdown memories must match both their normal
  workspace/session validity rules and the saved view;
- an explicit `project` request intersects the saved view project filter and
  can never be broadened by it;
- structured journal memories, objects, relations, facts, episodes, and
  sessions keep their existing project/session scope and authorization rules;
- missing, invalid, or unsupported saved views fail closed;
- a supplied definition hash must match the exact current YAML bytes.

The view is a selector, not a second ranking engine. Once file-backed candidates
are selected, Context Compiler still applies its normal deterministic
task-relevance, ranking, token-budget, and max-item logic.

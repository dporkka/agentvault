# Saved Views

AgentVault saved views are portable YAML files stored in:

```text
.agentvault/views/<id>.yaml
```

They are declarative filters over the canonical Markdown index. A saved view does
not introduce another source of truth: it compiles into the existing
`search.Query` path used by HTTP, MCP, and other clients.

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

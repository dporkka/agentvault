# AgentVault Universal Knowledge Core

AgentVault is a local-first knowledge and agent-state substrate. Human-authored
knowledge remains portable Markdown/YAML. Structured machine-authored state is
persisted as append-only events in the vault and projected into SQLite for fast
queries.

This document defines the invariants that integrations, agents, UI clients, and
future sync services must preserve.

## 1. Source-of-truth model

AgentVault deliberately has two canonical local representations:

1. **Markdown/YAML/files** are canonical for user-authored documents and
   artifacts.
2. **`80-agent-runs/knowledge.journal.jsonl`** is canonical for structured
   machine-authored knowledge and durable agent state.

`<vault>/.agentvault/agentvault.db` remains a rebuildable projection/cache. A
feature is not durable merely because it wrote a row to SQLite.

```text
Human-authored files                 Machine-authored state
Markdown / YAML / assets             knowledge.journal.jsonl
          |                                   |
          +----------------+------------------+
                           |
                           v
                    projection/indexer
                           |
              +------------+------------+
              |            |            |
             FTS         SQLite      embeddings
              |            |            |
              +------------+------------+
                           |
                           v
                 HTTP / MCP / UI / SDKs
```

### Journal write rule

Durable structured mutations follow this order:

```text
validate -> construct full event -> append -> fsync -> project to SQLite
```

If projection fails after the journal append, the canonical event still exists
and the projection can recover on replay. The reverse order is forbidden.

Journal events are versioned and contain stable IDs, timestamps, event type,
and a complete payload. Replay must be idempotent.

## 2. Universal object model

A `KnowledgeObject` is the stable envelope shared by application-specific
knowledge. It is intentionally generic enough to represent:

- project
- repository
- decision
- task
- requirement
- person
- organization
- dataset
- artifact
- deployment
- agent
- workflow
- document reference

Applications can introduce new types without schema migrations by putting
object-specific fields in `data`, while globally useful fields remain indexed.

```json
{
  "id": "obj_...",
  "type": "decision",
  "title": "Use AgentVault as the shared context layer",
  "status": "accepted",
  "organization": "example-org",
  "project": "example-project",
  "canonicalPath": "30-decisions/shared-context.md",
  "data": {},
  "provenanceId": "prov_...",
  "createdAt": "...",
  "updatedAt": "..."
}
```

### Stable identity

Object identity is independent of title and file path. Renaming a document must
not break graph relationships or application references.

`canonicalPath` is optional. When present it points at the portable document or
artifact that humans should treat as the canonical authored representation.

## 3. Relationships and temporal knowledge

`ObjectRelation` represents a typed edge between stable object IDs.

Relationships may include:

- `affects`
- `depends_on`
- `implements`
- `owns`
- `references`
- `produced_by`
- `deployed_as`
- `supersedes`

Relations can carry `validFrom`, `validTo`, `confidence`, provenance, and
metadata. This permits the graph to represent knowledge that changes over time
without rewriting history.

A later context compiler should prefer relations valid for the requested point
in time while retaining historical relationships for audit/reconstruction.

## 4. Provenance

Machine-created knowledge should answer:

- where did this come from?
- when was it observed?
- which agent/model produced or inferred it?
- what concrete evidence supports it?
- how confident is the producer?

`ProvenanceRecord` is append-only. Existing provenance must never be silently
edited to make an old inference appear supported by new evidence.

Confidence is in `[0, 1]` and defaults to `1` only when omitted. An explicit
zero is valid and must be preserved.

## 5. Memory model

AgentVault distinguishes four memory classes rather than treating all context as
vector-searchable text:

| Memory type | Purpose |
| --- | --- |
| `working` | Temporary context relevant to an active task/session |
| `episodic` | What happened: attempts, outcomes, failures, interactions |
| `semantic` | Durable facts and assertions about entities/projects |
| `procedural` | Reusable know-how, playbooks, release procedures, skills |

Every memory has an explicit scope (`user`, `organization`, `project`, `agent`,
`session`, etc.), confidence, optional provenance, optional temporal validity,
and optional `supersedesId`.

Supersession is append-only. Older memory remains reconstructable; readers can
choose whether they need current or historical context.

## 6. Durable agent sessions

An `AgentSession` is a durable workspace, not a chat transcript. It records the
objective and execution context around an agent's work.

```text
AgentSession
  objective
  agent identity
  project
  branch/worktree
  initial context snapshot
  append-only events
  terminal outcome
```

Events can represent decisions, tool results, artifacts, failures,
verifications, commits, deployments, or other domain-specific activity.

Closed sessions are immutable with respect to new events. If follow-up work is
required, create a new session and relate it to the prior one at the knowledge
layer.

## 7. Integration boundaries

Other products should integrate with AgentVault through stable contracts, not
by reading or mutating the SQLite schema directly.

### TypeScript

The shared `@agentvault/contract` package exports `createKnowledgeClient()`.

```ts
import { createKnowledgeClient } from '@agentvault/contract';

const vault = createKnowledgeClient({
  baseUrl: 'http://127.0.0.1:7777',
  token: process.env.AGENTVAULT_TOKEN,
});

const session = await vault.startSession({
  agentId: 'backend-engineer',
  project: 'my-project',
  objective: 'Implement contract renewal workflow',
});

await vault.recordMemory({
  memoryType: 'semantic',
  scopeType: 'project',
  scopeId: 'my-project',
  content: 'Contract renewal requires finance approval.',
});
```

### MCP

The CLI exposes the same knowledge substrate through MCP:

- `agentvault.create_provenance`
- `agentvault.upsert_object`
- `agentvault.get_object`
- `agentvault.create_relation`
- `agentvault.record_memory`
- `agentvault.list_memories`
- `agentvault.start_session`
- `agentvault.append_session_event`
- `agentvault.get_session`
- `agentvault.close_session`

MCP is an adapter. It must not introduce a parallel storage or memory model.

### HTTP

Current structured endpoints:

```text
GET/POST      /objects
GET/PUT       /objects/{id}
GET           /objects/{id}/relations
POST          /relations
GET/POST      /provenance[/id]
GET/POST      /memory
POST          /sessions
GET           /sessions/{id}
POST          /sessions/{id}/events
POST          /sessions/{id}/close
```

## 8. Boundaries for other products

### Transactional business systems

AgentVault is not an ERP/CRM transactional database. A product such as Adacavo
must keep PostgreSQL/domain services authoritative for invoices, contracts,
bookings, permissions, and other business invariants.

```text
Business DB/domain service --events--> AgentVault knowledge
          ^                              |
          |                              v
          +---- authorized tools <--- agent/context
```

Agents use AgentVault for context/provenance/memory and invoke the owning
application's authorized domain API for business mutations.

### Distributed agent runtimes

AgentVault owns durable knowledge and context. Distributed runtimes (for
example Nulang/Nulang Cloud) own scheduling, supervision, actor execution,
retries, and distributed coordination.

Do not turn AgentVault into a second workflow engine.

### Local-first applications

Applications such as Apex may eventually share low-level local-first libraries
(SQLite migrations, content addressing, CRDT/sync primitives), but application
semantics and user interfaces remain separate. AgentVault exposes knowledge
through contracts rather than becoming an embedded office suite.

## 9. Next platform primitives

The next implementation layers should build on this core in this order:

1. **Context Compiler** — evidence-backed, token-budgeted context assembled from
   notes, objects, memories, graph relations, and session history.
2. **Transactional agent mutations** — propose/dry-run/diff/approve/commit/undo
   for user-authored files and structured state.
3. **Current-memory resolution** — contradiction/supersession/temporal filters
   rather than returning every historical assertion equally.
4. **Agent identity and capability policy** — persistent agents, scopes, skills,
   and least-privilege tool permissions.
5. **Skills format** — portable vendor-neutral skill bundles.
6. **Git worktree orchestration** — session-to-branch/worktree/commit provenance.
7. **Optional sync/cloud** — encrypted multi-device/team synchronization without
   making cloud state authoritative.

## 10. Non-goals and invariants

- Do not make SQLite the only durable copy of agent state.
- Do not let MCP or plugins bypass validation/provenance rules.
- Do not let AgentVault directly mutate another application's private database.
- Do not overwrite contradictory memories to make history look consistent.
- Do not couple integrations to the desktop UI.
- Do not require cloud services for local operation.
- Do not adopt copyleft/source-available competitor code into the core unless a
  separate licensing decision is made explicitly.

# AgentVault Universal Knowledge Core

AgentVault is a local-first knowledge and agent-state substrate. Human-authored
knowledge remains portable Markdown/YAML. Structured machine-authored state is
persisted as append-only events in the vault and projected into SQLite for fast
queries.

This document defines the invariants that integrations, agents, UI clients, and
future sync services must preserve.

## 1. Source-of-truth model

AgentVault deliberately has two canonical local representations:

1. **Markdown/YAML/files** are canonical for user-authored documents, memories,
   and artifacts.
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

The Context Compiler prefers relations valid for the requested point in time
while retaining historical relationships for audit and reconstruction.

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

AgentVault separates **memory class** from **memory kind**.

Memory class describes lifecycle/cognitive role:

| Memory class | Purpose |
| --- | --- |
| `working` | Temporary context relevant to an active task/session |
| `episodic` | What happened: attempts, outcomes, failures, interactions |
| `semantic` | Durable assertions about entities, users, or projects |
| `procedural` | Reusable know-how, playbooks, release procedures, skills |

Memory kind describes semantic meaning:

| Memory kind | Typical meaning |
| --- | --- |
| `observation` | Direct observation or measured state |
| `episode` | Event/interaction summary |
| `fact` | Asserted factual knowledge |
| `preference` | User/team preference |
| `decision` | Chosen direction or architectural decision |
| `procedure` | Reusable steps or operating procedure |
| `constraint` | Requirement, limitation, policy, or invariant |
| `summary` | Condensed knowledge derived from other material |

The dimensions are orthogonal. A semantic memory may be a fact, preference, or
decision; an episodic memory may be an observation or episode.

Every memory has scope, confidence, optional provenance, optional temporal
validity, and optional supersession history. Markdown-backed memory uses
`workspace_id`, `agent_id`, and `session_id`; journal-backed machine memory uses
a general `scopeType`/`scopeId` pair.

### Two canonical memory sources

Human/file memory is canonical Markdown/YAML. Machine memory is canonical
journal state. They share the same class/kind vocabulary but do not share a
single canonical storage format.

The Context Compiler merges both into one retrieval plane. Classified Markdown
memory is never allowed to bypass memory scope/validity checks by re-entering
through generic FTS note search.

### Backward compatibility

Existing Markdown that has `memory_kind` but no `memory_class` normalizes to
`memory_class: semantic`. Explicit invalid classes or kinds are rejected instead
of being silently coerced.

Supersession is append-only. Older memory remains reconstructable; readers can
choose current or historical context.

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

## 7. Context Compiler

`POST /context/compile` and `agentvault.compile_context` assemble deterministic,
evidence-backed context from:

- current durable session/events;
- journal-backed machine memories;
- Markdown-backed scoped memories;
- typed objects and valid relations;
- ordinary indexed notes;
- recent durable session history.

`workspaceId` controls Markdown memory visibility. `project` remains a content
filter and is used as the workspace fallback when `workspaceId` is omitted for
compatibility with the original compiler contract.

The compiler does not call an LLM. It ranks locally, enforces temporal and
supersession rules, deduplicates classified memories from generic note search,
and fits results into a deterministic token budget.

## 8. Integration boundaries

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
  memoryClass: 'semantic',
  memoryKind: 'decision',
  scopeType: 'project',
  scopeId: 'my-project',
  content: 'Contract renewal requires finance approval.',
});

const context = await vault.compileContext({
  task: 'Implement contract renewal workflow',
  workspaceId: 'my-project',
  project: 'my-project',
  agentId: 'backend-engineer',
});
```

### MCP

The CLI exposes the same knowledge substrate through MCP:

- `agentvault.recall_memories` — Markdown-backed scoped recall
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
- `agentvault.compile_context`

MCP is an adapter. It must not introduce a parallel storage or memory model.

### HTTP

Current memory/knowledge endpoints:

```text
GET           /memories
GET           /memories/{id}
GET/POST      /objects
GET/PUT       /objects/{id}
GET           /objects/{id}/relations
POST          /relations
POST          /provenance
GET           /provenance/{id}
GET/POST      /memory
POST          /context/compile
POST          /sessions
GET           /sessions/{id}
POST          /sessions/{id}/events
POST          /sessions/{id}/close
```

Plural `/memories` is the Markdown-backed read surface. Singular `/memory` is
the journal-backed machine-memory surface.

## 9. Boundaries for other products

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

AgentVault owns durable knowledge and context. Distributed runtimes such as
Nulang/Nulang Cloud own scheduling, supervision, actor execution, retries, and
distributed coordination.

Do not turn AgentVault into a second workflow engine.

### Local-first applications

Applications such as Apex may eventually share low-level local-first libraries
(SQLite migrations, content addressing, CRDT/sync primitives), but application
semantics and user interfaces remain separate. AgentVault exposes knowledge
through contracts rather than becoming an embedded office suite.

## 10. Next platform primitives

With the knowledge core and unified Context Compiler in place, the next layers
should build in this order:

1. **Transactional agent mutations** — propose/dry-run/diff/approve/commit/undo
   for user-authored files and structured state.
2. **Contradiction resolution** — candidate extraction, entity resolution,
   contradiction detection, confidence/authority ranking, and explicit
   supersession proposals rather than silent overwrite.
3. **Agent identity and capability policy** — persistent agents, scopes, skills,
   and least-privilege tool permissions.
4. **Skills format** — portable vendor-neutral skill bundles.
5. **Git worktree orchestration** — session-to-branch/worktree/commit provenance.
6. **Journal integrity and compaction** — hash-linked segments, snapshots,
   retention/redaction controls, and validated compaction.
7. **Optional sync/cloud** — encrypted multi-device/team synchronization without
   making cloud state authoritative.

## 11. Non-goals and invariants

- Do not make SQLite the only durable copy of agent state.
- Do not let MCP or plugins bypass validation/provenance rules.
- Do not let AgentVault directly mutate another application's private database.
- Do not overwrite contradictory memories to make history look consistent.
- Do not let scoped memories bypass visibility checks through generic search.
- Do not couple integrations to the desktop UI.
- Do not require cloud services for local operation.
- Do not adopt copyleft/source-available competitor code into the core unless a
  separate licensing decision is made explicitly.

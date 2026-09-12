# AgentVault Context Compiler

The Context Compiler turns AgentVault's heterogeneous local knowledge into a
bounded, evidence-backed context bundle for an agent task. It is deterministic
and does not call an LLM.

## Request

```json
{
  "task": "Implement contract renewal finance approval workflow",
  "workspaceId": "adacavo",
  "project": "adacavo",
  "agentId": "backend-engineer",
  "sessionId": "session_...",
  "objectIds": ["obj_..."],
  "tokenBudget": 8000,
  "maxItems": 40,
  "asOf": "2026-09-10T12:00:00Z"
}
```

Only `task` is required. `workspaceId` controls visibility for Markdown-backed
memory. `project` is a content/domain filter. For compatibility with the first
compiler contract, `workspaceId` defaults to `project` when it is omitted.

`tokenBudget` defaults to 8,000 context-item tokens and is bounded to
256–128,000. `maxItems` defaults to 40 and is bounded to 1–200. `asOf` defaults
to the current UTC time.

The token count is a deterministic approximation used for admission/truncation,
not a provider-specific tokenizer. Providers may tokenize the same text
slightly differently, so callers should keep a small model-window reserve.

## Retrieval order

The unified compiler gathers candidates from six layers:

1. current durable session and its most recent events;
2. journal-backed session/project/agent machine memories after temporal and
   supersession resolution;
3. Markdown-backed human/file memories after workspace/agent/session visibility,
   temporal validity, contextual supersession, and optional project filtering;
4. explicitly requested and task-relevant project knowledge objects plus
   currently valid relations;
5. project/task-relevant indexed notes that are not classified memories;
6. recent matching agent sessions for execution history.

Gathering order does not determine output order. Candidates receive local,
deterministic relevance scores and are sorted by score, kind, then stable ID.

## Two canonical memory sources, one retrieval plane

AgentVault intentionally keeps two durable local representations:

- human-authored memory is canonical Markdown/YAML and is projected into the
  `notes` memory columns;
- machine-authored structured memory is canonical append-only journal data and
  is projected into `memory_records`.

Context compilation merges both after applying their source-specific visibility
and validity rules. SQLite remains a rebuildable projection/cache for both.

A classified Markdown memory can only enter compiled context through the memory
resolver. Generic FTS note search explicitly excludes classified memory notes.
This prevents a workspace-, agent-, session-, expired-, or superseded memory
that failed memory visibility checks from leaking back into context as an
ordinary note.

## Memory dimensions

Memory **class** describes lifecycle/cognitive role:

- `working`
- `episodic`
- `semantic`
- `procedural`

Memory **kind** describes meaning:

- `observation`
- `episode`
- `fact`
- `preference`
- `decision`
- `procedure`
- `constraint`
- `summary`

The dimensions are orthogonal. For example, a semantic memory may be a fact,
preference, or decision, while an episodic memory may be an observation or
episode.

## Current-memory semantics

A memory is current at `asOf` when:

- `validFrom` is absent or not later than `asOf`;
- `validTo` is absent or later than `asOf`; and
- no visible memory valid at `asOf` supersedes it within the applicable scope.

AgentVault does not delete the old memory. Historical compilation can select a
past `asOf` and recover knowledge that was valid then.

For Markdown memory, scope inheritance is explicit: global memory is visible to
narrower contexts, while workspace-, agent-, or session-scoped memory is only
visible when each populated scope dimension matches the compilation context.

## Temporal relations

Object relations use the same half-open validity rule:

```text
validFrom <= asOf < validTo
```

An absent boundary is unbounded.

## Provenance

Structured context items carry a compact provenance object inline when the
underlying object, relation, memory, or session event has provenance. This
includes source type/ID, producer agent/session/model, confidence, observation
time, and concrete evidence references.

Markdown memories receive provenance pointing at their canonical note ID/path,
plus any source metadata declared in frontmatter. Ordinary notes retain their
stable note ID and vault-relative path and do not require a synthetic durable
provenance row merely to be included.

## Budget behavior

Items are admitted from highest to lowest score. If the next item does not fit,
its content is truncated when enough budget remains; otherwise it is dropped.
The output reports:

- `estimatedTokens` — tokens used by included context items;
- `truncated` — whether any candidate was truncated or omitted for budget/item
  limits;
- candidate/included/dropped counts;
- counts by included source kind.

The task/envelope metadata is not charged against `tokenBudget`; the budget is
specifically the context payload intended to be inserted beside a model prompt.

## Interfaces

### HTTP

```text
POST /context/compile
```

The endpoint uses the same local AgentVault auth middleware as other protected
operations. Compilation does not mutate state, but compiled context may contain
private knowledge and must not be exposed through an unauthenticated local
process boundary.

### MCP

```text
agentvault.compile_context
```

The MCP input uses `workspace_id`, `project`, `agent_id`, `session_id`,
`object_ids`, `token_budget`, `max_items`, and `as_of`.

### TypeScript

```ts
import { createKnowledgeClient } from '@agentvault/contract';

const vault = createKnowledgeClient({
  baseUrl: 'http://127.0.0.1:7777',
  token: process.env.AGENTVAULT_TOKEN,
});

const context = await vault.compileContext({
  task: 'Implement contract renewal finance approval workflow',
  workspaceId: 'adacavo',
  project: 'adacavo',
  agentId: 'backend-engineer',
  tokenBudget: 8000,
});
```

## Boundary

The compiler selects and packages context. It does not decide what action to
take, invoke a model, execute tools, mutate another product's database, or run a
workflow. Agent runtimes remain responsible for reasoning/execution; owning
applications remain responsible for transactional domain invariants.

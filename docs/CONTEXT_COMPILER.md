# AgentVault Context Compiler

The Context Compiler turns AgentVault's heterogeneous local knowledge into a
bounded, evidence-backed context bundle for an agent task. It is deterministic
and does not call an LLM.

## Request

```json
{
  "task": "Implement contract renewal finance approval workflow",
  "project": "adacavo",
  "agentId": "backend-engineer",
  "sessionId": "session_...",
  "objectIds": ["obj_..."],
  "tokenBudget": 8000,
  "maxItems": 40,
  "asOf": "2026-09-10T12:00:00Z"
}
```

Only `task` is required. `tokenBudget` defaults to 8,000 context-item tokens and
is bounded to 256–128,000. `maxItems` defaults to 40 and is bounded to 1–200.
`asOf` defaults to the current UTC time.

The token count is a deterministic approximation used for admission/truncation,
not a provider-specific tokenizer. Providers may tokenize the same text
slightly differently, so callers should keep a small model-window reserve.

## Retrieval order

The compiler gathers candidates from five layers:

1. current durable session and its most recent events;
2. current session/project/agent memories after temporal and supersession
   resolution;
3. explicitly requested and task-relevant project knowledge objects plus
   currently valid relations;
4. project/task-relevant indexed notes;
5. recent matching agent sessions for execution history.

Gathering order does not determine output order. Candidates receive local,
deterministic relevance scores and are sorted by score, kind, then stable ID.

## Current-memory semantics

A memory is current at `asOf` when:

- `validFrom` is absent or not later than `asOf`;
- `validTo` is absent or later than `asOf`; and
- no memory valid at `asOf` supersedes it within the loaded scopes.

AgentVault does not delete the old memory. Historical compilation can select a
past `asOf` and recover knowledge that was valid then.

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

Notes retain their stable note ID and vault-relative path. They do not require a
synthetic provenance row merely to be included in context because the canonical
Markdown file is itself directly addressable evidence.

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

The endpoint uses the same local AgentVault write-auth middleware as other POST
operations even though compilation itself does not mutate state. This avoids
exposing private compiled context through unauthenticated cross-process calls.

### MCP

```text
agentvault.compile_context
```

### TypeScript

```ts
import { createKnowledgeClient } from '@agentvault/contract';

const vault = createKnowledgeClient({
  baseUrl: 'http://127.0.0.1:7777',
  token: process.env.AGENTVAULT_TOKEN,
});

const context = await vault.compileContext({
  task: 'Implement contract renewal finance approval workflow',
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

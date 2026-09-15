# External Integration Events

AgentVault can act as the durable, human-readable context and memory plane for external agent systems. The preferred integration surface is `POST /capture` because captures remain file-first Markdown, are indexed automatically, and support retry-safe idempotency through `external_id`.

## Ownership boundary

AgentVault stores durable context, evidence, decisions, and lifecycle history. It does **not** own agent execution, repository sandboxes, deployment, or workflow orchestration.

For the current platform split:

- **Dev Plane** owns software-development execution, approvals, testing, security review, and pull-request delivery.
- **AgentVault** owns durable agent-readable context and source-grounded retrieval.
- **Nulang Cloud** may provide execution/runtime infrastructure behind Dev Plane runtime providers.
- Product repositories own their product-specific domain agents and business workflows.

## Retry-safe event capture

External systems should assign one stable `external_id` per logical lifecycle event. AgentVault checks inbox captures for an existing matching `external_id` and returns the existing capture instead of creating a duplicate.

Recommended form:

```text
<system>:<entity-type>:<entity-id>:<event>
```

Examples:

```text
dev-plane:task:7d4d...:created
dev-plane:run:55a2...:completed
dev-plane:review:a190...:completed
api-factory:idea:9c31...:validated
```

Do not use timestamps or retry attempt numbers as the idempotency key for the same logical event.

## Capture shape

```json
{
  "type": "dev-plane.task.created",
  "title": "Dev Plane task created: Add webhook retry policy",
  "text": "Human-readable context and structured details.",
  "project": "dev-plane",
  "tags": ["dev-plane", "task", "created"],
  "external_id": "dev-plane:task:7d4d:created"
}
```

`type` is an integration discriminator; captures remain inbox notes. Put identifiers and important structured values in the body as well so the note remains understandable without a proprietary index.

## Recommended lifecycle coverage

For autonomous development systems, retain events that explain *why* and *how* work happened rather than every low-level log line:

1. task/spec created or materially revised;
2. architectural or product decision;
3. agent handoff or human escalation;
4. approval requested/resolved;
5. review/security finding that changes the outcome;
6. run failure with actionable diagnosis;
7. run/PR/release completion with final artifact references;
8. postmortem or measured outcome.

Raw command streams, token-by-token model output, and ephemeral progress should stay in the execution/observability system unless they are needed for an incident record.

## Retrieval guidance

Use stable tags for broad filtering (`dev-plane`, `api-factory`, `review`, `decision`) and project fields for product boundaries. Hybrid FTS/vector retrieval is appropriate for questions such as:

- Why was this implementation chosen?
- What failed in earlier attempts?
- Which security findings blocked release?
- What evidence justified building this API?

The answer pipeline should cite the retained notes rather than relying on ungrounded agent memory.

## Security

The local HTTP API security model still applies. Do not expose the AgentVault loopback API directly as a public multi-tenant event collector. A remote or multi-tenant deployment needs a stronger identity/authorization boundary in front of the vault service.

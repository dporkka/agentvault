package templates

var DeveloperOSTemplate = StarterTemplate{
	Name:        "developer",
	Description: "Developer operating system with architecture decision records, runbooks, and engineering standards",
	Folders:     []string{},
	Files: map[string]string{
		"30-decisions/database-choice.md": `---
id: dec_dev_001
type: decision
title: Use PostgreSQL as Primary Database
status: active
confidence: high
project: platform
tags: [architecture, database, storage]
created: 2026-01-10
updated: 2026-01-10
review_after: 2026-07-10
---

# Use PostgreSQL as Primary Database

## Decision

Use PostgreSQL as the primary database. Add Redis for caching and queues. Use S3 for object storage.

## Reasoning

- PostgreSQL: proven, ACID, great ecosystem, JSON support
- Redis: fast cache, pub/sub, job queues
- S3: durable, cheap, unlimited object storage
- Team has deep PostgreSQL experience

## Alternatives Considered

- MongoDB: flexible schema but weaker consistency guarantees
- DynamoDB: AWS lock-in, complex pricing
- CockroachDB: distributed but overkill for our scale

## Tradeoffs

- Operational complexity of running PostgreSQL
- Need migration strategy for schema changes
- Vertical scaling limits before sharding needed

## Revisit when

- Write throughput exceeds 10K TPS
- Need multi-region active-active
- Team grows to 50+ engineers
`,
		"30-decisions/api-design.md": `---
id: dec_dev_002
type: decision
title: REST API with OpenAPI Specification
status: active
confidence: high
project: platform
tags: [architecture, api, rest]
created: 2026-01-15
updated: 2026-01-15
review_after: 2026-07-15
---

# REST API with OpenAPI Specification

## Decision

Use REST with OpenAPI 3.0 spec. JSON API format. Version in URL (/v1/).

## Reasoning

- REST: simple, universally understood, great tooling
- OpenAPI: auto-generated docs, client SDKs, validation
- JSON API: consistent response format
- URL versioning: explicit, cache-friendly

## Alternatives Considered

- GraphQL: flexible queries but adds complexity, caching harder
- gRPC: fast but requires proxy for web clients
- tRPC: type-safe but framework-specific

## Tradeoffs

- More network requests than GraphQL for complex UIs
- API versioning overhead
- Less type safety than gRPC/tRPC

## Revisit when

- Mobile app needs complex data fetching
- Public API has many consumers with different needs
`,
		"30-decisions/deployment-strategy.md": `---
id: dec_dev_003
type: decision
title: Docker Compose for Dev, Kubernetes for Production
status: active
confidence: medium
project: platform
tags: [architecture, deployment, devops]
created: 2026-02-01
updated: 2026-02-01
review_after: 2026-08-01
---

# Docker Compose for Dev, Kubernetes for Production

## Decision

Local dev uses Docker Compose. Production uses managed Kubernetes (EKS/GKE).

## Reasoning

- Docker Compose: simple, fast local setup, all developers use same environment
- Kubernetes: industry standard, auto-scaling, self-healing, rich ecosystem
- Managed K8s: reduces operational burden vs self-managed

## Alternatives Considered

- Nomad: simpler but smaller ecosystem
- ECS: AWS-specific, less portable
- Fly.io/Railway: opinionated PaaS, less control

## Tradeoffs

- K8s complexity: steep learning curve, YAML heavy
- Compose/K8s mismatch: not identical environments
- Cost: managed K8s is expensive at small scale

## Revisit when

- Team grows beyond platform engineers
- Need multi-cloud deployment
- Cost becomes significant
`,
		"30-decisions/monitoring-stack.md": `---
id: dec_dev_004
type: decision
title: Prometheus + Grafana for Metrics, PagerDuty for Alerts
status: active
confidence: high
project: platform
tags: [architecture, monitoring, observability]
created: 2026-02-15
updated: 2026-02-15
review_after: 2026-08-15
---

# Prometheus + Grafana for Metrics, PagerDuty for Alerts

## Decision

Use Prometheus for metrics collection, Grafana for visualization, PagerDuty for alerting.

## Reasoning

- Prometheus: cloud-native, pull-based, great Go integration
- Grafana: flexible dashboards, many data sources
- PagerDuty: industry standard incident management
- All have excellent open-source tiers

## Alternatives Considered

- Datadog: great UX but expensive, vendor lock-in
- New Relic: similar to Datadog, per-seat pricing
- CloudWatch: AWS-only, limited flexibility

## Tradeoffs

- Self-hosted operational burden
- Slower to set up than SaaS alternatives
- Need expertise for scaling Prometheus

## Revisit when

- We have dedicated SRE team
- Metric volume exceeds Prometheus capacity
- Need distributed tracing (then add Jaeger)
`,
		"10-notes/system-design-template.md": `---
id: note_dev_001
type: note
title: System Design Template
status: active
tags: [template, system-design]
created: 2026-01-20
updated: 2026-01-20
---

# [System Name]

## Context

[What problem does this system solve]

## Requirements

### Functional
- [requirement 1]
- [requirement 2]

### Non-Functional
- Scale: [X requests/sec]
- Latency: [P99 < Xms]
- Availability: [99.X%]

## Architecture

[High-level diagram description]

## Data Model

[Key entities and relationships]

## API Design

[Key endpoints]

## Tradeoffs

| Option | Pros | Cons | Decision |
|--------|------|------|----------|
| A | ... | ... | |
| B | ... | ... | Chosen |
`,
		"10-notes/runbook-template.md": `---
id: note_dev_002
type: note
title: Runbook Template
status: active
tags: [template, runbook, operations]
created: 2026-01-25
updated: 2026-01-25
---

# [Service Name] - [Incident Type] Runbook

## Symptoms

[What alerts fire, what users see]

## Impact

[Which customers, which features]

## Diagnosis Steps

1. [Check metric/dashboard]
2. [Check logs]
3. [Check dependent services]

## Resolution

### Quick Fix
[Immediate mitigation]

### Root Cause Fix
[Permanent fix]

## Prevention

- [Action item 1]
- [Action item 2]

## Escalation

- Primary on-call: [name/link]
- Secondary: [name/link]
- Engineering manager: [name/link]
`,
		"10-notes/incident-postmortem-template.md": `---
id: note_dev_003
type: note
title: Incident Postmortem Template
status: active
tags: [template, incident, postmortem]
created: 2026-02-01
updated: 2026-02-01
---

# INCIDENT-[ID] - [Title] - Postmortem

## Summary

| Field | Value |
|-------|-------|
| Date | YYYY-MM-DD |
| Duration | X minutes |
| Severity | SEV-1/2/3 |
| Impact | [description] |

## Timeline

| Time | Event |
|------|-------|
| HH:MM | Issue detected via [alert/monitor/user] |
| HH:MM | On-call paged |
| HH:MM | Mitigation deployed |
| HH:MM | Service fully recovered |

## Root Cause

[What happened and why]

## Lessons Learned

- [Lesson 1]
- [Lesson 2]

## Action Items

| # | Action | Owner | Due Date |
|---|--------|-------|----------|
| 1 | [action] | [owner] | [date] |
| 2 | [action] | [owner] | [date] |
`,
		"20-projects/platform-migration.md": `---
id: prj_dev_001
type: project
title: Cloud Provider Migration
status: active
tags: [project, migration, infrastructure]
created: 2026-03-01
updated: 2026-03-01
---

# Cloud Provider Migration

## Context

Migrate from current cloud provider to [new provider] for cost and feature reasons.

## Goals

- [ ] Zero-downtime migration
- [ ] Cost reduction of 30%+
- [ ] Improve latency for EU users

## Key Decisions

- See: [[Docker Compose for Dev, Kubernetes for Production]]

## Timeline

| Phase | Target | Status |
|-------|--------|--------|
| Planning | 2026-03 | In Progress |
| Dev Environment | 2026-04 | Not Started |
| Production Migration | 2026-05 | Not Started |
| Validation | 2026-06 | Not Started |

## Risks

- Data migration complexity
- DNS cutover timing
- Third-party integration compatibility
`,
		"70-prompts/code-review.md": `---
id: prompt_dev_001
type: prompt
title: Code Review Assistant
status: active
tags: [prompt, code-review, engineering]
created: 2026-01-15
updated: 2026-01-15
---

# Code Review Assistant

## Context

You are a senior engineer reviewing a pull request. Be thorough but constructive.

## Prompt

Review the following code changes. Provide:

1. Summary of what the code does
2. Any bugs or logic errors
3. Security concerns
4. Performance issues
5. Style and maintainability suggestions
6. Questions for the author

Focus on the most important issues first. Don't nitpick style unless it significantly impacts readability.
`,
		"70-prompts/debugging.md": `---
id: prompt_dev_002
type: prompt
title: Debugging Assistant
status: active
tags: [prompt, debugging, engineering]
created: 2026-01-20
updated: 2026-01-20
---

# Debugging Assistant

## Context

You are helping debug a production issue. Use the provided logs, metrics, and code context.

## Prompt

Given the following error, logs, and code context:

1. Identify the most likely root cause
2. Suggest 3 hypotheses ranked by probability
3. For each hypothesis, suggest a verification step
4. Recommend the quickest path to mitigation
5. Suggest monitoring to prevent recurrence

Focus on actionable next steps. Don't speculate beyond what the evidence supports.
`,
	},
}

package templates

var AgentMemoryTemplate = StarterTemplate{
	Name:        "agent-memory",
	Description: "AI agent memory system with entity tracking, conversation logs, and preference storage",
	Folders:     []string{},
	Files: map[string]string{
		"10-notes/memory-structure.md": `---
id: note_agent_001
type: note
title: Memory Structure Guide
status: active
tags: [agent, memory, architecture]
created: 2026-01-15
updated: 2026-01-15
---

# Memory Structure Guide

## Overview

This vault stores an AI agent's persistent memory. Each note represents a fact, preference, conversation, or relationship.

## Organization

- **Entities** (50-people/): People, places, organizations the agent knows about
- **Preferences**: User preferences learned over time
- **Conversations**: Log of significant interactions
- **Decisions**: Important choices and their reasoning
- **Tasks**: Ongoing commitments and follow-ups

## Writing Guidelines

1. Be specific and factual
2. Include context (when/where learned)
3. Update when information changes
4. Link related notes with wiki links
`,
		"50-people/user-profile-template.md": `---
id: person_agent_001
type: person
title: User Profile - [Name]
status: active
tags: [user, profile, entity]
created: 2026-01-20
updated: 2026-01-20
---

# User Profile - [Name]

## Basic Info

- Name: [full name]
- Role: [job title / relationship]
- Communication style: [formal / casual / technical]

## Preferences

- Response length: [concise / detailed]
- Technical depth: [high-level / detailed]
- Follow-up style: [proactive / on-request]

## Known Facts

- [Fact learned from conversation]
- [Important context]

## Conversation History

| Date | Topic | Key Points |
|------|-------|------------|
| 2026-01-20 | [topic] | [summary] |
`,
		"10-notes/conversation-log-template.md": `---
id: note_agent_002
type: note
title: Conversation Log - [Date] - [Topic]
status: active
tags: [conversation, log]
created: 2026-01-20
updated: 2026-01-20
---

# Conversation Log - [Date] - [Topic]

## Participants

- [names]

## Summary

[Brief summary of the conversation]

## Key Points

- [Point 1]
- [Point 2]

## Decisions Made

- [Decision 1 with reasoning]

## Action Items

- [ ] [Action item] - Owner: [name]

## Context

[Any additional context that would be useful for future reference]
`,
		"10-notes/preference-tracking.md": `---
id: note_agent_003
type: note
title: User Preference Tracking
status: active
tags: [preferences, user, tracking]
created: 2026-01-25
updated: 2026-01-25
---

# User Preference Tracking

## Format

Preferences are stored as structured notes with clear provenance.

## Template

~~~
Preference: [what the user prefers]
Category: [communication / output / timing / tools]
Confidence: [high / medium / low]
Source: [which conversation / observation]
Date learned: [YYYY-MM-DD]
Exceptions: [any known exceptions]
~~~

## Active Preferences

- Prefers bullet points over paragraphs (high confidence, from multiple conversations)
- Wants code examples in explanations (medium confidence, observed 2026-01-20)
- Dislikes being asked clarifying questions (low confidence, single observation)
`,
		"30-decisions/memory-retrieval-strategy.md": `---
id: dec_agent_001
type: decision
title: Use Hybrid Retrieval - FTS + Vector Search
status: active
confidence: medium
project: agent-memory
tags: [ai, retrieval, memory]
created: 2026-02-01
updated: 2026-02-01
review_after: 2026-05-01
---

# Use Hybrid Retrieval - FTS + Vector Search

## Decision

Combine SQLite FTS5 for keyword search with vector embeddings for semantic search.

## Reasoning

- FTS5: exact match, fast, no ML dependency
- Vector: semantic similarity, handles paraphrasing
- Hybrid: best of both, configurable weights

## Implementation

1. Store embeddings in chunks table
2. Use cosine similarity for vector search
3. Rank combined: w1 * FTS_score + w2 * vector_score

## Tradeoffs

- Extra storage for embeddings
- Slower indexing (need to compute embeddings)
- Requires embedding model

## Revisit when

- Embedding quality is insufficient
- Query latency exceeds 100ms
- New retrieval methods emerge
`,
		"20-projects/agent-memory-system.md": `---
id: prj_agent_001
type: project
title: Agent Memory System v1
status: active
tags: [project, agent, memory, ai]
created: 2026-01-10
updated: 2026-01-10
---

# Agent Memory System v1

## Goals

- Persistent memory across conversations
- Fast retrieval of relevant context
- Graceful forgetting of old/irrelevant info
- Source attribution for all recalled facts

## Architecture

~~~
User Query
    |
    v
[Query Analyzer] --> Intent + Entities
    |
    v
[Retrieval Engine]
    |-- FTS5 (keywords)
    |-- Vector (semantic)
    |-- Entity (relationships)
    |
    v
[Context Builder] --> Ranked context window
    |
    v
[LLM] --> Response with citations
~~~

## Key Decisions

- See: [[Memory Retrieval Strategy]]
`,
		"70-prompts/memory-search.md": `---
id: prompt_agent_001
type: prompt
title: Memory Search Prompt
status: active
tags: [prompt, memory, retrieval]
created: 2026-01-15
updated: 2026-01-15
---

# Memory Search Prompt

## Context

You are an AI assistant with access to a personal knowledge base. Answer using ONLY the provided sources.

## Prompt

Sources:
[{{sources}}]

Question: {{question}}

Instructions:
1. Answer using only the sources above
2. Cite specific sources by their IDs
3. If sources are insufficient, say "I don't have enough information about that"
4. Be concise and direct
5. If multiple sources conflict, note the discrepancy
`,
	},
}

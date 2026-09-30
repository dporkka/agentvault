---
id: {{.ID}}
type: agent
title: {{.Title}}
status: active
revision: 1
created: {{.Created}}
updated: {{.Created}}
{{if .Tags}}tags: [{{join .Tags ", "}}]
{{end}}
identity_ref: ""
memory_refs: []
knowledge_scopes: []
artifact_scopes: []
conversation_scopes: []
capability_refs: []
context_policy_ref: ""
---

# {{.Title}}

## Purpose

Describe what this agent is responsible for and what is explicitly out of scope.

## Identity

Define stable role, behavioral constraints, and operating principles.

## Memory

Reference durable memories that should influence future behavior. Keep raw execution history separate.

## Knowledge

List the knowledge scopes this agent may retrieve from.

## Capabilities

List capability or permission references. AgentVault records grants and snapshots; the runtime executes tools.

## Context policy

Describe which identity, memory, knowledge, conversation, and artifact references are compiled into context.

## Notes

This file is the canonical agent manifest. Execution evidence, evaluations, and promotion lineage are stored separately and may reference this agent ID.

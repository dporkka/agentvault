---
id: dec_2026_05_29_002
type: decision
title: Use SQLite for Local Storage
status: active
project: agentvault
created: 2026-05-29T11:00:00Z
updated: 2026-05-29T11:00:00Z
decided_by: team
---

# Use SQLite for Local Storage

## Decision

Use SQLite with FTS5 for full-text search instead of an external search engine.

## Reasoning

- Zero external dependencies for end users
- Single file database (easy to backup and sync)
- FTS5 provides excellent full-text search capabilities
- Works well with Go via modernc.org/sqlite

## Tradeoffs

- Not distributed (acceptable for local-first)
- Single-writer at a time (acceptable for CLI usage)

## Revisit when

- Multi-user sync becomes a requirement
- Search performance degrades beyond acceptable thresholds

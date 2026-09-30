-- Higher-order memory synthesis lineage.
-- Migration 008: summaries, mental models, policies, and skills remain normal
-- durable memories while retaining explicit source-memory lineage.

CREATE TABLE IF NOT EXISTS memory_syntheses (
  id TEXT PRIMARY KEY,
  kind TEXT NOT NULL CHECK (kind IN ('summary', 'model', 'policy', 'skill')),
  scope_type TEXT NOT NULL,
  scope_id TEXT NOT NULL,
  target_memory_id TEXT NOT NULL UNIQUE,
  provenance_id TEXT,
  created_by TEXT,
  rationale TEXT,
  created_at TEXT NOT NULL,
  FOREIGN KEY(target_memory_id) REFERENCES memory_records(id) ON DELETE RESTRICT,
  FOREIGN KEY(provenance_id) REFERENCES provenance_records(id)
);

CREATE TABLE IF NOT EXISTS memory_synthesis_sources (
  synthesis_id TEXT NOT NULL,
  source_memory_id TEXT NOT NULL,
  source_order INTEGER NOT NULL,
  PRIMARY KEY(synthesis_id, source_memory_id),
  FOREIGN KEY(synthesis_id) REFERENCES memory_syntheses(id) ON DELETE CASCADE,
  FOREIGN KEY(source_memory_id) REFERENCES memory_records(id) ON DELETE RESTRICT
);

CREATE INDEX IF NOT EXISTS idx_memory_syntheses_scope
  ON memory_syntheses(scope_type, scope_id, created_at DESC);
CREATE INDEX IF NOT EXISTS idx_memory_syntheses_kind
  ON memory_syntheses(kind, created_at DESC);
CREATE INDEX IF NOT EXISTS idx_memory_synthesis_sources_memory
  ON memory_synthesis_sources(source_memory_id);

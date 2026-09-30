-- Reviewable semantic-memory candidates.
-- Migration 007: extraction may propose memories, but only explicit review
-- transitions can materialize durable semantic memory.

CREATE TABLE IF NOT EXISTS memory_candidates (
  id TEXT PRIMARY KEY,
  source_episode_id TEXT NOT NULL,
  memory_kind TEXT NOT NULL CHECK (memory_kind IN (
    'observation', 'fact', 'preference', 'decision', 'constraint', 'summary'
  )),
  scope_type TEXT NOT NULL,
  scope_id TEXT NOT NULL,
  content TEXT NOT NULL,
  object_id TEXT,
  provenance_id TEXT NOT NULL,
  confidence REAL NOT NULL DEFAULT 1.0 CHECK (confidence >= 0.0 AND confidence <= 1.0),
  status TEXT NOT NULL CHECK (status IN (
    'pending', 'accepted', 'rejected', 'merged', 'superseded'
  )),
  proposed_by TEXT,
  reviewed_by TEXT,
  review_reason TEXT,
  result_memory_id TEXT,
  target_memory_id TEXT,
  metadata_json TEXT NOT NULL DEFAULT '{}',
  created_at TEXT NOT NULL,
  updated_at TEXT NOT NULL,
  reviewed_at TEXT,
  FOREIGN KEY(source_episode_id) REFERENCES episodes(id) ON DELETE CASCADE,
  FOREIGN KEY(object_id) REFERENCES objects(id) ON DELETE SET NULL,
  FOREIGN KEY(provenance_id) REFERENCES provenance_records(id),
  FOREIGN KEY(result_memory_id) REFERENCES memory_records(id) ON DELETE SET NULL,
  FOREIGN KEY(target_memory_id) REFERENCES memory_records(id) ON DELETE SET NULL
);

CREATE INDEX IF NOT EXISTS idx_memory_candidates_status
  ON memory_candidates(status, updated_at DESC);
CREATE INDEX IF NOT EXISTS idx_memory_candidates_scope
  ON memory_candidates(scope_type, scope_id, updated_at DESC);
CREATE INDEX IF NOT EXISTS idx_memory_candidates_episode
  ON memory_candidates(source_episode_id);
CREATE INDEX IF NOT EXISTS idx_memory_candidates_kind
  ON memory_candidates(memory_kind, updated_at DESC);

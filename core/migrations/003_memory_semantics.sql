-- AgentVault: scoped memory semantics
-- Migration 003: additive metadata used by the memory projection/retrieval layer.
-- Markdown remains canonical; these columns are an indexed projection.

ALTER TABLE notes ADD COLUMN workspace_id TEXT NOT NULL DEFAULT '';
ALTER TABLE notes ADD COLUMN agent_id TEXT NOT NULL DEFAULT '';
ALTER TABLE notes ADD COLUMN session_id TEXT NOT NULL DEFAULT '';
ALTER TABLE notes ADD COLUMN memory_kind TEXT NOT NULL DEFAULT '';
ALTER TABLE notes ADD COLUMN confidence REAL CHECK(confidence IS NULL OR (confidence >= 0.0 AND confidence <= 1.0));
ALTER TABLE notes ADD COLUMN provenance_json TEXT;
ALTER TABLE notes ADD COLUMN observed_at TEXT;
ALTER TABLE notes ADD COLUMN valid_from TEXT;
ALTER TABLE notes ADD COLUMN valid_to TEXT;

CREATE TABLE memory_supersessions (
  superseding_note_id TEXT NOT NULL,
  superseded_note_id TEXT NOT NULL,
  reason TEXT,
  created_at TEXT NOT NULL DEFAULT (datetime('now')),
  PRIMARY KEY (superseding_note_id, superseded_note_id)
);

CREATE INDEX idx_notes_memory_scope
  ON notes(workspace_id, agent_id, session_id);
CREATE INDEX idx_notes_memory_kind
  ON notes(memory_kind);
CREATE INDEX idx_notes_memory_confidence
  ON notes(confidence);
CREATE INDEX idx_notes_memory_validity
  ON notes(valid_from, valid_to);
CREATE INDEX idx_memory_supersessions_superseded
  ON memory_supersessions(superseded_note_id);

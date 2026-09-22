-- Temporal knowledge primitives for AgentVault.
-- Migration 006: first-class episodes and bi-temporal facts.
--
-- Occurrence/validity time is stored on the record itself. Observation time is
-- carried independently by provenance_records.observed_at, which lets callers
-- distinguish "when this was true/happened" from "when AgentVault learned it".

CREATE TABLE IF NOT EXISTS episodes (
  id TEXT PRIMARY KEY,
  scope_type TEXT NOT NULL,
  scope_id TEXT NOT NULL,
  event_type TEXT NOT NULL,
  summary TEXT NOT NULL,
  object_ids_json TEXT NOT NULL DEFAULT '[]',
  provenance_id TEXT,
  occurred_at TEXT NOT NULL,
  ended_at TEXT,
  metadata_json TEXT NOT NULL DEFAULT '{}',
  created_at TEXT NOT NULL,
  FOREIGN KEY(provenance_id) REFERENCES provenance_records(id)
);

CREATE INDEX IF NOT EXISTS idx_episodes_scope
  ON episodes(scope_type, scope_id, occurred_at DESC);
CREATE INDEX IF NOT EXISTS idx_episodes_type
  ON episodes(event_type, occurred_at DESC);
CREATE INDEX IF NOT EXISTS idx_episodes_provenance
  ON episodes(provenance_id);

CREATE TABLE IF NOT EXISTS temporal_facts (
  id TEXT PRIMARY KEY,
  subject_id TEXT NOT NULL,
  predicate TEXT NOT NULL,
  object_id TEXT,
  value TEXT,
  provenance_id TEXT,
  confidence REAL NOT NULL DEFAULT 1.0
    CHECK (confidence >= 0.0 AND confidence <= 1.0),
  valid_from TEXT,
  valid_to TEXT,
  supersedes_id TEXT,
  superseded_at TEXT,
  superseded_by TEXT,
  metadata_json TEXT NOT NULL DEFAULT '{}',
  created_at TEXT NOT NULL,
  updated_at TEXT NOT NULL,
  FOREIGN KEY(subject_id) REFERENCES objects(id) ON DELETE CASCADE,
  FOREIGN KEY(object_id) REFERENCES objects(id) ON DELETE SET NULL,
  FOREIGN KEY(provenance_id) REFERENCES provenance_records(id),
  FOREIGN KEY(supersedes_id) REFERENCES temporal_facts(id) ON DELETE SET NULL,
  FOREIGN KEY(superseded_by) REFERENCES temporal_facts(id) ON DELETE SET NULL,
  CHECK (object_id IS NOT NULL OR (value IS NOT NULL AND length(trim(value)) > 0))
);

CREATE INDEX IF NOT EXISTS idx_temporal_facts_subject
  ON temporal_facts(subject_id, predicate);
CREATE INDEX IF NOT EXISTS idx_temporal_facts_object
  ON temporal_facts(object_id);
CREATE INDEX IF NOT EXISTS idx_temporal_facts_validity
  ON temporal_facts(valid_from, valid_to);
CREATE INDEX IF NOT EXISTS idx_temporal_facts_supersession
  ON temporal_facts(superseded_by, superseded_at);

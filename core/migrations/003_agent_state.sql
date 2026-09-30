-- AgentVault: durable agent state, structured execution evidence, evaluations, and promotion lineage
-- Migration 003 keeps Markdown notes canonical while SQLite stores audit/index state.

ALTER TABLE agent_runs ADD COLUMN agent_id TEXT;
ALTER TABLE agent_runs ADD COLUMN agent_revision INTEGER;
ALTER TABLE agent_runs ADD COLUMN conversation_id TEXT;
ALTER TABLE agent_runs ADD COLUMN status TEXT NOT NULL DEFAULT 'succeeded';
ALTER TABLE agent_runs ADD COLUMN started_at TEXT;
ALTER TABLE agent_runs ADD COLUMN ended_at TEXT;
ALTER TABLE agent_runs ADD COLUMN context_hash TEXT;
ALTER TABLE agent_runs ADD COLUMN capability_snapshot_json TEXT;
ALTER TABLE agent_runs ADD COLUMN runtime_metadata_json TEXT;

CREATE INDEX IF NOT EXISTS idx_agent_runs_agent ON agent_runs(agent_id, created_at);
CREATE INDEX IF NOT EXISTS idx_agent_runs_conversation ON agent_runs(conversation_id, created_at);

CREATE TABLE IF NOT EXISTS run_observations (
  id TEXT PRIMARY KEY,
  run_id TEXT NOT NULL,
  parent_observation_id TEXT,
  kind TEXT NOT NULL,
  name TEXT NOT NULL,
  status TEXT,
  input_json TEXT,
  output_json TEXT,
  evidence_json TEXT,
  started_at TEXT,
  ended_at TEXT,
  created_at TEXT NOT NULL,
  FOREIGN KEY(run_id) REFERENCES agent_runs(id) ON DELETE CASCADE,
  FOREIGN KEY(parent_observation_id) REFERENCES run_observations(id) ON DELETE SET NULL
);

CREATE INDEX IF NOT EXISTS idx_run_observations_run ON run_observations(run_id, created_at);
CREATE INDEX IF NOT EXISTS idx_run_observations_kind ON run_observations(kind, created_at);

CREATE TABLE IF NOT EXISTS evaluations (
  id TEXT PRIMARY KEY,
  run_id TEXT NOT NULL,
  observation_id TEXT,
  evaluator TEXT NOT NULL,
  name TEXT NOT NULL,
  score REAL,
  label TEXT,
  rationale TEXT,
  metadata_json TEXT,
  created_at TEXT NOT NULL,
  FOREIGN KEY(run_id) REFERENCES agent_runs(id) ON DELETE CASCADE,
  FOREIGN KEY(observation_id) REFERENCES run_observations(id) ON DELETE CASCADE
);

CREATE INDEX IF NOT EXISTS idx_evaluations_run ON evaluations(run_id, created_at);
CREATE INDEX IF NOT EXISTS idx_evaluations_observation ON evaluations(observation_id, created_at);

CREATE TABLE IF NOT EXISTS promotion_records (
  id TEXT PRIMARY KEY,
  agent_id TEXT NOT NULL,
  target_kind TEXT NOT NULL CHECK(target_kind IN ('memory', 'knowledge')),
  status TEXT NOT NULL CHECK(status IN ('proposed', 'approved', 'rejected', 'committed', 'superseded')),
  candidate TEXT NOT NULL,
  rationale TEXT,
  source_run_ids_json TEXT NOT NULL DEFAULT '[]',
  source_observation_ids_json TEXT NOT NULL DEFAULT '[]',
  source_evaluation_ids_json TEXT NOT NULL DEFAULT '[]',
  target_note_id TEXT,
  supersedes_note_id TEXT,
  created_at TEXT NOT NULL,
  reviewed_at TEXT,
  committed_at TEXT
);

CREATE INDEX IF NOT EXISTS idx_promotion_records_agent ON promotion_records(agent_id, created_at);
CREATE INDEX IF NOT EXISTS idx_promotion_records_status ON promotion_records(status, created_at);

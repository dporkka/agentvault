-- AgentVault: explicit promotion review plus reproducible evaluation datasets and experiments.

ALTER TABLE promotion_records ADD COLUMN reviewed_by TEXT;
ALTER TABLE promotion_records ADD COLUMN review_note TEXT;

CREATE TABLE IF NOT EXISTS evaluation_datasets (
  id TEXT PRIMARY KEY,
  name TEXT NOT NULL,
  description TEXT,
  agent_id TEXT,
  created_at TEXT NOT NULL
);

CREATE TABLE IF NOT EXISTS evaluation_cases (
  id TEXT PRIMARY KEY,
  dataset_id TEXT NOT NULL,
  name TEXT NOT NULL,
  input_json TEXT NOT NULL,
  expected_json TEXT,
  tags_json TEXT NOT NULL DEFAULT '[]',
  created_at TEXT NOT NULL,
  FOREIGN KEY(dataset_id) REFERENCES evaluation_datasets(id) ON DELETE CASCADE
);

CREATE TABLE IF NOT EXISTS experiments (
  id TEXT PRIMARY KEY,
  dataset_id TEXT NOT NULL,
  name TEXT NOT NULL,
  agent_id TEXT NOT NULL,
  agent_revision INTEGER NOT NULL,
  status TEXT NOT NULL CHECK(status IN ('planned', 'running', 'completed', 'failed', 'cancelled')),
  config_json TEXT NOT NULL DEFAULT '{}',
  created_at TEXT NOT NULL,
  completed_at TEXT,
  FOREIGN KEY(dataset_id) REFERENCES evaluation_datasets(id) ON DELETE CASCADE
);

CREATE TABLE IF NOT EXISTS experiment_results (
  experiment_id TEXT NOT NULL,
  case_id TEXT NOT NULL,
  run_id TEXT,
  score REAL,
  label TEXT,
  metadata_json TEXT NOT NULL DEFAULT '{}',
  created_at TEXT NOT NULL,
  PRIMARY KEY(experiment_id, case_id),
  FOREIGN KEY(experiment_id) REFERENCES experiments(id) ON DELETE CASCADE,
  FOREIGN KEY(case_id) REFERENCES evaluation_cases(id) ON DELETE CASCADE,
  FOREIGN KEY(run_id) REFERENCES agent_runs(id) ON DELETE SET NULL
);

CREATE INDEX IF NOT EXISTS idx_evaluation_cases_dataset ON evaluation_cases(dataset_id);
CREATE INDEX IF NOT EXISTS idx_experiments_dataset ON experiments(dataset_id, created_at);
CREATE INDEX IF NOT EXISTS idx_experiments_agent ON experiments(agent_id, agent_revision, created_at);
CREATE INDEX IF NOT EXISTS idx_experiment_results_run ON experiment_results(run_id);

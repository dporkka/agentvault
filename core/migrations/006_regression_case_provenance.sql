-- Preserve production-run provenance when a failure is captured as a regression case.

ALTER TABLE evaluation_cases ADD COLUMN source_run_id TEXT;
ALTER TABLE evaluation_cases ADD COLUMN source_observation_ids_json TEXT NOT NULL DEFAULT '[]';
ALTER TABLE evaluation_cases ADD COLUMN source_evaluation_ids_json TEXT NOT NULL DEFAULT '[]';
ALTER TABLE evaluation_cases ADD COLUMN agent_id TEXT;
ALTER TABLE evaluation_cases ADD COLUMN agent_revision INTEGER;

CREATE INDEX IF NOT EXISTS idx_evaluation_cases_source_run
  ON evaluation_cases(source_run_id);

CREATE UNIQUE INDEX IF NOT EXISTS idx_evaluation_cases_dataset_source_run
  ON evaluation_cases(dataset_id, source_run_id)
  WHERE source_run_id IS NOT NULL AND source_run_id != '';

-- AgentVault: immutable pre-dispatch action intents and terminal receipts.
--
-- External runtimes execute tools/actions. AgentVault records evidence that
-- binds one logical operation to its run, capability snapshot, input digest,
-- and exactly one terminal outcome.

CREATE TABLE IF NOT EXISTS action_intents (
  id TEXT PRIMARY KEY,
  run_id TEXT NOT NULL,
  observation_id TEXT,
  operation_id TEXT NOT NULL,
  action TEXT NOT NULL,
  capability_ref TEXT NOT NULL,
  authority_hash TEXT NOT NULL,
  input_hash TEXT NOT NULL,
  metadata_json TEXT NOT NULL DEFAULT '{}',
  created_at TEXT NOT NULL,
  UNIQUE(run_id, operation_id),
  FOREIGN KEY(run_id) REFERENCES agent_runs(id) ON DELETE RESTRICT,
  FOREIGN KEY(observation_id) REFERENCES run_observations(id) ON DELETE RESTRICT
);

CREATE TABLE IF NOT EXISTS action_receipts (
  id TEXT PRIMARY KEY,
  intent_id TEXT NOT NULL UNIQUE,
  status TEXT NOT NULL CHECK(status IN ('completed', 'failed', 'indeterminate')),
  result_hash TEXT,
  external_receipt_id TEXT,
  error_code TEXT,
  error_message TEXT,
  metadata_json TEXT NOT NULL DEFAULT '{}',
  completed_at TEXT NOT NULL,
  FOREIGN KEY(intent_id) REFERENCES action_intents(id) ON DELETE RESTRICT,
  CHECK(
    (status = 'completed' AND result_hash IS NOT NULL AND result_hash <> '')
    OR
    (status IN ('failed', 'indeterminate') AND (
      (error_code IS NOT NULL AND error_code <> '')
      OR (error_message IS NOT NULL AND error_message <> '')
    ))
  )
);

CREATE INDEX IF NOT EXISTS idx_action_intents_run
  ON action_intents(run_id, created_at);
CREATE INDEX IF NOT EXISTS idx_action_intents_action
  ON action_intents(action, created_at);
CREATE INDEX IF NOT EXISTS idx_action_receipts_status
  ON action_receipts(status, completed_at);

CREATE TRIGGER IF NOT EXISTS action_intents_no_update
BEFORE UPDATE ON action_intents
BEGIN
  SELECT RAISE(ABORT, 'action intents are immutable');
END;

CREATE TRIGGER IF NOT EXISTS action_intents_no_delete
BEFORE DELETE ON action_intents
BEGIN
  SELECT RAISE(ABORT, 'action intents are immutable');
END;

CREATE TRIGGER IF NOT EXISTS action_receipts_no_update
BEFORE UPDATE ON action_receipts
BEGIN
  SELECT RAISE(ABORT, 'action receipts are immutable');
END;

CREATE TRIGGER IF NOT EXISTS action_receipts_no_delete
BEFORE DELETE ON action_receipts
BEGIN
  SELECT RAISE(ABORT, 'action receipts are immutable');
END;

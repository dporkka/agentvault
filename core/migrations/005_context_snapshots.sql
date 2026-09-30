-- AgentVault: immutable compiled context snapshots for run reproducibility.

CREATE TABLE IF NOT EXISTS context_snapshots (
  hash TEXT PRIMARY KEY,
  agent_id TEXT NOT NULL,
  agent_revision INTEGER NOT NULL,
  task TEXT,
  conversation_id TEXT,
  context_json TEXT NOT NULL,
  created_at TEXT NOT NULL
);

CREATE INDEX IF NOT EXISTS idx_context_snapshots_agent
  ON context_snapshots(agent_id, agent_revision, created_at);

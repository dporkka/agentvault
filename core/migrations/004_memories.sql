-- AgentVault: typed memory candidates and durable promotion
-- Migration 004: memory state derived from immutable event evidence

CREATE TABLE IF NOT EXISTS memories (
  id TEXT PRIMARY KEY,
  agent_id TEXT NOT NULL,
  kind TEXT NOT NULL CHECK(kind IN ('episodic', 'semantic', 'procedural')),
  status TEXT NOT NULL DEFAULT 'candidate'
    CHECK(status IN ('candidate', 'durable', 'rejected', 'superseded')),
  content TEXT NOT NULL,
  confidence REAL NOT NULL CHECK(confidence >= 0.0 AND confidence <= 1.0),
  salience REAL NOT NULL CHECK(salience >= 0.0 AND salience <= 1.0),
  scope_type TEXT,
  scope_id TEXT,
  source_event_id TEXT NOT NULL,
  metadata_json TEXT NOT NULL DEFAULT '{}',
  created_at TEXT NOT NULL,
  updated_at TEXT NOT NULL,
  promoted_at TEXT,
  FOREIGN KEY(agent_id) REFERENCES agents(id),
  FOREIGN KEY(source_event_id) REFERENCES events(id)
);

CREATE INDEX IF NOT EXISTS idx_memories_agent_status
  ON memories(agent_id, status, updated_at);
CREATE INDEX IF NOT EXISTS idx_memories_scope
  ON memories(scope_type, scope_id, status, updated_at);
CREATE INDEX IF NOT EXISTS idx_memories_kind_status
  ON memories(kind, status, salience DESC, confidence DESC);
CREATE INDEX IF NOT EXISTS idx_memories_source_event
  ON memories(source_event_id);

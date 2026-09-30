-- AgentVault: persistent agents and append-only provenance events
-- Migration 003: canonical agent identity + immutable event substrate

CREATE TABLE IF NOT EXISTS agents (
  id TEXT PRIMARY KEY,
  name TEXT NOT NULL UNIQUE COLLATE NOCASE,
  display_name TEXT NOT NULL,
  description TEXT,
  instructions TEXT,
  status TEXT NOT NULL DEFAULT 'active'
    CHECK(status IN ('active', 'disabled', 'archived')),
  permissions_json TEXT NOT NULL DEFAULT '[]',
  metadata_json TEXT NOT NULL DEFAULT '{}',
  created_at TEXT NOT NULL,
  updated_at TEXT NOT NULL
);

CREATE INDEX IF NOT EXISTS idx_agents_status ON agents(status);
CREATE INDEX IF NOT EXISTS idx_agents_name ON agents(name COLLATE NOCASE);

ALTER TABLE agent_runs ADD COLUMN agent_id TEXT REFERENCES agents(id);
CREATE INDEX IF NOT EXISTS idx_agent_runs_agent ON agent_runs(agent_id);

CREATE TABLE IF NOT EXISTS events (
  id TEXT PRIMARY KEY,
  event_type TEXT NOT NULL,
  actor_type TEXT,
  actor_id TEXT,
  subject_type TEXT,
  subject_id TEXT,
  scope_type TEXT,
  scope_id TEXT,
  run_id TEXT,
  conversation_id TEXT,
  source_id TEXT,
  parent_event_id TEXT,
  payload_json TEXT NOT NULL DEFAULT '{}',
  metadata_json TEXT NOT NULL DEFAULT '{}',
  occurred_at TEXT NOT NULL,
  recorded_at TEXT NOT NULL,
  FOREIGN KEY(run_id) REFERENCES agent_runs(id),
  FOREIGN KEY(conversation_id) REFERENCES conversations(id) ON DELETE SET NULL,
  FOREIGN KEY(parent_event_id) REFERENCES events(id)
);

CREATE INDEX IF NOT EXISTS idx_events_type_time ON events(event_type, occurred_at);
CREATE INDEX IF NOT EXISTS idx_events_actor ON events(actor_type, actor_id, occurred_at);
CREATE INDEX IF NOT EXISTS idx_events_subject ON events(subject_type, subject_id, occurred_at);
CREATE INDEX IF NOT EXISTS idx_events_scope ON events(scope_type, scope_id, occurred_at);
CREATE INDEX IF NOT EXISTS idx_events_run ON events(run_id);
CREATE INDEX IF NOT EXISTS idx_events_conversation ON events(conversation_id);
CREATE INDEX IF NOT EXISTS idx_events_parent ON events(parent_event_id);

-- Events are evidence. Corrections and supersession are represented by new
-- events; existing rows are never mutated or deleted.
CREATE TRIGGER IF NOT EXISTS events_prevent_update
BEFORE UPDATE ON events
BEGIN
  SELECT RAISE(ABORT, 'events are append-only');
END;

CREATE TRIGGER IF NOT EXISTS events_prevent_delete
BEFORE DELETE ON events
BEGIN
  SELECT RAISE(ABORT, 'events are append-only');
END;

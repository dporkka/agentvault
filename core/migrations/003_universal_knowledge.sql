-- Universal knowledge primitives for AgentVault.
-- Markdown/YAML remains canonical user-authored content; these tables hold
-- derived knowledge, durable agent state, provenance, and integration data.

CREATE TABLE IF NOT EXISTS provenance_records (
  id TEXT PRIMARY KEY,
  source_type TEXT NOT NULL,
  source_id TEXT,
  agent_id TEXT,
  session_id TEXT,
  model TEXT,
  confidence REAL NOT NULL DEFAULT 1.0 CHECK (confidence >= 0.0 AND confidence <= 1.0),
  observed_at TEXT NOT NULL,
  evidence_json TEXT,
  metadata_json TEXT,
  created_at TEXT NOT NULL
);

CREATE TABLE IF NOT EXISTS objects (
  id TEXT PRIMARY KEY,
  type TEXT NOT NULL,
  title TEXT NOT NULL,
  status TEXT,
  organization TEXT,
  project TEXT,
  canonical_path TEXT,
  data_json TEXT NOT NULL DEFAULT '{}',
  provenance_id TEXT,
  created_at TEXT NOT NULL,
  updated_at TEXT NOT NULL,
  FOREIGN KEY(provenance_id) REFERENCES provenance_records(id)
);

CREATE UNIQUE INDEX IF NOT EXISTS idx_objects_canonical_path
  ON objects(canonical_path) WHERE canonical_path IS NOT NULL;
CREATE INDEX IF NOT EXISTS idx_objects_type ON objects(type);
CREATE INDEX IF NOT EXISTS idx_objects_project ON objects(project);
CREATE INDEX IF NOT EXISTS idx_objects_organization ON objects(organization);
CREATE INDEX IF NOT EXISTS idx_objects_status ON objects(status);

CREATE TABLE IF NOT EXISTS object_relations (
  id TEXT PRIMARY KEY,
  from_object_id TEXT NOT NULL,
  to_object_id TEXT NOT NULL,
  relation_type TEXT NOT NULL,
  valid_from TEXT,
  valid_to TEXT,
  confidence REAL NOT NULL DEFAULT 1.0 CHECK (confidence >= 0.0 AND confidence <= 1.0),
  provenance_id TEXT,
  metadata_json TEXT NOT NULL DEFAULT '{}',
  created_at TEXT NOT NULL,
  updated_at TEXT NOT NULL,
  FOREIGN KEY(from_object_id) REFERENCES objects(id) ON DELETE CASCADE,
  FOREIGN KEY(to_object_id) REFERENCES objects(id) ON DELETE CASCADE,
  FOREIGN KEY(provenance_id) REFERENCES provenance_records(id)
);

CREATE INDEX IF NOT EXISTS idx_object_relations_from ON object_relations(from_object_id);
CREATE INDEX IF NOT EXISTS idx_object_relations_to ON object_relations(to_object_id);
CREATE INDEX IF NOT EXISTS idx_object_relations_type ON object_relations(relation_type);
CREATE INDEX IF NOT EXISTS idx_object_relations_validity ON object_relations(valid_from, valid_to);

CREATE TABLE IF NOT EXISTS memory_records (
  id TEXT PRIMARY KEY,
  memory_type TEXT NOT NULL CHECK (memory_type IN ('working', 'episodic', 'semantic', 'procedural')),
  scope_type TEXT NOT NULL,
  scope_id TEXT NOT NULL,
  content TEXT NOT NULL,
  object_id TEXT,
  provenance_id TEXT,
  confidence REAL NOT NULL DEFAULT 1.0 CHECK (confidence >= 0.0 AND confidence <= 1.0),
  valid_from TEXT,
  valid_to TEXT,
  supersedes_id TEXT,
  metadata_json TEXT NOT NULL DEFAULT '{}',
  created_at TEXT NOT NULL,
  updated_at TEXT NOT NULL,
  FOREIGN KEY(object_id) REFERENCES objects(id) ON DELETE SET NULL,
  FOREIGN KEY(provenance_id) REFERENCES provenance_records(id),
  FOREIGN KEY(supersedes_id) REFERENCES memory_records(id) ON DELETE SET NULL
);

CREATE INDEX IF NOT EXISTS idx_memory_scope ON memory_records(scope_type, scope_id);
CREATE INDEX IF NOT EXISTS idx_memory_type ON memory_records(memory_type);
CREATE INDEX IF NOT EXISTS idx_memory_object ON memory_records(object_id);
CREATE INDEX IF NOT EXISTS idx_memory_validity ON memory_records(valid_from, valid_to);

CREATE TABLE IF NOT EXISTS agent_sessions (
  id TEXT PRIMARY KEY,
  agent_id TEXT NOT NULL,
  project TEXT,
  objective TEXT NOT NULL,
  status TEXT NOT NULL DEFAULT 'active',
  branch TEXT,
  worktree TEXT,
  context_json TEXT NOT NULL DEFAULT '{}',
  started_at TEXT NOT NULL,
  updated_at TEXT NOT NULL,
  ended_at TEXT
);

CREATE INDEX IF NOT EXISTS idx_agent_sessions_agent ON agent_sessions(agent_id);
CREATE INDEX IF NOT EXISTS idx_agent_sessions_project ON agent_sessions(project);
CREATE INDEX IF NOT EXISTS idx_agent_sessions_status ON agent_sessions(status);

CREATE TABLE IF NOT EXISTS session_events (
  id TEXT PRIMARY KEY,
  session_id TEXT NOT NULL,
  event_type TEXT NOT NULL,
  payload_json TEXT NOT NULL DEFAULT '{}',
  provenance_id TEXT,
  created_at TEXT NOT NULL,
  FOREIGN KEY(session_id) REFERENCES agent_sessions(id) ON DELETE CASCADE,
  FOREIGN KEY(provenance_id) REFERENCES provenance_records(id)
);

CREATE INDEX IF NOT EXISTS idx_session_events_session ON session_events(session_id, created_at);
CREATE INDEX IF NOT EXISTS idx_session_events_type ON session_events(event_type);

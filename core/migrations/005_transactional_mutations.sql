-- Transactional agent mutation projection.
-- Canonical mutation history remains in 80-agent-runs/knowledge.journal.jsonl;
-- this table is a rebuildable projection of proposal state.

CREATE TABLE IF NOT EXISTS mutation_proposals (
  id TEXT PRIMARY KEY,
  mutation_kind TEXT NOT NULL CHECK (mutation_kind IN ('create', 'replace', 'delete')),
  path TEXT NOT NULL,
  reason TEXT NOT NULL,
  agent_id TEXT,
  session_id TEXT,
  provenance_id TEXT,
  status TEXT NOT NULL CHECK (status IN (
    'proposed', 'approved', 'committing', 'committed', 'undoing', 'undone', 'conflicted', 'rejected'
  )),
  before_exists INTEGER NOT NULL DEFAULT 0 CHECK (before_exists IN (0, 1)),
  after_exists INTEGER NOT NULL DEFAULT 0 CHECK (after_exists IN (0, 1)),
  before_hash TEXT,
  after_hash TEXT,
  before_content TEXT,
  after_content TEXT,
  diff TEXT NOT NULL DEFAULT '',
  approved_by TEXT,
  last_error TEXT,
  created_at TEXT NOT NULL,
  updated_at TEXT NOT NULL,
  approved_at TEXT,
  committed_at TEXT,
  undone_at TEXT,
  FOREIGN KEY(session_id) REFERENCES agent_sessions(id) ON DELETE SET NULL,
  FOREIGN KEY(provenance_id) REFERENCES provenance_records(id) ON DELETE SET NULL
);

CREATE INDEX IF NOT EXISTS idx_mutation_proposals_status
  ON mutation_proposals(status, updated_at DESC);
CREATE INDEX IF NOT EXISTS idx_mutation_proposals_session
  ON mutation_proposals(session_id, updated_at DESC);
CREATE INDEX IF NOT EXISTS idx_mutation_proposals_agent
  ON mutation_proposals(agent_id, updated_at DESC);
CREATE INDEX IF NOT EXISTS idx_mutation_proposals_path
  ON mutation_proposals(path, updated_at DESC);

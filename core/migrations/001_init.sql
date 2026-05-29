-- AgentVault initial schema
-- Files are canonical; this is a rebuildable index

CREATE TABLE IF NOT EXISTS files (
  id TEXT PRIMARY KEY,
  path TEXT NOT NULL UNIQUE,
  content_hash TEXT NOT NULL,
  created_at TEXT,
  updated_at TEXT,
  indexed_at TEXT NOT NULL
);

CREATE TABLE IF NOT EXISTS notes (
  id TEXT PRIMARY KEY,
  file_id TEXT NOT NULL,
  title TEXT NOT NULL,
  type TEXT NOT NULL,
  status TEXT,
  project TEXT,
  created_at TEXT,
  updated_at TEXT,
  source_quality TEXT,
  frontmatter_json TEXT,
  body TEXT,
  FOREIGN KEY(file_id) REFERENCES files(id)
);

CREATE TABLE IF NOT EXISTS tags (
  id INTEGER PRIMARY KEY AUTOINCREMENT,
  note_id TEXT NOT NULL,
  tag TEXT NOT NULL,
  FOREIGN KEY(note_id) REFERENCES notes(id)
);

CREATE TABLE IF NOT EXISTS entities (
  id TEXT PRIMARY KEY,
  name TEXT NOT NULL,
  type TEXT,
  aliases_json TEXT,
  created_at TEXT,
  updated_at TEXT
);

CREATE TABLE IF NOT EXISTS note_entities (
  note_id TEXT NOT NULL,
  entity_id TEXT NOT NULL,
  PRIMARY KEY(note_id, entity_id)
);

CREATE TABLE IF NOT EXISTS links (
  id INTEGER PRIMARY KEY AUTOINCREMENT,
  from_note_id TEXT NOT NULL,
  to_note_id TEXT,
  raw_target TEXT NOT NULL,
  link_type TEXT
);

CREATE TABLE IF NOT EXISTS chunks (
  id TEXT PRIMARY KEY,
  note_id TEXT NOT NULL,
  chunk_index INTEGER NOT NULL,
  text TEXT NOT NULL,
  token_count INTEGER,
  embedding_model TEXT,
  embedding_json TEXT,
  created_at TEXT,
  FOREIGN KEY(note_id) REFERENCES notes(id)
);

-- FTS5 virtual table for full-text search
CREATE VIRTUAL TABLE IF NOT EXISTS notes_fts USING fts5(
  note_id UNINDEXED,
  title,
  body,
  tags,
  entities
);

CREATE TABLE IF NOT EXISTS agent_runs (
  id TEXT PRIMARY KEY,
  agent_name TEXT,
  task TEXT,
  input_json TEXT,
  output_json TEXT,
  files_changed_json TEXT,
  created_at TEXT NOT NULL
);

CREATE TABLE IF NOT EXISTS captures (
  id TEXT PRIMARY KEY,
  capture_type TEXT NOT NULL,
  title TEXT,
  source_url TEXT,
  project TEXT,
  tags_json TEXT,
  raw_payload_json TEXT,
  note_id TEXT,
  created_at TEXT NOT NULL
);

-- Migration tracking
CREATE TABLE IF NOT EXISTS schema_migrations (
  version INTEGER PRIMARY KEY,
  applied_at TEXT NOT NULL
);

-- Indexes for common queries
CREATE INDEX IF NOT EXISTS idx_notes_type ON notes(type);
CREATE INDEX IF NOT EXISTS idx_notes_project ON notes(project);
CREATE INDEX IF NOT EXISTS idx_notes_status ON notes(status);
CREATE INDEX IF NOT EXISTS idx_tags_note ON tags(note_id);
CREATE INDEX IF NOT EXISTS idx_tags_tag ON tags(tag);
CREATE INDEX IF NOT EXISTS idx_links_from ON links(from_note_id);
CREATE INDEX IF NOT EXISTS idx_links_to ON links(to_note_id);
CREATE INDEX IF NOT EXISTS idx_captures_project ON captures(project);

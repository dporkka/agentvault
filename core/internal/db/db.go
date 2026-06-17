// Package db provides SQLite database access for AgentVault.
package db

import (
	"database/sql"
	"fmt"
	"log"
	"path/filepath"
	"time"

	_ "modernc.org/sqlite"
)

// DB wraps a SQLite connection with vault-specific helpers.
type DB struct {
	conn *sql.DB
	path string
}

// Open opens the SQLite database at <vaultPath>/.agentvault/agentvault.db.
func Open(vaultPath string) (*DB, error) {
	dbPath := filepath.Join(vaultPath, ".agentvault", "agentvault.db")
	conn, err := sql.Open("sqlite", dbPath+"?_pragma=foreign_keys(1)")
	if err != nil {
		return nil, fmt.Errorf("failed to open database at %s: %w", dbPath, err)
	}
	if err := conn.Ping(); err != nil {
		return nil, fmt.Errorf("failed to ping database: %w", err)
	}

	// Configure connection pool for better performance
	conn.SetMaxOpenConns(1) // SQLite doesn't support concurrent writes
	conn.SetMaxIdleConns(5)
	conn.SetConnMaxLifetime(time.Hour)

	return &DB{conn: conn, path: dbPath}, nil
}

// Close closes the database connection.
func (d *DB) Close() error {
	return d.conn.Close()
}

// Conn returns the underlying *sql.DB.
func (d *DB) Conn() *sql.DB {
	return d.conn
}

// Path returns the database file path.
func (d *DB) Path() string {
	return d.path
}

// RunMigrations executes the inline migration SQL.
func (d *DB) RunMigrations() error {
	return d.runInlineMigrations()
}

// runInlineMigrations creates the schema directly when migration files aren't found.
func (d *DB) runInlineMigrations() error {
	schema := `
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

CREATE TABLE IF NOT EXISTS schema_migrations (
  version INTEGER PRIMARY KEY,
  applied_at TEXT NOT NULL
);

CREATE INDEX IF NOT EXISTS idx_notes_type ON notes(type);
CREATE INDEX IF NOT EXISTS idx_notes_project ON notes(project);
CREATE INDEX IF NOT EXISTS idx_notes_status ON notes(status);
CREATE INDEX IF NOT EXISTS idx_tags_note ON tags(note_id);
CREATE INDEX IF NOT EXISTS idx_tags_tag ON tags(tag);
CREATE INDEX IF NOT EXISTS idx_links_from ON links(from_note_id);
CREATE INDEX IF NOT EXISTS idx_links_to ON links(to_note_id);
CREATE INDEX IF NOT EXISTS idx_captures_project ON captures(project);
`
	_, err := d.conn.Exec(schema)
	if err != nil {
		return fmt.Errorf("failed to run inline migrations: %w", err)
	}
	_, err = d.conn.Exec(
		`INSERT OR IGNORE INTO schema_migrations (version, applied_at) VALUES (1, datetime('now'))`,
	)
	return err
}

// Exec executes a query without returning rows.
func (d *DB) Exec(query string, args ...interface{}) (sql.Result, error) {
	start := time.Now()
	result, err := d.conn.Exec(query, args...)
	if dur := time.Since(start); dur > 100*time.Millisecond {
		log.Printf("[DB] slow exec: %s (%v)", query[:min(len(query), 80)], dur)
	}
	return result, err
}

// Query executes a query that returns rows.
func (d *DB) Query(query string, args ...interface{}) (*sql.Rows, error) {
	start := time.Now()
	rows, err := d.conn.Query(query, args...)
	if dur := time.Since(start); dur > 100*time.Millisecond {
		log.Printf("[DB] slow query: %s (%v)", query[:min(len(query), 80)], dur)
	}
	return rows, err
}

// QueryRow executes a query that returns a single row.
func (d *DB) QueryRow(query string, args ...interface{}) *sql.Row {
	start := time.Now()
	row := d.conn.QueryRow(query, args...)
	if dur := time.Since(start); dur > 100*time.Millisecond {
		log.Printf("[DB] slow query row: %s (%v)", query[:min(len(query), 80)], dur)
	}
	return row
}

func min(a, b int) int {
	if a < b {
		return a
	}
	return b
}

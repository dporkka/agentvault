// Package db provides SQLite database access for AgentVault.
package db

import (
	"database/sql"
	"fmt"
	"io"
	"io/fs"
	"log"
	"path/filepath"
	"regexp"
	"sort"
	"strconv"
	"time"

	"github.com/agentvault/core/migrations"
	_ "modernc.org/sqlite"
)

// migrationsFS is the source of migration files. It defaults to the embedded
// migrations package and is overridable in tests to exercise migration paths.
var migrationsFS fs.FS = migrations.FS

// DB wraps a SQLite connection with vault-specific helpers.
type DB struct {
	conn *sql.DB
	path string
}

// Open opens the SQLite database at <vaultPath>/.agentvault/agentvault.db.
func Open(vaultPath string) (*DB, error) {
	dbPath := filepath.Join(vaultPath, ".agentvault", "agentvault.db")
	conn, err := sql.Open("sqlite", dbPath+"?_pragma=foreign_keys(1)&_pragma=journal_mode(WAL)")
	if err != nil {
		return nil, fmt.Errorf("failed to open database at %s: %w", dbPath, err)
	}
	if err := conn.Ping(); err != nil {
		return nil, fmt.Errorf("failed to ping database: %w", err)
	}

	// Configure connection pool for better performance.
	// WAL mode allows concurrent readers; writes are serialized by SQLite.
	conn.SetMaxOpenConns(16)
	conn.SetMaxIdleConns(4)
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

// Begin starts a new database transaction.
func (d *DB) Begin() (*sql.Tx, error) {
	return d.conn.Begin()
}

// RunMigrations executes embedded migration SQL when available, falling back to
// the inline schema if no migration files are embedded.
func (d *DB) RunMigrations() error {
	entries, err := fs.ReadDir(migrationsFS, ".")
	if err != nil || len(entries) == 0 {
		return d.runInlineMigrations()
	}
	return d.runEmbeddedMigrations(entries)
}

// migration represents a single embedded SQL migration.
type migration struct {
	version int
	name    string
	sql     string
}

var migrationVersionRe = regexp.MustCompile(`^(\d+)`)

// runEmbeddedMigrations creates the schema from embedded migration files.
func (d *DB) runEmbeddedMigrations(entries []fs.DirEntry) error {
	var migrationsList []migration
	for _, entry := range entries {
		if entry.IsDir() {
			continue
		}
		name := entry.Name()
		if !regexp.MustCompile(`\.sql$`).MatchString(name) {
			continue
		}

		match := migrationVersionRe.FindStringSubmatch(name)
		if match == nil {
			continue
		}
		version, err := strconv.Atoi(match[1])
		if err != nil {
			continue
		}

		f, err := migrationsFS.Open(name)
		if err != nil {
			return fmt.Errorf("failed to open migration %s: %w", name, err)
		}
		data, err := io.ReadAll(f)
		_ = f.Close()
		if err != nil {
			return fmt.Errorf("failed to read migration %s: %w", name, err)
		}

		migrationsList = append(migrationsList, migration{
			version: version,
			name:    name,
			sql:     string(data),
		})
	}

	sort.Slice(migrationsList, func(i, j int) bool {
		return migrationsList[i].version < migrationsList[j].version
	})

	// Ensure the migration tracking table exists before querying it.
	if _, err := d.conn.Exec(`
		CREATE TABLE IF NOT EXISTS schema_migrations (
			version INTEGER PRIMARY KEY,
			applied_at TEXT NOT NULL
		)
	`); err != nil {
		return fmt.Errorf("failed to create schema_migrations: %w", err)
	}

	var currentVersion int
	row := d.conn.QueryRow("SELECT COALESCE(MAX(version), 0) FROM schema_migrations")
	if err := row.Scan(&currentVersion); err != nil {
		return fmt.Errorf("failed to query current migration version: %w", err)
	}

	for _, m := range migrationsList {
		if m.version <= currentVersion {
			continue
		}
		if _, err := d.conn.Exec(m.sql); err != nil {
			return fmt.Errorf("failed to run migration %s: %w", m.name, err)
		}
		if _, err := d.conn.Exec(
			`INSERT OR REPLACE INTO schema_migrations (version, applied_at) VALUES (?, datetime('now'))`,
			m.version,
		); err != nil {
			return fmt.Errorf("failed to record migration %s: %w", m.name, err)
		}
	}

	return nil
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
  agent_id TEXT,
  agent_revision INTEGER,
  conversation_id TEXT,
  status TEXT NOT NULL DEFAULT 'succeeded',
  started_at TEXT,
  ended_at TEXT,
  context_hash TEXT,
  capability_snapshot_json TEXT,
  runtime_metadata_json TEXT,
  created_at TEXT NOT NULL
);

CREATE TABLE IF NOT EXISTS conversations (
  id TEXT PRIMARY KEY,
  title TEXT NOT NULL DEFAULT 'Untitled',
  created_at TEXT NOT NULL,
  updated_at TEXT NOT NULL
);

CREATE TABLE IF NOT EXISTS conversation_messages (
  id INTEGER PRIMARY KEY AUTOINCREMENT,
  conversation_id TEXT NOT NULL,
  role TEXT NOT NULL CHECK(role IN ('user', 'assistant', 'system')),
  content TEXT NOT NULL,
  sources_json TEXT,
  created_at TEXT NOT NULL,
  FOREIGN KEY(conversation_id) REFERENCES conversations(id) ON DELETE CASCADE
);

CREATE TABLE IF NOT EXISTS run_observations (
  id TEXT PRIMARY KEY,
  run_id TEXT NOT NULL,
  parent_observation_id TEXT,
  kind TEXT NOT NULL,
  name TEXT NOT NULL,
  status TEXT,
  input_json TEXT,
  output_json TEXT,
  evidence_json TEXT,
  started_at TEXT,
  ended_at TEXT,
  created_at TEXT NOT NULL,
  FOREIGN KEY(run_id) REFERENCES agent_runs(id) ON DELETE CASCADE,
  FOREIGN KEY(parent_observation_id) REFERENCES run_observations(id) ON DELETE SET NULL
);

CREATE TABLE IF NOT EXISTS evaluations (
  id TEXT PRIMARY KEY,
  run_id TEXT NOT NULL,
  observation_id TEXT,
  evaluator TEXT NOT NULL,
  name TEXT NOT NULL,
  score REAL,
  label TEXT,
  rationale TEXT,
  metadata_json TEXT,
  created_at TEXT NOT NULL,
  FOREIGN KEY(run_id) REFERENCES agent_runs(id) ON DELETE CASCADE,
  FOREIGN KEY(observation_id) REFERENCES run_observations(id) ON DELETE CASCADE
);

CREATE TABLE IF NOT EXISTS promotion_records (
  id TEXT PRIMARY KEY,
  agent_id TEXT NOT NULL,
  target_kind TEXT NOT NULL CHECK(target_kind IN ('memory', 'knowledge')),
  status TEXT NOT NULL CHECK(status IN ('proposed', 'approved', 'rejected', 'committed', 'superseded')),
  candidate TEXT NOT NULL,
  rationale TEXT,
  source_run_ids_json TEXT NOT NULL DEFAULT '[]',
  source_observation_ids_json TEXT NOT NULL DEFAULT '[]',
  source_evaluation_ids_json TEXT NOT NULL DEFAULT '[]',
  target_note_id TEXT,
  supersedes_note_id TEXT,
  created_at TEXT NOT NULL,
  reviewed_at TEXT,
  committed_at TEXT
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
CREATE INDEX IF NOT EXISTS idx_conv_messages_conv ON conversation_messages(conversation_id);
CREATE INDEX IF NOT EXISTS idx_conv_messages_created ON conversation_messages(conversation_id, created_at);
CREATE INDEX IF NOT EXISTS idx_agent_runs_agent ON agent_runs(agent_id, created_at);
CREATE INDEX IF NOT EXISTS idx_agent_runs_conversation ON agent_runs(conversation_id, created_at);
CREATE INDEX IF NOT EXISTS idx_run_observations_run ON run_observations(run_id, created_at);
CREATE INDEX IF NOT EXISTS idx_run_observations_kind ON run_observations(kind, created_at);
CREATE INDEX IF NOT EXISTS idx_evaluations_run ON evaluations(run_id, created_at);
CREATE INDEX IF NOT EXISTS idx_evaluations_observation ON evaluations(observation_id, created_at);
CREATE INDEX IF NOT EXISTS idx_promotion_records_agent ON promotion_records(agent_id, created_at);
CREATE INDEX IF NOT EXISTS idx_promotion_records_status ON promotion_records(status, created_at);
`
	_, err := d.conn.Exec(schema)
	if err != nil {
		return fmt.Errorf("failed to run inline migrations: %w", err)
	}
	_, err = d.conn.Exec(
		`INSERT OR IGNORE INTO schema_migrations (version, applied_at) VALUES (3, datetime('now'))`,
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

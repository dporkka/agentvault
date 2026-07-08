package db

import (
	"os"
	"path/filepath"
	"testing"
	"testing/fstest"

	"github.com/agentvault/core/migrations"
)

func TestOpen(t *testing.T) {
	t.Run("new database", func(t *testing.T) {
		tmpDir := t.TempDir()
		os.MkdirAll(filepath.Join(tmpDir, ".agentvault"), 0755)

		db, err := Open(tmpDir)
		if err != nil {
			t.Fatalf("Open failed: %v", err)
		}
		defer db.Close()

		if db.Path() == "" {
			t.Error("Expected non-empty path")
		}
	})

	t.Run("missing directory", func(t *testing.T) {
		tmpDir := t.TempDir()
		// Don't create .agentvault directory
		_, err := Open(tmpDir)
		if err == nil {
			t.Error("Expected error for missing .agentvault directory")
		}
	})
}

func TestRunMigrations(t *testing.T) {
	tmpDir := t.TempDir()
	os.MkdirAll(filepath.Join(tmpDir, ".agentvault"), 0755)

	db, err := Open(tmpDir)
	if err != nil {
		t.Fatalf("Open failed: %v", err)
	}
	defer db.Close()

	if err := db.RunMigrations(); err != nil {
		t.Fatalf("RunMigrations failed: %v", err)
	}

	// Verify migration was recorded
	var version int
	err = db.QueryRow("SELECT version FROM schema_migrations LIMIT 1").Scan(&version)
	if err != nil {
		t.Fatalf("Failed to query schema_migrations: %v", err)
	}
	if version != 1 {
		t.Errorf("Expected migration version 1, got %d", version)
	}

	// Verify tables exist
	tables := []string{"files", "notes", "tags", "entities", "links", "schema_migrations"}
	for _, table := range tables {
		var name string
		err := db.QueryRow("SELECT name FROM sqlite_master WHERE type='table' AND name=?", table).Scan(&name)
		if err != nil {
			t.Errorf("Expected table %s to exist: %v", table, err)
		}
	}
}

func TestExecAndQuery(t *testing.T) {
	tmpDir := t.TempDir()
	os.MkdirAll(filepath.Join(tmpDir, ".agentvault"), 0755)

	db, err := Open(tmpDir)
	if err != nil {
		t.Fatalf("Open failed: %v", err)
	}
	defer db.Close()

	if err := db.RunMigrations(); err != nil {
		t.Fatalf("RunMigrations failed: %v", err)
	}

	// Insert a file
	_, err = db.Exec(
		`INSERT INTO files (id, path, content_hash, indexed_at) VALUES (?, ?, ?, datetime('now'))`,
		"file_001", "test.md", "abc123",
	)
	if err != nil {
		t.Fatalf("Exec failed: %v", err)
	}

	// Query it back
	var path string
	err = db.QueryRow("SELECT path FROM files WHERE id = ?", "file_001").Scan(&path)
	if err != nil {
		t.Fatalf("QueryRow failed: %v", err)
	}
	if path != "test.md" {
		t.Errorf("Expected 'test.md', got '%s'", path)
	}

	// Query multiple rows
	_, err = db.Exec(
		`INSERT INTO files (id, path, content_hash, indexed_at) VALUES (?, ?, ?, datetime('now'))`,
		"file_002", "test2.md", "def456",
	)
	if err != nil {
		t.Fatalf("Second insert failed: %v", err)
	}

	rows, err := db.Query("SELECT id, path FROM files ORDER BY id")
	if err != nil {
		t.Fatalf("Query failed: %v", err)
	}
	defer rows.Close()

	count := 0
	for rows.Next() {
		var id, p string
		if err := rows.Scan(&id, &p); err != nil {
			t.Fatalf("Scan failed: %v", err)
		}
		count++
	}
	if count != 2 {
		t.Errorf("Expected 2 rows, got %d", count)
	}
}

func TestRunMigrationsIdempotent(t *testing.T) {
	tmpDir := t.TempDir()
	os.MkdirAll(filepath.Join(tmpDir, ".agentvault"), 0755)

	db, err := Open(tmpDir)
	if err != nil {
		t.Fatalf("Open failed: %v", err)
	}
	defer db.Close()

	if err := db.RunMigrations(); err != nil {
		t.Fatalf("RunMigrations failed: %v", err)
	}
	if err := db.RunMigrations(); err != nil {
		t.Fatalf("RunMigrations second run failed: %v", err)
	}

	var version int
	err = db.QueryRow("SELECT COALESCE(MAX(version), 0) FROM schema_migrations").Scan(&version)
	if err != nil {
		t.Fatalf("Failed to query schema_migrations: %v", err)
	}
	if version != 1 {
		t.Errorf("Expected migration version 1, got %d", version)
	}
}

func TestEmbeddedMigrationsPresent(t *testing.T) {
	entries, err := migrations.FS.ReadDir(".")
	if err != nil {
		t.Fatalf("Failed to read embedded migrations: %v", err)
	}
	found := false
	for _, e := range entries {
		if e.Name() == "001_init.sql" {
			found = true
			break
		}
	}
	if !found {
		t.Error("Expected 001_init.sql to be embedded")
	}
}

func TestConn(t *testing.T) {
	tmpDir := t.TempDir()
	os.MkdirAll(filepath.Join(tmpDir, ".agentvault"), 0755)

	db, err := Open(tmpDir)
	if err != nil {
		t.Fatalf("Open failed: %v", err)
	}
	defer db.Close()

	if db.Conn() == nil {
		t.Error("Expected Conn() to return a non-nil *sql.DB")
	}
	if db.Conn() != db.conn {
		t.Error("Expected Conn() to return the underlying connection")
	}
}

func TestRunInlineMigrations(t *testing.T) {
	tmpDir := t.TempDir()
	os.MkdirAll(filepath.Join(tmpDir, ".agentvault"), 0755)

	db, err := Open(tmpDir)
	if err != nil {
		t.Fatalf("Open failed: %v", err)
	}
	defer db.Close()

	if err := db.runInlineMigrations(); err != nil {
		t.Fatalf("runInlineMigrations failed: %v", err)
	}

	// Verify the inline schema created expected tables.
	tables := []string{"files", "notes", "tags", "entities", "links", "chunks", "schema_migrations"}
	for _, table := range tables {
		var name string
		err := db.QueryRow("SELECT name FROM sqlite_master WHERE type='table' AND name=?", table).Scan(&name)
		if err != nil {
			t.Errorf("Expected table %s to exist: %v", table, err)
		}
	}

	// Verify the FTS5 virtual table was created.
	var ftsName string
	err = db.QueryRow("SELECT name FROM sqlite_master WHERE type='table' AND name='notes_fts'").Scan(&ftsName)
	if err != nil {
		t.Errorf("Expected notes_fts virtual table to exist: %v", err)
	}

	// Verify migration version was recorded.
	var version int
	err = db.QueryRow("SELECT version FROM schema_migrations LIMIT 1").Scan(&version)
	if err != nil {
		t.Fatalf("Failed to query schema_migrations: %v", err)
	}
	if version != 1 {
		t.Errorf("Expected migration version 1, got %d", version)
	}
}

func TestRunMigrationsInlineFallback(t *testing.T) {
	origFS := migrationsFS
	defer func() { migrationsFS = origFS }()

	// Empty filesystem forces RunMigrations to fall back to inline migrations.
	migrationsFS = fstest.MapFS{}

	tmpDir := t.TempDir()
	os.MkdirAll(filepath.Join(tmpDir, ".agentvault"), 0755)

	db, err := Open(tmpDir)
	if err != nil {
		t.Fatalf("Open failed: %v", err)
	}
	defer db.Close()

	if err := db.RunMigrations(); err != nil {
		t.Fatalf("RunMigrations failed: %v", err)
	}

	var version int
	err = db.QueryRow("SELECT version FROM schema_migrations LIMIT 1").Scan(&version)
	if err != nil {
		t.Fatalf("Failed to query schema_migrations: %v", err)
	}
	if version != 1 {
		t.Errorf("Expected inline migration version 1, got %d", version)
	}
}

func TestRunMigrationsEmbeddedBranches(t *testing.T) {
	origFS := migrationsFS
	defer func() { migrationsFS = origFS }()

	migrationsFS = fstest.MapFS{
		"ignored_dir/": &fstest.MapFile{Mode: os.ModeDir | 0755},
		"readme.txt":   &fstest.MapFile{Data: []byte("not a migration")},
		"init.sql":     &fstest.MapFile{Data: []byte("CREATE TABLE init_table (id TEXT PRIMARY KEY);")},
		"001_init.sql": &fstest.MapFile{Data: []byte(`
			CREATE TABLE IF NOT EXISTS files (
				id TEXT PRIMARY KEY,
				path TEXT NOT NULL UNIQUE,
				content_hash TEXT NOT NULL,
				indexed_at TEXT NOT NULL
			);
			CREATE TABLE IF NOT EXISTS schema_migrations (
				version INTEGER PRIMARY KEY,
				applied_at TEXT NOT NULL
			);
		`)},
		"002_add_index.sql": &fstest.MapFile{Data: []byte(`
			CREATE INDEX IF NOT EXISTS idx_files_path ON files(path);
		`)},
	}

	tmpDir := t.TempDir()
	os.MkdirAll(filepath.Join(tmpDir, ".agentvault"), 0755)

	db, err := Open(tmpDir)
	if err != nil {
		t.Fatalf("Open failed: %v", err)
	}
	defer db.Close()

	if err := db.RunMigrations(); err != nil {
		t.Fatalf("RunMigrations failed: %v", err)
	}

	// Verify both migrations were applied and recorded in order.
	rows, err := db.Query("SELECT version FROM schema_migrations ORDER BY version")
	if err != nil {
		t.Fatalf("Failed to query schema_migrations: %v", err)
	}
	defer rows.Close()

	versions := []int{}
	for rows.Next() {
		var v int
		if err := rows.Scan(&v); err != nil {
			t.Fatalf("Scan failed: %v", err)
		}
		versions = append(versions, v)
	}
	if len(versions) != 2 || versions[0] != 1 || versions[1] != 2 {
		t.Errorf("Expected versions [1, 2], got %v", versions)
	}

	// Verify the index from migration 002 exists.
	var idxName string
	err = db.QueryRow("SELECT name FROM sqlite_master WHERE type='index' AND name='idx_files_path'").Scan(&idxName)
	if err != nil {
		t.Errorf("Expected idx_files_path index to exist: %v", err)
	}

	// Running again should be a no-op (skips already-applied versions).
	if err := db.RunMigrations(); err != nil {
		t.Fatalf("RunMigrations second run failed: %v", err)
	}

	var count int
	err = db.QueryRow("SELECT COUNT(*) FROM schema_migrations").Scan(&count)
	if err != nil {
		t.Fatalf("Failed to count schema_migrations: %v", err)
	}
	if count != 2 {
		t.Errorf("Expected 2 migration records after idempotent run, got %d", count)
	}
}

func TestRunEmbeddedMigrationsBadSQL(t *testing.T) {
	origFS := migrationsFS
	defer func() { migrationsFS = origFS }()

	migrationsFS = fstest.MapFS{
		"001_bad.sql": &fstest.MapFile{Data: []byte("THIS IS NOT SQL")},
	}

	tmpDir := t.TempDir()
	os.MkdirAll(filepath.Join(tmpDir, ".agentvault"), 0755)

	db, err := Open(tmpDir)
	if err != nil {
		t.Fatalf("Open failed: %v", err)
	}
	defer db.Close()

	if err := db.RunMigrations(); err == nil {
		t.Error("Expected RunMigrations to fail on invalid SQL")
	}
}

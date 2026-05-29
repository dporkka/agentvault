package db

import (
	"os"
	"path/filepath"
	"testing"
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

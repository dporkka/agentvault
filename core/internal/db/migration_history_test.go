package db

import (
	"os"
	"path/filepath"
	"strings"
	"testing"
	"testing/fstest"
)

func TestRunMigrationsRejectsHistoryGap(t *testing.T) {
	origFS := migrationsFS
	defer func() { migrationsFS = origFS }()

	migrationsFS = fstest.MapFS{
		"001_init.sql": &fstest.MapFile{Data: []byte(`
			CREATE TABLE IF NOT EXISTS first_step (id INTEGER PRIMARY KEY);
		`)},
		"002_middle.sql": &fstest.MapFile{Data: []byte(`
			CREATE TABLE IF NOT EXISTS should_not_apply (id INTEGER PRIMARY KEY);
		`)},
		"003_tail.sql": &fstest.MapFile{Data: []byte(`
			CREATE TABLE IF NOT EXISTS final_step (id INTEGER PRIMARY KEY);
		`)},
	}

	tmpDir := t.TempDir()
	if err := os.MkdirAll(filepath.Join(tmpDir, ".agentvault"), 0755); err != nil {
		t.Fatalf("mkdir: %v", err)
	}

	database, err := Open(tmpDir)
	if err != nil {
		t.Fatalf("Open failed: %v", err)
	}
	defer database.Close()

	if _, err := database.Exec(`
		CREATE TABLE schema_migrations (
			version INTEGER PRIMARY KEY,
			applied_at TEXT NOT NULL
		)
	`); err != nil {
		t.Fatalf("create migration ledger: %v", err)
	}
	if _, err := database.Exec(`
		INSERT INTO schema_migrations (version, applied_at)
		VALUES (1, datetime('now')), (3, datetime('now'))
	`); err != nil {
		t.Fatalf("seed migration ledger: %v", err)
	}

	err = database.RunMigrations()
	if err == nil {
		t.Fatal("expected migration history gap to fail")
	}
	if !strings.Contains(err.Error(), "migration history gap") ||
		!strings.Contains(err.Error(), "expected version 2") {
		t.Fatalf("unexpected error: %v", err)
	}

	var count int
	if err := database.QueryRow(`
		SELECT COUNT(*) FROM sqlite_master
		WHERE type = 'table' AND name = 'should_not_apply'
	`).Scan(&count); err != nil {
		t.Fatalf("query sqlite_master: %v", err)
	}
	if count != 0 {
		t.Fatalf("migration 2 must not be applied after version 3 is already recorded")
	}
}

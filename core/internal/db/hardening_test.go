package db

import (
	"os"
	"path/filepath"
	"testing"
	"testing/fstest"
)

func TestEmbeddedMigrationRollsBackSchemaOnFailure(t *testing.T) {
	origFS := migrationsFS
	defer func() { migrationsFS = origFS }()

	migrationsFS = fstest.MapFS{
		"001_partial_failure.sql": &fstest.MapFile{Data: []byte(`
			CREATE TABLE should_rollback (id INTEGER PRIMARY KEY);
			THIS IS NOT VALID SQL;
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

	if err := database.RunMigrations(); err == nil {
		t.Fatal("expected migration failure")
	}

	var tableCount int
	if err := database.QueryRow(
		"SELECT COUNT(*) FROM sqlite_master WHERE type='table' AND name='should_rollback'",
	).Scan(&tableCount); err != nil {
		t.Fatalf("query sqlite_master: %v", err)
	}
	if tableCount != 0 {
		t.Fatalf("expected schema changes to roll back, found %d table(s)", tableCount)
	}

	var versionCount int
	if err := database.QueryRow("SELECT COUNT(*) FROM schema_migrations").Scan(&versionCount); err != nil {
		t.Fatalf("query schema_migrations: %v", err)
	}
	if versionCount != 0 {
		t.Fatalf("expected failed migration not to be recorded, got %d records", versionCount)
	}
}

func TestOpenConfiguresBusyTimeout(t *testing.T) {
	tmpDir := t.TempDir()
	if err := os.MkdirAll(filepath.Join(tmpDir, ".agentvault"), 0755); err != nil {
		t.Fatalf("mkdir: %v", err)
	}

	database, err := Open(tmpDir)
	if err != nil {
		t.Fatalf("Open failed: %v", err)
	}
	defer database.Close()

	var busyTimeout int
	if err := database.QueryRow("PRAGMA busy_timeout").Scan(&busyTimeout); err != nil {
		t.Fatalf("query busy_timeout: %v", err)
	}
	if busyTimeout != 5000 {
		t.Fatalf("expected busy_timeout=5000ms, got %d", busyTimeout)
	}
}

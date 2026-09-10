package db

import (
	"os"
	"path/filepath"
	"testing"
	"testing/fstest"
)

func TestInlineFallbackCreatesLatestMemorySchema(t *testing.T) {
	origFS := migrationsFS
	defer func() { migrationsFS = origFS }()
	migrationsFS = fstest.MapFS{}

	root := t.TempDir()
	if err := os.MkdirAll(filepath.Join(root, ".agentvault"), 0755); err != nil {
		t.Fatal(err)
	}
	database, err := Open(root)
	if err != nil {
		t.Fatal(err)
	}
	defer database.Close()

	if err := database.RunMigrations(); err != nil {
		t.Fatalf("RunMigrations() fallback error: %v", err)
	}

	var version int
	if err := database.QueryRow("SELECT COALESCE(MAX(version), 0) FROM schema_migrations").Scan(&version); err != nil {
		t.Fatal(err)
	}
	if version != 3 {
		t.Fatalf("inline schema version = %d, want 3", version)
	}

	for _, table := range []string{"conversations", "conversation_messages", "memory_supersessions"} {
		var count int
		if err := database.QueryRow(
			"SELECT COUNT(*) FROM sqlite_master WHERE type = 'table' AND name = ?",
			table,
		).Scan(&count); err != nil {
			t.Fatal(err)
		}
		if count != 1 {
			t.Errorf("inline fallback missing table %s", table)
		}
	}

	columns := map[string]bool{}
	rows, err := database.Query("PRAGMA table_info(notes)")
	if err != nil {
		t.Fatal(err)
	}
	defer rows.Close()
	for rows.Next() {
		var cid int
		var name, columnType string
		var notNull int
		var defaultValue interface{}
		var primaryKey int
		if err := rows.Scan(&cid, &name, &columnType, &notNull, &defaultValue, &primaryKey); err != nil {
			t.Fatal(err)
		}
		columns[name] = true
	}
	for _, column := range []string{"workspace_id", "agent_id", "session_id", "memory_kind", "confidence", "provenance_json", "observed_at", "valid_from", "valid_to"} {
		if !columns[column] {
			t.Errorf("inline fallback missing notes.%s", column)
		}
	}
}

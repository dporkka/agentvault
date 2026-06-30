package indexer

import (
	"os"
	"path/filepath"
	"testing"

	"github.com/agentvault/core/internal/db"
)

func setupIndexerTest(t *testing.T) (string, *db.DB) {
	t.Helper()
	tmpDir := t.TempDir()
	os.MkdirAll(filepath.Join(tmpDir, ".agentvault"), 0755)

	database, err := db.Open(tmpDir)
	if err != nil {
		t.Fatalf("failed to open database: %v", err)
	}
	if err := database.RunMigrations(); err != nil {
		t.Fatalf("failed to run migrations: %v", err)
	}
	return tmpDir, database
}

func TestIndexCreatesWikiLink(t *testing.T) {
	vaultPath, database := setupIndexerTest(t)
	defer database.Close()

	notesDir := filepath.Join(vaultPath, "10-notes")
	if err := os.MkdirAll(notesDir, 0755); err != nil {
		t.Fatalf("failed to create notes dir: %v", err)
	}

	sourceContent := `---
id: note_source
type: note
title: Source Note
---

See [[Target Note]] for details.
`
	targetContent := `---
id: note_target
type: note
title: Target Note
---

# Target Note

This is the target.
`
	if err := os.WriteFile(filepath.Join(notesDir, "source.md"), []byte(sourceContent), 0644); err != nil {
		t.Fatalf("failed to write source note: %v", err)
	}
	if err := os.WriteFile(filepath.Join(notesDir, "target.md"), []byte(targetContent), 0644); err != nil {
		t.Fatalf("failed to write target note: %v", err)
	}

	idx := New(database, vaultPath)
	result, err := idx.Index(IndexOptions{})
	if err != nil {
		t.Fatalf("index failed: %v", err)
	}
	if result.Scanned != 2 {
		t.Errorf("expected 2 scanned, got %d", result.Scanned)
	}

	outgoing, err := database.GetOutgoingLinks("note_source")
	if err != nil {
		t.Fatalf("GetOutgoingLinks failed: %v", err)
	}
	if len(outgoing) != 1 {
		t.Fatalf("expected 1 outgoing link, got %d", len(outgoing))
	}
	if outgoing[0].ID != "note_target" {
		t.Errorf("expected outgoing target note_target, got %s", outgoing[0].ID)
	}

	backlinks, err := database.GetBacklinks("note_target")
	if err != nil {
		t.Fatalf("GetBacklinks failed: %v", err)
	}
	if len(backlinks) != 1 {
		t.Fatalf("expected 1 backlink, got %d", len(backlinks))
	}
	if backlinks[0].ID != "note_source" {
		t.Errorf("expected backlink source note_source, got %s", backlinks[0].ID)
	}
}

func TestIndexReplacesLinksOnReindex(t *testing.T) {
	vaultPath, database := setupIndexerTest(t)
	defer database.Close()

	notesDir := filepath.Join(vaultPath, "10-notes")
	if err := os.MkdirAll(notesDir, 0755); err != nil {
		t.Fatalf("failed to create notes dir: %v", err)
	}

	content := `---
id: note_source
type: note
title: Source Note
---

First [[Old Link]].
`
	if err := os.WriteFile(filepath.Join(notesDir, "source.md"), []byte(content), 0644); err != nil {
		t.Fatalf("failed to write note: %v", err)
	}

	idx := New(database, vaultPath)
	if _, err := idx.Index(IndexOptions{}); err != nil {
		t.Fatalf("first index failed: %v", err)
	}

	// Rewrite with a different link and force reindex.
	content = `---
id: note_source
type: note
title: Source Note
---

Second [[New Link]].
`
	if err := os.WriteFile(filepath.Join(notesDir, "source.md"), []byte(content), 0644); err != nil {
		t.Fatalf("failed to rewrite note: %v", err)
	}
	if _, err := idx.Index(IndexOptions{Force: true}); err != nil {
		t.Fatalf("second index failed: %v", err)
	}

	rows, err := database.Query("SELECT raw_target FROM links WHERE from_note_id = ?", "note_source")
	if err != nil {
		t.Fatalf("query links failed: %v", err)
	}
	defer rows.Close()

	var targets []string
	for rows.Next() {
		var target string
		if err := rows.Scan(&target); err != nil {
			t.Fatalf("scan failed: %v", err)
		}
		targets = append(targets, target)
	}
	if err := rows.Err(); err != nil {
		t.Fatalf("rows err: %v", err)
	}
	if len(targets) != 1 || targets[0] != "New Link" {
		t.Errorf("expected ['New Link'], got %v", targets)
	}
}

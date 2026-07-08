package main

import (
	"os"
	"path/filepath"
	"strings"
	"testing"
)

func TestRunReadByID(t *testing.T) {
	vp, database := setupTestVaultWithDB(t)
	defer database.Close()
	vaultPath = vp

	indexNote(t, database, vp, "10-notes/readable.md", "---\nid: read_001\ntype: note\ntitle: Readable Note\ntags: [go, test]\n---\n\n# Readable Note\n\nThis is the body.")

	cmd := cobraCommandArgs([]string{"read_001"})
	if err := readCmd.RunE(cmd, []string{"read_001"}); err != nil {
		t.Fatalf("readCmd returned error: %v", err)
	}
}

func TestRunReadByPath(t *testing.T) {
	vp, database := setupTestVaultWithDB(t)
	defer database.Close()
	vaultPath = vp

	indexNote(t, database, vp, "10-notes/by-path.md", "---\nid: read_002\ntype: note\ntitle: By Path\n---\n\n# By Path\n\nContent.")

	cmd := cobraCommandArgs([]string{"10-notes/by-path.md"})
	if err := readCmd.RunE(cmd, []string{"10-notes/by-path.md"}); err != nil {
		t.Fatalf("readCmd returned error: %v", err)
	}
}

func TestRunReadByPathAddsExtension(t *testing.T) {
	vp, database := setupTestVaultWithDB(t)
	defer database.Close()
	vaultPath = vp

	indexNote(t, database, vp, "10-notes/no-ext.md", "---\nid: read_003\ntype: note\ntitle: No Extension\n---\n\n# No Extension\n\nBody text.")

	cmd := cobraCommandArgs([]string{"10-notes/no-ext"})
	if err := readCmd.RunE(cmd, []string{"10-notes/no-ext"}); err != nil {
		t.Fatalf("readCmd returned error: %v", err)
	}
}

func TestRunReadNotFound(t *testing.T) {
	vp, database := setupTestVaultWithDB(t)
	defer database.Close()
	vaultPath = vp

	cmd := cobraCommandArgs([]string{"missing-id"})
	err := readCmd.RunE(cmd, []string{"missing-id"})
	if err == nil {
		t.Fatal("readCmd expected error for missing note")
	}
	if !strings.Contains(err.Error(), "note not found") {
		t.Errorf("error should mention 'note not found', got: %v", err)
	}
}

func TestRunReadFallbackToDB(t *testing.T) {
	vp, database := setupTestVaultWithDB(t)
	defer database.Close()
	vaultPath = vp

	indexNote(t, database, vp, "10-notes/deleted.md", "---\nid: read_004\ntype: note\ntitle: Deleted File\n---\n\n# Deleted File\n\nStill in the database.")

	// Remove the file so the read command falls back to database content.
	if err := os.Remove(filepath.Join(vp, "10-notes", "deleted.md")); err != nil {
		t.Fatalf("failed to remove note file: %v", err)
	}

	cmd := cobraCommandArgs([]string{"read_004"})
	if err := readCmd.RunE(cmd, []string{"read_004"}); err != nil {
		t.Fatalf("readCmd returned error: %v", err)
	}
}

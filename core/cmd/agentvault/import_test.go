package main

import (
	"os"
	"path/filepath"
	"strings"
	"testing"
)

func TestRunImportUnknownImporter(t *testing.T) {
	vp, database := setupTestVaultWithDB(t)
	defer database.Close()
	vaultPath = vp

	importMode = "copy"
	importProject = ""
	importTags = ""
	importKeepStructure = false
	importDryRun = false

	cmd := cobraCommandArgs([]string{"unknown", "/tmp/source"})
	err := runImport(cmd, []string{"unknown", "/tmp/source"})
	if err == nil {
		t.Fatal("runImport expected error for unknown importer")
	}
	if !strings.Contains(err.Error(), "unknown importer") {
		t.Errorf("error should mention 'unknown importer', got: %v", err)
	}
}

func TestRunImportInvalidMode(t *testing.T) {
	vp, database := setupTestVaultWithDB(t)
	defer database.Close()
	vaultPath = vp

	importMode = "invalid"
	importProject = ""
	importTags = ""
	importKeepStructure = false
	importDryRun = false

	cmd := cobraCommandArgs([]string{"markdown", "/tmp/source"})
	err := runImport(cmd, []string{"markdown", "/tmp/source"})
	if err == nil {
		t.Fatal("runImport expected error for invalid mode")
	}
	if !strings.Contains(err.Error(), "invalid mode") {
		t.Errorf("error should mention 'invalid mode', got: %v", err)
	}
}

func TestRunImportMarkdownCopy(t *testing.T) {
	vp, database := setupTestVaultWithDB(t)
	defer database.Close()
	vaultPath = vp

	sourceDir := t.TempDir()
	if err := os.WriteFile(filepath.Join(sourceDir, "hello.md"), []byte("# Hello\n\nWorld"), 0644); err != nil {
		t.Fatalf("failed to write source file: %v", err)
	}

	importMode = "copy"
	importProject = ""
	importTags = "imported, archive"
	importKeepStructure = false
	importDryRun = false

	cmd := cobraCommandArgs([]string{"markdown", sourceDir})
	if err := runImport(cmd, []string{"markdown", sourceDir}); err != nil {
		t.Fatalf("runImport returned error: %v", err)
	}

	matches, err := filepath.Glob(filepath.Join(vp, "10-notes", "*.md"))
	if err != nil {
		t.Fatalf("failed to glob imported notes: %v", err)
	}
	if len(matches) != 1 {
		t.Fatalf("expected 1 imported note, got %d", len(matches))
	}

	content, err := os.ReadFile(matches[0])
	if err != nil {
		t.Fatalf("failed to read imported note: %v", err)
	}
	if !strings.Contains(string(content), "# Hello") {
		t.Error("imported note missing expected heading")
	}
}

func TestRunImportMarkdownDryRun(t *testing.T) {
	vp, database := setupTestVaultWithDB(t)
	defer database.Close()
	vaultPath = vp

	sourceDir := t.TempDir()
	if err := os.WriteFile(filepath.Join(sourceDir, "dry.md"), []byte("# Dry"), 0644); err != nil {
		t.Fatalf("failed to write source file: %v", err)
	}

	importMode = "copy"
	importProject = ""
	importTags = ""
	importKeepStructure = false
	importDryRun = true

	cmd := cobraCommandArgs([]string{"markdown", sourceDir})
	if err := runImport(cmd, []string{"markdown", sourceDir}); err != nil {
		t.Fatalf("runImport returned error: %v", err)
	}

	matches, err := filepath.Glob(filepath.Join(vp, "10-notes", "*.md"))
	if err != nil {
		t.Fatalf("failed to glob notes: %v", err)
	}
	if len(matches) != 0 {
		t.Errorf("expected no files written in dry run, got %d", len(matches))
	}
}

func TestRunImportMarkdownNormalize(t *testing.T) {
	vp, database := setupTestVaultWithDB(t)
	defer database.Close()
	vaultPath = vp

	sourceDir := t.TempDir()
	sourceContent := "# My Note\n\nSome content."
	if err := os.WriteFile(filepath.Join(sourceDir, "note.md"), []byte(sourceContent), 0644); err != nil {
		t.Fatalf("failed to write source file: %v", err)
	}

	importMode = "normalize"
	importProject = "myproject"
	importTags = "tag1"
	importKeepStructure = false
	importDryRun = false

	cmd := cobraCommandArgs([]string{"markdown", sourceDir})
	if err := runImport(cmd, []string{"markdown", sourceDir}); err != nil {
		t.Fatalf("runImport returned error: %v", err)
	}

	matches, err := filepath.Glob(filepath.Join(vp, "10-notes", "*.md"))
	if err != nil {
		t.Fatalf("failed to glob notes: %v", err)
	}
	if len(matches) != 1 {
		t.Fatalf("expected 1 imported note, got %d", len(matches))
	}

	content, err := os.ReadFile(matches[0])
	if err != nil {
		t.Fatalf("failed to read imported note: %v", err)
	}
	if !strings.Contains(string(content), "id:") {
		t.Error("normalized note missing generated id")
	}
	if !strings.Contains(string(content), "type: note") {
		t.Error("normalized note missing type")
	}
	if !strings.Contains(string(content), "project: myproject") {
		t.Error("normalized note missing project")
	}
	if !strings.Contains(string(content), "tags:") {
		t.Error("normalized note missing tags")
	}
}

func TestRunImportIndexesFiles(t *testing.T) {
	vp, database := setupTestVaultWithDB(t)
	defer database.Close()
	vaultPath = vp

	sourceDir := t.TempDir()
	if err := os.WriteFile(filepath.Join(sourceDir, "indexed.md"), []byte("---\nid: imp_indexed\ntype: note\ntitle: Indexed Note\n---\n\nContent here."), 0644); err != nil {
		t.Fatalf("failed to write source file: %v", err)
	}

	importMode = "copy"
	importProject = ""
	importTags = ""
	importKeepStructure = false
	importDryRun = false

	cmd := cobraCommandArgs([]string{"markdown", sourceDir})
	if err := runImport(cmd, []string{"markdown", sourceDir}); err != nil {
		t.Fatalf("runImport returned error: %v", err)
	}

	rows, err := database.Query("SELECT id FROM notes WHERE id = ?", "imp_indexed")
	if err != nil {
		t.Fatalf("failed to query notes: %v", err)
	}
	defer rows.Close()
	if !rows.Next() {
		t.Error("expected imported note to be indexed in database")
	}
}

package importers

import (
	"os"
	"path/filepath"
	"strings"
	"testing"
)

func TestMarkdownImporter_Name(t *testing.T) {
	m := &MarkdownImporter{}
	if m.Name() != "markdown" {
		t.Errorf("Expected name 'markdown', got '%s'", m.Name())
	}
}

func TestMarkdownImporter_Description(t *testing.T) {
	m := &MarkdownImporter{}
	if m.Description() != "Import a folder of Markdown files" {
		t.Errorf("Unexpected description: %s", m.Description())
	}
}

func TestMarkdownImporter_ImportSingleFile(t *testing.T) {
	srcDir := t.TempDir()
	vaultDir := t.TempDir()

	// Create source markdown file
	srcFile := filepath.Join(srcDir, "test-note.md")
	content := `---
id: test-001
type: note
title: Test Note
tags:
  - tag1
---

This is the body of the note.
`
	if err := os.WriteFile(srcFile, []byte(content), 0644); err != nil {
		t.Fatal(err)
	}

	// Import
	m := &MarkdownImporter{}
	opts := ImportOptions{
		SourcePath:  srcDir,
		TargetVault: vaultDir,
		Mode:        "copy",
	}
	result, err := m.Import(opts)
	if err != nil {
		t.Fatalf("Import failed: %v", err)
	}

	if result.FilesImported != 1 {
		t.Errorf("Expected 1 file imported, got %d", result.FilesImported)
	}
	if result.FilesSkipped != 0 {
		t.Errorf("Expected 0 files skipped, got %d", result.FilesSkipped)
	}
	if len(result.Errors) != 0 {
		t.Errorf("Expected 0 errors, got %d: %v", len(result.Errors), result.Errors)
	}

	// Verify file exists in target vault
	targetFile := filepath.Join(vaultDir, "10-notes", "test-note.md")
	if _, err := os.Stat(targetFile); os.IsNotExist(err) {
		t.Errorf("Target file should exist: %s", targetFile)
	}

	// Verify content
	imported, err := os.ReadFile(targetFile)
	if err != nil {
		t.Fatal(err)
	}
	if !strings.Contains(string(imported), "Test Note") {
		t.Errorf("Imported file should contain title")
	}
}

func TestMarkdownImporter_ImportMultipleFiles(t *testing.T) {
	srcDir := t.TempDir()
	vaultDir := t.TempDir()

	// Create multiple source files
	files := map[string]string{
		"note1.md": "---\nid: n1\ntype: note\ntitle: Note One\n---\n\nBody one.\n",
		"note2.md": "---\nid: n2\ntype: note\ntitle: Note Two\n---\n\nBody two.\n",
		"subdir/note3.md": "---\nid: n3\ntype: note\ntitle: Note Three\n---\n\nBody three.\n",
	}

	for name, content := range files {
		path := filepath.Join(srcDir, name)
		if err := os.MkdirAll(filepath.Dir(path), 0755); err != nil {
			t.Fatal(err)
		}
		if err := os.WriteFile(path, []byte(content), 0644); err != nil {
			t.Fatal(err)
		}
	}

	m := &MarkdownImporter{}
	opts := ImportOptions{
		SourcePath:  srcDir,
		TargetVault: vaultDir,
		Mode:        "copy",
	}
	result, err := m.Import(opts)
	if err != nil {
		t.Fatalf("Import failed: %v", err)
	}

	if result.FilesImported != 3 {
		t.Errorf("Expected 3 files imported, got %d", result.FilesImported)
	}

	// Verify all files exist
	for name := range files {
		targetFile := filepath.Join(vaultDir, "10-notes", filepath.Base(name))
		if _, err := os.Stat(targetFile); os.IsNotExist(err) {
			t.Errorf("Target file should exist: %s", targetFile)
		}
	}
}

func TestMarkdownImporter_SkipNonMarkdownFiles(t *testing.T) {
	srcDir := t.TempDir()
	vaultDir := t.TempDir()

	// Create a markdown file and a non-markdown file
	if err := os.WriteFile(filepath.Join(srcDir, "note.md"), []byte("---\ntitle: Note\n---\n\nBody\n"), 0644); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(srcDir, "readme.txt"), []byte("This is a text file"), 0644); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(srcDir, "image.png"), []byte("fake-image-data"), 0644); err != nil {
		t.Fatal(err)
	}

	m := &MarkdownImporter{}
	opts := ImportOptions{
		SourcePath:  srcDir,
		TargetVault: vaultDir,
		Mode:        "copy",
	}
	result, err := m.Import(opts)
	if err != nil {
		t.Fatalf("Import failed: %v", err)
	}

	if result.FilesImported != 1 {
		t.Errorf("Expected 1 file imported (only .md), got %d", result.FilesImported)
	}

	// Verify only .md file exists in target
	targetMd := filepath.Join(vaultDir, "10-notes", "note.md")
	if _, err := os.Stat(targetMd); os.IsNotExist(err) {
		t.Errorf("Target .md file should exist")
	}

	targetTxt := filepath.Join(vaultDir, "10-notes", "readme.txt")
	if _, err := os.Stat(targetTxt); !os.IsNotExist(err) {
		t.Errorf("Target .txt file should NOT exist")
	}
}

func TestMarkdownImporter_NormalizeMode(t *testing.T) {
	srcDir := t.TempDir()
	vaultDir := t.TempDir()

	// Create a markdown file with minimal frontmatter
	srcFile := filepath.Join(srcDir, "minimal.md")
	content := `---
title: Minimal Note
---

# Heading

Some body content here.
`
	if err := os.WriteFile(srcFile, []byte(content), 0644); err != nil {
		t.Fatal(err)
	}

	m := &MarkdownImporter{}
	opts := ImportOptions{
		SourcePath:     srcDir,
		TargetVault:    vaultDir,
		Mode:           "normalize",
		DefaultProject: "testproject",
		Tags:           []string{"imported"},
	}
	result, err := m.Import(opts)
	if err != nil {
		t.Fatalf("Import failed: %v", err)
	}

	if result.FilesImported != 1 {
		t.Errorf("Expected 1 file imported, got %d", result.FilesImported)
	}

	// Verify normalized content
	targetFile := filepath.Join(vaultDir, "10-notes", "minimal.md")
	imported, err := os.ReadFile(targetFile)
	if err != nil {
		t.Fatal(err)
	}

	importedStr := string(imported)
	if !strings.Contains(importedStr, "id: ") {
		t.Errorf("Normalized file should have an 'id' field")
	}
	if !strings.Contains(importedStr, "type: note") {
		t.Errorf("Normalized file should have type=note")
	}
	if !strings.Contains(importedStr, "project: testproject") {
		t.Errorf("Normalized file should have project=testproject")
	}
	if !strings.Contains(importedStr, "imported") {
		t.Errorf("Normalized file should have imported tag")
	}
	if !strings.Contains(importedStr, "created: 2") {
		t.Errorf("Normalized file should have created date")
	}
	if !strings.Contains(importedStr, "updated: 2") {
		t.Errorf("Normalized file should have updated date")
	}
}

func TestMarkdownImporter_InvalidSourcePath(t *testing.T) {
	m := &MarkdownImporter{}
	opts := ImportOptions{
		SourcePath:  "/nonexistent/path/that/does/not/exist",
		TargetVault: t.TempDir(),
		Mode:        "copy",
	}
	_, err := m.Import(opts)
	if err == nil {
		t.Error("Expected error for non-existent source path")
	}
}

func TestMarkdownImporter_KeepStructure(t *testing.T) {
	srcDir := t.TempDir()
	vaultDir := t.TempDir()

	// Create files in subdirectories
	content := "---\ntitle: Sub Note\n---\n\nBody\n"
	subDir := filepath.Join(srcDir, "projects", "alpha")
	if err := os.MkdirAll(subDir, 0755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(subDir, "note.md"), []byte(content), 0644); err != nil {
		t.Fatal(err)
	}

	m := &MarkdownImporter{}
	opts := ImportOptions{
		SourcePath:    srcDir,
		TargetVault:   vaultDir,
		Mode:          "copy",
		KeepStructure: true,
	}
	result, err := m.Import(opts)
	if err != nil {
		t.Fatalf("Import failed: %v", err)
	}

	if result.FilesImported != 1 {
		t.Errorf("Expected 1 file imported, got %d", result.FilesImported)
	}

	// Verify file preserves structure
	targetFile := filepath.Join(vaultDir, "10-notes", "projects", "alpha", "note.md")
	if _, err := os.Stat(targetFile); os.IsNotExist(err) {
		t.Errorf("Target file should preserve structure: %s", targetFile)
	}
}

func TestMarkdownImporter_SkipHiddenDirectories(t *testing.T) {
	srcDir := t.TempDir()
	vaultDir := t.TempDir()

	// Create a file in a hidden directory
	if err := os.WriteFile(filepath.Join(srcDir, "visible.md"), []byte("---\ntitle: Visible\n---\n\nBody\n"), 0644); err != nil {
		t.Fatal(err)
	}
	if err := os.MkdirAll(filepath.Join(srcDir, ".hidden"), 0755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(srcDir, ".hidden", "secret.md"), []byte("---\ntitle: Secret\n---\n\nBody\n"), 0644); err != nil {
		t.Fatal(err)
	}

	m := &MarkdownImporter{}
	opts := ImportOptions{
		SourcePath:  srcDir,
		TargetVault: vaultDir,
		Mode:        "copy",
	}
	result, err := m.Import(opts)
	if err != nil {
		t.Fatalf("Import failed: %v", err)
	}

	if result.FilesImported != 1 {
		t.Errorf("Expected 1 file imported (skipping hidden dir), got %d", result.FilesImported)
	}

	// Verify hidden directory file was NOT imported
	hiddenTarget := filepath.Join(vaultDir, "10-notes", ".hidden", "secret.md")
	if _, err := os.Stat(hiddenTarget); !os.IsNotExist(err) {
		t.Errorf("Hidden directory file should NOT be imported")
	}
}

func TestExtractTitle(t *testing.T) {
	tests := []struct {
		body     string
		expected string
	}{
		{"# My Title\n\nSome body.", "My Title"},
		{"Some body without heading.", "Untitled"},
		{"\n\n# Another Title\nBody", "Another Title"},
		{"# Title with spaces \nBody", "Title with spaces"},
	}

	for _, tt := range tests {
		got := extractTitle(tt.body)
		if got != tt.expected {
			t.Errorf("extractTitle(%q) = %q, want %q", tt.body, got, tt.expected)
		}
	}
}

func TestMergeTags(t *testing.T) {
	tests := []struct {
		existing []string
		newTags  []string
		expected []string
	}{
		{[]string{"a", "b"}, []string{"c"}, []string{"a", "b", "c"}},
		{[]string{"a", "b"}, []string{"a", "c"}, []string{"a", "b", "c"}},
		{nil, []string{"a"}, []string{"a"}},
		{[]string{"a"}, nil, []string{"a"}},
	}

	for _, tt := range tests {
		got := mergeTags(tt.existing, tt.newTags)
		if len(got) != len(tt.expected) {
			t.Errorf("mergeTags(%v, %v) = %v, want %v", tt.existing, tt.newTags, got, tt.expected)
		}
		for i, v := range tt.expected {
			if i >= len(got) || got[i] != v {
				t.Errorf("mergeTags(%v, %v) = %v, want %v", tt.existing, tt.newTags, got, tt.expected)
				break
			}
		}
	}
}

func TestCollisionSafePath(t *testing.T) {
	tmpDir := t.TempDir()
	existingFile := filepath.Join(tmpDir, "test.md")
	if err := os.WriteFile(existingFile, []byte("existing"), 0644); err != nil {
		t.Fatal(err)
	}

	// Should return a path with _1 appended
	safe := CollisionSafePath(existingFile)
	if safe == existingFile {
		t.Error("CollisionSafePath should return a different path when file exists")
	}
	if !strings.HasSuffix(safe, "_1.md") {
		t.Errorf("Expected suffix _1.md, got: %s", safe)
	}

	// Non-existing file should return same path
	nonExisting := filepath.Join(tmpDir, "nonexistent.md")
	safe2 := CollisionSafePath(nonExisting)
	if safe2 != nonExisting {
		t.Errorf("CollisionSafePath should return same path for non-existing file")
	}
}

func TestMarkdownImporter_SkipDuplicateContent(t *testing.T) {
	srcDir := t.TempDir()
	vaultDir := t.TempDir()

	content := "---\ntitle: Duplicate Test\n---\n\nBody content here.\n"
	// Create same file in source and target
	if err := os.WriteFile(filepath.Join(srcDir, "dup.md"), []byte(content), 0644); err != nil {
		t.Fatal(err)
	}
	if err := os.MkdirAll(filepath.Join(vaultDir, "10-notes"), 0755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(vaultDir, "10-notes", "dup.md"), []byte(content), 0644); err != nil {
		t.Fatal(err)
	}

	m := &MarkdownImporter{}
	opts := ImportOptions{
		SourcePath:  srcDir,
		TargetVault: vaultDir,
		Mode:        "copy",
	}
	result, err := m.Import(opts)
	if err != nil {
		t.Fatalf("Import failed: %v", err)
	}

	if result.FilesImported != 0 {
		t.Errorf("Expected 0 files imported (duplicate content), got %d", result.FilesImported)
	}
	if result.FilesSkipped != 1 {
		t.Errorf("Expected 1 file skipped (duplicate content), got %d", result.FilesSkipped)
	}
}

func TestIsHiddenDir(t *testing.T) {
	tests := []struct {
		name   string
		hidden bool
	}{
		{".obsidian", true},
		{".git", true},
		{".hidden", true},
		{"notes", false},
		{"10-notes", false},
		{".", false}, // current dir is not hidden
	}

	for _, tt := range tests {
		got := IsHiddenDir(tt.name)
		if got != tt.hidden {
			t.Errorf("IsHiddenDir(%q) = %v, want %v", tt.name, got, tt.hidden)
		}
	}
}

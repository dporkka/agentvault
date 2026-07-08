package importers

import (
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/agentvault/core/internal/markdown"
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
		"note1.md":        "---\nid: n1\ntype: note\ntitle: Note One\n---\n\nBody one.\n",
		"note2.md":        "---\nid: n2\ntype: note\ntitle: Note Two\n---\n\nBody two.\n",
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

func TestMarkdownImporter_DryRun(t *testing.T) {
	srcDir := t.TempDir()
	vaultDir := t.TempDir()

	content := `---
title: Dry Run Note
---

Body content.
`
	if err := os.WriteFile(filepath.Join(srcDir, "note.md"), []byte(content), 0644); err != nil {
		t.Fatal(err)
	}

	m := &MarkdownImporter{}
	opts := ImportOptions{
		SourcePath:  srcDir,
		TargetVault: vaultDir,
		Mode:        "copy",
		DryRun:      true,
	}
	result, err := m.Import(opts)
	if err != nil {
		t.Fatalf("Import failed: %v", err)
	}

	if result.FilesImported != 1 {
		t.Errorf("Expected 1 file to be planned, got %d", result.FilesImported)
	}
	if len(result.PlannedWrites) != 1 {
		t.Errorf("Expected 1 planned write, got %d", len(result.PlannedWrites))
	}

	// Verify nothing was written
	targetFile := filepath.Join(vaultDir, "10-notes", "note.md")
	if _, err := os.Stat(targetFile); !os.IsNotExist(err) {
		t.Errorf("Dry run should not write target file: %s", targetFile)
	}
}

func TestMarkdownImporter_DryRunNormalize(t *testing.T) {
	srcDir := t.TempDir()
	vaultDir := t.TempDir()

	content := `---
title: Minimal
---

# Heading
`
	if err := os.WriteFile(filepath.Join(srcDir, "minimal.md"), []byte(content), 0644); err != nil {
		t.Fatal(err)
	}

	m := &MarkdownImporter{}
	opts := ImportOptions{
		SourcePath:  srcDir,
		TargetVault: vaultDir,
		Mode:        "normalize",
		DryRun:      true,
		Tags:        []string{"imported"},
	}
	result, err := m.Import(opts)
	if err != nil {
		t.Fatalf("Import failed: %v", err)
	}

	if result.FilesImported != 1 {
		t.Errorf("Expected 1 file to be planned, got %d", result.FilesImported)
	}
	if result.NormalizedCount != 1 {
		t.Errorf("Expected 1 normalized file, got %d", result.NormalizedCount)
	}

	targetFile := filepath.Join(vaultDir, "10-notes", "minimal.md")
	if _, err := os.Stat(targetFile); !os.IsNotExist(err) {
		t.Errorf("Dry run should not write target file: %s", targetFile)
	}
}

func TestMarkdownImporter_DryRunDuplicate(t *testing.T) {
	srcDir := t.TempDir()
	vaultDir := t.TempDir()

	content := "---\ntitle: Dup\n---\n\nBody.\n"
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
		DryRun:      true,
	}
	result, err := m.Import(opts)
	if err != nil {
		t.Fatalf("Import failed: %v", err)
	}

	if result.FilesImported != 0 {
		t.Errorf("Expected 0 files to be planned, got %d", result.FilesImported)
	}
	if result.DuplicateCount != 1 {
		t.Errorf("Expected 1 duplicate, got %d", result.DuplicateCount)
	}
	if len(result.PlannedWrites) != 0 {
		t.Errorf("Expected 0 planned writes for duplicate, got %d", len(result.PlannedWrites))
	}
}

func TestMarkdownImporter_MarkdownExtension(t *testing.T) {
	srcDir := t.TempDir()
	vaultDir := t.TempDir()

	content := "---\ntitle: Markdown Extension\n---\n\nBody.\n"
	if err := os.WriteFile(filepath.Join(srcDir, "note.markdown"), []byte(content), 0644); err != nil {
		t.Fatal(err)
	}

	m := &MarkdownImporter{}
	result, err := m.Import(ImportOptions{SourcePath: srcDir, TargetVault: vaultDir, Mode: "copy"})
	if err != nil {
		t.Fatalf("Import failed: %v", err)
	}
	if result.FilesImported != 1 {
		t.Errorf("Expected 1 file imported, got %d", result.FilesImported)
	}

	targetFile := filepath.Join(vaultDir, "10-notes", "note.markdown")
	if _, err := os.Stat(targetFile); os.IsNotExist(err) {
		t.Errorf("Target file should exist: %s", targetFile)
	}
}

func TestMarkdownImporter_ParseFailure(t *testing.T) {
	srcDir := t.TempDir()
	vaultDir := t.TempDir()

	content := "---\ntags: [unclosed\n---\n\nBody.\n"
	if err := os.WriteFile(filepath.Join(srcDir, "bad.md"), []byte(content), 0644); err != nil {
		t.Fatal(err)
	}

	m := &MarkdownImporter{}
	result, err := m.Import(ImportOptions{SourcePath: srcDir, TargetVault: vaultDir, Mode: "copy"})
	if err != nil {
		t.Fatalf("Import failed: %v", err)
	}
	if result.FilesImported != 0 {
		t.Errorf("Expected 0 files imported, got %d", result.FilesImported)
	}
	if result.FilesSkipped != 1 {
		t.Errorf("Expected 1 file skipped, got %d", result.FilesSkipped)
	}
	if len(result.Errors) != 1 {
		t.Errorf("Expected 1 error, got %d: %v", len(result.Errors), result.Errors)
	}
}

func TestMarkdownImporter_SourceNotDirectory(t *testing.T) {
	srcFile := filepath.Join(t.TempDir(), "notadir.md")
	if err := os.WriteFile(srcFile, []byte("---\ntitle: X\n---\n"), 0644); err != nil {
		t.Fatal(err)
	}

	m := &MarkdownImporter{}
	_, err := m.Import(ImportOptions{SourcePath: srcFile, TargetVault: t.TempDir(), Mode: "copy"})
	if err == nil {
		t.Fatal("Expected error when source path is a file")
	}
	if !strings.Contains(err.Error(), "is not a directory") {
		t.Errorf("Unexpected error message: %v", err)
	}
}

func TestMarkdownImporter_TargetVaultIsFile(t *testing.T) {
	srcDir := t.TempDir()
	vaultFile := filepath.Join(t.TempDir(), "vault")
	if err := os.WriteFile(vaultFile, []byte("x"), 0644); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(srcDir, "note.md"), []byte("---\ntitle: X\n---\n"), 0644); err != nil {
		t.Fatal(err)
	}

	m := &MarkdownImporter{}
	_, err := m.Import(ImportOptions{SourcePath: srcDir, TargetVault: vaultFile, Mode: "copy"})
	if err == nil {
		t.Fatal("Expected error when target vault path is a file")
	}
	if !strings.Contains(err.Error(), "failed to create target vault") {
		t.Errorf("Unexpected error message: %v", err)
	}
}

func TestDetermineBaseTargetPath(t *testing.T) {
	m := &MarkdownImporter{}
	srcRoot := "/source"

	t.Run("flat", func(t *testing.T) {
		opts := ImportOptions{SourcePath: srcRoot, TargetVault: "/vault", KeepStructure: false}
		got, err := m.determineBaseTargetPath("/source/sub/note.md", opts)
		if err != nil {
			t.Fatal(err)
		}
		want := filepath.Join("/vault", "10-notes", "note.md")
		if got != want {
			t.Errorf("got %q, want %q", got, want)
		}
	})

	t.Run("keep structure", func(t *testing.T) {
		opts := ImportOptions{SourcePath: srcRoot, TargetVault: "/vault", KeepStructure: true}
		got, err := m.determineBaseTargetPath("/source/sub/note.md", opts)
		if err != nil {
			t.Fatal(err)
		}
		want := filepath.Join("/vault", "10-notes", "sub", "note.md")
		if got != want {
			t.Errorf("got %q, want %q", got, want)
		}
	})
}

func TestNormalizeFrontmatter_PreservesExisting(t *testing.T) {
	doc := &markdown.ParsedDocument{
		Frontmatter: markdown.Frontmatter{
			ID:            "id-1",
			Type:          "page",
			Title:         "Existing Title",
			Status:        "active",
			Project:       "proj",
			Created:       "2024-01-01T00:00:00Z",
			Updated:       "2024-01-02T00:00:00Z",
			SourceQuality: "high",
			Tags:          []string{"a"},
			Entities:      []string{"e"},
		},
		Body: "body",
	}

	normalizeFrontmatter(doc, ImportOptions{DefaultProject: "other", Tags: []string{"a", "b"}})

	if doc.Frontmatter.ID != "id-1" {
		t.Errorf("ID changed: %q", doc.Frontmatter.ID)
	}
	if doc.Frontmatter.Type != "page" {
		t.Errorf("Type changed: %q", doc.Frontmatter.Type)
	}
	if doc.Frontmatter.Title != "Existing Title" {
		t.Errorf("Title changed: %q", doc.Frontmatter.Title)
	}
	if doc.Frontmatter.Project != "proj" {
		t.Errorf("Project changed: %q", doc.Frontmatter.Project)
	}
	wantTags := []string{"a", "b"}
	if !slicesEqual(doc.Frontmatter.Tags, wantTags) {
		t.Errorf("Tags = %v, want %v", doc.Frontmatter.Tags, wantTags)
	}
}

func TestNormalizeFrontmatter_EmptyTitleNoHeading(t *testing.T) {
	doc := &markdown.ParsedDocument{Frontmatter: markdown.Frontmatter{}, Body: "just body"}
	normalizeFrontmatter(doc, ImportOptions{})

	if doc.Frontmatter.Title != "Untitled" {
		t.Errorf("Title = %q, want Untitled", doc.Frontmatter.Title)
	}
	if !strings.HasPrefix(doc.Frontmatter.ID, "note_untitled") {
		t.Errorf("ID = %q, want prefix note_untitled", doc.Frontmatter.ID)
	}
	if doc.Frontmatter.Type != "note" {
		t.Errorf("Type = %q, want note", doc.Frontmatter.Type)
	}
}

func TestFrontmatterChanged(t *testing.T) {
	base := markdown.Frontmatter{
		ID:            "id",
		Type:          "note",
		Title:         "title",
		Status:        "status",
		Project:       "project",
		Created:       "created",
		Updated:       "updated",
		SourceQuality: "quality",
		Tags:          []string{"a"},
		Entities:      []string{"b"},
	}

	cases := []struct {
		name    string
		mutator func(*markdown.Frontmatter)
	}{
		{"ID", func(f *markdown.Frontmatter) { f.ID = "changed" }},
		{"Type", func(f *markdown.Frontmatter) { f.Type = "changed" }},
		{"Title", func(f *markdown.Frontmatter) { f.Title = "changed" }},
		{"Status", func(f *markdown.Frontmatter) { f.Status = "changed" }},
		{"Project", func(f *markdown.Frontmatter) { f.Project = "changed" }},
		{"Created", func(f *markdown.Frontmatter) { f.Created = "changed" }},
		{"Updated", func(f *markdown.Frontmatter) { f.Updated = "changed" }},
		{"SourceQuality", func(f *markdown.Frontmatter) { f.SourceQuality = "changed" }},
		{"Tags", func(f *markdown.Frontmatter) { f.Tags = []string{"changed"} }},
		{"Entities", func(f *markdown.Frontmatter) { f.Entities = []string{"changed"} }},
	}

	for _, tc := range cases {
		t.Run(tc.name+" changed", func(t *testing.T) {
			updated := base
			tc.mutator(&updated)
			if !frontmatterChanged(base, updated) {
				t.Error("expected frontmatterChanged to be true")
			}
		})
	}

	if frontmatterChanged(base, base) {
		t.Error("expected frontmatterChanged to be false for identical frontmatter")
	}
}

func TestSlicesEqual(t *testing.T) {
	if !slicesEqual(nil, nil) {
		t.Error("nil slices should be equal")
	}
	if !slicesEqual([]string{}, []string{}) {
		t.Error("empty slices should be equal")
	}
	if !slicesEqual([]string{"a", "b"}, []string{"a", "b"}) {
		t.Error("identical slices should be equal")
	}
	if slicesEqual([]string{"a"}, []string{"a", "b"}) {
		t.Error("different length slices should not be equal")
	}
	if slicesEqual([]string{"a"}, []string{"b"}) {
		t.Error("different element slices should not be equal")
	}
}

func TestRenderDocument_AllFields(t *testing.T) {
	doc := &markdown.ParsedDocument{
		Frontmatter: markdown.Frontmatter{
			ID:            "id-1",
			Type:          "note",
			Title:         "Title",
			Status:        "active",
			Project:       "proj",
			Tags:          []string{"a", "b"},
			Entities:      []string{"e1"},
			Created:       "2024-01-01T00:00:00Z",
			Updated:       "2024-01-02T00:00:00Z",
			SourceQuality: "high",
			Extra: map[string]interface{}{
				"title":  "should be skipped",
				"custom": "kept",
			},
		},
		Body: "body text",
	}

	rendered := renderDocument(doc)
	checks := []string{
		"id: id-1",
		"type: note",
		"title: Title",
		"status: active",
		"project: proj",
		"tags:",
		"  - a",
		"  - b",
		"entities:",
		"  - e1",
		"created: 2024-01-01T00:00:00Z",
		"updated: 2024-01-02T00:00:00Z",
		"source_quality: high",
		"custom: kept",
		"body text",
	}
	for _, want := range checks {
		if !strings.Contains(rendered, want) {
			t.Errorf("rendered document missing %q:\n%s", want, rendered)
		}
	}
	if strings.Contains(rendered, "title: should be skipped") {
		t.Error("rendered document should not duplicate core frontmatter keys from Extra")
	}
}

func TestShouldSkipIfDuplicate(t *testing.T) {
	tmp := t.TempDir()
	src := filepath.Join(tmp, "src.md")
	tgt := filepath.Join(tmp, "tgt.md")

	t.Run("target does not exist", func(t *testing.T) {
		if err := os.WriteFile(src, []byte("abc"), 0644); err != nil {
			t.Fatal(err)
		}
		skip, err := shouldSkipIfDuplicate(src, filepath.Join(tmp, "missing.md"))
		if err != nil {
			t.Fatal(err)
		}
		if skip {
			t.Error("should not skip when target does not exist")
		}
	})

	t.Run("different size", func(t *testing.T) {
		if err := os.WriteFile(src, []byte("abc"), 0644); err != nil {
			t.Fatal(err)
		}
		if err := os.WriteFile(tgt, []byte("ab"), 0644); err != nil {
			t.Fatal(err)
		}
		skip, err := shouldSkipIfDuplicate(src, tgt)
		if err != nil {
			t.Fatal(err)
		}
		if skip {
			t.Error("should not skip when sizes differ")
		}
	})

	t.Run("same size different content", func(t *testing.T) {
		if err := os.WriteFile(src, []byte("abc"), 0644); err != nil {
			t.Fatal(err)
		}
		if err := os.WriteFile(tgt, []byte("xyz"), 0644); err != nil {
			t.Fatal(err)
		}
		skip, err := shouldSkipIfDuplicate(src, tgt)
		if err != nil {
			t.Fatal(err)
		}
		if skip {
			t.Error("should not skip when content differs")
		}
	})

	t.Run("identical content", func(t *testing.T) {
		if err := os.WriteFile(src, []byte("abc"), 0644); err != nil {
			t.Fatal(err)
		}
		if err := os.WriteFile(tgt, []byte("abc"), 0644); err != nil {
			t.Fatal(err)
		}
		skip, err := shouldSkipIfDuplicate(src, tgt)
		if err != nil {
			t.Fatal(err)
		}
		if !skip {
			t.Error("should skip identical content")
		}
	})
}

func TestCopyFile(t *testing.T) {
	t.Run("success", func(t *testing.T) {
		src := filepath.Join(t.TempDir(), "src.md")
		dst := filepath.Join(t.TempDir(), "dst.md")
		if err := os.WriteFile(src, []byte("hello"), 0644); err != nil {
			t.Fatal(err)
		}
		if err := copyFile(src, dst); err != nil {
			t.Fatalf("copyFile failed: %v", err)
		}
		got, err := os.ReadFile(dst)
		if err != nil {
			t.Fatal(err)
		}
		if string(got) != "hello" {
			t.Errorf("copied content = %q, want hello", got)
		}
	})

	t.Run("source missing", func(t *testing.T) {
		if err := copyFile(filepath.Join(t.TempDir(), "missing.md"), filepath.Join(t.TempDir(), "dst.md")); err == nil {
			t.Error("expected error for missing source")
		}
	})

	t.Run("destination parent is file", func(t *testing.T) {
		src := filepath.Join(t.TempDir(), "src.md")
		if err := os.WriteFile(src, []byte("hello"), 0644); err != nil {
			t.Fatal(err)
		}
		badParent := filepath.Join(t.TempDir(), "bad")
		if err := os.WriteFile(badParent, []byte("x"), 0644); err != nil {
			t.Fatal(err)
		}
		dst := filepath.Join(badParent, "dst.md")
		if err := copyFile(src, dst); err == nil {
			t.Error("expected error when destination parent is a file")
		}
	})
}

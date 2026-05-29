package importers

import (
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/agentvault/core/internal/markdown"
)

func TestObsidianImporter_Name(t *testing.T) {
	o := &ObsidianImporter{}
	if o.Name() != "obsidian" {
		t.Errorf("Expected name 'obsidian', got '%s'", o.Name())
	}
}

func TestObsidianImporter_Description(t *testing.T) {
	o := &ObsidianImporter{}
	expected := "Import an Obsidian vault"
	if o.Description() != expected {
		t.Errorf("Expected description %q, got %q", expected, o.Description())
	}
}

func TestObsidianImporter_ImportVaultStructure(t *testing.T) {
	srcDir := t.TempDir()
	vaultDir := t.TempDir()

	// Create Obsidian vault structure
	files := map[string]string{
		"daily/2024-01-01.md": "---\ntitle: Daily Note\naliases: [\"Jan 1st\"]\n---\n\n# Daily Note\n\nToday I worked on #project-alpha.\n\nSee [[Another Note]] for context.",
		"projects/idea.md":    "---\ntitle: Big Idea\ntags: [\"idea\"]\n---\n\n# Big Idea\n\nThis is a #brainstorm session.\n\nLink to [[daily/2024-01-01|January 1st note]]",
		"inbox/todo.md":       "---\ntitle: Todo List\n---\n\n- [ ] Task 1 #urgent\n- [ ] Task 2 #later",
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

	o := &ObsidianImporter{}
	opts := ImportOptions{
		SourcePath:  srcDir,
		TargetVault: vaultDir,
		Mode:        "copy",
	}
	result, err := o.Import(opts)
	if err != nil {
		t.Fatalf("Import failed: %v", err)
	}

	if result.FilesImported != 3 {
		t.Errorf("Expected 3 files imported, got %d", result.FilesImported)
	}
	if len(result.Errors) != 0 {
		t.Errorf("Expected 0 errors, got %d: %v", len(result.Errors), result.Errors)
	}

	// Verify files exist in target
	for name := range files {
		targetFile := filepath.Join(vaultDir, "10-notes", filepath.Base(name))
		if _, err := os.Stat(targetFile); os.IsNotExist(err) {
			t.Errorf("Target file should exist: %s", targetFile)
		}
	}
}

func TestObsidianImporter_SkipObsidianFolder(t *testing.T) {
	srcDir := t.TempDir()
	vaultDir := t.TempDir()

	// Create regular note
	if err := os.WriteFile(filepath.Join(srcDir, "note.md"), []byte("---\ntitle: Note\n---\n\nBody\n"), 0644); err != nil {
		t.Fatal(err)
	}

	// Create .obsidian config folder
	obsidianDir := filepath.Join(srcDir, ".obsidian")
	if err := os.MkdirAll(obsidianDir, 0755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(obsidianDir, "app.json"), []byte("{\"theme\":\"dark\"}"), 0644); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(obsidianDir, "config.md"), []byte("---\nobsidian: true\n---\n\nConfig"), 0644); err != nil {
		t.Fatal(err)
	}

	// Create .trash folder
	trashDir := filepath.Join(srcDir, ".trash")
	if err := os.MkdirAll(trashDir, 0755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(trashDir, "deleted.md"), []byte("---\ntitle: Deleted\n---\n\nBody\n"), 0644); err != nil {
		t.Fatal(err)
	}

	o := &ObsidianImporter{}
	opts := ImportOptions{
		SourcePath:  srcDir,
		TargetVault: vaultDir,
		Mode:        "copy",
	}
	result, err := o.Import(opts)
	if err != nil {
		t.Fatalf("Import failed: %v", err)
	}

	if result.FilesImported != 1 {
		t.Errorf("Expected 1 file imported (skipping .obsidian and .trash), got %d", result.FilesImported)
	}

	// Verify .obsidian files were NOT imported
	obsidianTarget := filepath.Join(vaultDir, "10-notes", ".obsidian")
	if _, err := os.Stat(obsidianTarget); !os.IsNotExist(err) {
		t.Errorf(".obsidian folder should NOT be imported")
	}

	// Verify .trash files were NOT imported
	trashTarget := filepath.Join(vaultDir, "10-notes", ".trash")
	if _, err := os.Stat(trashTarget); !os.IsNotExist(err) {
		t.Errorf(".trash folder should NOT be imported")
	}
}

func TestObsidianImporter_ExtractInlineTags(t *testing.T) {
	body := `This is a note with #project-alpha and #idea tags.

Some other text with #urgent tag.

Hex colors like #FF00AB and #abc should be ignored.

Duplicate #idea tag here.
`

	tags := extractInlineTags(body)

	expected := map[string]bool{
		"project-alpha": true,
		"idea":          true,
		"urgent":        true,
	}

	if len(tags) != len(expected) {
		t.Errorf("Expected %d tags, got %d: %v", len(expected), len(tags), tags)
	}

	for _, tag := range tags {
		if !expected[tag] {
			t.Errorf("Unexpected tag: %s", tag)
		}
	}

	// Ensure hex colors are NOT included
	for _, tag := range tags {
		if tag == "FF00AB" || tag == "abc" {
			t.Errorf("Hex color should not be extracted as tag: %s", tag)
		}
	}
}

func TestObsidianImporter_AliasesExtraction(t *testing.T) {
	body := "---\ntitle: My Note\naliases: [\"First Alias\", \"Second Alias\"]\n---\n\nBody content with [[Another Page]].\n"

	doc, err := markdown.ParseBytes([]byte(body))
	if err != nil {
		t.Fatalf("Failed to parse: %v", err)
	}

	aliases := extractAliases(doc)
	if len(aliases) != 2 {
		t.Errorf("Expected 2 aliases, got %d: %v", len(aliases), aliases)
	}

	expected := []string{"First Alias", "Second Alias"}
	for i, alias := range expected {
		if i >= len(aliases) || aliases[i] != alias {
			t.Errorf("Expected alias %q at index %d, got %v", alias, i, aliases)
			break
		}
	}
}

func TestObsidianImporter_WikiLinkPreservation(t *testing.T) {
	srcDir := t.TempDir()
	vaultDir := t.TempDir()

	body := "---\ntitle: Wiki Note\n---\n\nThis note links to [[Another Note]] and [[Third Note|with a label]].\n"
	if err := os.WriteFile(filepath.Join(srcDir, "wiki.md"), []byte(body), 0644); err != nil {
		t.Fatal(err)
	}

	o := &ObsidianImporter{}
	opts := ImportOptions{
		SourcePath:  srcDir,
		TargetVault: vaultDir,
		Mode:        "copy",
	}
	result, err := o.Import(opts)
	if err != nil {
		t.Fatalf("Import failed: %v", err)
	}

	if result.FilesImported != 1 {
		t.Errorf("Expected 1 file imported, got %d", result.FilesImported)
	}

	// Verify wiki links are preserved
	targetFile := filepath.Join(vaultDir, "10-notes", "wiki.md")
	content, err := os.ReadFile(targetFile)
	if err != nil {
		t.Fatal(err)
	}

	contentStr := string(content)
	if !strings.Contains(contentStr, "[[Another Note]]") {
		t.Errorf("Wiki link [[Another Note]] should be preserved")
	}
	if !strings.Contains(contentStr, "[[Third Note|with a label]]") {
		t.Errorf("Wiki link with label should be preserved")
	}
}

func TestObsidianImporter_TagsAddedToFrontmatter(t *testing.T) {
	srcDir := t.TempDir()
	vaultDir := t.TempDir()

	body := "---\ntitle: Tagged Note\ntags:\n  - existing\n---\n\nThis has #newtag and #another-tag in the body.\n"
	if err := os.WriteFile(filepath.Join(srcDir, "tagged.md"), []byte(body), 0644); err != nil {
		t.Fatal(err)
	}

	o := &ObsidianImporter{}
	opts := ImportOptions{
		SourcePath:  srcDir,
		TargetVault: vaultDir,
		Mode:        "copy",
		Tags:        []string{"imported"},
	}
	result, err := o.Import(opts)
	if err != nil {
		t.Fatalf("Import failed: %v", err)
	}

	if result.FilesImported != 1 {
		t.Errorf("Expected 1 file imported, got %d", result.FilesImported)
	}

	// Verify tags in frontmatter include inline tags
	targetFile := filepath.Join(vaultDir, "10-notes", "tagged.md")
	content, err := os.ReadFile(targetFile)
	if err != nil {
		t.Fatal(err)
	}

	contentStr := string(content)
	if !strings.Contains(contentStr, "newtag") {
		t.Errorf("Inline tag 'newtag' should be in frontmatter")
	}
	if !strings.Contains(contentStr, "another-tag") {
		t.Errorf("Inline tag 'another-tag' should be in frontmatter")
	}
	if !strings.Contains(contentStr, "existing") {
		t.Errorf("Original tag 'existing' should be preserved")
	}
}

func TestObsidianImporter_CopyAttachments(t *testing.T) {
	srcDir := t.TempDir()
	vaultDir := t.TempDir()

	// Create a markdown file referencing an attachment
	if err := os.WriteFile(filepath.Join(srcDir, "note.md"), []byte("---\ntitle: Note with Image\n---\n\n![[image.png]]\n"), 0644); err != nil {
		t.Fatal(err)
	}

	// Create a fake attachment
	if err := os.WriteFile(filepath.Join(srcDir, "image.png"), []byte("fake-png-data"), 0644); err != nil {
		t.Fatal(err)
	}

	// Also create a PDF
	if err := os.WriteFile(filepath.Join(srcDir, "doc.pdf"), []byte("fake-pdf-data"), 0644); err != nil {
		t.Fatal(err)
	}

	o := &ObsidianImporter{}
	opts := ImportOptions{
		SourcePath:  srcDir,
		TargetVault: vaultDir,
		Mode:        "copy",
	}
	result, err := o.Import(opts)
	if err != nil {
		t.Fatalf("Import failed: %v", err)
	}

	if result.FilesImported != 1 {
		t.Errorf("Expected 1 markdown file imported, got %d", result.FilesImported)
	}

	// Verify attachments were copied
	attachPng := filepath.Join(vaultDir, "10-notes", "attachments", "image.png")
	if _, err := os.Stat(attachPng); os.IsNotExist(err) {
		t.Errorf("PNG attachment should be copied to vault: %s", attachPng)
	}

	attachPdf := filepath.Join(vaultDir, "10-notes", "attachments", "doc.pdf")
	if _, err := os.Stat(attachPdf); os.IsNotExist(err) {
		t.Errorf("PDF attachment should be copied to vault: %s", attachPdf)
	}
}

func TestObsidianImporter_InvalidSourcePath(t *testing.T) {
	o := &ObsidianImporter{}
	opts := ImportOptions{
		SourcePath:  "/nonexistent/path/that/does/not/exist",
		TargetVault: t.TempDir(),
		Mode:        "copy",
	}
	_, err := o.Import(opts)
	if err == nil {
		t.Error("Expected error for non-existent source path")
	}
}

func TestObsidianImporter_KeepStructure(t *testing.T) {
	srcDir := t.TempDir()
	vaultDir := t.TempDir()

	content := "---\ntitle: Sub Note\n---\n\nBody with #tag.\n"
	subDir := filepath.Join(srcDir, "areas", "work")
	if err := os.MkdirAll(subDir, 0755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(subDir, "note.md"), []byte(content), 0644); err != nil {
		t.Fatal(err)
	}

	o := &ObsidianImporter{}
	opts := ImportOptions{
		SourcePath:    srcDir,
		TargetVault:   vaultDir,
		Mode:          "copy",
		KeepStructure: true,
	}
	result, err := o.Import(opts)
	if err != nil {
		t.Fatalf("Import failed: %v", err)
	}

	if result.FilesImported != 1 {
		t.Errorf("Expected 1 file imported, got %d", result.FilesImported)
	}

	// Verify structure is preserved
	targetFile := filepath.Join(vaultDir, "10-notes", "areas", "work", "note.md")
	if _, err := os.Stat(targetFile); os.IsNotExist(err) {
		t.Errorf("Target file should preserve structure: %s", targetFile)
	}
}

func TestIsHexColor(t *testing.T) {
	tests := []struct {
		input    string
		expected bool
	}{
		{"FF00AB", true},
		{"abc", true},
		{"ABC", true},
		{"123", true},
		{"123456", true},
		{"aabbcc", true},
		{"project", false},
		{"tag1", false},
		{"my-tag", false},
		{"ff", false},
		{"fffff", false},
	}

	for _, tt := range tests {
		got := isHexColor(tt.input)
		if got != tt.expected {
			t.Errorf("isHexColor(%q) = %v, want %v", tt.input, got, tt.expected)
		}
	}
}

func TestIsAttachment(t *testing.T) {
	tests := []struct {
		ext      string
		expected bool
	}{
		{".png", true},
		{".jpg", true},
		{".jpeg", true},
		{".gif", true},
		{".pdf", true},
		{".docx", true},
		{".mp4", true},
		{".md", false},
		{".markdown", false},
		{".txt", false},
		{".go", false},
	}

	for _, tt := range tests {
		got := isAttachment(tt.ext)
		if got != tt.expected {
			t.Errorf("isAttachment(%q) = %v, want %v", tt.ext, got, tt.expected)
		}
	}
}

func TestObsidianImporter_EmptyVault(t *testing.T) {
	srcDir := t.TempDir()
	vaultDir := t.TempDir()

	o := &ObsidianImporter{}
	opts := ImportOptions{
		SourcePath:  srcDir,
		TargetVault: vaultDir,
		Mode:        "copy",
	}
	result, err := o.Import(opts)
	if err != nil {
		t.Fatalf("Import failed: %v", err)
	}

	if result.FilesImported != 0 {
		t.Errorf("Expected 0 files imported for empty vault, got %d", result.FilesImported)
	}
	if len(result.Errors) != 0 {
		t.Errorf("Expected 0 errors for empty vault, got %d", len(result.Errors))
	}
}

func TestObsidianImporter_AliasAsString(t *testing.T) {
	body := "---\ntitle: Single Alias\naliases: \"My Alias\"\n---\n\nBody content.\n"

	doc, err := markdown.ParseBytes([]byte(body))
	if err != nil {
		t.Fatalf("Failed to parse: %v", err)
	}

	aliases := extractAliases(doc)
	if len(aliases) != 1 {
		t.Errorf("Expected 1 alias, got %d: %v", len(aliases), aliases)
	}
	if len(aliases) > 0 && aliases[0] != "My Alias" {
		t.Errorf("Expected alias 'My Alias', got %q", aliases[0])
	}
}

package markdown

import (
	"os"
	"path/filepath"
	"strings"
	"testing"
)

func TestParseBytes(t *testing.T) {
	content := `---
id: note_2024_01_01_001
type: note
title: Test Note
status: active
project: myproject
tags:
  - tag1
  - tag2
---

This is the body.

[[Another Note]]
[[Another Note|with label]]
`

	doc, err := ParseBytes([]byte(content))
	if err != nil {
		t.Fatalf("ParseBytes failed: %v", err)
	}

	if doc.Frontmatter.ID != "note_2024_01_01_001" {
		t.Errorf("Expected ID 'note_2024_01_01_001', got '%s'", doc.Frontmatter.ID)
	}
	if doc.Frontmatter.Title != "Test Note" {
		t.Errorf("Expected Title 'Test Note', got '%s'", doc.Frontmatter.Title)
	}
	if doc.Frontmatter.Type != "note" {
		t.Errorf("Expected Type 'note', got '%s'", doc.Frontmatter.Type)
	}
	if !strings.Contains(doc.Body, "This is the body.") {
		t.Errorf("Body should contain 'This is the body.', got: %s", doc.Body)
	}
	if len(doc.WikiLinks) != 2 {
		t.Errorf("Expected 2 wiki links, got %d", len(doc.WikiLinks))
	}
}

func TestExtractWikiLinks(t *testing.T) {
	body := "See [[Target A]] and [[Target B|Label B]] for more."
	links := ExtractWikiLinks(body)
	if len(links) != 2 {
		t.Fatalf("Expected 2 links, got %d", len(links))
	}
	if links[0].Target != "Target A" {
		t.Errorf("Expected target 'Target A', got '%s'", links[0].Target)
	}
	if links[1].Label != "Label B" {
		t.Errorf("Expected label 'Label B', got '%s'", links[1].Label)
	}
}

func TestParseFile(t *testing.T) {
	tmpDir := t.TempDir()
	path := filepath.Join(tmpDir, "test.md")
	content := `---
id: test_001
type: note
title: File Test
---

Body content here.
`
	if err := os.WriteFile(path, []byte(content), 0644); err != nil {
		t.Fatal(err)
	}

	doc, err := ParseFile(path)
	if err != nil {
		t.Fatalf("ParseFile failed: %v", err)
	}
	if doc.Frontmatter.Title != "File Test" {
		t.Errorf("Expected title 'File Test', got '%s'", doc.Frontmatter.Title)
	}
}

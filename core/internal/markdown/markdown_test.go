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

func TestRenderMarkdown(t *testing.T) {
	html, err := RenderMarkdown("Hello world")
	if err != nil {
		t.Fatalf("RenderMarkdown failed: %v", err)
	}
	if !strings.Contains(html, "Hello world") {
		t.Errorf("Expected rendered HTML to contain input text, got %q", html)
	}
}

func TestParseBytesNoFrontmatter(t *testing.T) {
	content := "Just a plain markdown body.\n\nNo frontmatter here."
	doc, err := ParseBytes([]byte(content))
	if err != nil {
		t.Fatalf("ParseBytes failed: %v", err)
	}
	if doc.Body != content {
		t.Errorf("Expected body to equal input when no frontmatter, got %q", doc.Body)
	}
	if doc.RawFrontmatter != "" {
		t.Errorf("Expected empty raw frontmatter, got %q", doc.RawFrontmatter)
	}
}

func TestParseBytesInvalidYAML(t *testing.T) {
	content := `---
foo: [unclosed
---
body
`
	_, err := ParseBytes([]byte(content))
	if err == nil {
		t.Error("Expected error for invalid YAML frontmatter")
	}
}

func TestParseFilesInDir(t *testing.T) {
	tmpDir := t.TempDir()
	if err := os.WriteFile(filepath.Join(tmpDir, "a.md"), []byte("---\nid: a\n---\nA"), 0644); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(tmpDir, "b.md"), []byte("---\nid: b\n---\nB"), 0644); err != nil {
		t.Fatal(err)
	}

	results, err := ParseFilesInDir(tmpDir)
	if err != nil {
		t.Fatalf("ParseFilesInDir failed: %v", err)
	}
	if len(results) != 2 {
		t.Errorf("Expected 2 documents, got %d", len(results))
	}
	if results["a.md"] == nil || results["a.md"].Frontmatter.ID != "a" {
		t.Error("Expected document a")
	}
}

func TestExtractWikiLinksDeduplicates(t *testing.T) {
	body := "See [[Note A]] and [[Note A]] again."
	links := ExtractWikiLinks(body)
	if len(links) != 1 {
		t.Errorf("Expected 1 unique wiki link, got %d", len(links))
	}
}

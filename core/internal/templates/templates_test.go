package templates

import (
	"strings"
	"testing"
	"time"
)

func TestRenderNoteTemplate(t *testing.T) {
	data := TemplateData{
		ID:      "note_2026_05_29_001",
		Title:   "My Test Note",
		Project: "",
		Tags:    []string{"idea", "test"},
		Created: "2026-05-29T10:00:00Z",
	}

	result, err := Render("note", data)
	if err != nil {
		t.Fatalf("Render note template: %v", err)
	}

	if !strings.Contains(result, "id: note_2026_05_29_001") {
		t.Errorf("Expected ID in rendered output")
	}
	if !strings.Contains(result, "type: note") {
		t.Errorf("Expected type in rendered output")
	}
	if !strings.Contains(result, "title: My Test Note") {
		t.Errorf("Expected title in rendered output")
	}
	if !strings.Contains(result, "# My Test Note") {
		t.Errorf("Expected H1 heading in body")
	}
	if !strings.Contains(result, `tags: [idea, test]`) {
		t.Errorf("Expected tags in rendered output, got:\n%s", result)
	}
	if !strings.Contains(result, "## Notes") {
		t.Errorf("Expected ## Notes section in body")
	}
}

func TestRenderNoteWithoutTags(t *testing.T) {
	data := TemplateData{
		ID:      "note_2026_05_29_002",
		Title:   "Simple Note",
		Created: "2026-05-29T10:00:00Z",
	}

	result, err := Render("note", data)
	if err != nil {
		t.Fatalf("Render note template: %v", err)
	}

	// Tags line should not appear when Tags is empty
	lines := strings.Split(result, "\n")
	for _, line := range lines {
		if strings.HasPrefix(line, "tags:") {
			t.Errorf("Tags should not appear when empty, found: %q", line)
		}
	}
}

func TestRenderDecisionTemplateWithProject(t *testing.T) {
	data := TemplateData{
		ID:      "dec_2026_05_29_001",
		Title:   "Use Annual Contracts",
		Project: "adacavo",
		Created: "2026-05-29T10:00:00Z",
	}

	result, err := Render("decision", data)
	if err != nil {
		t.Fatalf("Render decision template: %v", err)
	}

	if !strings.Contains(result, "type: decision") {
		t.Errorf("Expected type: decision")
	}
	if !strings.Contains(result, "project: adacavo") {
		t.Errorf("Expected project in rendered output")
	}
	if !strings.Contains(result, "## Decision") {
		t.Errorf("Expected ## Decision section")
	}
	if !strings.Contains(result, "## Reasoning") {
		t.Errorf("Expected ## Reasoning section")
	}
	if !strings.Contains(result, "## Tradeoffs") {
		t.Errorf("Expected ## Tradeoffs section")
	}
}

func TestGenerateIDFormat(t *testing.T) {
	// Test known types
	tests := []struct {
		noteType   string
		wantPrefix string
	}{
		{"note", "note_"},
		{"decision", "dec_"},
		{"task", "task_"},
		{"meeting", "mtg_"},
		{"source", "src_"},
		{"project", "prj_"},
		{"prompt", "prm_"},
		{"capture", "cap_"},
	}

	for _, tt := range tests {
		t.Run(tt.noteType, func(t *testing.T) {
			id := GenerateID(tt.noteType)
			if !strings.HasPrefix(id, tt.wantPrefix) {
				t.Errorf("GenerateID(%q) = %q, want prefix %q", tt.noteType, id, tt.wantPrefix)
			}

			// Check format: {abbrev}_{YYYY}_{MM}_{DD}_{NNN}
			parts := strings.Split(id, "_")
			if len(parts) != 5 {
				t.Errorf("GenerateID(%q) = %q, expected 5 underscore-separated parts, got %d", tt.noteType, id, len(parts))
				return
			}

			// Check year is 4 digits and reasonable
			if len(parts[1]) != 4 {
				t.Errorf("Year part should be 4 digits, got %q", parts[1])
			}

			// Check month is 2 digits
			if len(parts[2]) != 2 {
				t.Errorf("Month part should be 2 digits, got %q", parts[2])
			}

			// Check day is 2 digits
			if len(parts[3]) != 2 {
				t.Errorf("Day part should be 2 digits, got %q", parts[3])
			}

			// Check random suffix is 3 digits
			if len(parts[4]) != 3 {
				t.Errorf("Random suffix should be 3 digits, got %q", parts[4])
			}
		})
	}
}

func TestGenerateIDUnknownType(t *testing.T) {
	id := GenerateID("unknown")
	if !strings.HasPrefix(id, "unknown_") {
		t.Errorf("GenerateID(unknown) should use type as-is, got %q", id)
	}
}

func TestAvailableReturnsAllTemplates(t *testing.T) {
	names := Available()
	if len(names) != 6 {
		t.Errorf("Available() returned %d names, want 6", len(names))
	}

	expected := map[string]bool{
		"note":     false,
		"decision": false,
		"task":     false,
		"meeting":  false,
		"source":   false,
		"project":  false,
	}

	for _, name := range names {
		if _, ok := expected[name]; !ok {
			t.Errorf("Available() returned unexpected name: %q", name)
		}
		expected[name] = true
	}

	for name, found := range expected {
		if !found {
			t.Errorf("Available() missing expected template: %q", name)
		}
	}
}

func TestRenderUnknownTemplate(t *testing.T) {
	data := TemplateData{
		ID:      "test_001",
		Title:   "Test",
		Created: time.Now().Format(time.RFC3339),
	}

	_, err := Render("nonexistent", data)
	if err == nil {
		t.Fatal("Expected error for unknown template, got nil")
	}
	if !strings.Contains(err.Error(), "unknown template") {
		t.Errorf("Error should mention 'unknown template', got: %v", err)
	}
}

func TestGetFrontmatterAndBody(t *testing.T) {
	data := TemplateData{
		ID:      "note_2026_05_29_003",
		Title:   "Split Test",
		Tags:    []string{"test"},
		Created: "2026-05-29T10:00:00Z",
	}

	frontmatter, body, err := GetFrontmatterAndBody("note", data)
	if err != nil {
		t.Fatalf("GetFrontmatterAndBody: %v", err)
	}

	// Frontmatter should contain YAML fields
	if !strings.Contains(frontmatter, "id: note_2026_05_29_003") {
		t.Errorf("Frontmatter should contain ID")
	}
	if !strings.Contains(frontmatter, "title: Split Test") {
		t.Errorf("Frontmatter should contain title")
	}

	// Body should contain markdown content (without frontmatter wrapper)
	if strings.Contains(body, "---") {
		t.Errorf("Body should not contain --- delimiters")
	}
	if !strings.Contains(body, "# Split Test") {
		t.Errorf("Body should contain H1 heading")
	}
	if !strings.Contains(body, "## Notes") {
		t.Errorf("Body should contain ## Notes section")
	}
}

func TestRenderTaskTemplate(t *testing.T) {
	data := TemplateData{
		ID:      "task_2026_05_29_001",
		Title:   "Build MCP Server",
		Project: "agentvault",
		Created: "2026-05-29T10:00:00Z",
	}

	result, err := Render("task", data)
	if err != nil {
		t.Fatalf("Render task template: %v", err)
	}

	if !strings.Contains(result, "type: task") {
		t.Errorf("Expected type: task")
	}
	if !strings.Contains(result, "status: open") {
		t.Errorf("Expected status: open")
	}
	if !strings.Contains(result, "priority: medium") {
		t.Errorf("Expected priority: medium")
	}
	if !strings.Contains(result, "project: agentvault") {
		t.Errorf("Expected project: agentvault")
	}
	if !strings.Contains(result, "- [ ]") {
		t.Errorf("Expected unchecked checkbox for acceptance criteria")
	}
}

func TestRenderMeetingTemplate(t *testing.T) {
	data := TemplateData{
		ID:      "mtg_2026_05_29_001",
		Title:   "Client Call",
		Project: "adacavo",
		Created: "2026-05-29T10:00:00Z",
	}

	result, err := Render("meeting", data)
	if err != nil {
		t.Fatalf("Render meeting template: %v", err)
	}

	if !strings.Contains(result, "type: meeting") {
		t.Errorf("Expected type: meeting")
	}
	if !strings.Contains(result, "## Attendees") {
		t.Errorf("Expected ## Attendees section")
	}
	if !strings.Contains(result, "## Action items") {
		t.Errorf("Expected ## Action items section")
	}
}

func TestRenderSourceTemplate(t *testing.T) {
	data := TemplateData{
		ID:      "src_2026_05_29_001",
		Title:   "Market Report",
		URL:     "https://example.com/report",
		Created: "2026-05-29T10:00:00Z",
	}

	result, err := Render("source", data)
	if err != nil {
		t.Fatalf("Render source template: %v", err)
	}

	if !strings.Contains(result, "type: source") {
		t.Errorf("Expected type: source")
	}
	if !strings.Contains(result, "url: https://example.com/report") {
		t.Errorf("Expected URL in frontmatter")
	}
	if !strings.Contains(result, "source_type: website") {
		t.Errorf("Expected source_type: website")
	}
	if !strings.Contains(result, "## Original URL") {
		t.Errorf("Expected ## Original URL section")
	}
}

func TestRenderSourceWithoutURL(t *testing.T) {
	data := TemplateData{
		ID:      "src_2026_05_29_002",
		Title:   "Book Reference",
		Created: "2026-05-29T10:00:00Z",
	}

	result, err := Render("source", data)
	if err != nil {
		t.Fatalf("Render source template: %v", err)
	}

	// URL line should not appear when URL is empty
	lines := strings.Split(result, "\n")
	for _, line := range lines {
		if strings.HasPrefix(line, "url:") {
			t.Errorf("URL should not appear when empty, found: %q", line)
		}
	}
	if strings.Contains(result, "## Original URL") {
		t.Errorf("Original URL section should not appear when URL is empty")
	}
}

func TestRenderProjectTemplate(t *testing.T) {
	data := TemplateData{
		ID:      "prj_2026_05_29_001",
		Title:   "AgentVault",
		Created: "2026-05-29T10:00:00Z",
	}

	result, err := Render("project", data)
	if err != nil {
		t.Fatalf("Render project template: %v", err)
	}

	if !strings.Contains(result, "type: project") {
		t.Errorf("Expected type: project")
	}
	if !strings.Contains(result, "## Overview") {
		t.Errorf("Expected ## Overview section")
	}
	if !strings.Contains(result, "## Goals") {
		t.Errorf("Expected ## Goals section")
	}
}

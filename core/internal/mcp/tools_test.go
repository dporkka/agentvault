package mcp

import (
	"fmt"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/agentvault/core/internal/db"
)

func TestHandleSearch_Results(t *testing.T) {
	s, db := setupTestServer(t)
	defer db.Close()

	// Seed test data
	addTestNote(t, db, "note_2024_01_15_123", "Pricing Strategy", "10-notes/pricing.md", "note", "business", "Discussion about pricing models and strategies.", []string{"pricing", "business"})
	addTestNote(t, db, "dec_2024_01_15_456", "Use Postgres", "30-decisions/postgres.md", "decision", "tech", "Decision to use PostgreSQL as primary database.", []string{"database", "tech"})

	// Test search without FTS (empty query returns all)
	result, err := s.handleSearch(map[string]interface{}{
		"query":   "",
		"project": "business",
		"limit":   float64(10),
	})
	if err != nil {
		t.Fatalf("handleSearch error: %v", err)
	}

	if !strings.Contains(result, "Pricing Strategy") {
		t.Errorf("expected result to contain 'Pricing Strategy', got:\n%s", result)
	}
	if strings.Contains(result, "Use Postgres") {
		t.Errorf("did not expect 'Use Postgres' in business project results, got:\n%s", result)
	}
}

func TestHandleSearch_NoResults(t *testing.T) {
	s, db := setupTestServer(t)
	defer db.Close()

	result, err := s.handleSearch(map[string]interface{}{
		"query": "xyznonexistent123",
		"limit": float64(10),
	})
	if err != nil {
		t.Fatalf("handleSearch error: %v", err)
	}

	if !strings.Contains(result, "No results found") {
		t.Errorf("expected 'No results found', got:\n%s", result)
	}
}

func TestHandleReadNote_ByID(t *testing.T) {
	s, db := setupTestServer(t)
	defer db.Close()

	// Create a test note file
	noteDir := filepath.Join(s.vaultPath, "10-notes")
	os.MkdirAll(noteDir, 0755)
	noteContent := "---\nid: note_read_001\ntype: note\ntitle: Test Readable\nproject: testproj\n---\n\nThis is the test note body.\n"
	notePath := filepath.Join(noteDir, "test-readable.md")
	os.WriteFile(notePath, []byte(noteContent), 0644)

	// Seed the database
	addTestNote(t, db, "note_read_001", "Test Readable", "10-notes/test-readable.md", "note", "testproj", "This is the test note body.", []string{"test"})

	result, err := s.handleReadNote(map[string]interface{}{
		"id": "note_read_001",
	})
	if err != nil {
		t.Fatalf("handleReadNote error: %v", err)
	}

	if !strings.Contains(result, "Test Readable") {
		t.Errorf("expected result to contain 'Test Readable', got:\n%s", result)
	}
	if !strings.Contains(result, "This is the test note body") {
		t.Errorf("expected result to contain body text, got:\n%s", result)
	}
}

func TestHandleReadNote_NotFound(t *testing.T) {
	s, db := setupTestServer(t)
	defer db.Close()

	_, err := s.handleReadNote(map[string]interface{}{
		"id": "note_nonexistent_xyz",
	})
	if err == nil {
		t.Fatal("expected error for nonexistent note")
	}
	if !strings.Contains(err.Error(), "not found") {
		t.Errorf("expected 'not found' in error, got: %v", err)
	}
}

func TestHandleCreateNote(t *testing.T) {
	s, db := setupTestServer(t)
	defer db.Close()

	result, err := s.handleCreateNote(map[string]interface{}{
		"type":    "note",
		"title":   "My New Note",
		"project": "testproj",
		"tags":    []interface{}{"test", "demo"},
	})
	if err != nil {
		t.Fatalf("handleCreateNote error: %v", err)
	}

	if !strings.Contains(result, "Created note:") {
		t.Errorf("expected 'Created note:' in result, got:\n%s", result)
	}
	if !strings.Contains(result, "note") {
		t.Error("expected note type in result")
	}

	// Verify file was created
	noteDir := filepath.Join(s.vaultPath, "10-notes")
	entries, err := os.ReadDir(noteDir)
	if err != nil {
		t.Fatalf("read notes dir: %v", err)
	}
	if len(entries) == 0 {
		t.Error("expected note file to be created")
	}
}

func TestHandleCreateDecision(t *testing.T) {
	s, db := setupTestServer(t)
	defer db.Close()

	result, err := s.handleCreateDecision(map[string]interface{}{
		"title":   "Use Redis",
		"project": "infra",
		"tags":    []interface{}{"cache", "infra"},
	})
	if err != nil {
		t.Fatalf("handleCreateDecision error: %v", err)
	}

	if !strings.Contains(result, "Created note:") {
		t.Errorf("expected 'Created note:' in result, got:\n%s", result)
	}

	// Verify file was created in decisions folder
	decDir := filepath.Join(s.vaultPath, "30-decisions")
	entries, err := os.ReadDir(decDir)
	if err != nil {
		t.Fatalf("read decisions dir: %v", err)
	}
	if len(entries) == 0 {
		t.Error("expected decision file to be created")
	}
}

func TestHandleCreateTask(t *testing.T) {
	s, db := setupTestServer(t)
	defer db.Close()

	result, err := s.handleCreateTask(map[string]interface{}{
		"title":   "Implement search",
		"project": "backend",
	})
	if err != nil {
		t.Fatalf("handleCreateTask error: %v", err)
	}

	if !strings.Contains(result, "Created note:") {
		t.Errorf("expected 'Created note:' in result, got:\n%s", result)
	}
}

func TestHandleCreateNote_InvalidType(t *testing.T) {
	s, db := setupTestServer(t)
	defer db.Close()

	_, err := s.handleCreateNote(map[string]interface{}{
		"type":  "nonexistent",
		"title": "Test",
	})
	if err == nil {
		t.Fatal("expected error for invalid note type")
	}
}

func TestHandleCapture(t *testing.T) {
	s, db := setupTestServer(t)
	defer db.Close()

	result, err := s.handleCapture(map[string]interface{}{
		"title":      "Quick Idea",
		"text":       "This is a quick capture idea for later.",
		"source_url": "https://example.com",
		"project":    "ideas",
		"tags":       []interface{}{"idea", "quick"},
	})
	if err != nil {
		t.Fatalf("handleCapture error: %v", err)
	}

	if !strings.Contains(result, "Captured to inbox:") {
		t.Errorf("expected 'Captured to inbox:' in result, got:\n%s", result)
	}

	// Verify file was created in inbox
	inboxDir := filepath.Join(s.vaultPath, "00-inbox")
	entries, err := os.ReadDir(inboxDir)
	if err != nil {
		t.Fatalf("read inbox dir: %v", err)
	}
	if len(entries) == 0 {
		t.Error("expected capture file to be created in inbox")
	}

	// Verify content
	content, err := os.ReadFile(filepath.Join(inboxDir, entries[0].Name()))
	if err != nil {
		t.Fatalf("read capture file: %v", err)
	}
	if !strings.Contains(string(content), "Quick Idea") {
		t.Error("expected capture title in file content")
	}
	if !strings.Contains(string(content), "quick capture idea") {
		t.Error("expected capture text in file content")
	}
}

func TestHandleCapture_MissingTitle(t *testing.T) {
	s, db := setupTestServer(t)
	defer db.Close()

	_, err := s.handleCapture(map[string]interface{}{
		"text": "No title here",
	})
	if err == nil {
		t.Fatal("expected error for missing title")
	}
}

func TestHandleSummarize(t *testing.T) {
	s, db := setupTestServer(t)
	defer db.Close()

	// Create some test files in a folder
	summaryDir := filepath.Join(s.vaultPath, "50-summary-test")
	os.MkdirAll(summaryDir, 0755)

	files := []struct {
		name    string
		content string
	}{
		{
			"doc1.md",
			"---\nid: doc1\ntype: note\ntitle: First Document\nstatus: active\ntags: [a, b]\n---\n\nThis is the first document body.\n",
		},
		{
			"doc2.md",
			"---\nid: doc2\ntype: decision\ntitle: Second Document\nstatus: pending\ntags: [c]\n---\n\nThis is the second document with more content here.\n",
		},
	}

	for _, f := range files {
		os.WriteFile(filepath.Join(summaryDir, f.name), []byte(f.content), 0644)
	}

	result, err := s.handleSummarize(map[string]interface{}{
		"path": "50-summary-test",
	})
	if err != nil {
		t.Fatalf("handleSummarize error: %v", err)
	}

	if !strings.Contains(result, "First Document") {
		t.Errorf("expected 'First Document' in result, got:\n%s", result)
	}
	if !strings.Contains(result, "Second Document") {
		t.Errorf("expected 'Second Document' in result, got:\n%s", result)
	}
	if !strings.Contains(result, "**Files:** 2") {
		t.Errorf("expected '**Files:** 2' in result, got:\n%s", result)
	}
}

func TestHandleSummarize_EmptyFolder(t *testing.T) {
	s, db := setupTestServer(t)
	defer db.Close()

	// Create empty folder
	emptyDir := filepath.Join(s.vaultPath, "60-empty")
	os.MkdirAll(emptyDir, 0755)

	result, err := s.handleSummarize(map[string]interface{}{
		"path": "60-empty",
	})
	if err != nil {
		t.Fatalf("handleSummarize error: %v", err)
	}

	if !strings.Contains(result, "No markdown files found") {
		t.Errorf("expected 'No markdown files found', got:\n%s", result)
	}
}

func TestHandleSummarize_NonexistentPath(t *testing.T) {
	s, db := setupTestServer(t)
	defer db.Close()

	_, err := s.handleSummarize(map[string]interface{}{
		"path": "nonexistent-folder-xyz",
	})
	if err == nil {
		t.Fatal("expected error for nonexistent path")
	}
}

func TestHandleListProjects(t *testing.T) {
	s, db := setupTestServer(t)
	defer db.Close()

	// Seed test data with projects
	addTestNote(t, db, "proj1_001", "Note One", "notes/n1.md", "note", "alpha", "body1", []string{})
	addTestNote(t, db, "proj2_001", "Note Two", "notes/n2.md", "note", "alpha", "body2", []string{})
	addTestNote(t, db, "proj3_001", "Note Three", "notes/n3.md", "note", "beta", "body3", []string{})

	result, err := s.handleListProjects(map[string]interface{}{})
	if err != nil {
		t.Fatalf("handleListProjects error: %v", err)
	}

	if !strings.Contains(result, "alpha") {
		t.Errorf("expected 'alpha' in result, got:\n%s", result)
	}
	if !strings.Contains(result, "beta") {
		t.Errorf("expected 'beta' in result, got:\n%s", result)
	}
}

func TestHandleListRecent(t *testing.T) {
	s, db := setupTestServer(t)
	defer db.Close()

	// Seed test data
	addTestNote(t, db, "recent_001", "Recent Note", "notes/recent.md", "note", "test", "recent body", []string{})

	result, err := s.handleListRecent(map[string]interface{}{
		"limit": float64(5),
	})
	if err != nil {
		t.Fatalf("handleListRecent error: %v", err)
	}

	if !strings.Contains(result, "Recent Note") {
		t.Errorf("expected 'Recent Note' in result, got:\n%s", result)
	}
}

func TestHandleGitStatus_NotARepo(t *testing.T) {
	s, db := setupTestServer(t)
	defer db.Close()

	result, err := s.handleGitStatus(map[string]interface{}{})
	if err != nil {
		t.Fatalf("handleGitStatus error: %v", err)
	}

	if !strings.Contains(result, "Not a git repository") {
		t.Errorf("expected 'Not a git repository', got:\n%s", result)
	}
}

func TestHandleLogAgentRun(t *testing.T) {
	s, db := setupTestServer(t)
	defer db.Close()

	result, err := s.handleLogAgentRun(map[string]interface{}{
		"agent_name":    "test-agent",
		"task":          "run unit tests",
		"files_changed": []interface{}{"file1.go", "file2.go"},
	})
	if err != nil {
		t.Fatalf("handleLogAgentRun error: %v", err)
	}

	if !strings.Contains(result, "Logged agent run:") {
		t.Errorf("expected 'Logged agent run:' in result, got:\n%s", result)
	}
	if !strings.Contains(result, "test-agent") {
		t.Errorf("expected agent name in result, got:\n%s", result)
	}
	if !strings.Contains(result, "run unit tests") {
		t.Errorf("expected task in result, got:\n%s", result)
	}
	if !strings.Contains(result, "2") {
		t.Errorf("expected files count in result, got:\n%s", result)
	}
}

func TestHandleLogAgentRun_MissingFields(t *testing.T) {
	s, db := setupTestServer(t)
	defer db.Close()

	_, err := s.handleLogAgentRun(map[string]interface{}{
		"agent_name": "test",
	})
	if err == nil {
		t.Fatal("expected error for missing task")
	}

	_, err = s.handleLogAgentRun(map[string]interface{}{
		"task": "test",
	})
	if err == nil {
		t.Fatal("expected error for missing agent_name")
	}
}

func TestSanitizeFilename(t *testing.T) {
	cases := []struct {
		input    string
		expected string
	}{
		{"Hello World", "hello-world"},
		{"Special!@#Chars", "specialchars"},
		{"  spaces  ", "spaces"},
		{"a--b---c", "a-b-c"},
		{"", "untitled"},
		{"UPPERCASE", "uppercase"},
		{"Mixed123-Text", "mixed123-text"},
	}

	for _, c := range cases {
		result := sanitizeFilename(c.input)
		if result != c.expected {
			t.Errorf("sanitizeFilename(%q) = %q, want %q", c.input, result, c.expected)
		}
	}
}

func TestFolderForType(t *testing.T) {
	cases := []struct {
		noteType string
		project  string
		expected string
	}{
		{"note", "", "10-notes"},
		{"decision", "p1", "30-decisions"},
		{"task", "", "10-notes"},
		{"meeting", "proj", "20-projects/proj"},
		{"meeting", "", "10-notes"},
		{"source", "", "40-research"},
		{"capture", "", "00-inbox"},
		{"unknown", "", "10-notes"},
	}

	for _, c := range cases {
		result := folderForType(c.noteType, c.project, "/vault")
		expected := filepath.Join("/vault", c.expected)
		if result != expected {
			t.Errorf("folderForType(%q, %q) = %q, want %q", c.noteType, c.project, result, expected)
		}
	}
}

func TestSchemaHelpers(t *testing.T) {
	// Test schemaString
	s := schemaString("A description")
	if s["type"] != "string" {
		t.Errorf("expected type=string, got %v", s["type"])
	}
	if s["description"] != "A description" {
		t.Errorf("expected description, got %v", s["description"])
	}

	// Test schemaStringEnum
	senum := schemaStringEnum("Type", []string{"a", "b"})
	if _, ok := senum["enum"]; !ok {
		t.Error("expected enum key")
	}

	// Test schemaInt
	si := schemaInt("Limit", 10)
	if si["type"] != "integer" {
		t.Errorf("expected type=integer, got %v", si["type"])
	}
	if si["default"] != 10 {
		t.Errorf("expected default=10, got %v", si["default"])
	}

	// Test schemaStringArray
	sa := schemaStringArray("Tags")
	if sa["type"] != "array" {
		t.Errorf("expected type=array, got %v", sa["type"])
	}

	// Test makeSchema
	schema := makeSchema(map[string]interface{}{
		"name": schemaString("Name"),
	}, []string{"name"})
	if schema["type"] != "object" {
		t.Errorf("expected schema type=object, got %v", schema["type"])
	}
	props, ok := schema["properties"].(map[string]interface{})
	if !ok || props["name"] == nil {
		t.Error("expected properties with name")
	}
	required, ok := schema["required"].([]string)
	if !ok || len(required) != 1 || required[0] != "name" {
		t.Errorf("expected required=[name], got %v", schema["required"])
	}
}

func TestCurrentTimestamp(t *testing.T) {
	ts := currentTimestamp()
	if ts == "" {
		t.Error("expected non-empty timestamp")
	}
	// Should be a valid RFC3339-ish timestamp
	if !strings.Contains(ts, "T") {
		t.Errorf("expected RFC3339 format with 'T', got: %s", ts)
	}
}

func TestMakeSchema_NoRequired(t *testing.T) {
	schema := makeSchema(map[string]interface{}{
		"opt": schemaString("Optional"),
	}, nil)
	if _, hasRequired := schema["required"]; hasRequired {
		t.Error("expected no required key when nil")
	}
}

// Benchmark tool registration
func BenchmarkRegisterTools(b *testing.B) {
	tmpDir := b.TempDir()
	os.MkdirAll(filepath.Join(tmpDir, ".agentvault"), 0755)
	database, err := db.Open(tmpDir)
	if err != nil {
		b.Fatalf("open db: %v", err)
	}
	defer database.Close()

	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		srv := NewServer(tmpDir, database)
		srv.RegisterTools()
	}
}

func TestHandleSearch_TypeFilter(t *testing.T) {
	s, db := setupTestServer(t)
	defer db.Close()

	addTestNote(t, db, "type_note_001", "A Note", "notes/n1.md", "note", "p1", "body", []string{})
	addTestNote(t, db, "type_dec_001", "A Decision", "decisions/d1.md", "decision", "p1", "body", []string{})

	result, err := s.handleSearch(map[string]interface{}{
		"query": "",
		"type":  "decision",
		"limit": float64(10),
	})
	if err != nil {
		t.Fatalf("handleSearch error: %v", err)
	}

	if !strings.Contains(result, "A Decision") {
		t.Errorf("expected 'A Decision' in result, got:\n%s", result)
	}
	if strings.Contains(result, "A Note") {
		t.Errorf("did not expect 'A Note' when filtering by decision type, got:\n%s", result)
	}
}

func TestHandleSearch_Limit(t *testing.T) {
	s, db := setupTestServer(t)
	defer db.Close()

	// Add many notes
	for i := 0; i < 20; i++ {
		id := fmt.Sprintf("limit_%02d", i)
		addTestNote(t, db, id, fmt.Sprintf("Note %d", i), fmt.Sprintf("notes/n%d.md", i), "note", "", fmt.Sprintf("body %d", i), []string{})
	}

	result, err := s.handleSearch(map[string]interface{}{
		"query": "",
		"limit": float64(5),
	})
	if err != nil {
		t.Fatalf("handleSearch error: %v", err)
	}

	// Should be limited to 5 results (non-FTS returns everything with limit)
	// We just check it doesn't crash and returns valid markdown
	if !strings.Contains(result, "Search Results") {
		t.Errorf("expected 'Search Results' header, got:\n%s", result)
	}
}

func TestHandleCapture_OnlyTitle(t *testing.T) {
	s, db := setupTestServer(t)
	defer db.Close()

	result, err := s.handleCapture(map[string]interface{}{
		"title": "Minimal Capture",
	})
	if err != nil {
		t.Fatalf("handleCapture error: %v", err)
	}

	if !strings.Contains(result, "Captured to inbox:") {
		t.Errorf("expected capture result, got:\n%s", result)
	}
}

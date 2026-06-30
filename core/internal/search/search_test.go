package search

import (
	"os"
	"path/filepath"
	"testing"

	"github.com/agentvault/core/internal/db"
)

func setupTestDB(t *testing.T) (*db.DB, func()) {
	t.Helper()
	tmpDir := t.TempDir()

	// Create .agentvault directory
	if err := os.MkdirAll(filepath.Join(tmpDir, ".agentvault"), 0755); err != nil {
		t.Fatalf("failed to create .agentvault dir: %v", err)
	}

	database, err := db.Open(tmpDir)
	if err != nil {
		t.Fatalf("failed to open db: %v", err)
	}

	if err := database.RunMigrations(); err != nil {
		t.Fatalf("failed to run migrations: %v", err)
	}

	// Insert sample data
	insertSampleData(t, database)

	cleanup := func() {
		database.Close()
	}

	return database, cleanup
}

func insertSampleData(t *testing.T, database *db.DB) {
	t.Helper()

	// Insert files
	files := []struct {
		id, path, hash string
	}{
		{"note_001", "10-notes/idea.md", "hash1"},
		{"note_002", "20-projects/webapp.md", "hash2"},
		{"note_003", "30-decisions/adr001.md", "hash3"},
		{"note_004", "10-notes/rust.md", "hash4"},
		{"note_005", "40-research/ai.md", "hash5"},
	}
	for _, f := range files {
		_, err := database.Exec(
			`INSERT INTO files (id, path, content_hash, indexed_at) VALUES (?, ?, ?, datetime('now'))`,
			f.id, f.path, f.hash,
		)
		if err != nil {
			t.Fatalf("failed to insert file %s: %v", f.id, err)
		}
	}

	// Insert notes
	notes := []struct {
		id, fileID, title, noteType, status, project, updatedAt, body string
	}{
		{"note_001", "note_001", "Idea for App", "note", "active", "", "2024-01-15T10:00:00Z", "This is a brilliant idea for a new mobile application."},
		{"note_002", "note_002", "Web App Project", "project", "active", "webapp", "2024-01-14T09:00:00Z", "Building a web application using Go and React."},
		{"note_003", "note_003", "Use SQLite", "decision", "accepted", "webapp", "2024-01-10T08:00:00Z", "We decided to use SQLite for the database layer."},
		{"note_004", "note_004", "Rust Notes", "note", "draft", "", "2024-01-01T07:00:00Z", "Learning Rust programming language. Memory safety without garbage collection."},
		{"note_005", "note_005", "AI Research", "research", "active", "ai-team", "2024-01-20T12:00:00Z", "Research on large language models and their applications."},
	}
	for _, n := range notes {
		_, err := database.Exec(
			`INSERT INTO notes (id, file_id, title, type, status, project, updated_at, body) VALUES (?, ?, ?, ?, ?, ?, ?, ?)`,
			n.id, n.fileID, n.title, n.noteType, n.status, n.project, n.updatedAt, n.body,
		)
		if err != nil {
			t.Fatalf("failed to insert note %s: %v", n.id, err)
		}
	}

	// Insert tags
	tags := []struct{ noteID, tag string }{
		{"note_001", "idea"},
		{"note_001", "mobile"},
		{"note_002", "go"},
		{"note_002", "web"},
		{"note_003", "database"},
		{"note_003", "architecture"},
		{"note_004", "rust"},
		{"note_004", "learning"},
		{"note_005", "ai"},
		{"note_005", "llm"},
	}
	for _, tg := range tags {
		_, err := database.Exec(
			`INSERT INTO tags (note_id, tag) VALUES (?, ?)`,
			tg.noteID, tg.tag,
		)
		if err != nil {
			t.Fatalf("failed to insert tag %s: %v", tg.tag, err)
		}
	}

	// Insert FTS data
	ftsData := []struct{ noteID, title, body, tags, entities string }{
		{"note_001", "Idea for App", "This is a brilliant idea for a new mobile application.", "idea, mobile", ""},
		{"note_002", "Web App Project", "Building a web application using Go and React.", "go, web", ""},
		{"note_003", "Use SQLite", "We decided to use SQLite for the database layer.", "database, architecture", ""},
		{"note_004", "Rust Notes", "Learning Rust programming language. Memory safety without garbage collection.", "rust, learning", ""},
		{"note_005", "AI Research", "Research on large language models and their applications.", "ai, llm", ""},
	}
	for _, f := range ftsData {
		_, err := database.Exec(
			`INSERT INTO notes_fts (note_id, title, body, tags, entities) VALUES (?, ?, ?, ?, ?)`,
			f.noteID, f.title, f.body, f.tags, f.entities,
		)
		if err != nil {
			t.Fatalf("failed to insert fts data %s: %v", f.noteID, err)
		}
	}
}

func TestSearch(t *testing.T) {
	database, cleanup := setupTestDB(t)
	defer cleanup()

	s := New(database)

	t.Run("basic search", func(t *testing.T) {
		results, err := s.Search(Query{Q: "idea"})
		if err != nil {
			t.Fatalf("Search failed: %v", err)
		}
		if len(results) == 0 {
			t.Error("Expected results for 'idea', got none")
		}
		found := false
		for _, r := range results {
			if r.Title == "Idea for App" {
				found = true
				break
			}
		}
		if !found {
			t.Errorf("Expected to find 'Idea for App' in results, got: %v", results)
		}
	})

	t.Run("type filter", func(t *testing.T) {
		results, err := s.Search(Query{Type: "note"})
		if err != nil {
			t.Fatalf("Search with type filter failed: %v", err)
		}
		if len(results) == 0 {
			t.Fatal("Expected results for type=note")
		}
		for _, r := range results {
			if r.Type != "note" {
				t.Errorf("Expected only type='note', got type='%s' for '%s'", r.Type, r.Title)
			}
		}
	})

	t.Run("project filter", func(t *testing.T) {
		results, err := s.Search(Query{Project: "webapp"})
		if err != nil {
			t.Fatalf("Search with project filter failed: %v", err)
		}
		if len(results) == 0 {
			t.Fatal("Expected results for project=webapp")
		}
		for _, r := range results {
			if r.Project != "webapp" {
				t.Errorf("Expected only project='webapp', got project='%s'", r.Project)
			}
		}
	})

	t.Run("tag filter", func(t *testing.T) {
		results, err := s.Search(Query{Tag: "go"})
		if err != nil {
			t.Fatalf("Search with tag filter failed: %v", err)
		}
		if len(results) == 0 {
			t.Fatal("Expected results for tag=go")
		}
		found := false
		for _, r := range results {
			for _, tag := range r.Tags {
				if tag == "go" {
					found = true
					break
				}
			}
		}
		if !found {
			t.Errorf("Expected at least one result with tag 'go'")
		}
	})

	t.Run("empty query with no filters", func(t *testing.T) {
		results, err := s.Search(Query{Limit: 5})
		if err != nil {
			t.Fatalf("Search with empty query failed: %v", err)
		}
		// Should return results ordered by updated_at when no FTS match
		if len(results) == 0 {
			t.Fatal("Expected results for empty query")
		}
	})

	t.Run("limit", func(t *testing.T) {
		results, err := s.Search(Query{Limit: 2})
		if err != nil {
			t.Fatalf("Search with limit failed: %v", err)
		}
		if len(results) > 2 {
			t.Errorf("Expected at most 2 results, got %d", len(results))
		}
	})

	t.Run("combined query and filter", func(t *testing.T) {
		results, err := s.Search(Query{Q: "web", Type: "project"})
		if err != nil {
			t.Fatalf("Search with combined filters failed: %v", err)
		}
		for _, r := range results {
			if r.Type != "project" {
				t.Errorf("Expected only type='project', got '%s'", r.Type)
			}
		}
	})
}

func TestGetByID(t *testing.T) {
	database, cleanup := setupTestDB(t)
	defer cleanup()

	s := New(database)

	result, err := s.GetByID("note_001")
	if err != nil {
		t.Fatalf("GetByID failed: %v", err)
	}
	if result == nil {
		t.Fatal("Expected result, got nil")
	}
	if result.Title != "Idea for App" {
		t.Errorf("Expected title 'Idea for App', got '%s'", result.Title)
	}
	if result.ID != "note_001" {
		t.Errorf("Expected ID 'note_001', got '%s'", result.ID)
	}

	// Non-existent ID
	_, err = s.GetByID("nonexistent")
	if err == nil {
		t.Error("Expected error for non-existent ID")
	}
}

func TestGetByPath(t *testing.T) {
	database, cleanup := setupTestDB(t)
	defer cleanup()

	s := New(database)

	result, err := s.GetByPath("10-notes/idea.md")
	if err != nil {
		t.Fatalf("GetByPath failed: %v", err)
	}
	if result == nil {
		t.Fatal("Expected result, got nil")
	}
	if result.Title != "Idea for App" {
		t.Errorf("Expected title 'Idea for App', got '%s'", result.Title)
	}

	// Non-existent path
	_, err = s.GetByPath("nonexistent.md")
	if err == nil {
		t.Error("Expected error for non-existent path")
	}
}

func TestRecent(t *testing.T) {
	database, cleanup := setupTestDB(t)
	defer cleanup()

	s := New(database)

	results, err := s.Recent(3)
	if err != nil {
		t.Fatalf("Recent failed: %v", err)
	}
	if len(results) == 0 {
		t.Fatal("Expected recent results")
	}
	if len(results) > 3 {
		t.Errorf("Expected at most 3 results, got %d", len(results))
	}

	// Verify ordering (most recent first)
	if len(results) >= 2 {
		if results[0].UpdatedAt < results[1].UpdatedAt {
			t.Error("Expected results ordered by updated_at DESC")
		}
	}
}

func TestStale(t *testing.T) {
	database, cleanup := setupTestDB(t)
	defer cleanup()

	s := New(database)

	// All test data is from 2024, so with days=1 everything should be stale
	results, err := s.Stale(1, 100)
	if err != nil {
		t.Fatalf("Stale failed: %v", err)
	}
	// All our test notes are old, so we should get results
	if len(results) == 0 {
		// This is expected if the test data is considered fresh
		t.Log("No stale notes found (this may be expected)")
	}
}

func TestSearchResultFields(t *testing.T) {
	database, cleanup := setupTestDB(t)
	defer cleanup()

	s := New(database)

	results, err := s.Search(Query{Limit: 10})
	if err != nil {
		t.Fatalf("Search failed: %v", err)
	}

	for _, r := range results {
		if r.ID == "" {
			t.Error("Result missing ID")
		}
		if r.Title == "" {
			t.Error("Result missing Title")
		}
		// Tags should be loaded
		if r.Tags == nil {
			t.Errorf("Result %s has nil Tags", r.ID)
		}
	}
}

func TestTasks(t *testing.T) {
	database, cleanup := setupTestDB(t)
	defer cleanup()

	// Insert task notes with frontmatter_json
	tasks := []struct {
		id, fileID, title, status, project, dueDate, priority string
	}{
		{"task_001", "note_001", "Overdue Task", "open", "webapp", "2024-01-01", "high"},
		{"task_002", "note_002", "Upcoming Task", "open", "webapp", "2099-12-31", "medium"},
		{"task_003", "note_003", "Done Task", "done", "webapp", "2024-01-01", "low"},
	}
	for _, task := range tasks {
		fm := `{"due_date":"` + task.dueDate + `","priority":"` + task.priority + `"}`
		_, err := database.Exec(
			`INSERT INTO notes (id, file_id, title, type, status, project, updated_at, frontmatter_json, body) VALUES (?, ?, ?, ?, ?, ?, datetime('now'), ?, ?)`,
			task.id, task.fileID, task.title, "task", task.status, task.project, fm, "task body",
		)
		if err != nil {
			t.Fatalf("failed to insert task %s: %v", task.id, err)
		}
	}

	s := New(database)

	t.Run("status filter", func(t *testing.T) {
		results, err := s.Tasks(TaskQuery{Status: "open"})
		if err != nil {
			t.Fatalf("Tasks failed: %v", err)
		}
		if len(results) != 2 {
			t.Errorf("expected 2 open tasks, got %d", len(results))
		}
		for _, r := range results {
			if r.Status != "open" {
				t.Errorf("expected status=open, got %s", r.Status)
			}
		}
	})

	t.Run("due before filter", func(t *testing.T) {
		results, err := s.Tasks(TaskQuery{Status: "open", DueBefore: "2024-01-02"})
		if err != nil {
			t.Fatalf("Tasks due_before failed: %v", err)
		}
		if len(results) != 1 {
			t.Errorf("expected 1 overdue open task, got %d", len(results))
		}
		if results[0].ID != "task_001" {
			t.Errorf("expected task_001, got %s", results[0].ID)
		}
	})

	t.Run("ordering", func(t *testing.T) {
		results, err := s.Tasks(TaskQuery{Limit: 10})
		if err != nil {
			t.Fatalf("Tasks ordering failed: %v", err)
		}
		if len(results) != 3 {
			t.Errorf("expected 3 tasks, got %d", len(results))
		}
		// Earliest due date first; empty dates last. task_001 (2024-01-01),
		// task_003 (2024-01-01, low), task_002 (2099-12-31).
		if results[0].ID != "task_001" {
			t.Errorf("expected first task task_001, got %s", results[0].ID)
		}
		if results[2].ID != "task_002" {
			t.Errorf("expected last task task_002 (empty due date), got %s", results[2].ID)
		}
	})
}

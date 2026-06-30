package rag

import (
	"context"
	"fmt"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/agentvault/core/internal/ai"
	"github.com/agentvault/core/internal/db"
	"github.com/agentvault/core/internal/search"
)

// setupTestDB creates a test database with schema.
func setupTestDB(t *testing.T) *db.DB {
	t.Helper()
	tmpDir := t.TempDir()

	// Create .agentvault directory (required by db.Open)
	if err := os.MkdirAll(filepath.Join(tmpDir, ".agentvault"), 0755); err != nil {
		t.Fatalf("failed to create .agentvault dir: %v", err)
	}

	database, err := db.Open(tmpDir)
	if err != nil {
		t.Fatalf("failed to open test db: %v", err)
	}
	if err := database.RunMigrations(); err != nil {
		t.Fatalf("failed to run migrations: %v", err)
	}
	return database
}

// seedTestData inserts test notes into the database.
func seedTestData(t *testing.T, database *db.DB) {
	t.Helper()

	// Insert test files and notes
	_, err := database.Exec(`
		INSERT INTO files (id, path, content_hash, indexed_at) VALUES
		('f1', '30-decisions/vector-db.md', 'hash1', datetime('now')),
		('f2', 'projects/adacavo/questions.md', 'hash2', datetime('now')),
		('f3', 'reference/go-concurrency.md', 'hash3', datetime('now'))
	`)
	if err != nil {
		t.Fatalf("failed to insert files: %v", err)
	}

	_, err = database.Exec(`
		INSERT INTO notes (id, file_id, title, type, status, project, body, updated_at) VALUES
		('n1', 'f1', 'Vector Database Decision', 'decision', 'active', 'infra', 'We decided to use Postgres with pgvector. It reduces operational complexity and integrates well with our existing stack.', datetime('now')),
		('n2', 'f2', 'Open Questions for Adacavo', 'question', 'open', 'adacavo', 'What are the pricing tiers? How does the API handle rate limiting? What is the SLA?', datetime('now')),
		('n3', 'f3', 'Go Concurrency Patterns', 'reference', 'active', 'learning', 'This note covers goroutines, channels, and the sync package patterns for concurrent programming in Go.', datetime('now'))
	`)
	if err != nil {
		t.Fatalf("failed to insert notes: %v", err)
	}

	// Insert FTS data
	_, err = database.Exec(`
		INSERT INTO notes_fts (note_id, title, body, tags, entities) VALUES
		('n1', 'Vector Database Decision', 'We decided to use Postgres with pgvector. It reduces operational complexity and integrates well with our existing stack.', 'database,postgres', 'pgvector'),
		('n2', 'Open Questions for Adacavo', 'What are the pricing tiers? How does the API handle rate limiting? What is the SLA?', 'adacavo,vendor', 'Adacavo'),
		('n3', 'Go Concurrency Patterns', 'This note covers goroutines, channels, and the sync package patterns for concurrent programming in Go.', 'go,concurrency', 'Go')
	`)
	if err != nil {
		t.Fatalf("failed to insert fts data: %v", err)
	}
}

func TestPipeline_Ask_WithResults(t *testing.T) {
	database := setupTestDB(t)
	defer database.Close()
	seedTestData(t, database)

	searcher := search.New(database)
	mock := &ai.MockProvider{
		Response: `Based on the sources, you decided to use Postgres with pgvector for your vector database needs.

Confidence: high

The decision was made because it reduces operational complexity and integrates well with your existing stack.

Caveats:
- This decision may need revisiting if query performance becomes an issue
- pgvector may not support all advanced vector search features

Suggested next actions:
- Read the full decision: agentvault read 30-decisions/vector-db.md
- Benchmark pgvector against dedicated vector databases`,
	}

	pipeline := New(searcher, mock)
	ctx := context.Background()

	answer, err := pipeline.Ask(ctx, "What have I decided about vector databases?")
	if err != nil {
		t.Fatalf("Ask() unexpected error: %v", err)
	}

	if answer.Answer == "" {
		t.Error("expected non-empty answer")
	}
	if len(answer.Sources) == 0 {
		t.Error("expected sources to be included")
	}
	if answer.Confidence == "" {
		t.Error("expected confidence to be set")
	}
}

func TestPipeline_Ask_NoResults(t *testing.T) {
	database := setupTestDB(t)
	defer database.Close()
	// Don't seed any data - no results expected

	searcher := search.New(database)
	mock := &ai.MockProvider{}

	pipeline := New(searcher, mock)
	ctx := context.Background()

	answer, err := pipeline.Ask(ctx, "What is the meaning of life?")
	if err != nil {
		t.Fatalf("Ask() unexpected error: %v", err)
	}

	if answer.Answer == "" {
		t.Error("expected non-empty answer for no-results case")
	}
	if len(answer.Sources) != 0 {
		t.Errorf("expected 0 sources, got %d", len(answer.Sources))
	}
	if answer.Confidence != "low" {
		t.Errorf("expected low confidence, got %s", answer.Confidence)
	}
	if len(answer.SuggestedActions) == 0 {
		t.Error("expected suggested actions for no-results case")
	}
}

func TestPipeline_Ask_ProviderError(t *testing.T) {
	database := setupTestDB(t)
	defer database.Close()
	seedTestData(t, database)

	searcher := search.New(database)
	mock := &ai.MockProvider{Err: fmt.Errorf("connection refused")}

	pipeline := New(searcher, mock)
	ctx := context.Background()

	_, err := pipeline.Ask(ctx, "What about vector databases?")
	if err == nil {
		t.Fatal("expected error from provider, got nil")
	}
}

func TestPipeline_Ask_SourcesAlwaysIncluded(t *testing.T) {
	database := setupTestDB(t)
	defer database.Close()
	seedTestData(t, database)

	searcher := search.New(database)
	mock := &ai.MockProvider{
		Response: "Simple answer without sources.",
	}

	pipeline := New(searcher, mock)
	ctx := context.Background()

	answer, err := pipeline.Ask(ctx, "Postgres")
	if err != nil {
		t.Fatalf("Ask() unexpected error: %v", err)
	}

	if len(answer.Sources) == 0 {
		t.Error("expected sources to always be included even if AI doesn't mention them")
	}

	// Verify source paths are meaningful
	for _, src := range answer.Sources {
		if src.Path == "" {
			t.Error("expected source to have a path")
		}
		if src.Title == "" {
			t.Error("expected source to have a title")
		}
	}
}

func TestBuildPrompt(t *testing.T) {
	sources := []promptSource{
		{Path: "test.md", Title: "Test Note", Summary: "A test note.", Excerpt: "This is a test."},
	}
	question := "What is this?"

	messages := BuildPrompt(sources, question)
	if len(messages) != 2 {
		t.Fatalf("expected 2 messages, got %d", len(messages))
	}

	if messages[0].Role != "system" {
		t.Errorf("expected role 'system', got %s", messages[0].Role)
	}
	if messages[1].Role != "user" {
		t.Errorf("expected role 'user', got %s", messages[1].Role)
	}
	if messages[1].Content != question {
		t.Errorf("expected user content %q, got %q", question, messages[1].Content)
	}

	// Verify system prompt contains sources
	if !strings.Contains(messages[0].Content, "Test Note") {
		t.Error("expected system prompt to contain source title")
	}
	if !strings.Contains(messages[0].Content, "test.md") {
		t.Error("expected system prompt to contain source path")
	}
	if !strings.Contains(messages[0].Content, "Summary:") {
		t.Error("expected system prompt to contain summary")
	}
}

func TestBuildPrompt_NoSources(t *testing.T) {
	messages := BuildPrompt([]promptSource{}, "What is this?")
	if len(messages) != 2 {
		t.Fatalf("expected 2 messages, got %d", len(messages))
	}
	if !strings.Contains(messages[0].Content, "none available") {
		t.Error("expected system prompt to indicate no sources available")
	}
}

func TestNew(t *testing.T) {
	database := setupTestDB(t)
	defer database.Close()

	searcher := search.New(database)
	mock := &ai.MockProvider{}

	pipeline := New(searcher, mock)
	if pipeline == nil {
		t.Fatal("expected non-nil pipeline")
	}
	if pipeline.searcher != searcher {
		t.Error("expected searcher to be set")
	}
	if pipeline.provider != mock {
		t.Error("expected provider to be set")
	}
}

// Test parseAnswer directly
func TestParseAnswer(t *testing.T) {
	sources := []Source{{Path: "test.md", Title: "Test", Excerpt: "excerpt"}}

	t.Run("high confidence", func(t *testing.T) {
		raw := "Answer here.\n\nConfidence: high"
		ans := ParseAnswer(raw, sources)
		if ans.Confidence != "high" {
			t.Errorf("expected high confidence, got %s", ans.Confidence)
		}
	})

	t.Run("low confidence", func(t *testing.T) {
		raw := "Answer here.\n\nConfidence: low"
		ans := ParseAnswer(raw, sources)
		if ans.Confidence != "low" {
			t.Errorf("expected low confidence, got %s", ans.Confidence)
		}
	})

	t.Run("medium confidence (default)", func(t *testing.T) {
		raw := "Answer here."
		ans := ParseAnswer(raw, sources)
		if ans.Confidence != "medium" {
			t.Errorf("expected medium confidence, got %s", ans.Confidence)
		}
	})

	t.Run("sources always preserved", func(t *testing.T) {
		raw := "Just a simple answer."
		ans := ParseAnswer(raw, sources)
		if len(ans.Sources) != 1 {
			t.Errorf("expected 1 source, got %d", len(ans.Sources))
		}
	})
}

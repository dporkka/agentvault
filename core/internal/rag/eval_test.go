package rag

import (
	"context"
	"encoding/json"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/agentvault/core/internal/ai"
	"github.com/agentvault/core/internal/db"
	"github.com/agentvault/core/internal/search"
)

// evalCase is a single retrieval evaluation case.
type evalCase struct {
	Question      string   `json:"question"`
	ExpectedPaths []string `json:"expected_paths"`
}

// loadEvalCases reads the embedded evaluation questions.
func loadEvalCases(t *testing.T) []evalCase {
	t.Helper()
	data, err := os.ReadFile(filepath.Join("testdata", "eval", "questions.json"))
	if err != nil {
		t.Fatalf("failed to read eval questions: %v", err)
	}
	var cases []evalCase
	if err := json.Unmarshal(data, &cases); err != nil {
		t.Fatalf("failed to parse eval questions: %v", err)
	}
	return cases
}

// insertEvalNote creates a single file/note/fts row for retrieval testing.
func insertEvalNote(t *testing.T, database *db.DB, path, title, body string) {
	t.Helper()

	fileID := strings.ReplaceAll(path, "/", "_")
	noteID := strings.TrimSuffix(fileID, ".md")

	_, err := database.Exec(`
		INSERT INTO files (id, path, content_hash, indexed_at)
		VALUES (?, ?, ?, datetime('now'))
	`, fileID, path, "hash_"+noteID)
	if err != nil {
		t.Fatalf("failed to insert file %s: %v", path, err)
	}

	_, err = database.Exec(`
		INSERT INTO notes (id, file_id, title, type, status, project, body, updated_at)
		VALUES (?, ?, ?, 'note', 'active', ?, ?, datetime('now'))
	`, noteID, fileID, title, filepath.Dir(path), body)
	if err != nil {
		t.Fatalf("failed to insert note %s: %v", path, err)
	}

	_, err = database.Exec(`
		INSERT INTO notes_fts (note_id, title, body, tags, entities)
		VALUES (?, ?, ?, '', '')
	`, noteID, title, body)
	if err != nil {
		t.Fatalf("failed to insert fts row %s: %v", path, err)
	}
}

// seedEvalVault populates the test database with the notes referenced by a
// retrieval case plus a few distractors.
func seedEvalVault(t *testing.T, database *db.DB, c evalCase) {
	t.Helper()

	notes := map[string]struct{ title, body string }{
		"30-decisions/vector-db.md": {
			title: "Vector Database Decision",
			body:  "We decided to use Postgres with pgvector. It reduces operational complexity and integrates well with our existing stack.",
		},
		"projects/adacavo/questions.md": {
			title: "Open Questions for Adacavo",
			body:  "What are the pricing tiers? How does the API handle rate limiting? What is the SLA for enterprise customers?",
		},
		"reference/go-concurrency.md": {
			title: "Go Concurrency Patterns",
			body:  "This note covers goroutines, channels, and the sync package patterns for concurrent programming in Go.",
		},
		"50-people/vacation-policy.md": {
			title: "Vacation Policy",
			body:  "Team members should request vacation at least two weeks in advance. Unlimited PTO is available for salaried employees.",
		},
		"30-decisions/incident-response.md": {
			title: "Incident Response Process",
			body:  "When an incident occurs, page the on-call engineer, create a war room, and communicate status in the incidents channel.",
		},
		"20-projects/mobile-app.md": {
			title: "Mobile App Roadmap",
			body:  "Planned features for the mobile app include offline reading, biometric unlock, and share-sheet integration.",
		},
	}

	// Insert the expected note(s).
	for _, path := range c.ExpectedPaths {
		if n, ok := notes[path]; ok {
			insertEvalNote(t, database, path, n.title, n.body)
		}
	}

	// Insert a distractor note so retrieval must distinguish relevant content.
	insertEvalNote(t, database, "10-notes/distractor.md", "Random Musings",
		"This note contains unrelated thoughts about gardening, weather, and weekend plans.")
}

func TestRetrievalEval(t *testing.T) {
	cases := loadEvalCases(t)
	if len(cases) == 0 {
		t.Fatal("no eval cases loaded")
	}

	for _, c := range cases {
		c := c
		t.Run(c.Question, func(t *testing.T) {
			database := setupTestDB(t)
			defer database.Close()
			seedEvalVault(t, database, c)

			searcher := search.New(database)
			pipeline := New(searcher, &ai.MockProvider{})

			sources, err := pipeline.Retrieve(context.Background(), c.Question)
			if err != nil {
				t.Fatalf("Retrieve() failed: %v", err)
			}

			topK := 3
			found := false
			for i, src := range sources {
				if i >= topK {
					break
				}
				for _, expected := range c.ExpectedPaths {
					if src.Path == expected {
						found = true
						break
					}
				}
				if found {
					break
				}
			}

			if !found {
				t.Errorf("expected one of %v in top-%d sources, got %v", c.ExpectedPaths, topK, sourcePaths(sources))
			}
		})
	}
}

func sourcePaths(sources []Source) []string {
	paths := make([]string, len(sources))
	for i, s := range sources {
		paths[i] = s.Path
	}
	return paths
}

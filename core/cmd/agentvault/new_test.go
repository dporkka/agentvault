package main

import (
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"

	"github.com/agentvault/core/internal/db"
)

func TestSanitizeFilename(t *testing.T) {
	tests := []struct {
		input string
		want  string
	}{
		{"My Test Note", "my-test-note"},
		{"Hello World!", "hello-world"},
		{"  Spaces  ", "spaces"},
		{"A--B---C", "a-b-c"},
		{"-leading-and-trailing-", "leading-and-trailing"},
		{"UPPERCASE", "uppercase"},
		{"!@#$%^&*()", "untitled"},
		{"Mixed123!@#Case", "mixed123case"},
		{"", "untitled"},
		{"ñoño", "oo"},
	}

	for _, tt := range tests {
		t.Run(tt.input, func(t *testing.T) {
			got := sanitizeFilename(tt.input)
			if got != tt.want {
				t.Errorf("sanitizeFilename(%q) = %q, want %q", tt.input, got, tt.want)
			}
		})
	}
}

func TestRunNewCreatesNote(t *testing.T) {
	vp := setupTestVault(t)
	vaultPath = vp

	// Initialize database so the vault is fully usable.
	database, err := db.Open(vp)
	if err != nil {
		t.Fatalf("failed to open db: %v", err)
	}
	defer database.Close()
	if err := database.RunMigrations(); err != nil {
		t.Fatalf("failed to run migrations: %v", err)
	}

	// Stub time to keep output deterministic.
	originalTimeNow := timeNowFunc
	timeNowFunc = func() time.Time {
		return time.Date(2026, 7, 7, 12, 0, 0, 0, time.UTC)
	}
	defer func() { timeNowFunc = originalTimeNow }()

	newTitle = "My Test Idea"
	newProject = ""
	newTags = "idea, test"
	newURL = ""
	newCommit = false

	cmd := cobraCommandArgs([]string{"note"})
	if err := runNew(cmd, []string{"note"}); err != nil {
		t.Fatalf("runNew returned error: %v", err)
	}

	// Look for created file in 10-notes.
	matches, err := filepath.Glob(filepath.Join(vp, "10-notes", "*.md"))
	if err != nil {
		t.Fatalf("failed to glob notes: %v", err)
	}
	if len(matches) != 1 {
		t.Fatalf("expected 1 note file, got %d", len(matches))
	}

	content, err := os.ReadFile(matches[0])
	if err != nil {
		t.Fatalf("failed to read created note: %v", err)
	}

	if !strings.Contains(string(content), "title: My Test Idea") {
		t.Errorf("created note missing expected title")
	}
	if !strings.Contains(string(content), "type: note") {
		t.Errorf("created note missing expected type")
	}
	if !strings.Contains(string(content), "tags: [idea, test]") {
		t.Errorf("created note missing expected tags")
	}
}

func TestRunNewRejectsUnknownType(t *testing.T) {
	vp := setupTestVault(t)
	vaultPath = vp
	newTitle = "Title"

	cmd := cobraCommandArgs([]string{"unknown"})
	err := runNew(cmd, []string{"unknown"})
	if err == nil {
		t.Fatal("runNew expected error for unknown note type")
	}
	if !strings.Contains(err.Error(), "unknown note type") {
		t.Errorf("error should mention 'unknown note type', got: %v", err)
	}
}

func TestRunNewRequiresVault(t *testing.T) {
	tmpDir := t.TempDir()
	vaultPath = tmpDir
	newTitle = "Title"

	cmd := cobraCommandArgs([]string{"note"})
	err := runNew(cmd, []string{"note"})
	if err == nil {
		t.Fatal("runNew expected error when not in a vault")
	}
}

func TestRunNewWithProjectCreatesProjectFolder(t *testing.T) {
	vp := setupTestVault(t)
	vaultPath = vp

	database, err := db.Open(vp)
	if err != nil {
		t.Fatalf("failed to open db: %v", err)
	}
	defer database.Close()
	if err := database.RunMigrations(); err != nil {
		t.Fatalf("failed to run migrations: %v", err)
	}

	newTitle = "Project Meeting"
	newProject = "myproject"
	newTags = ""
	newURL = ""
	newCommit = false

	cmd := cobraCommandArgs([]string{"meeting"})
	if err := runNew(cmd, []string{"meeting"}); err != nil {
		t.Fatalf("runNew returned error: %v", err)
	}

	matches, err := filepath.Glob(filepath.Join(vp, "20-projects", "myproject", "*.md"))
	if err != nil {
		t.Fatalf("failed to glob project meetings: %v", err)
	}
	if len(matches) != 1 {
		t.Fatalf("expected 1 project meeting file, got %d", len(matches))
	}
}

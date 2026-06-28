package templates

import (
	"path/filepath"
	"testing"
)

// TestFolderPathForType locks in the single canonical filing rule shared by
// the CLI `new` command, the HTTP API, the MCP server, and the desktop app:
// a note's type decides its folder, and only meetings move into a per-project
// subfolder when a project is given. For every other type the project is
// metadata, not a file location.
func TestFolderPathForType(t *testing.T) {
	cases := []struct {
		noteType string
		project  string
		expected string // vault-relative
	}{
		{"note", "", "10-notes"},
		{"decision", "p1", "30-decisions"},
		{"task", "", "10-notes"},
		{"task", "backend", "10-notes"},
		{"meeting", "proj", "20-projects/proj"},
		{"meeting", "", "10-notes"},
		{"source", "", "40-research"},
		{"source", "research", "40-research"},
		{"capture", "", "00-inbox"},
		{"project", "", "20-projects"},
		{"unknown", "", "10-notes"},
	}

	for _, c := range cases {
		rel := FolderRelForType(c.noteType, c.project)
		wantRel := c.expected
		if rel != wantRel {
			t.Errorf("FolderRelForType(%q, %q) = %q, want %q", c.noteType, c.project, rel, wantRel)
		}

		got := FolderPathForType(c.noteType, c.project, "/vault")
		wantFull := filepath.Join("/vault", c.expected)
		if got != wantFull {
			t.Errorf("FolderPathForType(%q, %q) = %q, want %q", c.noteType, c.project, got, wantFull)
		}
	}
}
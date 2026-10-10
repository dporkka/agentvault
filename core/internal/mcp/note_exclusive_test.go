package mcp

import (
	"errors"
	"fmt"
	"os"
	"path/filepath"
	"testing"

	"github.com/agentvault/core/internal/notepublish"
)

// MCP tool commands also generate 3-digit names. Exhaust collisions rather
// than trusting that the RNG never selects a previously occupied suffix.
func TestMCPCreateNoteRefusesOccupiedGeneratedIDs(t *testing.T) {
	srv, database := setupTestServer(t)
	defer database.Close()
	folder := filepath.Join(srv.vaultPath, "10-notes")
	if err := os.MkdirAll(folder, 0755); err != nil {
		t.Fatal(err)
	}
	const original = "user-owned note"
	for n := 100; n <= 999; n++ {
		if err := os.WriteFile(filepath.Join(folder, fmt.Sprintf("existing_%d.md", n)), []byte(original), 0600); err != nil {
			t.Fatal(err)
		}
	}
	_, err := srv.createNote("note", "Existing", "", nil)
	if !errors.Is(err, notepublish.ErrExhausted) {
		t.Fatalf("MCP should return no-unoccupied-note error, got %v", err)
	}
	for n := 100; n <= 999; n++ {
		data, err := os.ReadFile(filepath.Join(folder, fmt.Sprintf("existing_%d.md", n)))
		if err != nil || string(data) != original {
			t.Fatalf("MCP overwrote user note %d: %q (%v)", n, data, err)
		}
	}
}

package main

import (
	"errors"
	"fmt"
	"os"
	"path/filepath"
	"testing"

	"github.com/agentvault/core/internal/notepublish"
)

// Every random suffix is occupied. The CLI must fail without replacing any.
func TestRunNewRefusesOccupiedGeneratedIDs(t *testing.T) {
	vp := setupTestVault(t)
	oldVault, oldTitle, oldProject := vaultPath, newTitle, newProject
	oldTags, oldURL, oldCommit := newTags, newURL, newCommit
	defer func() {
		vaultPath, newTitle, newProject = oldVault, oldTitle, oldProject
		newTags, newURL, newCommit = oldTags, oldURL, oldCommit
	}()
	vaultPath, newTitle, newProject = vp, "Existing", ""
	newTags, newURL, newCommit = "", "", false
	folder := filepath.Join(vp, "10-notes")
	if err := os.MkdirAll(folder, 0755); err != nil {
		t.Fatal(err)
	}
	const original = "user-owned note"
	for n := 100; n <= 999; n++ {
		if err := os.WriteFile(filepath.Join(folder, fmt.Sprintf("existing_%d.md", n)), []byte(original), 0600); err != nil {
			t.Fatal(err)
		}
	}
	err := runNew(cobraCommandArgs([]string{"note"}), []string{"note"})
	if !errors.Is(err, notepublish.ErrExhausted) {
		t.Fatalf("CLI should return no-unoccupied-note error, got %v", err)
	}
	for n := 100; n <= 999; n++ {
		data, err := os.ReadFile(filepath.Join(folder, fmt.Sprintf("existing_%d.md", n)))
		if err != nil || string(data) != original {
			t.Fatalf("CLI overwrote user note %d: %q (%v)", n, data, err)
		}
	}
}

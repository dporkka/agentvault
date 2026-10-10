package mcp

import (
	"errors"
	"os"
	"path/filepath"
	"strings"
	"testing"
)

func TestPublishUniqueCaptureRetriesCollisionWithoutOverwriting(t *testing.T) {
	dir := t.TempDir()
	occupied := filepath.Join(dir, "quick-idea_123.md")
	if err := os.WriteFile(occupied, []byte("precious original capture"), 0600); err != nil {
		t.Fatal(err)
	}
	ids := []string{"cap_2026_10_10_123", "cap_2026_10_10_124"}
	calls := 0
	chooseID := func() string {
		if calls >= len(ids) {
			t.Fatal("unexpected extra attempt")
		}
		id := ids[calls]
		calls++
		return id
	}

	id, path, err := publishUniqueCapture(dir, "quick-idea", chooseID, func(id string) []byte {
		return []byte("---\nid: " + id + "\ntype: capture\n---\n\nnew capture\n")
	})
	if err != nil {
		t.Fatal(err)
	}
	if id != ids[1] || path != filepath.Join(dir, "quick-idea_124.md") {
		t.Fatalf("unexpected collision fallback: id=%q path=%q", id, path)
	}
	original, err := os.ReadFile(occupied)
	if err != nil || string(original) != "precious original capture" {
		t.Fatalf("existing capture overwritten: %q (%v)", original, err)
	}
	created, err := os.ReadFile(path)
	if err != nil || !strings.Contains(string(created), "id: "+id) {
		t.Fatalf("new capture was not published complete: %q (%v)", created, err)
	}
	if calls != 2 {
		t.Fatalf("attempts = %d; want 2", calls)
	}
	assertNoCaptureStagingFiles(t, dir)
}

func TestPublishUniqueCaptureNeverReplacesOccupiedSlot(t *testing.T) {
	dir := t.TempDir()
	occupied := filepath.Join(dir, "shared_777.md")
	if err := os.WriteFile(occupied, []byte("original"), 0644); err != nil {
		t.Fatal(err)
	}

	_, _, err := publishUniqueCapture(dir, "shared",
		func() string { return "cap_2026_10_10_777" },
		func(id string) []byte { return []byte("different") },
	)
	if err == nil {
		t.Fatal("persistent filename collisions must fail closed")
	}
	if errors.Is(err, os.ErrNotExist) {
		t.Fatalf("collision should not look like missing file: %v", err)
	}
	original, readErr := os.ReadFile(occupied)
	if readErr != nil || string(original) != "original" {
		t.Fatalf("existing file modified: %q (%v)", original, readErr)
	}
	assertNoCaptureStagingFiles(t, dir)
}

func assertNoCaptureStagingFiles(t *testing.T, dir string) {
	t.Helper()
	entries, err := os.ReadDir(dir)
	if err != nil {
		t.Fatal(err)
	}
	for _, entry := range entries {
		if strings.Contains(entry.Name(), ".tmp-") {
			t.Errorf("orphaned temporary capture: %s", entry.Name())
		}
	}
}

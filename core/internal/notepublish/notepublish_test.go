package notepublish

import (
	"errors"
	"os"
	"path/filepath"
	"strings"
	"testing"
)

func TestCreateRetriesOccupiedIDWithoutReplacingNote(t *testing.T) {
	dir := t.TempDir()
	occupied := filepath.Join(dir, "my-note_100.md")
	if err := os.WriteFile(occupied, []byte("original user note"), 0600); err != nil {
		t.Fatal(err)
	}
	ids := []string{"note_today_100", "note_today_101"}
	calls := 0
	id, path, err := Create(dir, func() string {
		next := ids[calls]
		calls++
		return next
	}, func(id string) string {
		return "my-note_" + id[strings.LastIndexByte(id, '_')+1:] + ".md"
	}, func(id string) ([]byte, error) {
		return []byte("id: " + id + "\ncomplete generated note"), nil
	})
	if err != nil || id != ids[1] || path != filepath.Join(dir, "my-note_101.md") || calls != 2 {
		t.Fatalf("collision handling: id=%q path=%q attempts=%d err=%v", id, path, calls, err)
	}
	original, err := os.ReadFile(occupied)
	if err != nil || string(original) != "original user note" {
		t.Fatalf("existing note overwritten: %q (%v)", original, err)
	}
	created, err := os.ReadFile(path)
	if err != nil || string(created) != "id: note_today_101\ncomplete generated note" {
		t.Fatalf("published note's ID does not match its filename: %q (%v)", created, err)
	}
}

func TestCreateFailsClosedWhenEveryGeneratedPathIsOccupied(t *testing.T) {
	dir := t.TempDir()
	path := filepath.Join(dir, "owned.md")
	if err := os.WriteFile(path, []byte("owner"), 0644); err != nil {
		t.Fatal(err)
	}
	attempts := 0
	_, _, err := Create(dir, func() string {
		attempts++
		return "generated"
	}, func(string) string { return "owned.md" }, func(string) ([]byte, error) {
		return []byte("new"), nil
	})
	if !errors.Is(err, ErrExhausted) || attempts != maxAttempts {
		t.Fatalf("expected bounded fail-closed collision: %v, attempts=%d", err, attempts)
	}
	actual, err := os.ReadFile(path)
	if err != nil || string(actual) != "owner" {
		t.Fatalf("exhaustion overwrote existing note: %q (%v)", actual, err)
	}
}

func TestCreateRejectsPathEscapeBeforeWriting(t *testing.T) {
	dir := t.TempDir()
	for _, name := range []string{"", "..", "../escape.md", "/absolute.md"} {
		_, _, err := Create(dir, func() string { return "id" }, func(string) string {
			return name
		}, func(string) ([]byte, error) { return []byte("content"), nil })
		if err == nil {
			t.Errorf("accepted unsafe generated filename %q", name)
		}
	}
}

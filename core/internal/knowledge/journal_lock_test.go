package knowledge

import (
	"path/filepath"
	"testing"
)

func TestJournalInstancesShareLockForCanonicalPath(t *testing.T) {
	vault := t.TempDir()
	first := NewJournal(vault)
	second := NewJournal(filepath.Join(vault, ".", "nested", ".."))
	if first.Path() != second.Path() {
		t.Fatalf("canonical journal paths differ: %q != %q", first.Path(), second.Path())
	}
	if first.mu != second.mu {
		t.Fatal("journal instances for the same vault must share an in-process lock")
	}
}

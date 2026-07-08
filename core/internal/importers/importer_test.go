package importers

import (
	"fmt"
	"os"
	"path/filepath"
	"strings"
	"testing"
)

func TestImportResult_String(t *testing.T) {
	r := &ImportResult{FilesImported: 5, FilesSkipped: 2}
	s := r.String()
	if !strings.Contains(s, "Imported: 5") || !strings.Contains(s, "Skipped: 2") {
		t.Errorf("unexpected result string: %s", s)
	}
	if strings.Contains(s, "Errors") || strings.Contains(s, "Warnings") {
		t.Errorf("string should not mention errors/warnings when empty: %s", s)
	}

	r.Errors = []ImportError{{Path: "a.md", Error: "boom"}}
	r.Warnings = []string{"warn"}
	s = r.String()
	if !strings.Contains(s, "Errors: 1") {
		t.Errorf("expected Errors count in string: %s", s)
	}
	if !strings.Contains(s, "Warnings: 1") {
		t.Errorf("expected Warnings count in string: %s", s)
	}
}

func TestRegistry(t *testing.T) {
	// Snapshot and clear the global registry so the test is deterministic.
	registryLock.Lock()
	orig := make(map[string]Importer, len(registry))
	for k, v := range registry {
		orig[k] = v
	}
	for k := range registry {
		delete(registry, k)
	}
	registryLock.Unlock()

	defer func() {
		registryLock.Lock()
		for k := range registry {
			delete(registry, k)
		}
		for k, v := range orig {
			registry[k] = v
		}
		registryLock.Unlock()
	}()

	md := &MarkdownImporter{}
	ob := &ObsidianImporter{}
	Register(md)
	Register(ob)

	if got, ok := Get("markdown"); !ok || got != md {
		t.Errorf("Get(markdown) = %v, %v", got, ok)
	}
	if got, ok := Get("obsidian"); !ok || got != ob {
		t.Errorf("Get(obsidian) = %v, %v", got, ok)
	}
	if _, ok := Get("missing"); ok {
		t.Error("Get(missing) should return false")
	}

	list := List()
	if len(list) != 2 {
		t.Errorf("List() returned %d importers, want 2", len(list))
	}
	names := make(map[string]bool, len(list))
	for _, imp := range list {
		names[imp.Name()] = true
	}
	if !names["markdown"] || !names["obsidian"] {
		t.Errorf("List() missing expected names: %v", names)
	}

	avail := Available()
	if !strings.Contains(avail, "markdown") || !strings.Contains(avail, "obsidian") {
		t.Errorf("Available() = %q, want both importer names", avail)
	}
}

func TestGenerateID(t *testing.T) {
	tests := []struct {
		prefix, filename, wantSub string
	}{
		{"note", "My Note.md", "my-note.md"},
		{"note", "my_note", "my-note"},
		{"note", "UPPER CASE", "upper-case"},
		{"task", " spaces ", "spaces"},
	}

	for _, tt := range tests {
		got := GenerateID(tt.prefix, tt.filename)
		if !strings.HasPrefix(got, tt.prefix+"_") {
			t.Errorf("GenerateID(%q, %q) = %q, want prefix %q_", tt.prefix, tt.filename, got, tt.prefix)
		}
		if !strings.Contains(got, tt.wantSub) {
			t.Errorf("GenerateID(%q, %q) = %q, want substring %q", tt.prefix, tt.filename, got, tt.wantSub)
		}
	}
}

func TestIsHiddenDir_EdgeCases(t *testing.T) {
	tests := []struct {
		name   string
		hidden bool
	}{
		{"", false},
		{".", false},
		{"..", true},
		{".obsidian", true},
		{"notes", false},
	}

	for _, tt := range tests {
		got := IsHiddenDir(tt.name)
		if got != tt.hidden {
			t.Errorf("IsHiddenDir(%q) = %v, want %v", tt.name, got, tt.hidden)
		}
	}
}

func TestCollisionSafePath_FallbackWhenExhausted(t *testing.T) {
	tmp := t.TempDir()
	base := filepath.Join(tmp, "file.md")

	// Create the base path and every _1 .. _999 candidate.
	for i := 0; i < 1000; i++ {
		path := base
		if i > 0 {
			path = fmt.Sprintf("%s_%d.md", strings.TrimSuffix(base, ".md"), i)
		}
		if err := os.WriteFile(path, []byte("x"), 0644); err != nil {
			t.Fatal(err)
		}
	}

	got := CollisionSafePath(base)
	if got != base {
		t.Errorf("CollisionSafePath fallback = %q, want %q", got, base)
	}
}

func TestCollisionSafePath_SecondCandidate(t *testing.T) {
	tmp := t.TempDir()
	base := filepath.Join(tmp, "file.md")
	if err := os.WriteFile(base, []byte("x"), 0644); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(tmp, "file_1.md"), []byte("x"), 0644); err != nil {
		t.Fatal(err)
	}

	got := CollisionSafePath(base)
	want := filepath.Join(tmp, "file_2.md")
	if got != want {
		t.Errorf("CollisionSafePath = %q, want %q", got, want)
	}
}

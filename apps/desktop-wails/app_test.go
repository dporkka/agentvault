package main

import (
	"os"
	"path/filepath"
	"testing"
)

func TestNoteServiceSaveNoteRejectsTraversal(t *testing.T) {
	tmp := t.TempDir()
	svc := &NoteService{app: &App{vaultPath: tmp}}

	cases := []string{
		"../escape.md",
		"foo/../../escape.md",
		"/etc/passwd",
	}

	for _, p := range cases {
		err := svc.SaveNote(p, "content")
		if err == nil {
			t.Errorf("expected error for unsafe path %q, got nil", p)
		}
	}
}

func TestNoteServiceSaveNoteCreatesDirectories(t *testing.T) {
	tmp := t.TempDir()
	svc := &NoteService{app: &App{vaultPath: tmp}}

	if err := svc.SaveNote("10-notes/new-note.md", "hello"); err != nil {
		t.Fatalf("SaveNote failed: %v", err)
	}

	full := filepath.Join(tmp, "10-notes", "new-note.md")
	data, err := os.ReadFile(full)
	if err != nil {
		t.Fatalf("expected file to be written: %v", err)
	}
	if string(data) != "hello" {
		t.Fatalf("unexpected content: %s", data)
	}
}

func TestNoteServiceGetNoteContentRejectsTraversal(t *testing.T) {
	tmp := t.TempDir()
	svc := &NoteService{app: &App{vaultPath: tmp}}

	cases := []string{
		"../escape.md",
		"foo/../../escape.md",
		"/etc/passwd",
	}

	for _, p := range cases {
		_, err := svc.GetNoteContent(p)
		if err == nil {
			t.Errorf("expected error for unsafe path %q, got nil", p)
		}
	}
}

package db

import (
	"os"
	"path/filepath"
	"testing"

	"github.com/agentvault/core/internal/contract"
)

func setupLinksDB(t *testing.T) *DB {
	t.Helper()
	tmpDir := t.TempDir()
	os.MkdirAll(filepath.Join(tmpDir, ".agentvault"), 0755)

	db, err := Open(tmpDir)
	if err != nil {
		t.Fatalf("Open failed: %v", err)
	}
	if err := db.RunMigrations(); err != nil {
		t.Fatalf("RunMigrations failed: %v", err)
	}
	return db
}

func seedNote(t *testing.T, db *DB, id, path, title string) {
	t.Helper()
	fileID := "file_" + id
	if _, err := db.Exec(
		`INSERT INTO files (id, path, content_hash, indexed_at) VALUES (?, ?, ?, datetime('now'))`,
		fileID, path, "hash",
	); err != nil {
		t.Fatalf("insert file failed: %v", err)
	}
	if _, err := db.Exec(
		`INSERT INTO notes (id, file_id, title, type) VALUES (?, ?, ?, ?)`,
		id, fileID, title, "note",
	); err != nil {
		t.Fatalf("insert note failed: %v", err)
	}
}

func TestGetBacklinks(t *testing.T) {
	db := setupLinksDB(t)
	defer db.Close()

	seedNote(t, db, "note_a", "a.md", "Note A")
	seedNote(t, db, "note_b", "b.md", "Note B")
	seedNote(t, db, "note_c", "c.md", "Note C")

	if _, err := db.Exec(
		`INSERT INTO links (from_note_id, to_note_id, raw_target, link_type) VALUES (?, ?, ?, ?)`,
		"note_b", "note_a", "Note A", "wiki",
	); err != nil {
		t.Fatalf("insert link failed: %v", err)
	}
	if _, err := db.Exec(
		`INSERT INTO links (from_note_id, to_note_id, raw_target, link_type) VALUES (?, ?, ?, ?)`,
		"note_c", "note_a", "A", "wiki",
	); err != nil {
		t.Fatalf("insert link failed: %v", err)
	}

	links, err := db.GetBacklinks("note_a")
	if err != nil {
		t.Fatalf("GetBacklinks failed: %v", err)
	}
	if len(links) != 2 {
		t.Fatalf("expected 2 backlinks, got %d", len(links))
	}
	if links[0].ID != "note_b" {
		t.Errorf("expected first backlink note_b, got %s", links[0].ID)
	}
	if links[1].ID != "note_c" {
		t.Errorf("expected second backlink note_c, got %s", links[1].ID)
	}
	for _, link := range links {
		if link.Title == "" || link.Path == "" {
			t.Errorf("expected title and path to be populated, got %+v", link)
		}
	}
}

func TestGetOutgoingLinks(t *testing.T) {
	db := setupLinksDB(t)
	defer db.Close()

	seedNote(t, db, "note_a", "a.md", "Note A")
	seedNote(t, db, "note_b", "b.md", "Note B")
	seedNote(t, db, "note_c", "c.md", "Note C")

	if _, err := db.Exec(
		`INSERT INTO links (from_note_id, to_note_id, raw_target, link_type) VALUES (?, ?, ?, ?)`,
		"note_a", "note_b", "Note B", "wiki",
	); err != nil {
		t.Fatalf("insert link failed: %v", err)
	}
	// unresolved link should be excluded
	if _, err := db.Exec(
		`INSERT INTO links (from_note_id, to_note_id, raw_target, link_type) VALUES (?, ?, ?, ?)`,
		"note_a", nil, "Missing", "wiki",
	); err != nil {
		t.Fatalf("insert unresolved link failed: %v", err)
	}

	links, err := db.GetOutgoingLinks("note_a")
	if err != nil {
		t.Fatalf("GetOutgoingLinks failed: %v", err)
	}
	if len(links) != 1 {
		t.Fatalf("expected 1 outgoing link, got %d", len(links))
	}
	if links[0].ID != "note_b" {
		t.Errorf("expected outgoing note_b, got %s", links[0].ID)
	}
}

func TestNoteLinksContractType(t *testing.T) {
	// Ensure the DB layer returns the shared contract type.
	var _ contract.NoteLink = contract.NoteLink{}
	var response contract.NoteLinksResponse
	response.Backlinks = []contract.NoteLink{{ID: "x", Title: "X", Path: "x.md"}}
	response.Outgoing = []contract.NoteLink{{ID: "y", Title: "Y", Path: "y.md"}}
}

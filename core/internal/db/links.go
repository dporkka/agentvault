package db

import (
	"database/sql"

	"github.com/agentvault/core/internal/contract"
)

// GetBacklinks returns notes that link to the given note ID.
func (d *DB) GetBacklinks(noteID string) ([]contract.NoteLink, error) {
	rows, err := d.Query(`
		SELECT n.id, n.title, f.path
		FROM links l
		JOIN notes n ON n.id = l.from_note_id
		JOIN files f ON f.id = n.file_id
		WHERE l.to_note_id = ?
		ORDER BY n.title
	`, noteID)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	return scanNoteLinks(rows)
}

// GetOutgoingLinks returns notes that the given note ID links to.
func (d *DB) GetOutgoingLinks(noteID string) ([]contract.NoteLink, error) {
	rows, err := d.Query(`
		SELECT n.id, n.title, f.path
		FROM links l
		JOIN notes n ON n.id = l.to_note_id
		JOIN files f ON f.id = n.file_id
		WHERE l.from_note_id = ? AND l.to_note_id IS NOT NULL
		ORDER BY n.title
	`, noteID)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	return scanNoteLinks(rows)
}

func scanNoteLinks(rows *sql.Rows) ([]contract.NoteLink, error) {
	links := []contract.NoteLink{}
	for rows.Next() {
		var link contract.NoteLink
		if err := rows.Scan(&link.ID, &link.Title, &link.Path); err != nil {
			return nil, err
		}
		links = append(links, link)
	}
	return links, rows.Err()
}

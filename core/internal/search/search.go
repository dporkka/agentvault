// Package search provides SQLite FTS5 search for AgentVault.
package search

import (
	"database/sql"
	"fmt"
	"strings"

	"github.com/agentvault/core/internal/db"
)

// Searcher performs searches against the vault database.
type Searcher struct {
	db *db.DB
}

// Query defines search parameters.
type Query struct {
	Q       string
	Type    string
	Project string
	Tag     string
	Status  string
	Limit   int
	Offset  int
}

// Result is a single search result.
type Result struct {
	ID        string
	Title     string
	Path      string
	Type      string
	Project   string
	Status    string
	Tags      []string
	Snippet   string
	Score     float64
	UpdatedAt string
}

// New creates a new Searcher.
func New(database *db.DB) *Searcher {
	return &Searcher{db: database}
}

// Search performs a full-text search with optional filters.
func (s *Searcher) Search(q Query) ([]Result, error) {
	if q.Limit <= 0 {
		q.Limit = 20
	}

	hasFTS := strings.TrimSpace(q.Q) != ""

	var args []interface{}
	var query string

	if hasFTS {
		// Use FTS5 for text search
		ftsMatch := buildFTSMatch(q)
		query = `
			SELECT
				notes.id,
				notes.title,
				files.path,
				notes.type,
				notes.project,
				notes.status,
				notes.updated_at,
				snippet(notes_fts, 1, '<b>', '</b>', '...', 30),
				rank
			FROM notes_fts
			JOIN notes ON notes.id = notes_fts.note_id
			JOIN files ON files.id = notes.file_id
			WHERE notes_fts MATCH ?
		`
		args = append(args, ftsMatch)
	} else {
		// No text search - query notes table directly
		query = `
			SELECT
				notes.id,
				notes.title,
				files.path,
				notes.type,
				notes.project,
				notes.status,
				notes.updated_at,
				substr(notes.body, 1, 200),
				0.0
			FROM notes
			JOIN files ON files.id = notes.file_id
			WHERE 1=1
		`
	}

	// Tag filter requires additional join
	tagJoin := ""
	if q.Tag != "" {
		tagJoin = ` JOIN tags ON tags.note_id = notes.id `
		// Insert tag join before WHERE clause
		idx := strings.LastIndex(query, "WHERE")
		if idx >= 0 {
			query = query[:idx] + tagJoin + query[idx:]
		}
	}

	// Apply filters
	if q.Type != "" {
		query += " AND notes.type = ?"
		args = append(args, q.Type)
	}
	if q.Project != "" {
		query += " AND notes.project = ?"
		args = append(args, q.Project)
	}
	if q.Status != "" {
		query += " AND notes.status = ?"
		args = append(args, q.Status)
	}
	if q.Tag != "" {
		query += " AND tags.tag = ?"
		args = append(args, q.Tag)
	}

	// Group by to handle tag joins
	if q.Tag != "" {
		query += " GROUP BY notes.id"
	}

	// Order by rank for FTS, updated_at otherwise
	if hasFTS {
		query += " ORDER BY rank"
	} else {
		query += " ORDER BY notes.updated_at DESC"
	}

	query += " LIMIT ? OFFSET ?"
	args = append(args, q.Limit, q.Offset)

	rows, err := s.db.Query(query, args...)
	if err != nil {
		return nil, fmt.Errorf("search query failed: %w", err)
	}
	defer rows.Close()

	return s.scanResults(rows)
}

// GetByID looks up a note by its exact ID.
func (s *Searcher) GetByID(id string) (*Result, error) {
	row := s.db.QueryRow(`
		SELECT
			notes.id,
			notes.title,
			files.path,
			notes.type,
			notes.project,
			notes.status,
			notes.updated_at,
			notes.body,
			0.0
		FROM notes
		JOIN files ON files.id = notes.file_id
		WHERE notes.id = ?
	`, id)

	result, err := s.scanSingle(row)
	if err != nil {
		if err == sql.ErrNoRows {
			return nil, fmt.Errorf("note not found: %s", id)
		}
		return nil, err
	}

	tags, err := s.loadTags(id)
	if err != nil {
		return nil, err
	}
	result.Tags = tags

	return result, nil
}

// GetByPath looks up a note by its file path.
func (s *Searcher) GetByPath(path string) (*Result, error) {
	row := s.db.QueryRow(`
		SELECT
			notes.id,
			notes.title,
			files.path,
			notes.type,
			notes.project,
			notes.status,
			notes.updated_at,
			notes.body,
			0.0
		FROM notes
		JOIN files ON files.id = notes.file_id
		WHERE files.path = ?
	`, path)

	result, err := s.scanSingle(row)
	if err != nil {
		if err == sql.ErrNoRows {
			return nil, fmt.Errorf("note not found at path: %s", path)
		}
		return nil, err
	}

	tags, err := s.loadTags(result.ID)
	if err != nil {
		return nil, err
	}
	result.Tags = tags

	return result, nil
}

// Recent returns the most recently updated notes.
func (s *Searcher) Recent(limit int) ([]Result, error) {
	if limit <= 0 {
		limit = 20
	}

	rows, err := s.db.Query(`
		SELECT
			notes.id,
			notes.title,
			files.path,
			notes.type,
			notes.project,
			notes.status,
			notes.updated_at,
			substr(notes.body, 1, 200),
			0.0
		FROM notes
		JOIN files ON files.id = notes.file_id
		ORDER BY notes.updated_at DESC
		LIMIT ?
	`, limit)
	if err != nil {
		return nil, fmt.Errorf("recent query failed: %w", err)
	}
	defer rows.Close()

	return s.scanResults(rows)
}

// Stale returns notes not updated in the last N days.
func (s *Searcher) Stale(days int) ([]Result, error) {
	if days <= 0 {
		days = 30
	}

	rows, err := s.db.Query(`
		SELECT
			notes.id,
			notes.title,
			files.path,
			notes.type,
			notes.project,
			notes.status,
			notes.updated_at,
			substr(notes.body, 1, 200),
			0.0
		FROM notes
		JOIN files ON files.id = notes.file_id
		WHERE notes.updated_at < datetime('now', '-' || ? || ' days')
		ORDER BY notes.updated_at ASC
	`, days)
	if err != nil {
		return nil, fmt.Errorf("stale query failed: %w", err)
	}
	defer rows.Close()

	return s.scanResults(rows)
}

// buildFTSMatch constructs an FTS5 MATCH expression from the query.
func buildFTSMatch(q Query) string {
	q.Q = strings.TrimSpace(q.Q)
	if q.Q == "" {
		return ""
	}

	// Escape quotes in the query
	escaped := strings.ReplaceAll(q.Q, `"`, `""`)

	// If the query contains special FTS characters, pass through
	if strings.ContainsAny(escaped, "*+~") {
		return escaped
	}

	// Simple word query - use OR between words for broader matching
	words := strings.Fields(escaped)
	if len(words) > 1 {
		return strings.Join(words, " OR ")
	}

	return escaped
}

// loadTags fetches tags for a note.
func (s *Searcher) loadTags(noteID string) ([]string, error) {
	rows, err := s.db.Query("SELECT tag FROM tags WHERE note_id = ? ORDER BY tag", noteID)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	var tags []string
	for rows.Next() {
		var tag string
		if err := rows.Scan(&tag); err != nil {
			return nil, err
		}
		tags = append(tags, tag)
	}
	return tags, rows.Err()
}

// scanResults scans multiple search results from rows.
func (s *Searcher) scanResults(rows *sql.Rows) ([]Result, error) {
	var results []Result
	for rows.Next() {
		var r Result
		var snippet sql.NullString
		err := rows.Scan(&r.ID, &r.Title, &r.Path, &r.Type, &r.Project, &r.Status, &r.UpdatedAt, &snippet, &r.Score)
		if err != nil {
			return nil, fmt.Errorf("scan failed: %w", err)
		}
		if snippet.Valid {
			r.Snippet = snippet.String
		}

		// Load tags for this result
		tags, err := s.loadTags(r.ID)
		if err != nil {
			return nil, err
		}
		r.Tags = tags

		results = append(results, r)
	}
	return results, rows.Err()
}

// scanSingle scans a single result from a row.
func (s *Searcher) scanSingle(row *sql.Row) (*Result, error) {
	var r Result
	var body sql.NullString
	err := row.Scan(&r.ID, &r.Title, &r.Path, &r.Type, &r.Project, &r.Status, &r.UpdatedAt, &body, &r.Score)
	if err != nil {
		return nil, err
	}
	if body.Valid {
		r.Snippet = body.String
	}
	return &r, nil
}

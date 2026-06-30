// Package search provides SQLite FTS5 search for AgentVault.
package search

import (
	"database/sql"
	"encoding/json"
	"fmt"
	"strings"

	"github.com/agentvault/core/internal/config"
	"github.com/agentvault/core/internal/contract"
	"github.com/agentvault/core/internal/db"
	"github.com/agentvault/core/internal/embeddings"
)

// Searcher performs searches against the vault database.
type Searcher struct {
	db         *db.DB
	embedClient *embeddings.Client
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

// Result is a single search result. It is an alias of contract.SearchResult
// so existing call sites (and the HTTP handlers that re-export this name)
// keep working unchanged.
type Result = contract.SearchResult

// New creates a new Searcher.
func New(database *db.DB) *Searcher {
	return &Searcher{db: database}
}

// SetEmbedClient sets the embedding client for vector search.
func (s *Searcher) SetEmbedClient(client *embeddings.Client) {
	s.embedClient = client
}

// ConfigureEmbeddings loads the vault config and creates an embedding client
// for vector/hybrid search. It mirrors the configuration logic used by the
// indexer so search and indexing agree on which model and endpoint to use.
// A missing or invalid config falls back to the Ollama default.
func (s *Searcher) ConfigureEmbeddings(vaultPath string) {
	cfg, err := config.Load(vaultPath)
	if err != nil {
		s.embedClient = embeddings.NewClient("http://localhost:11434", "nomic-embed-text")
		return
	}

	baseURL := "http://localhost:11434"
	model := "nomic-embed-text"

	if cfg.AI != nil {
		if cfg.AI.BaseURL != "" {
			baseURL = cfg.AI.BaseURL
		}
		if cfg.AI.EmbeddingModel != "" {
			model = cfg.AI.EmbeddingModel
		}
	}

	s.embedClient = embeddings.NewClient(baseURL, model)
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

// NoteLinks returns the backlinks and outgoing links for a note.
func (s *Searcher) NoteLinks(noteID string) (backlinks []contract.NoteLink, outgoing []contract.NoteLink, err error) {
	backlinks, err = s.db.GetBacklinks(noteID)
	if err != nil {
		return nil, nil, fmt.Errorf("failed to load backlinks: %w", err)
	}
	outgoing, err = s.db.GetOutgoingLinks(noteID)
	if err != nil {
		return nil, nil, fmt.Errorf("failed to load outgoing links: %w", err)
	}
	return backlinks, outgoing, nil
}

// Stale returns notes not updated in the last N days.
func (s *Searcher) Stale(days, limit int) ([]Result, error) {
	if days <= 0 {
		days = 30
	}
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
		WHERE notes.updated_at < datetime('now', '-' || ? || ' days')
		ORDER BY notes.updated_at ASC
		LIMIT ?
	`, days, limit)
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
//
// IMPORTANT: loadTags issues a nested query on the same DB. With
// MaxOpenConns(1) (required for SQLite write serialization), a nested
// query issued while the parent *sql.Rows is still being iterated would
// deadlock — the parent holds the only connection. To avoid that, we
// fully drain and close the parent rows first, then load tags.
func (s *Searcher) scanResults(rows *sql.Rows) ([]Result, error) {
	type rawResult struct {
		r       Result
		snippet sql.NullString
	}
	var raws []rawResult
	for rows.Next() {
		var raw rawResult
		err := rows.Scan(&raw.r.ID, &raw.r.Title, &raw.r.Path, &raw.r.Type, &raw.r.Project, &raw.r.Status, &raw.r.UpdatedAt, &raw.snippet, &raw.r.Score)
		if err != nil {
			return nil, fmt.Errorf("scan failed: %w", err)
		}
		raws = append(raws, raw)
	}
	if err := rows.Err(); err != nil {
		return nil, err
	}
	rows.Close()

	results := make([]Result, 0, len(raws))
	for _, raw := range raws {
		r := raw.r
		if raw.snippet.Valid {
			r.Snippet = raw.snippet.String
		}
		tags, err := s.loadTags(r.ID)
		if err != nil {
			return nil, err
		}
		r.Tags = tags
		results = append(results, r)
	}
	return results, nil
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

// TaskResult is an alias of contract.TaskResult so callers can use the
// search package directly without importing contract.
type TaskResult = contract.TaskResult

// DecisionResult is an alias of contract.DecisionResult.
type DecisionResult = contract.DecisionResult

// MeetingResult is an alias of contract.MeetingResult.
type MeetingResult = contract.MeetingResult

// TaskQuery is an alias of contract.TaskQuery.
type TaskQuery = contract.TaskQuery

// Tasks returns actionable tasks filtered by status and due date.
func (s *Searcher) Tasks(q TaskQuery) ([]TaskResult, error) {
	if q.Limit <= 0 {
		q.Limit = 50
	}

	args := []interface{}{"task"}
	query := `
		SELECT
			notes.id,
			notes.title,
			files.path,
			notes.status,
			coalesce(json_extract(frontmatter_json, '$.priority'), ''),
			coalesce(json_extract(frontmatter_json, '$.due_date'), ''),
			notes.project
		FROM notes
		JOIN files ON files.id = notes.file_id
		WHERE notes.type = ?
	`

	if q.Status != "" {
		query += " AND notes.status = ?"
		args = append(args, q.Status)
	}
	if q.DueBefore != "" {
		query += " AND json_extract(frontmatter_json, '$.due_date') != '' AND json_extract(frontmatter_json, '$.due_date') < ?"
		args = append(args, q.DueBefore)
	}
	if q.DueAfter != "" {
		query += " AND json_extract(frontmatter_json, '$.due_date') != '' AND json_extract(frontmatter_json, '$.due_date') >= ?"
		args = append(args, q.DueAfter)
	}

	query += `
		ORDER BY
			case when json_extract(frontmatter_json, '$.due_date') = '' then 1 else 0 end,
			json_extract(frontmatter_json, '$.due_date') ASC,
			case json_extract(frontmatter_json, '$.priority')
				when 'high' then 0
				when 'medium' then 1
				when 'low' then 2
				else 3
			end
		LIMIT ?
	`
	args = append(args, q.Limit)

	rows, err := s.db.Query(query, args...)
	if err != nil {
		return nil, fmt.Errorf("tasks query failed: %w", err)
	}
	defer rows.Close()

	results := []TaskResult{}
	for rows.Next() {
		var r TaskResult
		if err := rows.Scan(&r.ID, &r.Title, &r.Path, &r.Status, &r.Priority, &r.DueDate, &r.Project); err != nil {
			return nil, fmt.Errorf("tasks scan failed: %w", err)
		}
		results = append(results, r)
	}
	return results, rows.Err()
}

// Decisions returns decisions, optionally filtered by status.
func (s *Searcher) Decisions(status string, limit int) ([]DecisionResult, error) {
	if limit <= 0 {
		limit = 50
	}

	args := []interface{}{"decision"}
	query := `
		SELECT
			notes.id,
			notes.title,
			files.path,
			notes.status
		FROM notes
		JOIN files ON files.id = notes.file_id
		WHERE notes.type = ?
	`
	if status != "" {
		query += " AND notes.status = ?"
		args = append(args, status)
	}
	query += " ORDER BY notes.updated_at DESC LIMIT ?"
	args = append(args, limit)

	rows, err := s.db.Query(query, args...)
	if err != nil {
		return nil, fmt.Errorf("decisions query failed: %w", err)
	}
	defer rows.Close()

	results := []DecisionResult{}
	for rows.Next() {
		var r DecisionResult
		if err := rows.Scan(&r.ID, &r.Title, &r.Path, &r.Status); err != nil {
			return nil, fmt.Errorf("decisions scan failed: %w", err)
		}
		results = append(results, r)
	}
	return results, rows.Err()
}

// Meetings returns meetings with attendees extracted from frontmatter_json.
func (s *Searcher) Meetings(limit int) ([]MeetingResult, error) {
	if limit <= 0 {
		limit = 50
	}

	rows, err := s.db.Query(`
		SELECT
			notes.id,
			notes.title,
			files.path,
			coalesce(json_extract(frontmatter_json, '$.attendees'), '[]')
		FROM notes
		JOIN files ON files.id = notes.file_id
		WHERE notes.type = ?
		ORDER BY notes.updated_at DESC
		LIMIT ?
	`, "meeting", limit)
	if err != nil {
		return nil, fmt.Errorf("meetings query failed: %w", err)
	}
	defer rows.Close()

	results := []MeetingResult{}
	for rows.Next() {
		var r MeetingResult
		var attendeesJSON string
		if err := rows.Scan(&r.ID, &r.Title, &r.Path, &attendeesJSON); err != nil {
			return nil, fmt.Errorf("meetings scan failed: %w", err)
		}
		if attendeesJSON != "" {
			_ = json.Unmarshal([]byte(attendeesJSON), &r.Attendees)
		}
		results = append(results, r)
	}
	return results, rows.Err()
}

// ChunkContext represents one chunk of a note, with position relative to a
// matched chunk, for expanding RAG source context.
type ChunkContext struct {
	Text     string
	Index    int
	IsCenter bool
}

// LoadAdjacentChunks returns the chunks surrounding chunkIndex for a note.
// The window argument controls how many chunks before and after the center
// are included. If the note has one or fewer chunks, only the center chunk is
// returned and no extra lookup is performed.
func (s *Searcher) LoadAdjacentChunks(noteID string, chunkIndex int, window int) ([]ChunkContext, error) {
	if window < 0 {
		window = 0
	}

	rows, err := s.db.Query(`
		SELECT chunk_index, text
		FROM chunks
		WHERE note_id = ?
		ORDER BY chunk_index
	`, noteID)
	if err != nil {
		return nil, fmt.Errorf("failed to load chunks: %w", err)
	}
	defer rows.Close()

	var chunks []ChunkContext
	for rows.Next() {
		var idx int
		var text string
		if err := rows.Scan(&idx, &text); err != nil {
			return nil, fmt.Errorf("failed to scan chunk: %w", err)
		}
		chunks = append(chunks, ChunkContext{Text: text, Index: idx})
	}
	if err := rows.Err(); err != nil {
		return nil, err
	}

	if len(chunks) == 0 {
		return nil, nil
	}
	if len(chunks) <= 1 {
		chunks[0].IsCenter = true
		return chunks, nil
	}

	var out []ChunkContext
	centerFound := false
	for _, c := range chunks {
		if c.Index == chunkIndex {
			centerFound = true
		}
		if c.Index >= chunkIndex-window && c.Index <= chunkIndex+window {
			c.IsCenter = c.Index == chunkIndex
			out = append(out, c)
		}
	}
	if !centerFound {
		return nil, nil
	}
	return out, nil
}

// FindChunkIndex returns the chunk_index for a note whose text exactly matches
// the supplied snippet. It is used to locate the chunk a search result matched
// so adjacent chunks can be loaded for context expansion.
func (s *Searcher) FindChunkIndex(noteID, text string) (int, bool) {
	row := s.db.QueryRow(`
		SELECT chunk_index FROM chunks
		WHERE note_id = ? AND text = ?
	`, noteID, text)
	var idx int
	if err := row.Scan(&idx); err != nil {
		return 0, false
	}
	return idx, true
}

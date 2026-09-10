package memory

import (
	"context"
	"database/sql"
	"encoding/json"
	"fmt"
	"strings"
	"time"

	"github.com/agentvault/core/internal/db"
)

// Store persists and retrieves the SQLite projection of canonical memory
// metadata. Callers that mutate user-visible memory must update Markdown first;
// Project only updates the rebuildable index representation.
type Store struct {
	db *db.DB
}

// NewStore creates a semantic memory store backed by the vault database.
func NewStore(database *db.DB) *Store {
	return &Store{db: database}
}

// Project replaces the indexed semantic metadata for an existing note and its
// outgoing supersession relations atomically.
func (s *Store) Project(ctx context.Context, metadata Metadata) error {
	normalized, err := metadata.Normalize()
	if err != nil {
		return err
	}
	metadata = normalized

	var provenanceJSON interface{}
	if !isZeroProvenance(metadata.Provenance) {
		encoded, err := json.Marshal(metadata.Provenance)
		if err != nil {
			return fmt.Errorf("encode provenance: %w", err)
		}
		provenanceJSON = string(encoded)
	}

	tx, err := s.db.Begin()
	if err != nil {
		return fmt.Errorf("begin memory projection: %w", err)
	}
	committed := false
	defer func() {
		if !committed {
			_ = tx.Rollback()
		}
	}()

	result, err := tx.ExecContext(ctx, `
		UPDATE notes
		SET workspace_id = ?, agent_id = ?, session_id = ?, memory_kind = ?,
			confidence = ?, provenance_json = ?, observed_at = ?, valid_from = ?, valid_to = ?
		WHERE id = ?
	`, metadata.Scope.WorkspaceID, metadata.Scope.AgentID, metadata.Scope.SessionID,
		string(metadata.Kind), nullableFloat(metadata.Confidence), provenanceJSON,
		nullableString(metadata.ObservedAt), nullableString(metadata.ValidFrom),
		nullableString(metadata.ValidTo), metadata.NoteID)
	if err != nil {
		return fmt.Errorf("update memory metadata: %w", err)
	}
	rows, err := result.RowsAffected()
	if err != nil {
		return fmt.Errorf("inspect memory update: %w", err)
	}
	if rows == 0 {
		return fmt.Errorf("note not found: %s", metadata.NoteID)
	}

	if _, err := tx.ExecContext(ctx,
		"DELETE FROM memory_supersessions WHERE superseding_note_id = ?",
		metadata.NoteID,
	); err != nil {
		return fmt.Errorf("clear supersession projection: %w", err)
	}

	for _, supersededID := range metadata.Supersedes {
		if _, err := tx.ExecContext(ctx, `
			INSERT INTO memory_supersessions
				(superseding_note_id, superseded_note_id, reason, created_at)
			VALUES (?, ?, ?, datetime('now'))
		`, metadata.NoteID, supersededID, nullableString(metadata.SupersessionReason)); err != nil {
			return fmt.Errorf("project supersession %s -> %s: %w", metadata.NoteID, supersededID, err)
		}
	}

	if err := tx.Commit(); err != nil {
		return fmt.Errorf("commit memory projection: %w", err)
	}
	committed = true
	return nil
}

// Get returns semantic metadata and note identity for one note. Superseded is
// true when any supersession relation points at the note; scoped retrieval uses
// contextual supersession rules in Query rather than this absolute flag.
func (s *Store) Get(ctx context.Context, noteID string) (*Record, error) {
	row := s.db.QueryRow(`
		SELECT notes.id, notes.title, files.path, notes.type, notes.project, notes.status,
			notes.updated_at, notes.workspace_id, notes.agent_id, notes.session_id,
			notes.memory_kind, notes.confidence, notes.provenance_json, notes.observed_at,
			notes.valid_from, notes.valid_to,
			EXISTS(SELECT 1 FROM memory_supersessions ms WHERE ms.superseded_note_id = notes.id)
		FROM notes
		JOIN files ON files.id = notes.file_id
		WHERE notes.id = ?
	`, noteID)

	record, err := scanRecord(row.Scan)
	if err != nil {
		if err == sql.ErrNoRows {
			return nil, fmt.Errorf("memory note not found: %s", noteID)
		}
		return nil, err
	}

	supersedes, reason, err := s.loadSupersedes(noteID)
	if err != nil {
		return nil, err
	}
	record.Supersedes = supersedes
	record.SupersessionReason = reason
	return record, nil
}

// Query returns memories visible from the supplied scope. Broader memories are
// inherited by narrower contexts; scoped memories never leak into a different
// workspace, agent, or session. Expired/future memories are excluded at the
// current time unless Query.At selects another instant.
func (s *Store) Query(ctx context.Context, query Query) ([]Record, error) {
	limit := query.Limit
	if limit <= 0 {
		limit = 20
	}
	if limit > 200 {
		limit = 200
	}

	for _, kind := range query.Kinds {
		if !kind.Valid() {
			return nil, fmt.Errorf("unsupported memory kind %q", kind)
		}
	}
	if query.MinConfidence != nil && (*query.MinConfidence < 0 || *query.MinConfidence > 1) {
		return nil, fmt.Errorf("minimum confidence must be between 0 and 1")
	}

	at := time.Now().UTC()
	if query.At != nil {
		at = query.At.UTC()
	}
	atText := at.Format(time.RFC3339)

	var sqlText strings.Builder
	sqlText.WriteString(`
		SELECT notes.id, notes.title, files.path, notes.type, notes.project, notes.status,
			notes.updated_at, notes.workspace_id, notes.agent_id, notes.session_id,
			notes.memory_kind, notes.confidence, notes.provenance_json, notes.observed_at,
			notes.valid_from, notes.valid_to,
			EXISTS(SELECT 1 FROM memory_supersessions ms WHERE ms.superseded_note_id = notes.id)
		FROM notes
		JOIN files ON files.id = notes.file_id
		WHERE (notes.workspace_id = '' OR notes.workspace_id = ?)
		  AND (notes.agent_id = '' OR notes.agent_id = ?)
		  AND (notes.session_id = '' OR notes.session_id = ?)
		  AND (notes.valid_from IS NULL OR notes.valid_from = '' OR notes.valid_from <= ?)
		  AND (notes.valid_to IS NULL OR notes.valid_to = '' OR notes.valid_to > ?)
	`)
	args := []interface{}{
		query.Context.WorkspaceID,
		query.Context.AgentID,
		query.Context.SessionID,
		atText,
		atText,
	}

	if len(query.Kinds) > 0 {
		placeholders := make([]string, len(query.Kinds))
		for i, kind := range query.Kinds {
			placeholders[i] = "?"
			args = append(args, string(kind))
		}
		sqlText.WriteString(" AND notes.memory_kind IN (" + strings.Join(placeholders, ",") + ")")
	}
	if query.MinConfidence != nil {
		sqlText.WriteString(" AND notes.confidence IS NOT NULL AND notes.confidence >= ?")
		args = append(args, *query.MinConfidence)
	}
	if !query.IncludeSuperseded {
		// Supersession is contextual. A scoped replacement can suppress an older
		// memory only in contexts where that replacement itself is visible and
		// temporally active. This prevents workspace-local facts from invalidating
		// a broader/global memory for unrelated workspaces.
		sqlText.WriteString(`
			AND NOT EXISTS (
				SELECT 1
				FROM memory_supersessions ms
				JOIN notes sup ON sup.id = ms.superseding_note_id
				WHERE ms.superseded_note_id = notes.id
				  AND (sup.workspace_id = '' OR sup.workspace_id = ?)
				  AND (sup.agent_id = '' OR sup.agent_id = ?)
				  AND (sup.session_id = '' OR sup.session_id = ?)
				  AND (sup.valid_from IS NULL OR sup.valid_from = '' OR sup.valid_from <= ?)
				  AND (sup.valid_to IS NULL OR sup.valid_to = '' OR sup.valid_to > ?)
			)
		`)
		args = append(args,
			query.Context.WorkspaceID,
			query.Context.AgentID,
			query.Context.SessionID,
			atText,
			atText,
		)
	}

	sqlText.WriteString(`
		ORDER BY
			CASE
				WHEN notes.session_id <> '' THEN 3
				WHEN notes.agent_id <> '' THEN 2
				WHEN notes.workspace_id <> '' THEN 1
				ELSE 0
			END DESC,
			COALESCE(notes.confidence, 0.5) DESC,
			COALESCE(notes.observed_at, notes.updated_at, notes.created_at) DESC
		LIMIT ?
	`)
	args = append(args, limit)

	rows, err := s.db.Query(sqlText.String(), args...)
	if err != nil {
		return nil, fmt.Errorf("query memories: %w", err)
	}
	defer rows.Close()

	var records []Record
	for rows.Next() {
		record, err := scanRecord(rows.Scan)
		if err != nil {
			return nil, err
		}
		records = append(records, *record)
	}
	if err := rows.Err(); err != nil {
		return nil, err
	}
	return records, nil
}

type rowScanner func(dest ...interface{}) error

func scanRecord(scan rowScanner) (*Record, error) {
	var record Record
	var kind string
	var confidence sql.NullFloat64
	var provenanceJSON sql.NullString
	var observedAt sql.NullString
	var validFrom sql.NullString
	var validTo sql.NullString
	var updatedAt sql.NullString
	var project sql.NullString
	var status sql.NullString

	if err := scan(
		&record.NoteID,
		&record.Title,
		&record.Path,
		&record.Type,
		&project,
		&status,
		&updatedAt,
		&record.Scope.WorkspaceID,
		&record.Scope.AgentID,
		&record.Scope.SessionID,
		&kind,
		&confidence,
		&provenanceJSON,
		&observedAt,
		&validFrom,
		&validTo,
		&record.Superseded,
	); err != nil {
		return nil, err
	}

	record.Kind = Kind(kind)
	if project.Valid {
		record.Project = project.String
	}
	if status.Valid {
		record.Status = status.String
	}
	if updatedAt.Valid {
		record.UpdatedAt = updatedAt.String
	}
	if confidence.Valid {
		value := confidence.Float64
		record.Confidence = &value
	}
	if provenanceJSON.Valid && provenanceJSON.String != "" {
		if err := json.Unmarshal([]byte(provenanceJSON.String), &record.Provenance); err != nil {
			return nil, fmt.Errorf("decode provenance for %s: %w", record.NoteID, err)
		}
	}
	if observedAt.Valid {
		record.ObservedAt = observedAt.String
	}
	if validFrom.Valid {
		record.ValidFrom = validFrom.String
	}
	if validTo.Valid {
		record.ValidTo = validTo.String
	}
	return &record, nil
}

func (s *Store) loadSupersedes(noteID string) ([]string, string, error) {
	rows, err := s.db.Query(`
		SELECT superseded_note_id, reason
		FROM memory_supersessions
		WHERE superseding_note_id = ?
		ORDER BY superseded_note_id
	`, noteID)
	if err != nil {
		return nil, "", fmt.Errorf("load supersessions: %w", err)
	}
	defer rows.Close()

	var ids []string
	var reason string
	for rows.Next() {
		var id string
		var rowReason sql.NullString
		if err := rows.Scan(&id, &rowReason); err != nil {
			return nil, "", err
		}
		ids = append(ids, id)
		if reason == "" && rowReason.Valid {
			reason = rowReason.String
		}
	}
	return ids, reason, rows.Err()
}

func nullableString(value string) interface{} {
	if strings.TrimSpace(value) == "" {
		return nil
	}
	return value
}

func nullableFloat(value *float64) interface{} {
	if value == nil {
		return nil
	}
	return *value
}

func isZeroProvenance(p Provenance) bool {
	return p.SourceType == "" && p.SourceRef == "" && p.Actor == "" &&
		p.Model == "" && p.CapturedAt == ""
}

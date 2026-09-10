// Package knowledge provides durable, structured knowledge primitives on top
// of AgentVault's rebuildable SQLite index. Markdown/YAML files remain the
// canonical representation for user-authored documents; this package stores
// machine-derived objects, relations, provenance, memories, and agent state.
package knowledge

import (
	"database/sql"
	"encoding/json"
	"errors"
	"fmt"
	"strings"
	"time"

	"github.com/agentvault/core/internal/contract"
	"github.com/agentvault/core/internal/db"
	"github.com/google/uuid"
)

const defaultLimit = 100

// Store persists universal knowledge primitives.
type Store struct {
	db *db.DB
}

// New returns a knowledge store backed by database.
func New(database *db.DB) *Store {
	return &Store{db: database}
}

// CreateProvenance appends a provenance record. Provenance is intentionally
// append-only so evidence behind existing facts is never silently rewritten.
func (s *Store) CreateProvenance(record contract.ProvenanceRecord) (contract.ProvenanceRecord, error) {
	if strings.TrimSpace(record.SourceType) == "" {
		return contract.ProvenanceRecord{}, errors.New("sourceType is required")
	}
	if record.Confidence < 0 || record.Confidence > 1 {
		return contract.ProvenanceRecord{}, errors.New("confidence must be between 0 and 1")
	}

	now := time.Now().UTC().Format(time.RFC3339Nano)
	if record.ID == "" {
		record.ID = newID("prov")
	}
	if record.ObservedAt == "" {
		record.ObservedAt = now
	}
	if record.CreatedAt == "" {
		record.CreatedAt = now
	}

	evidence, err := json.Marshal(orEmptySlice(record.Evidence))
	if err != nil {
		return contract.ProvenanceRecord{}, fmt.Errorf("marshal provenance evidence: %w", err)
	}
	metadata, err := json.Marshal(orEmptyMap(record.Metadata))
	if err != nil {
		return contract.ProvenanceRecord{}, fmt.Errorf("marshal provenance metadata: %w", err)
	}

	_, err = s.db.Exec(`
		INSERT INTO provenance_records (
			id, source_type, source_id, agent_id, session_id, model, confidence,
			observed_at, evidence_json, metadata_json, created_at
		) VALUES (?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?)`,
		record.ID, record.SourceType, nullIfEmpty(record.SourceID), nullIfEmpty(record.AgentID),
		nullIfEmpty(record.SessionID), nullIfEmpty(record.Model), record.Confidence,
		record.ObservedAt, string(evidence), string(metadata), record.CreatedAt,
	)
	if err != nil {
		return contract.ProvenanceRecord{}, fmt.Errorf("create provenance: %w", err)
	}
	return record, nil
}

// GetProvenance returns a provenance record by ID.
func (s *Store) GetProvenance(id string) (contract.ProvenanceRecord, error) {
	var record contract.ProvenanceRecord
	var evidenceJSON, metadataJSON string
	err := s.db.QueryRow(`
		SELECT id, source_type, COALESCE(source_id, ''), COALESCE(agent_id, ''),
		       COALESCE(session_id, ''), COALESCE(model, ''), confidence,
		       observed_at, COALESCE(evidence_json, '[]'), COALESCE(metadata_json, '{}'), created_at
		FROM provenance_records WHERE id = ?`, id).Scan(
		&record.ID, &record.SourceType, &record.SourceID, &record.AgentID, &record.SessionID,
		&record.Model, &record.Confidence, &record.ObservedAt, &evidenceJSON, &metadataJSON,
		&record.CreatedAt,
	)
	if err != nil {
		return contract.ProvenanceRecord{}, err
	}
	if err := json.Unmarshal([]byte(evidenceJSON), &record.Evidence); err != nil {
		return contract.ProvenanceRecord{}, fmt.Errorf("decode provenance evidence: %w", err)
	}
	if err := json.Unmarshal([]byte(metadataJSON), &record.Metadata); err != nil {
		return contract.ProvenanceRecord{}, fmt.Errorf("decode provenance metadata: %w", err)
	}
	return record, nil
}

// UpsertObject creates or updates a typed knowledge object. The object's ID is
// stable across title/path changes, which makes links safe to rename.
func (s *Store) UpsertObject(req contract.UpsertKnowledgeObjectRequest) (contract.KnowledgeObject, error) {
	if strings.TrimSpace(req.Type) == "" {
		return contract.KnowledgeObject{}, errors.New("type is required")
	}
	if strings.TrimSpace(req.Title) == "" {
		return contract.KnowledgeObject{}, errors.New("title is required")
	}

	id := req.ID
	if id == "" {
		id = newID("obj")
	}
	now := time.Now().UTC().Format(time.RFC3339Nano)
	data, err := json.Marshal(orEmptyMap(req.Data))
	if err != nil {
		return contract.KnowledgeObject{}, fmt.Errorf("marshal object data: %w", err)
	}

	_, err = s.db.Exec(`
		INSERT INTO objects (
			id, type, title, status, organization, project, canonical_path,
			data_json, provenance_id, created_at, updated_at
		) VALUES (?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?)
		ON CONFLICT(id) DO UPDATE SET
			type = excluded.type,
			title = excluded.title,
			status = excluded.status,
			organization = excluded.organization,
			project = excluded.project,
			canonical_path = excluded.canonical_path,
			data_json = excluded.data_json,
			provenance_id = excluded.provenance_id,
			updated_at = excluded.updated_at`,
		id, req.Type, req.Title, nullIfEmpty(req.Status), nullIfEmpty(req.Organization),
		nullIfEmpty(req.Project), nullIfEmpty(req.CanonicalPath), string(data),
		nullIfEmpty(req.ProvenanceID), now, now,
	)
	if err != nil {
		return contract.KnowledgeObject{}, fmt.Errorf("upsert object: %w", err)
	}
	return s.GetObject(id)
}

// GetObject returns a typed knowledge object by stable ID.
func (s *Store) GetObject(id string) (contract.KnowledgeObject, error) {
	var object contract.KnowledgeObject
	var dataJSON string
	err := s.db.QueryRow(`
		SELECT id, type, title, COALESCE(status, ''), COALESCE(organization, ''),
		       COALESCE(project, ''), COALESCE(canonical_path, ''), data_json,
		       COALESCE(provenance_id, ''), created_at, updated_at
		FROM objects WHERE id = ?`, id).Scan(
		&object.ID, &object.Type, &object.Title, &object.Status, &object.Organization,
		&object.Project, &object.CanonicalPath, &dataJSON, &object.ProvenanceID,
		&object.CreatedAt, &object.UpdatedAt,
	)
	if err != nil {
		return contract.KnowledgeObject{}, err
	}
	if err := json.Unmarshal([]byte(dataJSON), &object.Data); err != nil {
		return contract.KnowledgeObject{}, fmt.Errorf("decode object data: %w", err)
	}
	return object, nil
}

// ListObjects returns typed objects matching filter, newest first.
func (s *Store) ListObjects(filter contract.KnowledgeObjectFilter) ([]contract.KnowledgeObject, error) {
	query := `
		SELECT id, type, title, COALESCE(status, ''), COALESCE(organization, ''),
		       COALESCE(project, ''), COALESCE(canonical_path, ''), data_json,
		       COALESCE(provenance_id, ''), created_at, updated_at
		FROM objects WHERE 1=1`
	args := make([]interface{}, 0, 5)
	if filter.Type != "" {
		query += " AND type = ?"
		args = append(args, filter.Type)
	}
	if filter.Organization != "" {
		query += " AND organization = ?"
		args = append(args, filter.Organization)
	}
	if filter.Project != "" {
		query += " AND project = ?"
		args = append(args, filter.Project)
	}
	if filter.Status != "" {
		query += " AND status = ?"
		args = append(args, filter.Status)
	}
	limit := filter.Limit
	if limit <= 0 || limit > 1000 {
		limit = defaultLimit
	}
	query += " ORDER BY updated_at DESC LIMIT ?"
	args = append(args, limit)

	rows, err := s.db.Query(query, args...)
	if err != nil {
		return nil, fmt.Errorf("list objects: %w", err)
	}
	defer rows.Close()

	objects := make([]contract.KnowledgeObject, 0)
	for rows.Next() {
		var object contract.KnowledgeObject
		var dataJSON string
		if err := rows.Scan(
			&object.ID, &object.Type, &object.Title, &object.Status, &object.Organization,
			&object.Project, &object.CanonicalPath, &dataJSON, &object.ProvenanceID,
			&object.CreatedAt, &object.UpdatedAt,
		); err != nil {
			return nil, err
		}
		if err := json.Unmarshal([]byte(dataJSON), &object.Data); err != nil {
			return nil, fmt.Errorf("decode object data: %w", err)
		}
		objects = append(objects, object)
	}
	return objects, rows.Err()
}

// CreateRelation creates a typed relation between two stable object IDs.
func (s *Store) CreateRelation(req contract.CreateObjectRelationRequest) (contract.ObjectRelation, error) {
	if req.FromObjectID == "" || req.ToObjectID == "" {
		return contract.ObjectRelation{}, errors.New("fromObjectId and toObjectId are required")
	}
	if strings.TrimSpace(req.RelationType) == "" {
		return contract.ObjectRelation{}, errors.New("relationType is required")
	}
	confidence := 1.0
	if req.Confidence != nil {
		confidence = *req.Confidence
	}
	if confidence < 0 || confidence > 1 {
		return contract.ObjectRelation{}, errors.New("confidence must be between 0 and 1")
	}

	id := req.ID
	if id == "" {
		id = newID("rel")
	}
	now := time.Now().UTC().Format(time.RFC3339Nano)
	metadata, err := json.Marshal(orEmptyMap(req.Metadata))
	if err != nil {
		return contract.ObjectRelation{}, fmt.Errorf("marshal relation metadata: %w", err)
	}

	_, err = s.db.Exec(`
		INSERT INTO object_relations (
			id, from_object_id, to_object_id, relation_type, valid_from, valid_to,
			confidence, provenance_id, metadata_json, created_at, updated_at
		) VALUES (?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?)`,
		id, req.FromObjectID, req.ToObjectID, req.RelationType, nullIfEmpty(req.ValidFrom),
		nullIfEmpty(req.ValidTo), confidence, nullIfEmpty(req.ProvenanceID), string(metadata), now, now,
	)
	if err != nil {
		return contract.ObjectRelation{}, fmt.Errorf("create relation: %w", err)
	}

	return contract.ObjectRelation{
		ID: id, FromObjectID: req.FromObjectID, ToObjectID: req.ToObjectID,
		RelationType: req.RelationType, ValidFrom: req.ValidFrom, ValidTo: req.ValidTo,
		Confidence: confidence, ProvenanceID: req.ProvenanceID, Metadata: orEmptyMap(req.Metadata),
		CreatedAt: now, UpdatedAt: now,
	}, nil
}

// RelationsForObject returns incoming and outgoing relations for objectID.
func (s *Store) RelationsForObject(objectID string) ([]contract.ObjectRelation, error) {
	rows, err := s.db.Query(`
		SELECT id, from_object_id, to_object_id, relation_type,
		       COALESCE(valid_from, ''), COALESCE(valid_to, ''), confidence,
		       COALESCE(provenance_id, ''), metadata_json, created_at, updated_at
		FROM object_relations
		WHERE from_object_id = ? OR to_object_id = ?
		ORDER BY updated_at DESC`, objectID, objectID)
	if err != nil {
		return nil, fmt.Errorf("list relations: %w", err)
	}
	defer rows.Close()

	relations := make([]contract.ObjectRelation, 0)
	for rows.Next() {
		var relation contract.ObjectRelation
		var metadataJSON string
		if err := rows.Scan(
			&relation.ID, &relation.FromObjectID, &relation.ToObjectID, &relation.RelationType,
			&relation.ValidFrom, &relation.ValidTo, &relation.Confidence, &relation.ProvenanceID,
			&metadataJSON, &relation.CreatedAt, &relation.UpdatedAt,
		); err != nil {
			return nil, err
		}
		if err := json.Unmarshal([]byte(metadataJSON), &relation.Metadata); err != nil {
			return nil, fmt.Errorf("decode relation metadata: %w", err)
		}
		relations = append(relations, relation)
	}
	return relations, rows.Err()
}

// RecordMemory creates a scoped memory record without mutating older memories.
// Superseded records remain queryable for historical reconstruction.
func (s *Store) RecordMemory(req contract.CreateMemoryRequest) (contract.MemoryRecord, error) {
	if !validMemoryType(req.MemoryType) {
		return contract.MemoryRecord{}, errors.New("memoryType must be working, episodic, semantic, or procedural")
	}
	if strings.TrimSpace(req.ScopeType) == "" || strings.TrimSpace(req.ScopeID) == "" {
		return contract.MemoryRecord{}, errors.New("scopeType and scopeId are required")
	}
	if strings.TrimSpace(req.Content) == "" {
		return contract.MemoryRecord{}, errors.New("content is required")
	}
	confidence := 1.0
	if req.Confidence != nil {
		confidence = *req.Confidence
	}
	if confidence < 0 || confidence > 1 {
		return contract.MemoryRecord{}, errors.New("confidence must be between 0 and 1")
	}

	id := req.ID
	if id == "" {
		id = newID("mem")
	}
	now := time.Now().UTC().Format(time.RFC3339Nano)
	metadata, err := json.Marshal(orEmptyMap(req.Metadata))
	if err != nil {
		return contract.MemoryRecord{}, fmt.Errorf("marshal memory metadata: %w", err)
	}

	_, err = s.db.Exec(`
		INSERT INTO memory_records (
			id, memory_type, scope_type, scope_id, content, object_id, provenance_id,
			confidence, valid_from, valid_to, supersedes_id, metadata_json, created_at, updated_at
		) VALUES (?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?)`,
		id, req.MemoryType, req.ScopeType, req.ScopeID, req.Content, nullIfEmpty(req.ObjectID),
		nullIfEmpty(req.ProvenanceID), confidence, nullIfEmpty(req.ValidFrom), nullIfEmpty(req.ValidTo),
		nullIfEmpty(req.SupersedesID), string(metadata), now, now,
	)
	if err != nil {
		return contract.MemoryRecord{}, fmt.Errorf("record memory: %w", err)
	}

	return contract.MemoryRecord{
		ID: id, MemoryType: req.MemoryType, ScopeType: req.ScopeType, ScopeID: req.ScopeID,
		Content: req.Content, ObjectID: req.ObjectID, ProvenanceID: req.ProvenanceID,
		Confidence: confidence, ValidFrom: req.ValidFrom, ValidTo: req.ValidTo,
		SupersedesID: req.SupersedesID, Metadata: orEmptyMap(req.Metadata), CreatedAt: now, UpdatedAt: now,
	}, nil
}

// ListMemories returns memories for a scope, optionally filtered by memoryType.
func (s *Store) ListMemories(scopeType, scopeID, memoryType string, limit int) ([]contract.MemoryRecord, error) {
	if scopeType == "" || scopeID == "" {
		return nil, errors.New("scopeType and scopeId are required")
	}
	if memoryType != "" && !validMemoryType(memoryType) {
		return nil, errors.New("invalid memoryType")
	}
	if limit <= 0 || limit > 1000 {
		limit = defaultLimit
	}

	query := `
		SELECT id, memory_type, scope_type, scope_id, content,
		       COALESCE(object_id, ''), COALESCE(provenance_id, ''), confidence,
		       COALESCE(valid_from, ''), COALESCE(valid_to, ''), COALESCE(supersedes_id, ''),
		       metadata_json, created_at, updated_at
		FROM memory_records WHERE scope_type = ? AND scope_id = ?`
	args := []interface{}{scopeType, scopeID}
	if memoryType != "" {
		query += " AND memory_type = ?"
		args = append(args, memoryType)
	}
	query += " ORDER BY updated_at DESC LIMIT ?"
	args = append(args, limit)

	rows, err := s.db.Query(query, args...)
	if err != nil {
		return nil, fmt.Errorf("list memories: %w", err)
	}
	defer rows.Close()

	memories := make([]contract.MemoryRecord, 0)
	for rows.Next() {
		var memory contract.MemoryRecord
		var metadataJSON string
		if err := rows.Scan(
			&memory.ID, &memory.MemoryType, &memory.ScopeType, &memory.ScopeID, &memory.Content,
			&memory.ObjectID, &memory.ProvenanceID, &memory.Confidence, &memory.ValidFrom,
			&memory.ValidTo, &memory.SupersedesID, &metadataJSON, &memory.CreatedAt, &memory.UpdatedAt,
		); err != nil {
			return nil, err
		}
		if err := json.Unmarshal([]byte(metadataJSON), &memory.Metadata); err != nil {
			return nil, fmt.Errorf("decode memory metadata: %w", err)
		}
		memories = append(memories, memory)
	}
	return memories, rows.Err()
}

// StartSession creates a durable workspace for an agent objective.
func (s *Store) StartSession(req contract.StartAgentSessionRequest) (contract.AgentSession, error) {
	if strings.TrimSpace(req.AgentID) == "" {
		return contract.AgentSession{}, errors.New("agentId is required")
	}
	if strings.TrimSpace(req.Objective) == "" {
		return contract.AgentSession{}, errors.New("objective is required")
	}
	id := req.ID
	if id == "" {
		id = newID("session")
	}
	now := time.Now().UTC().Format(time.RFC3339Nano)
	contextJSON, err := json.Marshal(orEmptyMap(req.Context))
	if err != nil {
		return contract.AgentSession{}, fmt.Errorf("marshal session context: %w", err)
	}

	_, err = s.db.Exec(`
		INSERT INTO agent_sessions (
			id, agent_id, project, objective, status, branch, worktree, context_json,
			started_at, updated_at
		) VALUES (?, ?, ?, ?, 'active', ?, ?, ?, ?, ?)`,
		id, req.AgentID, nullIfEmpty(req.Project), req.Objective, nullIfEmpty(req.Branch),
		nullIfEmpty(req.Worktree), string(contextJSON), now, now,
	)
	if err != nil {
		return contract.AgentSession{}, fmt.Errorf("start session: %w", err)
	}
	return s.GetSession(id)
}

// AppendSessionEvent appends a durable event and advances the session's
// updatedAt timestamp without rewriting historical events.
func (s *Store) AppendSessionEvent(sessionID string, req contract.AppendSessionEventRequest) (contract.SessionEvent, error) {
	if strings.TrimSpace(sessionID) == "" {
		return contract.SessionEvent{}, errors.New("session id is required")
	}
	if strings.TrimSpace(req.EventType) == "" {
		return contract.SessionEvent{}, errors.New("eventType is required")
	}
	id := req.ID
	if id == "" {
		id = newID("event")
	}
	now := time.Now().UTC().Format(time.RFC3339Nano)
	payload, err := json.Marshal(orEmptyMap(req.Payload))
	if err != nil {
		return contract.SessionEvent{}, fmt.Errorf("marshal session event payload: %w", err)
	}

	tx, err := s.db.Begin()
	if err != nil {
		return contract.SessionEvent{}, err
	}
	defer func() { _ = tx.Rollback() }()

	result, err := tx.Exec(`UPDATE agent_sessions SET updated_at = ? WHERE id = ?`, now, sessionID)
	if err != nil {
		return contract.SessionEvent{}, fmt.Errorf("touch session: %w", err)
	}
	rows, err := result.RowsAffected()
	if err != nil {
		return contract.SessionEvent{}, err
	}
	if rows == 0 {
		return contract.SessionEvent{}, sql.ErrNoRows
	}
	_, err = tx.Exec(`
		INSERT INTO session_events (id, session_id, event_type, payload_json, provenance_id, created_at)
		VALUES (?, ?, ?, ?, ?, ?)`,
		id, sessionID, req.EventType, string(payload), nullIfEmpty(req.ProvenanceID), now,
	)
	if err != nil {
		return contract.SessionEvent{}, fmt.Errorf("append session event: %w", err)
	}
	if err := tx.Commit(); err != nil {
		return contract.SessionEvent{}, err
	}

	return contract.SessionEvent{
		ID: id, SessionID: sessionID, EventType: req.EventType, Payload: orEmptyMap(req.Payload),
		ProvenanceID: req.ProvenanceID, CreatedAt: now,
	}, nil
}

// CloseSession marks a session terminal. The default terminal status is completed.
func (s *Store) CloseSession(id, status string) (contract.AgentSession, error) {
	if status == "" {
		status = "completed"
	}
	if status == "active" {
		return contract.AgentSession{}, errors.New("close status cannot be active")
	}
	now := time.Now().UTC().Format(time.RFC3339Nano)
	result, err := s.db.Exec(`
		UPDATE agent_sessions SET status = ?, updated_at = ?, ended_at = ? WHERE id = ?`,
		status, now, now, id,
	)
	if err != nil {
		return contract.AgentSession{}, fmt.Errorf("close session: %w", err)
	}
	rows, err := result.RowsAffected()
	if err != nil {
		return contract.AgentSession{}, err
	}
	if rows == 0 {
		return contract.AgentSession{}, sql.ErrNoRows
	}
	return s.GetSession(id)
}

// GetSession returns a session and its ordered event history.
func (s *Store) GetSession(id string) (contract.AgentSession, error) {
	var session contract.AgentSession
	var contextJSON string
	err := s.db.QueryRow(`
		SELECT id, agent_id, COALESCE(project, ''), objective, status,
		       COALESCE(branch, ''), COALESCE(worktree, ''), context_json,
		       started_at, updated_at, COALESCE(ended_at, '')
		FROM agent_sessions WHERE id = ?`, id).Scan(
		&session.ID, &session.AgentID, &session.Project, &session.Objective, &session.Status,
		&session.Branch, &session.Worktree, &contextJSON, &session.StartedAt, &session.UpdatedAt,
		&session.EndedAt,
	)
	if err != nil {
		return contract.AgentSession{}, err
	}
	if err := json.Unmarshal([]byte(contextJSON), &session.Context); err != nil {
		return contract.AgentSession{}, fmt.Errorf("decode session context: %w", err)
	}

	rows, err := s.db.Query(`
		SELECT id, session_id, event_type, payload_json, COALESCE(provenance_id, ''), created_at
		FROM session_events WHERE session_id = ? ORDER BY created_at, id`, id)
	if err != nil {
		return contract.AgentSession{}, fmt.Errorf("list session events: %w", err)
	}
	defer rows.Close()

	session.Events = make([]contract.SessionEvent, 0)
	for rows.Next() {
		var event contract.SessionEvent
		var payloadJSON string
		if err := rows.Scan(&event.ID, &event.SessionID, &event.EventType, &payloadJSON, &event.ProvenanceID, &event.CreatedAt); err != nil {
			return contract.AgentSession{}, err
		}
		if err := json.Unmarshal([]byte(payloadJSON), &event.Payload); err != nil {
			return contract.AgentSession{}, fmt.Errorf("decode session event payload: %w", err)
		}
		session.Events = append(session.Events, event)
	}
	return session, rows.Err()
}

func validMemoryType(value string) bool {
	switch value {
	case "working", "episodic", "semantic", "procedural":
		return true
	default:
		return false
	}
}

func newID(prefix string) string {
	return prefix + "_" + uuid.NewString()
}

func nullIfEmpty(value string) interface{} {
	if value == "" {
		return nil
	}
	return value
}

func orEmptyMap(value map[string]interface{}) map[string]interface{} {
	if value == nil {
		return map[string]interface{}{}
	}
	return value
}

func orEmptySlice(value []contract.ProvenanceEvidence) []contract.ProvenanceEvidence {
	if value == nil {
		return []contract.ProvenanceEvidence{}
	}
	return value
}

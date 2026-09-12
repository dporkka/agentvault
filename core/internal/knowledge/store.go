// Package knowledge provides durable, structured knowledge primitives for
// AgentVault. Machine-authored state is persisted to a user-owned JSONL journal
// and projected into SQLite for fast querying; Markdown/YAML remains canonical
// for user-authored documents.
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

// Store persists universal knowledge primitives. When journal is configured,
// write operations are journal-first so SQLite can be treated as a projection.
type Store struct {
	db      *db.DB
	journal *Journal
}

// New returns a knowledge store backed by database. Supplying vaultPath enables
// the canonical append-only journal used for durable machine-authored state.
func New(database *db.DB, vaultPath ...string) *Store {
	store := &Store{db: database}
	if len(vaultPath) > 0 && strings.TrimSpace(vaultPath[0]) != "" {
		store.journal = NewJournal(vaultPath[0])
	}
	return store
}

// JournalPath returns the canonical journal path, or an empty string when this
// store was created without durable journaling (primarily useful in tests).
func (s *Store) JournalPath() string {
	if s.journal == nil {
		return ""
	}
	return s.journal.Path()
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
	record.Evidence = orEmptySlice(record.Evidence)
	record.Metadata = orEmptyMap(record.Metadata)

	if err := s.ensureIDAbsent("provenance_records", record.ID); err != nil {
		return contract.ProvenanceRecord{}, err
	}
	if err := s.persist(eventProvenanceCreated, record, func() error {
		return s.projectProvenance(record)
	}); err != nil {
		return contract.ProvenanceRecord{}, err
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
	if err := s.validateCanonicalPath(id, req.CanonicalPath); err != nil {
		return contract.KnowledgeObject{}, err
	}
	if err := s.validateOptionalProvenance(req.ProvenanceID); err != nil {
		return contract.KnowledgeObject{}, err
	}

	now := time.Now().UTC().Format(time.RFC3339Nano)
	createdAt := now
	if existing, err := s.GetObject(id); err == nil {
		createdAt = existing.CreatedAt
	} else if !errors.Is(err, sql.ErrNoRows) {
		return contract.KnowledgeObject{}, err
	}

	object := contract.KnowledgeObject{
		ID:            id,
		Type:          req.Type,
		Title:         req.Title,
		Status:        req.Status,
		Organization:  req.Organization,
		Project:       req.Project,
		CanonicalPath: req.CanonicalPath,
		Data:          orEmptyMap(req.Data),
		ProvenanceID:  req.ProvenanceID,
		CreatedAt:     createdAt,
		UpdatedAt:     now,
	}
	if err := s.persist(eventObjectUpserted, object, func() error {
		return s.projectObject(object)
	}); err != nil {
		return contract.KnowledgeObject{}, err
	}
	return object, nil
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
	if _, err := s.GetObject(req.FromObjectID); err != nil {
		return contract.ObjectRelation{}, fmt.Errorf("from object: %w", err)
	}
	if _, err := s.GetObject(req.ToObjectID); err != nil {
		return contract.ObjectRelation{}, fmt.Errorf("to object: %w", err)
	}
	if err := s.validateOptionalProvenance(req.ProvenanceID); err != nil {
		return contract.ObjectRelation{}, err
	}

	id := req.ID
	if id == "" {
		id = newID("rel")
	}
	if err := s.ensureIDAbsent("object_relations", id); err != nil {
		return contract.ObjectRelation{}, err
	}
	now := time.Now().UTC().Format(time.RFC3339Nano)
	relation := contract.ObjectRelation{
		ID:           id,
		FromObjectID: req.FromObjectID,
		ToObjectID:   req.ToObjectID,
		RelationType: req.RelationType,
		ValidFrom:    req.ValidFrom,
		ValidTo:      req.ValidTo,
		Confidence:   confidence,
		ProvenanceID: req.ProvenanceID,
		Metadata:     orEmptyMap(req.Metadata),
		CreatedAt:    now,
		UpdatedAt:    now,
	}
	if err := s.persist(eventRelationCreated, relation, func() error {
		return s.projectRelation(relation)
	}); err != nil {
		return contract.ObjectRelation{}, err
	}
	return relation, nil
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
	if !validMemoryClass(req.MemoryClass) {
		return contract.MemoryRecord{}, errors.New("memoryClass must be working, episodic, semantic, or procedural")
	}
	if req.MemoryKind != "" && !validMemoryKind(req.MemoryKind) {
		return contract.MemoryRecord{}, errors.New("invalid memoryKind")
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
	if err := s.validateOptionalObject(req.ObjectID); err != nil {
		return contract.MemoryRecord{}, err
	}
	if err := s.validateOptionalProvenance(req.ProvenanceID); err != nil {
		return contract.MemoryRecord{}, err
	}
	if req.SupersedesID != "" {
		if _, err := s.GetMemory(req.SupersedesID); err != nil {
			return contract.MemoryRecord{}, fmt.Errorf("superseded memory: %w", err)
		}
	}

	id := req.ID
	if id == "" {
		id = newID("mem")
	}
	if err := s.ensureIDAbsent("memory_records", id); err != nil {
		return contract.MemoryRecord{}, err
	}
	now := time.Now().UTC().Format(time.RFC3339Nano)
	memory := contract.MemoryRecord{
		ID:           id,
		MemoryClass:  req.MemoryClass,
		MemoryKind:   req.MemoryKind,
		ScopeType:    req.ScopeType,
		ScopeID:      req.ScopeID,
		Content:      req.Content,
		ObjectID:     req.ObjectID,
		ProvenanceID: req.ProvenanceID,
		Confidence:   confidence,
		ValidFrom:    req.ValidFrom,
		ValidTo:      req.ValidTo,
		SupersedesID: req.SupersedesID,
		Metadata:     orEmptyMap(req.Metadata),
		CreatedAt:    now,
		UpdatedAt:    now,
	}
	if err := s.persist(eventMemoryRecorded, memory, func() error {
		return s.projectMemory(memory)
	}); err != nil {
		return contract.MemoryRecord{}, err
	}
	return memory, nil
}

// GetMemory returns one durable memory record by ID.
func (s *Store) GetMemory(id string) (contract.MemoryRecord, error) {
	var memory contract.MemoryRecord
	var metadataJSON string
	err := s.db.QueryRow(`
		SELECT id, memory_class, COALESCE(memory_kind, ''), scope_type, scope_id, content,
		       COALESCE(object_id, ''), COALESCE(provenance_id, ''), confidence,
		       COALESCE(valid_from, ''), COALESCE(valid_to, ''), COALESCE(supersedes_id, ''),
		       metadata_json, created_at, updated_at
		FROM memory_records WHERE id = ?`, id).Scan(
		&memory.ID, &memory.MemoryClass, &memory.MemoryKind, &memory.ScopeType, &memory.ScopeID, &memory.Content,
		&memory.ObjectID, &memory.ProvenanceID, &memory.Confidence, &memory.ValidFrom,
		&memory.ValidTo, &memory.SupersedesID, &metadataJSON, &memory.CreatedAt, &memory.UpdatedAt,
	)
	if err != nil {
		return contract.MemoryRecord{}, err
	}
	if err := json.Unmarshal([]byte(metadataJSON), &memory.Metadata); err != nil {
		return contract.MemoryRecord{}, fmt.Errorf("decode memory metadata: %w", err)
	}
	return memory, nil
}

// ListMemories returns memories for a scope, optionally filtered by memoryClass.
func (s *Store) ListMemories(scopeType, scopeID, memoryClass string, limit int) ([]contract.MemoryRecord, error) {
	if scopeType == "" || scopeID == "" {
		return nil, errors.New("scopeType and scopeId are required")
	}
	if memoryClass != "" && !validMemoryClass(memoryClass) {
		return nil, errors.New("invalid memoryClass")
	}
	if limit <= 0 || limit > 1000 {
		limit = defaultLimit
	}

	query := `
		SELECT id, memory_class, COALESCE(memory_kind, ''), scope_type, scope_id, content,
		       COALESCE(object_id, ''), COALESCE(provenance_id, ''), confidence,
		       COALESCE(valid_from, ''), COALESCE(valid_to, ''), COALESCE(supersedes_id, ''),
		       metadata_json, created_at, updated_at
		FROM memory_records WHERE scope_type = ? AND scope_id = ?`
	args := []interface{}{scopeType, scopeID}
	if memoryClass != "" {
		query += " AND memory_class = ?"
		args = append(args, memoryClass)
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
			&memory.ID, &memory.MemoryClass, &memory.MemoryKind, &memory.ScopeType, &memory.ScopeID, &memory.Content,
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
	if err := s.ensureIDAbsent("agent_sessions", id); err != nil {
		return contract.AgentSession{}, err
	}
	now := time.Now().UTC().Format(time.RFC3339Nano)
	session := contract.AgentSession{
		ID:        id,
		AgentID:   req.AgentID,
		Project:   req.Project,
		Objective: req.Objective,
		Status:    "active",
		Branch:    req.Branch,
		Worktree:  req.Worktree,
		Context:   orEmptyMap(req.Context),
		StartedAt: now,
		UpdatedAt: now,
		Events:    []contract.SessionEvent{},
	}
	if err := s.persist(eventSessionStarted, session, func() error {
		return s.projectSessionStart(session)
	}); err != nil {
		return contract.AgentSession{}, err
	}
	return session, nil
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
	session, err := s.GetSession(sessionID)
	if err != nil {
		return contract.SessionEvent{}, err
	}
	if session.Status != "active" {
		return contract.SessionEvent{}, fmt.Errorf("session %s is %s", sessionID, session.Status)
	}
	if err := s.validateOptionalProvenance(req.ProvenanceID); err != nil {
		return contract.SessionEvent{}, err
	}

	id := req.ID
	if id == "" {
		id = newID("event")
	}
	if err := s.ensureIDAbsent("session_events", id); err != nil {
		return contract.SessionEvent{}, err
	}
	now := time.Now().UTC().Format(time.RFC3339Nano)
	event := contract.SessionEvent{
		ID:           id,
		SessionID:    sessionID,
		EventType:    req.EventType,
		Payload:      orEmptyMap(req.Payload),
		ProvenanceID: req.ProvenanceID,
		CreatedAt:    now,
	}
	if err := s.persist(eventSessionEvent, event, func() error {
		return s.projectSessionEvent(event)
	}); err != nil {
		return contract.SessionEvent{}, err
	}
	return event, nil
}

// CloseSession marks an active session terminal while preserving its history.
func (s *Store) CloseSession(id, status string) (contract.AgentSession, error) {
	if status == "" {
		status = "completed"
	}
	if status == "active" {
		return contract.AgentSession{}, errors.New("close status cannot be active")
	}
	session, err := s.GetSession(id)
	if err != nil {
		return contract.AgentSession{}, err
	}
	if session.Status != "active" {
		return contract.AgentSession{}, fmt.Errorf("session %s is already %s", id, session.Status)
	}

	now := time.Now().UTC().Format(time.RFC3339Nano)
	closeEvent := sessionCloseProjection{
		ID:        id,
		Status:    status,
		UpdatedAt: now,
		EndedAt:   now,
	}
	if err := s.persist(eventSessionClosed, closeEvent, func() error {
		return s.projectSessionClose(closeEvent)
	}); err != nil {
		return contract.AgentSession{}, err
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

func (s *Store) persist(eventType string, payload interface{}, project func() error) error {
	if s.journal != nil {
		if _, err := s.journal.Append(eventType, payload); err != nil {
			return fmt.Errorf("persist canonical knowledge event: %w", err)
		}
	}
	if err := project(); err != nil {
		return fmt.Errorf("project canonical knowledge event: %w", err)
	}
	return nil
}

func (s *Store) ensureIDAbsent(table, id string) error {
	var existing string
	query := "SELECT id FROM " + table + " WHERE id = ?"
	err := s.db.QueryRow(query, id).Scan(&existing)
	if errors.Is(err, sql.ErrNoRows) {
		return nil
	}
	if err != nil {
		return err
	}
	return fmt.Errorf("%s id %s already exists", table, id)
}

func (s *Store) validateCanonicalPath(id, canonicalPath string) error {
	if canonicalPath == "" {
		return nil
	}
	var existing string
	err := s.db.QueryRow(`SELECT id FROM objects WHERE canonical_path = ? AND id <> ?`, canonicalPath, id).Scan(&existing)
	if errors.Is(err, sql.ErrNoRows) {
		return nil
	}
	if err != nil {
		return err
	}
	return fmt.Errorf("canonicalPath %q is already owned by object %s", canonicalPath, existing)
}

func (s *Store) validateOptionalProvenance(id string) error {
	if id == "" {
		return nil
	}
	if _, err := s.GetProvenance(id); err != nil {
		return fmt.Errorf("provenance: %w", err)
	}
	return nil
}

func (s *Store) validateOptionalObject(id string) error {
	if id == "" {
		return nil
	}
	if _, err := s.GetObject(id); err != nil {
		return fmt.Errorf("object: %w", err)
	}
	return nil
}

func validMemoryClass(value string) bool {
	switch value {
	case "working", "episodic", "semantic", "procedural":
		return true
	default:
		return false
	}
}

func validMemoryKind(value string) bool {
	switch value {
	case "observation", "episode", "fact", "preference", "decision", "procedure", "constraint", "summary":
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

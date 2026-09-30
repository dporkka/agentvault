// Package memories models derived agent memory separately from raw history.
package memories

import (
	"context"
	"crypto/rand"
	"database/sql"
	"encoding/hex"
	"encoding/json"
	"errors"
	"fmt"
	"strings"
	"time"

	"github.com/agentvault/core/internal/db"
	"github.com/agentvault/core/internal/events"
)

const (
	KindEpisodic   = "episodic"
	KindSemantic   = "semantic"
	KindProcedural = "procedural"

	StatusCandidate  = "candidate"
	StatusDurable    = "durable"
	StatusRejected   = "rejected"
	StatusSuperseded = "superseded"
)

type Memory struct {
	ID            string                 `json:"id"`
	AgentID       string                 `json:"agentId"`
	Kind          string                 `json:"kind"`
	Status        string                 `json:"status"`
	Content       string                 `json:"content"`
	Confidence    float64                `json:"confidence"`
	Salience      float64                `json:"salience"`
	ScopeType     string                 `json:"scopeType,omitempty"`
	ScopeID       string                 `json:"scopeId,omitempty"`
	SourceEventID string                 `json:"sourceEventId"`
	Metadata      map[string]interface{} `json:"metadata"`
	CreatedAt     time.Time              `json:"createdAt"`
	UpdatedAt     time.Time              `json:"updatedAt"`
	PromotedAt    *time.Time             `json:"promotedAt,omitempty"`
}

type CreateCandidateInput struct {
	AgentID       string
	Kind          string
	Content       string
	Confidence    float64
	Salience      float64
	ScopeType     string
	ScopeID       string
	SourceEventID string
	Metadata      map[string]interface{}
}

type PromoteInput struct {
	ActorType string
	ActorID   string
	Reason    string
}

type Store struct {
	db *db.DB
}

func NewStore(database *db.DB) *Store {
	return &Store{db: database}
}

func (s *Store) CreateCandidate(ctx context.Context, input CreateCandidateInput) (Memory, error) {
	if err := validateCandidate(input); err != nil {
		return Memory{}, err
	}

	metadata := input.Metadata
	if metadata == nil {
		metadata = map[string]interface{}{}
	}
	metadataJSON, err := json.Marshal(metadata)
	if err != nil {
		return Memory{}, fmt.Errorf("marshal memory metadata: %w", err)
	}

	id, err := newID("mem_")
	if err != nil {
		return Memory{}, err
	}
	now := time.Now().UTC()

	tx, err := s.db.Conn().BeginTx(ctx, nil)
	if err != nil {
		return Memory{}, fmt.Errorf("begin memory transaction: %w", err)
	}
	defer tx.Rollback()

	if _, err := tx.ExecContext(ctx, `
		INSERT INTO memories (
			id, agent_id, kind, status, content, confidence, salience,
			scope_type, scope_id, source_event_id, metadata_json,
			created_at, updated_at
		) VALUES (?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?)
	`,
		id,
		input.AgentID,
		input.Kind,
		StatusCandidate,
		strings.TrimSpace(input.Content),
		input.Confidence,
		input.Salience,
		nullIfEmpty(input.ScopeType),
		nullIfEmpty(input.ScopeID),
		input.SourceEventID,
		string(metadataJSON),
		formatTime(now),
		formatTime(now),
	); err != nil {
		return Memory{}, fmt.Errorf("create memory candidate: %w", err)
	}

	if _, err := events.NewStore(s.db).AppendTx(ctx, tx, events.AppendInput{
		Type:          "memory.candidate.created",
		ActorType:     "agent",
		ActorID:       input.AgentID,
		SubjectType:   "memory",
		SubjectID:     id,
		ScopeType:     input.ScopeType,
		ScopeID:       input.ScopeID,
		ParentEventID: input.SourceEventID,
		Payload: map[string]interface{}{
			"kind":       input.Kind,
			"confidence": input.Confidence,
			"salience":   input.Salience,
		},
		Metadata: map[string]interface{}{
			"source_event_id": input.SourceEventID,
		},
	}); err != nil {
		return Memory{}, fmt.Errorf("record memory candidate event: %w", err)
	}

	if err := tx.Commit(); err != nil {
		return Memory{}, fmt.Errorf("commit memory candidate: %w", err)
	}

	return Memory{
		ID:            id,
		AgentID:       input.AgentID,
		Kind:          input.Kind,
		Status:        StatusCandidate,
		Content:       strings.TrimSpace(input.Content),
		Confidence:    input.Confidence,
		Salience:      input.Salience,
		ScopeType:     input.ScopeType,
		ScopeID:       input.ScopeID,
		SourceEventID: input.SourceEventID,
		Metadata:      metadata,
		CreatedAt:     now,
		UpdatedAt:     now,
	}, nil
}

func (s *Store) Promote(ctx context.Context, id string, input PromoteInput) (Memory, error) {
	if strings.TrimSpace(id) == "" {
		return Memory{}, errors.New("memory id is required")
	}

	tx, err := s.db.Conn().BeginTx(ctx, nil)
	if err != nil {
		return Memory{}, fmt.Errorf("begin promotion transaction: %w", err)
	}
	defer tx.Rollback()

	current, err := scanMemory(tx.QueryRowContext(ctx, `
		SELECT id, agent_id, kind, status, content, confidence, salience,
		       scope_type, scope_id, source_event_id, metadata_json,
		       created_at, updated_at, promoted_at
		FROM memories
		WHERE id = ?
	`, id))
	if err != nil {
		return Memory{}, err
	}
	if current.Status != StatusCandidate {
		return Memory{}, fmt.Errorf("memory %s is %s, expected candidate", id, current.Status)
	}

	now := time.Now().UTC()
	result, err := tx.ExecContext(ctx, `
		UPDATE memories
		SET status = ?, promoted_at = ?, updated_at = ?
		WHERE id = ? AND status = ?
	`, StatusDurable, formatTime(now), formatTime(now), id, StatusCandidate)
	if err != nil {
		return Memory{}, fmt.Errorf("promote memory: %w", err)
	}
	affected, err := result.RowsAffected()
	if err != nil {
		return Memory{}, fmt.Errorf("read promotion result: %w", err)
	}
	if affected != 1 {
		return Memory{}, fmt.Errorf("memory %s was not promoted", id)
	}

	actorType := strings.TrimSpace(input.ActorType)
	if actorType == "" {
		actorType = "system"
	}
	if _, err := events.NewStore(s.db).AppendTx(ctx, tx, events.AppendInput{
		Type:          "memory.promoted",
		ActorType:     actorType,
		ActorID:       strings.TrimSpace(input.ActorID),
		SubjectType:   "memory",
		SubjectID:     id,
		ScopeType:     current.ScopeType,
		ScopeID:       current.ScopeID,
		ParentEventID: current.SourceEventID,
		Payload: map[string]interface{}{
			"kind":       current.Kind,
			"confidence": current.Confidence,
			"salience":   current.Salience,
			"reason":     strings.TrimSpace(input.Reason),
		},
	}); err != nil {
		return Memory{}, fmt.Errorf("record memory promotion event: %w", err)
	}

	if err := tx.Commit(); err != nil {
		return Memory{}, fmt.Errorf("commit memory promotion: %w", err)
	}

	current.Status = StatusDurable
	current.PromotedAt = &now
	current.UpdatedAt = now
	return current, nil
}

func (s *Store) Get(ctx context.Context, id string) (Memory, error) {
	return scanMemory(s.db.Conn().QueryRowContext(ctx, `
		SELECT id, agent_id, kind, status, content, confidence, salience,
		       scope_type, scope_id, source_event_id, metadata_json,
		       created_at, updated_at, promoted_at
		FROM memories
		WHERE id = ?
	`, id))
}

func validateCandidate(input CreateCandidateInput) error {
	if strings.TrimSpace(input.AgentID) == "" {
		return errors.New("agent id is required")
	}
	switch input.Kind {
	case KindEpisodic, KindSemantic, KindProcedural:
	default:
		return fmt.Errorf("invalid memory kind %q", input.Kind)
	}
	if strings.TrimSpace(input.Content) == "" {
		return errors.New("memory content is required")
	}
	if input.Confidence < 0 || input.Confidence > 1 {
		return errors.New("memory confidence must be between 0 and 1")
	}
	if input.Salience < 0 || input.Salience > 1 {
		return errors.New("memory salience must be between 0 and 1")
	}
	if strings.TrimSpace(input.SourceEventID) == "" {
		return errors.New("source event id is required")
	}
	return nil
}

type scanner interface {
	Scan(dest ...interface{}) error
}

func scanMemory(row scanner) (Memory, error) {
	var memory Memory
	var scopeType, scopeID sql.NullString
	var metadataJSON, createdAt, updatedAt string
	var promotedAt sql.NullString

	if err := row.Scan(
		&memory.ID,
		&memory.AgentID,
		&memory.Kind,
		&memory.Status,
		&memory.Content,
		&memory.Confidence,
		&memory.Salience,
		&scopeType,
		&scopeID,
		&memory.SourceEventID,
		&metadataJSON,
		&createdAt,
		&updatedAt,
		&promotedAt,
	); err != nil {
		return Memory{}, err
	}

	memory.ScopeType = scopeType.String
	memory.ScopeID = scopeID.String
	if err := json.Unmarshal([]byte(metadataJSON), &memory.Metadata); err != nil {
		return Memory{}, fmt.Errorf("decode memory metadata: %w", err)
	}
	if memory.Metadata == nil {
		memory.Metadata = map[string]interface{}{}
	}

	var err error
	memory.CreatedAt, err = time.Parse(time.RFC3339Nano, createdAt)
	if err != nil {
		return Memory{}, fmt.Errorf("parse memory created_at: %w", err)
	}
	memory.UpdatedAt, err = time.Parse(time.RFC3339Nano, updatedAt)
	if err != nil {
		return Memory{}, fmt.Errorf("parse memory updated_at: %w", err)
	}
	if promotedAt.Valid {
		value, err := time.Parse(time.RFC3339Nano, promotedAt.String)
		if err != nil {
			return Memory{}, fmt.Errorf("parse memory promoted_at: %w", err)
		}
		memory.PromotedAt = &value
	}

	return memory, nil
}

func newID(prefix string) (string, error) {
	var raw [16]byte
	if _, err := rand.Read(raw[:]); err != nil {
		return "", fmt.Errorf("generate id: %w", err)
	}
	return prefix + hex.EncodeToString(raw[:]), nil
}

func nullIfEmpty(value string) interface{} {
	if strings.TrimSpace(value) == "" {
		return nil
	}
	return strings.TrimSpace(value)
}

func formatTime(value time.Time) string {
	return value.UTC().Format(time.RFC3339Nano)
}

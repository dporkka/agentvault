// Package events provides the append-only provenance substrate for AgentVault.
package events

import (
	"context"
	"crypto/rand"
	"encoding/hex"
	"encoding/json"
	"errors"
	"fmt"
	"time"

	"github.com/agentvault/core/internal/db"
)

type Event struct {
	ID             string                 `json:"id"`
	Type           string                 `json:"type"`
	ActorType      string                 `json:"actorType,omitempty"`
	ActorID        string                 `json:"actorId,omitempty"`
	SubjectType    string                 `json:"subjectType,omitempty"`
	SubjectID      string                 `json:"subjectId,omitempty"`
	ScopeType      string                 `json:"scopeType,omitempty"`
	ScopeID        string                 `json:"scopeId,omitempty"`
	RunID          string                 `json:"runId,omitempty"`
	ConversationID string                 `json:"conversationId,omitempty"`
	SourceID       string                 `json:"sourceId,omitempty"`
	ParentEventID  string                 `json:"parentEventId,omitempty"`
	Payload        map[string]interface{} `json:"payload"`
	Metadata       map[string]interface{} `json:"metadata"`
	OccurredAt     time.Time              `json:"occurredAt"`
	RecordedAt     time.Time              `json:"recordedAt"`
}

type AppendInput struct {
	Type           string
	ActorType      string
	ActorID        string
	SubjectType    string
	SubjectID      string
	ScopeType      string
	ScopeID        string
	RunID          string
	ConversationID string
	SourceID       string
	ParentEventID  string
	Payload        map[string]interface{}
	Metadata       map[string]interface{}
	OccurredAt     time.Time
}

type Store struct {
	db *db.DB
}

func NewStore(database *db.DB) *Store {
	return &Store{db: database}
}

func (s *Store) Append(ctx context.Context, input AppendInput) (Event, error) {
	if input.Type == "" {
		return Event{}, errors.New("event type is required")
	}

	payload := input.Payload
	if payload == nil {
		payload = map[string]interface{}{}
	}
	metadata := input.Metadata
	if metadata == nil {
		metadata = map[string]interface{}{}
	}
	payloadJSON, err := json.Marshal(payload)
	if err != nil {
		return Event{}, fmt.Errorf("marshal event payload: %w", err)
	}
	metadataJSON, err := json.Marshal(metadata)
	if err != nil {
		return Event{}, fmt.Errorf("marshal event metadata: %w", err)
	}

	id, err := newID("evt_")
	if err != nil {
		return Event{}, err
	}
	recordedAt := time.Now().UTC()
	occurredAt := input.OccurredAt.UTC()
	if input.OccurredAt.IsZero() {
		occurredAt = recordedAt
	}

	_, err = s.db.Conn().ExecContext(ctx, `
		INSERT INTO events (
			id, event_type, actor_type, actor_id, subject_type, subject_id,
			scope_type, scope_id, run_id, conversation_id, source_id,
			parent_event_id, payload_json, metadata_json, occurred_at, recorded_at
		) VALUES (?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?)
	`,
		id,
		input.Type,
		nullIfEmpty(input.ActorType),
		nullIfEmpty(input.ActorID),
		nullIfEmpty(input.SubjectType),
		nullIfEmpty(input.SubjectID),
		nullIfEmpty(input.ScopeType),
		nullIfEmpty(input.ScopeID),
		nullIfEmpty(input.RunID),
		nullIfEmpty(input.ConversationID),
		nullIfEmpty(input.SourceID),
		nullIfEmpty(input.ParentEventID),
		string(payloadJSON),
		string(metadataJSON),
		formatTime(occurredAt),
		formatTime(recordedAt),
	)
	if err != nil {
		return Event{}, fmt.Errorf("append event: %w", err)
	}

	return Event{
		ID:             id,
		Type:           input.Type,
		ActorType:      input.ActorType,
		ActorID:        input.ActorID,
		SubjectType:    input.SubjectType,
		SubjectID:      input.SubjectID,
		ScopeType:      input.ScopeType,
		ScopeID:        input.ScopeID,
		RunID:          input.RunID,
		ConversationID: input.ConversationID,
		SourceID:       input.SourceID,
		ParentEventID:  input.ParentEventID,
		Payload:        payload,
		Metadata:       metadata,
		OccurredAt:     occurredAt,
		RecordedAt:     recordedAt,
	}, nil
}

func (s *Store) Get(ctx context.Context, id string) (Event, error) {
	return scanEvent(s.db.Conn().QueryRowContext(ctx, `
		SELECT id, event_type, actor_type, actor_id, subject_type, subject_id,
		       scope_type, scope_id, run_id, conversation_id, source_id,
		       parent_event_id, payload_json, metadata_json, occurred_at, recorded_at
		FROM events
		WHERE id = ?
	`, id))
}

type scanner interface {
	Scan(dest ...interface{}) error
}

func scanEvent(row scanner) (Event, error) {
	var event Event
	var actorType, actorID, subjectType, subjectID nullableString
	var scopeType, scopeID, runID, conversationID nullableString
	var sourceID, parentEventID nullableString
	var payloadJSON, metadataJSON, occurredAt, recordedAt string

	if err := row.Scan(
		&event.ID,
		&event.Type,
		&actorType,
		&actorID,
		&subjectType,
		&subjectID,
		&scopeType,
		&scopeID,
		&runID,
		&conversationID,
		&sourceID,
		&parentEventID,
		&payloadJSON,
		&metadataJSON,
		&occurredAt,
		&recordedAt,
	); err != nil {
		return Event{}, err
	}

	event.ActorType = actorType.String
	event.ActorID = actorID.String
	event.SubjectType = subjectType.String
	event.SubjectID = subjectID.String
	event.ScopeType = scopeType.String
	event.ScopeID = scopeID.String
	event.RunID = runID.String
	event.ConversationID = conversationID.String
	event.SourceID = sourceID.String
	event.ParentEventID = parentEventID.String

	if err := json.Unmarshal([]byte(payloadJSON), &event.Payload); err != nil {
		return Event{}, fmt.Errorf("decode event payload: %w", err)
	}
	if event.Payload == nil {
		event.Payload = map[string]interface{}{}
	}
	if err := json.Unmarshal([]byte(metadataJSON), &event.Metadata); err != nil {
		return Event{}, fmt.Errorf("decode event metadata: %w", err)
	}
	if event.Metadata == nil {
		event.Metadata = map[string]interface{}{}
	}

	var err error
	event.OccurredAt, err = time.Parse(time.RFC3339Nano, occurredAt)
	if err != nil {
		return Event{}, fmt.Errorf("parse event occurred_at: %w", err)
	}
	event.RecordedAt, err = time.Parse(time.RFC3339Nano, recordedAt)
	if err != nil {
		return Event{}, fmt.Errorf("parse event recorded_at: %w", err)
	}

	return event, nil
}

// nullableString implements sql.Scanner without exporting database/sql details
// into the Event API.
type nullableString struct {
	String string
	Valid  bool
}

func (n *nullableString) Scan(src interface{}) error {
	if src == nil {
		n.String = ""
		n.Valid = false
		return nil
	}
	switch value := src.(type) {
	case string:
		n.String = value
	case []byte:
		n.String = string(value)
	default:
		return fmt.Errorf("cannot scan %T into nullableString", src)
	}
	n.Valid = true
	return nil
}

func nullIfEmpty(value string) interface{} {
	if value == "" {
		return nil
	}
	return value
}

func newID(prefix string) (string, error) {
	var raw [16]byte
	if _, err := rand.Read(raw[:]); err != nil {
		return "", fmt.Errorf("generate id: %w", err)
	}
	return prefix + hex.EncodeToString(raw[:]), nil
}

func formatTime(t time.Time) string {
	return t.UTC().Format(time.RFC3339Nano)
}

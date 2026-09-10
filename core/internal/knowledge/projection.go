package knowledge

import (
	"encoding/json"
	"fmt"

	"github.com/agentvault/core/internal/contract"
)

const (
	eventProvenanceCreated = "provenance.created"
	eventObjectUpserted     = "object.upserted"
	eventRelationCreated    = "relation.created"
	eventMemoryRecorded     = "memory.recorded"
	eventSessionStarted     = "session.started"
	eventSessionEvent       = "session.event"
	eventSessionClosed      = "session.closed"
)

type sessionCloseProjection struct {
	ID        string `json:"id"`
	Status    string `json:"status"`
	UpdatedAt string `json:"updatedAt"`
	EndedAt   string `json:"endedAt"`
}

// ReplayJournal rebuilds the structured SQLite projection from the canonical
// JSONL journal. Projection operations are idempotent so replay is safe on an
// already-populated database as well as a fresh one.
func (s *Store) ReplayJournal() error {
	if s.journal == nil {
		return nil
	}
	return s.journal.Replay(s.projectJournalEvent)
}

func (s *Store) projectJournalEvent(event JournalEvent) error {
	switch event.Type {
	case eventProvenanceCreated:
		var record contract.ProvenanceRecord
		if err := json.Unmarshal(event.Payload, &record); err != nil {
			return err
		}
		return s.projectProvenance(record)
	case eventObjectUpserted:
		var object contract.KnowledgeObject
		if err := json.Unmarshal(event.Payload, &object); err != nil {
			return err
		}
		return s.projectObject(object)
	case eventRelationCreated:
		var relation contract.ObjectRelation
		if err := json.Unmarshal(event.Payload, &relation); err != nil {
			return err
		}
		return s.projectRelation(relation)
	case eventMemoryRecorded:
		var memory contract.MemoryRecord
		if err := json.Unmarshal(event.Payload, &memory); err != nil {
			return err
		}
		return s.projectMemory(memory)
	case eventSessionStarted:
		var session contract.AgentSession
		if err := json.Unmarshal(event.Payload, &session); err != nil {
			return err
		}
		return s.projectSessionStart(session)
	case eventSessionEvent:
		var sessionEvent contract.SessionEvent
		if err := json.Unmarshal(event.Payload, &sessionEvent); err != nil {
			return err
		}
		return s.projectSessionEvent(sessionEvent)
	case eventSessionClosed:
		var closeEvent sessionCloseProjection
		if err := json.Unmarshal(event.Payload, &closeEvent); err != nil {
			return err
		}
		return s.projectSessionClose(closeEvent)
	default:
		handled, err := s.projectMutationJournalEvent(event)
		if handled {
			return err
		}
		return fmt.Errorf("unknown knowledge journal event type %q", event.Type)
	}
}

func (s *Store) projectProvenance(record contract.ProvenanceRecord) error {
	evidence, err := json.Marshal(orEmptySlice(record.Evidence))
	if err != nil {
		return fmt.Errorf("marshal provenance evidence: %w", err)
	}
	metadata, err := json.Marshal(orEmptyMap(record.Metadata))
	if err != nil {
		return fmt.Errorf("marshal provenance metadata: %w", err)
	}
	_, err = s.db.Exec(`
		INSERT OR IGNORE INTO provenance_records (
			id, source_type, source_id, agent_id, session_id, model, confidence,
			observed_at, evidence_json, metadata_json, created_at
		) VALUES (?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?)`,
		record.ID, record.SourceType, nullIfEmpty(record.SourceID), nullIfEmpty(record.AgentID),
		nullIfEmpty(record.SessionID), nullIfEmpty(record.Model), record.Confidence,
		record.ObservedAt, string(evidence), string(metadata), record.CreatedAt,
	)
	if err != nil {
		return fmt.Errorf("project provenance: %w", err)
	}
	return nil
}

func (s *Store) projectObject(object contract.KnowledgeObject) error {
	data, err := json.Marshal(orEmptyMap(object.Data))
	if err != nil {
		return fmt.Errorf("marshal object data: %w", err)
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
			created_at = excluded.created_at,
			updated_at = excluded.updated_at`,
		object.ID, object.Type, object.Title, nullIfEmpty(object.Status), nullIfEmpty(object.Organization),
		nullIfEmpty(object.Project), nullIfEmpty(object.CanonicalPath), string(data),
		nullIfEmpty(object.ProvenanceID), object.CreatedAt, object.UpdatedAt,
	)
	if err != nil {
		return fmt.Errorf("project object: %w", err)
	}
	return nil
}

func (s *Store) projectRelation(relation contract.ObjectRelation) error {
	metadata, err := json.Marshal(orEmptyMap(relation.Metadata))
	if err != nil {
		return fmt.Errorf("marshal relation metadata: %w", err)
	}
	_, err = s.db.Exec(`
		INSERT OR IGNORE INTO object_relations (
			id, from_object_id, to_object_id, relation_type, valid_from, valid_to,
			confidence, provenance_id, metadata_json, created_at, updated_at
		) VALUES (?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?)`,
		relation.ID, relation.FromObjectID, relation.ToObjectID, relation.RelationType,
		nullIfEmpty(relation.ValidFrom), nullIfEmpty(relation.ValidTo), relation.Confidence,
		nullIfEmpty(relation.ProvenanceID), string(metadata), relation.CreatedAt, relation.UpdatedAt,
	)
	if err != nil {
		return fmt.Errorf("project relation: %w", err)
	}
	return nil
}

func (s *Store) projectMemory(memory contract.MemoryRecord) error {
	metadata, err := json.Marshal(orEmptyMap(memory.Metadata))
	if err != nil {
		return fmt.Errorf("marshal memory metadata: %w", err)
	}
	_, err = s.db.Exec(`
		INSERT OR IGNORE INTO memory_records (
			id, memory_type, scope_type, scope_id, content, object_id, provenance_id,
			confidence, valid_from, valid_to, supersedes_id, metadata_json, created_at, updated_at
		) VALUES (?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?)`,
		memory.ID, memory.MemoryType, memory.ScopeType, memory.ScopeID, memory.Content,
		nullIfEmpty(memory.ObjectID), nullIfEmpty(memory.ProvenanceID), memory.Confidence,
		nullIfEmpty(memory.ValidFrom), nullIfEmpty(memory.ValidTo), nullIfEmpty(memory.SupersedesID),
		string(metadata), memory.CreatedAt, memory.UpdatedAt,
	)
	if err != nil {
		return fmt.Errorf("project memory: %w", err)
	}
	return nil
}

func (s *Store) projectSessionStart(session contract.AgentSession) error {
	contextJSON, err := json.Marshal(orEmptyMap(session.Context))
	if err != nil {
		return fmt.Errorf("marshal session context: %w", err)
	}
	_, err = s.db.Exec(`
		INSERT OR IGNORE INTO agent_sessions (
			id, agent_id, project, objective, status, branch, worktree, context_json,
			started_at, updated_at, ended_at
		) VALUES (?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?)`,
		session.ID, session.AgentID, nullIfEmpty(session.Project), session.Objective, session.Status,
		nullIfEmpty(session.Branch), nullIfEmpty(session.Worktree), string(contextJSON), session.StartedAt,
		session.UpdatedAt, nullIfEmpty(session.EndedAt),
	)
	if err != nil {
		return fmt.Errorf("project session start: %w", err)
	}
	return nil
}

func (s *Store) projectSessionEvent(event contract.SessionEvent) error {
	payload, err := json.Marshal(orEmptyMap(event.Payload))
	if err != nil {
		return fmt.Errorf("marshal session event payload: %w", err)
	}

	tx, err := s.db.Begin()
	if err != nil {
		return err
	}
	defer tx.Rollback()

	result, err := tx.Exec(`
		UPDATE agent_sessions
		SET updated_at = CASE WHEN updated_at < ? THEN ? ELSE updated_at END
		WHERE id = ?`, event.CreatedAt, event.CreatedAt, event.SessionID)
	if err != nil {
		return fmt.Errorf("project session timestamp: %w", err)
	}
	rows, err := result.RowsAffected()
	if err != nil {
		return err
	}
	if rows == 0 {
		return fmt.Errorf("session %s does not exist", event.SessionID)
	}
	_, err = tx.Exec(`
		INSERT OR IGNORE INTO session_events (
			id, session_id, event_type, payload_json, provenance_id, created_at
		) VALUES (?, ?, ?, ?, ?, ?)`,
		event.ID, event.SessionID, event.EventType, string(payload), nullIfEmpty(event.ProvenanceID), event.CreatedAt,
	)
	if err != nil {
		return fmt.Errorf("project session event: %w", err)
	}
	if err := tx.Commit(); err != nil {
		return err
	}
	return nil
}

func (s *Store) projectSessionClose(closeEvent sessionCloseProjection) error {
	result, err := s.db.Exec(`
		UPDATE agent_sessions
		SET status = ?, updated_at = ?, ended_at = ?
		WHERE id = ?`, closeEvent.Status, closeEvent.UpdatedAt, closeEvent.EndedAt, closeEvent.ID)
	if err != nil {
		return fmt.Errorf("project session close: %w", err)
	}
	rows, err := result.RowsAffected()
	if err != nil {
		return err
	}
	if rows == 0 {
		return fmt.Errorf("session %s does not exist", closeEvent.ID)
	}
	return nil
}

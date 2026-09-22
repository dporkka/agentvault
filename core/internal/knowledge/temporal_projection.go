package knowledge

import (
	"encoding/json"
	"fmt"

	"github.com/agentvault/core/internal/contract"
)

func (s *Store) projectEpisode(episode contract.EpisodeRecord) error {
	objectIDs, err := json.Marshal(normalizeIDs(episode.ObjectIDs))
	if err != nil {
		return fmt.Errorf("marshal episode object ids: %w", err)
	}
	metadata, err := json.Marshal(orEmptyMap(episode.Metadata))
	if err != nil {
		return fmt.Errorf("marshal episode metadata: %w", err)
	}
	_, err = s.db.Exec(`
		INSERT OR IGNORE INTO episodes (
			id, scope_type, scope_id, event_type, summary, object_ids_json,
			provenance_id, occurred_at, ended_at, metadata_json, created_at
		) VALUES (?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?)`,
		episode.ID, episode.ScopeType, episode.ScopeID, episode.EventType, episode.Summary,
		string(objectIDs), nullIfEmpty(episode.ProvenanceID), episode.OccurredAt,
		nullIfEmpty(episode.EndedAt), string(metadata), episode.CreatedAt,
	)
	if err != nil {
		return fmt.Errorf("project episode: %w", err)
	}
	return nil
}

func (s *Store) projectFact(fact contract.TemporalFact) error {
	metadata, err := json.Marshal(orEmptyMap(fact.Metadata))
	if err != nil {
		return fmt.Errorf("marshal fact metadata: %w", err)
	}

	tx, err := s.db.Begin()
	if err != nil {
		return err
	}
	defer tx.Rollback()

	_, err = tx.Exec(`
		INSERT OR IGNORE INTO temporal_facts (
			id, subject_id, predicate, object_id, value, provenance_id, confidence,
			valid_from, valid_to, supersedes_id, superseded_at, superseded_by,
			metadata_json, created_at, updated_at
		) VALUES (?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?)`,
		fact.ID, fact.SubjectID, fact.Predicate, nullIfEmpty(fact.ObjectID), nullIfEmpty(fact.Value),
		nullIfEmpty(fact.ProvenanceID), fact.Confidence, nullIfEmpty(fact.ValidFrom),
		nullIfEmpty(fact.ValidTo), nullIfEmpty(fact.SupersedesID), nullIfEmpty(fact.SupersededAt),
		nullIfEmpty(fact.SupersededBy), string(metadata), fact.CreatedAt, fact.UpdatedAt,
	)
	if err != nil {
		return fmt.Errorf("project fact: %w", err)
	}

	if fact.SupersedesID != "" {
		result, err := tx.Exec(`
			UPDATE temporal_facts
			SET superseded_at = ?, superseded_by = ?, updated_at = ?
			WHERE id = ? AND (superseded_by IS NULL OR superseded_by = '' OR superseded_by = ?)`,
			fact.CreatedAt, fact.ID, fact.CreatedAt, fact.SupersedesID, fact.ID,
		)
		if err != nil {
			return fmt.Errorf("project fact supersession: %w", err)
		}
		rows, err := result.RowsAffected()
		if err != nil {
			return err
		}
		if rows == 0 {
			return fmt.Errorf("superseded fact %s missing or already superseded", fact.SupersedesID)
		}
	}
	if err := tx.Commit(); err != nil {
		return err
	}
	return nil
}

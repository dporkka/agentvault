package knowledge

import (
	"encoding/json"
	"errors"
	"fmt"
	"strings"
	"time"

	"github.com/agentvault/core/internal/contract"
)

const (
	eventEpisodeRecorded = "episode.recorded"
	eventFactRecorded    = "fact.recorded"
)

// RecordEpisode stores an immutable occurrence. The occurrence clock is
// independent from provenance.observedAt so AgentVault can distinguish when an
// event happened from when it was learned.
func (s *Store) RecordEpisode(req contract.CreateEpisodeRequest) (contract.EpisodeRecord, error) {
	if strings.TrimSpace(req.ScopeType) == "" || strings.TrimSpace(req.ScopeID) == "" {
		return contract.EpisodeRecord{}, errors.New("scopeType and scopeId are required")
	}
	if strings.TrimSpace(req.EventType) == "" {
		return contract.EpisodeRecord{}, errors.New("eventType is required")
	}
	if strings.TrimSpace(req.Summary) == "" {
		return contract.EpisodeRecord{}, errors.New("summary is required")
	}
	if err := s.validateOptionalProvenance(req.ProvenanceID); err != nil {
		return contract.EpisodeRecord{}, err
	}
	for _, objectID := range req.ObjectIDs {
		if err := s.validateOptionalObject(objectID); err != nil {
			return contract.EpisodeRecord{}, err
		}
	}

	now := time.Now().UTC().Format(time.RFC3339Nano)
	occurredAt := strings.TrimSpace(req.OccurredAt)
	if occurredAt == "" && req.ProvenanceID != "" {
		if provenance, err := s.GetProvenance(req.ProvenanceID); err == nil {
			occurredAt = provenance.ObservedAt
		}
	}
	if occurredAt == "" {
		occurredAt = now
	}
	if err := validateTemporalInterval(occurredAt, req.EndedAt); err != nil {
		return contract.EpisodeRecord{}, err
	}

	id := strings.TrimSpace(req.ID)
	if id == "" {
		id = newID("episode")
	}
	if err := s.ensureIDAbsent("episodes", id); err != nil {
		return contract.EpisodeRecord{}, err
	}

	episode := contract.EpisodeRecord{
		ID:           id,
		ScopeType:    req.ScopeType,
		ScopeID:      req.ScopeID,
		EventType:    req.EventType,
		Summary:      req.Summary,
		ObjectIDs:    normalizeIDs(req.ObjectIDs),
		ProvenanceID: req.ProvenanceID,
		OccurredAt:   occurredAt,
		EndedAt:      req.EndedAt,
		Metadata:     orEmptyMap(req.Metadata),
		CreatedAt:    now,
	}
	if err := s.persist(eventEpisodeRecorded, episode, func() error {
		return s.projectEpisode(episode)
	}); err != nil {
		return contract.EpisodeRecord{}, err
	}
	return episode, nil
}

// GetEpisode returns one immutable episode.
func (s *Store) GetEpisode(id string) (contract.EpisodeRecord, error) {
	var episode contract.EpisodeRecord
	var objectIDsJSON, metadataJSON string
	err := s.db.QueryRow(`
		SELECT id, scope_type, scope_id, event_type, summary, object_ids_json,
		       COALESCE(provenance_id, ''), occurred_at, COALESCE(ended_at, ''),
		       metadata_json, created_at
		FROM episodes WHERE id = ?`, id).Scan(
		&episode.ID, &episode.ScopeType, &episode.ScopeID, &episode.EventType, &episode.Summary,
		&objectIDsJSON, &episode.ProvenanceID, &episode.OccurredAt, &episode.EndedAt,
		&metadataJSON, &episode.CreatedAt,
	)
	if err != nil {
		return contract.EpisodeRecord{}, err
	}
	if err := json.Unmarshal([]byte(objectIDsJSON), &episode.ObjectIDs); err != nil {
		return contract.EpisodeRecord{}, fmt.Errorf("decode episode object ids: %w", err)
	}
	if err := json.Unmarshal([]byte(metadataJSON), &episode.Metadata); err != nil {
		return contract.EpisodeRecord{}, fmt.Errorf("decode episode metadata: %w", err)
	}
	return episode, nil
}

// ListEpisodes returns newest episodes for one visibility scope.
func (s *Store) ListEpisodes(scopeType, scopeID string, limit int) ([]contract.EpisodeRecord, error) {
	if strings.TrimSpace(scopeType) == "" || strings.TrimSpace(scopeID) == "" {
		return nil, errors.New("scopeType and scopeId are required")
	}
	if limit <= 0 || limit > 1000 {
		limit = defaultLimit
	}
	rows, err := s.db.Query(`
		SELECT id, scope_type, scope_id, event_type, summary, object_ids_json,
		       COALESCE(provenance_id, ''), occurred_at, COALESCE(ended_at, ''),
		       metadata_json, created_at
		FROM episodes
		WHERE scope_type = ? AND scope_id = ?
		ORDER BY occurred_at DESC, created_at DESC
		LIMIT ?`, scopeType, scopeID, limit)
	if err != nil {
		return nil, fmt.Errorf("list episodes: %w", err)
	}
	defer rows.Close()

	result := make([]contract.EpisodeRecord, 0)
	for rows.Next() {
		var episode contract.EpisodeRecord
		var objectIDsJSON, metadataJSON string
		if err := rows.Scan(
			&episode.ID, &episode.ScopeType, &episode.ScopeID, &episode.EventType, &episode.Summary,
			&objectIDsJSON, &episode.ProvenanceID, &episode.OccurredAt, &episode.EndedAt,
			&metadataJSON, &episode.CreatedAt,
		); err != nil {
			return nil, err
		}
		if err := json.Unmarshal([]byte(objectIDsJSON), &episode.ObjectIDs); err != nil {
			return nil, fmt.Errorf("decode episode object ids: %w", err)
		}
		if err := json.Unmarshal([]byte(metadataJSON), &episode.Metadata); err != nil {
			return nil, fmt.Errorf("decode episode metadata: %w", err)
		}
		result = append(result, episode)
	}
	return result, rows.Err()
}

// RecordFact records a truth claim without rewriting historical claims.
// Supersession is constrained to the same subject+predicate, preventing an
// unrelated fact from invalidating another claim.
func (s *Store) RecordFact(req contract.CreateTemporalFactRequest) (contract.TemporalFact, error) {
	if strings.TrimSpace(req.SubjectID) == "" {
		return contract.TemporalFact{}, errors.New("subjectId is required")
	}
	if strings.TrimSpace(req.Predicate) == "" {
		return contract.TemporalFact{}, errors.New("predicate is required")
	}
	if strings.TrimSpace(req.ObjectID) == "" && strings.TrimSpace(req.Value) == "" {
		return contract.TemporalFact{}, errors.New("objectId or value is required")
	}
	if err := s.validateOptionalObject(req.SubjectID); err != nil {
		return contract.TemporalFact{}, fmt.Errorf("subject: %w", err)
	}
	if err := s.validateOptionalObject(req.ObjectID); err != nil {
		return contract.TemporalFact{}, fmt.Errorf("object: %w", err)
	}
	if err := s.validateOptionalProvenance(req.ProvenanceID); err != nil {
		return contract.TemporalFact{}, err
	}
	if err := validateTemporalInterval(req.ValidFrom, req.ValidTo); err != nil {
		return contract.TemporalFact{}, err
	}

	confidence := 1.0
	if req.Confidence != nil {
		confidence = *req.Confidence
	}
	if confidence < 0 || confidence > 1 {
		return contract.TemporalFact{}, errors.New("confidence must be between 0 and 1")
	}

	if req.SupersedesID != "" {
		previous, err := s.GetFact(req.SupersedesID)
		if err != nil {
			return contract.TemporalFact{}, fmt.Errorf("superseded fact: %w", err)
		}
		if previous.SubjectID != req.SubjectID || previous.Predicate != req.Predicate {
			return contract.TemporalFact{}, errors.New("superseded fact must have the same subjectId and predicate")
		}
		if previous.SupersededBy != "" {
			return contract.TemporalFact{}, fmt.Errorf("fact %s is already superseded by %s", previous.ID, previous.SupersededBy)
		}
	}

	id := strings.TrimSpace(req.ID)
	if id == "" {
		id = newID("fact")
	}
	if err := s.ensureIDAbsent("temporal_facts", id); err != nil {
		return contract.TemporalFact{}, err
	}
	now := time.Now().UTC().Format(time.RFC3339Nano)
	fact := contract.TemporalFact{
		ID:           id,
		SubjectID:    req.SubjectID,
		Predicate:    req.Predicate,
		ObjectID:     req.ObjectID,
		Value:        req.Value,
		ProvenanceID: req.ProvenanceID,
		Confidence:   confidence,
		ValidFrom:    req.ValidFrom,
		ValidTo:      req.ValidTo,
		SupersedesID: req.SupersedesID,
		Metadata:     orEmptyMap(req.Metadata),
		CreatedAt:    now,
		UpdatedAt:    now,
	}
	if err := s.persist(eventFactRecorded, fact, func() error {
		return s.projectFact(fact)
	}); err != nil {
		return contract.TemporalFact{}, err
	}
	return fact, nil
}

// GetFact returns a fact including derived supersession state.
func (s *Store) GetFact(id string) (contract.TemporalFact, error) {
	var fact contract.TemporalFact
	var metadataJSON string
	err := s.db.QueryRow(`
		SELECT id, subject_id, predicate, COALESCE(object_id, ''), COALESCE(value, ''),
		       COALESCE(provenance_id, ''), confidence, COALESCE(valid_from, ''),
		       COALESCE(valid_to, ''), COALESCE(supersedes_id, ''),
		       COALESCE(superseded_at, ''), COALESCE(superseded_by, ''),
		       metadata_json, created_at, updated_at
		FROM temporal_facts WHERE id = ?`, id).Scan(
		&fact.ID, &fact.SubjectID, &fact.Predicate, &fact.ObjectID, &fact.Value,
		&fact.ProvenanceID, &fact.Confidence, &fact.ValidFrom, &fact.ValidTo,
		&fact.SupersedesID, &fact.SupersededAt, &fact.SupersededBy,
		&metadataJSON, &fact.CreatedAt, &fact.UpdatedAt,
	)
	if err != nil {
		return contract.TemporalFact{}, err
	}
	if err := json.Unmarshal([]byte(metadataJSON), &fact.Metadata); err != nil {
		return contract.TemporalFact{}, fmt.Errorf("decode fact metadata: %w", err)
	}
	return fact, nil
}

// FactsForObject returns facts where objectID is the subject or linked object.
func (s *Store) FactsForObject(objectID string, limit int) ([]contract.TemporalFact, error) {
	if strings.TrimSpace(objectID) == "" {
		return nil, errors.New("object id is required")
	}
	if limit <= 0 || limit > 1000 {
		limit = defaultLimit
	}
	rows, err := s.db.Query(`
		SELECT id, subject_id, predicate, COALESCE(object_id, ''), COALESCE(value, ''),
		       COALESCE(provenance_id, ''), confidence, COALESCE(valid_from, ''),
		       COALESCE(valid_to, ''), COALESCE(supersedes_id, ''),
		       COALESCE(superseded_at, ''), COALESCE(superseded_by, ''),
		       metadata_json, created_at, updated_at
		FROM temporal_facts
		WHERE subject_id = ? OR object_id = ?
		ORDER BY updated_at DESC
		LIMIT ?`, objectID, objectID, limit)
	if err != nil {
		return nil, fmt.Errorf("list facts for object: %w", err)
	}
	defer rows.Close()

	facts := make([]contract.TemporalFact, 0)
	for rows.Next() {
		var fact contract.TemporalFact
		var metadataJSON string
		if err := rows.Scan(
			&fact.ID, &fact.SubjectID, &fact.Predicate, &fact.ObjectID, &fact.Value,
			&fact.ProvenanceID, &fact.Confidence, &fact.ValidFrom, &fact.ValidTo,
			&fact.SupersedesID, &fact.SupersededAt, &fact.SupersededBy,
			&metadataJSON, &fact.CreatedAt, &fact.UpdatedAt,
		); err != nil {
			return nil, err
		}
		if err := json.Unmarshal([]byte(metadataJSON), &fact.Metadata); err != nil {
			return nil, fmt.Errorf("decode fact metadata: %w", err)
		}
		facts = append(facts, fact)
	}
	return facts, rows.Err()
}


func (s *Store) FactsForProject(project string, limit int) ([]contract.TemporalFact, error) {
	if strings.TrimSpace(project) == "" {
		return nil, errors.New("project is required")
	}
	if limit <= 0 || limit > 1000 {
		limit = defaultLimit
	}
	rows, err := s.db.Query(`
		SELECT f.id, f.subject_id, f.predicate, COALESCE(f.object_id, ''), COALESCE(f.value, ''),
		       COALESCE(f.provenance_id, ''), f.confidence, COALESCE(f.valid_from, ''),
		       COALESCE(f.valid_to, ''), COALESCE(f.supersedes_id, ''),
		       COALESCE(f.superseded_at, ''), COALESCE(f.superseded_by, ''),
		       f.metadata_json, f.created_at, f.updated_at
		FROM temporal_facts f
		JOIN objects subject ON subject.id = f.subject_id
		WHERE subject.project = ?
		ORDER BY f.updated_at DESC
		LIMIT ?`, project, limit)
	if err != nil {
		return nil, fmt.Errorf("list project facts: %w", err)
	}
	defer rows.Close()

	facts := make([]contract.TemporalFact, 0)
	for rows.Next() {
		var fact contract.TemporalFact
		var metadataJSON string
		if err := rows.Scan(
			&fact.ID, &fact.SubjectID, &fact.Predicate, &fact.ObjectID, &fact.Value,
			&fact.ProvenanceID, &fact.Confidence, &fact.ValidFrom, &fact.ValidTo,
			&fact.SupersedesID, &fact.SupersededAt, &fact.SupersededBy,
			&metadataJSON, &fact.CreatedAt, &fact.UpdatedAt,
		); err != nil {
			return nil, err
		}
		if err := json.Unmarshal([]byte(metadataJSON), &fact.Metadata); err != nil {
			return nil, fmt.Errorf("decode fact metadata: %w", err)
		}
		facts = append(facts, fact)
	}
	return facts, rows.Err()
}

func normalizeIDs(values []string) []string {
	result := make([]string, 0, len(values))
	seen := make(map[string]bool)
	for _, value := range values {
		value = strings.TrimSpace(value)
		if value == "" || seen[value] {
			continue
		}
		seen[value] = true
		result = append(result, value)
	}
	return result
}

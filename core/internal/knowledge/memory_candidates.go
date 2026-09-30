package knowledge

import (
	"database/sql"
	"encoding/json"
	"errors"
	"fmt"
	"strings"
	"sync"
	"time"

	"github.com/agentvault/core/internal/contract"
)

const (
	eventMemoryCandidateProposed   = "memory_candidate.proposed"
	eventMemoryCandidateAccepted   = "memory_candidate.accepted"
	eventMemoryCandidateRejected   = "memory_candidate.rejected"
	eventMemoryCandidateMerged     = "memory_candidate.merged"
	eventMemoryCandidateSuperseded = "memory_candidate.superseded"
)

var memoryCandidateLocks sync.Map // candidate id -> *sync.Mutex

type memoryCandidateResolution struct {
	CandidateID    string                         `json:"candidateId"`
	Status         contract.MemoryCandidateStatus `json:"status"`
	ReviewedBy     string                         `json:"reviewedBy"`
	Reason         string                         `json:"reason,omitempty"`
	TargetMemoryID string                         `json:"targetMemoryId,omitempty"`
	ResultMemory   *contract.MemoryRecord         `json:"resultMemory,omitempty"`
	ReviewedAt     string                         `json:"reviewedAt"`
}

// ProposeMemoryCandidate creates a reviewable semantic-memory proposal from one
// provenance-backed episode. Scope and provenance are inherited from the
// episode so extraction cannot silently widen visibility or replace evidence.
func (s *Store) ProposeMemoryCandidate(req contract.CreateMemoryCandidateRequest) (contract.MemoryCandidate, error) {
	req.EpisodeID = strings.TrimSpace(req.EpisodeID)
	req.MemoryKind = strings.TrimSpace(req.MemoryKind)
	req.Content = strings.TrimSpace(req.Content)
	req.ObjectID = strings.TrimSpace(req.ObjectID)
	req.ProposedBy = strings.TrimSpace(req.ProposedBy)

	if req.EpisodeID == "" {
		return contract.MemoryCandidate{}, errors.New("episodeId is required")
	}
	if !validSemanticCandidateKind(req.MemoryKind) {
		return contract.MemoryCandidate{}, errors.New("memoryKind must be observation, fact, preference, decision, constraint, or summary")
	}
	if req.Content == "" {
		return contract.MemoryCandidate{}, errors.New("content is required")
	}

	episode, err := s.GetEpisode(req.EpisodeID)
	if err != nil {
		return contract.MemoryCandidate{}, fmt.Errorf("source episode: %w", err)
	}
	if strings.TrimSpace(episode.ProvenanceID) == "" {
		return contract.MemoryCandidate{}, errors.New("source episode must have provenance before proposing semantic memory")
	}
	provenance, err := s.GetProvenance(episode.ProvenanceID)
	if err != nil {
		return contract.MemoryCandidate{}, fmt.Errorf("source episode provenance: %w", err)
	}
	if err := s.validateCandidateObjectScope(episode, req.ObjectID); err != nil {
		return contract.MemoryCandidate{}, err
	}

	confidence := provenance.Confidence
	if req.Confidence != nil {
		confidence = *req.Confidence
	}
	if confidence < 0 || confidence > 1 {
		return contract.MemoryCandidate{}, errors.New("confidence must be between 0 and 1")
	}

	id := strings.TrimSpace(req.ID)
	if id == "" {
		id = deterministicPromotionID("candidate", episode.ID, req.MemoryKind, req.Content, req.ObjectID)
	}
	unlock := lockMemoryCandidate(id)
	defer unlock()

	if existing, err := s.GetMemoryCandidate(id); err == nil {
		if existing.SourceEpisodeID == episode.ID &&
			existing.MemoryKind == req.MemoryKind &&
			existing.Content == req.Content &&
			existing.ObjectID == req.ObjectID {
			return existing, nil
		}
		return contract.MemoryCandidate{}, fmt.Errorf("memory candidate id %s already exists with different content", id)
	} else if !errors.Is(err, sql.ErrNoRows) {
		return contract.MemoryCandidate{}, err
	}

	now := time.Now().UTC().Format(time.RFC3339Nano)
	candidate := contract.MemoryCandidate{
		ID:              id,
		SourceEpisodeID: episode.ID,
		MemoryKind:      req.MemoryKind,
		ScopeType:       episode.ScopeType,
		ScopeID:         episode.ScopeID,
		Content:         req.Content,
		ObjectID:        req.ObjectID,
		ProvenanceID:    episode.ProvenanceID,
		Confidence:      confidence,
		Status:          contract.MemoryCandidatePending,
		ProposedBy:      req.ProposedBy,
		Metadata:        orEmptyMap(req.Metadata),
		CreatedAt:       now,
		UpdatedAt:       now,
	}
	if err := validateMemoryCandidate(candidate); err != nil {
		return contract.MemoryCandidate{}, err
	}
	if err := s.persist(eventMemoryCandidateProposed, candidate, func() error {
		return s.projectMemoryCandidate(candidate)
	}); err != nil {
		return contract.MemoryCandidate{}, err
	}
	return candidate, nil
}

// GetMemoryCandidate returns one reviewable candidate.
func (s *Store) GetMemoryCandidate(id string) (contract.MemoryCandidate, error) {
	var candidate contract.MemoryCandidate
	var metadataJSON string
	err := s.db.QueryRow(`
		SELECT id, source_episode_id, memory_kind, scope_type, scope_id, content,
		       COALESCE(object_id, ''), provenance_id, confidence, status,
		       COALESCE(proposed_by, ''), COALESCE(reviewed_by, ''),
		       COALESCE(review_reason, ''), COALESCE(result_memory_id, ''),
		       COALESCE(target_memory_id, ''), metadata_json, created_at, updated_at,
		       COALESCE(reviewed_at, '')
		FROM memory_candidates WHERE id = ?`, id).Scan(
		&candidate.ID, &candidate.SourceEpisodeID, &candidate.MemoryKind,
		&candidate.ScopeType, &candidate.ScopeID, &candidate.Content,
		&candidate.ObjectID, &candidate.ProvenanceID, &candidate.Confidence,
		&candidate.Status, &candidate.ProposedBy, &candidate.ReviewedBy,
		&candidate.ReviewReason, &candidate.ResultMemoryID, &candidate.TargetMemoryID,
		&metadataJSON, &candidate.CreatedAt, &candidate.UpdatedAt, &candidate.ReviewedAt,
	)
	if err != nil {
		return contract.MemoryCandidate{}, err
	}
	if err := json.Unmarshal([]byte(metadataJSON), &candidate.Metadata); err != nil {
		return contract.MemoryCandidate{}, fmt.Errorf("decode memory candidate metadata: %w", err)
	}
	return candidate, nil
}

// ListMemoryCandidates returns the review queue newest-first.
func (s *Store) ListMemoryCandidates(filter contract.MemoryCandidateFilter) ([]contract.MemoryCandidate, error) {
	if filter.Status != "" && !validMemoryCandidateStatus(filter.Status) {
		return nil, errors.New("invalid memory candidate status")
	}
	if filter.MemoryKind != "" && !validSemanticCandidateKind(filter.MemoryKind) {
		return nil, errors.New("invalid memory candidate kind")
	}
	if (filter.ScopeType == "") != (filter.ScopeID == "") {
		return nil, errors.New("scopeType and scopeId must be supplied together")
	}
	limit := filter.Limit
	if limit <= 0 || limit > 500 {
		limit = 100
	}

	query := "SELECT id FROM memory_candidates WHERE 1=1"
	args := make([]interface{}, 0, 5)
	if filter.Status != "" {
		query += " AND status = ?"
		args = append(args, filter.Status)
	}
	if filter.ScopeType != "" {
		query += " AND scope_type = ? AND scope_id = ?"
		args = append(args, strings.TrimSpace(filter.ScopeType), strings.TrimSpace(filter.ScopeID))
	}
	if filter.MemoryKind != "" {
		query += " AND memory_kind = ?"
		args = append(args, filter.MemoryKind)
	}
	query += " ORDER BY updated_at DESC, id LIMIT ?"
	args = append(args, limit)

	rows, err := s.db.Query(query, args...)
	if err != nil {
		return nil, fmt.Errorf("list memory candidates: %w", err)
	}
	defer rows.Close()

	out := make([]contract.MemoryCandidate, 0)
	for rows.Next() {
		var id string
		if err := rows.Scan(&id); err != nil {
			return nil, err
		}
		candidate, err := s.GetMemoryCandidate(id)
		if err != nil {
			return nil, err
		}
		out = append(out, candidate)
	}
	return out, rows.Err()
}

func (s *Store) AcceptMemoryCandidate(id string, req contract.ReviewMemoryCandidateRequest) (contract.MemoryCandidate, error) {
	return s.resolveMemoryCandidate(id, contract.MemoryCandidateAccepted, req.ReviewedBy, req.Reason, "", "")
}

func (s *Store) RejectMemoryCandidate(id string, req contract.ReviewMemoryCandidateRequest) (contract.MemoryCandidate, error) {
	return s.resolveMemoryCandidate(id, contract.MemoryCandidateRejected, req.ReviewedBy, req.Reason, "", "")
}

func (s *Store) SupersedeMemoryWithCandidate(id string, req contract.SupersedeMemoryCandidateRequest) (contract.MemoryCandidate, error) {
	return s.resolveMemoryCandidate(
		id,
		contract.MemoryCandidateSuperseded,
		req.ReviewedBy,
		req.Reason,
		req.TargetMemoryID,
		"",
	)
}

func (s *Store) MergeMemoryCandidate(id string, req contract.MergeMemoryCandidateRequest) (contract.MemoryCandidate, error) {
	return s.resolveMemoryCandidate(
		id,
		contract.MemoryCandidateMerged,
		req.ReviewedBy,
		req.Reason,
		req.TargetMemoryID,
		req.MergedContent,
	)
}

func (s *Store) resolveMemoryCandidate(
	id string,
	status contract.MemoryCandidateStatus,
	reviewedBy, reason, targetMemoryID, mergedContent string,
) (contract.MemoryCandidate, error) {
	id = strings.TrimSpace(id)
	reviewedBy = strings.TrimSpace(reviewedBy)
	reason = strings.TrimSpace(reason)
	targetMemoryID = strings.TrimSpace(targetMemoryID)
	mergedContent = strings.TrimSpace(mergedContent)
	if id == "" {
		return contract.MemoryCandidate{}, errors.New("candidate id is required")
	}
	if reviewedBy == "" {
		return contract.MemoryCandidate{}, errors.New("reviewedBy is required")
	}

	unlock := lockMemoryCandidate(id)
	defer unlock()

	candidate, err := s.GetMemoryCandidate(id)
	if err != nil {
		return contract.MemoryCandidate{}, err
	}
	if candidate.Status == status {
		if targetMemoryID == "" || candidate.TargetMemoryID == targetMemoryID {
			return candidate, nil
		}
	}
	if candidate.Status != contract.MemoryCandidatePending {
		return contract.MemoryCandidate{}, fmt.Errorf("memory candidate %s is already %s", id, candidate.Status)
	}

	var target contract.MemoryRecord
	switch status {
	case contract.MemoryCandidateAccepted:
		if targetMemoryID != "" {
			return contract.MemoryCandidate{}, errors.New("accepted candidate cannot specify targetMemoryId")
		}
	case contract.MemoryCandidateRejected:
		if targetMemoryID != "" || mergedContent != "" {
			return contract.MemoryCandidate{}, errors.New("rejected candidate cannot materialize memory")
		}
	case contract.MemoryCandidateSuperseded, contract.MemoryCandidateMerged:
		if targetMemoryID == "" {
			return contract.MemoryCandidate{}, errors.New("targetMemoryId is required")
		}
		target, err = s.GetMemory(targetMemoryID)
		if err != nil {
			return contract.MemoryCandidate{}, fmt.Errorf("target memory: %w", err)
		}
		if !strings.EqualFold(target.ScopeType, candidate.ScopeType) || target.ScopeID != candidate.ScopeID {
			return contract.MemoryCandidate{}, errors.New("target memory must be in the same scope as the candidate")
		}
		if target.MemoryKind != candidate.MemoryKind {
			return contract.MemoryCandidate{}, errors.New("target memory must have the same memoryKind as the candidate")
		}
		var replacementID string
		err := s.db.QueryRow("SELECT id FROM memory_records WHERE supersedes_id = ? LIMIT 1", targetMemoryID).Scan(&replacementID)
		if err == nil {
			return contract.MemoryCandidate{}, fmt.Errorf("target memory %s is already superseded by %s", targetMemoryID, replacementID)
		}
		if !errors.Is(err, sql.ErrNoRows) {
			return contract.MemoryCandidate{}, fmt.Errorf("check target memory supersession: %w", err)
		}
		if status == contract.MemoryCandidateMerged && mergedContent == "" {
			return contract.MemoryCandidate{}, errors.New("mergedContent is required")
		}
	default:
		return contract.MemoryCandidate{}, errors.New("unsupported memory candidate review status")
	}

	now := time.Now().UTC().Format(time.RFC3339Nano)
	resolution := memoryCandidateResolution{
		CandidateID:    candidate.ID,
		Status:         status,
		ReviewedBy:     reviewedBy,
		Reason:         reason,
		TargetMemoryID: targetMemoryID,
		ReviewedAt:     now,
	}

	if status != contract.MemoryCandidateRejected {
		content := candidate.Content
		if status == contract.MemoryCandidateMerged {
			content = mergedContent
		}
		resultID := deterministicPromotionID("mem", candidate.ID, string(status), targetMemoryID)
		metadata := make(map[string]interface{}, len(candidate.Metadata)+6)
		for key, value := range candidate.Metadata {
			metadata[key] = value
		}
		metadata["candidateId"] = candidate.ID
		metadata["sourceEpisodeId"] = candidate.SourceEpisodeID
		metadata["reviewAction"] = string(status)
		metadata["reviewedBy"] = reviewedBy
		if reason != "" {
			metadata["reviewReason"] = reason
		}
		if targetMemoryID != "" {
			metadata["targetMemoryId"] = targetMemoryID
		}
		result := contract.MemoryRecord{
			ID:           resultID,
			MemoryClass:  "semantic",
			MemoryKind:   candidate.MemoryKind,
			ScopeType:    candidate.ScopeType,
			ScopeID:      candidate.ScopeID,
			Content:      content,
			ObjectID:     candidate.ObjectID,
			ProvenanceID: candidate.ProvenanceID,
			Confidence:   candidate.Confidence,
			SupersedesID: targetMemoryID,
			Metadata:     metadata,
			CreatedAt:    now,
			UpdatedAt:    now,
		}
		resolution.ResultMemory = &result
	}

	eventType := map[contract.MemoryCandidateStatus]string{
		contract.MemoryCandidateAccepted:   eventMemoryCandidateAccepted,
		contract.MemoryCandidateRejected:   eventMemoryCandidateRejected,
		contract.MemoryCandidateMerged:     eventMemoryCandidateMerged,
		contract.MemoryCandidateSuperseded: eventMemoryCandidateSuperseded,
	}[status]
	if err := s.persist(eventType, resolution, func() error {
		return s.projectMemoryCandidateResolution(resolution)
	}); err != nil {
		return contract.MemoryCandidate{}, err
	}
	return s.GetMemoryCandidate(candidate.ID)
}

func (s *Store) validateCandidateObjectScope(episode contract.EpisodeRecord, objectID string) error {
	objectID = strings.TrimSpace(objectID)
	if objectID == "" {
		return nil
	}
	object, err := s.GetObject(objectID)
	if err != nil {
		return fmt.Errorf("object: %w", err)
	}

	switch strings.ToLower(strings.TrimSpace(episode.ScopeType)) {
	case "project":
		if object.Project == "" || object.Project != episode.ScopeID {
			return errors.New("candidate object must belong to the source episode project")
		}
	case "session":
		session, err := s.GetSession(episode.ScopeID)
		if err != nil {
			return fmt.Errorf("source episode session: %w", err)
		}
		if session.Project == "" || object.Project != session.Project {
			return errors.New("candidate object must belong to the source episode session project")
		}
	}
	return nil
}

func lockMemoryCandidate(id string) func() {
	value, _ := memoryCandidateLocks.LoadOrStore(id, &sync.Mutex{})
	mutex := value.(*sync.Mutex)
	mutex.Lock()
	return mutex.Unlock
}

func validSemanticCandidateKind(kind string) bool {
	switch strings.TrimSpace(kind) {
	case "observation", "fact", "preference", "decision", "constraint", "summary":
		return true
	default:
		return false
	}
}

func validMemoryCandidateStatus(status contract.MemoryCandidateStatus) bool {
	switch status {
	case contract.MemoryCandidatePending,
		contract.MemoryCandidateAccepted,
		contract.MemoryCandidateRejected,
		contract.MemoryCandidateMerged,
		contract.MemoryCandidateSuperseded:
		return true
	default:
		return false
	}
}

func validateMemoryCandidate(candidate contract.MemoryCandidate) error {
	if strings.TrimSpace(candidate.ID) == "" {
		return errors.New("memory candidate id is required")
	}
	if strings.TrimSpace(candidate.SourceEpisodeID) == "" {
		return errors.New("memory candidate sourceEpisodeId is required")
	}
	if !validSemanticCandidateKind(candidate.MemoryKind) {
		return errors.New("invalid memory candidate kind")
	}
	if strings.TrimSpace(candidate.ScopeType) == "" || strings.TrimSpace(candidate.ScopeID) == "" {
		return errors.New("memory candidate scopeType and scopeId are required")
	}
	if strings.TrimSpace(candidate.Content) == "" {
		return errors.New("memory candidate content is required")
	}
	if strings.TrimSpace(candidate.ProvenanceID) == "" {
		return errors.New("memory candidate provenanceId is required")
	}
	if candidate.Confidence < 0 || candidate.Confidence > 1 {
		return errors.New("memory candidate confidence must be between 0 and 1")
	}
	if candidate.Status != contract.MemoryCandidatePending {
		return errors.New("new memory candidate status must be pending")
	}
	if strings.TrimSpace(candidate.CreatedAt) == "" || strings.TrimSpace(candidate.UpdatedAt) == "" {
		return errors.New("memory candidate timestamps are required")
	}
	return nil
}

func validateMemoryCandidateResolution(resolution memoryCandidateResolution) error {
	if strings.TrimSpace(resolution.CandidateID) == "" {
		return errors.New("candidate resolution candidateId is required")
	}
	if resolution.Status == contract.MemoryCandidatePending || !validMemoryCandidateStatus(resolution.Status) {
		return errors.New("candidate resolution must be terminal")
	}
	if strings.TrimSpace(resolution.ReviewedBy) == "" || strings.TrimSpace(resolution.ReviewedAt) == "" {
		return errors.New("candidate resolution reviewer and timestamp are required")
	}
	if resolution.Status == contract.MemoryCandidateRejected {
		if resolution.ResultMemory != nil || resolution.TargetMemoryID != "" {
			return errors.New("rejected candidate resolution cannot materialize memory")
		}
		return nil
	}
	if resolution.ResultMemory == nil {
		return errors.New("candidate resolution resultMemory is required")
	}
	memory := resolution.ResultMemory
	if memory.MemoryClass != "semantic" || !validSemanticCandidateKind(memory.MemoryKind) {
		return errors.New("candidate resolution must materialize semantic memory")
	}
	if strings.TrimSpace(memory.ID) == "" || strings.TrimSpace(memory.ScopeType) == "" ||
		strings.TrimSpace(memory.ScopeID) == "" || strings.TrimSpace(memory.Content) == "" ||
		strings.TrimSpace(memory.ProvenanceID) == "" {
		return errors.New("candidate resolution materialized memory is incomplete")
	}
	if memory.Confidence < 0 || memory.Confidence > 1 {
		return errors.New("candidate resolution memory confidence must be between 0 and 1")
	}
	if (resolution.Status == contract.MemoryCandidateMerged || resolution.Status == contract.MemoryCandidateSuperseded) &&
		strings.TrimSpace(resolution.TargetMemoryID) == "" {
		return errors.New("candidate resolution targetMemoryId is required")
	}
	if memory.SupersedesID != resolution.TargetMemoryID {
		return errors.New("candidate resolution memory supersedesId must match targetMemoryId")
	}
	return nil
}

func (s *Store) projectMemoryCandidate(candidate contract.MemoryCandidate) error {
	metadata, err := json.Marshal(orEmptyMap(candidate.Metadata))
	if err != nil {
		return fmt.Errorf("marshal memory candidate metadata: %w", err)
	}
	_, err = s.db.Exec(`
		INSERT INTO memory_candidates (
			id, source_episode_id, memory_kind, scope_type, scope_id, content,
			object_id, provenance_id, confidence, status, proposed_by, reviewed_by,
			review_reason, result_memory_id, target_memory_id, metadata_json,
			created_at, updated_at, reviewed_at
		) VALUES (?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?)
		ON CONFLICT(id) DO UPDATE SET
			source_episode_id = excluded.source_episode_id,
			memory_kind = excluded.memory_kind,
			scope_type = excluded.scope_type,
			scope_id = excluded.scope_id,
			content = excluded.content,
			object_id = excluded.object_id,
			provenance_id = excluded.provenance_id,
			confidence = excluded.confidence,
			proposed_by = excluded.proposed_by,
			metadata_json = excluded.metadata_json,
			created_at = excluded.created_at`,
		candidate.ID, candidate.SourceEpisodeID, candidate.MemoryKind, candidate.ScopeType,
		candidate.ScopeID, candidate.Content, nullIfEmpty(candidate.ObjectID),
		candidate.ProvenanceID, candidate.Confidence, candidate.Status,
		nullIfEmpty(candidate.ProposedBy), nullIfEmpty(candidate.ReviewedBy),
		nullIfEmpty(candidate.ReviewReason), nullIfEmpty(candidate.ResultMemoryID),
		nullIfEmpty(candidate.TargetMemoryID), string(metadata), candidate.CreatedAt,
		candidate.UpdatedAt, nullIfEmpty(candidate.ReviewedAt),
	)
	if err != nil {
		return fmt.Errorf("project memory candidate: %w", err)
	}
	return nil
}

func (s *Store) projectMemoryCandidateResolution(resolution memoryCandidateResolution) error {
	tx, err := s.db.Begin()
	if err != nil {
		return err
	}
	defer tx.Rollback()

	resultMemoryID := ""
	if resolution.ResultMemory != nil {
		memory := *resolution.ResultMemory
		metadata, err := json.Marshal(orEmptyMap(memory.Metadata))
		if err != nil {
			return fmt.Errorf("marshal candidate memory metadata: %w", err)
		}
		if _, err := tx.Exec(`
			INSERT OR IGNORE INTO memory_records (
				id, memory_class, memory_kind, scope_type, scope_id, content, object_id,
				provenance_id, confidence, valid_from, valid_to, supersedes_id,
				metadata_json, created_at, updated_at
			) VALUES (?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?)`,
			memory.ID, memory.MemoryClass, nullIfEmpty(memory.MemoryKind), memory.ScopeType,
			memory.ScopeID, memory.Content, nullIfEmpty(memory.ObjectID),
			nullIfEmpty(memory.ProvenanceID), memory.Confidence,
			nullIfEmpty(memory.ValidFrom), nullIfEmpty(memory.ValidTo),
			nullIfEmpty(memory.SupersedesID), string(metadata), memory.CreatedAt,
			memory.UpdatedAt,
		); err != nil {
			return fmt.Errorf("project candidate memory: %w", err)
		}
		resultMemoryID = memory.ID
	}

	result, err := tx.Exec(`
		UPDATE memory_candidates
		SET status = ?, reviewed_by = ?, review_reason = ?, result_memory_id = ?,
		    target_memory_id = ?, updated_at = ?, reviewed_at = ?
		WHERE id = ? AND (status = ? OR status = ?)`,
		resolution.Status, resolution.ReviewedBy, nullIfEmpty(resolution.Reason),
		nullIfEmpty(resultMemoryID), nullIfEmpty(resolution.TargetMemoryID),
		resolution.ReviewedAt, resolution.ReviewedAt, resolution.CandidateID,
		contract.MemoryCandidatePending, resolution.Status,
	)
	if err != nil {
		return fmt.Errorf("project memory candidate resolution: %w", err)
	}
	rows, err := result.RowsAffected()
	if err != nil {
		return err
	}
	if rows == 0 {
		return fmt.Errorf("memory candidate %s does not exist", resolution.CandidateID)
	}
	if err := tx.Commit(); err != nil {
		return err
	}
	return nil
}

// projectMemoryCandidateJournalEvent replays candidate lifecycle events.
func (s *Store) projectMemoryCandidateJournalEvent(event JournalEvent) (bool, error) {
	switch event.Type {
	case eventMemoryCandidateProposed:
		var candidate contract.MemoryCandidate
		if err := json.Unmarshal(event.Payload, &candidate); err != nil {
			return true, fmt.Errorf("decode memory candidate: %w", err)
		}
		if err := validateMemoryCandidate(candidate); err != nil {
			return true, fmt.Errorf("validate memory candidate: %w", err)
		}
		return true, s.projectMemoryCandidate(candidate)
	case eventMemoryCandidateAccepted,
		eventMemoryCandidateRejected,
		eventMemoryCandidateMerged,
		eventMemoryCandidateSuperseded:
		var resolution memoryCandidateResolution
		if err := json.Unmarshal(event.Payload, &resolution); err != nil {
			return true, fmt.Errorf("decode memory candidate resolution: %w", err)
		}
		if err := validateMemoryCandidateResolution(resolution); err != nil {
			return true, fmt.Errorf("validate memory candidate resolution: %w", err)
		}
		expected := map[string]contract.MemoryCandidateStatus{
			eventMemoryCandidateAccepted:   contract.MemoryCandidateAccepted,
			eventMemoryCandidateRejected:   contract.MemoryCandidateRejected,
			eventMemoryCandidateMerged:     contract.MemoryCandidateMerged,
			eventMemoryCandidateSuperseded: contract.MemoryCandidateSuperseded,
		}[event.Type]
		if resolution.Status != expected {
			return true, fmt.Errorf("%s payload has status %q, want %q", event.Type, resolution.Status, expected)
		}
		return true, s.projectMemoryCandidateResolution(resolution)
	default:
		return false, nil
	}
}

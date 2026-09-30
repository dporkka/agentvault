package knowledge

import (
	"database/sql"
	"encoding/json"
	"errors"
	"fmt"
	"strings"
	"sync"

	"github.com/agentvault/core/internal/contract"
)

const deterministicExtractorID = "deterministic-extractor-v1"

var memoryExtractionLocks sync.Map // semantic fingerprint -> *sync.Mutex

type semanticCandidateSpec struct {
	Kind     string
	Content  string
	ObjectID string
}

// ExtractMemoryCandidatesFromEpisode derives reviewable semantic-memory
// candidates using only explicit, deterministic signals. It never calls a
// model and never accepts candidates into durable semantic memory.
func (s *Store) ExtractMemoryCandidatesFromEpisode(episodeID string) ([]contract.MemoryCandidate, error) {
	episodeID = strings.TrimSpace(episodeID)
	if episodeID == "" {
		return nil, errors.New("episode id is required")
	}
	episode, err := s.GetEpisode(episodeID)
	if err != nil {
		return nil, err
	}

	spec, ok, err := deterministicSemanticSpec(episode)
	if err != nil {
		return nil, err
	}
	if !ok {
		return []contract.MemoryCandidate{}, nil
	}
	if strings.TrimSpace(episode.ProvenanceID) == "" {
		return nil, errors.New("semantic candidate extraction requires episode provenance")
	}

	candidateID := deterministicPromotionID("candidate", episode.ID, spec.Kind, spec.Content, spec.ObjectID)
	if existing, err := s.GetMemoryCandidate(candidateID); err == nil {
		return []contract.MemoryCandidate{existing}, nil
	} else if !errors.Is(err, sql.ErrNoRows) {
		return nil, err
	}

	fingerprint := semanticFingerprint(episode.ScopeType, episode.ScopeID, spec.Kind, spec.Content, spec.ObjectID)
	unlock := lockMemoryExtraction(fingerprint)
	defer unlock()

	// Re-check after acquiring the semantic lock in case another equivalent
	// episode won while this goroutine was waiting.
	if existing, err := s.GetMemoryCandidate(candidateID); err == nil {
		return []contract.MemoryCandidate{existing}, nil
	} else if !errors.Is(err, sql.ErrNoRows) {
		return nil, err
	}

	duplicate, err := s.hasEquivalentPendingCandidate(episode.ScopeType, episode.ScopeID, spec.Kind, spec.Content, spec.ObjectID)
	if err != nil {
		return nil, err
	}
	if duplicate {
		return []contract.MemoryCandidate{}, nil
	}
	duplicate, err = s.hasEquivalentCurrentSemanticMemory(episode.ScopeType, episode.ScopeID, spec.Kind, spec.Content, spec.ObjectID)
	if err != nil {
		return nil, err
	}
	if duplicate {
		return []contract.MemoryCandidate{}, nil
	}

	metadata := map[string]interface{}{
		"extractor":          deterministicExtractorID,
		"sourceEventType":    episode.EventType,
		"semanticFingerprint": fingerprint,
	}
	candidate, err := s.ProposeMemoryCandidate(contract.CreateMemoryCandidateRequest{
		ID:         candidateID,
		EpisodeID:  episode.ID,
		MemoryKind: spec.Kind,
		Content:    spec.Content,
		ObjectID:   spec.ObjectID,
		ProposedBy: deterministicExtractorID,
		Metadata:   metadata,
	})
	if err != nil {
		return nil, err
	}
	return []contract.MemoryCandidate{candidate}, nil
}


// promoteSemanticSessionEvent turns an explicitly semantic durable session event
// into a provenance-backed episode and review candidate. It returns eligible=false
// for ordinary execution/history events so callers can treat those as raw evidence.
func (s *Store) promoteSemanticSessionEvent(
	session contract.AgentSession,
	event contract.SessionEvent,
) (PromotionResult, bool, error) {
	kind, explicitKind, err := semanticKindFromSessionEvent(event)
	if err != nil {
		return PromotionResult{}, true, err
	}
	if kind == "" {
		return PromotionResult{}, false, nil
	}

	summary := stringMapValue(event.Payload, "summary")
	if summary == "" {
		summary = stringMapValue(event.Payload, "memoryContent")
	}
	if summary == "" {
		// Semantic classification without durable content is not promotable.
		return PromotionResult{}, false, nil
	}

	metadata := map[string]interface{}{
		"sessionEventId": event.ID,
	}
	if explicitKind {
		metadata["memoryKind"] = kind
	}
	if value := stringMapValue(event.Payload, "memoryContent"); value != "" {
		metadata["memoryContent"] = value
	}

	objectIDs, err := stringSliceMapValue(event.Payload, "objectIds")
	if err != nil {
		return PromotionResult{}, true, err
	}
	if objectID := stringMapValue(event.Payload, "memoryObjectId"); objectID != "" {
		metadata["memoryObjectId"] = objectID
		if !containsID(objectIDs, objectID) {
			objectIDs = append(objectIDs, objectID)
		}
	}

	result, err := s.PromoteEvent(PromotionInput{
		SourceType:   "session-event",
		SourceID:     event.ID,
		Project:      session.Project,
		AgentID:      session.AgentID,
		SessionID:    session.ID,
		EventType:    event.EventType,
		Summary:      summary,
		OccurredAt:   event.CreatedAt,
		ProvenanceID: event.ProvenanceID,
		Metadata:     metadata,
		ObjectIDs:    objectIDs,
	})
	if err != nil {
		return PromotionResult{}, true, err
	}
	return result, true, nil
}

// ReconcileSemanticSessionEvents backfills semantic promotion for durable
// session events that were committed before enrichment completed. Promotion IDs
// and candidate IDs are deterministic, so retries are safe.
func (s *Store) ReconcileSemanticSessionEvents(limit int) error {
	if limit <= 0 || limit > 5000 {
		limit = 1000
	}

	rows, err := s.db.Query(`
		SELECT se.id, se.session_id, se.event_type, se.payload_json,
		       COALESCE(se.provenance_id, ''), se.created_at,
		       s.agent_id, COALESCE(s.project, '')
		FROM session_events se
		JOIN agent_sessions s ON s.id = se.session_id
		WHERE (
			lower(se.event_type) IN (
				'observation', 'observation.observed', 'observation.recorded',
				'fact', 'fact.observed', 'fact.recorded', 'fact.asserted',
				'preference', 'preference.observed', 'preference.recorded',
				'decision', 'decision.observed', 'decision.recorded',
				'constraint', 'constraint.observed', 'constraint.recorded',
				'summary', 'summary.generated', 'summary.recorded'
			)
			OR lower(COALESCE(json_extract(se.payload_json, '$.memoryKind'), '')) IN (
				'observation', 'fact', 'preference', 'decision', 'constraint', 'summary'
			)
		)
		AND NOT EXISTS (
			SELECT 1
			FROM episodes e
			WHERE json_extract(e.metadata_json, '$.promotionSourceType') = 'session-event'
			  AND json_extract(e.metadata_json, '$.promotionSourceId') = se.id
		)
		ORDER BY se.created_at ASC, se.id ASC
		LIMIT ?`, limit)
	if err != nil {
		return fmt.Errorf("list semantic session events for reconciliation: %w", err)
	}
	defer rows.Close()

	type pendingEvent struct {
		event   contract.SessionEvent
		session contract.AgentSession
	}
	pending := make([]pendingEvent, 0)
	for rows.Next() {
		var item pendingEvent
		var payloadJSON string
		if err := rows.Scan(
			&item.event.ID,
			&item.event.SessionID,
			&item.event.EventType,
			&payloadJSON,
			&item.event.ProvenanceID,
			&item.event.CreatedAt,
			&item.session.AgentID,
			&item.session.Project,
		); err != nil {
			return err
		}
		item.session.ID = item.event.SessionID
		if err := json.Unmarshal([]byte(payloadJSON), &item.event.Payload); err != nil {
			return fmt.Errorf("decode semantic session event payload: %w", err)
		}
		pending = append(pending, item)
	}
	if err := rows.Err(); err != nil {
		return err
	}

	var problems []error
	for _, item := range pending {
		if _, _, err := s.promoteSemanticSessionEvent(item.session, item.event); err != nil {
			problems = append(problems, fmt.Errorf("session event %s: %w", item.event.ID, err))
		}
	}
	return errors.Join(problems...)
}

func semanticKindFromSessionEvent(event contract.SessionEvent) (string, bool, error) {
	if rawKind, exists := event.Payload["memoryKind"]; exists {
		value, ok := rawKind.(string)
		if !ok {
			return "", true, errors.New("session event memoryKind must be a string")
		}
		kind := strings.ToLower(strings.TrimSpace(value))
		if !validSemanticCandidateKind(kind) {
			return "", true, fmt.Errorf("session event memoryKind %q is not a semantic candidate kind", value)
		}
		return kind, true, nil
	}
	return semanticKindForEventType(event.EventType), false, nil
}

func stringMapValue(values map[string]interface{}, key string) string {
	value, _ := values[key].(string)
	return strings.TrimSpace(value)
}

func stringSliceMapValue(values map[string]interface{}, key string) ([]string, error) {
	raw, exists := values[key]
	if !exists || raw == nil {
		return nil, nil
	}
	switch value := raw.(type) {
	case []string:
		return normalizeIDs(value), nil
	case []interface{}:
		result := make([]string, 0, len(value))
		for _, item := range value {
			text, ok := item.(string)
			if !ok {
				return nil, fmt.Errorf("%s must contain only string values", key)
			}
			result = append(result, text)
		}
		return normalizeIDs(result), nil
	default:
		return nil, fmt.Errorf("%s must be an array of strings", key)
	}
}

func deterministicSemanticSpec(episode contract.EpisodeRecord) (semanticCandidateSpec, bool, error) {
	kind := ""
	content := strings.TrimSpace(episode.Summary)
	objectID := ""

	if rawKind, exists := episode.Metadata["memoryKind"]; exists {
		value, ok := rawKind.(string)
		if !ok {
			return semanticCandidateSpec{}, false, errors.New("episode metadata memoryKind must be a string")
		}
		kind = strings.ToLower(strings.TrimSpace(value))
		if !validSemanticCandidateKind(kind) {
			return semanticCandidateSpec{}, false, fmt.Errorf("episode metadata memoryKind %q is not a semantic candidate kind", value)
		}
	} else {
		kind = semanticKindForEventType(episode.EventType)
	}
	if kind == "" {
		return semanticCandidateSpec{}, false, nil
	}

	if rawContent, exists := episode.Metadata["memoryContent"]; exists {
		value, ok := rawContent.(string)
		if !ok {
			return semanticCandidateSpec{}, false, errors.New("episode metadata memoryContent must be a string")
		}
		content = strings.TrimSpace(value)
	}
	if content == "" {
		return semanticCandidateSpec{}, false, errors.New("semantic candidate content is empty")
	}

	if rawObjectID, exists := episode.Metadata["memoryObjectId"]; exists {
		value, ok := rawObjectID.(string)
		if !ok {
			return semanticCandidateSpec{}, false, errors.New("episode metadata memoryObjectId must be a string")
		}
		objectID = strings.TrimSpace(value)
		if objectID != "" && !containsID(episode.ObjectIDs, objectID) {
			return semanticCandidateSpec{}, false, errors.New("episode metadata memoryObjectId must reference one of the episode objectIds")
		}
	} else if len(episode.ObjectIDs) == 1 {
		objectID = episode.ObjectIDs[0]
	}

	return semanticCandidateSpec{Kind: kind, Content: content, ObjectID: objectID}, true, nil
}

func semanticKindForEventType(eventType string) string {
	eventType = strings.ToLower(strings.TrimSpace(eventType))
	switch eventType {
	case "observation", "observation.observed", "observation.recorded":
		return "observation"
	case "fact", "fact.observed", "fact.recorded", "fact.asserted":
		return "fact"
	case "preference", "preference.observed", "preference.recorded":
		return "preference"
	case "decision", "decision.observed", "decision.recorded":
		return "decision"
	case "constraint", "constraint.observed", "constraint.recorded":
		return "constraint"
	case "summary", "summary.generated", "summary.recorded":
		return "summary"
	default:
		return ""
	}
}

func (s *Store) hasEquivalentPendingCandidate(scopeType, scopeID, kind, content, objectID string) (bool, error) {
	rows, err := s.db.Query(`
		SELECT content, COALESCE(object_id, '')
		FROM memory_candidates
		WHERE status = ? AND scope_type = ? AND scope_id = ? AND memory_kind = ?`,
		contract.MemoryCandidatePending, scopeType, scopeID, kind,
	)
	if err != nil {
		return false, fmt.Errorf("list equivalent pending candidates: %w", err)
	}
	defer rows.Close()

	for rows.Next() {
		var existingContent, existingObjectID string
		if err := rows.Scan(&existingContent, &existingObjectID); err != nil {
			return false, err
		}
		if equivalentSemanticContent(existingContent, content) && existingObjectID == objectID {
			return true, nil
		}
	}
	return false, rows.Err()
}

func (s *Store) hasEquivalentCurrentSemanticMemory(scopeType, scopeID, kind, content, objectID string) (bool, error) {
	rows, err := s.db.Query(`
		SELECT m.content, COALESCE(m.object_id, '')
		FROM memory_records m
		WHERE m.memory_class = 'semantic'
		  AND m.scope_type = ?
		  AND m.scope_id = ?
		  AND m.memory_kind = ?
		  AND NOT EXISTS (
		    SELECT 1 FROM memory_records newer WHERE newer.supersedes_id = m.id
		  )`,
		scopeType, scopeID, kind,
	)
	if err != nil {
		return false, fmt.Errorf("list equivalent semantic memories: %w", err)
	}
	defer rows.Close()

	for rows.Next() {
		var existingContent, existingObjectID string
		if err := rows.Scan(&existingContent, &existingObjectID); err != nil {
			return false, err
		}
		if equivalentSemanticContent(existingContent, content) && existingObjectID == objectID {
			return true, nil
		}
	}
	return false, rows.Err()
}

func semanticFingerprint(scopeType, scopeID, kind, content, objectID string) string {
	return deterministicPromotionID(
		"semantic",
		strings.ToLower(strings.TrimSpace(scopeType)),
		strings.TrimSpace(scopeID),
		strings.ToLower(strings.TrimSpace(kind)),
		normalizeSemanticContent(content),
		strings.TrimSpace(objectID),
	)
}

func equivalentSemanticContent(left, right string) bool {
	return normalizeSemanticContent(left) == normalizeSemanticContent(right)
}

func normalizeSemanticContent(value string) string {
	return strings.ToLower(strings.Join(strings.Fields(strings.TrimSpace(value)), " "))
}

func containsID(values []string, wanted string) bool {
	for _, value := range values {
		if strings.TrimSpace(value) == wanted {
			return true
		}
	}
	return false
}

func lockMemoryExtraction(key string) func() {
	value, _ := memoryExtractionLocks.LoadOrStore(key, &sync.Mutex{})
	mutex := value.(*sync.Mutex)
	mutex.Lock()
	return mutex.Unlock
}

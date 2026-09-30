package knowledge

import (
	"database/sql"
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

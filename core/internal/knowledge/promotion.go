package knowledge

import (
	"crypto/sha256"
	"database/sql"
	"errors"
	"fmt"
	"strings"
	"sync"
	"time"

	"github.com/agentvault/core/internal/contract"
)

// PromotionInput describes a low-ambiguity source event that should become a
// provenance-backed episode. Promotion is intentionally narrower than memory
// extraction: callers provide the event semantics instead of asking AgentVault
// to infer durable facts or preferences automatically.
type PromotionInput struct {
	SourceType   string
	SourceID     string
	EvidencePath string
	Project      string
	AgentID      string
	SessionID    string
	EventType    string
	Summary      string
	OccurredAt   string
	ProvenanceID string
	Confidence   *float64
	Metadata     map[string]interface{}
}

// PromotionResult contains the durable provenance and episode representing one
// promoted source event.
type PromotionResult struct {
	Provenance contract.ProvenanceRecord
	Episode    contract.EpisodeRecord
	Candidates []contract.MemoryCandidate
}

var promotionLocks sync.Map // deterministic source identity -> *sync.Mutex

// PromoteEvent idempotently turns one already-durable source event into a
// provenance-backed episode. IDs are derived from source identity, so retries,
// forced reindexing, and recovery cannot duplicate the promoted knowledge.
//
// Scope priority is session > project > agent. This intentionally rejects
// unscoped promotion instead of inventing a global semantic scope.
func (s *Store) PromoteEvent(input PromotionInput) (PromotionResult, error) {
	input.SourceType = strings.TrimSpace(input.SourceType)
	input.SourceID = strings.TrimSpace(input.SourceID)
	input.EvidencePath = strings.TrimSpace(input.EvidencePath)
	input.Project = strings.TrimSpace(input.Project)
	input.AgentID = strings.TrimSpace(input.AgentID)
	input.SessionID = strings.TrimSpace(input.SessionID)
	input.EventType = strings.TrimSpace(input.EventType)
	input.Summary = strings.TrimSpace(input.Summary)
	input.OccurredAt = strings.TrimSpace(input.OccurredAt)
	input.ProvenanceID = strings.TrimSpace(input.ProvenanceID)

	if input.SourceType == "" {
		return PromotionResult{}, errors.New("promotion sourceType is required")
	}
	if input.SourceID == "" {
		return PromotionResult{}, errors.New("promotion sourceId is required")
	}
	if input.EventType == "" {
		return PromotionResult{}, errors.New("promotion eventType is required")
	}
	if input.Summary == "" {
		return PromotionResult{}, errors.New("promotion summary is required")
	}

	unlock := lockPromotion(input.SourceType + "\x00" + input.SourceID)
	defer unlock()

	scopeType, scopeID, err := s.resolvePromotionScope(input)
	if err != nil {
		return PromotionResult{}, err
	}

	occurredAt := input.OccurredAt
	if occurredAt == "" {
		occurredAt = time.Now().UTC().Format(time.RFC3339Nano)
	}

	confidence := 1.0
	if input.Confidence != nil {
		confidence = *input.Confidence
	}
	if confidence < 0 || confidence > 1 {
		return PromotionResult{}, errors.New("promotion confidence must be between 0 and 1")
	}

	provenance, err := s.resolvePromotionProvenance(input, occurredAt, confidence)
	if err != nil {
		return PromotionResult{}, err
	}

	episodeID := deterministicPromotionID("episode", input.SourceType, input.SourceID, input.EventType)
	episode, err := s.GetEpisode(episodeID)
	if err == nil {
		return s.finishPromotion(provenance, episode)
	}
	if !errors.Is(err, sql.ErrNoRows) {
		return PromotionResult{}, fmt.Errorf("load promoted episode: %w", err)
	}

	episode, err = s.RecordEpisode(contract.CreateEpisodeRequest{
		ID:           episodeID,
		ScopeType:    scopeType,
		ScopeID:      scopeID,
		EventType:    input.EventType,
		Summary:      input.Summary,
		ProvenanceID: provenance.ID,
		OccurredAt:   occurredAt,
		Metadata:     promotionMetadata(input.Metadata, input),
	})
	if err != nil {
		// Another concurrent producer may have won after our read. Re-load the
		// deterministic ID before surfacing a failure.
		if existing, loadErr := s.GetEpisode(episodeID); loadErr == nil {
			return PromotionResult{Provenance: provenance, Episode: existing}, nil
		}
		return PromotionResult{}, fmt.Errorf("record promoted episode: %w", err)
	}

	return s.finishPromotion(provenance, episode)
}

func (s *Store) finishPromotion(provenance contract.ProvenanceRecord, episode contract.EpisodeRecord) (PromotionResult, error) {
	candidates, err := s.ExtractMemoryCandidatesFromEpisode(episode.ID)
	if err != nil {
		return PromotionResult{}, fmt.Errorf("extract promoted memory candidates: %w", err)
	}
	return PromotionResult{
		Provenance: provenance,
		Episode:    episode,
		Candidates: candidates,
	}, nil
}

func (s *Store) resolvePromotionScope(input PromotionInput) (string, string, error) {
	if input.SessionID != "" {
		session, err := s.GetSession(input.SessionID)
		if err != nil {
			return "", "", fmt.Errorf("promotion session: %w", err)
		}
		if input.AgentID != "" && session.AgentID != input.AgentID {
			return "", "", fmt.Errorf("promotion session %s belongs to agent %s, not %s", session.ID, session.AgentID, input.AgentID)
		}
		if input.Project != "" && session.Project != "" && session.Project != input.Project {
			return "", "", fmt.Errorf("promotion session %s belongs to project %s, not %s", session.ID, session.Project, input.Project)
		}
		return "session", input.SessionID, nil
	}
	if input.Project != "" {
		return "project", input.Project, nil
	}
	if input.AgentID != "" {
		return "agent", input.AgentID, nil
	}
	return "", "", errors.New("promotion requires project, agentId, or sessionId scope")
}

func (s *Store) resolvePromotionProvenance(input PromotionInput, observedAt string, confidence float64) (contract.ProvenanceRecord, error) {
	if input.ProvenanceID != "" {
		record, err := s.GetProvenance(input.ProvenanceID)
		if err != nil {
			return contract.ProvenanceRecord{}, fmt.Errorf("promotion provenance: %w", err)
		}
		return record, nil
	}

	provenanceID := deterministicPromotionID("prov", input.SourceType, input.SourceID)
	if existing, err := s.GetProvenance(provenanceID); err == nil {
		return existing, nil
	} else if !errors.Is(err, sql.ErrNoRows) {
		return contract.ProvenanceRecord{}, fmt.Errorf("load promoted provenance: %w", err)
	}

	evidence := []contract.ProvenanceEvidence{}
	if input.EvidencePath != "" {
		evidence = append(evidence, contract.ProvenanceEvidence{
			Source: input.SourceType,
			ID:     input.SourceID,
			Path:   input.EvidencePath,
		})
	}
	record, err := s.CreateProvenance(contract.ProvenanceRecord{
		ID:         provenanceID,
		SourceType: input.SourceType,
		SourceID:   input.SourceID,
		AgentID:    input.AgentID,
		SessionID:  input.SessionID,
		Confidence: confidence,
		ObservedAt: observedAt,
		Evidence:   evidence,
		Metadata:   promotionMetadata(input.Metadata, input),
	})
	if err != nil {
		if existing, loadErr := s.GetProvenance(provenanceID); loadErr == nil {
			return existing, nil
		}
		return contract.ProvenanceRecord{}, fmt.Errorf("create promoted provenance: %w", err)
	}
	return record, nil
}

func promotionMetadata(metadata map[string]interface{}, input PromotionInput) map[string]interface{} {
	result := make(map[string]interface{}, len(metadata)+4)
	for key, value := range metadata {
		result[key] = value
	}
	result["promotionSourceType"] = input.SourceType
	result["promotionSourceId"] = input.SourceID
	if input.Project != "" {
		result["project"] = input.Project
	}
	if input.EvidencePath != "" {
		result["path"] = input.EvidencePath
	}
	return result
}

func deterministicPromotionID(prefix string, parts ...string) string {
	hash := sha256.Sum256([]byte(strings.Join(parts, "\x00")))
	return fmt.Sprintf("%s_%x", prefix, hash[:12])
}

func lockPromotion(key string) func() {
	value, _ := promotionLocks.LoadOrStore(key, &sync.Mutex{})
	mutex := value.(*sync.Mutex)
	mutex.Lock()
	return mutex.Unlock
}

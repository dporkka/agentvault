package knowledge

import (
	"fmt"
	"strings"
	"time"

	"github.com/agentvault/core/internal/contract"
)

func validateJournalPayload(eventType string, payload interface{}) error {
	switch eventType {
	case eventEpisodeRecorded:
		episode, ok := payload.(contract.EpisodeRecord)
		if !ok {
			return fmt.Errorf("%s payload must be EpisodeRecord", eventType)
		}
		if strings.TrimSpace(episode.ScopeType) == "" || strings.TrimSpace(episode.ScopeID) == "" {
			return fmt.Errorf("episode scopeType and scopeId are required")
		}
		if strings.TrimSpace(episode.EventType) == "" || strings.TrimSpace(episode.Summary) == "" {
			return fmt.Errorf("episode eventType and summary are required")
		}
		if strings.TrimSpace(episode.OccurredAt) == "" {
			return fmt.Errorf("episode occurredAt is required")
		}
		return validateTemporalInterval(episode.OccurredAt, episode.EndedAt)
	case eventFactRecorded:
		fact, ok := payload.(contract.TemporalFact)
		if !ok {
			return fmt.Errorf("%s payload must be TemporalFact", eventType)
		}
		if strings.TrimSpace(fact.SubjectID) == "" || strings.TrimSpace(fact.Predicate) == "" {
			return fmt.Errorf("fact subjectId and predicate are required")
		}
		if strings.TrimSpace(fact.ObjectID) == "" && strings.TrimSpace(fact.Value) == "" {
			return fmt.Errorf("fact objectId or value is required")
		}
		if fact.Confidence < 0 || fact.Confidence > 1 {
			return fmt.Errorf("fact confidence must be between 0 and 1")
		}
		return validateTemporalInterval(fact.ValidFrom, fact.ValidTo)
	case eventRelationCreated:
		relation, ok := payload.(contract.ObjectRelation)
		if !ok {
			return fmt.Errorf("%s payload must be ObjectRelation", eventType)
		}
		return validateTemporalInterval(relation.ValidFrom, relation.ValidTo)
	case eventMemoryRecorded:
		memory, ok := payload.(contract.MemoryRecord)
		if !ok {
			return fmt.Errorf("%s payload must be MemoryRecord", eventType)
		}
		return validateTemporalInterval(memory.ValidFrom, memory.ValidTo)
	case eventProvenanceCreated:
		record, ok := payload.(contract.ProvenanceRecord)
		if !ok {
			return fmt.Errorf("%s payload must be ProvenanceRecord", eventType)
		}
		if record.ObservedAt == "" {
			return nil
		}
		if _, err := time.Parse(time.RFC3339, record.ObservedAt); err != nil {
			return fmt.Errorf("observedAt must be RFC3339: %w", err)
		}
	}
	return nil
}

func validateTemporalInterval(validFrom, validTo string) error {
	var from, to time.Time
	var err error
	if validFrom != "" {
		from, err = parseKnowledgeTime(validFrom)
		if err != nil {
			return fmt.Errorf("validFrom must be YYYY-MM-DD or RFC3339: %w", err)
		}
	}
	if validTo != "" {
		to, err = parseKnowledgeTime(validTo)
		if err != nil {
			return fmt.Errorf("validTo must be YYYY-MM-DD or RFC3339: %w", err)
		}
	}
	if !from.IsZero() && !to.IsZero() && !from.Before(to) {
		return fmt.Errorf("validFrom must be earlier than validTo")
	}
	return nil
}

func parseKnowledgeTime(value string) (time.Time, error) {
	if parsed, err := time.Parse(time.RFC3339, value); err == nil {
		return parsed.UTC(), nil
	}
	parsed, err := time.Parse("2006-01-02", value)
	if err != nil {
		return time.Time{}, err
	}
	return parsed.UTC(), nil
}

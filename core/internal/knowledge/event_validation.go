package knowledge

import (
	"fmt"
	"time"

	"github.com/agentvault/core/internal/contract"
)

func validateJournalPayload(eventType string, payload interface{}) error {
	switch eventType {
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

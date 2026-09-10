package knowledge

import (
	"encoding/json"
	"fmt"

	"github.com/agentvault/core/internal/contract"
)

func validateReplayedJournalEvent(event JournalEvent) error {
	switch event.Type {
	case eventRelationCreated:
		var relation contract.ObjectRelation
		if err := json.Unmarshal(event.Payload, &relation); err != nil {
			return fmt.Errorf("decode relation payload: %w", err)
		}
		return validateJournalPayload(event.Type, relation)
	case eventMemoryRecorded:
		var memory contract.MemoryRecord
		if err := json.Unmarshal(event.Payload, &memory); err != nil {
			return fmt.Errorf("decode memory payload: %w", err)
		}
		return validateJournalPayload(event.Type, memory)
	case eventProvenanceCreated:
		var provenance contract.ProvenanceRecord
		if err := json.Unmarshal(event.Payload, &provenance); err != nil {
			return fmt.Errorf("decode provenance payload: %w", err)
		}
		return validateJournalPayload(event.Type, provenance)
	default:
		return nil
	}
}

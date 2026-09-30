package knowledge

import (
	"encoding/json"
	"os"
	"path/filepath"
	"testing"

	"github.com/agentvault/core/internal/contract"
)

func TestReplayRejectsCorruptedTemporalEvent(t *testing.T) {
	vault := t.TempDir()
	journal := NewJournal(vault)
	if err := os.MkdirAll(filepath.Dir(journal.Path()), 0o755); err != nil {
		t.Fatal(err)
	}

	payload, err := json.Marshal(contract.ObjectRelation{
		ID:           "rel_corrupt",
		FromObjectID: "obj_a",
		ToObjectID:   "obj_b",
		RelationType: "affects",
		ValidFrom:    "2026-10-01",
		ValidTo:      "2026-09-01",
		Confidence:   1,
		CreatedAt:    "2026-09-10T12:00:00Z",
		UpdatedAt:    "2026-09-10T12:00:00Z",
	})
	if err != nil {
		t.Fatal(err)
	}
	event := JournalEvent{
		Version:   journalVersion,
		ID:        "evt_corrupt",
		Type:      eventRelationCreated,
		Timestamp: "2026-09-10T12:00:00Z",
		Payload:   payload,
	}
	line, err := json.Marshal(event)
	if err != nil {
		t.Fatal(err)
	}
	line = append(line, '\n')
	if err := os.WriteFile(journal.Path(), line, 0o600); err != nil {
		t.Fatal(err)
	}

	called := false
	err = journal.Replay(func(JournalEvent) error {
		called = true
		return nil
	})
	if err == nil {
		t.Fatal("expected corrupted temporal event to fail replay")
	}
	if called {
		t.Fatal("replay handler must not receive invalid canonical events")
	}
}

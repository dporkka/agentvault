package knowledge

import (
	"os"
	"testing"

	"github.com/agentvault/core/internal/contract"
)

func TestInvalidTemporalRelationNeverEntersJournal(t *testing.T) {
	store, database, _ := setupStore(t)
	defer database.Close()

	from, err := store.UpsertObject(contract.UpsertKnowledgeObjectRequest{
		ID:    "obj_temporal_from",
		Type:  "decision",
		Title: "From",
	})
	if err != nil {
		t.Fatal(err)
	}
	to, err := store.UpsertObject(contract.UpsertKnowledgeObjectRequest{
		ID:    "obj_temporal_to",
		Type:  "project",
		Title: "To",
	})
	if err != nil {
		t.Fatal(err)
	}

	before, err := os.ReadFile(store.JournalPath())
	if err != nil {
		t.Fatal(err)
	}
	_, err = store.CreateRelation(contract.CreateObjectRelationRequest{
		ID:           "rel_invalid_temporal",
		FromObjectID: from.ID,
		ToObjectID:   to.ID,
		RelationType: "affects",
		ValidFrom:    "2026-10-01",
		ValidTo:      "2026-09-01",
	})
	if err == nil {
		t.Fatal("expected inverted temporal relation to fail")
	}
	after, err := os.ReadFile(store.JournalPath())
	if err != nil {
		t.Fatal(err)
	}
	if string(after) != string(before) {
		t.Fatal("invalid relation mutated the canonical journal")
	}
	if _, err := store.RelationsForObject(from.ID); err != nil {
		t.Fatal(err)
	}
	relations, _ := store.RelationsForObject(from.ID)
	if len(relations) != 0 {
		t.Fatalf("invalid relation reached SQLite projection: %+v", relations)
	}
}

func TestInvalidMemoryValidityNeverEntersJournal(t *testing.T) {
	store, database, _ := setupStore(t)
	defer database.Close()

	beforePath := store.JournalPath()
	before, err := os.ReadFile(beforePath)
	if err != nil && !os.IsNotExist(err) {
		t.Fatal(err)
	}
	_, err = store.RecordMemory(contract.CreateMemoryRequest{
		ID:         "mem_invalid_temporal",
		MemoryType: "semantic",
		ScopeType:  "project",
		ScopeID:    "agentvault",
		Content:    "This should never persist.",
		ValidFrom:  "not-a-date",
	})
	if err == nil {
		t.Fatal("expected malformed memory validity to fail")
	}
	after, err := os.ReadFile(beforePath)
	if err != nil && !os.IsNotExist(err) {
		t.Fatal(err)
	}
	if string(after) != string(before) {
		t.Fatal("invalid memory mutated the canonical journal")
	}
	memories, err := store.ListMemories("project", "agentvault", "semantic", 10)
	if err != nil {
		t.Fatal(err)
	}
	if len(memories) != 0 {
		t.Fatalf("invalid memory reached SQLite projection: %+v", memories)
	}
}

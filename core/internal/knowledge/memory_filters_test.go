package knowledge

import (
	"testing"

	"github.com/agentvault/core/internal/contract"
)

func TestListMemoriesFilteredSeparatesClassAndKind(t *testing.T) {
	store, database, _ := setupStore(t)
	defer database.Close()

	for _, req := range []contract.CreateMemoryRequest{
		{
			ID:          "semantic_decision",
			MemoryClass: "semantic",
			MemoryKind:  "decision",
			ScopeType:   "project",
			ScopeID:     "agentvault",
			Content:     "Use the unified memory model.",
		},
		{
			ID:          "semantic_fact",
			MemoryClass: "semantic",
			MemoryKind:  "fact",
			ScopeType:   "project",
			ScopeID:     "agentvault",
			Content:     "SQLite is a rebuildable projection.",
		},
		{
			ID:          "episodic_decision",
			MemoryClass: "episodic",
			MemoryKind:  "decision",
			ScopeType:   "project",
			ScopeID:     "agentvault",
			Content:     "The agent chose the unified model during this run.",
		},
	} {
		if _, err := store.RecordMemory(req); err != nil {
			t.Fatalf("RecordMemory(%s): %v", req.ID, err)
		}
	}

	semanticDecisions, err := store.ListMemoriesFiltered("project", "agentvault", "semantic", "decision", 20)
	if err != nil {
		t.Fatalf("ListMemoriesFiltered: %v", err)
	}
	if len(semanticDecisions) != 1 || semanticDecisions[0].ID != "semantic_decision" {
		t.Fatalf("semantic decisions = %+v", semanticDecisions)
	}

	allDecisions, err := store.ListMemoriesFiltered("project", "agentvault", "", "decision", 20)
	if err != nil {
		t.Fatalf("ListMemoriesFiltered(kind only): %v", err)
	}
	if len(allDecisions) != 2 {
		t.Fatalf("decision count = %d, want 2: %+v", len(allDecisions), allDecisions)
	}

	if _, err := store.ListMemoriesFiltered("project", "agentvault", "semantic", "mystery", 20); err == nil {
		t.Fatal("expected invalid memory kind filter to fail")
	}
}

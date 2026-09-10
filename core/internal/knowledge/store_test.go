package knowledge

import (
	"os"
	"path/filepath"
	"testing"

	"github.com/agentvault/core/internal/contract"
	"github.com/agentvault/core/internal/db"
)

func setupStore(t *testing.T) (*Store, *db.DB) {
	t.Helper()
	vault := t.TempDir()
	if err := os.MkdirAll(filepath.Join(vault, ".agentvault"), 0o755); err != nil {
		t.Fatal(err)
	}
	database, err := db.Open(vault)
	if err != nil {
		t.Fatal(err)
	}
	if err := database.RunMigrations(); err != nil {
		database.Close()
		t.Fatal(err)
	}
	return New(database), database
}

func TestObjectRelationMemoryLifecycle(t *testing.T) {
	store, database := setupStore(t)
	defer database.Close()

	confidence := 0.93
	provenance, err := store.CreateProvenance(contract.ProvenanceRecord{
		SourceType: "agent-session",
		SourceID:   "source-1",
		AgentID:    "architect",
		Confidence: confidence,
		Evidence: []contract.ProvenanceEvidence{{
			Source: "file",
			Path:   "docs/architecture.md",
		}},
	})
	if err != nil {
		t.Fatalf("CreateProvenance: %v", err)
	}
	if provenance.ID == "" || provenance.ObservedAt == "" {
		t.Fatalf("expected generated provenance identity/timestamps: %+v", provenance)
	}

	project, err := store.UpsertObject(contract.UpsertKnowledgeObjectRequest{
		ID:           "project_adacavo",
		Type:         "project",
		Title:        "Adacavo",
		Organization: "adacavo",
		Project:      "adacavo",
		Data:         map[string]interface{}{"repository": "dporkka/adacavo"},
		ProvenanceID: provenance.ID,
	})
	if err != nil {
		t.Fatalf("UpsertObject(project): %v", err)
	}

	decision, err := store.UpsertObject(contract.UpsertKnowledgeObjectRequest{
		ID:            "decision_authz",
		Type:          "decision",
		Title:         "Use OpenFGA for authorization",
		Status:        "accepted",
		Project:       "adacavo",
		CanonicalPath: "30-decisions/openfga.md",
		ProvenanceID:  provenance.ID,
	})
	if err != nil {
		t.Fatalf("UpsertObject(decision): %v", err)
	}

	relation, err := store.CreateRelation(contract.CreateObjectRelationRequest{
		FromObjectID: decision.ID,
		ToObjectID:   project.ID,
		RelationType: "affects",
		Confidence:   &confidence,
		ProvenanceID: provenance.ID,
	})
	if err != nil {
		t.Fatalf("CreateRelation: %v", err)
	}
	if relation.RelationType != "affects" {
		t.Fatalf("unexpected relation: %+v", relation)
	}

	relations, err := store.RelationsForObject(project.ID)
	if err != nil {
		t.Fatalf("RelationsForObject: %v", err)
	}
	if len(relations) != 1 || relations[0].FromObjectID != decision.ID {
		t.Fatalf("unexpected relations: %+v", relations)
	}

	memory, err := store.RecordMemory(contract.CreateMemoryRequest{
		MemoryType:   "semantic",
		ScopeType:    "project",
		ScopeID:      "adacavo",
		Content:      "Authorization decisions use OpenFGA.",
		ObjectID:     decision.ID,
		ProvenanceID: provenance.ID,
		Confidence:   &confidence,
	})
	if err != nil {
		t.Fatalf("RecordMemory: %v", err)
	}
	if memory.ID == "" || memory.MemoryType != "semantic" {
		t.Fatalf("unexpected memory: %+v", memory)
	}

	memories, err := store.ListMemories("project", "adacavo", "semantic", 10)
	if err != nil {
		t.Fatalf("ListMemories: %v", err)
	}
	if len(memories) != 1 || memories[0].ID != memory.ID {
		t.Fatalf("unexpected memories: %+v", memories)
	}

	objects, err := store.ListObjects(contract.KnowledgeObjectFilter{Project: "adacavo", Limit: 10})
	if err != nil {
		t.Fatalf("ListObjects: %v", err)
	}
	if len(objects) != 2 {
		t.Fatalf("expected 2 project objects, got %d", len(objects))
	}
}

func TestAgentSessionLifecycle(t *testing.T) {
	store, database := setupStore(t)
	defer database.Close()

	session, err := store.StartSession(contract.StartAgentSessionRequest{
		AgentID:   "backend-engineer",
		Project:   "agentvault",
		Objective: "Implement universal knowledge primitives",
		Branch:    "feat/universal-knowledge-core",
		Context:   map[string]interface{}{"tokenBudget": float64(16000)},
	})
	if err != nil {
		t.Fatalf("StartSession: %v", err)
	}
	if session.Status != "active" || session.ID == "" {
		t.Fatalf("unexpected session: %+v", session)
	}

	event, err := store.AppendSessionEvent(session.ID, contract.AppendSessionEventRequest{
		EventType: "decision",
		Payload: map[string]interface{}{
			"summary": "Keep Markdown canonical and SQLite derived",
		},
	})
	if err != nil {
		t.Fatalf("AppendSessionEvent: %v", err)
	}
	if event.SessionID != session.ID {
		t.Fatalf("unexpected event: %+v", event)
	}

	loaded, err := store.GetSession(session.ID)
	if err != nil {
		t.Fatalf("GetSession: %v", err)
	}
	if len(loaded.Events) != 1 || loaded.Events[0].EventType != "decision" {
		t.Fatalf("unexpected loaded session: %+v", loaded)
	}

	closed, err := store.CloseSession(session.ID, "completed")
	if err != nil {
		t.Fatalf("CloseSession: %v", err)
	}
	if closed.Status != "completed" || closed.EndedAt == "" {
		t.Fatalf("unexpected closed session: %+v", closed)
	}
}

func TestRecordMemoryRejectsUnknownType(t *testing.T) {
	store, database := setupStore(t)
	defer database.Close()

	_, err := store.RecordMemory(contract.CreateMemoryRequest{
		MemoryType: "mystery",
		ScopeType:  "project",
		ScopeID:    "agentvault",
		Content:    "invalid",
	})
	if err == nil {
		t.Fatal("expected invalid memory type to fail")
	}
}

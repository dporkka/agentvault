package mcp

import (
	"errors"
	"testing"

	"github.com/agentvault/core/internal/authz"
	"github.com/agentvault/core/internal/contract"
	"github.com/agentvault/core/internal/knowledge"
)

func TestSessionScopedKnowledgeWriteOwnsOnlySessionProvenanceObjects(t *testing.T) {
	_, database, vault := setupKnowledgeMCPServer(t)
	defer database.Close()
	store := knowledge.New(database, vault)
	if err := store.ReplayJournal(); err != nil {
		t.Fatal(err)
	}
	session, err := store.StartSession(contract.StartAgentSessionRequest{AgentID: "writer", Project: "alpha", Objective: "session"})
	if err != nil {
		t.Fatal(err)
	}
	unowned, err := store.UpsertObject(contract.UpsertKnowledgeObjectRequest{Type: "decision", Title: "Shared project object", Project: "alpha"})
	if err != nil {
		t.Fatal(err)
	}
	registry, err := authz.NewRegistry(vault)
	if err != nil {
		t.Fatal(err)
	}
	issued, err := registry.Mint(authz.MintRequest{
		AgentID: "writer", Capabilities: []authz.Capability{authz.KnowledgeWrite},
		Scope: authz.Scope{Projects: []string{"alpha"}, Sessions: []string{session.ID}},
	})
	if err != nil {
		t.Fatal(err)
	}
	server := NewServer(vault, database)
	if err := server.SetCapabilityToken(issued.Token); err != nil {
		t.Fatal(err)
	}
	if err := server.RegisterRuntimeSurface(false); err != nil {
		t.Fatal(err)
	}
	provText, err := server.tools["agentvault.create_provenance"].Handler(map[string]interface{}{
		"source_type": "agent-session", "session_id": session.ID,
	})
	if err != nil {
		t.Fatal(err)
	}
	var provenance contract.ProvenanceRecord
	if err := jsonUnmarshal([]byte(provText), &provenance); err != nil {
		t.Fatal(err)
	}

	if _, err := server.tools["agentvault.upsert_object"].Handler(map[string]interface{}{
		"id": unowned.ID,
		"type": "decision",
		"title": "Attempted takeover",
		"project": "alpha",
		"session_id": session.ID,
		"provenance_id": provenance.ID,
	}); !errors.Is(err, authz.ErrForbidden) {
		t.Fatalf("session-scoped writer should not update unowned project object, got %v", err)
	}

	ownedText, err := server.tools["agentvault.upsert_object"].Handler(map[string]interface{}{
		"type": "decision",
		"title": "Session-owned object",
		"project": "alpha",
		"session_id": session.ID,
		"provenance_id": provenance.ID,
	})
	if err != nil {
		t.Fatal(err)
	}
	var owned contract.KnowledgeObject
	if err := jsonUnmarshal([]byte(ownedText), &owned); err != nil {
		t.Fatal(err)
	}

	if _, err := server.tools["agentvault.upsert_object"].Handler(map[string]interface{}{
		"id": owned.ID,
		"type": "decision",
		"title": "Session-owned object updated",
		"project": "alpha",
		"session_id": session.ID,
		"provenance_id": provenance.ID,
	}); err != nil {
		t.Fatalf("session-scoped writer should update its provenance-owned object: %v", err)
	}

	if _, err := server.tools["agentvault.create_relation"].Handler(map[string]interface{}{
		"from_object_id": owned.ID,
		"to_object_id": unowned.ID,
		"relation_type": "depends_on",
		"session_id": session.ID,
		"provenance_id": provenance.ID,
	}); !errors.Is(err, authz.ErrForbidden) {
		t.Fatalf("session-scoped relation should reject unowned endpoint, got %v", err)
	}
}

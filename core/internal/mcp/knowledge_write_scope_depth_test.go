package mcp

import (
	"errors"
	"testing"

	"github.com/agentvault/core/internal/authz"
	"github.com/agentvault/core/internal/contract"
	"github.com/agentvault/core/internal/knowledge"
)

func TestScopedWriteDenialsReachResourceBoundaryAfterValidProvenance(t *testing.T) {
	_, database, vault := setupKnowledgeMCPServer(t)
	defer database.Close()
	store := knowledge.New(database, vault)
	if err := store.ReplayJournal(); err != nil {
		t.Fatal(err)
	}
	alphaSession, err := store.StartSession(contract.StartAgentSessionRequest{AgentID: "agent", Project: "alpha", Objective: "alpha"})
	if err != nil {
		t.Fatal(err)
	}
	betaSession, err := store.StartSession(contract.StartAgentSessionRequest{AgentID: "agent", Project: "beta", Objective: "beta"})
	if err != nil {
		t.Fatal(err)
	}
	alphaObject, err := store.UpsertObject(contract.UpsertKnowledgeObjectRequest{Type: "task", Title: "Alpha", Project: "alpha"})
	if err != nil {
		t.Fatal(err)
	}
	betaObject, err := store.UpsertObject(contract.UpsertKnowledgeObjectRequest{Type: "task", Title: "Beta", Project: "beta"})
	if err != nil {
		t.Fatal(err)
	}
	alphaProvenance, err := store.CreateProvenance(contract.ProvenanceRecord{
		SourceType: "agent-session", AgentID: "agent", SessionID: alphaSession.ID, Confidence: 1,
	})
	if err != nil {
		t.Fatal(err)
	}
	betaProvenance, err := store.CreateProvenance(contract.ProvenanceRecord{
		SourceType: "agent-session", AgentID: "agent", SessionID: betaSession.ID, Confidence: 1,
	})
	if err != nil {
		t.Fatal(err)
	}
	projectMemory, err := store.RecordMemory(contract.CreateMemoryRequest{
		MemoryClass: "semantic", ScopeType: "project", ScopeID: "alpha", Content: "project memory", ProvenanceID: alphaProvenance.ID,
	})
	if err != nil {
		t.Fatal(err)
	}

	registry, err := authz.NewRegistry(vault)
	if err != nil {
		t.Fatal(err)
	}
	issued, err := registry.Mint(authz.MintRequest{
		AgentID: "agent",
		Capabilities: []authz.Capability{authz.KnowledgeWrite, authz.MemoryWrite},
		Scope: authz.Scope{Projects: []string{"alpha"}, Sessions: []string{alphaSession.ID}},
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

	if _, err := server.tools["agentvault.upsert_object"].Handler(map[string]interface{}{
		"id": betaObject.ID,
		"type": "task",
		"title": "Attempted beta takeover",
		"project": "beta",
		"session_id": alphaSession.ID,
		"provenance_id": alphaProvenance.ID,
	}); !errors.Is(err, authz.ErrForbidden) {
		t.Fatalf("cross-project object update should reach scope denial, got %v", err)
	}

	if _, err := server.tools["agentvault.upsert_object"].Handler(map[string]interface{}{
		"type": "artifact",
		"title": "Path injection",
		"project": "alpha",
		"session_id": alphaSession.ID,
		"canonical_path": "30-projects/beta/secret.md",
		"provenance_id": alphaProvenance.ID,
	}); !errors.Is(err, authz.ErrForbidden) {
		t.Fatalf("canonical path introduction should reach path-policy denial, got %v", err)
	}

	if _, err := server.tools["agentvault.create_relation"].Handler(map[string]interface{}{
		"from_object_id": alphaObject.ID,
		"to_object_id": betaObject.ID,
		"relation_type": "leaks_to",
		"session_id": alphaSession.ID,
		"provenance_id": alphaProvenance.ID,
	}); !errors.Is(err, authz.ErrForbidden) {
		t.Fatalf("cross-project relation should reach endpoint-scope denial, got %v", err)
	}

	memoryCases := []struct {
		name string
		args map[string]interface{}
	}{
		{
			name: "project promotion",
			args: map[string]interface{}{
				"memory_class": "semantic", "scope_type": "project", "scope_id": "alpha", "content": "promote", "provenance_id": alphaProvenance.ID,
			},
		},
		{
			name: "other session",
			args: map[string]interface{}{
				"memory_class": "semantic", "scope_type": "session", "scope_id": betaSession.ID, "content": "cross", "provenance_id": alphaProvenance.ID,
			},
		},
		{
			name: "other project object",
			args: map[string]interface{}{
				"memory_class": "semantic", "scope_type": "session", "scope_id": alphaSession.ID, "content": "cross object", "object_id": betaObject.ID, "provenance_id": alphaProvenance.ID,
			},
		},
		{
			name: "other session provenance",
			args: map[string]interface{}{
				"memory_class": "semantic", "scope_type": "session", "scope_id": alphaSession.ID, "content": "cross provenance", "provenance_id": betaProvenance.ID,
			},
		},
		{
			name: "cross scope supersession",
			args: map[string]interface{}{
				"memory_class": "semantic", "scope_type": "session", "scope_id": alphaSession.ID, "content": "cross supersede", "provenance_id": alphaProvenance.ID, "supersedes_id": projectMemory.ID,
			},
		},
	}
	for _, tc := range memoryCases {
		t.Run(tc.name, func(t *testing.T) {
			if _, err := server.tools["agentvault.record_memory"].Handler(tc.args); !errors.Is(err, authz.ErrForbidden) {
				t.Fatalf("expected specific scope denial, got %v", err)
			}
		})
	}
}

package mcp

import (
	"encoding/json"
	"errors"
	"testing"

	"github.com/agentvault/core/internal/authz"
	"github.com/agentvault/core/internal/contract"
	"github.com/agentvault/core/internal/knowledge"
)

func TestKnowledgeReadProjectScopeFiltersObjectsRelationsMemoriesAndSessions(t *testing.T) {
	_, database, vault := setupKnowledgeMCPServer(t)
	defer database.Close()
	store := knowledge.New(database, vault)

	alphaOne, err := store.UpsertObject(contract.UpsertKnowledgeObjectRequest{ID: "obj_alpha_1", Type: "task", Title: "Alpha One", Project: "alpha"})
	if err != nil {
		t.Fatal(err)
	}
	alphaTwo, err := store.UpsertObject(contract.UpsertKnowledgeObjectRequest{ID: "obj_alpha_2", Type: "task", Title: "Alpha Two", Project: "alpha"})
	if err != nil {
		t.Fatal(err)
	}
	beta, err := store.UpsertObject(contract.UpsertKnowledgeObjectRequest{ID: "obj_beta", Type: "task", Title: "Beta", Project: "beta"})
	if err != nil {
		t.Fatal(err)
	}
	insideRelation, err := store.CreateRelation(contract.CreateObjectRelationRequest{
		ID: "rel_alpha", FromObjectID: alphaOne.ID, ToObjectID: alphaTwo.ID, RelationType: "depends_on",
	})
	if err != nil {
		t.Fatal(err)
	}
	outsideRelation, err := store.CreateRelation(contract.CreateObjectRelationRequest{
		ID: "rel_cross_project", FromObjectID: alphaOne.ID, ToObjectID: beta.ID, RelationType: "references",
	})
	if err != nil {
		t.Fatal(err)
	}
	if _, err := store.RecordMemory(contract.CreateMemoryRequest{
		ID: "mem_alpha", MemoryClass: "semantic", ScopeType: "project", ScopeID: "alpha", Content: "alpha memory",
	}); err != nil {
		t.Fatal(err)
	}
	if _, err := store.RecordMemory(contract.CreateMemoryRequest{
		ID: "mem_beta", MemoryClass: "semantic", ScopeType: "project", ScopeID: "beta", Content: "beta memory",
	}); err != nil {
		t.Fatal(err)
	}
	alphaSession, err := store.StartSession(contract.StartAgentSessionRequest{ID: "session_alpha", AgentID: "reader", Project: "alpha", Objective: "alpha"})
	if err != nil {
		t.Fatal(err)
	}
	betaSession, err := store.StartSession(contract.StartAgentSessionRequest{ID: "session_beta", AgentID: "reader", Project: "beta", Objective: "beta"})
	if err != nil {
		t.Fatal(err)
	}

	registry, err := authz.NewRegistry(vault)
	if err != nil {
		t.Fatal(err)
	}
	issued, err := registry.Mint(authz.MintRequest{
		AgentID: "reader", Capabilities: []authz.Capability{authz.KnowledgeRead}, Scope: authz.Scope{Projects: []string{"alpha"}},
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

	objectText, err := server.tools["agentvault.get_object"].Handler(map[string]interface{}{"id": alphaOne.ID})
	if err != nil {
		t.Fatalf("read alpha object: %v", err)
	}
	var objectResult struct {
		Object    contract.KnowledgeObject `json:"object"`
		Relations []contract.ObjectRelation `json:"relations"`
	}
	if err := json.Unmarshal([]byte(objectText), &objectResult); err != nil {
		t.Fatal(err)
	}
	if objectResult.Object.ID != alphaOne.ID {
		t.Fatalf("object=%s want %s", objectResult.Object.ID, alphaOne.ID)
	}
	if len(objectResult.Relations) != 1 || objectResult.Relations[0].ID != insideRelation.ID {
		t.Fatalf("relations=%+v; cross-project relation %s must be hidden", objectResult.Relations, outsideRelation.ID)
	}
	if _, err := server.tools["agentvault.get_object"].Handler(map[string]interface{}{"id": beta.ID}); !errors.Is(err, authz.ErrForbidden) {
		t.Fatalf("beta object should be forbidden, got %v", err)
	}

	memoryText, err := server.tools["agentvault.list_memories"].Handler(map[string]interface{}{"scope_type": "project", "scope_id": "alpha"})
	if err != nil {
		t.Fatal(err)
	}
	var memories []contract.MemoryRecord
	if err := json.Unmarshal([]byte(memoryText), &memories); err != nil {
		t.Fatal(err)
	}
	if len(memories) != 1 || memories[0].ID != "mem_alpha" {
		t.Fatalf("alpha memories=%+v", memories)
	}
	if _, err := server.tools["agentvault.list_memories"].Handler(map[string]interface{}{"scope_type": "project", "scope_id": "beta"}); !errors.Is(err, authz.ErrForbidden) {
		t.Fatalf("beta memories should be forbidden, got %v", err)
	}

	if _, err := server.tools["agentvault.get_session"].Handler(map[string]interface{}{"session_id": alphaSession.ID}); err != nil {
		t.Fatalf("alpha session should be readable: %v", err)
	}
	if _, err := server.tools["agentvault.get_session"].Handler(map[string]interface{}{"session_id": betaSession.ID}); !errors.Is(err, authz.ErrForbidden) {
		t.Fatalf("beta session should be forbidden, got %v", err)
	}
}

func TestKnowledgeReadSessionScopeDoesNotBecomeProjectWideRead(t *testing.T) {
	_, database, vault := setupKnowledgeMCPServer(t)
	defer database.Close()
	store := knowledge.New(database, vault)

	alphaObject, err := store.UpsertObject(contract.UpsertKnowledgeObjectRequest{ID: "obj_alpha", Type: "task", Title: "Alpha", Project: "alpha"})
	if err != nil {
		t.Fatal(err)
	}
	alphaSession, err := store.StartSession(contract.StartAgentSessionRequest{ID: "session_alpha", AgentID: "reader", Project: "alpha", Objective: "alpha"})
	if err != nil {
		t.Fatal(err)
	}
	betaSession, err := store.StartSession(contract.StartAgentSessionRequest{ID: "session_beta", AgentID: "reader", Project: "alpha", Objective: "other session"})
	if err != nil {
		t.Fatal(err)
	}
	if _, err := store.RecordMemory(contract.CreateMemoryRequest{
		ID: "mem_session_alpha", MemoryClass: "working", ScopeType: "session", ScopeID: alphaSession.ID, Content: "session-private",
	}); err != nil {
		t.Fatal(err)
	}
	if _, err := store.RecordMemory(contract.CreateMemoryRequest{
		ID: "mem_project_alpha", MemoryClass: "semantic", ScopeType: "project", ScopeID: "alpha", Content: "project-wide",
	}); err != nil {
		t.Fatal(err)
	}

	registry, err := authz.NewRegistry(vault)
	if err != nil {
		t.Fatal(err)
	}
	issued, err := registry.Mint(authz.MintRequest{
		AgentID: "reader", Capabilities: []authz.Capability{authz.KnowledgeRead}, Scope: authz.Scope{Sessions: []string{alphaSession.ID}},
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

	if _, err := server.tools["agentvault.get_session"].Handler(map[string]interface{}{"session_id": alphaSession.ID}); err != nil {
		t.Fatalf("bound session should be readable: %v", err)
	}
	if _, err := server.tools["agentvault.get_session"].Handler(map[string]interface{}{"session_id": betaSession.ID}); !errors.Is(err, authz.ErrForbidden) {
		t.Fatalf("other session should be forbidden, got %v", err)
	}
	if _, err := server.tools["agentvault.get_object"].Handler(map[string]interface{}{"id": alphaObject.ID}); !errors.Is(err, authz.ErrForbidden) {
		t.Fatalf("session-scoped identity must not gain project object access, got %v", err)
	}
	if _, err := server.tools["agentvault.list_memories"].Handler(map[string]interface{}{"scope_type": "project", "scope_id": "alpha"}); !errors.Is(err, authz.ErrForbidden) {
		t.Fatalf("session-scoped identity must not gain project-memory access, got %v", err)
	}
	text, err := server.tools["agentvault.list_memories"].Handler(map[string]interface{}{"scope_type": "session", "scope_id": alphaSession.ID})
	if err != nil {
		t.Fatalf("session memory should be readable: %v", err)
	}
	var memories []contract.MemoryRecord
	if err := json.Unmarshal([]byte(text), &memories); err != nil {
		t.Fatal(err)
	}
	if len(memories) != 1 || memories[0].ID != "mem_session_alpha" {
		t.Fatalf("session memories=%+v", memories)
	}
}

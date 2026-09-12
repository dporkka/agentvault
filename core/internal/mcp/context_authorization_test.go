package mcp

import (
	"errors"
	"testing"

	"github.com/agentvault/core/internal/authz"
	"github.com/agentvault/core/internal/contract"
	"github.com/agentvault/core/internal/knowledge"
)

func TestPrepareContextRequestBindsDurableSessionAndRejectsExplicitEscape(t *testing.T) {
	_, database, vault := setupKnowledgeMCPServer(t)
	defer database.Close()
	store := knowledge.New(database, vault)

	session, err := store.StartSession(contract.StartAgentSessionRequest{
		ID: "session_alpha", AgentID: "agent-alpha", Project: "alpha", Objective: "work",
	})
	if err != nil {
		t.Fatal(err)
	}
	if _, err := store.UpsertObject(contract.UpsertKnowledgeObjectRequest{ID: "obj_beta", Type: "decision", Title: "Beta secret", Project: "beta"}); err != nil {
		t.Fatal(err)
	}

	registry, err := authz.NewRegistry(vault)
	if err != nil {
		t.Fatal(err)
	}
	issued, err := registry.Mint(authz.MintRequest{
		AgentID: "agent-alpha", Capabilities: []authz.Capability{authz.ContextCompile}, Scope: authz.Scope{Sessions: []string{session.ID}},
	})
	if err != nil {
		t.Fatal(err)
	}
	server := NewServer(vault, database)
	if err := server.SetCapabilityToken(issued.Token); err != nil {
		t.Fatal(err)
	}

	prepared, principal, err := server.prepareContextRequest(store, contract.CompileContextRequest{Task: "continue", SessionID: session.ID})
	if err != nil {
		t.Fatal(err)
	}
	if principal == nil {
		t.Fatal("expected bound principal")
	}
	if prepared.Project != "alpha" || prepared.WorkspaceID != "alpha" || prepared.AgentID != "agent-alpha" {
		t.Fatalf("durable session was not bound into request: %+v", prepared)
	}

	if _, _, err := server.prepareContextRequest(store, contract.CompileContextRequest{Task: "missing session"}); !errors.Is(err, authz.ErrForbidden) {
		t.Fatalf("session-scoped context without session_id should fail, got %v", err)
	}
	if _, _, err := server.prepareContextRequest(store, contract.CompileContextRequest{
		Task: "escape", SessionID: session.ID, ObjectIDs: []string{"obj_beta"},
	}); !errors.Is(err, authz.ErrForbidden) {
		t.Fatalf("explicit cross-project object should fail, got %v", err)
	}
}

func TestFilterContextBundleDropsAlternatePathLeaksAndRedactsProvenance(t *testing.T) {
	_, database, vault := setupKnowledgeMCPServer(t)
	defer database.Close()
	store := knowledge.New(database, vault)

	alpha, err := store.UpsertObject(contract.UpsertKnowledgeObjectRequest{ID: "obj_alpha", Type: "decision", Title: "Alpha", Project: "alpha"})
	if err != nil {
		t.Fatal(err)
	}
	beta, err := store.UpsertObject(contract.UpsertKnowledgeObjectRequest{ID: "obj_beta", Type: "decision", Title: "Beta", Project: "beta"})
	if err != nil {
		t.Fatal(err)
	}
	alphaSession, err := store.StartSession(contract.StartAgentSessionRequest{ID: "session_alpha", AgentID: "agent", Project: "alpha", Objective: "alpha"})
	if err != nil {
		t.Fatal(err)
	}
	betaSession, err := store.StartSession(contract.StartAgentSessionRequest{ID: "session_beta", AgentID: "agent", Project: "beta", Objective: "beta"})
	if err != nil {
		t.Fatal(err)
	}

	principal := authz.Principal{Capabilities: []authz.Capability{authz.ContextCompile}, Scope: authz.Scope{Projects: []string{"alpha"}}}
	req := contract.CompileContextRequest{Task: "alpha", Project: "alpha", WorkspaceID: "alpha"}
	provenance := &contract.ContextProvenance{
		ID: "prov_1", SourceType: "file", SourceID: "secret-source", AgentID: "other-agent", Model: "model-x",
		Confidence: 0.9, ObservedAt: "2026-09-12T12:00:00Z",
		Evidence: []contract.ProvenanceEvidence{{Source: "file", Path: "30-projects/beta/secret.md", Quote: "secret"}},
	}
	bundle := contract.ContextBundle{
		Version: "1", Task: "alpha", Project: "alpha", WorkspaceID: "alpha", TokenBudget: 8000,
		Items: []contract.ContextItem{
			{Kind: "object", ID: alpha.ID, Content: "allowed object", ObjectIDs: []string{alpha.ID}, EstimatedTokens: 10, Provenance: provenance},
			{Kind: "object", ID: beta.ID, Content: "forbidden object", ObjectIDs: []string{beta.ID}, EstimatedTokens: 10},
			{Kind: "relation", ID: "rel_cross", Content: "cross relation", ObjectIDs: []string{alpha.ID, beta.ID}, EstimatedTokens: 10},
			{Kind: "memory", ID: "mem_alpha", Content: "allowed project memory", EstimatedTokens: 10, Metadata: map[string]interface{}{"scopeType": "project", "scopeId": "alpha"}},
			{Kind: "memory", ID: "mem_agent", Content: "agent-global leak", EstimatedTokens: 10, Metadata: map[string]interface{}{"scopeType": "agent", "scopeId": "agent"}},
			{Kind: "note", ID: "note_alpha", Content: "allowed note", EstimatedTokens: 10, Metadata: map[string]interface{}{"project": "alpha"}},
			{Kind: "note", ID: "note_beta", Content: "forbidden note", EstimatedTokens: 10, Metadata: map[string]interface{}{"project": "beta"}},
			{Kind: "session_history", ID: alphaSession.ID, Content: "allowed session", EstimatedTokens: 10},
			{Kind: "session_history", ID: betaSession.ID, Content: "forbidden session", EstimatedTokens: 10},
			{Kind: "future_unknown", ID: "unknown", Content: "must fail closed", EstimatedTokens: 10},
		},
	}

	filtered := filterContextBundle(store, &principal, req, bundle)
	visible := map[string]contract.ContextItem{}
	for _, item := range filtered.Items {
		visible[item.ID] = item
	}
	for _, expected := range []string{alpha.ID, "mem_alpha", "note_alpha", alphaSession.ID} {
		if _, ok := visible[expected]; !ok {
			t.Errorf("expected authorized context item %s", expected)
		}
	}
	for _, forbidden := range []string{beta.ID, "rel_cross", "mem_agent", "note_beta", betaSession.ID, "unknown"} {
		if _, ok := visible[forbidden]; ok {
			t.Errorf("out-of-scope context item leaked: %s", forbidden)
		}
	}
	alphaItem := visible[alpha.ID]
	if alphaItem.Provenance == nil {
		t.Fatal("expected minimal provenance to remain")
	}
	if alphaItem.Provenance.SourceID != "" || alphaItem.Provenance.AgentID != "" || alphaItem.Provenance.Model != "" || len(alphaItem.Provenance.Evidence) != 0 {
		t.Fatalf("unbound provenance routing/evidence data was not redacted: %+v", alphaItem.Provenance)
	}
	if filtered.Stats.Candidates != len(filtered.Items) || filtered.Stats.Included != len(filtered.Items) || filtered.Stats.Dropped != 0 {
		t.Fatalf("filtered stats can disclose hidden candidate counts: %+v", filtered.Stats)
	}
}

func TestSessionScopedContextKeepsProjectEvidenceButNotOtherSessions(t *testing.T) {
	_, database, vault := setupKnowledgeMCPServer(t)
	defer database.Close()
	store := knowledge.New(database, vault)
	alpha, err := store.UpsertObject(contract.UpsertKnowledgeObjectRequest{ID: "obj_alpha", Type: "decision", Title: "Alpha", Project: "alpha"})
	if err != nil {
		t.Fatal(err)
	}
	current, err := store.StartSession(contract.StartAgentSessionRequest{ID: "session_current", AgentID: "agent", Project: "alpha", Objective: "current"})
	if err != nil {
		t.Fatal(err)
	}
	other, err := store.StartSession(contract.StartAgentSessionRequest{ID: "session_other", AgentID: "agent", Project: "alpha", Objective: "other"})
	if err != nil {
		t.Fatal(err)
	}
	principal := authz.Principal{Capabilities: []authz.Capability{authz.ContextCompile}, Scope: authz.Scope{Sessions: []string{current.ID}}}
	req := contract.CompileContextRequest{Task: "work", Project: "alpha", WorkspaceID: "alpha", SessionID: current.ID}
	bundle := contract.ContextBundle{Items: []contract.ContextItem{
		{Kind: "object", ID: alpha.ID, Content: "project context", EstimatedTokens: 5},
		{Kind: "memory", ID: "project_memory", Content: "project memory", EstimatedTokens: 5, Metadata: map[string]interface{}{"scopeType": "project", "scopeId": "alpha"}},
		{Kind: "session_history", ID: current.ID, Content: "current", EstimatedTokens: 5},
		{Kind: "session_history", ID: other.ID, Content: "other", EstimatedTokens: 5},
	}}
	filtered := filterContextBundle(store, &principal, req, bundle)
	visible := map[string]bool{}
	for _, item := range filtered.Items {
		visible[item.ID] = true
	}
	if !visible[alpha.ID] || !visible["project_memory"] || !visible[current.ID] {
		t.Fatalf("session context lost authorized project evidence: %+v", visible)
	}
	if visible[other.ID] {
		t.Fatal("session-scoped context leaked another session in the same project")
	}
}

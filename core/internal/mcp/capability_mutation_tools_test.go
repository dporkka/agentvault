package mcp

import (
	"encoding/json"
	"os"
	"path/filepath"
	"testing"

	"github.com/agentvault/core/internal/authz"
	"github.com/agentvault/core/internal/contract"
	"github.com/agentvault/core/internal/knowledge"
)

func TestScopedMCPIdentityControlsMutationToolsAndResources(t *testing.T) {
	server, database, vault := setupKnowledgeMCPServer(t)
	defer database.Close()
	store := knowledge.New(database, vault)
	session, err := store.StartSession(contract.StartAgentSessionRequest{
		AgentID: "builder", Project: "alpha", Objective: "scoped mutation",
	})
	if err != nil {
		t.Fatal(err)
	}
	if err := os.MkdirAll(filepath.Join(vault, "30-projects", "alpha"), 0o755); err != nil {
		t.Fatal(err)
	}
	target := filepath.Join(vault, "30-projects", "alpha", "note.md")
	if err := os.WriteFile(target, []byte("before\n"), 0o644); err != nil {
		t.Fatal(err)
	}

	registry, err := authz.NewRegistry(vault)
	if err != nil {
		t.Fatal(err)
	}
	issued, err := registry.Mint(authz.MintRequest{
		ID:           "mcp-builder",
		AgentID:      "builder",
		Capabilities: []authz.Capability{authz.MutationRead, authz.MutationPropose, authz.MutationApprove, authz.MutationCommit},
		Scope:        authz.Scope{PathPrefixes: []string{"30-projects/alpha"}, Projects: []string{"alpha"}, Sessions: []string{session.ID}},
	})
	if err != nil {
		t.Fatal(err)
	}
	if err := server.SetCapabilityToken(issued.Token); err != nil {
		t.Fatal(err)
	}
	server.RegisterMutationTools()

	for _, expected := range []string{"agentvault.propose_mutation", "agentvault.get_mutation", "agentvault.list_mutations", "agentvault.approve_mutation", "agentvault.commit_mutation"} {
		if _, ok := server.tools[expected]; !ok {
			t.Fatalf("missing scoped mutation tool %s", expected)
		}
	}
	for _, absent := range []string{"agentvault.undo_mutation", "agentvault.reject_mutation"} {
		if _, ok := server.tools[absent]; ok {
			t.Fatalf("tool %s must not be registered without capability", absent)
		}
	}

	after := "after\n"
	text, err := server.tools["agentvault.propose_mutation"].Handler(map[string]interface{}{
		"kind": "replace", "path": "30-projects/alpha/note.md", "content": after, "reason": "scoped MCP",
		"session_id": session.ID,
	})
	if err != nil {
		t.Fatal(err)
	}
	var proposal contract.MutationProposal
	if err := json.Unmarshal([]byte(text), &proposal); err != nil {
		t.Fatal(err)
	}
	if proposal.AgentID != "builder" {
		t.Fatalf("agent id=%q want builder", proposal.AgentID)
	}
	if _, err := server.tools["agentvault.approve_mutation"].Handler(map[string]interface{}{"id": proposal.ID}); err != nil {
		t.Fatal(err)
	}
	if _, err := server.tools["agentvault.commit_mutation"].Handler(map[string]interface{}{"id": proposal.ID}); err != nil {
		t.Fatal(err)
	}
	if got, _ := os.ReadFile(target); string(got) != after {
		t.Fatalf("committed content=%q", got)
	}

	outside := filepath.Join(vault, "outside.md")
	if err := os.WriteFile(outside, []byte("before\n"), 0o644); err != nil {
		t.Fatal(err)
	}
	if _, err := server.tools["agentvault.propose_mutation"].Handler(map[string]interface{}{
		"kind": "replace", "path": "outside.md", "content": "blocked\n", "reason": "must fail", "session_id": session.ID,
	}); err == nil {
		t.Fatal("expected out-of-scope path to be rejected")
	}
}

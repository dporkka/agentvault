package mcp

import (
	"encoding/json"
	"os"
	"path/filepath"
	"testing"

	"github.com/agentvault/core/internal/contract"
)

func TestMutationMCPToolsAreProposalOnly(t *testing.T) {
	server, database, vault := setupKnowledgeMCPServer(t)
	defer database.Close()
	server.RegisterMutationTools()

	for _, expected := range []string{
		"agentvault.propose_mutation",
		"agentvault.get_mutation",
		"agentvault.list_mutations",
	} {
		if _, ok := server.tools[expected]; !ok {
			t.Fatalf("missing mutation MCP tool %s", expected)
		}
	}
	for _, forbidden := range []string{
		"agentvault.approve_mutation",
		"agentvault.commit_mutation",
		"agentvault.undo_mutation",
		"agentvault.reject_mutation",
	} {
		if _, ok := server.tools[forbidden]; ok {
			t.Fatalf("unsafe control-plane tool %s must not be exposed over unscoped MCP", forbidden)
		}
	}

	path := filepath.Join(vault, "proposal.md")
	before := "before\n"
	after := "after\n"
	if err := os.WriteFile(path, []byte(before), 0o644); err != nil {
		t.Fatal(err)
	}

	text, err := server.tools["agentvault.propose_mutation"].Handler(map[string]interface{}{
		"kind":     "replace",
		"path":     "proposal.md",
		"content":  after,
		"reason":   "MCP should propose but not apply",
		"agent_id": "mcp-agent",
	})
	if err != nil {
		t.Fatalf("propose_mutation: %v", err)
	}
	var proposal contract.MutationProposal
	if err := json.Unmarshal([]byte(text), &proposal); err != nil {
		t.Fatalf("decode proposal: %v", err)
	}
	if proposal.Status != contract.MutationProposed || proposal.AgentID != "mcp-agent" {
		t.Fatalf("unexpected proposal: %+v", proposal)
	}
	if got, _ := os.ReadFile(path); string(got) != before {
		t.Fatalf("MCP proposal mutated target: %q", got)
	}

	text, err = server.tools["agentvault.get_mutation"].Handler(map[string]interface{}{"id": proposal.ID})
	if err != nil {
		t.Fatalf("get_mutation: %v", err)
	}
	var loaded contract.MutationProposal
	if err := json.Unmarshal([]byte(text), &loaded); err != nil {
		t.Fatalf("decode loaded proposal: %v", err)
	}
	if loaded.ID != proposal.ID || loaded.Status != contract.MutationProposed {
		t.Fatalf("unexpected loaded proposal: %+v", loaded)
	}

	text, err = server.tools["agentvault.list_mutations"].Handler(map[string]interface{}{
		"status":   "proposed",
		"agent_id": "mcp-agent",
		"limit":    float64(10),
	})
	if err != nil {
		t.Fatalf("list_mutations: %v", err)
	}
	var listed []contract.MutationProposal
	if err := json.Unmarshal([]byte(text), &listed); err != nil {
		t.Fatalf("decode list: %v", err)
	}
	if len(listed) != 1 || listed[0].ID != proposal.ID {
		t.Fatalf("unexpected mutation list: %+v", listed)
	}
}

func TestMutationMCPDeleteRejectsContent(t *testing.T) {
	server, database, vault := setupKnowledgeMCPServer(t)
	defer database.Close()
	server.RegisterMutationTools()
	if err := os.WriteFile(filepath.Join(vault, "delete.md"), []byte("content\n"), 0o644); err != nil {
		t.Fatal(err)
	}

	_, err := server.tools["agentvault.propose_mutation"].Handler(map[string]interface{}{
		"kind":    "delete",
		"path":    "delete.md",
		"content": "must not be accepted",
		"reason":  "bad request",
	})
	if err == nil {
		t.Fatal("delete proposal with content should fail")
	}
	if got, _ := os.ReadFile(filepath.Join(vault, "delete.md")); string(got) != "content\n" {
		t.Fatalf("failed proposal changed target: %q", got)
	}
}

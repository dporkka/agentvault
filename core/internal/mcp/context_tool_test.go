package mcp

import (
	"encoding/json"
	"testing"

	"github.com/agentvault/core/internal/contract"
	"github.com/agentvault/core/internal/knowledge"
)

func TestCompileContextTool(t *testing.T) {
	server, database, vault := setupKnowledgeMCPServer(t)
	defer database.Close()
	server.RegisterContextTool()

	store := knowledge.New(database, vault)
	confidence := 0.95
	if _, err := store.RecordMemory(contract.CreateMemoryRequest{
		ID:          "mem_mcp_context",
		MemoryClass: "procedural",
		MemoryKind:  "procedure",
		ScopeType:   "project",
		ScopeID:     "agentvault",
		Content:     "Run focused tests before merging context compiler changes.",
		Confidence:  &confidence,
	}); err != nil {
		t.Fatal(err)
	}

	tool, ok := server.tools["agentvault.compile_context"]
	if !ok {
		t.Fatal("compile_context tool is not registered")
	}
	text, err := tool.Handler(map[string]interface{}{
		"task":         "test context compiler changes",
		"project":      "agentvault",
		"token_budget": float64(500),
		"max_items":    float64(10),
		"as_of":        "2026-09-10T12:00:00Z",
	})
	if err != nil {
		t.Fatalf("compile_context: %v", err)
	}
	var bundle contract.ContextBundle
	if err := json.Unmarshal([]byte(text), &bundle); err != nil {
		t.Fatalf("decode bundle: %v", err)
	}
	if bundle.EstimatedTokens > bundle.TokenBudget {
		t.Fatalf("bundle exceeded token budget: %+v", bundle)
	}
	found := false
	for _, item := range bundle.Items {
		if item.ID == "mem_mcp_context" {
			found = true
			if item.Metadata["memoryClass"] != "procedural" || item.Metadata["memoryKind"] != "procedure" {
				t.Fatalf("unexpected memory metadata: %+v", item.Metadata)
			}
			break
		}
	}
	if !found {
		t.Fatalf("expected procedural memory in compiled context: %+v", bundle.Items)
	}
}

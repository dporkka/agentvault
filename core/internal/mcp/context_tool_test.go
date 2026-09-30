package mcp

import (
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"errors"
	"os"
	"path/filepath"
	"testing"

	"github.com/agentvault/core/internal/contextcompiler"
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

func TestCompileContextToolAppliesSavedViewScope(t *testing.T) {
	server, database, vault := setupKnowledgeMCPServer(t)
	defer database.Close()

	viewDir := filepath.Join(vault, ".agentvault", "views")
	if err := os.MkdirAll(viewDir, 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(viewDir, "agentvault-only.yaml"), []byte(`version: 1
name: AgentVault only
query:
  projects: [agentvault]
`), 0o644); err != nil {
		t.Fatal(err)
	}

	server.RegisterContextTool()
	tool := server.tools["agentvault.compile_context"]
	text, err := tool.Handler(map[string]interface{}{
		"task":         "context compiler",
		"project":      "agentvault",
		"view_id":      "agentvault-only",
		"as_of":        "2026-09-10T12:00:00Z",
		"max_items":    float64(10),
		"token_budget": float64(1000),
	})
	if err != nil {
		t.Fatalf("compile_context: %v", err)
	}

	var bundle contract.ContextBundle
	if err := json.Unmarshal([]byte(text), &bundle); err != nil {
		t.Fatal(err)
	}
	if bundle.ViewID != "agentvault-only" {
		t.Fatalf("viewId = %q, want agentvault-only", bundle.ViewID)
	}
}

func TestCompileContextToolRejectsChangedPinnedView(t *testing.T) {
	server, database, vault := setupKnowledgeMCPServer(t)
	defer database.Close()

	viewDir := filepath.Join(vault, ".agentvault", "views")
	if err := os.MkdirAll(viewDir, 0o755); err != nil {
		t.Fatal(err)
	}
	original := []byte("version: 1\nname: Pinned\nquery:\n  projects: [agentvault]\n")
	viewPath := filepath.Join(viewDir, "pinned.yaml")
	if err := os.WriteFile(viewPath, original, 0o644); err != nil {
		t.Fatal(err)
	}
	sum := sha256.Sum256(original)
	expectedHash := hex.EncodeToString(sum[:])
	if err := os.WriteFile(viewPath, append(original, []byte("# changed\n")...), 0o644); err != nil {
		t.Fatal(err)
	}

	server.RegisterContextTool()
	tool := server.tools["agentvault.compile_context"]
	_, err := tool.Handler(map[string]interface{}{
		"task":                       "context compiler",
		"project":                    "agentvault",
		"view_id":                    "pinned",
		"expected_view_content_hash": expectedHash,
	})
	if !errors.Is(err, contextcompiler.ErrViewContentHashMismatch) {
		t.Fatalf("expected ErrViewContentHashMismatch, got %v", err)
	}
}

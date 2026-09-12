package mcp

import (
	"context"
	"encoding/json"
	"strings"
	"testing"

	"github.com/agentvault/core/internal/memory"
)

func TestRecallMemoriesToolReturnsScopedRecords(t *testing.T) {
	s, database := setupTestServer(t)
	defer database.Close()

	addTestNote(t, database, "memory_tool_1", "Scoped Fact", "10-notes/scoped-fact.md", "note", "", "Evidence body", nil)
	if _, err := database.Exec(`
		UPDATE notes
		SET workspace_id = 'acme', memory_kind = 'fact', confidence = 0.92,
			observed_at = '2026-02-01T00:00:00Z'
		WHERE id = 'memory_tool_1'
	`); err != nil {
		t.Fatalf("classify test memory: %v", err)
	}

	req := JSONRPCRequest{
		JSONRPC: "2.0",
		ID:      float64(100),
		Method:  "tools/call",
		Params: map[string]interface{}{
			"name": "agentvault.recall_memories",
			"arguments": map[string]interface{}{
				"workspace":      "acme",
				"kind":           "fact",
				"min_confidence": float64(0.9),
				"limit":          float64(5),
			},
		},
	}

	resp := s.Handle(context.Background(), req)
	if resp.Error != nil {
		t.Fatalf("unexpected JSON-RPC error: %v", resp.Error)
	}
	result, ok := resp.Result.(toolCallResult)
	if !ok || len(result.Content) != 1 {
		t.Fatalf("unexpected tool result: %#v", resp.Result)
	}

	var records []memory.Record
	if err := json.Unmarshal([]byte(result.Content[0].Text), &records); err != nil {
		t.Fatalf("decode recall output: %v\n%s", err, result.Content[0].Text)
	}
	if len(records) != 1 {
		t.Fatalf("expected one memory, got %d: %#v", len(records), records)
	}
	if records[0].NoteID != "memory_tool_1" || records[0].Kind != memory.KindFact {
		t.Fatalf("unexpected memory: %#v", records[0])
	}
	if records[0].Scope.WorkspaceID != "acme" {
		t.Fatalf("workspace = %q", records[0].Scope.WorkspaceID)
	}
}

func TestRecallMemoriesToolRejectsInvalidFilters(t *testing.T) {
	s, database := setupTestServer(t)
	defer database.Close()

	for _, args := range []map[string]interface{}{
		{"kind": "unknown"},
		{"min_confidence": float64(1.2)},
		{"at": "yesterday"},
		{"include_superseded": "true"},
		{"limit": float64(0)},
	} {
		text, err := s.handleRecallMemories(args)
		if err == nil {
			t.Fatalf("expected invalid args %#v to fail, output %q", args, text)
		}
	}
}

func TestRecallMemoriesToolAnnotations(t *testing.T) {
	annotations := annotationsForTool("agentvault.recall_memories")
	if !annotations.ReadOnlyHint || !annotations.IdempotentHint {
		t.Fatalf("recall must be read-only and idempotent: %#v", annotations)
	}
	if annotations.DestructiveHint || annotations.OpenWorldHint {
		t.Fatalf("recall must remain local/non-destructive: %#v", annotations)
	}

	data, err := json.Marshal(toolDescription{
		Name:        "agentvault.recall_memories",
		Description: "Recall memories",
		InputSchema: map[string]interface{}{"type": "object"},
	})
	if err != nil {
		t.Fatalf("marshal tool description: %v", err)
	}
	if !strings.Contains(string(data), `"readOnlyHint":true`) ||
		!strings.Contains(string(data), `"idempotentHint":true`) {
		t.Fatalf("tools/list annotations missing from %s", data)
	}
}

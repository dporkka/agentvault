package mcp

import (
	"context"
	"strings"
	"testing"

	"github.com/agentvault/core/internal/agents"
	"github.com/agentvault/core/internal/events"
)

func TestMemoryToolsCreateCandidateAndPromote(t *testing.T) {
	s, database := setupTestServer(t)
	defer database.Close()

	agent, err := agents.NewStore(database).Create(context.Background(), agents.CreateInput{Name: "codex"})
	if err != nil {
		t.Fatalf("create agent: %v", err)
	}
	sourceEvent, err := events.NewStore(database).Append(context.Background(), events.AppendInput{
		Type:      "observation.recorded",
		ActorType: "agent",
		ActorID:   agent.ID,
		Payload:   map[string]interface{}{"text": "Prefer focused tests first."},
	})
	if err != nil {
		t.Fatalf("create source event: %v", err)
	}

	createResp := s.Handle(context.Background(), JSONRPCRequest{
		JSONRPC: "2.0",
		ID:      float64(81),
		Method:  "tools/call",
		Params: map[string]interface{}{
			"name": "agentvault.create_memory_candidate",
			"arguments": map[string]interface{}{
				"agent_name":      "codex",
				"kind":            "procedural",
				"content":         "Run focused tests before the full suite.",
				"confidence":      0.9,
				"salience":        0.85,
				"scope_type":      "project",
				"scope_id":        "agentvault",
				"source_event_id": sourceEvent.ID,
			},
		},
	})
	if createResp.Error != nil {
		t.Fatalf("create memory JSON-RPC error: %v", createResp.Error)
	}
	createResult, ok := createResp.Result.(toolCallResult)
	if !ok || len(createResult.Content) == 0 {
		t.Fatalf("unexpected create result: %#v", createResp.Result)
	}
	if strings.Contains(createResult.Content[0].Text, "Error:") {
		t.Fatalf("create candidate failed: %s", createResult.Content[0].Text)
	}

	var memoryID string
	if err := database.QueryRow(
		"SELECT id FROM memories WHERE agent_id = ? AND status = 'candidate'",
		agent.ID,
	).Scan(&memoryID); err != nil {
		t.Fatalf("candidate was not persisted: %v", err)
	}

	promoteResp := s.Handle(context.Background(), JSONRPCRequest{
		JSONRPC: "2.0",
		ID:      float64(82),
		Method:  "tools/call",
		Params: map[string]interface{}{
			"name": "agentvault.promote_memory",
			"arguments": map[string]interface{}{
				"memory_id":  memoryID,
				"actor_name": "codex",
				"reason":     "Repeatedly validated.",
			},
		},
	})
	if promoteResp.Error != nil {
		t.Fatalf("promote memory JSON-RPC error: %v", promoteResp.Error)
	}
	promoteResult, ok := promoteResp.Result.(toolCallResult)
	if !ok || len(promoteResult.Content) == 0 {
		t.Fatalf("unexpected promote result: %#v", promoteResp.Result)
	}
	if strings.Contains(promoteResult.Content[0].Text, "Error:") {
		t.Fatalf("promote memory failed: %s", promoteResult.Content[0].Text)
	}

	var status string
	if err := database.QueryRow("SELECT status FROM memories WHERE id = ?", memoryID).Scan(&status); err != nil {
		t.Fatalf("query promoted memory: %v", err)
	}
	if status != "durable" {
		t.Fatalf("status = %q, want durable", status)
	}
}

func TestMemoryToolsAreRegistered(t *testing.T) {
	s, database := setupTestServer(t)
	defer database.Close()

	if _, ok := s.tools["agentvault.create_memory_candidate"]; !ok {
		t.Fatal("create_memory_candidate tool is not registered")
	}
	if _, ok := s.tools["agentvault.promote_memory"]; !ok {
		t.Fatal("promote_memory tool is not registered")
	}
}

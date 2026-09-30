package mcp

import (
	"context"
	"testing"
)

func TestLogAgentRunCreatesCanonicalAgentAndEvent(t *testing.T) {
	s, database := setupTestServer(t)
	defer database.Close()

	req := JSONRPCRequest{
		JSONRPC: "2.0",
		ID:      float64(71),
		Method:  "tools/call",
		Params: map[string]interface{}{
			"name": "agentvault.log_agent_run",
			"arguments": map[string]interface{}{
				"agent_name":    "codex",
				"task":          "Implement persistent agent state",
				"files_changed": []interface{}{"core/internal/agents/store.go"},
			},
		},
	}

	resp := s.Handle(context.Background(), req)
	if resp.Error != nil {
		t.Fatalf("unexpected JSON-RPC error: %v", resp.Error)
	}
	result, ok := resp.Result.(toolCallResult)
	if !ok || len(result.Content) == 0 {
		t.Fatalf("expected toolCallResult, got %#v", resp.Result)
	}

	var agentID string
	if err := database.QueryRow("SELECT id FROM agents WHERE name = ?", "codex").Scan(&agentID); err != nil {
		t.Fatalf("canonical agent was not created: %v", err)
	}
	if agentID == "" {
		t.Fatal("canonical agent id is empty")
	}

	var runID string
	if err := database.QueryRow("SELECT id FROM agent_runs WHERE agent_id = ?", agentID).Scan(&runID); err != nil {
		t.Fatalf("agent run is not linked to canonical agent: %v", err)
	}

	var eventCount int
	if err := database.QueryRow(
		"SELECT COUNT(*) FROM events WHERE event_type = ? AND actor_id = ? AND run_id = ?",
		"agent.run.logged", agentID, runID,
	).Scan(&eventCount); err != nil {
		t.Fatalf("query event: %v", err)
	}
	if eventCount != 1 {
		t.Fatalf("expected one agent.run.logged event, got %d", eventCount)
	}
}

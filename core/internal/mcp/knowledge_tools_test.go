package mcp

import (
	"encoding/json"
	"os"
	"path/filepath"
	"testing"

	"github.com/agentvault/core/internal/db"
)

func setupKnowledgeMCPServer(t *testing.T) (*Server, *db.DB, string) {
	t.Helper()
	vault := t.TempDir()
	if err := os.MkdirAll(filepath.Join(vault, ".agentvault"), 0o755); err != nil {
		t.Fatal(err)
	}
	database, err := db.Open(vault)
	if err != nil {
		t.Fatal(err)
	}
	if err := database.RunMigrations(); err != nil {
		database.Close()
		t.Fatal(err)
	}
	server := NewServer(vault, database)
	server.RegisterKnowledgeTools()
	return server, database, vault
}

func TestRegisterKnowledgeTools(t *testing.T) {
	server, database, _ := setupKnowledgeMCPServer(t)
	defer database.Close()

	expected := []string{
		"agentvault.create_provenance",
		"agentvault.upsert_object",
		"agentvault.get_object",
		"agentvault.create_relation",
		"agentvault.record_memory",
		"agentvault.list_memories",
		"agentvault.start_session",
		"agentvault.append_session_event",
		"agentvault.get_session",
		"agentvault.close_session",
	}
	if len(server.tools) != len(expected) {
		t.Fatalf("expected %d knowledge tools, got %d", len(expected), len(server.tools))
	}
	for _, name := range expected {
		tool, ok := server.tools[name]
		if !ok {
			t.Errorf("missing knowledge tool %s", name)
			continue
		}
		if tool.Description == "" || tool.InputSchema == nil || tool.Handler == nil {
			t.Errorf("knowledge tool %s is incomplete", name)
		}
	}
}

func TestKnowledgeToolsEndToEnd(t *testing.T) {
	server, database, vault := setupKnowledgeMCPServer(t)
	defer database.Close()

	call := func(name string, args map[string]interface{}, target interface{}) {
		t.Helper()
		tool, ok := server.tools[name]
		if !ok {
			t.Fatalf("tool %s is not registered", name)
		}
		text, err := tool.Handler(args)
		if err != nil {
			t.Fatalf("%s: %v", name, err)
		}
		if target != nil {
			if err := json.Unmarshal([]byte(text), target); err != nil {
				t.Fatalf("decode %s response %q: %v", name, text, err)
			}
		}
	}

	var provenance struct {
		ID string `json:"id"`
	}
	call("agentvault.create_provenance", map[string]interface{}{
		"id":          "prov_mcp",
		"source_type": "agent-session",
		"agent_id":    "architect",
		"confidence":  0.95,
		"evidence": []interface{}{
			map[string]interface{}{"source": "file", "path": "README.md"},
		},
	}, &provenance)
	if provenance.ID != "prov_mcp" {
		t.Fatalf("unexpected provenance id %q", provenance.ID)
	}

	var project struct {
		ID string `json:"id"`
	}
	call("agentvault.upsert_object", map[string]interface{}{
		"id":            "obj_mcp_project",
		"type":          "project",
		"title":         "AgentVault",
		"project":       "agentvault",
		"provenance_id": provenance.ID,
	}, &project)

	var decision struct {
		ID string `json:"id"`
	}
	call("agentvault.upsert_object", map[string]interface{}{
		"id":             "obj_mcp_decision",
		"type":           "decision",
		"title":          "Use one shared durable knowledge substrate",
		"project":        "agentvault",
		"canonical_path": "30-decisions/shared-knowledge.md",
		"provenance_id":  provenance.ID,
	}, &decision)

	call("agentvault.create_relation", map[string]interface{}{
		"id":             "rel_mcp",
		"from_object_id": decision.ID,
		"to_object_id":   project.ID,
		"relation_type":  "affects",
		"provenance_id":  provenance.ID,
	}, nil)

	var objectResult struct {
		Object struct {
			ID string `json:"id"`
		} `json:"object"`
		Relations []struct {
			ID string `json:"id"`
		} `json:"relations"`
	}
	call("agentvault.get_object", map[string]interface{}{"id": decision.ID}, &objectResult)
	if objectResult.Object.ID != decision.ID || len(objectResult.Relations) != 1 {
		t.Fatalf("unexpected get_object result: %+v", objectResult)
	}

	call("agentvault.record_memory", map[string]interface{}{
		"id":            "mem_mcp",
		"memory_class":  "semantic",
		"memory_kind":   "fact",
		"scope_type":    "project",
		"scope_id":      "agentvault",
		"content":       "AgentVault knowledge writes are journal-first.",
		"object_id":     decision.ID,
		"provenance_id": provenance.ID,
	}, nil)

	var memories []struct {
		ID          string `json:"id"`
		MemoryClass string `json:"memoryClass"`
		MemoryKind  string `json:"memoryKind"`
	}
	call("agentvault.list_memories", map[string]interface{}{
		"scope_type":   "project",
		"scope_id":     "agentvault",
		"memory_class": "semantic",
	}, &memories)
	if len(memories) != 1 || memories[0].ID != "mem_mcp" || memories[0].MemoryClass != "semantic" || memories[0].MemoryKind != "fact" {
		t.Fatalf("unexpected memories: %+v", memories)
	}

	var session struct {
		ID     string `json:"id"`
		Status string `json:"status"`
	}
	call("agentvault.start_session", map[string]interface{}{
		"id":        "session_mcp",
		"agent_id":  "backend-engineer",
		"project":   "agentvault",
		"objective": "Exercise the knowledge MCP surface",
		"branch":    "feat/unified-memory-core",
	}, &session)
	if session.Status != "active" {
		t.Fatalf("unexpected initial session: %+v", session)
	}

	call("agentvault.append_session_event", map[string]interface{}{
		"session_id": session.ID,
		"id":         "event_mcp",
		"event_type": "verification",
		"payload":    map[string]interface{}{"passed": true},
	}, nil)

	var loadedSession struct {
		ID     string `json:"id"`
		Events []struct {
			ID string `json:"id"`
		} `json:"events"`
	}
	call("agentvault.get_session", map[string]interface{}{"session_id": session.ID}, &loadedSession)
	if loadedSession.ID != session.ID || len(loadedSession.Events) != 1 || loadedSession.Events[0].ID != "event_mcp" {
		t.Fatalf("unexpected session history: %+v", loadedSession)
	}

	var closedSession struct {
		Status  string `json:"status"`
		EndedAt string `json:"endedAt"`
	}
	call("agentvault.close_session", map[string]interface{}{
		"session_id": session.ID,
		"status":     "completed",
	}, &closedSession)
	if closedSession.Status != "completed" || closedSession.EndedAt == "" {
		t.Fatalf("unexpected closed session: %+v", closedSession)
	}

	journalPath := filepath.Join(vault, "80-agent-runs", "knowledge.journal.jsonl")
	if info, err := os.Stat(journalPath); err != nil || info.Size() == 0 {
		t.Fatalf("expected MCP mutations to persist canonical journal %s: info=%v err=%v", journalPath, info, err)
	}
}

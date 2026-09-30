package mcp

import (
	"errors"
	"strings"
	"testing"

	"github.com/agentvault/core/internal/authz"
	"github.com/agentvault/core/internal/contract"
	"github.com/agentvault/core/internal/knowledge"
)

func TestScopedDurableWritesRequireSessionBoundProvenance(t *testing.T) {
	_, database, vault := setupKnowledgeMCPServer(t)
	defer database.Close()
	store := knowledge.New(database, vault)
	if err := store.ReplayJournal(); err != nil {
		t.Fatal(err)
	}
	session, err := store.StartSession(contract.StartAgentSessionRequest{AgentID: "agent", Project: "alpha", Objective: "work"})
	if err != nil {
		t.Fatal(err)
	}

	registry, err := authz.NewRegistry(vault)
	if err != nil {
		t.Fatal(err)
	}
	issued, err := registry.Mint(authz.MintRequest{
		AgentID: "agent",
		Capabilities: []authz.Capability{
			authz.KnowledgeWrite,
			authz.MemoryWrite,
		},
		Scope: authz.Scope{Projects: []string{"alpha"}, Sessions: []string{session.ID}},
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

	if _, err := server.tools["agentvault.upsert_object"].Handler(map[string]interface{}{
		"type": "decision", "title": "No provenance", "project": "alpha", "session_id": session.ID,
	}); !errors.Is(err, authz.ErrForbidden) || !strings.Contains(err.Error(), "provenance") {
		t.Fatalf("scoped object without provenance should fail, got %v", err)
	}
	if _, err := server.tools["agentvault.record_memory"].Handler(map[string]interface{}{
		"memory_class": "semantic", "scope_type": "session", "scope_id": session.ID, "content": "No provenance",
	}); !errors.Is(err, authz.ErrForbidden) || !strings.Contains(err.Error(), "provenance") {
		t.Fatalf("scoped memory without provenance should fail, got %v", err)
	}
}

func TestScopedSessionWriteRejectsUnknownTerminalStatus(t *testing.T) {
	_, database, vault := setupKnowledgeMCPServer(t)
	defer database.Close()
	store := knowledge.New(database, vault)
	if err := store.ReplayJournal(); err != nil {
		t.Fatal(err)
	}
	session, err := store.StartSession(contract.StartAgentSessionRequest{AgentID: "agent", Project: "alpha", Objective: "work"})
	if err != nil {
		t.Fatal(err)
	}
	registry, err := authz.NewRegistry(vault)
	if err != nil {
		t.Fatal(err)
	}
	issued, err := registry.Mint(authz.MintRequest{
		AgentID: "agent", Capabilities: []authz.Capability{authz.SessionWrite},
		Scope: authz.Scope{Projects: []string{"alpha"}, Sessions: []string{session.ID}},
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

	if _, err := server.tools["agentvault.close_session"].Handler(map[string]interface{}{
		"session_id": session.ID, "status": "arbitrary-terminal-state",
	}); err == nil || !strings.Contains(err.Error(), "unsupported session terminal status") {
		t.Fatalf("unknown terminal status should fail, got %v", err)
	}
	loaded, err := store.GetSession(session.ID)
	if err != nil {
		t.Fatal(err)
	}
	if loaded.Status != "active" {
		t.Fatalf("invalid close changed session status to %q", loaded.Status)
	}
}

package mcp

import (
	"encoding/json"
	"errors"
	"testing"

	"github.com/agentvault/core/internal/authz"
	"github.com/agentvault/core/internal/contract"
	"github.com/agentvault/core/internal/knowledge"
)

func TestScopedKnowledgeWriteAuthorizesGraphResources(t *testing.T) {
	_, database, vault := setupKnowledgeMCPServer(t)
	defer database.Close()
	store := knowledge.New(database, vault)
	if err := store.ReplayJournal(); err != nil {
		t.Fatal(err)
	}
	alphaSession, err := store.StartSession(contract.StartAgentSessionRequest{AgentID: "writer", Project: "alpha", Objective: "alpha"})
	if err != nil {
		t.Fatal(err)
	}
	betaSession, err := store.StartSession(contract.StartAgentSessionRequest{AgentID: "writer", Project: "beta", Objective: "beta"})
	if err != nil {
		t.Fatal(err)
	}
	betaObject, err := store.UpsertObject(contract.UpsertKnowledgeObjectRequest{Type: "decision", Title: "Beta", Project: "beta"})
	if err != nil {
		t.Fatal(err)
	}

	registry, err := authz.NewRegistry(vault)
	if err != nil {
		t.Fatal(err)
	}
	issued, err := registry.Mint(authz.MintRequest{
		AgentID:      "writer",
		Capabilities: []authz.Capability{authz.KnowledgeWrite},
		Scope: authz.Scope{
			Projects: []string{"alpha"},
			Sessions: []string{alphaSession.ID},
		},
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
	for _, name := range []string{"agentvault.create_provenance", "agentvault.upsert_object", "agentvault.create_relation"} {
		if _, ok := server.tools[name]; !ok {
			t.Fatalf("missing knowledge:write tool %s", name)
		}
	}
	for _, name := range []string{"agentvault.record_memory", "agentvault.start_session", "agentvault.get_object"} {
		if _, ok := server.tools[name]; ok {
			t.Fatalf("knowledge:write unexpectedly granted %s", name)
		}
	}

	text, err := server.tools["agentvault.create_provenance"].Handler(map[string]interface{}{
		"source_type": "agent-session",
		"session_id":  alphaSession.ID,
		"confidence":  0.9,
	})
	if err != nil {
		t.Fatal(err)
	}
	var provenance contract.ProvenanceRecord
	if err := json.Unmarshal([]byte(text), &provenance); err != nil {
		t.Fatal(err)
	}
	if provenance.AgentID != "writer" || provenance.SessionID != alphaSession.ID {
		t.Fatalf("provenance identity was not bound: %+v", provenance)
	}

	createObject := func(title string) contract.KnowledgeObject {
		t.Helper()
		text, err := server.tools["agentvault.upsert_object"].Handler(map[string]interface{}{
			"type": "decision", "title": title, "project": "alpha", "session_id": alphaSession.ID,
			"provenance_id": provenance.ID,
		})
		if err != nil {
			t.Fatal(err)
		}
		var object contract.KnowledgeObject
		if err := json.Unmarshal([]byte(text), &object); err != nil {
			t.Fatal(err)
		}
		return object
	}
	first := createObject("First")
	second := createObject("Second")
	if first.ID == "" || second.ID == "" || first.Project != "alpha" {
		t.Fatalf("unexpected scoped object: %+v %+v", first, second)
	}

	if _, err := server.tools["agentvault.upsert_object"].Handler(map[string]interface{}{
		"id": "guessed-new-id", "type": "decision", "title": "Probe", "project": "alpha", "session_id": alphaSession.ID,
	}); !errors.Is(err, authz.ErrForbidden) {
		t.Fatalf("scoped create with caller ID should fail closed, got %v", err)
	}
	if _, err := server.tools["agentvault.upsert_object"].Handler(map[string]interface{}{
		"id": betaObject.ID, "type": "decision", "title": "Hijack", "project": "alpha", "session_id": alphaSession.ID,
	}); !errors.Is(err, authz.ErrForbidden) {
		t.Fatalf("cross-project object hijack should fail, got %v", err)
	}
	if _, err := server.tools["agentvault.upsert_object"].Handler(map[string]interface{}{
		"type": "artifact", "title": "Path injection", "project": "alpha", "session_id": alphaSession.ID,
		"canonical_path": "30-projects/beta/secret.md",
	}); !errors.Is(err, authz.ErrForbidden) {
		t.Fatalf("scoped canonical path injection should fail, got %v", err)
	}

	if _, err := server.tools["agentvault.create_relation"].Handler(map[string]interface{}{
		"from_object_id": first.ID, "to_object_id": second.ID, "relation_type": "depends_on",
		"session_id": alphaSession.ID, "provenance_id": provenance.ID,
	}); err != nil {
		t.Fatalf("authorized relation: %v", err)
	}
	if _, err := server.tools["agentvault.create_relation"].Handler(map[string]interface{}{
		"from_object_id": first.ID, "to_object_id": betaObject.ID, "relation_type": "leaks_to",
		"session_id": alphaSession.ID,
	}); !errors.Is(err, authz.ErrForbidden) {
		t.Fatalf("cross-project relation should fail, got %v", err)
	}
	if _, err := server.tools["agentvault.create_provenance"].Handler(map[string]interface{}{
		"source_type": "agent-session", "session_id": betaSession.ID,
	}); !errors.Is(err, authz.ErrForbidden) {
		t.Fatalf("cross-project provenance should fail, got %v", err)
	}
}

func TestSessionScopedMemoryWriteCannotPromoteOrCrossScope(t *testing.T) {
	_, database, vault := setupKnowledgeMCPServer(t)
	defer database.Close()
	store := knowledge.New(database, vault)
	if err := store.ReplayJournal(); err != nil {
		t.Fatal(err)
	}
	alphaSession, _ := store.StartSession(contract.StartAgentSessionRequest{AgentID: "memory-agent", Project: "alpha", Objective: "alpha"})
	betaSession, _ := store.StartSession(contract.StartAgentSessionRequest{AgentID: "memory-agent", Project: "beta", Objective: "beta"})
	alphaObject, _ := store.UpsertObject(contract.UpsertKnowledgeObjectRequest{Type: "task", Title: "Alpha", Project: "alpha"})
	betaObject, _ := store.UpsertObject(contract.UpsertKnowledgeObjectRequest{Type: "task", Title: "Beta", Project: "beta"})
	alphaProv, _ := store.CreateProvenance(contract.ProvenanceRecord{SourceType: "agent-session", AgentID: "memory-agent", SessionID: alphaSession.ID, Confidence: 1})
	betaProv, _ := store.CreateProvenance(contract.ProvenanceRecord{SourceType: "agent-session", AgentID: "memory-agent", SessionID: betaSession.ID, Confidence: 1})
	old, _ := store.RecordMemory(contract.CreateMemoryRequest{MemoryClass: "semantic", ScopeType: "session", ScopeID: alphaSession.ID, Content: "old"})
	projectOld, _ := store.RecordMemory(contract.CreateMemoryRequest{MemoryClass: "semantic", ScopeType: "project", ScopeID: "alpha", Content: "project old"})

	registry, err := authz.NewRegistry(vault)
	if err != nil {
		t.Fatal(err)
	}
	issued, err := registry.Mint(authz.MintRequest{
		AgentID: "memory-agent", Capabilities: []authz.Capability{authz.MemoryWrite},
		Scope: authz.Scope{Projects: []string{"alpha"}, Sessions: []string{alphaSession.ID}},
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
	if _, ok := server.tools["agentvault.record_memory"]; !ok {
		t.Fatal("memory:write did not register record_memory")
	}
	if _, ok := server.tools["agentvault.upsert_object"]; ok {
		t.Fatal("memory:write must not imply knowledge:write")
	}

	text, err := server.tools["agentvault.record_memory"].Handler(map[string]interface{}{
		"memory_class": "semantic", "scope_type": "session", "scope_id": alphaSession.ID,
		"content": "new", "object_id": alphaObject.ID, "provenance_id": alphaProv.ID, "supersedes_id": old.ID,
	})
	if err != nil {
		t.Fatal(err)
	}
	var memory contract.MemoryRecord
	if err := json.Unmarshal([]byte(text), &memory); err != nil {
		t.Fatal(err)
	}
	if memory.ScopeID != alphaSession.ID || memory.SupersedesID != old.ID {
		t.Fatalf("unexpected memory: %+v", memory)
	}

	for name, args := range map[string]map[string]interface{}{
		"project promotion": {"memory_class": "semantic", "scope_type": "project", "scope_id": "alpha", "content": "promote"},
		"other session": {"memory_class": "semantic", "scope_type": "session", "scope_id": betaSession.ID, "content": "cross"},
		"other object": {"memory_class": "semantic", "scope_type": "session", "scope_id": alphaSession.ID, "content": "cross object", "object_id": betaObject.ID},
		"other provenance": {"memory_class": "semantic", "scope_type": "session", "scope_id": alphaSession.ID, "content": "cross provenance", "provenance_id": betaProv.ID},
		"cross-scope supersede": {"memory_class": "semantic", "scope_type": "session", "scope_id": alphaSession.ID, "content": "cross supersede", "supersedes_id": projectOld.ID},
		"explicit id": {"id": "mem_probe", "memory_class": "semantic", "scope_type": "session", "scope_id": alphaSession.ID, "content": "probe"},
	} {
		t.Run(name, func(t *testing.T) {
			if _, err := server.tools["agentvault.record_memory"].Handler(args); !errors.Is(err, authz.ErrForbidden) {
				t.Fatalf("expected scoped memory denial, got %v", err)
			}
		})
	}
}

func TestProjectScopedSessionWriteBindsAgentAndLifecycle(t *testing.T) {
	_, database, vault := setupKnowledgeMCPServer(t)
	defer database.Close()
	store := knowledge.New(database, vault)
	if err := store.ReplayJournal(); err != nil {
		t.Fatal(err)
	}
	betaSession, _ := store.StartSession(contract.StartAgentSessionRequest{AgentID: "other", Project: "beta", Objective: "beta"})

	registry, err := authz.NewRegistry(vault)
	if err != nil {
		t.Fatal(err)
	}
	issued, err := registry.Mint(authz.MintRequest{
		AgentID: "session-agent", Capabilities: []authz.Capability{authz.SessionWrite}, Scope: authz.Scope{Projects: []string{"alpha"}},
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
	for _, name := range []string{"agentvault.start_session", "agentvault.append_session_event", "agentvault.close_session"} {
		if _, ok := server.tools[name]; !ok {
			t.Fatalf("missing session:write tool %s", name)
		}
	}

	text, err := server.tools["agentvault.start_session"].Handler(map[string]interface{}{
		"project": "alpha", "objective": "authorized", "agent_id": "spoofed",
	})
	if err != nil {
		t.Fatal(err)
	}
	var session contract.AgentSession
	if err := json.Unmarshal([]byte(text), &session); err != nil {
		t.Fatal(err)
	}
	if session.AgentID != "session-agent" || session.Project != "alpha" {
		t.Fatalf("session identity/project not bound: %+v", session)
	}
	if _, err := server.tools["agentvault.start_session"].Handler(map[string]interface{}{
		"id": "session_probe", "project": "alpha", "objective": "probe",
	}); !errors.Is(err, authz.ErrForbidden) {
		t.Fatalf("explicit scoped session ID should fail, got %v", err)
	}
	if _, err := server.tools["agentvault.start_session"].Handler(map[string]interface{}{
		"project": "beta", "objective": "escape",
	}); !errors.Is(err, authz.ErrForbidden) {
		t.Fatalf("cross-project session start should fail, got %v", err)
	}

	if _, err := server.tools["agentvault.append_session_event"].Handler(map[string]interface{}{
		"session_id": session.ID, "event_type": "decision", "payload": map[string]interface{}{"ok": true},
	}); err != nil {
		t.Fatalf("append authorized event: %v", err)
	}
	if _, err := server.tools["agentvault.append_session_event"].Handler(map[string]interface{}{
		"session_id": betaSession.ID, "event_type": "decision",
	}); !errors.Is(err, authz.ErrForbidden) {
		t.Fatalf("cross-project event should fail, got %v", err)
	}
	if _, err := server.tools["agentvault.close_session"].Handler(map[string]interface{}{"session_id": session.ID}); err != nil {
		t.Fatalf("close authorized session: %v", err)
	}
}

func TestSessionScopedSessionWriteCannotMintNewSession(t *testing.T) {
	_, database, vault := setupKnowledgeMCPServer(t)
	defer database.Close()
	store := knowledge.New(database, vault)
	if err := store.ReplayJournal(); err != nil {
		t.Fatal(err)
	}
	session, _ := store.StartSession(contract.StartAgentSessionRequest{AgentID: "agent", Project: "alpha", Objective: "existing"})
	registry, _ := authz.NewRegistry(vault)
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
	if _, err := server.tools["agentvault.start_session"].Handler(map[string]interface{}{"project": "alpha", "objective": "new"}); !errors.Is(err, authz.ErrForbidden) {
		t.Fatalf("session-scoped identity should not mint sessions, got %v", err)
	}
	if _, err := server.tools["agentvault.append_session_event"].Handler(map[string]interface{}{"session_id": session.ID, "event_type": "verification"}); err != nil {
		t.Fatalf("session-scoped append should succeed: %v", err)
	}
}

func TestDurableWriteCapabilitiesRejectPathScope(t *testing.T) {
	for _, capability := range []authz.Capability{authz.KnowledgeWrite, authz.MemoryWrite, authz.SessionWrite} {
		t.Run(string(capability), func(t *testing.T) {
			_, database, vault := setupKnowledgeMCPServer(t)
			defer database.Close()
			registry, _ := authz.NewRegistry(vault)
			issued, err := registry.Mint(authz.MintRequest{
				AgentID: "agent", Capabilities: []authz.Capability{capability}, Scope: authz.Scope{PathPrefixes: []string{"30-projects/alpha"}},
			})
			if err != nil {
				t.Fatal(err)
			}
			server := NewServer(vault, database)
			if err := server.SetCapabilityToken(issued.Token); err != nil {
				t.Fatal(err)
			}
			if err := server.RegisterRuntimeSurface(false); err == nil {
				t.Fatal("path-scoped durable write capability should fail registration")
			}
			if len(server.tools) != 0 {
				t.Fatalf("failed registration leaked %d tools", len(server.tools))
			}
		})
	}
}

package mcp

import (
	"encoding/json"
	"errors"
	"testing"

	"github.com/agentvault/core/internal/authz"
	"github.com/agentvault/core/internal/contract"
	"github.com/agentvault/core/internal/knowledge"
)

func seedCandidateEpisode(t *testing.T, store *knowledge.Store, project, sessionID string) contract.EpisodeRecord {
	t.Helper()
	provenance, err := store.CreateProvenance(contract.ProvenanceRecord{
		SourceType: "agent-session",
		AgentID:    "extractor",
		SessionID:  sessionID,
		Confidence: 0.9,
	})
	if err != nil {
		t.Fatal(err)
	}
	scopeType, scopeID := "project", project
	if sessionID != "" {
		scopeType, scopeID = "session", sessionID
	}
	episode, err := store.RecordEpisode(contract.CreateEpisodeRequest{
		ScopeType:    scopeType,
		ScopeID:      scopeID,
		EventType:    "decision.observed",
		Summary:      "Observed a candidate decision",
		ProvenanceID: provenance.ID,
	})
	if err != nil {
		t.Fatal(err)
	}
	return episode
}

func TestTrustedMemoryCandidateToolsRequireReviewBeforeMemory(t *testing.T) {
	server, database, vault := setupKnowledgeMCPServer(t)
	defer database.Close()
	store := knowledge.New(database, vault)
	if err := store.ReplayJournal(); err != nil {
		t.Fatal(err)
	}
	episode := seedCandidateEpisode(t, store, "agentvault", "")

	server.RegisterMemoryCandidateTools()
	for _, name := range []string{
		"agentvault.propose_memory_candidate",
		"agentvault.extract_memory_candidates",
		"agentvault.list_memory_candidates",
		"agentvault.get_memory_candidate",
		"agentvault.accept_memory_candidate",
		"agentvault.reject_memory_candidate",
		"agentvault.merge_memory_candidate",
		"agentvault.supersede_memory_candidate",
	} {
		if _, ok := server.tools[name]; !ok {
			t.Fatalf("missing candidate tool %s", name)
		}
	}

	extractedText, err := server.tools["agentvault.extract_memory_candidates"].Handler(map[string]interface{}{
		"episode_id": episode.ID,
	})
	if err != nil {
		t.Fatal(err)
	}
	var extracted []contract.MemoryCandidate
	if err := json.Unmarshal([]byte(extractedText), &extracted); err != nil {
		t.Fatal(err)
	}
	if len(extracted) != 1 || extracted[0].MemoryKind != "decision" {
		t.Fatalf("unexpected trusted deterministic extraction: %+v", extracted)
	}

	text, err := server.tools["agentvault.propose_memory_candidate"].Handler(map[string]interface{}{
		"episode_id":  episode.ID,
		"memory_kind": "decision",
		"content":     "Review candidates before semantic promotion.",
		"proposed_by": "extractor",
	})
	if err != nil {
		t.Fatal(err)
	}
	var candidate contract.MemoryCandidate
	if err := json.Unmarshal([]byte(text), &candidate); err != nil {
		t.Fatal(err)
	}
	if candidate.Status != contract.MemoryCandidatePending {
		t.Fatalf("unexpected candidate: %+v", candidate)
	}
	memories, err := store.ListMemories("project", "agentvault", "semantic", 10)
	if err != nil {
		t.Fatal(err)
	}
	if len(memories) != 0 {
		t.Fatalf("proposal created memory before review: %+v", memories)
	}

	text, err = server.tools["agentvault.accept_memory_candidate"].Handler(map[string]interface{}{
		"id": candidate.ID, "reviewed_by": "reviewer", "reason": "verified",
	})
	if err != nil {
		t.Fatal(err)
	}
	var accepted contract.MemoryCandidate
	if err := json.Unmarshal([]byte(text), &accepted); err != nil {
		t.Fatal(err)
	}
	if accepted.Status != contract.MemoryCandidateAccepted || accepted.ResultMemoryID == "" {
		t.Fatalf("unexpected accepted candidate: %+v", accepted)
	}
}

func TestScopedMemoryWriterCanProposeButCannotReviewCandidates(t *testing.T) {
	_, database, vault := setupKnowledgeMCPServer(t)
	defer database.Close()
	store := knowledge.New(database, vault)
	if err := store.ReplayJournal(); err != nil {
		t.Fatal(err)
	}
	alphaSession, err := store.StartSession(contract.StartAgentSessionRequest{
		AgentID: "extractor", Project: "alpha", Objective: "extract alpha",
	})
	if err != nil {
		t.Fatal(err)
	}
	betaSession, err := store.StartSession(contract.StartAgentSessionRequest{
		AgentID: "extractor", Project: "beta", Objective: "extract beta",
	})
	if err != nil {
		t.Fatal(err)
	}
	alphaEpisode := seedCandidateEpisode(t, store, "alpha", alphaSession.ID)
	betaEpisode := seedCandidateEpisode(t, store, "beta", betaSession.ID)

	registry, err := authz.NewRegistry(vault)
	if err != nil {
		t.Fatal(err)
	}
	issued, err := registry.Mint(authz.MintRequest{
		AgentID:      "extractor",
		Capabilities: []authz.Capability{authz.MemoryWrite},
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
	if _, ok := server.tools["agentvault.propose_memory_candidate"]; !ok {
		t.Fatal("memory:write did not register candidate proposal")
	}
	if _, ok := server.tools["agentvault.extract_memory_candidates"]; !ok {
		t.Fatal("memory:write did not register deterministic extraction")
	}
	for _, name := range []string{
		"agentvault.accept_memory_candidate",
		"agentvault.reject_memory_candidate",
		"agentvault.merge_memory_candidate",
		"agentvault.supersede_memory_candidate",
	} {
		if _, ok := server.tools[name]; ok {
			t.Fatalf("scoped memory writer unexpectedly received review tool %s", name)
		}
	}

	extractedText, err := server.tools["agentvault.extract_memory_candidates"].Handler(map[string]interface{}{
		"episode_id": alphaEpisode.ID,
	})
	if err != nil {
		t.Fatal(err)
	}
	var extracted []contract.MemoryCandidate
	if err := json.Unmarshal([]byte(extractedText), &extracted); err != nil {
		t.Fatal(err)
	}
	if len(extracted) != 1 || extracted[0].ScopeType != "session" || extracted[0].ScopeID != alphaSession.ID {
		t.Fatalf("unexpected scoped deterministic extraction: %+v", extracted)
	}
	if _, err := server.tools["agentvault.extract_memory_candidates"].Handler(map[string]interface{}{
		"episode_id": betaEpisode.ID,
	}); !errors.Is(err, authz.ErrForbidden) {
		t.Fatalf("cross-scope deterministic extraction should fail, got %v", err)
	}

	text, err := server.tools["agentvault.propose_memory_candidate"].Handler(map[string]interface{}{
		"episode_id": alphaEpisode.ID, "memory_kind": "fact", "content": "Alpha fact",
		"proposed_by": "spoofed-reviewer",
	})
	if err != nil {
		t.Fatal(err)
	}
	var candidate contract.MemoryCandidate
	if err := json.Unmarshal([]byte(text), &candidate); err != nil {
		t.Fatal(err)
	}
	if candidate.ProposedBy != "extractor" {
		t.Fatalf("scoped proposer identity was not bound: %+v", candidate)
	}

	if _, err := server.tools["agentvault.propose_memory_candidate"].Handler(map[string]interface{}{
		"episode_id": betaEpisode.ID, "memory_kind": "fact", "content": "Beta fact",
	}); !errors.Is(err, authz.ErrForbidden) {
		t.Fatalf("cross-scope candidate proposal should fail, got %v", err)
	}
}

func TestScopedKnowledgeReaderCannotCrossCandidateScope(t *testing.T) {
	_, database, vault := setupKnowledgeMCPServer(t)
	defer database.Close()
	store := knowledge.New(database, vault)
	if err := store.ReplayJournal(); err != nil {
		t.Fatal(err)
	}
	alphaEpisode := seedCandidateEpisode(t, store, "alpha", "")
	betaEpisode := seedCandidateEpisode(t, store, "beta", "")
	alphaCandidate, err := store.ProposeMemoryCandidate(contract.CreateMemoryCandidateRequest{
		EpisodeID: alphaEpisode.ID, MemoryKind: "decision", Content: "Alpha decision",
	})
	if err != nil {
		t.Fatal(err)
	}
	betaCandidate, err := store.ProposeMemoryCandidate(contract.CreateMemoryCandidateRequest{
		EpisodeID: betaEpisode.ID, MemoryKind: "decision", Content: "Beta decision",
	})
	if err != nil {
		t.Fatal(err)
	}

	registry, err := authz.NewRegistry(vault)
	if err != nil {
		t.Fatal(err)
	}
	issued, err := registry.Mint(authz.MintRequest{
		AgentID:      "reader",
		Capabilities: []authz.Capability{authz.KnowledgeRead},
		Scope:        authz.Scope{Projects: []string{"alpha"}},
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

	if _, err := server.tools["agentvault.list_memory_candidates"].Handler(map[string]interface{}{}); !errors.Is(err, authz.ErrForbidden) {
		t.Fatalf("unscoped candidate listing should fail closed, got %v", err)
	}
	if _, err := server.tools["agentvault.get_memory_candidate"].Handler(map[string]interface{}{"id": betaCandidate.ID}); !errors.Is(err, authz.ErrForbidden) {
		t.Fatalf("cross-project candidate read should fail, got %v", err)
	}
	if _, err := server.tools["agentvault.get_memory_candidate"].Handler(map[string]interface{}{"id": alphaCandidate.ID}); err != nil {
		t.Fatalf("authorized candidate read failed: %v", err)
	}
}


func TestRuntimeSurfaceReconcilesMissedSemanticSessionEventPromotion(t *testing.T) {
	server, database, vault := setupKnowledgeMCPServer(t)
	defer database.Close()

	if _, err := database.Exec(`
		INSERT INTO agent_sessions (
			id, agent_id, project, objective, status, context_json, started_at, updated_at
		) VALUES (?, ?, ?, ?, ?, ?, ?, ?)`,
		"session_mcp_reconcile",
		"architect",
		"agentvault",
		"Recover MCP semantic enrichment",
		"active",
		"{}",
		"2026-09-30T15:00:00Z",
		"2026-09-30T15:01:00Z",
	); err != nil {
		t.Fatal(err)
	}
	payload, err := json.Marshal(map[string]interface{}{
		"summary": "MCP startup should repair missed semantic session-event enrichment.",
	})
	if err != nil {
		t.Fatal(err)
	}
	if _, err := database.Exec(`
		INSERT INTO session_events (
			id, session_id, event_type, payload_json, created_at
		) VALUES (?, ?, ?, ?, ?)`,
		"event_mcp_reconcile",
		"session_mcp_reconcile",
		"constraint",
		string(payload),
		"2026-09-30T15:01:00Z",
	); err != nil {
		t.Fatal(err)
	}

	if err := server.RegisterRuntimeSurface(false); err != nil {
		t.Fatal(err)
	}

	store := knowledge.New(database, vault)
	if err := store.ReplayJournal(); err != nil {
		t.Fatal(err)
	}
	candidates, err := store.ListMemoryCandidates(contract.MemoryCandidateFilter{
		Status:    contract.MemoryCandidatePending,
		ScopeType: "session",
		ScopeID:   "session_mcp_reconcile",
		Limit:     20,
	})
	if err != nil {
		t.Fatal(err)
	}
	if len(candidates) != 1 || candidates[0].MemoryKind != "constraint" {
		t.Fatalf("MCP runtime reconciliation did not backfill candidate: %+v", candidates)
	}
}

package knowledge

import (
	"testing"

	"github.com/agentvault/core/internal/contract"
)

func TestPromoteEventCreatesIdempotentProvenanceBackedEpisode(t *testing.T) {
	store, database, _ := setupStore(t)
	defer database.Close()

	input := PromotionInput{
		SourceType:   "capture",
		SourceID:     "capture_2026_09_30_001",
		EvidencePath: "00-inbox/2026-09-30_capture_001.md",
		Project:      "agentvault",
		EventType:    "capture.recorded",
		Summary:      "Captured Browser research",
		OccurredAt:   "2026-09-30T12:00:00Z",
		Metadata: map[string]interface{}{
			"sourceUrl": "https://example.com/research",
		},
	}

	first, err := store.PromoteEvent(input)
	if err != nil {
		t.Fatalf("PromoteEvent first: %v", err)
	}
	second, err := store.PromoteEvent(input)
	if err != nil {
		t.Fatalf("PromoteEvent second: %v", err)
	}

	if first.Provenance.ID == "" || first.Episode.ID == "" {
		t.Fatalf("promotion returned empty ids: %+v", first)
	}
	if second.Provenance.ID != first.Provenance.ID || second.Episode.ID != first.Episode.ID {
		t.Fatalf("promotion is not idempotent: first=%+v second=%+v", first, second)
	}
	if first.Episode.ScopeType != "project" || first.Episode.ScopeID != "agentvault" {
		t.Fatalf("unexpected episode scope: %+v", first.Episode)
	}
	if first.Episode.ProvenanceID != first.Provenance.ID {
		t.Fatalf("episode provenance=%q want %q", first.Episode.ProvenanceID, first.Provenance.ID)
	}
	if first.Provenance.SourceType != "capture" || first.Provenance.SourceID != input.SourceID {
		t.Fatalf("unexpected provenance: %+v", first.Provenance)
	}
	if len(first.Provenance.Evidence) != 1 || first.Provenance.Evidence[0].Path != input.EvidencePath {
		t.Fatalf("unexpected evidence: %+v", first.Provenance.Evidence)
	}

	episodes, err := store.ListEpisodes("project", "agentvault", 10)
	if err != nil {
		t.Fatal(err)
	}
	if len(episodes) != 1 || episodes[0].ID != first.Episode.ID {
		t.Fatalf("expected one promoted episode, got %+v", episodes)
	}
}

func TestPromoteEventUsesSessionAsMostSpecificScope(t *testing.T) {
	store, database, _ := setupStore(t)
	defer database.Close()

	session, err := store.StartSession(contract.StartAgentSessionRequest{
		ID:        "session_promotion",
		AgentID:   "backend-engineer",
		Project:   "agentvault",
		Objective: "Promote durable mutation activity",
	})
	if err != nil {
		t.Fatal(err)
	}

	result, err := store.PromoteEvent(PromotionInput{
		SourceType:   "mutation",
		SourceID:     "mut_123",
		EvidencePath: "10-notes/example.md",
		Project:      "agentvault",
		AgentID:      "backend-engineer",
		SessionID:    session.ID,
		EventType:    "mutation.committed",
		Summary:      "Committed replace mutation for 10-notes/example.md",
	})
	if err != nil {
		t.Fatal(err)
	}
	if result.Episode.ScopeType != "session" || result.Episode.ScopeID != session.ID {
		t.Fatalf("expected session scope, got %+v", result.Episode)
	}
	if result.Provenance.AgentID != "backend-engineer" || result.Provenance.SessionID != session.ID {
		t.Fatalf("promotion lost actor/session provenance: %+v", result.Provenance)
	}
}

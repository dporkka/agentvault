package knowledge

import (
	"sync"
	"testing"

	"github.com/agentvault/core/internal/contract"
)

func seedExtractionEpisode(
	t *testing.T,
	store *Store,
	id, scopeType, scopeID, eventType, summary string,
	metadata map[string]interface{},
	objectIDs ...string,
) contract.EpisodeRecord {
	t.Helper()
	provenance, err := store.CreateProvenance(contract.ProvenanceRecord{
		ID:         "prov_" + id,
		SourceType: "agent-session",
		SourceID:   "source_" + id,
		Confidence: 0.88,
	})
	if err != nil {
		t.Fatal(err)
	}
	episode, err := store.RecordEpisode(contract.CreateEpisodeRequest{
		ID:           id,
		ScopeType:    scopeType,
		ScopeID:      scopeID,
		EventType:    eventType,
		Summary:      summary,
		ObjectIDs:    objectIDs,
		ProvenanceID: provenance.ID,
		Metadata:     metadata,
	})
	if err != nil {
		t.Fatal(err)
	}
	return episode
}

func TestDeterministicExtractionCreatesCandidateOnlyForExplicitSemanticEpisode(t *testing.T) {
	store, database, _ := setupStore(t)
	defer database.Close()

	episode := seedExtractionEpisode(
		t, store,
		"episode_extract_decision", "project", "agentvault",
		"decision.observed",
		"Keep candidate extraction deterministic before adding model assistance.",
		nil,
	)

	candidates, err := store.ExtractMemoryCandidatesFromEpisode(episode.ID)
	if err != nil {
		t.Fatalf("ExtractMemoryCandidatesFromEpisode: %v", err)
	}
	if len(candidates) != 1 {
		t.Fatalf("expected one candidate, got %+v", candidates)
	}
	candidate := candidates[0]
	if candidate.MemoryKind != "decision" || candidate.Content != episode.Summary {
		t.Fatalf("unexpected extracted candidate: %+v", candidate)
	}
	if candidate.ScopeType != episode.ScopeType || candidate.ScopeID != episode.ScopeID {
		t.Fatalf("candidate widened episode scope: %+v", candidate)
	}
	if candidate.ProvenanceID != episode.ProvenanceID || candidate.Confidence != 0.88 {
		t.Fatalf("candidate lost evidence/confidence: %+v", candidate)
	}
	if candidate.ProposedBy != deterministicExtractorID {
		t.Fatalf("candidate proposer=%q want %q", candidate.ProposedBy, deterministicExtractorID)
	}

	retry, err := store.ExtractMemoryCandidatesFromEpisode(episode.ID)
	if err != nil {
		t.Fatal(err)
	}
	if len(retry) != 1 || retry[0].ID != candidate.ID {
		t.Fatalf("episode extraction retry was not idempotent: first=%+v retry=%+v", candidate, retry)
	}
}

func TestDeterministicExtractionIgnoresRawActivityEpisodes(t *testing.T) {
	store, database, _ := setupStore(t)
	defer database.Close()

	for _, eventType := range []string{"capture.recorded", "mutation.committed", "architecture.changed", "tool.result"} {
		t.Run(eventType, func(t *testing.T) {
			episode := seedExtractionEpisode(
				t, store,
				"episode_raw_"+deterministicPromotionID("id", eventType), "project", "agentvault",
				eventType,
				"Raw activity is evidence, not semantic truth.",
				nil,
			)
			candidates, err := store.ExtractMemoryCandidatesFromEpisode(episode.ID)
			if err != nil {
				t.Fatal(err)
			}
			if len(candidates) != 0 {
				t.Fatalf("raw event %q created semantic candidate: %+v", eventType, candidates)
			}
		})
	}
}

func TestDeterministicExtractionSupportsExplicitMetadataOverride(t *testing.T) {
	store, database, _ := setupStore(t)
	defer database.Close()

	object, err := store.UpsertObject(contract.UpsertKnowledgeObjectRequest{
		ID:      "obj_extraction_project",
		Type:    "project",
		Title:   "AgentVault",
		Project: "agentvault",
	})
	if err != nil {
		t.Fatal(err)
	}
	episode := seedExtractionEpisode(
		t, store,
		"episode_metadata_extract", "project", "agentvault",
		"workflow.completed",
		"Workflow completed.",
		map[string]interface{}{
			"memoryKind":     "constraint",
			"memoryContent":  "Semantic promotion must always require explicit review.",
			"memoryObjectId": object.ID,
		},
		object.ID,
	)

	candidates, err := store.ExtractMemoryCandidatesFromEpisode(episode.ID)
	if err != nil {
		t.Fatal(err)
	}
	if len(candidates) != 1 {
		t.Fatalf("expected metadata-directed candidate, got %+v", candidates)
	}
	candidate := candidates[0]
	if candidate.MemoryKind != "constraint" ||
		candidate.Content != "Semantic promotion must always require explicit review." ||
		candidate.ObjectID != object.ID {
		t.Fatalf("metadata override was not preserved: %+v", candidate)
	}
}

func TestDeterministicExtractionDeduplicatesPendingAndAcceptedSemanticMemory(t *testing.T) {
	t.Run("pending", func(t *testing.T) {
		store, database, _ := setupStore(t)
		defer database.Close()

		first := seedExtractionEpisode(t, store, "episode_pending_a", "project", "agentvault", "fact.observed", "  SQLite   is rebuildable. ", nil)
		second := seedExtractionEpisode(t, store, "episode_pending_b", "project", "agentvault", "fact.observed", "sqlite is rebuildable.", nil)

		firstCandidates, err := store.ExtractMemoryCandidatesFromEpisode(first.ID)
		if err != nil {
			t.Fatal(err)
		}
		if len(firstCandidates) != 1 {
			t.Fatalf("expected first candidate: %+v", firstCandidates)
		}
		secondCandidates, err := store.ExtractMemoryCandidatesFromEpisode(second.ID)
		if err != nil {
			t.Fatal(err)
		}
		if len(secondCandidates) != 0 {
			t.Fatalf("equivalent pending candidate was duplicated: %+v", secondCandidates)
		}
	})

	t.Run("accepted memory", func(t *testing.T) {
		store, database, _ := setupStore(t)
		defer database.Close()

		first := seedExtractionEpisode(t, store, "episode_accepted_a", "project", "agentvault", "decision.observed", "Keep Markdown canonical.", nil)
		candidates, err := store.ExtractMemoryCandidatesFromEpisode(first.ID)
		if err != nil || len(candidates) != 1 {
			t.Fatalf("first extraction=%+v err=%v", candidates, err)
		}
		if _, err := store.AcceptMemoryCandidate(candidates[0].ID, contract.ReviewMemoryCandidateRequest{ReviewedBy: "reviewer"}); err != nil {
			t.Fatal(err)
		}

		second := seedExtractionEpisode(t, store, "episode_accepted_b", "project", "agentvault", "decision.observed", " keep markdown canonical. ", nil)
		secondCandidates, err := store.ExtractMemoryCandidatesFromEpisode(second.ID)
		if err != nil {
			t.Fatal(err)
		}
		if len(secondCandidates) != 0 {
			t.Fatalf("accepted semantic memory was proposed again: %+v", secondCandidates)
		}
	})
}

func TestDeterministicExtractionAllowsFreshEvidenceAfterRejection(t *testing.T) {
	store, database, _ := setupStore(t)
	defer database.Close()

	first := seedExtractionEpisode(t, store, "episode_reject_extract_a", "project", "agentvault", "fact.observed", "A claim needs review.", nil)
	candidates, err := store.ExtractMemoryCandidatesFromEpisode(first.ID)
	if err != nil || len(candidates) != 1 {
		t.Fatalf("first extraction=%+v err=%v", candidates, err)
	}
	if _, err := store.RejectMemoryCandidate(candidates[0].ID, contract.ReviewMemoryCandidateRequest{ReviewedBy: "reviewer"}); err != nil {
		t.Fatal(err)
	}

	second := seedExtractionEpisode(t, store, "episode_reject_extract_b", "project", "agentvault", "fact.observed", "A claim needs review.", nil)
	fresh, err := store.ExtractMemoryCandidatesFromEpisode(second.ID)
	if err != nil {
		t.Fatal(err)
	}
	if len(fresh) != 1 || fresh[0].ID == candidates[0].ID {
		t.Fatalf("new evidence after rejection should create a fresh candidate: %+v", fresh)
	}
}

func TestDeterministicExtractionSerializesEquivalentConcurrentEpisodes(t *testing.T) {
	store, database, _ := setupStore(t)
	defer database.Close()

	episodes := make([]contract.EpisodeRecord, 12)
	for i := range episodes {
		episodes[i] = seedExtractionEpisode(
			t, store,
			deterministicPromotionID("episode_concurrent_extract", string(rune('a'+i))),
			"project", "agentvault", "constraint.observed",
			"Durable semantic memory requires review.",
			nil,
		)
	}

	var wg sync.WaitGroup
	errs := make(chan error, len(episodes))
	for _, episode := range episodes {
		episode := episode
		wg.Add(1)
		go func() {
			defer wg.Done()
			_, err := store.ExtractMemoryCandidatesFromEpisode(episode.ID)
			errs <- err
		}()
	}
	wg.Wait()
	close(errs)
	for err := range errs {
		if err != nil {
			t.Fatalf("concurrent extraction failed: %v", err)
		}
	}

	pending, err := store.ListMemoryCandidates(contract.MemoryCandidateFilter{
		Status:    contract.MemoryCandidatePending,
		ScopeType: "project",
		ScopeID:   "agentvault",
		Limit:     100,
	})
	if err != nil {
		t.Fatal(err)
	}
	if len(pending) != 1 {
		t.Fatalf("concurrent equivalent episodes created duplicate candidates: %+v", pending)
	}
}

func TestPromoteEventRunsDeterministicExtractionWithoutPromotingRawActivity(t *testing.T) {
	store, database, _ := setupStore(t)
	defer database.Close()

	semantic, err := store.PromoteEvent(PromotionInput{
		SourceType: "session-event",
		SourceID:   "semantic-event-1",
		Project:    "agentvault",
		EventType:  "decision.observed",
		Summary:    "Use reviewable candidate extraction.",
	})
	if err != nil {
		t.Fatal(err)
	}
	if len(semantic.Candidates) != 1 || semantic.Candidates[0].MemoryKind != "decision" {
		t.Fatalf("semantic promotion did not extract candidate: %+v", semantic)
	}

	raw, err := store.PromoteEvent(PromotionInput{
		SourceType: "capture",
		SourceID:   "raw-event-1",
		Project:    "agentvault",
		EventType:  "capture.recorded",
		Summary:    "Captured raw research.",
	})
	if err != nil {
		t.Fatal(err)
	}
	if len(raw.Candidates) != 0 {
		t.Fatalf("raw activity was promoted into semantic candidate: %+v", raw.Candidates)
	}
}

func TestAppendSemanticSessionEventPromotesEpisodeAndCandidate(t *testing.T) {
	store, database, _ := setupStore(t)
	defer database.Close()

	session, err := store.StartSession(contract.StartAgentSessionRequest{
		ID:        "session_semantic_event",
		AgentID:   "architect",
		Project:   "agentvault",
		Objective: "Record explicit semantic events",
	})
	if err != nil {
		t.Fatal(err)
	}
	event, err := store.AppendSessionEvent(session.ID, contract.AppendSessionEventRequest{
		ID:        "event_semantic_decision",
		EventType: "decision",
		Payload: map[string]interface{}{
			"summary": "Use deterministic extraction before model-assisted extraction.",
		},
	})
	if err != nil {
		t.Fatal(err)
	}

	episodeID := deterministicPromotionID("episode", "session-event", event.ID, event.EventType)
	episode, err := store.GetEpisode(episodeID)
	if err != nil {
		t.Fatalf("semantic session event was not promoted: %v", err)
	}
	if episode.ScopeType != "session" || episode.ScopeID != session.ID || episode.Summary != "Use deterministic extraction before model-assisted extraction." {
		t.Fatalf("unexpected promoted session episode: %+v", episode)
	}

	candidates, err := store.ListMemoryCandidates(contract.MemoryCandidateFilter{
		Status:    contract.MemoryCandidatePending,
		ScopeType: "session",
		ScopeID:   session.ID,
		Limit:     20,
	})
	if err != nil {
		t.Fatal(err)
	}
	if len(candidates) != 1 || candidates[0].MemoryKind != "decision" || candidates[0].SourceEpisodeID != episode.ID {
		t.Fatalf("semantic session event did not create review candidate: %+v", candidates)
	}
}

func TestAppendRawSessionEventDoesNotPromoteSemanticMemory(t *testing.T) {
	store, database, _ := setupStore(t)
	defer database.Close()

	session, err := store.StartSession(contract.StartAgentSessionRequest{
		ID:        "session_raw_event",
		AgentID:   "worker",
		Project:   "agentvault",
		Objective: "Record raw execution events",
	})
	if err != nil {
		t.Fatal(err)
	}
	event, err := store.AppendSessionEvent(session.ID, contract.AppendSessionEventRequest{
		ID:        "event_raw_verification",
		EventType: "verification",
		Payload: map[string]interface{}{
			"summary": "All tests passed.",
		},
	})
	if err != nil {
		t.Fatal(err)
	}

	episodeID := deterministicPromotionID("episode", "session-event", event.ID, event.EventType)
	if _, err := store.GetEpisode(episodeID); err == nil {
		t.Fatal("raw session event unexpectedly became a semantic episode")
	}
	candidates, err := store.ListMemoryCandidates(contract.MemoryCandidateFilter{
		Status:    contract.MemoryCandidatePending,
		ScopeType: "session",
		ScopeID:   session.ID,
		Limit:     20,
	})
	if err != nil {
		t.Fatal(err)
	}
	if len(candidates) != 0 {
		t.Fatalf("raw session event created semantic candidates: %+v", candidates)
	}
}

func TestReconcileSemanticSessionEventsBackfillsMissedPromotion(t *testing.T) {
	store, database, _ := setupStore(t)
	defer database.Close()

	session, err := store.StartSession(contract.StartAgentSessionRequest{
		ID:        "session_reconcile_semantic",
		AgentID:   "architect",
		Project:   "agentvault",
		Objective: "Recover semantic event promotion",
	})
	if err != nil {
		t.Fatal(err)
	}

	// Persist the canonical session event directly to simulate a crash after the
	// event commit but before enrichment/promotion ran.
	event := contract.SessionEvent{
		ID:        "event_reconcile_decision",
		SessionID: session.ID,
		EventType: "decision.observed",
		Payload: map[string]interface{}{
			"summary": "Recover review candidates from durable session history.",
		},
		CreatedAt: "2026-09-30T15:00:00Z",
	}
	if err := store.persist(eventSessionEvent, event, func() error {
		return store.projectSessionEvent(event)
	}); err != nil {
		t.Fatal(err)
	}

	if _, err := store.GetEpisode(deterministicPromotionID("episode", "session-event", event.ID, event.EventType)); err == nil {
		t.Fatal("test setup unexpectedly promoted the session event")
	}

	if err := store.ReconcileSemanticSessionEvents(100); err != nil {
		t.Fatal(err)
	}
	if err := store.ReconcileSemanticSessionEvents(100); err != nil {
		t.Fatalf("reconciliation retry must be idempotent: %v", err)
	}

	episode, err := store.GetEpisode(deterministicPromotionID("episode", "session-event", event.ID, event.EventType))
	if err != nil {
		t.Fatalf("reconciliation did not restore promoted episode: %v", err)
	}
	candidates, err := store.ListMemoryCandidates(contract.MemoryCandidateFilter{
		Status:    contract.MemoryCandidatePending,
		ScopeType: "session",
		ScopeID:   session.ID,
		Limit:     20,
	})
	if err != nil {
		t.Fatal(err)
	}
	if len(candidates) != 1 || candidates[0].SourceEpisodeID != episode.ID {
		t.Fatalf("reconciliation did not restore candidate exactly once: %+v", candidates)
	}
}

package knowledge

import (
	"os"
	"path/filepath"
	"testing"

	"github.com/agentvault/core/internal/contract"
	"github.com/agentvault/core/internal/db"
)

func createCandidateEpisode(t *testing.T, store *Store, scopeType, scopeID string) contract.EpisodeRecord {
	t.Helper()
	provenance, err := store.CreateProvenance(contract.ProvenanceRecord{
		ID:         "prov_candidate_" + scopeID,
		SourceType: "agent-session",
		SourceID:   "source_" + scopeID,
		Confidence: 0.94,
		Evidence: []contract.ProvenanceEvidence{{
			Source: "file",
			Path:   "notes/source.md",
			Quote:  "Use explicit review before durable memory.",
		}},
	})
	if err != nil {
		t.Fatal(err)
	}
	episode, err := store.RecordEpisode(contract.CreateEpisodeRequest{
		ID:           "episode_candidate_" + scopeID,
		ScopeType:    scopeType,
		ScopeID:      scopeID,
		EventType:    "decision.observed",
		Summary:      "Agent observed a decision worth remembering",
		ProvenanceID: provenance.ID,
	})
	if err != nil {
		t.Fatal(err)
	}
	return episode
}

func TestMemoryCandidateLifecycleRequiresExplicitReview(t *testing.T) {
	store, database, _ := setupStore(t)
	defer database.Close()

	episode := createCandidateEpisode(t, store, "project", "agentvault")
	confidence := 0.87
	candidate, err := store.ProposeMemoryCandidate(contract.CreateMemoryCandidateRequest{
		EpisodeID:   episode.ID,
		MemoryKind:  "decision",
		Content:     "Keep automatic extraction separate from durable semantic memory.",
		Confidence:  &confidence,
		ProposedBy:  "extractor-agent",
		Metadata:    map[string]interface{}{"extractor": "deterministic-test"},
	})
	if err != nil {
		t.Fatalf("ProposeMemoryCandidate: %v", err)
	}
	if candidate.Status != contract.MemoryCandidatePending {
		t.Fatalf("candidate status=%s want pending", candidate.Status)
	}
	if candidate.ScopeType != episode.ScopeType || candidate.ScopeID != episode.ScopeID {
		t.Fatalf("candidate did not inherit episode scope: %+v", candidate)
	}
	if candidate.ProvenanceID != episode.ProvenanceID {
		t.Fatalf("candidate provenance=%q want %q", candidate.ProvenanceID, episode.ProvenanceID)
	}

	memories, err := store.ListMemories("project", "agentvault", "semantic", 10)
	if err != nil {
		t.Fatal(err)
	}
	if len(memories) != 0 {
		t.Fatalf("pending candidate leaked into durable memory: %+v", memories)
	}

	accepted, err := store.AcceptMemoryCandidate(candidate.ID, contract.ReviewMemoryCandidateRequest{
		ReviewedBy: "david",
		Reason:     "Evidence supports the decision.",
	})
	if err != nil {
		t.Fatalf("AcceptMemoryCandidate: %v", err)
	}
	if accepted.Status != contract.MemoryCandidateAccepted || accepted.ResultMemoryID == "" {
		t.Fatalf("unexpected accepted candidate: %+v", accepted)
	}

	memory, err := store.GetMemory(accepted.ResultMemoryID)
	if err != nil {
		t.Fatal(err)
	}
	if memory.MemoryClass != "semantic" || memory.MemoryKind != "decision" {
		t.Fatalf("unexpected materialized memory: %+v", memory)
	}
	if memory.Content != candidate.Content || memory.ProvenanceID != episode.ProvenanceID {
		t.Fatalf("materialized memory lost candidate evidence: %+v", memory)
	}

	again, err := store.AcceptMemoryCandidate(candidate.ID, contract.ReviewMemoryCandidateRequest{
		ReviewedBy: "david",
		Reason:     "retry",
	})
	if err != nil {
		t.Fatalf("idempotent accept retry: %v", err)
	}
	if again.ResultMemoryID != accepted.ResultMemoryID {
		t.Fatalf("accept retry changed result memory: first=%+v second=%+v", accepted, again)
	}

	memories, err = store.ListMemories("project", "agentvault", "semantic", 10)
	if err != nil {
		t.Fatal(err)
	}
	if len(memories) != 1 {
		t.Fatalf("accept retry duplicated durable memory: %+v", memories)
	}
}

func TestMemoryCandidateRejectCreatesNoMemory(t *testing.T) {
	store, database, _ := setupStore(t)
	defer database.Close()

	episode := createCandidateEpisode(t, store, "project", "reject-project")
	candidate, err := store.ProposeMemoryCandidate(contract.CreateMemoryCandidateRequest{
		EpisodeID:  episode.ID,
		MemoryKind: "fact",
		Content:    "This candidate should remain review history only.",
	})
	if err != nil {
		t.Fatal(err)
	}

	rejected, err := store.RejectMemoryCandidate(candidate.ID, contract.ReviewMemoryCandidateRequest{
		ReviewedBy: "reviewer",
		Reason:     "Insufficient support.",
	})
	if err != nil {
		t.Fatal(err)
	}
	if rejected.Status != contract.MemoryCandidateRejected || rejected.ResultMemoryID != "" {
		t.Fatalf("unexpected rejected candidate: %+v", rejected)
	}

	memories, err := store.ListMemories("project", "reject-project", "semantic", 10)
	if err != nil {
		t.Fatal(err)
	}
	if len(memories) != 0 {
		t.Fatalf("rejected candidate created durable memory: %+v", memories)
	}
}

func TestMemoryCandidateSupersedeAndMergeAreExplicit(t *testing.T) {
	t.Run("supersede", func(t *testing.T) {
		store, database, _ := setupStore(t)
		defer database.Close()

		episode := createCandidateEpisode(t, store, "project", "supersede-project")
		old, err := store.RecordMemory(contract.CreateMemoryRequest{
			ID:          "mem_old",
			MemoryClass: "semantic",
			MemoryKind:  "fact",
			ScopeType:   "project",
			ScopeID:     "supersede-project",
			Content:     "The old fact.",
		})
		if err != nil {
			t.Fatal(err)
		}
		candidate, err := store.ProposeMemoryCandidate(contract.CreateMemoryCandidateRequest{
			EpisodeID:  episode.ID,
			MemoryKind: "fact",
			Content:    "The corrected fact.",
		})
		if err != nil {
			t.Fatal(err)
		}

		resolved, err := store.SupersedeMemoryWithCandidate(candidate.ID, contract.SupersedeMemoryCandidateRequest{
			ReviewedBy:     "reviewer",
			TargetMemoryID: old.ID,
			Reason:         "New evidence corrects the old fact.",
		})
		if err != nil {
			t.Fatal(err)
		}
		if resolved.Status != contract.MemoryCandidateSuperseded || resolved.TargetMemoryID != old.ID {
			t.Fatalf("unexpected supersede resolution: %+v", resolved)
		}
		newMemory, err := store.GetMemory(resolved.ResultMemoryID)
		if err != nil {
			t.Fatal(err)
		}
		if newMemory.SupersedesID != old.ID || newMemory.Content != candidate.Content {
			t.Fatalf("supersede did not materialize replacement: %+v", newMemory)
		}
	})

	t.Run("merge", func(t *testing.T) {
		store, database, _ := setupStore(t)
		defer database.Close()

		episode := createCandidateEpisode(t, store, "project", "merge-project")
		old, err := store.RecordMemory(contract.CreateMemoryRequest{
			ID:          "mem_merge_old",
			MemoryClass: "semantic",
			MemoryKind:  "preference",
			ScopeType:   "project",
			ScopeID:     "merge-project",
			Content:     "Prefer local-first storage.",
		})
		if err != nil {
			t.Fatal(err)
		}
		candidate, err := store.ProposeMemoryCandidate(contract.CreateMemoryCandidateRequest{
			EpisodeID:  episode.ID,
			MemoryKind: "preference",
			Content:    "Prefer auditable storage.",
		})
		if err != nil {
			t.Fatal(err)
		}

		resolved, err := store.MergeMemoryCandidate(candidate.ID, contract.MergeMemoryCandidateRequest{
			ReviewedBy:     "reviewer",
			TargetMemoryID: old.ID,
			MergedContent:  "Prefer local-first, auditable storage.",
			Reason:         "Both observations describe the same preference.",
		})
		if err != nil {
			t.Fatal(err)
		}
		if resolved.Status != contract.MemoryCandidateMerged || resolved.TargetMemoryID != old.ID {
			t.Fatalf("unexpected merge resolution: %+v", resolved)
		}
		newMemory, err := store.GetMemory(resolved.ResultMemoryID)
		if err != nil {
			t.Fatal(err)
		}
		if newMemory.SupersedesID != old.ID || newMemory.Content != "Prefer local-first, auditable storage." {
			t.Fatalf("merge did not materialize reviewed content: %+v", newMemory)
		}
	})
}

func TestMemoryCandidateRejectsUnprovenancedEpisodeAndConflictingReview(t *testing.T) {
	store, database, _ := setupStore(t)
	defer database.Close()

	episode, err := store.RecordEpisode(contract.CreateEpisodeRequest{
		ID:        "episode_unprovenanced",
		ScopeType: "project",
		ScopeID:   "agentvault",
		EventType: "observation",
		Summary:   "No provenance",
	})
	if err != nil {
		t.Fatal(err)
	}
	if _, err := store.ProposeMemoryCandidate(contract.CreateMemoryCandidateRequest{
		EpisodeID:  episode.ID,
		MemoryKind: "fact",
		Content:    "Should fail.",
	}); err == nil {
		t.Fatal("expected candidate from unprovenanced episode to fail")
	}

	provenanced := createCandidateEpisode(t, store, "project", "terminal-project")
	candidate, err := store.ProposeMemoryCandidate(contract.CreateMemoryCandidateRequest{
		EpisodeID:  provenanced.ID,
		MemoryKind: "decision",
		Content:    "Terminal review is immutable.",
	})
	if err != nil {
		t.Fatal(err)
	}
	if _, err := store.RejectMemoryCandidate(candidate.ID, contract.ReviewMemoryCandidateRequest{ReviewedBy: "reviewer"}); err != nil {
		t.Fatal(err)
	}
	if _, err := store.AcceptMemoryCandidate(candidate.ID, contract.ReviewMemoryCandidateRequest{ReviewedBy: "reviewer"}); err == nil {
		t.Fatal("expected conflicting terminal review to fail")
	}
}


func TestMemoryCandidateReplayRestoresReviewAndResultMemory(t *testing.T) {
	store, database, vault := setupStore(t)
	episode := createCandidateEpisode(t, store, "project", "replay-project")
	candidate, err := store.ProposeMemoryCandidate(contract.CreateMemoryCandidateRequest{
		ID:         "candidate_replay",
		EpisodeID:  episode.ID,
		MemoryKind: "decision",
		Content:    "Reviewed candidate state is canonical journal history.",
	})
	if err != nil {
		t.Fatal(err)
	}
	accepted, err := store.AcceptMemoryCandidate(candidate.ID, contract.ReviewMemoryCandidateRequest{
		ReviewedBy: "reviewer",
		Reason:     "Verified from source evidence.",
	})
	if err != nil {
		t.Fatal(err)
	}
	if err := database.Close(); err != nil {
		t.Fatal(err)
	}
	for _, suffix := range []string{"", "-wal", "-shm"} {
		path := filepath.Join(vault, ".agentvault", "agentvault.db") + suffix
		if err := os.Remove(path); err != nil && !os.IsNotExist(err) {
			t.Fatal(err)
		}
	}

	rebuiltDB, err := db.Open(vault)
	if err != nil {
		t.Fatal(err)
	}
	defer rebuiltDB.Close()
	if err := rebuiltDB.RunMigrations(); err != nil {
		t.Fatal(err)
	}
	rebuilt := New(rebuiltDB, vault)
	if err := rebuilt.ReplayJournal(); err != nil {
		t.Fatal(err)
	}
	if err := rebuilt.ReplayJournal(); err != nil {
		t.Fatalf("second replay must be idempotent: %v", err)
	}

	restored, err := rebuilt.GetMemoryCandidate(candidate.ID)
	if err != nil {
		t.Fatal(err)
	}
	if restored.Status != contract.MemoryCandidateAccepted ||
		restored.ResultMemoryID != accepted.ResultMemoryID ||
		restored.ReviewedBy != "reviewer" {
		t.Fatalf("candidate review did not replay exactly: %+v", restored)
	}
	memory, err := rebuilt.GetMemory(restored.ResultMemoryID)
	if err != nil {
		t.Fatal(err)
	}
	if memory.Content != candidate.Content || memory.Metadata["candidateId"] != candidate.ID {
		t.Fatalf("candidate result memory did not replay exactly: %+v", memory)
	}
}

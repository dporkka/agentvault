package knowledge

import (
	"testing"

	"github.com/agentvault/core/internal/contract"
)

func TestTemporalFactSupersessionAndEpisodeReplay(t *testing.T) {
	store, database, _ := setupStore(t)
	defer database.Close()

	provenance, err := store.CreateProvenance(contract.ProvenanceRecord{
		ID:         "prov_temporal",
		SourceType: "agent-session",
		SourceID:   "session-temporal",
		AgentID:    "architect",
		Confidence: 0.96,
		ObservedAt: "2026-09-20T15:00:00Z",
	})
	if err != nil {
		t.Fatal(err)
	}
	subject, err := store.UpsertObject(contract.UpsertKnowledgeObjectRequest{
		ID:           "obj_temporal_project",
		Type:         "project",
		Title:        "AgentVault",
		Project:      "agentvault",
		ProvenanceID: provenance.ID,
	})
	if err != nil {
		t.Fatal(err)
	}

	episode, err := store.RecordEpisode(contract.CreateEpisodeRequest{
		ID:           "episode_temporal",
		ScopeType:    "project",
		ScopeID:      "agentvault",
		EventType:    "architecture.changed",
		Summary:      "The storage architecture moved to the unified memory core.",
		ObjectIDs:    []string{subject.ID, subject.ID, ""},
		ProvenanceID: provenance.ID,
		OccurredAt:   "2026-09-19T12:00:00Z",
	})
	if err != nil {
		t.Fatalf("RecordEpisode: %v", err)
	}
	if len(episode.ObjectIDs) != 1 || episode.ObjectIDs[0] != subject.ID {
		t.Fatalf("episode object IDs not normalized: %+v", episode.ObjectIDs)
	}

	oldFact, err := store.RecordFact(contract.CreateTemporalFactRequest{
		ID:           "fact_storage_old",
		SubjectID:    subject.ID,
		Predicate:    "storage.engine",
		Value:        "SQLite-only",
		ProvenanceID: provenance.ID,
		ValidFrom:    "2026-01-01",
	})
	if err != nil {
		t.Fatalf("RecordFact(old): %v", err)
	}
	newFact, err := store.RecordFact(contract.CreateTemporalFactRequest{
		ID:           "fact_storage_new",
		SubjectID:    subject.ID,
		Predicate:    "storage.engine",
		Value:        "Markdown/JSONL canonical with SQLite projection",
		ProvenanceID: provenance.ID,
		ValidFrom:    "2026-09-19",
		SupersedesID: oldFact.ID,
	})
	if err != nil {
		t.Fatalf("RecordFact(new): %v", err)
	}

	loadedOld, err := store.GetFact(oldFact.ID)
	if err != nil {
		t.Fatal(err)
	}
	if loadedOld.SupersededBy != newFact.ID || loadedOld.SupersededAt == "" {
		t.Fatalf("old fact missing derived supersession state: %+v", loadedOld)
	}

	if _, err := store.RecordFact(contract.CreateTemporalFactRequest{
		SubjectID:    subject.ID,
		Predicate:    "different.predicate",
		Value:        "invalid",
		ProvenanceID: provenance.ID,
		SupersedesID: newFact.ID,
	}); err == nil {
		t.Fatal("expected unrelated fact supersession to fail")
	}

	// Replaying the journal onto an already-populated projection must be safe.
	if err := store.ReplayJournal(); err != nil {
		t.Fatalf("ReplayJournal: %v", err)
	}
	loadedOld, err = store.GetFact(oldFact.ID)
	if err != nil {
		t.Fatal(err)
	}
	if loadedOld.SupersededBy != newFact.ID {
		t.Fatalf("replay changed supersession: %+v", loadedOld)
	}

	episodes, err := store.ListEpisodes("project", "agentvault", 10)
	if err != nil {
		t.Fatal(err)
	}
	if len(episodes) != 1 || episodes[0].ID != episode.ID {
		t.Fatalf("unexpected episodes after replay: %+v", episodes)
	}
}

package contextcompiler

import (
	"testing"
	"time"

	"github.com/agentvault/core/internal/contract"
)

func TestFactValidAtHonorsSupersessionTimeline(t *testing.T) {
	fact := contract.TemporalFact{
		ID:           "fact_old",
		SubjectID:    "obj_project",
		Predicate:    "deployment.target",
		Value:        "old",
		CreatedAt:    "2026-01-01T00:00:00Z",
		SupersededAt: "2026-06-01T00:00:00Z",
		SupersededBy: "fact_new",
	}
	before, _ := time.Parse(time.RFC3339, "2026-05-01T00:00:00Z")
	after, _ := time.Parse(time.RFC3339, "2026-07-01T00:00:00Z")
	if !factValidAt(fact, before) {
		t.Fatal("fact should be visible before supersession")
	}
	if factValidAt(fact, after) {
		t.Fatal("fact should be hidden after supersession")
	}
}

func TestCompileIncludesCurrentTemporalKnowledgeWithoutProjectLeak(t *testing.T) {
	compiler, store, database, _ := setupCompiler(t)
	defer database.Close()

	provenance, err := store.CreateProvenance(contract.ProvenanceRecord{
		ID:         "prov_temporal_context",
		SourceType: "agent-session",
		Confidence: 0.98,
		ObservedAt: "2026-09-20T12:00:00Z",
	})
	if err != nil {
		t.Fatal(err)
	}
	project, err := store.UpsertObject(contract.UpsertKnowledgeObjectRequest{
		ID:           "obj_agentvault_temporal",
		Type:         "project",
		Title:        "AgentVault",
		Project:      "agentvault",
		ProvenanceID: provenance.ID,
	})
	if err != nil {
		t.Fatal(err)
	}
	other, err := store.UpsertObject(contract.UpsertKnowledgeObjectRequest{
		ID:           "obj_hidden_project",
		Type:         "project",
		Title:        "Hidden",
		Project:      "other-project",
		ProvenanceID: provenance.ID,
	})
	if err != nil {
		t.Fatal(err)
	}

	oldFact, err := store.RecordFact(contract.CreateTemporalFactRequest{
		ID:           "fact_context_old",
		SubjectID:    project.ID,
		Predicate:    "deployment.target",
		Value:        "old-host",
		ProvenanceID: provenance.ID,
	})
	if err != nil {
		t.Fatal(err)
	}
	currentFact, err := store.RecordFact(contract.CreateTemporalFactRequest{
		ID:           "fact_context_current",
		SubjectID:    project.ID,
		Predicate:    "deployment.target",
		Value:        "cloud-run",
		ProvenanceID: provenance.ID,
		SupersedesID: oldFact.ID,
	})
	if err != nil {
		t.Fatal(err)
	}
	hiddenFact, err := store.RecordFact(contract.CreateTemporalFactRequest{
		ID:           "fact_context_hidden",
		SubjectID:    other.ID,
		Predicate:    "deployment.target",
		Value:        "hidden-host",
		ProvenanceID: provenance.ID,
	})
	if err != nil {
		t.Fatal(err)
	}
	episode, err := store.RecordEpisode(contract.CreateEpisodeRequest{
		ID:           "episode_context",
		ScopeType:    "project",
		ScopeID:      "agentvault",
		EventType:    "deployment.changed",
		Summary:      "Deployment moved to Cloud Run.",
		ObjectIDs:    []string{project.ID},
		ProvenanceID: provenance.ID,
		OccurredAt:   "2026-09-20T10:00:00Z",
	})
	if err != nil {
		t.Fatal(err)
	}

	bundle, err := compiler.Compile(contract.CompileContextRequest{
		Task:        "deployment target",
		Project:     "agentvault",
		TokenBudget: 4000,
		MaxItems:    30,
		AsOf:        "2027-01-01T00:00:00Z",
	})
	if err != nil {
		t.Fatal(err)
	}
	foundCurrent := false
	foundEpisode := false
	for _, item := range bundle.Items {
		switch item.ID {
		case oldFact.ID:
			t.Fatalf("superseded fact leaked into current context: %+v", item)
		case hiddenFact.ID:
			t.Fatalf("cross-project fact leaked into context: %+v", item)
		case currentFact.ID:
			if item.Kind != "fact" {
				t.Fatalf("current fact kind = %q", item.Kind)
			}
			foundCurrent = true
		case episode.ID:
			if item.Kind != "episode" {
				t.Fatalf("episode kind = %q", item.Kind)
			}
			foundEpisode = true
		}
	}
	if !foundCurrent {
		t.Fatal("current temporal fact missing from context")
	}
	if !foundEpisode {
		t.Fatal("project episode missing from context")
	}
}

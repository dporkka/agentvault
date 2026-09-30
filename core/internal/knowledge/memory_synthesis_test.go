package knowledge

import (
	"os"
	"path/filepath"
	"testing"

	"github.com/agentvault/core/internal/contract"
	"github.com/agentvault/core/internal/db"
)

func TestSynthesizeMemoryCreatesHigherOrderMemoryWithLineage(t *testing.T) {
	store, database, _ := setupStore(t)
	defer database.Close()

	first, err := store.RecordMemory(contract.CreateMemoryRequest{
		ID:          "mem_model_source_1",
		MemoryClass: "semantic",
		MemoryKind:  "fact",
		ScopeType:   "project",
		ScopeID:     "alpha",
		Content:     "Deployments require a successful smoke test.",
	})
	if err != nil {
		t.Fatal(err)
	}
	second, err := store.RecordMemory(contract.CreateMemoryRequest{
		ID:          "mem_model_source_2",
		MemoryClass: "semantic",
		MemoryKind:  "decision",
		ScopeType:   "project",
		ScopeID:     "alpha",
		Content:     "Production releases use staged rollout.",
	})
	if err != nil {
		t.Fatal(err)
	}

	synthesis, err := store.SynthesizeMemory(contract.CreateMemorySynthesisRequest{
		ID:              "synth_release_model",
		Kind:            contract.MemorySynthesisModel,
		SourceMemoryIDs: []string{first.ID, second.ID},
		Content:         "Safe releases combine smoke-test evidence with staged rollout.",
		CreatedBy:       "reflection-agent",
		Rationale:       "Two accepted memories describe the same release-safety model.",
	})
	if err != nil {
		t.Fatalf("SynthesizeMemory() error: %v", err)
	}
	if synthesis.ScopeType != "project" || synthesis.ScopeID != "alpha" {
		t.Fatalf("unexpected synthesis scope: %+v", synthesis)
	}
	if synthesis.TargetMemoryID == "" {
		t.Fatal("expected target memory id")
	}

	target, err := store.GetMemory(synthesis.TargetMemoryID)
	if err != nil {
		t.Fatal(err)
	}
	if target.MemoryClass != "semantic" || target.MemoryKind != "summary" {
		t.Fatalf("unexpected target memory classification: %+v", target)
	}
	if target.Metadata["synthesisKind"] != "model" {\n\t\tt.Fatalf("target memory missing model classification: %+v", target.Metadata)\n\t}\n\tif target.Metadata["synthesisId"] != synthesis.ID {
		t.Fatalf("target memory missing synthesis lineage: %+v", target.Metadata)
	}

	got, err := store.GetMemorySynthesis(synthesis.ID)
	if err != nil {
		t.Fatal(err)
	}
	if len(got.SourceMemoryIDs) != 2 || got.SourceMemoryIDs[0] != first.ID || got.SourceMemoryIDs[1] != second.ID {
		t.Fatalf("unexpected source lineage: %+v", got.SourceMemoryIDs)
	}
	if got.TargetMemoryID != target.ID {
		t.Fatalf("target lineage mismatch: got %q want %q", got.TargetMemoryID, target.ID)
	}
}

func TestSynthesizeMemoryMapsPolicyAndSkillToProceduralMemory(t *testing.T) {
	store, database, _ := setupStore(t)
	defer database.Close()

	source, err := store.RecordMemory(contract.CreateMemoryRequest{
		ID:          "mem_procedure_source",
		MemoryClass: "semantic",
		MemoryKind:  "constraint",
		ScopeType:   "agent",
		ScopeID:     "release-agent",
		Content:     "Never deploy while merge-critical checks are red.",
	})
	if err != nil {
		t.Fatal(err)
	}

	for _, tc := range []struct {
		name string
		kind contract.MemorySynthesisKind
		want string
	}{
		{name: "policy", kind: contract.MemorySynthesisPolicy, want: "policy"},
		{name: "skill", kind: contract.MemorySynthesisSkill, want: "skill"},
	} {
		t.Run(tc.name, func(t *testing.T) {
			result, err := store.SynthesizeMemory(contract.CreateMemorySynthesisRequest{
				Kind:            tc.kind,
				SourceMemoryIDs: []string{source.ID},
				Content:         "Verify merge-critical checks before deploying.",
				CreatedBy:       "reflection-agent",
			})
			if err != nil {
				t.Fatal(err)
			}
			target, err := store.GetMemory(result.TargetMemoryID)
			if err != nil {
				t.Fatal(err)
			}
			if target.MemoryClass != "procedural" || target.Metadata["synthesisKind"] != tc.want {
				t.Fatalf("unexpected target memory: %+v", target)
			}
		})
	}
}

func TestSynthesizeMemoryRejectsCrossScopeSources(t *testing.T) {
	store, database, _ := setupStore(t)
	defer database.Close()

	alpha, err := store.RecordMemory(contract.CreateMemoryRequest{
		ID:          "mem_alpha_source",
		MemoryClass: "semantic",
		MemoryKind:  "fact",
		ScopeType:   "project",
		ScopeID:     "alpha",
		Content:     "Alpha fact.",
	})
	if err != nil {
		t.Fatal(err)
	}
	beta, err := store.RecordMemory(contract.CreateMemoryRequest{
		ID:          "mem_beta_source",
		MemoryClass: "semantic",
		MemoryKind:  "fact",
		ScopeType:   "project",
		ScopeID:     "beta",
		Content:     "Beta fact.",
	})
	if err != nil {
		t.Fatal(err)
	}

	if _, err := store.SynthesizeMemory(contract.CreateMemorySynthesisRequest{
		Kind:            contract.MemorySynthesisSummary,
		SourceMemoryIDs: []string{alpha.ID, beta.ID},
		Content:         "This must not cross project boundaries.",
	}); err == nil {
		t.Fatal("expected cross-scope synthesis to fail")
	}
}

func TestSynthesizeMemoryIsIdempotentForEquivalentRequest(t *testing.T) {
	store, database, _ := setupStore(t)
	defer database.Close()

	source, err := store.RecordMemory(contract.CreateMemoryRequest{
		ID:          "mem_idempotent_source",
		MemoryClass: "semantic",
		MemoryKind:  "fact",
		ScopeType:   "project",
		ScopeID:     "alpha",
		Content:     "A stable source fact.",
	})
	if err != nil {
		t.Fatal(err)
	}

	req := contract.CreateMemorySynthesisRequest{
		Kind:            contract.MemorySynthesisModel,
		SourceMemoryIDs: []string{source.ID},
		Content:         "A stable derived model.",
		CreatedBy:       "reflection-agent",
	}
	first, err := store.SynthesizeMemory(req)
	if err != nil {
		t.Fatal(err)
	}
	second, err := store.SynthesizeMemory(req)
	if err != nil {
		t.Fatal(err)
	}
	if first.ID != second.ID || first.TargetMemoryID != second.TargetMemoryID {
		t.Fatalf("equivalent synthesis duplicated output: first=%+v second=%+v", first, second)
	}
}


func TestSynthesizeMemoryRejectsSupersededSource(t *testing.T) {
	store, database, _ := setupStore(t)
	defer database.Close()

	old, err := store.RecordMemory(contract.CreateMemoryRequest{
		ID:          "mem_stale_source",
		MemoryClass: "semantic",
		MemoryKind:  "fact",
		ScopeType:   "project",
		ScopeID:     "alpha",
		Content:     "Old release rule.",
	})
	if err != nil {
		t.Fatal(err)
	}
	_, err = store.RecordMemory(contract.CreateMemoryRequest{
		ID:           "mem_current_source",
		MemoryClass:  "semantic",
		MemoryKind:   "fact",
		ScopeType:    "project",
		ScopeID:      "alpha",
		Content:      "Current release rule.",
		SupersedesID: old.ID,
	})
	if err != nil {
		t.Fatal(err)
	}

	if _, err := store.SynthesizeMemory(contract.CreateMemorySynthesisRequest{
		Kind:            contract.MemorySynthesisModel,
		SourceMemoryIDs: []string{old.ID},
		Content:         "A model derived from stale knowledge.",
	}); err == nil {
		t.Fatal("expected superseded source memory to be rejected")
	}
}

func TestMemorySynthesisReplaysLineageAndTargetMemory(t *testing.T) {
	store, database, vault := setupStore(t)

	source, err := store.RecordMemory(contract.CreateMemoryRequest{
		ID:          "mem_replay_source",
		MemoryClass: "semantic",
		MemoryKind:  "fact",
		ScopeType:   "project",
		ScopeID:     "replay-project",
		Content:     "Replay source fact.",
	})
	if err != nil {
		t.Fatal(err)
	}
	synthesis, err := store.SynthesizeMemory(contract.CreateMemorySynthesisRequest{
		ID:              "synth_replay",
		Kind:            contract.MemorySynthesisSkill,
		SourceMemoryIDs: []string{source.ID},
		Content:         "Apply the replay-safe procedure.",
		CreatedBy:       "reflection-agent",
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

	got, err := rebuilt.GetMemorySynthesis(synthesis.ID)
	if err != nil {
		t.Fatal(err)
	}
	if got.TargetMemoryID != synthesis.TargetMemoryID || len(got.SourceMemoryIDs) != 1 || got.SourceMemoryIDs[0] != source.ID {
		t.Fatalf("synthesis lineage did not replay exactly: %+v", got)
	}
	target, err := rebuilt.GetMemory(got.TargetMemoryID)
	if err != nil {
		t.Fatal(err)
	}
	if target.MemoryClass != "procedural" || target.MemoryKind != "procedure" || target.Metadata["synthesisKind"] != "skill" {
		t.Fatalf("target memory did not replay exactly: %+v", target)
	}
}

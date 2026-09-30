package agentstate

import (
	"errors"
	"testing"
)

func TestProposeRunLearningCandidateLinksRunAndEvidence(t *testing.T) {
	database := setupRunTestDB(t)
	defer database.Close()

	snapshot := ContextSnapshot{
		Hash: "sha256:learning", AgentID: "agt_learning", AgentRevision: 2,
		AgentTitle: "Learning Agent", KnowledgeScopes: []string{}, ArtifactScopes: []string{},
		ConversationScopes: []string{}, CapabilityRefs: []string{},
		Sections: []ContextSection{}, Unresolved: []ContextReferenceIssue{}, Text: "compiled",
	}
	seedContextSnapshot(t, database, snapshot)

	run, err := RecordRun(database, RunRecord{
		AgentName: "learning-agent", AgentID: "agt_learning", AgentRevision: 2,
		Task: "fix regression", Status: RunFailed, ContextHash: snapshot.Hash,
	})
	if err != nil {
		t.Fatal(err)
	}

	if _, err := database.Exec(`
		INSERT INTO run_observations (
			id, run_id, kind, name, status, input_json, output_json, evidence_json, created_at
		) VALUES (
			'obs_learning_1', ?, 'tool', 'go test', 'failed', '{}', '{}',
			'{"failure":"regression"}', datetime('now')
		)
	`, run.ID); err != nil {
		t.Fatal(err)
	}
	if _, err := database.Exec(`
		INSERT INTO evaluations (
			id, run_id, observation_id, evaluator, name, score, label, rationale, metadata_json, created_at
		) VALUES (
			'eval_learning_1', ?, 'obs_learning_1', 'human:test', 'regression', 0.0,
			'fail', 'Regression escaped the focused test loop.', '{}', datetime('now')
		)
	`, run.ID); err != nil {
		t.Fatal(err)
	}

	promotion, err := ProposeRunLearningCandidate(database, RunLearningCandidate{
		RunID:                run.ID,
		TargetKind:           PromotionMemory,
		Candidate:            "Run the focused regression test before broader verification.",
		Rationale:            "Evaluation identified a missing verification step.",
		SourceObservationIDs: []string{"obs_learning_1"},
		SourceEvaluationIDs:  []string{"eval_learning_1"},
	})
	if err != nil {
		t.Fatalf("ProposeRunLearningCandidate: %v", err)
	}
	if promotion.AgentID != "agt_learning" || promotion.Status != PromotionProposed {
		t.Fatalf("unexpected promotion identity/state: %#v", promotion)
	}
	if len(promotion.SourceRunIDs) != 1 || promotion.SourceRunIDs[0] != run.ID {
		t.Fatalf("expected originating run lineage, got %#v", promotion.SourceRunIDs)
	}
	if len(promotion.SourceObservationIDs) != 1 || promotion.SourceObservationIDs[0] != "obs_learning_1" {
		t.Fatalf("expected observation lineage, got %#v", promotion.SourceObservationIDs)
	}
	if len(promotion.SourceEvaluationIDs) != 1 || promotion.SourceEvaluationIDs[0] != "eval_learning_1" {
		t.Fatalf("expected evaluation lineage, got %#v", promotion.SourceEvaluationIDs)
	}
}

func TestProposeRunLearningCandidateRejectsCrossRunEvidence(t *testing.T) {
	database := setupRunTestDB(t)
	defer database.Close()

	runA, err := RecordRun(database, RunRecord{
		AgentName: "learning-agent", AgentID: "agt_learning", AgentRevision: 1,
		Task: "run A", Status: RunFailed,
	})
	if err != nil {
		t.Fatal(err)
	}
	runB, err := RecordRun(database, RunRecord{
		AgentName: "learning-agent", AgentID: "agt_learning", AgentRevision: 1,
		Task: "run B", Status: RunFailed,
	})
	if err != nil {
		t.Fatal(err)
	}
	if _, err := database.Exec(`
		INSERT INTO run_observations (
			id, run_id, kind, name, status, input_json, output_json, evidence_json, created_at
		) VALUES ('obs_other_run', ?, 'event', 'other evidence', 'failed', '{}', '{}', '{}', datetime('now'))
	`, runB.ID); err != nil {
		t.Fatal(err)
	}
	if _, err := database.Exec(`
		INSERT INTO evaluations (
			id, run_id, evaluator, name, label, metadata_json, created_at
		) VALUES ('eval_other_run', ?, 'human:test', 'regression', 'fail', '{}', datetime('now'))
	`, runB.ID); err != nil {
		t.Fatal(err)
	}

	_, err = ProposeRunLearningCandidate(database, RunLearningCandidate{
		RunID:                runA.ID,
		TargetKind:           PromotionKnowledge,
		Candidate:            "Do not mix evidence across unrelated runs.",
		SourceObservationIDs: []string{"obs_other_run"},
	})
	if !errors.Is(err, ErrConflict) {
		t.Fatalf("expected cross-run observation conflict, got %v", err)
	}

	_, err = ProposeRunLearningCandidate(database, RunLearningCandidate{
		RunID:               runA.ID,
		TargetKind:          PromotionKnowledge,
		Candidate:           "Do not mix evidence across unrelated runs.",
		SourceEvaluationIDs: []string{"eval_other_run"},
	})
	if !errors.Is(err, ErrConflict) {
		t.Fatalf("expected cross-run evaluation conflict, got %v", err)
	}
}

func TestProposeRunLearningCandidateRejectsUnboundLegacyRunAndMissingEvidence(t *testing.T) {
	database := setupRunTestDB(t)
	defer database.Close()

	legacy, err := RecordRun(database, RunRecord{
		AgentName: "legacy-agent", Task: "legacy", Status: RunFailed,
	})
	if err != nil {
		t.Fatal(err)
	}

	_, err = ProposeRunLearningCandidate(database, RunLearningCandidate{
		RunID: legacy.ID, TargetKind: PromotionMemory, Candidate: "Remember this.",
	})
	if !errors.Is(err, ErrConflict) {
		t.Fatalf("expected unbound run conflict, got %v", err)
	}

	bound, err := RecordRun(database, RunRecord{
		AgentName: "bound-agent", AgentID: "agt_bound", AgentRevision: 1,
		Task: "bound", Status: RunFailed,
	})
	if err != nil {
		t.Fatal(err)
	}
	_, err = ProposeRunLearningCandidate(database, RunLearningCandidate{
		RunID: bound.ID, TargetKind: PromotionMemory, Candidate: "Remember this.",
		SourceEvaluationIDs: []string{"eval_missing"},
	})
	if !errors.Is(err, ErrNotFound) {
		t.Fatalf("expected missing evaluation error, got %v", err)
	}
}

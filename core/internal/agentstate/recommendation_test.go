package agentstate

import "testing"

func TestRecommendRunLearningSurfacesDeterministicSignals(t *testing.T) {
	database := setupRunTestDB(t)
	defer database.Close()

	snapshot := ContextSnapshot{
		Hash: "sha256:recommend-learning", AgentID: "agt_recommend", AgentRevision: 4,
		AgentTitle: "Recommendation Agent", KnowledgeScopes: []string{}, ArtifactScopes: []string{},
		ConversationScopes: []string{}, CapabilityRefs: []string{},
		Sections: []ContextSection{
			{Kind: ContextMemory, SourceID: "memory_existing_1", Title: "Existing memory", Content: "Old behavior."},
			{Kind: ContextIdentity, SourceID: "identity_recommend", Title: "Identity", Content: "Be precise."},
		},
		Unresolved: []ContextReferenceIssue{}, Text: "compiled",
	}
	seedContextSnapshot(t, database, snapshot)

	run, err := RecordRun(database, RunRecord{
		AgentName: "recommend-agent", AgentID: "agt_recommend", AgentRevision: 4,
		Task: "fix a regression", Status: RunFailed, ContextHash: snapshot.Hash,
	})
	if err != nil {
		t.Fatal(err)
	}

	if _, err := database.Exec(`
		INSERT INTO run_observations (
			id, run_id, kind, name, status, input_json, output_json, evidence_json, created_at
		) VALUES
		('obs_failed_tool', ?, 'tool', 'go test focused', 'failed', '{}', '{}', '{"exitCode":1}', '2026-09-30T10:00:00Z'),
		('obs_context', ?, 'retrieval', 'load memory', 'succeeded', '{}', '{}', '{}', '2026-09-30T10:00:01Z')
	`, run.ID, run.ID); err != nil {
		t.Fatal(err)
	}

	if _, err := database.Exec(`
		INSERT INTO evaluations (
			id, run_id, observation_id, evaluator, name, score, label, rationale, metadata_json, created_at
		) VALUES
		('eval_regression', ?, 'obs_failed_tool', 'human:test', 'regression', 0.1, 'fail',
		 'Focused regression test was skipped before the broad suite.',
		 '{"target_kind":"memory","supersedes_note_id":"memory_existing_1"}',
		 '2026-09-30T10:00:02Z')
	`, run.ID); err != nil {
		t.Fatal(err)
	}

	rec, err := RecommendRunLearning(database, run.ID)
	if err != nil {
		t.Fatalf("RecommendRunLearning: %v", err)
	}
	if !rec.Eligible {
		t.Fatalf("expected eligible recommendation: %#v", rec)
	}
	if rec.RunID != run.ID || rec.AgentID != "agt_recommend" || rec.AgentRevision != 4 {
		t.Fatalf("unexpected run identity: %#v", rec)
	}
	if rec.SupportLevel != LearningSupportStrong || rec.EvidenceCount != 2 {
		t.Fatalf("unexpected support: level=%s count=%d", rec.SupportLevel, rec.EvidenceCount)
	}
	if rec.SuggestedTargetKind != PromotionMemory {
		t.Fatalf("expected explicit memory target hint, got %q", rec.SuggestedTargetKind)
	}
	if len(rec.SourceObservationIDs) != 1 || rec.SourceObservationIDs[0] != "obs_failed_tool" {
		t.Fatalf("unexpected source observations: %#v", rec.SourceObservationIDs)
	}
	if len(rec.SourceEvaluationIDs) != 1 || rec.SourceEvaluationIDs[0] != "eval_regression" {
		t.Fatalf("unexpected source evaluations: %#v", rec.SourceEvaluationIDs)
	}
	if len(rec.ContextMemoryRefs) != 1 || rec.ContextMemoryRefs[0] != "memory_existing_1" {
		t.Fatalf("unexpected context memory refs: %#v", rec.ContextMemoryRefs)
	}
	if len(rec.SupersedesNoteIDs) != 1 || rec.SupersedesNoteIDs[0] != "memory_existing_1" {
		t.Fatalf("unexpected supersession hints: %#v", rec.SupersedesNoteIDs)
	}
	if len(rec.Signals) != 2 {
		t.Fatalf("expected failed observation + failed evaluation signals, got %#v", rec.Signals)
	}
	if rec.Signals[0].Kind != LearningSignalObservation || rec.Signals[1].Kind != LearningSignalEvaluation {
		t.Fatalf("unexpected deterministic signal order: %#v", rec.Signals)
	}
}

func TestRecommendRunLearningDoesNotInterpretSuccessfulOrScoreOnlyEvidenceAsFailure(t *testing.T) {
	database := setupRunTestDB(t)
	defer database.Close()

	run, err := RecordRun(database, RunRecord{
		AgentName: "quiet-agent", AgentID: "agt_quiet", AgentRevision: 1,
		Task: "successful run", Status: RunSucceeded,
	})
	if err != nil {
		t.Fatal(err)
	}
	if _, err := database.Exec(`
		INSERT INTO run_observations (
			id, run_id, kind, name, status, input_json, output_json, evidence_json, created_at
		) VALUES ('obs_ok', ?, 'tool', 'tests', 'succeeded', '{}', '{}', '{}', datetime('now'))
	`, run.ID); err != nil {
		t.Fatal(err)
	}
	if _, err := database.Exec(`
		INSERT INTO evaluations (
			id, run_id, evaluator, name, score, metadata_json, created_at
		) VALUES ('eval_score_only', ?, 'metric:test', 'quality', 0.1, '{}', datetime('now'))
	`, run.ID); err != nil {
		t.Fatal(err)
	}

	rec, err := RecommendRunLearning(database, run.ID)
	if err != nil {
		t.Fatalf("RecommendRunLearning: %v", err)
	}
	if rec.Eligible || rec.SupportLevel != LearningSupportNone || rec.EvidenceCount != 0 {
		t.Fatalf("score-only/success evidence must not be inferred negative: %#v", rec)
	}
	if len(rec.SourceEvaluationIDs) != 0 || len(rec.SourceObservationIDs) != 0 {
		t.Fatalf("expected no learning evidence sources: %#v", rec)
	}
	if !containsString(rec.ReasonCodes, "no_negative_evidence") {
		t.Fatalf("expected no_negative_evidence reason: %#v", rec.ReasonCodes)
	}
}

func TestRecommendRunLearningMarksLegacyRunIneligible(t *testing.T) {
	database := setupRunTestDB(t)
	defer database.Close()

	run, err := RecordRun(database, RunRecord{
		AgentName: "legacy-agent", Task: "legacy failure", Status: RunFailed,
	})
	if err != nil {
		t.Fatal(err)
	}
	if _, err := database.Exec(`
		INSERT INTO run_observations (
			id, run_id, kind, name, status, input_json, output_json, evidence_json, created_at
		) VALUES ('obs_legacy_failed', ?, 'event', 'legacy failure', 'failed', '{}', '{}', '{}', datetime('now'))
	`, run.ID); err != nil {
		t.Fatal(err)
	}

	rec, err := RecommendRunLearning(database, run.ID)
	if err != nil {
		t.Fatalf("RecommendRunLearning: %v", err)
	}
	if rec.Eligible {
		t.Fatalf("legacy unbound run must not be eligible: %#v", rec)
	}
	if rec.SupportLevel != LearningSupportWeak || rec.EvidenceCount != 1 {
		t.Fatalf("expected evidence to remain visible despite ineligibility: %#v", rec)
	}
	if !containsString(rec.ReasonCodes, "run_unbound") {
		t.Fatalf("expected run_unbound reason: %#v", rec.ReasonCodes)
	}
}

func containsString(values []string, want string) bool {
	for _, value := range values {
		if value == want {
			return true
		}
	}
	return false
}

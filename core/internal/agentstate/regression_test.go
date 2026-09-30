package agentstate

import (
	"errors"
	"reflect"
	"testing"
)

func TestRecommendRunRegressionCaseDerivesEvidenceBackedProposal(t *testing.T) {
	database := setupRunTestDB(t)
	defer database.Close()

	run, err := RecordRun(database, RunRecord{
		AgentName: "regression-agent", AgentID: "agt_regression", AgentRevision: 3,
		Task: "fix checkout regression", Status: RunFailed,
		Input: map[string]any{"prompt": "fix checkout regression", "fixture": "checkout-42"},
	})
	if err != nil {
		t.Fatal(err)
	}
	if _, err := database.Exec(`
		INSERT INTO run_observations (
			id, run_id, kind, name, status, input_json, output_json, evidence_json, created_at
		) VALUES ('obs_regression_1', ?, 'tool', 'focused checkout test', 'failed', '{}', '{}', '{}', datetime('now'))
	`, run.ID); err != nil {
		t.Fatal(err)
	}
	if _, err := database.Exec(`
		INSERT INTO evaluations (
			id, run_id, observation_id, evaluator, name, label, rationale, metadata_json, created_at
		) VALUES (
			'eval_regression_1', ?, 'obs_regression_1', 'human:test', 'checkout regression', 'fail',
			'Checkout should remain idempotent.',
			'{"expected":{"checkout":"idempotent","status":"pass"}}',
			datetime('now')
		)
	`, run.ID); err != nil {
		t.Fatal(err)
	}

	proposal, err := RecommendRunRegressionCase(database, run.ID)
	if err != nil {
		t.Fatalf("RecommendRunRegressionCase: %v", err)
	}
	if !proposal.Eligible {
		t.Fatalf("expected eligible regression proposal: %#v", proposal)
	}
	if proposal.RunID != run.ID || proposal.AgentID != "agt_regression" || proposal.AgentRevision != 3 {
		t.Fatalf("unexpected proposal identity: %#v", proposal)
	}
	if proposal.Name != "Regression: fix checkout regression" {
		t.Fatalf("unexpected deterministic case name: %q", proposal.Name)
	}
	if !reflect.DeepEqual(proposal.Input, run.Input) {
		t.Fatalf("expected original run input, got %#v", proposal.Input)
	}
	wantExpected := map[string]any{"checkout": "idempotent", "status": "pass"}
	if !reflect.DeepEqual(proposal.Expected, wantExpected) {
		t.Fatalf("unexpected expected hint: %#v", proposal.Expected)
	}
	if !containsString(proposal.Tags, "regression") {
		t.Fatalf("expected regression tag: %#v", proposal.Tags)
	}
	if len(proposal.SourceObservationIDs) != 1 || proposal.SourceObservationIDs[0] != "obs_regression_1" {
		t.Fatalf("unexpected observation lineage: %#v", proposal.SourceObservationIDs)
	}
	if len(proposal.SourceEvaluationIDs) != 1 || proposal.SourceEvaluationIDs[0] != "eval_regression_1" {
		t.Fatalf("unexpected evaluation lineage: %#v", proposal.SourceEvaluationIDs)
	}
}

func TestRecommendRunRegressionCaseDoesNotGuessConflictingExpectedBehavior(t *testing.T) {
	database := setupRunTestDB(t)
	defer database.Close()

	run, err := RecordRun(database, RunRecord{
		AgentName: "regression-agent", AgentID: "agt_regression", AgentRevision: 1,
		Task: "resolve ambiguous behavior", Status: RunFailed,
		Input: map[string]any{"prompt": "ambiguous"},
	})
	if err != nil {
		t.Fatal(err)
	}
	if _, err := database.Exec(`
		INSERT INTO evaluations (
			id, run_id, evaluator, name, label, metadata_json, created_at
		) VALUES
		('eval_expected_a', ?, 'human:a', 'behavior', 'fail', '{"expected":{"answer":"A"}}', '2026-09-30T10:00:00Z'),
		('eval_expected_b', ?, 'human:b', 'behavior', 'fail', '{"expected":{"answer":"B"}}', '2026-09-30T10:00:01Z')
	`, run.ID, run.ID); err != nil {
		t.Fatal(err)
	}

	proposal, err := RecommendRunRegressionCase(database, run.ID)
	if err != nil {
		t.Fatalf("RecommendRunRegressionCase: %v", err)
	}
	if proposal.Expected != nil {
		t.Fatalf("conflicting expected hints must not be guessed: %#v", proposal.Expected)
	}
	if !containsString(proposal.ReasonCodes, "conflicting_expected_hints") {
		t.Fatalf("expected conflict reason: %#v", proposal.ReasonCodes)
	}
}

func TestCaptureRunRegressionCasePersistsProvenanceAndIsIdempotent(t *testing.T) {
	database := setupRunTestDB(t)
	defer database.Close()

	dataset, err := CreateEvaluationDataset(database, EvaluationDataset{
		Name: "Checkout regressions", AgentID: "agt_regression",
	})
	if err != nil {
		t.Fatal(err)
	}
	run, err := RecordRun(database, RunRecord{
		AgentName: "regression-agent", AgentID: "agt_regression", AgentRevision: 5,
		Task: "fix checkout regression", Status: RunFailed,
		Input: map[string]any{"fixture": "checkout-42"},
	})
	if err != nil {
		t.Fatal(err)
	}
	if _, err := database.Exec(`
		INSERT INTO run_observations (
			id, run_id, kind, name, status, input_json, output_json, evidence_json, created_at
		) VALUES ('obs_capture_regression', ?, 'tool', 'checkout test', 'failed', '{}', '{}', '{}', datetime('now'))
	`, run.ID); err != nil {
		t.Fatal(err)
	}
	if _, err := database.Exec(`
		INSERT INTO evaluations (
			id, run_id, observation_id, evaluator, name, label, metadata_json, created_at
		) VALUES (
			'eval_capture_regression', ?, 'obs_capture_regression', 'human:test', 'checkout', 'fail',
			'{"expected":{"status":"pass"}}', datetime('now')
		)
	`, run.ID); err != nil {
		t.Fatal(err)
	}

	first, err := CaptureRunRegressionCase(database, RunRegressionCaseCapture{
		RunID: run.ID, DatasetID: dataset.ID,
		Tags: []string{"checkout"},
	})
	if err != nil {
		t.Fatalf("CaptureRunRegressionCase: %v", err)
	}
	if first.SourceRunID != run.ID || first.AgentID != "agt_regression" || first.AgentRevision != 5 {
		t.Fatalf("missing persisted run/agent provenance: %#v", first)
	}
	if len(first.SourceObservationIDs) != 1 || first.SourceObservationIDs[0] != "obs_capture_regression" {
		t.Fatalf("missing observation provenance: %#v", first.SourceObservationIDs)
	}
	if len(first.SourceEvaluationIDs) != 1 || first.SourceEvaluationIDs[0] != "eval_capture_regression" {
		t.Fatalf("missing evaluation provenance: %#v", first.SourceEvaluationIDs)
	}
	if !containsString(first.Tags, "regression") || !containsString(first.Tags, "checkout") {
		t.Fatalf("expected default + caller tags: %#v", first.Tags)
	}

	second, err := CaptureRunRegressionCase(database, RunRegressionCaseCapture{
		RunID: run.ID, DatasetID: dataset.ID,
		Name: "retry should not duplicate",
	})
	if err != nil {
		t.Fatalf("idempotent retry: %v", err)
	}
	if second.ID != first.ID {
		t.Fatalf("expected idempotent existing case %s, got %s", first.ID, second.ID)
	}
	var count int
	if err := database.QueryRow(`
		SELECT COUNT(*) FROM evaluation_cases WHERE dataset_id = ? AND source_run_id = ?
	`, dataset.ID, run.ID).Scan(&count); err != nil {
		t.Fatal(err)
	}
	if count != 1 {
		t.Fatalf("expected exactly one regression case for source run, got %d", count)
	}
}

func TestCaptureRunRegressionCaseRejectsDatasetForDifferentAgent(t *testing.T) {
	database := setupRunTestDB(t)
	defer database.Close()

	dataset, err := CreateEvaluationDataset(database, EvaluationDataset{
		Name: "Other agent regressions", AgentID: "agt_other",
	})
	if err != nil {
		t.Fatal(err)
	}
	run, err := RecordRun(database, RunRecord{
		AgentName: "regression-agent", AgentID: "agt_regression", AgentRevision: 2,
		Task: "failure", Status: RunFailed, Input: map[string]any{"x": true},
	})
	if err != nil {
		t.Fatal(err)
	}
	if _, err := database.Exec(`
		INSERT INTO run_observations (
			id, run_id, kind, name, status, input_json, output_json, evidence_json, created_at
		) VALUES ('obs_agent_mismatch', ?, 'event', 'failure', 'failed', '{}', '{}', '{}', datetime('now'))
	`, run.ID); err != nil {
		t.Fatal(err)
	}

	_, err = CaptureRunRegressionCase(database, RunRegressionCaseCapture{
		RunID: run.ID, DatasetID: dataset.ID,
	})
	if !errors.Is(err, ErrConflict) {
		t.Fatalf("expected dataset/agent conflict, got %v", err)
	}
}

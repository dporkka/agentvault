package agentstate

import (
	"errors"
	"testing"
)

func TestCompareExperimentsClassifiesExplicitLabelTransitions(t *testing.T) {
	database := setupRunTestDB(t)
	defer database.Close()

	dataset, err := CreateEvaluationDataset(database, EvaluationDataset{
		Name: "Regression suite", AgentID: "agt_compare",
	})
	if err != nil {
		t.Fatal(err)
	}

	cases := []EvaluationCase{
		{ID: "case_fixed", DatasetID: dataset.ID, Name: "Fixed", Input: map[string]any{"case": "fixed"}},
		{ID: "case_regressed", DatasetID: dataset.ID, Name: "Regressed", Input: map[string]any{"case": "regressed"}},
		{ID: "case_stable", DatasetID: dataset.ID, Name: "Stable", Input: map[string]any{"case": "stable"}},
		{ID: "case_unknown", DatasetID: dataset.ID, Name: "Unknown score scale", Input: map[string]any{"case": "unknown"}},
		{ID: "case_missing", DatasetID: dataset.ID, Name: "Missing candidate", Input: map[string]any{"case": "missing"}},
	}
	for _, item := range cases {
		if _, err := AddEvaluationCase(database, item); err != nil {
			t.Fatal(err)
		}
	}

	baseline, err := RecordExperiment(database, Experiment{
		ID: "exp_baseline", DatasetID: dataset.ID, Name: "baseline",
		AgentID: "agt_compare", AgentRevision: 1, Status: ExperimentCompleted,
		Config: map[string]any{"contextHash": "sha256:baseline"},
	})
	if err != nil {
		t.Fatal(err)
	}
	candidate, err := RecordExperiment(database, Experiment{
		ID: "exp_candidate", DatasetID: dataset.ID, Name: "candidate",
		AgentID: "agt_compare", AgentRevision: 2, Status: ExperimentCompleted,
		Config: map[string]any{"contextHash": "sha256:candidate"},
	})
	if err != nil {
		t.Fatal(err)
	}

	score02, score09 := 0.2, 0.9
	score08, score01 := 0.8, 0.1
	score04, score07 := 0.4, 0.7
	records := []ExperimentResult{
		{ExperimentID: baseline.ID, CaseID: "case_fixed", Label: "fail", Score: &score02},
		{ExperimentID: candidate.ID, CaseID: "case_fixed", Label: "pass", Score: &score09},
		{ExperimentID: baseline.ID, CaseID: "case_regressed", Label: "pass", Score: &score08},
		{ExperimentID: candidate.ID, CaseID: "case_regressed", Label: "fail", Score: &score01},
		{ExperimentID: baseline.ID, CaseID: "case_stable", Label: "pass"},
		{ExperimentID: candidate.ID, CaseID: "case_stable", Label: "passed"},
		{ExperimentID: baseline.ID, CaseID: "case_unknown", Label: "bronze", Score: &score04},
		{ExperimentID: candidate.ID, CaseID: "case_unknown", Label: "silver", Score: &score07},
		{ExperimentID: baseline.ID, CaseID: "case_missing", Label: "fail"},
	}
	for _, record := range records {
		if _, err := RecordExperimentResult(database, record); err != nil {
			t.Fatal(err)
		}
	}

	comparison, err := CompareExperiments(database, baseline.ID, candidate.ID)
	if err != nil {
		t.Fatalf("CompareExperiments: %v", err)
	}

	if comparison.BaselineExperimentID != baseline.ID || comparison.CandidateExperimentID != candidate.ID {
		t.Fatalf("unexpected comparison identity: %#v", comparison)
	}
	if comparison.DatasetID != dataset.ID || comparison.AgentID != "agt_compare" {
		t.Fatalf("unexpected comparison scope: %#v", comparison)
	}
	if comparison.BaselineAgentRevision != 1 || comparison.CandidateAgentRevision != 2 {
		t.Fatalf("unexpected revisions: %#v", comparison)
	}
	if !comparison.Comparable {
		t.Fatalf("expected comparable experiments: %#v", comparison)
	}

	if comparison.Summary.TotalCases != 5 ||
		comparison.Summary.Fixes != 1 ||
		comparison.Summary.Regressions != 1 ||
		comparison.Summary.StablePass != 1 ||
		comparison.Summary.Unclassified != 1 ||
		comparison.Summary.MissingCandidate != 1 {
		t.Fatalf("unexpected comparison summary: %#v", comparison.Summary)
	}

	byID := map[string]ExperimentCaseComparison{}
	for _, item := range comparison.Cases {
		byID[item.CaseID] = item
	}
	if byID["case_fixed"].Transition != ExperimentTransitionFixed {
		t.Fatalf("expected fixed transition: %#v", byID["case_fixed"])
	}
	if byID["case_regressed"].Transition != ExperimentTransitionRegressed {
		t.Fatalf("expected regression transition: %#v", byID["case_regressed"])
	}
	if byID["case_stable"].Transition != ExperimentTransitionStablePass {
		t.Fatalf("expected stable pass transition: %#v", byID["case_stable"])
	}
	if byID["case_unknown"].Transition != ExperimentTransitionUnclassified {
		t.Fatalf("numeric scores and unknown labels must remain unclassified: %#v", byID["case_unknown"])
	}
	if byID["case_unknown"].ScoreDelta == nil || *byID["case_unknown"].ScoreDelta != 0.3 {
		t.Fatalf("expected raw score delta without semantic judgment: %#v", byID["case_unknown"])
	}
	if byID["case_missing"].Transition != ExperimentTransitionMissingCandidate {
		t.Fatalf("expected missing candidate transition: %#v", byID["case_missing"])
	}
}

func TestCompareExperimentsRejectsDifferentDatasetOrAgent(t *testing.T) {
	database := setupRunTestDB(t)
	defer database.Close()

	dsA, err := CreateEvaluationDataset(database, EvaluationDataset{Name: "A"})
	if err != nil {
		t.Fatal(err)
	}
	dsB, err := CreateEvaluationDataset(database, EvaluationDataset{Name: "B"})
	if err != nil {
		t.Fatal(err)
	}

	base, err := RecordExperiment(database, Experiment{
		ID: "exp_scope_base", DatasetID: dsA.ID, Name: "base",
		AgentID: "agt_a", AgentRevision: 1, Status: ExperimentCompleted,
	})
	if err != nil {
		t.Fatal(err)
	}
	otherDataset, err := RecordExperiment(database, Experiment{
		ID: "exp_scope_dataset", DatasetID: dsB.ID, Name: "other dataset",
		AgentID: "agt_a", AgentRevision: 2, Status: ExperimentCompleted,
	})
	if err != nil {
		t.Fatal(err)
	}
	otherAgent, err := RecordExperiment(database, Experiment{
		ID: "exp_scope_agent", DatasetID: dsA.ID, Name: "other agent",
		AgentID: "agt_b", AgentRevision: 2, Status: ExperimentCompleted,
	})
	if err != nil {
		t.Fatal(err)
	}

	if _, err := CompareExperiments(database, base.ID, otherDataset.ID); !errors.Is(err, ErrConflict) {
		t.Fatalf("expected dataset conflict, got %v", err)
	}
	if _, err := CompareExperiments(database, base.ID, otherAgent.ID); !errors.Is(err, ErrConflict) {
		t.Fatalf("expected agent conflict, got %v", err)
	}
}

func TestCompareExperimentsIncludesCasesMissingFromBaseline(t *testing.T) {
	database := setupRunTestDB(t)
	defer database.Close()

	dataset, err := CreateEvaluationDataset(database, EvaluationDataset{Name: "Suite", AgentID: "agt_compare"})
	if err != nil {
		t.Fatal(err)
	}
	if _, err := AddEvaluationCase(database, EvaluationCase{
		ID: "case_new", DatasetID: dataset.ID, Name: "New case", Input: map[string]any{"new": true},
	}); err != nil {
		t.Fatal(err)
	}

	baseline, err := RecordExperiment(database, Experiment{
		ID: "exp_missing_base", DatasetID: dataset.ID, Name: "baseline",
		AgentID: "agt_compare", AgentRevision: 1, Status: ExperimentCompleted,
	})
	if err != nil {
		t.Fatal(err)
	}
	candidate, err := RecordExperiment(database, Experiment{
		ID: "exp_missing_candidate", DatasetID: dataset.ID, Name: "candidate",
		AgentID: "agt_compare", AgentRevision: 2, Status: ExperimentCompleted,
	})
	if err != nil {
		t.Fatal(err)
	}
	if _, err := RecordExperimentResult(database, ExperimentResult{
		ExperimentID: candidate.ID, CaseID: "case_new", Label: "pass",
	}); err != nil {
		t.Fatal(err)
	}

	comparison, err := CompareExperiments(database, baseline.ID, candidate.ID)
	if err != nil {
		t.Fatal(err)
	}
	if comparison.Summary.MissingBaseline != 1 || len(comparison.Cases) != 1 {
		t.Fatalf("unexpected missing-baseline summary: %#v", comparison)
	}
	if comparison.Cases[0].Transition != ExperimentTransitionMissingBaseline {
		t.Fatalf("expected missing-baseline transition: %#v", comparison.Cases[0])
	}
}

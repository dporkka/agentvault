package agentstate

import "testing"

func TestAgentManifestValidate(t *testing.T) {
	valid := AgentManifest{ID: "agt_1", Name: "Coder", Revision: 1}
	if err := valid.Validate(); err != nil {
		t.Fatalf("valid manifest rejected: %v", err)
	}

	cases := []AgentManifest{
		{Name: "Coder", Revision: 1},
		{ID: "agt_1", Revision: 1},
		{ID: "agt_1", Name: "Coder", Revision: 0},
	}
	for _, tc := range cases {
		if err := tc.Validate(); err == nil {
			t.Fatalf("expected invalid manifest to fail: %+v", tc)
		}
	}
}

func TestRunValidate(t *testing.T) {
	r := Run{ID: "run_1", AgentID: "agt_1", AgentRevision: 1, Status: RunSucceeded}
	if err := r.Validate(); err != nil {
		t.Fatalf("valid run rejected: %v", err)
	}
	r.Status = "mystery"
	if err := r.Validate(); err == nil {
		t.Fatal("unknown run status should fail")
	}
}

func TestObservationValidate(t *testing.T) {
	obs := Observation{ID: "obs_1", RunID: "run_1", Kind: ObservationTool, Name: "github.search"}
	if err := obs.Validate(); err != nil {
		t.Fatalf("valid observation rejected: %v", err)
	}
	if err := (Observation{ID: "obs_1", RunID: "run_1", Kind: "unknown", Name: "x"}).Validate(); err == nil {
		t.Fatal("expected unknown observation kind to fail")
	}
}

func TestEvaluationValidate(t *testing.T) {
	score := 0.9
	e := Evaluation{ID: "eval_1", RunID: "run_1", Evaluator: "human", Name: "correctness", Score: &score}
	if err := e.Validate(); err != nil {
		t.Fatalf("valid evaluation rejected: %v", err)
	}
	e.Score = nil
	if err := e.Validate(); err == nil {
		t.Fatal("evaluation without score or label should fail")
	}
}

func TestPromotionRecordValidate(t *testing.T) {
	p := PromotionRecord{
		ID: "promo_1", AgentID: "agt_1", TargetKind: PromotionMemory,
		Status: PromotionProposed, SourceObservationIDs: []string{"obs_1"}, Candidate: "Prefer concise diffs",
	}
	if err := p.Validate(); err != nil {
		t.Fatalf("valid proposed promotion rejected: %v", err)
	}

	committed := p
	committed.Status = PromotionCommitted
	if err := committed.Validate(); err == nil {
		t.Fatal("committed promotion without target note should fail")
	}
	committed.TargetNoteID = "note_memory_1"
	if err := committed.Validate(); err != nil {
		t.Fatalf("committed promotion with target note rejected: %v", err)
	}

	noEvidence := p
	noEvidence.SourceObservationIDs = nil
	if err := noEvidence.Validate(); err == nil {
		t.Fatal("promotion without evidence should fail")
	}
}

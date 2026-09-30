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

func TestPromotionTransitionValidate(t *testing.T) {
	if err := ValidatePromotionTransition(PromotionProposed, PromotionApproved); err != nil {
		t.Fatalf("proposed -> approved rejected: %v", err)
	}
	if err := ValidatePromotionTransition(PromotionProposed, PromotionRejected); err != nil {
		t.Fatalf("proposed -> rejected rejected: %v", err)
	}
	if err := ValidatePromotionTransition(PromotionApproved, PromotionCommitted); err != nil {
		t.Fatalf("approved -> committed rejected: %v", err)
	}

	for _, tc := range []struct {
		from PromotionStatus
		to   PromotionStatus
	}{
		{PromotionProposed, PromotionCommitted},
		{PromotionRejected, PromotionApproved},
		{PromotionCommitted, PromotionApproved},
	} {
		if err := ValidatePromotionTransition(tc.from, tc.to); err == nil {
			t.Fatalf("expected invalid promotion transition %s -> %s", tc.from, tc.to)
		}
	}
}

func TestEvaluationDatasetValidate(t *testing.T) {
	valid := EvaluationDataset{ID: "ds_1", Name: "Golden path"}
	if err := valid.Validate(); err != nil {
		t.Fatalf("valid dataset rejected: %v", err)
	}
	if err := (EvaluationDataset{ID: "ds_1"}).Validate(); err == nil {
		t.Fatal("dataset without name should fail")
	}
}

func TestEvaluationCaseValidate(t *testing.T) {
	valid := EvaluationCase{
		ID: "case_1", DatasetID: "ds_1", Name: "Create note",
		Input: map[string]any{"prompt": "create a note"},
	}
	if err := valid.Validate(); err != nil {
		t.Fatalf("valid case rejected: %v", err)
	}
	if err := (EvaluationCase{ID: "case_1", DatasetID: "ds_1", Name: "bad"}).Validate(); err == nil {
		t.Fatal("case without input should fail")
	}
}

func TestExperimentValidate(t *testing.T) {
	valid := Experiment{
		ID: "exp_1", DatasetID: "ds_1", Name: "baseline",
		AgentID: "agt_1", AgentRevision: 2, Status: ExperimentCompleted,
	}
	if err := valid.Validate(); err != nil {
		t.Fatalf("valid experiment rejected: %v", err)
	}
	valid.Status = "mystery"
	if err := valid.Validate(); err == nil {
		t.Fatal("unknown experiment status should fail")
	}
}

func TestExperimentResultValidate(t *testing.T) {
	score := 0.8
	valid := ExperimentResult{
		ExperimentID: "exp_1", CaseID: "case_1", Score: &score,
	}
	if err := valid.Validate(); err != nil {
		t.Fatalf("valid experiment result rejected: %v", err)
	}
	valid.Score = nil
	if err := valid.Validate(); err == nil {
		t.Fatal("experiment result without score or label should fail")
	}
}


func TestActionIntentValidate(t *testing.T) {
	valid := ActionIntent{
		ID: "intent_1", RunID: "run_1", OperationID: "op_1",
		Action: "github.create_pr", CapabilityRef: "github.write",
		AuthorityHash: "sha256:authority", InputHash: "sha256:input",
	}
	if err := valid.Validate(); err != nil {
		t.Fatalf("valid action intent rejected: %v", err)
	}

	cases := []ActionIntent{
		{RunID: "run_1", OperationID: "op_1", Action: "github.create_pr", CapabilityRef: "github.write", AuthorityHash: "sha256:a", InputHash: "sha256:i"},
		{ID: "intent_1", OperationID: "op_1", Action: "github.create_pr", CapabilityRef: "github.write", AuthorityHash: "sha256:a", InputHash: "sha256:i"},
		{ID: "intent_1", RunID: "run_1", Action: "github.create_pr", CapabilityRef: "github.write", AuthorityHash: "sha256:a", InputHash: "sha256:i"},
		{ID: "intent_1", RunID: "run_1", OperationID: "op_1", CapabilityRef: "github.write", AuthorityHash: "sha256:a", InputHash: "sha256:i"},
		{ID: "intent_1", RunID: "run_1", OperationID: "op_1", Action: "github.create_pr", AuthorityHash: "sha256:a", InputHash: "sha256:i"},
		{ID: "intent_1", RunID: "run_1", OperationID: "op_1", Action: "github.create_pr", CapabilityRef: "github.write", InputHash: "sha256:i"},
		{ID: "intent_1", RunID: "run_1", OperationID: "op_1", Action: "github.create_pr", CapabilityRef: "github.write", AuthorityHash: "sha256:a"},
	}
	for _, tc := range cases {
		if err := tc.Validate(); err == nil {
			t.Fatalf("expected invalid action intent to fail: %+v", tc)
		}
	}
}

func TestActionReceiptValidate(t *testing.T) {
	completed := ActionReceipt{
		ID: "receipt_1", IntentID: "intent_1",
		Status: ActionCompleted, ResultHash: "sha256:result",
	}
	if err := completed.Validate(); err != nil {
		t.Fatalf("valid completed receipt rejected: %v", err)
	}
	failed := ActionReceipt{
		ID: "receipt_2", IntentID: "intent_1",
		Status: ActionFailed, ErrorCode: "provider_error",
	}
	if err := failed.Validate(); err != nil {
		t.Fatalf("valid failed receipt rejected: %v", err)
	}
	indeterminate := ActionReceipt{
		ID: "receipt_3", IntentID: "intent_1",
		Status: ActionIndeterminate, ErrorMessage: "provider outcome unknown",
	}
	if err := indeterminate.Validate(); err != nil {
		t.Fatalf("valid indeterminate receipt rejected: %v", err)
	}

	for name, receipt := range map[string]ActionReceipt{
		"missing intent": {ID: "r", Status: ActionFailed, ErrorCode: "x"},
		"unknown status": {ID: "r", IntentID: "i", Status: "pending", ErrorCode: "x"},
		"completed without result": {ID: "r", IntentID: "i", Status: ActionCompleted},
		"failed without reason": {ID: "r", IntentID: "i", Status: ActionFailed},
		"indeterminate without reason": {ID: "r", IntentID: "i", Status: ActionIndeterminate},
	} {
		t.Run(name, func(t *testing.T) {
			if err := receipt.Validate(); err == nil {
				t.Fatalf("expected invalid receipt to fail: %+v", receipt)
			}
		})
	}
}

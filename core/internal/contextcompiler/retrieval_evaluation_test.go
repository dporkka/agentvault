package contextcompiler

import (
	"fmt"
	"testing"

	"github.com/agentvault/core/internal/contract"
)

func TestContextCompilerRetrievalEvaluationSuite(t *testing.T) {
	compiler, store, database, vault := setupCompiler(t)
	defer database.Close()

	writeAndIndexNote(t, database, vault, "deployment.md", `---
id: note_eval_deployment
type: note
title: Production Deployment Verification
project: adacavo
status: active
created: 2026-09-20T10:00:00Z
updated: 2026-09-29T10:00:00Z
---
Production deployment requires a signed verification record and rollback check.
`)
	writeAndIndexNote(t, database, vault, "other-project.md", `---
id: note_eval_other
type: note
title: Production Deployment Secret
project: other-project
status: active
created: 2026-09-20T10:00:00Z
updated: 2026-09-29T10:00:00Z
---
Production deployment uses the secret other-project release procedure.
`)

	confidence := 0.98
	provenance, err := store.CreateProvenance(contract.ProvenanceRecord{
		ID:         "prov_eval_suite",
		SourceType: "file",
		SourceID:   "deployment-spec",
		AgentID:    "release-agent",
		Confidence: confidence,
		ObservedAt: "2026-09-29T10:00:00Z",
		Evidence: []contract.ProvenanceEvidence{{
			Source: "file",
			Path:   "10-notes/deployment.md",
		}},
	})
	if err != nil {
		t.Fatal(err)
	}

	deploymentObject, err := store.UpsertObject(contract.UpsertKnowledgeObjectRequest{
		ID:           "obj_eval_deployment",
		Type:         "requirement",
		Title:        "Signed deployment verification",
		Project:      "adacavo",
		ProvenanceID: provenance.ID,
		Data: map[string]interface{}{
			"policy": "signed verification record",
		},
	})
	if err != nil {
		t.Fatal(err)
	}
	if _, err := store.UpsertObject(contract.UpsertKnowledgeObjectRequest{
		ID:           "obj_eval_distractor",
		Type:         "artifact",
		Title:        "Unrelated customer export",
		Project:      "adacavo",
		ProvenanceID: provenance.ID,
	}); err != nil {
		t.Fatal(err)
	}

	oldMemory, err := store.RecordMemory(contract.CreateMemoryRequest{
		ID:           "mem_eval_old",
		MemoryClass:  "semantic",
		MemoryKind:   "constraint",
		ScopeType:    "project",
		ScopeID:      "adacavo",
		Content:      "Production deployments require verbal approval.",
		ObjectID:     deploymentObject.ID,
		ProvenanceID: provenance.ID,
		Confidence:   &confidence,
	})
	if err != nil {
		t.Fatal(err)
	}
	if _, err := store.RecordMemory(contract.CreateMemoryRequest{
		ID:           "mem_eval_current",
		MemoryClass:  "semantic",
		MemoryKind:   "constraint",
		ScopeType:    "project",
		ScopeID:      "adacavo",
		Content:      "Production deployments require a signed verification record.",
		ObjectID:     deploymentObject.ID,
		ProvenanceID: provenance.ID,
		Confidence:   &confidence,
		SupersedesID: oldMemory.ID,
	}); err != nil {
		t.Fatal(err)
	}
	if _, err := store.RecordMemory(contract.CreateMemoryRequest{
		ID:           "mem_eval_other_project",
		MemoryClass:  "semantic",
		MemoryKind:   "constraint",
		ScopeType:    "project",
		ScopeID:      "other-project",
		Content:      "Secret other-project production deployment procedure.",
		ProvenanceID: provenance.ID,
		Confidence:   &confidence,
	}); err != nil {
		t.Fatal(err)
	}

	oldFact, err := store.RecordFact(contract.CreateTemporalFactRequest{
		ID:           "fact_eval_old",
		SubjectID:    deploymentObject.ID,
		Predicate:    "approvalPolicy",
		Value:        "verbal approval",
		ProvenanceID: provenance.ID,
		Confidence:   &confidence,
		ValidFrom:    "2026-01-01",
	})
	if err != nil {
		t.Fatal(err)
	}
	if _, err := store.RecordFact(contract.CreateTemporalFactRequest{
		ID:           "fact_eval_current",
		SubjectID:    deploymentObject.ID,
		Predicate:    "approvalPolicy",
		Value:        "signed verification record",
		ProvenanceID: provenance.ID,
		Confidence:   &confidence,
		ValidFrom:    "2026-09-01",
		SupersedesID: oldFact.ID,
	}); err != nil {
		t.Fatal(err)
	}

	session, err := store.StartSession(contract.StartAgentSessionRequest{
		ID:        "session_eval_suite",
		AgentID:   "release-agent",
		Project:   "adacavo",
		Objective: "Verify the production deployment",
	})
	if err != nil {
		t.Fatal(err)
	}
	for i := 0; i < 15; i++ {
		if _, err := store.AppendSessionEvent(session.ID, contract.AppendSessionEventRequest{
			ID:        fmt.Sprintf("event_eval_%02d", i),
			EventType: "verification",
			Payload: map[string]interface{}{
				"step": fmt.Sprintf("deployment verification step %02d", i),
			},
			ProvenanceID: provenance.ID,
		}); err != nil {
			t.Fatal(err)
		}
	}

	compile := func(req contract.CompileContextRequest) contract.ContextBundle {
		t.Helper()
		bundle, err := compiler.Compile(req)
		if err != nil {
			t.Fatalf("Compile(%s): %v", req.Task, err)
		}
		return bundle
	}

	projectBundle := compile(contract.CompileContextRequest{
		Task:        "production deployment signed verification record",
		Project:     "adacavo",
		TokenBudget: 1400,
		MaxItems:    20,
		AsOf:        "2099-01-01T00:00:00Z",
	})
	temporalBundle := compile(contract.CompileContextRequest{
		Task:        "what is the deployment approval policy",
		Project:     "adacavo",
		TokenBudget: 1200,
		MaxItems:    20,
		AsOf:        "2099-01-01T00:00:00Z",
	})
	explicitBundle := compile(contract.CompileContextRequest{
		Task:        "signed deployment verification requirement",
		Project:     "adacavo",
		ObjectIDs:   []string{deploymentObject.ID},
		TokenBudget: 1200,
		MaxItems:    20,
		AsOf:        "2099-01-01T00:00:00Z",
	})
	sessionBundle := compile(contract.CompileContextRequest{
		Task:        "verify the production deployment",
		Project:     "adacavo",
		AgentID:     "release-agent",
		SessionID:   session.ID,
		TokenBudget: 1800,
		MaxItems:    30,
		AsOf:        "2099-01-01T00:00:00Z",
	})

	report := EvaluateRetrieval([]RetrievalEvaluationCase{
		{
			Name:   "project recall and isolation",
			Bundle: projectBundle,
			Expect: RetrievalExpectation{
				RequiredIDs:       []string{"mem_eval_current", "note_eval_deployment"},
				ForbiddenIDs:      []string{"mem_eval_old", "mem_eval_other_project", "note_eval_other"},
				RequireProvenance: []string{"mem_eval_current"},
			},
		},
		{
			Name:   "temporal supersession",
			Bundle: temporalBundle,
			Expect: RetrievalExpectation{
				RequiredIDs:       []string{"fact_eval_current"},
				ForbiddenIDs:      []string{"fact_eval_old"},
				RequireProvenance: []string{"fact_eval_current"},
			},
		},
		{
			Name:   "explicit object ranking",
			Bundle: explicitBundle,
			Expect: RetrievalExpectation{
				RequiredIDs:       []string{"obj_eval_deployment", "note_eval_deployment"},
				RequireProvenance: []string{"obj_eval_deployment"},
				PreferredBefore: []RankingPreference{{
					HigherID: "obj_eval_deployment",
					LowerID:  "note_eval_deployment",
				}},
			},
		},
		{
			Name:   "bounded current session recall",
			Bundle: sessionBundle,
			Expect: RetrievalExpectation{
				RequiredIDs:       []string{"session_eval_suite", "event_eval_14"},
				ForbiddenIDs:      []string{"event_eval_00", "mem_eval_other_project", "note_eval_other"},
				RequireProvenance: []string{"event_eval_14"},
			},
		},
	})

	for _, result := range report.Cases {
		if !result.Passed {
			t.Errorf("retrieval evaluation case failed: %+v", result)
		}
	}
	if !report.Passed {
		t.Fatalf("retrieval evaluation suite failed: %+v", report)
	}
	if report.MacroRecall != 1 {
		t.Fatalf("expected perfect required-context recall, got %.3f", report.MacroRecall)
	}
	if report.TotalLeakage != 0 {
		t.Fatalf("expected zero forbidden-context leakage, got %d", report.TotalLeakage)
	}
	if report.MacroProvenanceCoverage != 1 {
		t.Fatalf("expected complete required provenance coverage, got %.3f", report.MacroProvenanceCoverage)
	}
	if report.MeanTokenUtilization > 1 {
		t.Fatalf("context exceeded token budget: utilization %.3f", report.MeanTokenUtilization)
	}
	if report.MeanRequiredPer1KTokens < 1 {
		t.Fatalf("retrieval density regressed below floor: %.3f required items / 1k tokens", report.MeanRequiredPer1KTokens)
	}

	t.Logf("retrieval eval: recall=%.3f leakage=%d provenance=%.3f token_util=%.3f required_per_1k=%.3f",
		report.MacroRecall,
		report.TotalLeakage,
		report.MacroProvenanceCoverage,
		report.MeanTokenUtilization,
		report.MeanRequiredPer1KTokens,
	)
}

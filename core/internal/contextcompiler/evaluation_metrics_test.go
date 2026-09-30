package contextcompiler

import (
	"math"
	"testing"

	"github.com/agentvault/core/internal/contract"
)

func TestEvaluateRetrievalAggregatesRecallLeakageProvenanceOrderingAndEfficiency(t *testing.T) {
	report := EvaluateRetrieval([]RetrievalEvaluationCase{
		{
			Name: "deployment",
			Bundle: contract.ContextBundle{
				TokenBudget:     1000,
				EstimatedTokens: 400,
				Items: []contract.ContextItem{
					{ID: "required-a", Score: 0.9, Provenance: &contract.ContextProvenance{ID: "prov-a", SourceType: "file"}},
					{ID: "required-b", Score: 0.8},
					{ID: "forbidden", Score: 0.7},
				},
			},
			Expect: RetrievalExpectation{
				RequiredIDs:       []string{"required-a", "required-b", "missing"},
				ForbiddenIDs:      []string{"forbidden", "not-present"},
				RequireProvenance: []string{"required-a"},
				PreferredBefore: []RankingPreference{
					{HigherID: "required-a", LowerID: "required-b"},
				},
			},
		},
	})

	if len(report.Cases) != 1 {
		t.Fatalf("expected one case, got %+v", report)
	}
	result := report.Cases[0]
	if math.Abs(result.Recall-2.0/3.0) > 0.000001 {
		t.Fatalf("unexpected recall %f", result.Recall)
	}
	if result.LeakageCount != 1 || len(result.ForbiddenFound) != 1 || result.ForbiddenFound[0] != "forbidden" {
		t.Fatalf("unexpected leakage result %+v", result)
	}
	if result.ProvenanceCoverage != 1 {
		t.Fatalf("unexpected provenance coverage %f", result.ProvenanceCoverage)
	}
	if len(result.OrderViolations) != 0 {
		t.Fatalf("unexpected ordering violations %+v", result.OrderViolations)
	}
	if math.Abs(result.TokenUtilization-0.4) > 0.000001 {
		t.Fatalf("unexpected token utilization %f", result.TokenUtilization)
	}
	if math.Abs(result.RequiredPer1KTokens-5.0) > 0.000001 {
		t.Fatalf("unexpected required-per-1k metric %f", result.RequiredPer1KTokens)
	}
	if result.Passed {
		t.Fatal("case with leakage and missing required context must fail")
	}
	if report.Passed {
		t.Fatal("report must fail when a case fails")
	}
	if report.TotalLeakage != 1 {
		t.Fatalf("unexpected total leakage %d", report.TotalLeakage)
	}
	if math.Abs(report.MacroRecall-result.Recall) > 0.000001 {
		t.Fatalf("unexpected macro recall %f", report.MacroRecall)
	}
}

func TestEvaluateRetrievalPassesPerfectCase(t *testing.T) {
	report := EvaluateRetrieval([]RetrievalEvaluationCase{
		{
			Name: "perfect",
			Bundle: contract.ContextBundle{
				TokenBudget:     500,
				EstimatedTokens: 100,
				Items: []contract.ContextItem{
					{ID: "explicit", Score: 0.9, Provenance: &contract.ContextProvenance{ID: "prov", SourceType: "file"}},
					{ID: "support", Score: 0.8},
				},
			},
			Expect: RetrievalExpectation{
				RequiredIDs:       []string{"explicit", "support"},
				ForbiddenIDs:      []string{"secret"},
				RequireProvenance: []string{"explicit"},
				PreferredBefore:   []RankingPreference{{HigherID: "explicit", LowerID: "support"}},
			},
		},
	})

	if !report.Passed || !report.Cases[0].Passed {
		t.Fatalf("perfect evaluation should pass: %+v", report)
	}
	if report.MacroRecall != 1 || report.TotalLeakage != 0 || report.MacroProvenanceCoverage != 1 {
		t.Fatalf("unexpected aggregate metrics: %+v", report)
	}
}

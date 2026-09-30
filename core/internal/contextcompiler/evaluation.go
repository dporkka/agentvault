package contextcompiler

import (
	"fmt"

	"github.com/agentvault/core/internal/contract"
)

// RetrievalEvaluationCase evaluates one compiled context bundle against
// explicit expectations. The evaluator is deliberately model-agnostic so the
// same corpus can compare deterministic, vector, graph, or hybrid rankers.
type RetrievalEvaluationCase struct {
	Name   string
	Bundle contract.ContextBundle
	Expect RetrievalExpectation
}

// RetrievalExpectation declares the context that must be present, must not be
// present, must retain provenance, and must outrank another item.
type RetrievalExpectation struct {
	RequiredIDs       []string
	ForbiddenIDs      []string
	RequireProvenance []string
	PreferredBefore   []RankingPreference
}

// RankingPreference asserts that HigherID should appear before LowerID.
type RankingPreference struct {
	HigherID string
	LowerID  string
}

// RetrievalEvaluationCaseResult contains metrics for one evaluation case.
type RetrievalEvaluationCaseResult struct {
	Name                string
	RequiredFound       int
	RequiredTotal       int
	Recall              float64
	ForbiddenFound      []string
	LeakageCount        int
	ProvenanceCovered   int
	ProvenanceRequired  int
	ProvenanceCoverage  float64
	OrderViolations     []string
	PreferenceTotal     int
	PreferenceAccuracy  float64
	TokenUtilization    float64
	RequiredPer1KTokens float64
	TokenBudgetExceeded bool
	Passed              bool
}

// RetrievalEvaluationReport aggregates case-level retrieval quality metrics.
type RetrievalEvaluationReport struct {
	Cases                   []RetrievalEvaluationCaseResult
	MacroRecall             float64
	TotalLeakage            int
	MacroProvenanceCoverage float64
	MeanPreferenceAccuracy  float64
	MeanTokenUtilization    float64
	MeanRequiredPer1KTokens float64
	Passed                  bool
}

// EvaluateRetrieval scores compiled context bundles against deterministic
// expectations. It never performs retrieval itself; callers control the corpus,
// compiler variant, and request so results are suitable for regression gates.
func EvaluateRetrieval(cases []RetrievalEvaluationCase) RetrievalEvaluationReport {
	report := RetrievalEvaluationReport{
		Cases:  make([]RetrievalEvaluationCaseResult, 0, len(cases)),
		Passed: true,
	}
	if len(cases) == 0 {
		return report
	}

	for _, evaluationCase := range cases {
		result := evaluateRetrievalCase(evaluationCase)
		report.Cases = append(report.Cases, result)
		report.MacroRecall += result.Recall
		report.TotalLeakage += result.LeakageCount
		report.MacroProvenanceCoverage += result.ProvenanceCoverage
		report.MeanPreferenceAccuracy += result.PreferenceAccuracy
		report.MeanTokenUtilization += result.TokenUtilization
		report.MeanRequiredPer1KTokens += result.RequiredPer1KTokens
		if !result.Passed {
			report.Passed = false
		}
	}

	count := float64(len(report.Cases))
	report.MacroRecall /= count
	report.MacroProvenanceCoverage /= count
	report.MeanPreferenceAccuracy /= count
	report.MeanTokenUtilization /= count
	report.MeanRequiredPer1KTokens /= count
	return report
}

func evaluateRetrievalCase(evaluationCase RetrievalEvaluationCase) RetrievalEvaluationCaseResult {
	bundle := evaluationCase.Bundle
	expect := evaluationCase.Expect
	index := make(map[string]int, len(bundle.Items))
	items := make(map[string]contract.ContextItem, len(bundle.Items))
	for position, item := range bundle.Items {
		if _, exists := index[item.ID]; exists {
			continue
		}
		index[item.ID] = position
		items[item.ID] = item
	}

	result := RetrievalEvaluationCaseResult{
		Name:               evaluationCase.Name,
		RequiredTotal:      len(expect.RequiredIDs),
		ProvenanceRequired: len(expect.RequireProvenance),
		ForbiddenFound:     make([]string, 0),
		OrderViolations:    make([]string, 0),
	}

	for _, id := range uniqueNonEmpty(expect.RequiredIDs) {
		if _, ok := items[id]; ok {
			result.RequiredFound++
		}
	}
	result.RequiredTotal = len(uniqueNonEmpty(expect.RequiredIDs))
	if result.RequiredTotal == 0 {
		result.Recall = 1
	} else {
		result.Recall = float64(result.RequiredFound) / float64(result.RequiredTotal)
	}

	for _, id := range uniqueNonEmpty(expect.ForbiddenIDs) {
		if _, ok := items[id]; ok {
			result.ForbiddenFound = append(result.ForbiddenFound, id)
		}
	}
	result.LeakageCount = len(result.ForbiddenFound)

	provenanceIDs := uniqueNonEmpty(expect.RequireProvenance)
	result.ProvenanceRequired = len(provenanceIDs)
	for _, id := range provenanceIDs {
		item, ok := items[id]
		if !ok || item.Provenance == nil || item.Provenance.ID == "" || item.Provenance.SourceType == "" || item.Provenance.SourceType == "unresolved" {
			continue
		}
		result.ProvenanceCovered++
	}
	if result.ProvenanceRequired == 0 {
		result.ProvenanceCoverage = 1
	} else {
		result.ProvenanceCoverage = float64(result.ProvenanceCovered) / float64(result.ProvenanceRequired)
	}

	result.PreferenceTotal = len(expect.PreferredBefore)
	for _, preference := range expect.PreferredBefore {
		higher, higherOK := index[preference.HigherID]
		lower, lowerOK := index[preference.LowerID]
		if higherOK && lowerOK && higher < lower {
			continue
		}
		result.OrderViolations = append(result.OrderViolations, fmt.Sprintf("%s !< %s", preference.HigherID, preference.LowerID))
	}
	if result.PreferenceTotal == 0 {
		result.PreferenceAccuracy = 1
	} else {
		result.PreferenceAccuracy = float64(result.PreferenceTotal-len(result.OrderViolations)) / float64(result.PreferenceTotal)
	}

	if bundle.TokenBudget > 0 {
		result.TokenUtilization = float64(bundle.EstimatedTokens) / float64(bundle.TokenBudget)
		result.TokenBudgetExceeded = bundle.EstimatedTokens > bundle.TokenBudget
	}
	if bundle.EstimatedTokens > 0 {
		result.RequiredPer1KTokens = float64(result.RequiredFound) * 1000 / float64(bundle.EstimatedTokens)
	}

	result.Passed = result.Recall == 1 &&
		result.LeakageCount == 0 &&
		result.ProvenanceCoverage == 1 &&
		len(result.OrderViolations) == 0 &&
		!result.TokenBudgetExceeded
	return result
}

func uniqueNonEmpty(values []string) []string {
	seen := make(map[string]bool, len(values))
	result := make([]string, 0, len(values))
	for _, value := range values {
		if value == "" || seen[value] {
			continue
		}
		seen[value] = true
		result = append(result, value)
	}
	return result
}

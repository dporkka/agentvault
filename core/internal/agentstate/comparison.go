package agentstate

import (
	"fmt"
	"math"
	"strings"

	"github.com/agentvault/core/internal/db"
)

type ExperimentTransition string

const (
	ExperimentTransitionFixed            ExperimentTransition = "fixed"
	ExperimentTransitionRegressed        ExperimentTransition = "regressed"
	ExperimentTransitionStablePass       ExperimentTransition = "stable_pass"
	ExperimentTransitionStableFail       ExperimentTransition = "stable_fail"
	ExperimentTransitionUnclassified     ExperimentTransition = "unclassified"
	ExperimentTransitionMissingBaseline  ExperimentTransition = "missing_baseline"
	ExperimentTransitionMissingCandidate ExperimentTransition = "missing_candidate"
)

type ExperimentCaseComparison struct {
	CaseID         string
	CaseName       string
	Transition     ExperimentTransition
	BaselineRunID  string
	CandidateRunID string
	BaselineLabel  string
	CandidateLabel string
	BaselineScore  *float64
	CandidateScore *float64
	ScoreDelta     *float64
}

type ExperimentComparisonSummary struct {
	TotalCases       int
	PairedResults    int
	Fixes            int
	Regressions      int
	StablePass       int
	StableFail       int
	Unclassified     int
	MissingBaseline  int
	MissingCandidate int
}

type ExperimentComparison struct {
	BaselineExperimentID   string
	CandidateExperimentID  string
	DatasetID              string
	AgentID                string
	BaselineAgentRevision  int
	CandidateAgentRevision int
	Comparable             bool
	ReasonCodes            []string
	Summary                ExperimentComparisonSummary
	Cases                  []ExperimentCaseComparison
}

// CompareExperiments compares two persisted experiment result sets without
// executing anything. Experiments must target the same dataset and agent.
// Numeric scores are reported only as raw deltas; semantic fix/regression
// classification comes exclusively from explicit categorical labels.
func CompareExperiments(database *db.DB, baselineID, candidateID string) (*ExperimentComparison, error) {
	baselineID = strings.TrimSpace(baselineID)
	candidateID = strings.TrimSpace(candidateID)
	if baselineID == "" || candidateID == "" {
		return nil, fmt.Errorf("%w: baseline and candidate experiment ids are required", ErrInvalid)
	}
	if baselineID == candidateID {
		return nil, fmt.Errorf("%w: baseline and candidate experiments must differ", ErrInvalid)
	}

	baseline, err := GetExperiment(database, baselineID)
	if err != nil {
		return nil, err
	}
	candidate, err := GetExperiment(database, candidateID)
	if err != nil {
		return nil, err
	}

	if baseline.DatasetID != candidate.DatasetID {
		return nil, fmt.Errorf(
			"%w: experiments use different datasets (%s vs %s)",
			ErrConflict, baseline.DatasetID, candidate.DatasetID,
		)
	}
	if baseline.AgentID != candidate.AgentID {
		return nil, fmt.Errorf(
			"%w: experiments use different agents (%s vs %s)",
			ErrConflict, baseline.AgentID, candidate.AgentID,
		)
	}

	dataset, err := GetEvaluationDataset(database, baseline.DatasetID)
	if err != nil {
		return nil, err
	}

	comparison := &ExperimentComparison{
		BaselineExperimentID:   baseline.ID,
		CandidateExperimentID:  candidate.ID,
		DatasetID:              baseline.DatasetID,
		AgentID:                baseline.AgentID,
		BaselineAgentRevision:  baseline.AgentRevision,
		CandidateAgentRevision: candidate.AgentRevision,
		Comparable:             true,
		ReasonCodes:            []string{},
		Cases:                  make([]ExperimentCaseComparison, 0, len(dataset.Cases)),
	}
	if baseline.Status != ExperimentCompleted {
		comparison.ReasonCodes = append(comparison.ReasonCodes, "baseline_not_completed")
	}
	if candidate.Status != ExperimentCompleted {
		comparison.ReasonCodes = append(comparison.ReasonCodes, "candidate_not_completed")
	}

	baselineResults := make(map[string]ExperimentResult, len(baseline.Results))
	for _, result := range baseline.Results {
		baselineResults[result.CaseID] = result
	}
	candidateResults := make(map[string]ExperimentResult, len(candidate.Results))
	for _, result := range candidate.Results {
		candidateResults[result.CaseID] = result
	}

	comparison.Summary.TotalCases = len(dataset.Cases)
	for _, evaluationCase := range dataset.Cases {
		item := ExperimentCaseComparison{
			CaseID:   evaluationCase.ID,
			CaseName: evaluationCase.Name,
		}

		baselineResult, hasBaseline := baselineResults[evaluationCase.ID]
		candidateResult, hasCandidate := candidateResults[evaluationCase.ID]

		switch {
		case !hasBaseline:
			item.Transition = ExperimentTransitionMissingBaseline
			comparison.Summary.MissingBaseline++
		case !hasCandidate:
			item.Transition = ExperimentTransitionMissingCandidate
			comparison.Summary.MissingCandidate++
		default:
			comparison.Summary.PairedResults++
			item.BaselineRunID = baselineResult.RunID
			item.CandidateRunID = candidateResult.RunID
			item.BaselineLabel = baselineResult.Label
			item.CandidateLabel = candidateResult.Label
			item.BaselineScore = baselineResult.Score
			item.CandidateScore = candidateResult.Score
			item.ScoreDelta = scoreDelta(baselineResult.Score, candidateResult.Score)
			item.Transition = compareExplicitLabels(baselineResult.Label, candidateResult.Label)
			switch item.Transition {
			case ExperimentTransitionFixed:
				comparison.Summary.Fixes++
			case ExperimentTransitionRegressed:
				comparison.Summary.Regressions++
			case ExperimentTransitionStablePass:
				comparison.Summary.StablePass++
			case ExperimentTransitionStableFail:
				comparison.Summary.StableFail++
			default:
				comparison.Summary.Unclassified++
			}
		}

		comparison.Cases = append(comparison.Cases, item)
	}

	return comparison, nil
}

type explicitOutcome int

const (
	outcomeUnknown explicitOutcome = iota
	outcomePass
	outcomeFail
)

func classifyExplicitOutcome(label string) explicitOutcome {
	switch strings.ToLower(strings.TrimSpace(label)) {
	case "pass", "passed", "success", "succeeded", "correct", "accepted":
		return outcomePass
	case "fail", "failed", "error", "incorrect", "regression", "reject", "rejected", "needs_improvement", "needs-improvement":
		return outcomeFail
	default:
		return outcomeUnknown
	}
}

func compareExplicitLabels(baseline, candidate string) ExperimentTransition {
	before := classifyExplicitOutcome(baseline)
	after := classifyExplicitOutcome(candidate)

	switch {
	case before == outcomeFail && after == outcomePass:
		return ExperimentTransitionFixed
	case before == outcomePass && after == outcomeFail:
		return ExperimentTransitionRegressed
	case before == outcomePass && after == outcomePass:
		return ExperimentTransitionStablePass
	case before == outcomeFail && after == outcomeFail:
		return ExperimentTransitionStableFail
	default:
		return ExperimentTransitionUnclassified
	}
}

func scoreDelta(baseline, candidate *float64) *float64 {
	if baseline == nil || candidate == nil {
		return nil
	}
	value := math.Round((*candidate-*baseline)*1e12) / 1e12
	return &value
}

package agentstate

import (
	"fmt"
	"reflect"
	"strings"

	"github.com/agentvault/core/internal/db"
)

// RegressionCaseProposal is a read-only projection of one failed run into the
// shape needed for a reusable evaluation case. It preserves original run input
// and explicit evaluator expectations without inventing expected behavior.
type RegressionCaseProposal struct {
	RunID                string
	AgentID              string
	AgentRevision        int
	Eligible             bool
	SupportLevel         LearningSupportLevel
	Name                 string
	Input                map[string]any
	Expected             map[string]any
	Tags                 []string
	ReasonCodes          []string
	SourceObservationIDs []string
	SourceEvaluationIDs  []string
}

// RunRegressionCaseCapture is the explicit write request that turns a proposal
// into one dataset case. DatasetID is caller-selected; identity and evidence
// lineage are always derived from the persisted run.
type RunRegressionCaseCapture struct {
	RunID     string
	DatasetID string
	Name      string
	Expected  map[string]any
	Tags      []string
}

// RecommendRunRegressionCase builds a deterministic regression-case proposal.
// It only accepts expected behavior explicitly supplied by negative evaluation
// metadata under expected / expected_output / expectedOutput.
func RecommendRunRegressionCase(database *db.DB, runID string) (*RegressionCaseProposal, error) {
	learning, err := RecommendRunLearning(database, runID)
	if err != nil {
		return nil, err
	}
	audit, err := GetRunAudit(database, runID)
	if err != nil {
		return nil, err
	}

	proposal := &RegressionCaseProposal{
		RunID:                learning.RunID,
		AgentID:              learning.AgentID,
		AgentRevision:        learning.AgentRevision,
		Eligible:             learning.Eligible,
		SupportLevel:         learning.SupportLevel,
		Name:                 regressionCaseName(audit.Run.Task),
		Input:                copyObject(audit.Run.Input),
		Tags:                 []string{"regression"},
		ReasonCodes:          append([]string{}, learning.ReasonCodes...),
		SourceObservationIDs: append([]string{}, learning.SourceObservationIDs...),
		SourceEvaluationIDs:  append([]string{}, learning.SourceEvaluationIDs...),
	}

	sourceEvaluation := make(map[string]struct{}, len(learning.SourceEvaluationIDs))
	for _, id := range learning.SourceEvaluationIDs {
		sourceEvaluation[id] = struct{}{}
	}

	var expected map[string]any
	conflictingExpected := false
	for _, evaluation := range audit.Evaluations {
		if _, ok := sourceEvaluation[evaluation.ID]; !ok {
			continue
		}
		hint, ok := expectedBehaviorHint(evaluation.Metadata)
		if !ok {
			continue
		}
		if expected == nil {
			expected = copyObject(hint)
			continue
		}
		if !reflect.DeepEqual(expected, hint) {
			conflictingExpected = true
		}
	}

	switch {
	case conflictingExpected:
		proposal.ReasonCodes = appendUnique(proposal.ReasonCodes, "conflicting_expected_hints")
	case expected == nil:
		proposal.ReasonCodes = appendUnique(proposal.ReasonCodes, "no_expected_hint")
	default:
		proposal.Expected = expected
	}

	return proposal, nil
}

// CaptureRunRegressionCase explicitly stores an eligible run as one reusable
// dataset case. Repeating the same dataset+run capture returns the existing
// case so callers can safely retry.
func CaptureRunRegressionCase(database *db.DB, capture RunRegressionCaseCapture) (*EvaluationCase, error) {
	capture.RunID = strings.TrimSpace(capture.RunID)
	capture.DatasetID = strings.TrimSpace(capture.DatasetID)
	capture.Name = strings.TrimSpace(capture.Name)
	if capture.RunID == "" || capture.DatasetID == "" {
		return nil, fmt.Errorf("%w: run id and dataset id are required", ErrInvalid)
	}

	proposal, err := RecommendRunRegressionCase(database, capture.RunID)
	if err != nil {
		return nil, err
	}
	if !proposal.Eligible {
		return nil, fmt.Errorf("%w: run %s is not eligible for regression capture", ErrConflict, capture.RunID)
	}

	dataset, err := GetEvaluationDataset(database, capture.DatasetID)
	if err != nil {
		return nil, err
	}
	if dataset.AgentID != "" && dataset.AgentID != proposal.AgentID {
		return nil, fmt.Errorf(
			"%w: dataset %s belongs to agent %s, run belongs to %s",
			ErrConflict, dataset.ID, dataset.AgentID, proposal.AgentID,
		)
	}

	for i := range dataset.Cases {
		if dataset.Cases[i].SourceRunID == capture.RunID {
			item := dataset.Cases[i]
			return &item, nil
		}
	}

	name := proposal.Name
	if capture.Name != "" {
		name = capture.Name
	}
	expected := proposal.Expected
	if capture.Expected != nil {
		expected = copyObject(capture.Expected)
	}
	tags := append([]string{}, proposal.Tags...)
	for _, tag := range capture.Tags {
		tags = appendUnique(tags, tag)
	}

	return AddEvaluationCase(database, EvaluationCase{
		DatasetID:            capture.DatasetID,
		Name:                 name,
		Input:                copyObject(proposal.Input),
		Expected:             copyObject(expected),
		Tags:                 tags,
		SourceRunID:          proposal.RunID,
		SourceObservationIDs: append([]string{}, proposal.SourceObservationIDs...),
		SourceEvaluationIDs:  append([]string{}, proposal.SourceEvaluationIDs...),
		AgentID:              proposal.AgentID,
		AgentRevision:        proposal.AgentRevision,
	})
}

func regressionCaseName(task string) string {
	task = strings.TrimSpace(task)
	if task == "" {
		return "Regression case"
	}
	return "Regression: " + task
}

func expectedBehaviorHint(metadata map[string]any) (map[string]any, bool) {
	for _, key := range []string{"expected", "expected_output", "expectedOutput"} {
		value, ok := metadata[key]
		if !ok {
			continue
		}
		object, ok := value.(map[string]any)
		if !ok || object == nil {
			return nil, false
		}
		return object, true
	}
	return nil, false
}

func copyObject(value map[string]any) map[string]any {
	if value == nil {
		return nil
	}
	out := make(map[string]any, len(value))
	for key, item := range value {
		out[key] = item
	}
	return out
}

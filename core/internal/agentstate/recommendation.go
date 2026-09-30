package agentstate

import (
	"fmt"
	"strings"

	"github.com/agentvault/core/internal/db"
)

type LearningSupportLevel string

const (
	LearningSupportNone     LearningSupportLevel = "none"
	LearningSupportWeak     LearningSupportLevel = "weak"
	LearningSupportModerate LearningSupportLevel = "moderate"
	LearningSupportStrong   LearningSupportLevel = "strong"
)

type LearningSignalKind string

const (
	LearningSignalObservation LearningSignalKind = "observation"
	LearningSignalEvaluation  LearningSignalKind = "evaluation"
)

// LearningSignal is one explicit piece of negative evidence surfaced from an
// audited run. It preserves the evaluator/runtime wording rather than
// synthesizing a candidate memory statement.
type LearningSignal struct {
	Kind          LearningSignalKind
	ID            string
	ObservationID string
	Name          string
	Status        string
	Label         string
	Score         *float64
	Rationale     string
}

// LearningRecommendation is a deterministic set of candidate inputs for an
// explicit promotion decision. It never contains generated candidate text.
type LearningRecommendation struct {
	RunID                string
	AgentID              string
	AgentRevision        int
	Eligible             bool
	SupportLevel         LearningSupportLevel
	EvidenceCount        int
	SuggestedTargetKind  PromotionTargetKind
	ReasonCodes          []string
	SourceObservationIDs []string
	SourceEvaluationIDs  []string
	ContextMemoryRefs    []string
	SupersedesNoteIDs    []string
	Signals              []LearningSignal
}

// RecommendRunLearning inspects immutable run evidence and surfaces only
// explicit failure signals. Numeric evaluation scores are intentionally not
// interpreted because AgentVault does not know an evaluator's score scale or
// threshold unless the evaluator expresses a categorical failure label.
func RecommendRunLearning(database *db.DB, runID string) (*LearningRecommendation, error) {
	audit, err := GetRunAudit(database, runID)
	if err != nil {
		return nil, err
	}

	rec := &LearningRecommendation{
		RunID:                audit.Run.ID,
		AgentID:              audit.Run.AgentID,
		AgentRevision:        audit.Run.AgentRevision,
		SupportLevel:         LearningSupportNone,
		ReasonCodes:          []string{},
		SourceObservationIDs: []string{},
		SourceEvaluationIDs:  []string{},
		ContextMemoryRefs:    []string{},
		SupersedesNoteIDs:    []string{},
		Signals:              []LearningSignal{},
	}

	if audit.Run.Status == RunFailed {
		rec.ReasonCodes = append(rec.ReasonCodes, "run_failed")
	}
	if audit.Context != nil {
		for _, section := range audit.Context.Sections {
			if section.Kind == ContextMemory && strings.TrimSpace(section.SourceID) != "" {
				rec.ContextMemoryRefs = appendUnique(rec.ContextMemoryRefs, strings.TrimSpace(section.SourceID))
			}
		}
	}

	observationByID := make(map[string]RunObservationRecord, len(audit.Observations))
	failedObservationCount := 0
	for _, observation := range audit.Observations {
		observationByID[observation.ID] = observation
		if !isFailedObservationStatus(observation.Status) {
			continue
		}
		failedObservationCount++
		rec.SourceObservationIDs = appendUnique(rec.SourceObservationIDs, observation.ID)
		rec.Signals = append(rec.Signals, LearningSignal{
			Kind:   LearningSignalObservation,
			ID:     observation.ID,
			Name:   observation.Name,
			Status: observation.Status,
		})
	}

	negativeEvaluationCount := 0
	targetHints := []PromotionTargetKind{}
	for _, evaluation := range audit.Evaluations {
		if !isNegativeEvaluationLabel(evaluation.Label) {
			continue
		}
		negativeEvaluationCount++
		rec.SourceEvaluationIDs = appendUnique(rec.SourceEvaluationIDs, evaluation.ID)
		if evaluation.ObservationID != "" {
			if _, ok := observationByID[evaluation.ObservationID]; ok {
				rec.SourceObservationIDs = appendUnique(rec.SourceObservationIDs, evaluation.ObservationID)
			}
		}
		rec.Signals = append(rec.Signals, LearningSignal{
			Kind:          LearningSignalEvaluation,
			ID:            evaluation.ID,
			ObservationID: evaluation.ObservationID,
			Name:          evaluation.Name,
			Label:         evaluation.Label,
			Score:         evaluation.Score,
			Rationale:     evaluation.Rationale,
		})

		if hint, ok := promotionTargetHint(evaluation.Metadata); ok {
			targetHints = append(targetHints, hint)
		}
		for _, noteID := range supersessionHints(evaluation.Metadata) {
			rec.SupersedesNoteIDs = appendUnique(rec.SupersedesNoteIDs, noteID)
		}
	}

	if failedObservationCount > 0 {
		rec.ReasonCodes = append(rec.ReasonCodes, "failed_observation")
	}
	if negativeEvaluationCount > 0 {
		rec.ReasonCodes = append(rec.ReasonCodes, "negative_evaluation")
	}

	rec.EvidenceCount = len(rec.SourceObservationIDs) + len(rec.SourceEvaluationIDs)
	switch {
	case negativeEvaluationCount >= 2 || (negativeEvaluationCount >= 1 && failedObservationCount >= 1):
		rec.SupportLevel = LearningSupportStrong
	case negativeEvaluationCount >= 1:
		rec.SupportLevel = LearningSupportModerate
	case failedObservationCount >= 1:
		rec.SupportLevel = LearningSupportWeak
	default:
		rec.SupportLevel = LearningSupportNone
		rec.ReasonCodes = append(rec.ReasonCodes, "no_negative_evidence")
	}

	if len(targetHints) > 0 {
		first := targetHints[0]
		consistent := true
		for _, hint := range targetHints[1:] {
			if hint != first {
				consistent = false
				break
			}
		}
		if consistent {
			rec.SuggestedTargetKind = first
		} else {
			rec.ReasonCodes = append(rec.ReasonCodes, "conflicting_target_kind_hints")
		}
	}

	bound := audit.Run.AgentID != "" && audit.Run.AgentRevision >= 1
	if !bound {
		rec.ReasonCodes = append(rec.ReasonCodes, "run_unbound")
	}
	rec.Eligible = bound && rec.EvidenceCount > 0

	return rec, nil
}

func isFailedObservationStatus(status string) bool {
	switch strings.ToLower(strings.TrimSpace(status)) {
	case "failed", "error":
		return true
	default:
		return false
	}
}

func isNegativeEvaluationLabel(label string) bool {
	switch strings.ToLower(strings.TrimSpace(label)) {
	case "fail", "failed", "error", "incorrect", "regression", "reject", "rejected", "needs_improvement", "needs-improvement":
		return true
	default:
		return false
	}
}

func promotionTargetHint(metadata map[string]any) (PromotionTargetKind, bool) {
	for _, key := range []string{"target_kind", "targetKind"} {
		raw, ok := metadata[key]
		if !ok {
			continue
		}
		value, ok := raw.(string)
		if !ok {
			return "", false
		}
		switch PromotionTargetKind(strings.TrimSpace(value)) {
		case PromotionMemory:
			return PromotionMemory, true
		case PromotionKnowledge:
			return PromotionKnowledge, true
		default:
			return "", false
		}
	}
	return "", false
}

func supersessionHints(metadata map[string]any) []string {
	out := []string{}
	for _, key := range []string{"supersedes_note_id", "supersedesNoteId"} {
		if value, ok := metadata[key].(string); ok {
			out = appendUnique(out, strings.TrimSpace(value))
		}
	}
	for _, key := range []string{"supersedes_note_ids", "supersedesNoteIds"} {
		switch values := metadata[key].(type) {
		case []string:
			for _, value := range values {
				out = appendUnique(out, strings.TrimSpace(value))
			}
		case []any:
			for _, raw := range values {
				if value, ok := raw.(string); ok {
					out = appendUnique(out, strings.TrimSpace(value))
				}
			}
		}
	}
	return out
}

func appendUnique(values []string, value string) []string {
	value = strings.TrimSpace(value)
	if value == "" {
		return values
	}
	for _, existing := range values {
		if existing == value {
			return values
		}
	}
	return append(values, value)
}

func (r LearningRecommendation) ValidateForProposal() error {
	if !r.Eligible {
		return fmt.Errorf("%w: recommendation is not eligible for promotion", ErrConflict)
	}
	if r.AgentID == "" || r.AgentRevision < 1 {
		return fmt.Errorf("%w: recommendation has no canonical agent binding", ErrConflict)
	}
	if r.EvidenceCount == 0 {
		return fmt.Errorf("%w: recommendation has no negative evidence", ErrConflict)
	}
	return nil
}

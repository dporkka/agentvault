package agentstate

import (
	"database/sql"
	"errors"
	"fmt"
	"strings"

	"github.com/agentvault/core/internal/db"
)

// RunLearningCandidate is a reviewable learning proposal derived from one
// recorded run. AgentID is intentionally absent: it is derived from the run so
// callers cannot attach learning to a different agent than the evidence.
type RunLearningCandidate struct {
	RunID                string
	TargetKind           PromotionTargetKind
	Candidate            string
	Rationale            string
	SourceObservationIDs []string
	SourceEvaluationIDs  []string
	SupersedesNoteID     string
}

// ProposeRunLearningCandidate closes the evidence-to-learning link without
// automatically mutating long-term memory. It validates all selected evidence
// belongs to the originating run, derives the canonical agent ID from that run,
// and creates an ordinary proposed promotion for explicit review.
func ProposeRunLearningCandidate(database *db.DB, candidate RunLearningCandidate) (*PromotionRecord, error) {
	candidate.RunID = strings.TrimSpace(candidate.RunID)
	candidate.Candidate = strings.TrimSpace(candidate.Candidate)
	candidate.Rationale = strings.TrimSpace(candidate.Rationale)
	candidate.SupersedesNoteID = strings.TrimSpace(candidate.SupersedesNoteID)
	candidate.SourceObservationIDs = uniqueNonEmpty(candidate.SourceObservationIDs)
	candidate.SourceEvaluationIDs = uniqueNonEmpty(candidate.SourceEvaluationIDs)

	if candidate.RunID == "" {
		return nil, fmt.Errorf("%w: run id is required", ErrInvalid)
	}
	if candidate.TargetKind != PromotionMemory && candidate.TargetKind != PromotionKnowledge {
		return nil, fmt.Errorf("%w: unknown promotion target kind %q", ErrInvalid, candidate.TargetKind)
	}
	if candidate.Candidate == "" {
		return nil, fmt.Errorf("%w: learning candidate text is required", ErrInvalid)
	}

	run, err := getRun(database, candidate.RunID)
	if err != nil {
		return nil, err
	}
	if run.AgentID == "" || run.AgentRevision < 1 {
		return nil, fmt.Errorf("%w: run %s has no canonical agent binding", ErrConflict, candidate.RunID)
	}

	for _, observationID := range candidate.SourceObservationIDs {
		if err := evidenceBelongsToRun(database, "run_observations", observationID, candidate.RunID); err != nil {
			return nil, fmt.Errorf("observation %s: %w", observationID, err)
		}
	}
	for _, evaluationID := range candidate.SourceEvaluationIDs {
		if err := evidenceBelongsToRun(database, "evaluations", evaluationID, candidate.RunID); err != nil {
			return nil, fmt.Errorf("evaluation %s: %w", evaluationID, err)
		}
	}

	return ProposePromotion(database, PromotionRecord{
		AgentID:              run.AgentID,
		TargetKind:           candidate.TargetKind,
		Candidate:            candidate.Candidate,
		Rationale:            candidate.Rationale,
		SourceRunIDs:         []string{run.ID},
		SourceObservationIDs: candidate.SourceObservationIDs,
		SourceEvaluationIDs:  candidate.SourceEvaluationIDs,
		SupersedesNoteID:     candidate.SupersedesNoteID,
	})
}

func evidenceBelongsToRun(database *db.DB, table, evidenceID, runID string) error {
	if evidenceID == "" {
		return fmt.Errorf("%w: evidence id is required", ErrInvalid)
	}

	var evidenceRunID string
	query := ""
	switch table {
	case "run_observations":
		query = "SELECT run_id FROM run_observations WHERE id = ?"
	case "evaluations":
		query = "SELECT run_id FROM evaluations WHERE id = ?"
	default:
		return fmt.Errorf("%w: unsupported evidence table", ErrInvalid)
	}

	err := database.QueryRow(query, evidenceID).Scan(&evidenceRunID)
	if err != nil {
		if errors.Is(err, sql.ErrNoRows) {
			return fmt.Errorf("%w: %s", ErrNotFound, evidenceID)
		}
		return fmt.Errorf("%w: lookup %s: %v", ErrStorage, evidenceID, err)
	}
	if evidenceRunID != runID {
		return fmt.Errorf("%w: evidence belongs to run %s, not %s", ErrConflict, evidenceRunID, runID)
	}
	return nil
}

func uniqueNonEmpty(values []string) []string {
	if len(values) == 0 {
		return []string{}
	}
	seen := make(map[string]struct{}, len(values))
	out := make([]string, 0, len(values))
	for _, value := range values {
		value = strings.TrimSpace(value)
		if value == "" {
			continue
		}
		if _, ok := seen[value]; ok {
			continue
		}
		seen[value] = struct{}{}
		out = append(out, value)
	}
	return out
}

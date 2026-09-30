package agentstate

import (
	"database/sql"
	"encoding/json"
	"errors"
	"fmt"
	"time"

	"github.com/agentvault/core/internal/db"
)

var (
	ErrInvalid  = errors.New("invalid agent state")
	ErrNotFound = errors.New("agent state not found")
	ErrConflict = errors.New("agent state conflict")
	ErrStorage  = errors.New("agent state storage failure")
)

func nowTimestamp() string {
	return time.Now().UTC().Format(time.RFC3339)
}

func newID(prefix string) string {
	return fmt.Sprintf("%s_%d", prefix, time.Now().UnixNano())
}

// GetPromotion fetches one promotion record by ID.
func GetPromotion(database *db.DB, id string) (*PromotionRecord, error) {
	if id == "" {
		return nil, fmt.Errorf("%w: promotion id is required", ErrInvalid)
	}

	var p PromotionRecord
	var runJSON, observationJSON, evaluationJSON string
	err := database.QueryRow(`
		SELECT
			id, agent_id, target_kind, status, candidate, COALESCE(rationale, ''),
			source_run_ids_json, source_observation_ids_json, source_evaluation_ids_json,
			COALESCE(target_note_id, ''), COALESCE(supersedes_note_id, ''),
			created_at, COALESCE(reviewed_at, ''), COALESCE(reviewed_by, ''),
			COALESCE(review_note, ''), COALESCE(committed_at, '')
		FROM promotion_records
		WHERE id = ?
	`, id).Scan(
		&p.ID, &p.AgentID, &p.TargetKind, &p.Status, &p.Candidate, &p.Rationale,
		&runJSON, &observationJSON, &evaluationJSON,
		&p.TargetNoteID, &p.SupersedesNoteID,
		&p.CreatedAt, &p.ReviewedAt, &p.ReviewedBy, &p.ReviewNote, &p.CommittedAt,
	)
	if err != nil {
		if errors.Is(err, sql.ErrNoRows) {
			return nil, fmt.Errorf("%w: promotion %s", ErrNotFound, id)
		}
		return nil, fmt.Errorf("%w: query promotion: %v", ErrStorage, err)
	}
	if err := decodeStringList(runJSON, &p.SourceRunIDs); err != nil {
		return nil, fmt.Errorf("%w: decode promotion %s run ids: %v", ErrStorage, id, err)
	}
	if err := decodeStringList(observationJSON, &p.SourceObservationIDs); err != nil {
		return nil, fmt.Errorf("%w: decode promotion %s observation ids: %v", ErrStorage, id, err)
	}
	if err := decodeStringList(evaluationJSON, &p.SourceEvaluationIDs); err != nil {
		return nil, fmt.Errorf("%w: decode promotion %s evaluation ids: %v", ErrStorage, id, err)
	}
	return &p, nil
}

// ProposePromotion persists an evidence-backed promotion proposal without
// mutating canonical memory or knowledge.
func ProposePromotion(database *db.DB, record PromotionRecord) (*PromotionRecord, error) {
	if record.ID == "" {
		record.ID = newID("promo")
	}
	record.Status = PromotionProposed
	record.CreatedAt = nowTimestamp()
	if err := record.Validate(); err != nil {
		return nil, fmt.Errorf("%w: %v", ErrInvalid, err)
	}

	runJSON, err := json.Marshal(record.SourceRunIDs)
	if err != nil {
		return nil, fmt.Errorf("%w: encode source runs: %v", ErrInvalid, err)
	}
	observationJSON, err := json.Marshal(record.SourceObservationIDs)
	if err != nil {
		return nil, fmt.Errorf("%w: encode source observations: %v", ErrInvalid, err)
	}
	evaluationJSON, err := json.Marshal(record.SourceEvaluationIDs)
	if err != nil {
		return nil, fmt.Errorf("%w: encode source evaluations: %v", ErrInvalid, err)
	}

	_, err = database.Exec(`
		INSERT INTO promotion_records (
			id, agent_id, target_kind, status, candidate, rationale,
			source_run_ids_json, source_observation_ids_json, source_evaluation_ids_json,
			supersedes_note_id, created_at
		) VALUES (?, ?, ?, ?, ?, NULLIF(?, ''), ?, ?, ?, NULLIF(?, ''), ?)
	`,
		record.ID, record.AgentID, string(record.TargetKind), string(record.Status),
		record.Candidate, record.Rationale, string(runJSON), string(observationJSON),
		string(evaluationJSON), record.SupersedesNoteID, record.CreatedAt,
	)
	if err != nil {
		return nil, fmt.Errorf("%w: propose promotion: %v", ErrStorage, err)
	}
	return GetPromotion(database, record.ID)
}

// ReviewPromotion applies an explicit approve/reject transition.
func ReviewPromotion(database *db.DB, promotionID string, decision PromotionStatus, reviewer, note string) (*PromotionRecord, error) {
	if promotionID == "" || reviewer == "" {
		return nil, fmt.Errorf("%w: promotion id and reviewer are required", ErrInvalid)
	}
	if decision != PromotionApproved && decision != PromotionRejected {
		return nil, fmt.Errorf("%w: review decision must be approved or rejected", ErrInvalid)
	}

	current, err := GetPromotion(database, promotionID)
	if err != nil {
		return nil, err
	}
	if err := ValidatePromotionTransition(current.Status, decision); err != nil {
		return nil, fmt.Errorf("%w: %v", ErrConflict, err)
	}

	reviewedAt := nowTimestamp()
	if _, err := database.Exec(`
		UPDATE promotion_records
		SET status = ?, reviewed_at = ?, reviewed_by = ?, review_note = NULLIF(?, '')
		WHERE id = ?
	`, string(decision), reviewedAt, reviewer, note, promotionID); err != nil {
		return nil, fmt.Errorf("%w: review promotion: %v", ErrStorage, err)
	}
	return GetPromotion(database, promotionID)
}

// CommitPromotion records the final transition after the caller has verified
// that targetNoteID is canonical Markdown containing the exact candidate text.
func CommitPromotion(database *db.DB, promotionID, targetNoteID string) (*PromotionRecord, error) {
	if promotionID == "" || targetNoteID == "" {
		return nil, fmt.Errorf("%w: promotion id and target note id are required", ErrInvalid)
	}
	current, err := GetPromotion(database, promotionID)
	if err != nil {
		return nil, err
	}
	if err := ValidatePromotionTransition(current.Status, PromotionCommitted); err != nil {
		return nil, fmt.Errorf("%w: %v", ErrConflict, err)
	}

	if _, err := database.Exec(`
		UPDATE promotion_records
		SET status = ?, target_note_id = ?, committed_at = ?
		WHERE id = ?
	`, string(PromotionCommitted), targetNoteID, nowTimestamp(), promotionID); err != nil {
		return nil, fmt.Errorf("%w: commit promotion: %v", ErrStorage, err)
	}
	return GetPromotion(database, promotionID)
}

// CreateEvaluationDataset stores a reusable set of evaluation cases.
func CreateEvaluationDataset(database *db.DB, record EvaluationDataset) (*EvaluationDataset, error) {
	if record.ID == "" {
		record.ID = newID("ds")
	}
	record.CreatedAt = nowTimestamp()
	if err := record.Validate(); err != nil {
		return nil, fmt.Errorf("%w: %v", ErrInvalid, err)
	}

	if _, err := database.Exec(`
		INSERT INTO evaluation_datasets (id, name, description, agent_id, created_at)
		VALUES (?, ?, NULLIF(?, ''), NULLIF(?, ''), ?)
	`, record.ID, record.Name, record.Description, record.AgentID, record.CreatedAt); err != nil {
		return nil, fmt.Errorf("%w: create evaluation dataset: %v", ErrStorage, err)
	}
	return &record, nil
}

// AddEvaluationCase adds a reproducible case to an existing dataset.
func AddEvaluationCase(database *db.DB, record EvaluationCase) (*EvaluationCase, error) {
	if record.DatasetID == "" {
		return nil, fmt.Errorf("%w: dataset id is required", ErrInvalid)
	}
	var one int
	if err := database.QueryRow(`SELECT 1 FROM evaluation_datasets WHERE id = ?`, record.DatasetID).Scan(&one); err != nil {
		if errors.Is(err, sql.ErrNoRows) {
			return nil, fmt.Errorf("%w: evaluation dataset %s", ErrNotFound, record.DatasetID)
		}
		return nil, fmt.Errorf("%w: lookup evaluation dataset: %v", ErrStorage, err)
	}

	if record.ID == "" {
		record.ID = newID("case")
	}
	record.CreatedAt = nowTimestamp()
	if record.Tags == nil {
		record.Tags = []string{}
	}
	if record.SourceObservationIDs == nil {
		record.SourceObservationIDs = []string{}
	}
	if record.SourceEvaluationIDs == nil {
		record.SourceEvaluationIDs = []string{}
	}
	if err := record.Validate(); err != nil {
		return nil, fmt.Errorf("%w: %v", ErrInvalid, err)
	}

	inputJSON, err := json.Marshal(record.Input)
	if err != nil {
		return nil, fmt.Errorf("%w: encode evaluation input: %v", ErrInvalid, err)
	}
	expectedJSON := ""
	if record.Expected != nil {
		b, err := json.Marshal(record.Expected)
		if err != nil {
			return nil, fmt.Errorf("%w: encode expected output: %v", ErrInvalid, err)
		}
		expectedJSON = string(b)
	}
	tagsJSON, err := json.Marshal(record.Tags)
	if err != nil {
		return nil, fmt.Errorf("%w: encode case tags: %v", ErrInvalid, err)
	}
	observationJSON, err := json.Marshal(record.SourceObservationIDs)
	if err != nil {
		return nil, fmt.Errorf("%w: encode case source observations: %v", ErrInvalid, err)
	}
	evaluationJSON, err := json.Marshal(record.SourceEvaluationIDs)
	if err != nil {
		return nil, fmt.Errorf("%w: encode case source evaluations: %v", ErrInvalid, err)
	}

	if _, err := database.Exec(`
		INSERT INTO evaluation_cases (
			id, dataset_id, name, input_json, expected_json, tags_json,
			source_run_id, source_observation_ids_json, source_evaluation_ids_json,
			agent_id, agent_revision, created_at
		) VALUES (
			?, ?, ?, ?, NULLIF(?, ''), ?,
			NULLIF(?, ''), ?, ?, NULLIF(?, ''), NULLIF(?, 0), ?
		)
	`,
		record.ID, record.DatasetID, record.Name, string(inputJSON), expectedJSON, string(tagsJSON),
		record.SourceRunID, string(observationJSON), string(evaluationJSON),
		record.AgentID, record.AgentRevision, record.CreatedAt,
	); err != nil {
		return nil, fmt.Errorf("%w: add evaluation case: %v", ErrStorage, err)
	}
	return &record, nil
}

// RecordExperiment records an experiment executed by an external runtime.
func RecordExperiment(database *db.DB, record Experiment) (*Experiment, error) {
	if record.DatasetID == "" {
		return nil, fmt.Errorf("%w: dataset id is required", ErrInvalid)
	}
	var one int
	if err := database.QueryRow(`SELECT 1 FROM evaluation_datasets WHERE id = ?`, record.DatasetID).Scan(&one); err != nil {
		if errors.Is(err, sql.ErrNoRows) {
			return nil, fmt.Errorf("%w: evaluation dataset %s", ErrNotFound, record.DatasetID)
		}
		return nil, fmt.Errorf("%w: lookup evaluation dataset: %v", ErrStorage, err)
	}

	if record.ID == "" {
		record.ID = newID("exp")
	}
	if record.Status == "" {
		record.Status = ExperimentCompleted
	}
	if record.Config == nil {
		record.Config = map[string]any{}
	}
	record.CreatedAt = nowTimestamp()
	if record.Status == ExperimentCompleted || record.Status == ExperimentFailed || record.Status == ExperimentCancelled {
		record.CompletedAt = record.CreatedAt
	}
	if err := record.Validate(); err != nil {
		return nil, fmt.Errorf("%w: %v", ErrInvalid, err)
	}

	configJSON, err := json.Marshal(record.Config)
	if err != nil {
		return nil, fmt.Errorf("%w: encode experiment config: %v", ErrInvalid, err)
	}
	if _, err := database.Exec(`
		INSERT INTO experiments (
			id, dataset_id, name, agent_id, agent_revision, status, config_json, created_at, completed_at
		) VALUES (?, ?, ?, ?, ?, ?, ?, ?, NULLIF(?, ''))
	`,
		record.ID, record.DatasetID, record.Name, record.AgentID, record.AgentRevision,
		string(record.Status), string(configJSON), record.CreatedAt, record.CompletedAt,
	); err != nil {
		return nil, fmt.Errorf("%w: record experiment: %v", ErrStorage, err)
	}
	return &record, nil
}

// RecordExperimentResult records a case-level result and enforces that the case
// belongs to the experiment's dataset.
func RecordExperimentResult(database *db.DB, record ExperimentResult) (*ExperimentResult, error) {
	if record.Metadata == nil {
		record.Metadata = map[string]any{}
	}
	if err := record.Validate(); err != nil {
		return nil, fmt.Errorf("%w: %v", ErrInvalid, err)
	}

	var experimentDataset, caseDataset string
	if err := database.QueryRow(`SELECT dataset_id FROM experiments WHERE id = ?`, record.ExperimentID).Scan(&experimentDataset); err != nil {
		if errors.Is(err, sql.ErrNoRows) {
			return nil, fmt.Errorf("%w: experiment %s", ErrNotFound, record.ExperimentID)
		}
		return nil, fmt.Errorf("%w: lookup experiment: %v", ErrStorage, err)
	}
	if err := database.QueryRow(`SELECT dataset_id FROM evaluation_cases WHERE id = ?`, record.CaseID).Scan(&caseDataset); err != nil {
		if errors.Is(err, sql.ErrNoRows) {
			return nil, fmt.Errorf("%w: evaluation case %s", ErrNotFound, record.CaseID)
		}
		return nil, fmt.Errorf("%w: lookup evaluation case: %v", ErrStorage, err)
	}
	if experimentDataset != caseDataset {
		return nil, fmt.Errorf("%w: evaluation case dataset does not match experiment dataset", ErrConflict)
	}

	record.CreatedAt = nowTimestamp()
	metadataJSON, err := json.Marshal(record.Metadata)
	if err != nil {
		return nil, fmt.Errorf("%w: encode result metadata: %v", ErrInvalid, err)
	}
	var score interface{}
	if record.Score != nil {
		score = *record.Score
	}
	if _, err := database.Exec(`
		INSERT INTO experiment_results (
			experiment_id, case_id, run_id, score, label, metadata_json, created_at
		) VALUES (?, ?, NULLIF(?, ''), ?, NULLIF(?, ''), ?, ?)
	`,
		record.ExperimentID, record.CaseID, record.RunID, score, record.Label,
		string(metadataJSON), record.CreatedAt,
	); err != nil {
		return nil, fmt.Errorf("%w: record experiment result: %v", ErrStorage, err)
	}
	return &record, nil
}

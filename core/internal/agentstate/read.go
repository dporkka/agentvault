package agentstate

import (
	"database/sql"
	"encoding/json"
	"fmt"

	"github.com/agentvault/core/internal/db"
)

type EvaluationDatasetDetail struct {
	EvaluationDataset
	Cases []EvaluationCase
}

type ExperimentDetail struct {
	Experiment
	Results []ExperimentResult
}

func ListPromotions(database *db.DB, status, agentID string, limit int) ([]PromotionRecord, error) {
	if status == "" {
		status = string(PromotionProposed)
	}
	switch status {
	case "all", string(PromotionProposed), string(PromotionApproved), string(PromotionRejected), string(PromotionCommitted), string(PromotionSuperseded):
	default:
		return nil, fmt.Errorf("invalid promotion status %q", status)
	}
	if limit <= 0 {
		limit = 50
	}
	if limit > 100 {
		limit = 100
	}

	rows, err := database.Query(`
		SELECT
			id, agent_id, target_kind, status, candidate, COALESCE(rationale, ''),
			source_run_ids_json, source_observation_ids_json, source_evaluation_ids_json,
			COALESCE(target_note_id, ''), COALESCE(supersedes_note_id, ''),
			created_at, COALESCE(reviewed_at, ''), COALESCE(reviewed_by, ''),
			COALESCE(review_note, ''), COALESCE(committed_at, '')
		FROM promotion_records
		WHERE (? = 'all' OR status = ?)
		  AND (? = '' OR agent_id = ?)
		ORDER BY created_at DESC, id DESC
		LIMIT ?
	`, status, status, agentID, agentID, limit)
	if err != nil {
		return nil, fmt.Errorf("query promotions: %w", err)
	}
	defer rows.Close()

	promotions := []PromotionRecord{}
	for rows.Next() {
		var p PromotionRecord
		var runJSON, observationJSON, evaluationJSON string
		if err := rows.Scan(
			&p.ID, &p.AgentID, &p.TargetKind, &p.Status, &p.Candidate, &p.Rationale,
			&runJSON, &observationJSON, &evaluationJSON,
			&p.TargetNoteID, &p.SupersedesNoteID,
			&p.CreatedAt, &p.ReviewedAt, &p.ReviewedBy, &p.ReviewNote, &p.CommittedAt,
		); err != nil {
			return nil, fmt.Errorf("scan promotion: %w", err)
		}
		if err := decodeStringList(runJSON, &p.SourceRunIDs); err != nil {
			return nil, fmt.Errorf("decode promotion %s run ids: %w", p.ID, err)
		}
		if err := decodeStringList(observationJSON, &p.SourceObservationIDs); err != nil {
			return nil, fmt.Errorf("decode promotion %s observation ids: %w", p.ID, err)
		}
		if err := decodeStringList(evaluationJSON, &p.SourceEvaluationIDs); err != nil {
			return nil, fmt.Errorf("decode promotion %s evaluation ids: %w", p.ID, err)
		}
		promotions = append(promotions, p)
	}
	if err := rows.Err(); err != nil {
		return nil, fmt.Errorf("iterate promotions: %w", err)
	}
	return promotions, nil
}

func GetEvaluationDataset(database *db.DB, id string) (*EvaluationDatasetDetail, error) {
	var detail EvaluationDatasetDetail
	err := database.QueryRow(`
		SELECT id, name, COALESCE(description, ''), COALESCE(agent_id, ''), created_at
		FROM evaluation_datasets
		WHERE id = ?
	`, id).Scan(
		&detail.ID, &detail.Name, &detail.Description, &detail.AgentID, &detail.CreatedAt,
	)
	if err != nil {
		if err == sql.ErrNoRows {
			return nil, fmt.Errorf("evaluation dataset not found: %s", id)
		}
		return nil, fmt.Errorf("query evaluation dataset: %w", err)
	}

	rows, err := database.Query(`
		SELECT id, dataset_id, name, input_json, COALESCE(expected_json, ''), tags_json, created_at
		FROM evaluation_cases
		WHERE dataset_id = ?
		ORDER BY created_at, id
	`, id)
	if err != nil {
		return nil, fmt.Errorf("query evaluation cases: %w", err)
	}
	defer rows.Close()

	detail.Cases = []EvaluationCase{}
	for rows.Next() {
		var item EvaluationCase
		var inputJSON, expectedJSON, tagsJSON string
		if err := rows.Scan(
			&item.ID, &item.DatasetID, &item.Name, &inputJSON, &expectedJSON, &tagsJSON, &item.CreatedAt,
		); err != nil {
			return nil, fmt.Errorf("scan evaluation case: %w", err)
		}
		if err := decodeObject(inputJSON, &item.Input); err != nil {
			return nil, fmt.Errorf("decode evaluation case %s input: %w", item.ID, err)
		}
		if expectedJSON != "" {
			if err := decodeObject(expectedJSON, &item.Expected); err != nil {
				return nil, fmt.Errorf("decode evaluation case %s expected output: %w", item.ID, err)
			}
		}
		if err := decodeStringList(tagsJSON, &item.Tags); err != nil {
			return nil, fmt.Errorf("decode evaluation case %s tags: %w", item.ID, err)
		}
		detail.Cases = append(detail.Cases, item)
	}
	if err := rows.Err(); err != nil {
		return nil, fmt.Errorf("iterate evaluation cases: %w", err)
	}
	return &detail, nil
}

func GetExperiment(database *db.DB, id string) (*ExperimentDetail, error) {
	var detail ExperimentDetail
	var configJSON string
	err := database.QueryRow(`
		SELECT
			id, dataset_id, name, agent_id, agent_revision, status,
			config_json, created_at, COALESCE(completed_at, '')
		FROM experiments
		WHERE id = ?
	`, id).Scan(
		&detail.ID, &detail.DatasetID, &detail.Name, &detail.AgentID, &detail.AgentRevision,
		&detail.Status, &configJSON, &detail.CreatedAt, &detail.CompletedAt,
	)
	if err != nil {
		if err == sql.ErrNoRows {
			return nil, fmt.Errorf("experiment not found: %s", id)
		}
		return nil, fmt.Errorf("query experiment: %w", err)
	}
	if err := decodeObject(configJSON, &detail.Config); err != nil {
		return nil, fmt.Errorf("decode experiment %s config: %w", id, err)
	}

	rows, err := database.Query(`
		SELECT
			experiment_id, case_id, COALESCE(run_id, ''), score,
			COALESCE(label, ''), metadata_json, created_at
		FROM experiment_results
		WHERE experiment_id = ?
		ORDER BY created_at, case_id
	`, id)
	if err != nil {
		return nil, fmt.Errorf("query experiment results: %w", err)
	}
	defer rows.Close()

	detail.Results = []ExperimentResult{}
	for rows.Next() {
		var item ExperimentResult
		var score sql.NullFloat64
		var metadataJSON string
		if err := rows.Scan(
			&item.ExperimentID, &item.CaseID, &item.RunID, &score,
			&item.Label, &metadataJSON, &item.CreatedAt,
		); err != nil {
			return nil, fmt.Errorf("scan experiment result: %w", err)
		}
		if score.Valid {
			value := score.Float64
			item.Score = &value
		}
		if err := decodeObject(metadataJSON, &item.Metadata); err != nil {
			return nil, fmt.Errorf("decode experiment result %s/%s metadata: %w", item.ExperimentID, item.CaseID, err)
		}
		detail.Results = append(detail.Results, item)
	}
	if err := rows.Err(); err != nil {
		return nil, fmt.Errorf("iterate experiment results: %w", err)
	}
	return &detail, nil
}

func decodeStringList(raw string, dst *[]string) error {
	if raw == "" {
		*dst = []string{}
		return nil
	}
	if err := json.Unmarshal([]byte(raw), dst); err != nil {
		return err
	}
	if *dst == nil {
		*dst = []string{}
	}
	return nil
}

func decodeObject(raw string, dst *map[string]any) error {
	if raw == "" {
		*dst = map[string]any{}
		return nil
	}
	if err := json.Unmarshal([]byte(raw), dst); err != nil {
		return err
	}
	if *dst == nil {
		*dst = map[string]any{}
	}
	return nil
}

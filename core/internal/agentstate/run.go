package agentstate

import (
	"database/sql"
	"encoding/json"
	"errors"
	"fmt"
	"strings"

	"github.com/agentvault/core/internal/db"
)

// RunRecord is the persisted execution record shared by MCP, HTTP, and audit
// reads. Legacy runs may omit AgentID/AgentRevision/ContextHash.
type RunRecord struct {
	ID                 string
	AgentName          string
	AgentID            string
	AgentRevision      int
	Task               string
	Status             RunStatus
	ConversationID     string
	ContextHash        string
	Input              map[string]any
	Output             map[string]any
	CapabilitySnapshot map[string]any
	RuntimeMetadata    map[string]any
	StartedAt          string
	EndedAt            string
	FilesChanged       []string
	CreatedAt          string
}

type RunObservationRecord struct {
	ID                  string
	RunID               string
	ParentObservationID string
	Kind                ObservationKind
	Name                string
	Status              string
	Input               map[string]any
	Output              map[string]any
	Evidence            map[string]any
	StartedAt           string
	EndedAt             string
	CreatedAt           string
}

type RunEvaluationRecord struct {
	ID            string
	RunID         string
	ObservationID string
	Evaluator     string
	Name          string
	Score         *float64
	Label         string
	Rationale     string
	Metadata      map[string]any
	CreatedAt     string
}

type RunAudit struct {
	Run          RunRecord
	Context      *ContextSnapshot
	Observations []RunObservationRecord
	Evaluations  []RunEvaluationRecord
}

// RecordRun stores execution evidence and, when a context hash is supplied,
// proves that the immutable context snapshot belongs to the same agent
// revision before accepting the run.
func RecordRun(database *db.DB, record RunRecord) (*RunRecord, error) {
	record.AgentName = strings.TrimSpace(record.AgentName)
	record.AgentID = strings.TrimSpace(record.AgentID)
	record.Task = strings.TrimSpace(record.Task)
	record.ContextHash = strings.TrimSpace(record.ContextHash)
	record.ConversationID = strings.TrimSpace(record.ConversationID)

	if record.AgentName == "" {
		return nil, fmt.Errorf("%w: agent name is required", ErrInvalid)
	}
	if record.Task == "" {
		return nil, fmt.Errorf("%w: task is required", ErrInvalid)
	}
	if record.Status == "" {
		record.Status = RunSucceeded
	}
	if !record.Status.valid() {
		return nil, fmt.Errorf("%w: unknown run status %q", ErrInvalid, record.Status)
	}
	if record.AgentID != "" && record.AgentRevision < 1 {
		return nil, fmt.Errorf("%w: agent revision must be at least 1 when agent id is supplied", ErrInvalid)
	}
	if record.AgentID == "" && record.AgentRevision > 0 {
		return nil, fmt.Errorf("%w: agent id is required when agent revision is supplied", ErrInvalid)
	}
	if record.ContextHash != "" {
		if record.AgentID == "" || record.AgentRevision < 1 {
			return nil, fmt.Errorf("%w: context-bound run requires agent id and revision", ErrInvalid)
		}
		snapshot, err := GetContextSnapshot(database, record.ContextHash)
		if err != nil {
			return nil, err
		}
		if snapshot.AgentID != record.AgentID || snapshot.AgentRevision != record.AgentRevision {
			return nil, fmt.Errorf(
				"%w: context %s belongs to %s@%d, run claims %s@%d",
				ErrConflict,
				record.ContextHash,
				snapshot.AgentID,
				snapshot.AgentRevision,
				record.AgentID,
				record.AgentRevision,
			)
		}
	}

	if record.ID == "" {
		record.ID = newID("run")
	}
	now := nowTimestamp()
	if record.StartedAt == "" {
		record.StartedAt = now
	}
	if record.Status != RunRunning && record.EndedAt == "" {
		record.EndedAt = now
	}
	if record.CreatedAt == "" {
		record.CreatedAt = now
	}
	if record.Input == nil {
		record.Input = map[string]any{}
	}
	if record.Output == nil {
		record.Output = map[string]any{}
	}
	if record.CapabilitySnapshot == nil {
		record.CapabilitySnapshot = map[string]any{}
	}
	if record.RuntimeMetadata == nil {
		record.RuntimeMetadata = map[string]any{}
	}
	if record.FilesChanged == nil {
		record.FilesChanged = []string{}
	}

	inputJSON, err := json.Marshal(record.Input)
	if err != nil {
		return nil, fmt.Errorf("%w: encode run input: %v", ErrInvalid, err)
	}
	outputJSON, err := json.Marshal(record.Output)
	if err != nil {
		return nil, fmt.Errorf("%w: encode run output: %v", ErrInvalid, err)
	}
	capabilityJSON, err := json.Marshal(record.CapabilitySnapshot)
	if err != nil {
		return nil, fmt.Errorf("%w: encode capability snapshot: %v", ErrInvalid, err)
	}
	runtimeJSON, err := json.Marshal(record.RuntimeMetadata)
	if err != nil {
		return nil, fmt.Errorf("%w: encode runtime metadata: %v", ErrInvalid, err)
	}
	filesJSON, err := json.Marshal(record.FilesChanged)
	if err != nil {
		return nil, fmt.Errorf("%w: encode changed files: %v", ErrInvalid, err)
	}

	_, err = database.Exec(
		`INSERT INTO agent_runs (
			id, agent_name, agent_id, agent_revision, task, status, conversation_id,
			context_hash, input_json, output_json, capability_snapshot_json,
			runtime_metadata_json, started_at, ended_at, files_changed_json, created_at
		) VALUES (?, ?, NULLIF(?, ''), NULLIF(?, 0), ?, ?, NULLIF(?, ''),
			NULLIF(?, ''), ?, ?, ?, ?, ?, NULLIF(?, ''), ?, ?)`,
		record.ID,
		record.AgentName,
		record.AgentID,
		record.AgentRevision,
		record.Task,
		string(record.Status),
		record.ConversationID,
		record.ContextHash,
		string(inputJSON),
		string(outputJSON),
		string(capabilityJSON),
		string(runtimeJSON),
		record.StartedAt,
		record.EndedAt,
		string(filesJSON),
		record.CreatedAt,
	)
	if err != nil {
		return nil, fmt.Errorf("%w: record agent run: %v", ErrStorage, err)
	}
	return &record, nil
}

// GetRunAudit returns one run together with the exact immutable context
// snapshot it used plus all observations and evaluations recorded for the run.
func GetRunAudit(database *db.DB, runID string) (*RunAudit, error) {
	runID = strings.TrimSpace(runID)
	if runID == "" {
		return nil, fmt.Errorf("%w: run id is required", ErrInvalid)
	}

	run, err := getRun(database, runID)
	if err != nil {
		return nil, err
	}
	audit := &RunAudit{
		Run:          *run,
		Observations: []RunObservationRecord{},
		Evaluations:  []RunEvaluationRecord{},
	}
	if run.ContextHash != "" {
		snapshot, err := GetContextSnapshot(database, run.ContextHash)
		if err != nil {
			return nil, err
		}
		if snapshot.AgentID != run.AgentID || snapshot.AgentRevision != run.AgentRevision {
			return nil, fmt.Errorf(
				"%w: persisted run/context identity mismatch for %s",
				ErrConflict,
				runID,
			)
		}
		audit.Context = snapshot
	}

	rows, err := database.Query(
		`SELECT
			id, run_id, COALESCE(parent_observation_id, ''), kind, name,
			COALESCE(status, ''), COALESCE(input_json, '{}'), COALESCE(output_json, '{}'),
			COALESCE(evidence_json, '{}'), COALESCE(started_at, ''), COALESCE(ended_at, ''),
			created_at
		FROM run_observations
		WHERE run_id = ?
		ORDER BY created_at, id`,
		runID,
	)
	if err != nil {
		return nil, fmt.Errorf("%w: query run observations: %v", ErrStorage, err)
	}
	for rows.Next() {
		var item RunObservationRecord
		var inputJSON, outputJSON, evidenceJSON string
		if err := rows.Scan(
			&item.ID, &item.RunID, &item.ParentObservationID, &item.Kind, &item.Name,
			&item.Status, &inputJSON, &outputJSON, &evidenceJSON,
			&item.StartedAt, &item.EndedAt, &item.CreatedAt,
		); err != nil {
			rows.Close()
			return nil, fmt.Errorf("%w: scan run observation: %v", ErrStorage, err)
		}
		if err := decodeObject(inputJSON, &item.Input); err != nil {
			rows.Close()
			return nil, fmt.Errorf("%w: decode observation %s input: %v", ErrStorage, item.ID, err)
		}
		if err := decodeObject(outputJSON, &item.Output); err != nil {
			rows.Close()
			return nil, fmt.Errorf("%w: decode observation %s output: %v", ErrStorage, item.ID, err)
		}
		if err := decodeObject(evidenceJSON, &item.Evidence); err != nil {
			rows.Close()
			return nil, fmt.Errorf("%w: decode observation %s evidence: %v", ErrStorage, item.ID, err)
		}
		audit.Observations = append(audit.Observations, item)
	}
	if err := rows.Err(); err != nil {
		rows.Close()
		return nil, fmt.Errorf("%w: iterate run observations: %v", ErrStorage, err)
	}
	rows.Close()

	evaluationRows, err := database.Query(
		`SELECT
			id, run_id, COALESCE(observation_id, ''), evaluator, name, score,
			COALESCE(label, ''), COALESCE(rationale, ''), COALESCE(metadata_json, '{}'),
			created_at
		FROM evaluations
		WHERE run_id = ?
		ORDER BY created_at, id`,
		runID,
	)
	if err != nil {
		return nil, fmt.Errorf("%w: query run evaluations: %v", ErrStorage, err)
	}
	defer evaluationRows.Close()

	for evaluationRows.Next() {
		var item RunEvaluationRecord
		var score sql.NullFloat64
		var metadataJSON string
		if err := evaluationRows.Scan(
			&item.ID, &item.RunID, &item.ObservationID, &item.Evaluator, &item.Name,
			&score, &item.Label, &item.Rationale, &metadataJSON, &item.CreatedAt,
		); err != nil {
			return nil, fmt.Errorf("%w: scan run evaluation: %v", ErrStorage, err)
		}
		if score.Valid {
			value := score.Float64
			item.Score = &value
		}
		if err := decodeObject(metadataJSON, &item.Metadata); err != nil {
			return nil, fmt.Errorf("%w: decode evaluation %s metadata: %v", ErrStorage, item.ID, err)
		}
		audit.Evaluations = append(audit.Evaluations, item)
	}
	if err := evaluationRows.Err(); err != nil {
		return nil, fmt.Errorf("%w: iterate run evaluations: %v", ErrStorage, err)
	}

	return audit, nil
}

func getRun(database *db.DB, runID string) (*RunRecord, error) {
	var record RunRecord
	var revision sql.NullInt64
	var inputJSON, outputJSON, capabilityJSON, runtimeJSON, filesJSON string
	err := database.QueryRow(
		`SELECT
			id, COALESCE(agent_name, ''), COALESCE(agent_id, ''), agent_revision,
			COALESCE(task, ''), status, COALESCE(conversation_id, ''), COALESCE(context_hash, ''),
			COALESCE(input_json, '{}'), COALESCE(output_json, '{}'),
			COALESCE(capability_snapshot_json, '{}'), COALESCE(runtime_metadata_json, '{}'),
			COALESCE(started_at, ''), COALESCE(ended_at, ''), COALESCE(files_changed_json, '[]'),
			created_at
		FROM agent_runs
		WHERE id = ?`,
		runID,
	).Scan(
		&record.ID, &record.AgentName, &record.AgentID, &revision,
		&record.Task, &record.Status, &record.ConversationID, &record.ContextHash,
		&inputJSON, &outputJSON, &capabilityJSON, &runtimeJSON,
		&record.StartedAt, &record.EndedAt, &filesJSON, &record.CreatedAt,
	)
	if err != nil {
		if errors.Is(err, sql.ErrNoRows) {
			return nil, fmt.Errorf("%w: run %s", ErrNotFound, runID)
		}
		return nil, fmt.Errorf("%w: query run: %v", ErrStorage, err)
	}
	if revision.Valid {
		record.AgentRevision = int(revision.Int64)
	}
	if err := decodeObject(inputJSON, &record.Input); err != nil {
		return nil, fmt.Errorf("%w: decode run input: %v", ErrStorage, err)
	}
	if err := decodeObject(outputJSON, &record.Output); err != nil {
		return nil, fmt.Errorf("%w: decode run output: %v", ErrStorage, err)
	}
	if err := decodeObject(capabilityJSON, &record.CapabilitySnapshot); err != nil {
		return nil, fmt.Errorf("%w: decode run capability snapshot: %v", ErrStorage, err)
	}
	if err := decodeObject(runtimeJSON, &record.RuntimeMetadata); err != nil {
		return nil, fmt.Errorf("%w: decode run runtime metadata: %v", ErrStorage, err)
	}
	if err := decodeStringList(filesJSON, &record.FilesChanged); err != nil {
		return nil, fmt.Errorf("%w: decode run changed files: %v", ErrStorage, err)
	}
	return &record, nil
}

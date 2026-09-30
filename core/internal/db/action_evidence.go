package db

import (
	"database/sql"
	"encoding/json"
	"fmt"
	"reflect"
	"time"

	"github.com/agentvault/core/internal/agentstate"
)

// RecordActionIntent stores pre-dispatch evidence for one logical operation.
// Repeating the same run/operation with identical semantic evidence is
// idempotent; a divergent retry fails closed.
func (d *DB) RecordActionIntent(intent agentstate.ActionIntent) (agentstate.ActionIntent, bool, error) {
	if err := intent.Validate(); err != nil {
		return agentstate.ActionIntent{}, false, err
	}
	if intent.ObservationID != "" {
		var observationRunID string
		err := d.conn.QueryRow(
			`SELECT run_id FROM run_observations WHERE id = ?`,
			intent.ObservationID,
		).Scan(&observationRunID)
		if err != nil {
			if err == sql.ErrNoRows {
				return agentstate.ActionIntent{}, false, fmt.Errorf(
					"action intent observation not found: %s",
					intent.ObservationID,
				)
			}
			return agentstate.ActionIntent{}, false, fmt.Errorf("read action intent observation: %w", err)
		}
		if observationRunID != intent.RunID {
			return agentstate.ActionIntent{}, false, fmt.Errorf(
				"action intent observation %q belongs to run %q, not %q",
				intent.ObservationID, observationRunID, intent.RunID,
			)
		}
	}
	if intent.CreatedAt.IsZero() {
		intent.CreatedAt = time.Now().UTC()
	}
	metadataJSON, err := canonicalJSON(intent.Metadata)
	if err != nil {
		return agentstate.ActionIntent{}, false, fmt.Errorf("encode action intent metadata: %w", err)
	}

	result, err := d.conn.Exec(
		`INSERT OR IGNORE INTO action_intents (
			id, run_id, observation_id, operation_id, action, capability_ref,
			authority_hash, input_hash, metadata_json, created_at
		) VALUES (?, ?, NULLIF(?, ''), ?, ?, ?, ?, ?, ?, ?)`,
		intent.ID, intent.RunID, intent.ObservationID, intent.OperationID,
		intent.Action, intent.CapabilityRef, intent.AuthorityHash, intent.InputHash,
		metadataJSON, intent.CreatedAt.UTC().Format(time.RFC3339Nano),
	)
	if err != nil {
		return agentstate.ActionIntent{}, false, fmt.Errorf("record action intent: %w", err)
	}
	rows, err := result.RowsAffected()
	if err != nil {
		return agentstate.ActionIntent{}, false, fmt.Errorf("record action intent rows affected: %w", err)
	}
	if rows == 1 {
		return intent, false, nil
	}

	existing, err := d.actionIntentByOperation(intent.RunID, intent.OperationID)
	if err != nil {
		if err == sql.ErrNoRows {
			return agentstate.ActionIntent{}, false, fmt.Errorf("action intent conflict: duplicate id %q", intent.ID)
		}
		return agentstate.ActionIntent{}, false, err
	}
	if !sameActionIntent(existing, intent) {
		return agentstate.ActionIntent{}, false, fmt.Errorf(
			"action intent conflict for run %q operation %q",
			intent.RunID, intent.OperationID,
		)
	}
	return existing, true, nil
}

// RecordActionReceipt stores exactly one terminal result for an intent.
// An identical retry is idempotent; a different terminal result is rejected.
func (d *DB) RecordActionReceipt(receipt agentstate.ActionReceipt) (agentstate.ActionReceipt, bool, error) {
	if err := receipt.Validate(); err != nil {
		return agentstate.ActionReceipt{}, false, err
	}
	if receipt.CompletedAt.IsZero() {
		receipt.CompletedAt = time.Now().UTC()
	}
	metadataJSON, err := canonicalJSON(receipt.Metadata)
	if err != nil {
		return agentstate.ActionReceipt{}, false, fmt.Errorf("encode action receipt metadata: %w", err)
	}

	result, err := d.conn.Exec(
		`INSERT OR IGNORE INTO action_receipts (
			id, intent_id, status, result_hash, external_receipt_id,
			error_code, error_message, metadata_json, completed_at
		) VALUES (?, ?, ?, NULLIF(?, ''), NULLIF(?, ''), NULLIF(?, ''), NULLIF(?, ''), ?, ?)`,
		receipt.ID, receipt.IntentID, string(receipt.Status), receipt.ResultHash,
		receipt.ExternalReceiptID, receipt.ErrorCode, receipt.ErrorMessage,
		metadataJSON, receipt.CompletedAt.UTC().Format(time.RFC3339Nano),
	)
	if err != nil {
		return agentstate.ActionReceipt{}, false, fmt.Errorf("record action receipt: %w", err)
	}
	rows, err := result.RowsAffected()
	if err != nil {
		return agentstate.ActionReceipt{}, false, fmt.Errorf("record action receipt rows affected: %w", err)
	}
	if rows == 1 {
		return receipt, false, nil
	}

	existing, err := d.actionReceiptByIntent(receipt.IntentID)
	if err != nil {
		if err == sql.ErrNoRows {
			return agentstate.ActionReceipt{}, false, fmt.Errorf("action receipt conflict: duplicate id %q", receipt.ID)
		}
		return agentstate.ActionReceipt{}, false, err
	}
	if !sameActionReceipt(existing, receipt) {
		return agentstate.ActionReceipt{}, false, fmt.Errorf(
			"action receipt conflict for intent %q",
			receipt.IntentID,
		)
	}
	return existing, true, nil
}

func (d *DB) actionIntentByOperation(runID, operationID string) (agentstate.ActionIntent, error) {
	var (
		intent       agentstate.ActionIntent
		observation  sql.NullString
		metadataJSON string
		createdAt    string
	)
	err := d.conn.QueryRow(
		`SELECT id, run_id, observation_id, operation_id, action, capability_ref,
		        authority_hash, input_hash, metadata_json, created_at
		 FROM action_intents
		 WHERE run_id = ? AND operation_id = ?`,
		runID, operationID,
	).Scan(
		&intent.ID, &intent.RunID, &observation, &intent.OperationID,
		&intent.Action, &intent.CapabilityRef, &intent.AuthorityHash,
		&intent.InputHash, &metadataJSON, &createdAt,
	)
	if err != nil {
		return agentstate.ActionIntent{}, err
	}
	if observation.Valid {
		intent.ObservationID = observation.String
	}
	if err := json.Unmarshal([]byte(metadataJSON), &intent.Metadata); err != nil {
		return agentstate.ActionIntent{}, fmt.Errorf("decode action intent metadata: %w", err)
	}
	intent.CreatedAt = parseEvidenceTime(createdAt)
	return intent, nil
}

func (d *DB) actionReceiptByIntent(intentID string) (agentstate.ActionReceipt, error) {
	var (
		receipt      agentstate.ActionReceipt
		resultHash   sql.NullString
		externalID   sql.NullString
		errorCode    sql.NullString
		errorMessage sql.NullString
		metadataJSON string
		completedAt  string
	)
	err := d.conn.QueryRow(
		`SELECT id, intent_id, status, result_hash, external_receipt_id,
		        error_code, error_message, metadata_json, completed_at
		 FROM action_receipts
		 WHERE intent_id = ?`,
		intentID,
	).Scan(
		&receipt.ID, &receipt.IntentID, &receipt.Status, &resultHash, &externalID,
		&errorCode, &errorMessage, &metadataJSON, &completedAt,
	)
	if err != nil {
		return agentstate.ActionReceipt{}, err
	}
	if resultHash.Valid {
		receipt.ResultHash = resultHash.String
	}
	if externalID.Valid {
		receipt.ExternalReceiptID = externalID.String
	}
	if errorCode.Valid {
		receipt.ErrorCode = errorCode.String
	}
	if errorMessage.Valid {
		receipt.ErrorMessage = errorMessage.String
	}
	if err := json.Unmarshal([]byte(metadataJSON), &receipt.Metadata); err != nil {
		return agentstate.ActionReceipt{}, fmt.Errorf("decode action receipt metadata: %w", err)
	}
	receipt.CompletedAt = parseEvidenceTime(completedAt)
	return receipt, nil
}

func sameActionIntent(existing, candidate agentstate.ActionIntent) bool {
	return existing.RunID == candidate.RunID &&
		existing.ObservationID == candidate.ObservationID &&
		existing.OperationID == candidate.OperationID &&
		existing.Action == candidate.Action &&
		existing.CapabilityRef == candidate.CapabilityRef &&
		existing.AuthorityHash == candidate.AuthorityHash &&
		existing.InputHash == candidate.InputHash &&
		reflect.DeepEqual(normalizeMetadata(existing.Metadata), normalizeMetadata(candidate.Metadata))
}

func sameActionReceipt(existing, candidate agentstate.ActionReceipt) bool {
	return existing.IntentID == candidate.IntentID &&
		existing.Status == candidate.Status &&
		existing.ResultHash == candidate.ResultHash &&
		existing.ExternalReceiptID == candidate.ExternalReceiptID &&
		existing.ErrorCode == candidate.ErrorCode &&
		existing.ErrorMessage == candidate.ErrorMessage &&
		reflect.DeepEqual(normalizeMetadata(existing.Metadata), normalizeMetadata(candidate.Metadata))
}

func canonicalJSON(value map[string]any) (string, error) {
	if value == nil {
		value = map[string]any{}
	}
	data, err := json.Marshal(value)
	if err != nil {
		return "", err
	}
	return string(data), nil
}

func normalizeMetadata(value map[string]any) map[string]any {
	if value == nil {
		return map[string]any{}
	}
	return value
}

func parseEvidenceTime(value string) time.Time {
	for _, layout := range []string{time.RFC3339Nano, time.RFC3339, "2006-01-02 15:04:05"} {
		if parsed, err := time.Parse(layout, value); err == nil {
			return parsed
		}
	}
	return time.Time{}
}

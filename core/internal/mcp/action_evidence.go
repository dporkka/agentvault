package mcp

import (
	"crypto/sha256"
	"database/sql"
	"encoding/json"
	"fmt"
	"strings"
	"time"

	"github.com/agentvault/core/internal/agentstate"
)

func (s *Server) registerRecordActionIntent() {
	s.tools["agentvault.record_action_intent"] = Tool{
		Name:        "agentvault.record_action_intent",
		Description: "Record immutable pre-dispatch evidence for one externally executed action. AgentVault derives the authority hash from the parent run's recorded capability snapshot and does not execute the action.",
		InputSchema: makeSchema(map[string]interface{}{
			"run_id":          schemaString("Parent agent run ID"),
			"observation_id":  schemaString("Optional tool/action observation associated with this intent"),
			"operation_id":    schemaString("Stable logical operation/idempotency ID within the run"),
			"action":          schemaString("Stable action name, for example github.create_pr"),
			"capability_ref":  schemaString("Capability that authorizes this action and must be present in the run snapshot"),
			"input_hash":      schemaString("Digest of the exact action input; keep secret/plaintext inputs outside AgentVault when appropriate"),
			"metadata_json":   schemaString("Optional JSON object with non-secret action evidence"),
		}, []string{"run_id", "operation_id", "action", "capability_ref", "input_hash"}),
		Handler: s.handleRecordActionIntent,
	}
}

func (s *Server) handleRecordActionIntent(args map[string]interface{}) (string, error) {
	runID := stringArg(args, "run_id")
	operationID := stringArg(args, "operation_id")
	action := stringArg(args, "action")
	capabilityRef := stringArg(args, "capability_ref")
	inputHash := stringArg(args, "input_hash")
	observationID := stringArg(args, "observation_id")

	authorityHash, err := s.authorityHashForRun(runID, capabilityRef)
	if err != nil {
		return "", err
	}
	metadataJSON, err := normalizedJSONObjectArg(args, "metadata_json")
	if err != nil {
		return "", err
	}
	var metadata map[string]any
	if err := json.Unmarshal([]byte(metadataJSON), &metadata); err != nil {
		return "", fmt.Errorf("decode action intent metadata: %w", err)
	}

	intent := agentstate.ActionIntent{
		ID:            fmt.Sprintf("intent_%d", time.Now().UnixNano()),
		RunID:         runID,
		ObservationID: observationID,
		OperationID:   operationID,
		Action:        action,
		CapabilityRef: capabilityRef,
		AuthorityHash: authorityHash,
		InputHash:     inputHash,
		Metadata:      metadata,
		CreatedAt:     time.Now().UTC(),
	}
	if err := intent.Validate(); err != nil {
		return "", err
	}

	recorded, existed, err := s.db.RecordActionIntent(intent)
	if err != nil {
		return "", err
	}
	prefix := "Recorded action intent"
	if existed {
		prefix = "Recorded action intent (idempotent)"
	}
	return fmt.Sprintf(
		"%s: %s\n- **Run:** %s\n- **Operation:** %s\n- **Action:** %s\n- **Capability:** %s\n- **Authority:** %s",
		prefix, recorded.ID, recorded.RunID, recorded.OperationID, recorded.Action,
		recorded.CapabilityRef, recorded.AuthorityHash,
	), nil
}

func (s *Server) registerRecordActionReceipt() {
	s.tools["agentvault.record_action_receipt"] = Tool{
		Name:        "agentvault.record_action_receipt",
		Description: "Record the immutable terminal receipt for a previously recorded action intent. A logical intent can have only one terminal outcome; identical retries are idempotent and divergent retries fail closed.",
		InputSchema: makeSchema(map[string]interface{}{
			"intent_id":            schemaString("Action intent ID"),
			"status":               schemaStringEnum("Terminal action status", []string{"completed", "failed", "indeterminate"}),
			"result_hash":          schemaString("Digest of the result; required for completed actions"),
			"external_receipt_id":  schemaString("Provider receipt/request/resource ID when available"),
			"error_code":           schemaString("Stable failure/indeterminate error code"),
			"error_message":        schemaString("Failure/indeterminate explanation"),
			"metadata_json":        schemaString("Optional JSON object with non-secret terminal evidence"),
		}, []string{"intent_id", "status"}),
		Handler: s.handleRecordActionReceipt,
	}
}

func (s *Server) handleRecordActionReceipt(args map[string]interface{}) (string, error) {
	metadataJSON, err := normalizedJSONObjectArg(args, "metadata_json")
	if err != nil {
		return "", err
	}
	var metadata map[string]any
	if err := json.Unmarshal([]byte(metadataJSON), &metadata); err != nil {
		return "", fmt.Errorf("decode action receipt metadata: %w", err)
	}

	receipt := agentstate.ActionReceipt{
		ID:                fmt.Sprintf("receipt_%d", time.Now().UnixNano()),
		IntentID:          stringArg(args, "intent_id"),
		Status:            agentstate.ActionReceiptStatus(stringArg(args, "status")),
		ResultHash:        stringArg(args, "result_hash"),
		ExternalReceiptID: stringArg(args, "external_receipt_id"),
		ErrorCode:         stringArg(args, "error_code"),
		ErrorMessage:      stringArg(args, "error_message"),
		Metadata:          metadata,
		CompletedAt:       time.Now().UTC(),
	}
	if err := receipt.Validate(); err != nil {
		return "", err
	}

	recorded, existed, err := s.db.RecordActionReceipt(receipt)
	if err != nil {
		return "", err
	}
	prefix := "Recorded action receipt"
	if existed {
		prefix = "Recorded action receipt (idempotent)"
	}
	return fmt.Sprintf(
		"%s: %s\n- **Intent:** %s\n- **Status:** %s",
		prefix, recorded.ID, recorded.IntentID, recorded.Status,
	), nil
}

func (s *Server) authorityHashForRun(runID, capabilityRef string) (string, error) {
	if strings.TrimSpace(runID) == "" {
		return "", fmt.Errorf("run_id is required")
	}
	if strings.TrimSpace(capabilityRef) == "" {
		return "", fmt.Errorf("capability_ref is required")
	}

	var raw sql.NullString
	if err := s.db.QueryRow(
		`SELECT capability_snapshot_json FROM agent_runs WHERE id = ?`,
		runID,
	).Scan(&raw); err != nil {
		if err == sql.ErrNoRows {
			return "", fmt.Errorf("run not found: %s", runID)
		}
		return "", fmt.Errorf("read run capability snapshot: %w", err)
	}
	if !raw.Valid || strings.TrimSpace(raw.String) == "" {
		return "", fmt.Errorf("run %s has no capability snapshot", runID)
	}

	var snapshot map[string]any
	if err := json.Unmarshal([]byte(raw.String), &snapshot); err != nil {
		return "", fmt.Errorf("invalid run capability snapshot: %w", err)
	}
	grant, ok := snapshot[capabilityRef]
	if !ok || grant == nil {
		return "", fmt.Errorf("capability %q is not present in run capability snapshot", capabilityRef)
	}
	if allowed, isBool := grant.(bool); isBool && !allowed {
		return "", fmt.Errorf("capability %q is not present in run capability snapshot", capabilityRef)
	}

	canonical, err := json.Marshal(snapshot)
	if err != nil {
		return "", fmt.Errorf("canonicalize run capability snapshot: %w", err)
	}
	sum := sha256.Sum256(canonical)
	return fmt.Sprintf("sha256:%x", sum[:]), nil
}

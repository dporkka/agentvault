package db

import (
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"

	"github.com/agentvault/core/internal/agentstate"
)

func TestRecordActionIntentIsIdempotentAndRejectsConflictingOperation(t *testing.T) {
	database := openActionEvidenceTestDB(t)
	defer database.Close()
	seedActionEvidenceRun(t, database, "run_action_1")

	intent := agentstate.ActionIntent{
		ID: "intent_1", RunID: "run_action_1", OperationID: "op_create_pr",
		Action: "github.create_pr", CapabilityRef: "github.write",
		AuthorityHash: "sha256:authority", InputHash: "sha256:input",
		Metadata: map[string]any{"repo": "owner/repo"},
		CreatedAt: time.Date(2026, 9, 30, 12, 0, 0, 0, time.UTC),
	}
	first, existed, err := database.RecordActionIntent(intent)
	if err != nil {
		t.Fatalf("RecordActionIntent first: %v", err)
	}
	if existed {
		t.Fatal("first RecordActionIntent existed = true")
	}

	retry := intent
	retry.ID = "intent_retry"
	retry.CreatedAt = intent.CreatedAt.Add(time.Second)
	second, existed, err := database.RecordActionIntent(retry)
	if err != nil {
		t.Fatalf("RecordActionIntent idempotent retry: %v", err)
	}
	if !existed || second.ID != first.ID {
		t.Fatalf("idempotent retry = (%+v, existed=%v), want original %q", second, existed, first.ID)
	}

	conflict := retry
	conflict.ID = "intent_conflict"
	conflict.InputHash = "sha256:different"
	if _, _, err := database.RecordActionIntent(conflict); err == nil || !strings.Contains(err.Error(), "conflict") {
		t.Fatalf("conflicting operation error = %v, want conflict", err)
	}
}

func TestRecordActionReceiptIsTerminalAndIdempotent(t *testing.T) {
	database := openActionEvidenceTestDB(t)
	defer database.Close()
	seedActionEvidenceRun(t, database, "run_action_2")

	intent, _, err := database.RecordActionIntent(agentstate.ActionIntent{
		ID: "intent_2", RunID: "run_action_2", OperationID: "op_send",
		Action: "email.send", CapabilityRef: "email.send",
		AuthorityHash: "sha256:authority", InputHash: "sha256:input",
	})
	if err != nil {
		t.Fatal(err)
	}
	receipt := agentstate.ActionReceipt{
		ID: "receipt_1", IntentID: intent.ID, Status: agentstate.ActionCompleted,
		ResultHash: "sha256:result", ExternalReceiptID: "provider-123",
	}
	first, existed, err := database.RecordActionReceipt(receipt)
	if err != nil {
		t.Fatalf("RecordActionReceipt first: %v", err)
	}
	if existed {
		t.Fatal("first receipt existed = true")
	}

	retry := receipt
	retry.ID = "receipt_retry"
	second, existed, err := database.RecordActionReceipt(retry)
	if err != nil {
		t.Fatalf("RecordActionReceipt idempotent retry: %v", err)
	}
	if !existed || second.ID != first.ID {
		t.Fatalf("idempotent receipt retry = (%+v, existed=%v), want original %q", second, existed, first.ID)
	}

	conflict := retry
	conflict.Status = agentstate.ActionIndeterminate
	conflict.ResultHash = ""
	conflict.ErrorCode = "timeout"
	if _, _, err := database.RecordActionReceipt(conflict); err == nil || !strings.Contains(err.Error(), "conflict") {
		t.Fatalf("conflicting terminal receipt error = %v, want conflict", err)
	}
}

func TestActionEvidenceRowsAreImmutable(t *testing.T) {
	database := openActionEvidenceTestDB(t)
	defer database.Close()
	seedActionEvidenceRun(t, database, "run_action_3")

	intent, _, err := database.RecordActionIntent(agentstate.ActionIntent{
		ID: "intent_3", RunID: "run_action_3", OperationID: "op_immutable",
		Action: "github.create_issue", CapabilityRef: "github.write",
		AuthorityHash: "sha256:authority", InputHash: "sha256:input",
	})
	if err != nil {
		t.Fatal(err)
	}
	if _, err := database.Exec(`UPDATE action_intents SET input_hash = 'changed' WHERE id = ?`, intent.ID); err == nil {
		t.Fatal("expected direct action_intents UPDATE to fail")
	}

	receipt, _, err := database.RecordActionReceipt(agentstate.ActionReceipt{
		ID: "receipt_3", IntentID: intent.ID, Status: agentstate.ActionFailed,
		ErrorCode: "rejected",
	})
	if err != nil {
		t.Fatal(err)
	}
	if _, err := database.Exec(`DELETE FROM action_receipts WHERE id = ?`, receipt.ID); err == nil {
		t.Fatal("expected direct action_receipts DELETE to fail")
	}
}

func openActionEvidenceTestDB(t *testing.T) *DB {
	t.Helper()
	dir := t.TempDir()
	if err := os.MkdirAll(filepath.Join(dir, ".agentvault"), 0o755); err != nil {
		t.Fatal(err)
	}
	database, err := Open(dir)
	if err != nil {
		t.Fatal(err)
	}
	if err := database.RunMigrations(); err != nil {
		database.Close()
		t.Fatal(err)
	}
	return database
}

func seedActionEvidenceRun(t *testing.T, database *DB, id string) {
	t.Helper()
	if _, err := database.Exec(
		`INSERT INTO agent_runs (id, agent_name, task, status, capability_snapshot_json, created_at)
		 VALUES (?, 'test-agent', 'test action evidence', 'running', '{"github.write":true}', datetime('now'))`,
		id,
	); err != nil {
		t.Fatalf("seed action run: %v", err)
	}
}


func TestRecordActionIntentRejectsObservationFromDifferentRun(t *testing.T) {
	database := openActionEvidenceTestDB(t)
	defer database.Close()
	seedActionEvidenceRun(t, database, "run_action_owner")
	seedActionEvidenceRun(t, database, "run_action_other")

	if _, err := database.Exec(
		`INSERT INTO run_observations (
			id, run_id, kind, name, created_at
		) VALUES ('obs_other', 'run_action_other', 'tool', 'github.create_pr', datetime('now'))`,
	); err != nil {
		t.Fatalf("seed observation: %v", err)
	}

	_, _, err := database.RecordActionIntent(agentstate.ActionIntent{
		ID: "intent_wrong_obs", RunID: "run_action_owner", ObservationID: "obs_other",
		OperationID: "op_wrong_obs", Action: "github.create_pr",
		CapabilityRef: "github.write", AuthorityHash: "sha256:authority",
		InputHash: "sha256:input",
	})
	if err == nil || !strings.Contains(err.Error(), "belongs to run") {
		t.Fatalf("RecordActionIntent error = %v, want observation/run mismatch", err)
	}
}

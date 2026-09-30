package agentstate

import (
	"encoding/json"
	"errors"
	"os"
	"path/filepath"
	"testing"

	"github.com/agentvault/core/internal/db"
)

func setupRunTestDB(t *testing.T) *db.DB {
	t.Helper()
	vaultPath := t.TempDir()
	if err := os.MkdirAll(filepath.Join(vaultPath, ".agentvault"), 0755); err != nil {
		t.Fatal(err)
	}
	database, err := db.Open(vaultPath)
	if err != nil {
		t.Fatal(err)
	}
	if err := database.RunMigrations(); err != nil {
		database.Close()
		t.Fatal(err)
	}
	return database
}

func seedContextSnapshot(t *testing.T, database *db.DB, snapshot ContextSnapshot) {
	t.Helper()
	raw, err := json.Marshal(snapshot)
	if err != nil {
		t.Fatal(err)
	}
	if _, err := database.Exec(`
		INSERT INTO context_snapshots (
			hash, agent_id, agent_revision, task, conversation_id, context_json, created_at
		) VALUES (?, ?, ?, NULLIF(?, ''), NULLIF(?, ''), ?, datetime('now'))
	`, snapshot.Hash, snapshot.AgentID, snapshot.AgentRevision, snapshot.Task, snapshot.ConversationID, string(raw)); err != nil {
		t.Fatal(err)
	}
}

func TestRecordRunValidatesContextBinding(t *testing.T) {
	database := setupRunTestDB(t)
	defer database.Close()

	snapshot := ContextSnapshot{
		Hash: "sha256:ctx-match", AgentID: "agt_1", AgentRevision: 3, AgentTitle: "Coder",
		KnowledgeScopes: []string{}, ArtifactScopes: []string{}, ConversationScopes: []string{},
		CapabilityRefs: []string{}, Sections: []ContextSection{}, Unresolved: []ContextReferenceIssue{},
		Text: "compiled",
	}
	seedContextSnapshot(t, database, snapshot)

	record, err := RecordRun(database, RunRecord{
		AgentName: "coding-agent", AgentID: "agt_1", AgentRevision: 3,
		Task: "implement audit linkage", Status: RunSucceeded, ContextHash: snapshot.Hash,
		Input: map[string]any{"issue": 81}, Output: map[string]any{"result": "ok"},
		CapabilitySnapshot: map[string]any{"github.read": true},
		RuntimeMetadata: map[string]any{"runtime": "test"},
		FilesChanged: []string{"core/internal/agentstate/run.go"},
	})
	if err != nil {
		t.Fatalf("RecordRun matching context: %v", err)
	}
	if record.ID == "" || record.ContextHash != snapshot.Hash || record.AgentRevision != 3 {
		t.Fatalf("unexpected recorded run: %#v", record)
	}

	_, err = RecordRun(database, RunRecord{
		AgentName: "wrong-agent", AgentID: "agt_2", AgentRevision: 3,
		Task: "bad binding", Status: RunSucceeded, ContextHash: snapshot.Hash,
	})
	if !errors.Is(err, ErrConflict) {
		t.Fatalf("expected agent mismatch conflict, got %v", err)
	}

	_, err = RecordRun(database, RunRecord{
		AgentName: "wrong-revision", AgentID: "agt_1", AgentRevision: 4,
		Task: "bad binding", Status: RunSucceeded, ContextHash: snapshot.Hash,
	})
	if !errors.Is(err, ErrConflict) {
		t.Fatalf("expected revision mismatch conflict, got %v", err)
	}

	_, err = RecordRun(database, RunRecord{
		AgentName: "missing-context", AgentID: "agt_1", AgentRevision: 3,
		Task: "bad binding", Status: RunSucceeded, ContextHash: "sha256:missing",
	})
	if !errors.Is(err, ErrNotFound) {
		t.Fatalf("expected missing context error, got %v", err)
	}
}

func TestRecordRunKeepsLegacyUnboundRunsCompatible(t *testing.T) {
	database := setupRunTestDB(t)
	defer database.Close()

	record, err := RecordRun(database, RunRecord{
		AgentName: "legacy-agent",
		Task:      "legacy task",
		Status:    RunSucceeded,
	})
	if err != nil {
		t.Fatalf("RecordRun legacy: %v", err)
	}
	if record.ID == "" || record.AgentID != "" || record.AgentRevision != 0 || record.ContextHash != "" {
		t.Fatalf("unexpected legacy run: %#v", record)
	}
}

func TestGetRunAuditReturnsContextObservationsEvaluationsAndProvenance(t *testing.T) {
	database := setupRunTestDB(t)
	defer database.Close()

	snapshot := ContextSnapshot{
		Hash: "sha256:audit", AgentID: "agt_audit", AgentRevision: 7, AgentTitle: "Audit Agent",
		KnowledgeScopes: []string{"project:test"}, ArtifactScopes: []string{}, ConversationScopes: []string{},
		CapabilityRefs: []string{"github"}, ContextPolicyRef: "policy_1",
		Sections: []ContextSection{
			{Kind: ContextIdentity, SourceID: "identity_1", SourcePath: "10-notes/identity.md", Title: "Identity", Content: "Be precise."},
			{Kind: ContextMemory, SourceID: "memory_1", SourcePath: "10-notes/memory.md", Title: "Memory", Content: "Preserve provenance."},
		},
		Unresolved: []ContextReferenceIssue{},
		Text:       "compiled audit context",
	}
	seedContextSnapshot(t, database, snapshot)

	run, err := RecordRun(database, RunRecord{
		AgentName: "audit-agent", AgentID: "agt_audit", AgentRevision: 7,
		Task: "audit this run", Status: RunSucceeded, ContextHash: snapshot.Hash,
		Input: map[string]any{"prompt": "audit"}, Output: map[string]any{"ok": true},
		CapabilitySnapshot: map[string]any{"github": "read"}, RuntimeMetadata: map[string]any{"model": "test"},
		FilesChanged: []string{"README.md"},
	})
	if err != nil {
		t.Fatal(err)
	}

	if _, err := database.Exec(`
		INSERT INTO run_observations (
			id, run_id, kind, name, status, input_json, output_json, evidence_json,
			started_at, ended_at, created_at
		) VALUES (
			'obs_audit_1', ?, 'retrieval', 'retrieve knowledge', 'succeeded',
			'{"query":"audit"}', '{"count":1}', '{"noteIds":["memory_1"]}',
			'2026-09-30T10:00:00Z', '2026-09-30T10:00:01Z', '2026-09-30T10:00:01Z'
		)
	`, run.ID); err != nil {
		t.Fatal(err)
	}
	if _, err := database.Exec(`
		INSERT INTO evaluations (
			id, run_id, observation_id, evaluator, name, score, label, rationale, metadata_json, created_at
		) VALUES (
			'eval_audit_1', ?, 'obs_audit_1', 'human:test', 'correctness',
			0.95, 'pass', 'Matches expected behavior', '{"source":"test"}',
			'2026-09-30T10:00:02Z'
		)
	`, run.ID); err != nil {
		t.Fatal(err)
	}

	audit, err := GetRunAudit(database, run.ID)
	if err != nil {
		t.Fatalf("GetRunAudit: %v", err)
	}
	if audit.Run.ID != run.ID || audit.Run.ContextHash != snapshot.Hash {
		t.Fatalf("unexpected audit run: %#v", audit.Run)
	}
	if audit.Context == nil || audit.Context.Hash != snapshot.Hash || len(audit.Context.Sections) != 2 {
		t.Fatalf("unexpected audit context: %#v", audit.Context)
	}
	if audit.Context.Sections[0].SourceID != "identity_1" || audit.Context.Sections[1].SourceID != "memory_1" {
		t.Fatalf("missing context provenance: %#v", audit.Context.Sections)
	}
	if len(audit.Observations) != 1 || audit.Observations[0].ID != "obs_audit_1" {
		t.Fatalf("unexpected observations: %#v", audit.Observations)
	}
	if audit.Observations[0].Evidence["noteIds"] == nil {
		t.Fatalf("expected decoded observation evidence: %#v", audit.Observations[0].Evidence)
	}
	if len(audit.Evaluations) != 1 || audit.Evaluations[0].ID != "eval_audit_1" {
		t.Fatalf("unexpected evaluations: %#v", audit.Evaluations)
	}
	if audit.Evaluations[0].Score == nil || *audit.Evaluations[0].Score != 0.95 || audit.Evaluations[0].Label != "pass" {
		t.Fatalf("unexpected evaluation evidence: %#v", audit.Evaluations[0])
	}
}

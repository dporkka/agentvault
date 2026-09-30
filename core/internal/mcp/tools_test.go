package mcp

import (
	"encoding/json"
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"testing"

	"github.com/agentvault/core/internal/agentstate"
	"github.com/agentvault/core/internal/db"
)

func TestHandleSearch_Results(t *testing.T) {
	s, db := setupTestServer(t)
	defer db.Close()

	// Seed test data
	addTestNote(t, db, "note_2024_01_15_123", "Pricing Strategy", "10-notes/pricing.md", "note", "business", "Discussion about pricing models and strategies.", []string{"pricing", "business"})
	addTestNote(t, db, "dec_2024_01_15_456", "Use Postgres", "30-decisions/postgres.md", "decision", "tech", "Decision to use PostgreSQL as primary database.", []string{"database", "tech"})

	// Test search without FTS (empty query returns all)
	result, err := s.handleSearch(map[string]interface{}{
		"query":   "",
		"project": "business",
		"limit":   float64(10),
	})
	if err != nil {
		t.Fatalf("handleSearch error: %v", err)
	}

	if !strings.Contains(result, "Pricing Strategy") {
		t.Errorf("expected result to contain 'Pricing Strategy', got:\n%s", result)
	}
	if strings.Contains(result, "Use Postgres") {
		t.Errorf("did not expect 'Use Postgres' in business project results, got:\n%s", result)
	}
}

func TestHandleSearch_NoResults(t *testing.T) {
	s, db := setupTestServer(t)
	defer db.Close()

	result, err := s.handleSearch(map[string]interface{}{
		"query": "xyznonexistent123",
		"limit": float64(10),
	})
	if err != nil {
		t.Fatalf("handleSearch error: %v", err)
	}

	if !strings.Contains(result, "No results found") {
		t.Errorf("expected 'No results found', got:\n%s", result)
	}
}

func TestHandleReadNote_ByID(t *testing.T) {
	s, db := setupTestServer(t)
	defer db.Close()

	// Create a test note file
	noteDir := filepath.Join(s.vaultPath, "10-notes")
	os.MkdirAll(noteDir, 0755)
	noteContent := "---\nid: note_read_001\ntype: note\ntitle: Test Readable\nproject: testproj\n---\n\nThis is the test note body.\n"
	notePath := filepath.Join(noteDir, "test-readable.md")
	os.WriteFile(notePath, []byte(noteContent), 0644)

	// Seed the database
	addTestNote(t, db, "note_read_001", "Test Readable", "10-notes/test-readable.md", "note", "testproj", "This is the test note body.", []string{"test"})

	result, err := s.handleReadNote(map[string]interface{}{
		"id": "note_read_001",
	})
	if err != nil {
		t.Fatalf("handleReadNote error: %v", err)
	}

	if !strings.Contains(result, "Test Readable") {
		t.Errorf("expected result to contain 'Test Readable', got:\n%s", result)
	}
	if !strings.Contains(result, "This is the test note body") {
		t.Errorf("expected result to contain body text, got:\n%s", result)
	}
}

func TestHandleReadNote_NotFound(t *testing.T) {
	s, db := setupTestServer(t)
	defer db.Close()

	_, err := s.handleReadNote(map[string]interface{}{
		"id": "note_nonexistent_xyz",
	})
	if err == nil {
		t.Fatal("expected error for nonexistent note")
	}
	if !strings.Contains(err.Error(), "not found") {
		t.Errorf("expected 'not found' in error, got: %v", err)
	}
}

func TestHandleCreateNote(t *testing.T) {
	s, db := setupTestServer(t)
	defer db.Close()

	result, err := s.handleCreateNote(map[string]interface{}{
		"type":    "note",
		"title":   "My New Note",
		"project": "testproj",
		"tags":    []interface{}{"test", "demo"},
	})
	if err != nil {
		t.Fatalf("handleCreateNote error: %v", err)
	}

	if !strings.Contains(result, "Created note:") {
		t.Errorf("expected 'Created note:' in result, got:\n%s", result)
	}
	if !strings.Contains(result, "note") {
		t.Error("expected note type in result")
	}

	// Verify file was created
	noteDir := filepath.Join(s.vaultPath, "10-notes")
	entries, err := os.ReadDir(noteDir)
	if err != nil {
		t.Fatalf("read notes dir: %v", err)
	}
	if len(entries) == 0 {
		t.Error("expected note file to be created")
	}
}

func TestHandleCreateDecision(t *testing.T) {
	s, db := setupTestServer(t)
	defer db.Close()

	result, err := s.handleCreateDecision(map[string]interface{}{
		"title":   "Use Redis",
		"project": "infra",
		"tags":    []interface{}{"cache", "infra"},
	})
	if err != nil {
		t.Fatalf("handleCreateDecision error: %v", err)
	}

	if !strings.Contains(result, "Created note:") {
		t.Errorf("expected 'Created note:' in result, got:\n%s", result)
	}

	// Verify file was created in decisions folder
	decDir := filepath.Join(s.vaultPath, "30-decisions")
	entries, err := os.ReadDir(decDir)
	if err != nil {
		t.Fatalf("read decisions dir: %v", err)
	}
	if len(entries) == 0 {
		t.Error("expected decision file to be created")
	}
}

func TestHandleCreateTask(t *testing.T) {
	s, db := setupTestServer(t)
	defer db.Close()

	result, err := s.handleCreateTask(map[string]interface{}{
		"title":   "Implement search",
		"project": "backend",
	})
	if err != nil {
		t.Fatalf("handleCreateTask error: %v", err)
	}

	if !strings.Contains(result, "Created note:") {
		t.Errorf("expected 'Created note:' in result, got:\n%s", result)
	}
}

func TestHandleCreateNote_InvalidType(t *testing.T) {
	s, db := setupTestServer(t)
	defer db.Close()

	_, err := s.handleCreateNote(map[string]interface{}{
		"type":  "nonexistent",
		"title": "Test",
	})
	if err == nil {
		t.Fatal("expected error for invalid note type")
	}
}

func TestHandleCapture(t *testing.T) {
	s, db := setupTestServer(t)
	defer db.Close()

	result, err := s.handleCapture(map[string]interface{}{
		"title":      "Quick Idea",
		"text":       "This is a quick capture idea for later.",
		"source_url": "https://example.com",
		"project":    "ideas",
		"tags":       []interface{}{"idea", "quick"},
	})
	if err != nil {
		t.Fatalf("handleCapture error: %v", err)
	}

	if !strings.Contains(result, "Captured to inbox:") {
		t.Errorf("expected 'Captured to inbox:' in result, got:\n%s", result)
	}

	// Verify file was created in inbox
	inboxDir := filepath.Join(s.vaultPath, "00-inbox")
	entries, err := os.ReadDir(inboxDir)
	if err != nil {
		t.Fatalf("read inbox dir: %v", err)
	}
	if len(entries) == 0 {
		t.Error("expected capture file to be created in inbox")
	}

	// Verify content
	content, err := os.ReadFile(filepath.Join(inboxDir, entries[0].Name()))
	if err != nil {
		t.Fatalf("read capture file: %v", err)
	}
	if !strings.Contains(string(content), "Quick Idea") {
		t.Error("expected capture title in file content")
	}
	if !strings.Contains(string(content), "quick capture idea") {
		t.Error("expected capture text in file content")
	}
}

func TestHandleCapture_MissingTitle(t *testing.T) {
	s, db := setupTestServer(t)
	defer db.Close()

	_, err := s.handleCapture(map[string]interface{}{
		"text": "No title here",
	})
	if err == nil {
		t.Fatal("expected error for missing title")
	}
}

func TestHandleSummarize(t *testing.T) {
	s, db := setupTestServer(t)
	defer db.Close()

	// Create some test files in a folder
	summaryDir := filepath.Join(s.vaultPath, "50-summary-test")
	os.MkdirAll(summaryDir, 0755)

	files := []struct {
		name    string
		content string
	}{
		{
			"doc1.md",
			"---\nid: doc1\ntype: note\ntitle: First Document\nstatus: active\ntags: [a, b]\n---\n\nThis is the first document body.\n",
		},
		{
			"doc2.md",
			"---\nid: doc2\ntype: decision\ntitle: Second Document\nstatus: pending\ntags: [c]\n---\n\nThis is the second document with more content here.\n",
		},
	}

	for _, f := range files {
		os.WriteFile(filepath.Join(summaryDir, f.name), []byte(f.content), 0644)
	}

	result, err := s.handleSummarize(map[string]interface{}{
		"path": "50-summary-test",
	})
	if err != nil {
		t.Fatalf("handleSummarize error: %v", err)
	}

	if !strings.Contains(result, "First Document") {
		t.Errorf("expected 'First Document' in result, got:\n%s", result)
	}
	if !strings.Contains(result, "Second Document") {
		t.Errorf("expected 'Second Document' in result, got:\n%s", result)
	}
	if !strings.Contains(result, "**Files:** 2") {
		t.Errorf("expected '**Files:** 2' in result, got:\n%s", result)
	}
}

func TestHandleSummarize_EmptyFolder(t *testing.T) {
	s, db := setupTestServer(t)
	defer db.Close()

	// Create empty folder
	emptyDir := filepath.Join(s.vaultPath, "60-empty")
	os.MkdirAll(emptyDir, 0755)

	result, err := s.handleSummarize(map[string]interface{}{
		"path": "60-empty",
	})
	if err != nil {
		t.Fatalf("handleSummarize error: %v", err)
	}

	if !strings.Contains(result, "No markdown files found") {
		t.Errorf("expected 'No markdown files found', got:\n%s", result)
	}
}

func TestHandleSummarize_NonexistentPath(t *testing.T) {
	s, db := setupTestServer(t)
	defer db.Close()

	_, err := s.handleSummarize(map[string]interface{}{
		"path": "nonexistent-folder-xyz",
	})
	if err == nil {
		t.Fatal("expected error for nonexistent path")
	}
}

func TestHandleListProjects(t *testing.T) {
	s, db := setupTestServer(t)
	defer db.Close()

	// Seed test data with projects
	addTestNote(t, db, "proj1_001", "Note One", "notes/n1.md", "note", "alpha", "body1", []string{})
	addTestNote(t, db, "proj2_001", "Note Two", "notes/n2.md", "note", "alpha", "body2", []string{})
	addTestNote(t, db, "proj3_001", "Note Three", "notes/n3.md", "note", "beta", "body3", []string{})

	result, err := s.handleListProjects(map[string]interface{}{})
	if err != nil {
		t.Fatalf("handleListProjects error: %v", err)
	}

	if !strings.Contains(result, "alpha") {
		t.Errorf("expected 'alpha' in result, got:\n%s", result)
	}
	if !strings.Contains(result, "beta") {
		t.Errorf("expected 'beta' in result, got:\n%s", result)
	}
}

func TestHandleListRecent(t *testing.T) {
	s, db := setupTestServer(t)
	defer db.Close()

	// Seed test data
	addTestNote(t, db, "recent_001", "Recent Note", "notes/recent.md", "note", "test", "recent body", []string{})

	result, err := s.handleListRecent(map[string]interface{}{
		"limit": float64(5),
	})
	if err != nil {
		t.Fatalf("handleListRecent error: %v", err)
	}

	if !strings.Contains(result, "Recent Note") {
		t.Errorf("expected 'Recent Note' in result, got:\n%s", result)
	}
}

func TestHandleGitStatus_NotARepo(t *testing.T) {
	s, db := setupTestServer(t)
	defer db.Close()

	result, err := s.handleGitStatus(map[string]interface{}{})
	if err != nil {
		t.Fatalf("handleGitStatus error: %v", err)
	}

	if !strings.Contains(result, "Not a git repository") {
		t.Errorf("expected 'Not a git repository', got:\n%s", result)
	}
}

func TestHandleLogAgentRun(t *testing.T) {
	s, db := setupTestServer(t)
	defer db.Close()

	result, err := s.handleLogAgentRun(map[string]interface{}{
		"agent_name":    "test-agent",
		"task":          "run unit tests",
		"files_changed": []interface{}{"file1.go", "file2.go"},
	})
	if err != nil {
		t.Fatalf("handleLogAgentRun error: %v", err)
	}

	if !strings.Contains(result, "Logged agent run:") {
		t.Errorf("expected 'Logged agent run:' in result, got:\n%s", result)
	}
	if !strings.Contains(result, "test-agent") {
		t.Errorf("expected agent name in result, got:\n%s", result)
	}
	if !strings.Contains(result, "run unit tests") {
		t.Errorf("expected task in result, got:\n%s", result)
	}
	if !strings.Contains(result, "2") {
		t.Errorf("expected files count in result, got:\n%s", result)
	}
}

func TestHandleLogAgentRun_MissingFields(t *testing.T) {
	s, db := setupTestServer(t)
	defer db.Close()

	_, err := s.handleLogAgentRun(map[string]interface{}{
		"agent_name": "test",
	})
	if err == nil {
		t.Fatal("expected error for missing task")
	}

	_, err = s.handleLogAgentRun(map[string]interface{}{
		"task": "test",
	})
	if err == nil {
		t.Fatal("expected error for missing agent_name")
	}
}

func TestHandleLogAgentRun_StructuredEvidence(t *testing.T) {
	s, db := setupTestServer(t)
	defer db.Close()

	snapshot := `{"hash":"sha256:abc","agentId":"agt_1","agentRevision":3,"agentTitle":"Coding Agent","task":"review change","conversationId":"conv_1","knowledgeScopes":[],"artifactScopes":[],"conversationScopes":[],"capabilityRefs":[],"contextPolicyRef":"","sections":[],"unresolved":[],"text":"compiled"}`
	if _, err := db.Exec(`
		INSERT INTO context_snapshots (hash, agent_id, agent_revision, task, conversation_id, context_json, created_at)
		VALUES ('sha256:abc', 'agt_1', 3, 'review change', 'conv_1', ?, datetime('now'))
	`, snapshot); err != nil {
		t.Fatal(err)
	}

	result, err := s.handleLogAgentRun(map[string]interface{}{
		"agent_name":               "coding-agent",
		"agent_id":                 "agt_1",
		"agent_revision":           float64(3),
		"task":                     "review change",
		"status":                   "succeeded",
		"conversation_id":          "conv_1",
		"context_hash":             "sha256:abc",
		"input_json":               `{"issue":123}`,
		"output_json":              `{"result":"ok"}`,
		"capability_snapshot_json": `{"github.read":true}`,
		"runtime_metadata_json":    `{"runtime":"test"}`,
	})
	if err != nil {
		t.Fatalf("handleLogAgentRun structured error: %v", err)
	}
	if !strings.Contains(result, "agt_1@3") {
		t.Fatalf("expected agent revision in result, got:\n%s", result)
	}

	var agentID, status, conversationID, contextHash, inputJSON string
	var revision int
	err = db.QueryRow(`
		SELECT agent_id, agent_revision, status, conversation_id, context_hash, input_json
		FROM agent_runs WHERE agent_name = ?
	`, "coding-agent").Scan(&agentID, &revision, &status, &conversationID, &contextHash, &inputJSON)
	if err != nil {
		t.Fatalf("query structured agent run: %v", err)
	}
	if agentID != "agt_1" || revision != 3 || status != "succeeded" {
		t.Fatalf("unexpected structured run identity: %s@%d status=%s", agentID, revision, status)
	}
	if conversationID != "conv_1" || contextHash != "sha256:abc" {
		t.Fatalf("unexpected run linkage: conversation=%s context=%s", conversationID, contextHash)
	}
	if inputJSON != `{"issue":123}` {
		t.Fatalf("unexpected normalized input JSON: %s", inputJSON)
	}
}

func TestHandleLogAgentRun_RejectsInvalidStructuredFields(t *testing.T) {
	s, db := setupTestServer(t)
	defer db.Close()

	_, err := s.handleLogAgentRun(map[string]interface{}{
		"agent_name":     "coding-agent",
		"agent_id":       "agt_1",
		"agent_revision": float64(0),
		"task":           "review",
	})
	if err == nil || !strings.Contains(err.Error(), "agent_revision") {
		t.Fatalf("expected invalid agent revision error, got %v", err)
	}

	_, err = s.handleLogAgentRun(map[string]interface{}{
		"agent_name": "coding-agent",
		"task":       "review",
		"status":     "mystery",
	})
	if err == nil || !strings.Contains(err.Error(), "status") {
		t.Fatalf("expected invalid status error, got %v", err)
	}

	_, err = s.handleLogAgentRun(map[string]interface{}{
		"agent_name": "coding-agent",
		"task":       "review",
		"input_json": "not-json",
	})
	if err == nil || !strings.Contains(err.Error(), "input_json") {
		t.Fatalf("expected invalid JSON error, got %v", err)
	}
}

func TestHandleLogObservation(t *testing.T) {
	s, db := setupTestServer(t)
	defer db.Close()

	if _, err := db.Exec(`
		INSERT INTO agent_runs (id, agent_name, task, status, created_at)
		VALUES ('run_obs_1', 'test-agent', 'test', 'succeeded', datetime('now'))
	`); err != nil {
		t.Fatalf("seed run: %v", err)
	}

	result, err := s.handleLogObservation(map[string]interface{}{
		"run_id":        "run_obs_1",
		"kind":          "tool",
		"name":          "github.search",
		"status":        "succeeded",
		"input_json":    `{"query":"agent"}`,
		"evidence_json": `{"result_count":4}`,
	})
	if err != nil {
		t.Fatalf("handleLogObservation error: %v", err)
	}
	if !strings.Contains(result, "github.search") {
		t.Fatalf("expected observation name in result, got:\n%s", result)
	}

	var kind, name, evidence string
	if err := db.QueryRow(`
		SELECT kind, name, evidence_json FROM run_observations WHERE run_id = ?
	`, "run_obs_1").Scan(&kind, &name, &evidence); err != nil {
		t.Fatalf("query observation: %v", err)
	}
	if kind != "tool" || name != "github.search" || evidence != `{"result_count":4}` {
		t.Fatalf("unexpected observation: kind=%s name=%s evidence=%s", kind, name, evidence)
	}
}

func TestHandleLogObservation_RejectsInvalidKind(t *testing.T) {
	s, db := setupTestServer(t)
	defer db.Close()

	_, err := s.handleLogObservation(map[string]interface{}{
		"run_id": "run_1",
		"kind":   "unknown",
		"name":   "bad",
	})
	if err == nil || !strings.Contains(err.Error(), "observation kind") {
		t.Fatalf("expected invalid observation kind error, got %v", err)
	}
}

func TestHandleLogEvaluation(t *testing.T) {
	s, db := setupTestServer(t)
	defer db.Close()

	if _, err := db.Exec(`
		INSERT INTO agent_runs (id, agent_name, task, status, created_at)
		VALUES ('run_eval_1', 'test-agent', 'test', 'succeeded', datetime('now'))
	`); err != nil {
		t.Fatalf("seed run: %v", err)
	}

	result, err := s.handleLogEvaluation(map[string]interface{}{
		"run_id":    "run_eval_1",
		"evaluator": "human",
		"name":      "correctness",
		"score":     float64(0.95),
		"rationale": "grounded answer",
	})
	if err != nil {
		t.Fatalf("handleLogEvaluation error: %v", err)
	}
	if !strings.Contains(result, "correctness") {
		t.Fatalf("expected evaluation name in result, got:\n%s", result)
	}

	var score float64
	var evaluator string
	if err := db.QueryRow(`
		SELECT score, evaluator FROM evaluations WHERE run_id = ?
	`, "run_eval_1").Scan(&score, &evaluator); err != nil {
		t.Fatalf("query evaluation: %v", err)
	}
	if score != 0.95 || evaluator != "human" {
		t.Fatalf("unexpected evaluation: score=%v evaluator=%s", score, evaluator)
	}
}

func TestHandleProposePromotion(t *testing.T) {
	s, db := setupTestServer(t)
	defer db.Close()

	result, err := s.handleProposePromotion(map[string]interface{}{
		"agent_id":               "agt_1",
		"target_kind":            "memory",
		"candidate":              "Run generated-code checks before formatting.",
		"rationale":              "Repeated successful behavior",
		"source_observation_ids": []interface{}{"obs_1", "obs_2"},
	})
	if err != nil {
		t.Fatalf("handleProposePromotion error: %v", err)
	}
	if !strings.Contains(result, "proposed") {
		t.Fatalf("expected proposed status in result, got:\n%s", result)
	}

	var status, targetKind, sourceIDs string
	if err := db.QueryRow(`
		SELECT status, target_kind, source_observation_ids_json
		FROM promotion_records WHERE agent_id = ?
	`, "agt_1").Scan(&status, &targetKind, &sourceIDs); err != nil {
		t.Fatalf("query promotion: %v", err)
	}
	if status != "proposed" || targetKind != "memory" || sourceIDs != `["obs_1","obs_2"]` {
		t.Fatalf("unexpected promotion: status=%s kind=%s sources=%s", status, targetKind, sourceIDs)
	}
}

func TestHandleProposePromotion_RequiresEvidence(t *testing.T) {
	s, db := setupTestServer(t)
	defer db.Close()

	_, err := s.handleProposePromotion(map[string]interface{}{
		"agent_id":    "agt_1",
		"target_kind": "memory",
		"candidate":   "Remember this",
	})
	if err == nil || !strings.Contains(err.Error(), "evidence") {
		t.Fatalf("expected evidence requirement error, got %v", err)
	}
}

func TestHandleReviewPromotionApprove(t *testing.T) {
	s, db := setupTestServer(t)
	defer db.Close()

	if _, err := db.Exec(`
		INSERT INTO promotion_records (
			id, agent_id, target_kind, status, candidate,
			source_observation_ids_json, created_at
		) VALUES ('promo_review_1', 'agt_1', 'memory', 'proposed', 'Prefer focused tests.', '["obs_1"]', datetime('now'))
	`); err != nil {
		t.Fatalf("seed promotion: %v", err)
	}

	result, err := s.handleReviewPromotion(map[string]interface{}{
		"promotion_id": "promo_review_1",
		"decision":     "approve",
		"reviewer":     "human:david",
		"note":         "Evidence is consistent.",
	})
	if err != nil {
		t.Fatalf("handleReviewPromotion error: %v", err)
	}
	if !strings.Contains(result, "approved") {
		t.Fatalf("expected approved result, got:\n%s", result)
	}

	var status, reviewer, note string
	var reviewedAt interface{}
	if err := db.QueryRow(`
		SELECT status, reviewed_by, review_note, reviewed_at
		FROM promotion_records WHERE id = 'promo_review_1'
	`).Scan(&status, &reviewer, &note, &reviewedAt); err != nil {
		t.Fatalf("query reviewed promotion: %v", err)
	}
	if status != "approved" || reviewer != "human:david" || note != "Evidence is consistent." || reviewedAt == nil {
		t.Fatalf("unexpected review state: status=%s reviewer=%s note=%s reviewedAt=%v", status, reviewer, note, reviewedAt)
	}
}

func TestHandleReviewPromotionRejectsInvalidTransition(t *testing.T) {
	s, db := setupTestServer(t)
	defer db.Close()

	if _, err := db.Exec(`
		INSERT INTO promotion_records (
			id, agent_id, target_kind, status, candidate,
			source_observation_ids_json, created_at
		) VALUES ('promo_review_2', 'agt_1', 'memory', 'committed', 'Already done.', '["obs_1"]', datetime('now'))
	`); err != nil {
		t.Fatalf("seed promotion: %v", err)
	}

	_, err := s.handleReviewPromotion(map[string]interface{}{
		"promotion_id": "promo_review_2",
		"decision":     "approve",
		"reviewer":     "human:david",
	})
	if err == nil || !strings.Contains(err.Error(), "transition") {
		t.Fatalf("expected invalid transition error, got %v", err)
	}
}

func TestHandleCommitPromotionRequiresCanonicalContent(t *testing.T) {
	s, db := setupTestServer(t)
	defer db.Close()

	candidate := "Prefer focused tests."
	if _, err := db.Exec(`
		INSERT INTO promotion_records (
			id, agent_id, target_kind, status, candidate,
			source_observation_ids_json, created_at, reviewed_at
		) VALUES ('promo_commit_1', 'agt_1', 'memory', 'approved', ?, '["obs_1"]', datetime('now'), datetime('now'))
	`, candidate); err != nil {
		t.Fatalf("seed promotion: %v", err)
	}

	addTestNote(t, db, "note_memory_1", "Agent memory", "10-notes/agent-memory.md", "note", "", candidate, []string{"agent-memory"})
	fullPath := filepath.Join(s.vaultPath, "10-notes", "agent-memory.md")
	if err := os.MkdirAll(filepath.Dir(fullPath), 0755); err != nil {
		t.Fatalf("create note dir: %v", err)
	}
	if err := os.WriteFile(fullPath, []byte("---\nid: note_memory_1\ntype: note\ntitle: Agent memory\n---\n\n"+candidate+"\n"), 0644); err != nil {
		t.Fatalf("write canonical note: %v", err)
	}

	result, err := s.handleCommitPromotion(map[string]interface{}{
		"promotion_id":   "promo_commit_1",
		"target_note_id": "note_memory_1",
	})
	if err != nil {
		t.Fatalf("handleCommitPromotion error: %v", err)
	}
	if !strings.Contains(result, "committed") {
		t.Fatalf("expected committed result, got:\n%s", result)
	}

	var status, targetNote string
	var committedAt interface{}
	if err := db.QueryRow(`
		SELECT status, target_note_id, committed_at
		FROM promotion_records WHERE id = 'promo_commit_1'
	`).Scan(&status, &targetNote, &committedAt); err != nil {
		t.Fatalf("query committed promotion: %v", err)
	}
	if status != "committed" || targetNote != "note_memory_1" || committedAt == nil {
		t.Fatalf("unexpected commit state: status=%s target=%s committedAt=%v", status, targetNote, committedAt)
	}
}

func TestHandleCommitPromotionRejectsMissingCandidateContent(t *testing.T) {
	s, db := setupTestServer(t)
	defer db.Close()

	if _, err := db.Exec(`
		INSERT INTO promotion_records (
			id, agent_id, target_kind, status, candidate,
			source_observation_ids_json, created_at, reviewed_at
		) VALUES ('promo_commit_2', 'agt_1', 'memory', 'approved', 'Required memory text.', '["obs_1"]', datetime('now'), datetime('now'))
	`); err != nil {
		t.Fatalf("seed promotion: %v", err)
	}

	addTestNote(t, db, "note_memory_2", "Agent memory", "10-notes/agent-memory-2.md", "note", "", "Different text.", nil)
	fullPath := filepath.Join(s.vaultPath, "10-notes", "agent-memory-2.md")
	if err := os.MkdirAll(filepath.Dir(fullPath), 0755); err != nil {
		t.Fatalf("create note dir: %v", err)
	}
	if err := os.WriteFile(fullPath, []byte("---\nid: note_memory_2\ntype: note\ntitle: Agent memory\n---\n\nDifferent text.\n"), 0644); err != nil {
		t.Fatalf("write canonical note: %v", err)
	}

	_, err := s.handleCommitPromotion(map[string]interface{}{
		"promotion_id":   "promo_commit_2",
		"target_note_id": "note_memory_2",
	})
	if err == nil || !strings.Contains(err.Error(), "candidate") {
		t.Fatalf("expected candidate content error, got %v", err)
	}
}

func TestHandleCreateEvaluationDataset(t *testing.T) {
	s, db := setupTestServer(t)
	defer db.Close()

	result, err := s.handleCreateEvaluationDataset(map[string]interface{}{
		"name":        "Golden path",
		"description": "Core agent behavior",
		"agent_id":    "agt_1",
	})
	if err != nil {
		t.Fatalf("handleCreateEvaluationDataset error: %v", err)
	}
	if !strings.Contains(result, "Golden path") {
		t.Fatalf("expected dataset name in result, got:\n%s", result)
	}

	var name, agentID string
	if err := db.QueryRow(`
		SELECT name, agent_id FROM evaluation_datasets WHERE name = 'Golden path'
	`).Scan(&name, &agentID); err != nil {
		t.Fatalf("query dataset: %v", err)
	}
	if name != "Golden path" || agentID != "agt_1" {
		t.Fatalf("unexpected dataset: name=%s agent=%s", name, agentID)
	}
}

func TestHandleAddEvaluationCase(t *testing.T) {
	s, db := setupTestServer(t)
	defer db.Close()

	if _, err := db.Exec(`
		INSERT INTO evaluation_datasets (id, name, created_at)
		VALUES ('ds_1', 'Golden path', datetime('now'))
	`); err != nil {
		t.Fatalf("seed dataset: %v", err)
	}

	result, err := s.handleAddEvaluationCase(map[string]interface{}{
		"dataset_id":    "ds_1",
		"name":          "Create note",
		"input_json":    `{"prompt":"create a note"}`,
		"expected_json": `{"type":"note"}`,
		"tags":          []interface{}{"golden", "notes"},
	})
	if err != nil {
		t.Fatalf("handleAddEvaluationCase error: %v", err)
	}
	if !strings.Contains(result, "Create note") {
		t.Fatalf("expected case name in result, got:\n%s", result)
	}

	var inputJSON, expectedJSON, tagsJSON string
	if err := db.QueryRow(`
		SELECT input_json, expected_json, tags_json
		FROM evaluation_cases WHERE dataset_id = 'ds_1'
	`).Scan(&inputJSON, &expectedJSON, &tagsJSON); err != nil {
		t.Fatalf("query evaluation case: %v", err)
	}
	if inputJSON != `{"prompt":"create a note"}` || expectedJSON != `{"type":"note"}` || tagsJSON != `["golden","notes"]` {
		t.Fatalf("unexpected case payloads: input=%s expected=%s tags=%s", inputJSON, expectedJSON, tagsJSON)
	}
}

func TestHandleRecordExperimentAndResult(t *testing.T) {
	s, db := setupTestServer(t)
	defer db.Close()

	if _, err := db.Exec(`
		INSERT INTO evaluation_datasets (id, name, created_at)
		VALUES ('ds_exp_1', 'Golden path', datetime('now'))
	`); err != nil {
		t.Fatalf("seed dataset: %v", err)
	}
	if _, err := db.Exec(`
		INSERT INTO evaluation_cases (id, dataset_id, name, input_json, created_at)
		VALUES ('case_exp_1', 'ds_exp_1', 'Create note', '{"prompt":"create"}', datetime('now'))
	`); err != nil {
		t.Fatalf("seed case: %v", err)
	}

	experimentResult, err := s.handleRecordExperiment(map[string]interface{}{
		"dataset_id":     "ds_exp_1",
		"name":           "baseline",
		"agent_id":       "agt_1",
		"agent_revision": float64(2),
		"status":         "completed",
		"config_json":    `{"model":"test"}`,
	})
	if err != nil {
		t.Fatalf("handleRecordExperiment error: %v", err)
	}
	if !strings.Contains(experimentResult, "baseline") {
		t.Fatalf("expected experiment name in result, got:\n%s", experimentResult)
	}

	var experimentID string
	if err := db.QueryRow(`
		SELECT id FROM experiments WHERE dataset_id = 'ds_exp_1' AND name = 'baseline'
	`).Scan(&experimentID); err != nil {
		t.Fatalf("query experiment: %v", err)
	}

	result, err := s.handleRecordExperimentResult(map[string]interface{}{
		"experiment_id": experimentID,
		"case_id":       "case_exp_1",
		"score":         float64(0.9),
		"label":         "pass",
		"metadata_json": `{"latency_ms":42}`,
	})
	if err != nil {
		t.Fatalf("handleRecordExperimentResult error: %v", err)
	}
	if !strings.Contains(result, "case_exp_1") {
		t.Fatalf("expected case id in result, got:\n%s", result)
	}

	var score float64
	var label string
	if err := db.QueryRow(`
		SELECT score, label FROM experiment_results
		WHERE experiment_id = ? AND case_id = 'case_exp_1'
	`, experimentID).Scan(&score, &label); err != nil {
		t.Fatalf("query experiment result: %v", err)
	}
	if score != 0.9 || label != "pass" {
		t.Fatalf("unexpected experiment result: score=%v label=%s", score, label)
	}
}

func TestHandleRecordExperimentResultRejectsCaseFromOtherDataset(t *testing.T) {
	s, db := setupTestServer(t)
	defer db.Close()

	if _, err := db.Exec(`
		INSERT INTO evaluation_datasets (id, name, created_at) VALUES
		('ds_a', 'A', datetime('now')),
		('ds_b', 'B', datetime('now'))
	`); err != nil {
		t.Fatalf("seed datasets: %v", err)
	}
	if _, err := db.Exec(`
		INSERT INTO evaluation_cases (id, dataset_id, name, input_json, created_at)
		VALUES ('case_b', 'ds_b', 'B case', '{}', datetime('now'))
	`); err != nil {
		t.Fatalf("seed case: %v", err)
	}
	if _, err := db.Exec(`
		INSERT INTO experiments (id, dataset_id, name, agent_id, agent_revision, status, config_json, created_at)
		VALUES ('exp_a', 'ds_a', 'A exp', 'agt_1', 1, 'completed', '{}', datetime('now'))
	`); err != nil {
		t.Fatalf("seed experiment: %v", err)
	}

	_, err := s.handleRecordExperimentResult(map[string]interface{}{
		"experiment_id": "exp_a",
		"case_id":       "case_b",
		"label":         "pass",
	})
	if err == nil || !strings.Contains(err.Error(), "dataset") {
		t.Fatalf("expected dataset mismatch error, got %v", err)
	}
}

func TestHandleListPromotionsDefaultsToPending(t *testing.T) {
	s, db := setupTestServer(t)
	defer db.Close()

	if _, err := db.Exec(`
		INSERT INTO promotion_records (
			id, agent_id, target_kind, status, candidate, rationale,
			source_run_ids_json, source_observation_ids_json, source_evaluation_ids_json,
			created_at
		) VALUES
		('promo_pending', 'agt_1', 'memory', 'proposed', 'Pending memory', 'needs review', '["run_1"]', '["obs_1"]', '[]', '2026-09-30T10:00:00Z'),
		('promo_done', 'agt_1', 'knowledge', 'committed', 'Committed fact', '', '["run_2"]', '[]', '[]', '2026-09-30T09:00:00Z')
	`); err != nil {
		t.Fatalf("seed promotions: %v", err)
	}

	result, err := s.handleListPromotions(map[string]interface{}{})
	if err != nil {
		t.Fatalf("handleListPromotions error: %v", err)
	}
	if !strings.Contains(result, "promo_pending") || !strings.Contains(result, "Pending memory") {
		t.Fatalf("expected pending promotion, got:\n%s", result)
	}
	if strings.Contains(result, "promo_done") {
		t.Fatalf("did not expect committed promotion by default, got:\n%s", result)
	}
}

func TestHandleGetEvaluationDatasetIncludesCases(t *testing.T) {
	s, db := setupTestServer(t)
	defer db.Close()

	if _, err := db.Exec(`
		INSERT INTO evaluation_datasets (id, name, description, agent_id, created_at)
		VALUES ('ds_read_1', 'Golden path', 'Core behavior', 'agt_1', '2026-09-30T10:00:00Z')
	`); err != nil {
		t.Fatalf("seed dataset: %v", err)
	}
	if _, err := db.Exec(`
		INSERT INTO evaluation_cases (id, dataset_id, name, input_json, expected_json, tags_json, created_at)
		VALUES
		('case_read_1', 'ds_read_1', 'Create note', '{"prompt":"create"}', '{"type":"note"}', '["golden"]', '2026-09-30T10:01:00Z'),
		('case_read_2', 'ds_read_1', 'Search note', '{"query":"pricing"}', NULL, '[]', '2026-09-30T10:02:00Z')
	`); err != nil {
		t.Fatalf("seed cases: %v", err)
	}

	result, err := s.handleGetEvaluationDataset(map[string]interface{}{"dataset_id": "ds_read_1"})
	if err != nil {
		t.Fatalf("handleGetEvaluationDataset error: %v", err)
	}
	for _, want := range []string{"Golden path", "Create note", "Search note", "golden"} {
		if !strings.Contains(result, want) {
			t.Fatalf("expected %q in dataset output, got:\n%s", want, result)
		}
	}
}

func TestHandleGetExperimentIncludesResults(t *testing.T) {
	s, db := setupTestServer(t)
	defer db.Close()

	if _, err := db.Exec(`
		INSERT INTO evaluation_datasets (id, name, created_at)
		VALUES ('ds_exp_read', 'Golden path', '2026-09-30T10:00:00Z')
	`); err != nil {
		t.Fatalf("seed dataset: %v", err)
	}
	if _, err := db.Exec(`
		INSERT INTO evaluation_cases (id, dataset_id, name, input_json, created_at)
		VALUES ('case_exp_read', 'ds_exp_read', 'Create note', '{"prompt":"create"}', '2026-09-30T10:01:00Z')
	`); err != nil {
		t.Fatalf("seed case: %v", err)
	}
	if _, err := db.Exec(`
		INSERT INTO experiments (
			id, dataset_id, name, agent_id, agent_revision, status, config_json, created_at, completed_at
		) VALUES (
			'exp_read_1', 'ds_exp_read', 'baseline', 'agt_1', 3, 'completed',
			'{"model":"test"}', '2026-09-30T10:02:00Z', '2026-09-30T10:03:00Z'
		)
	`); err != nil {
		t.Fatalf("seed experiment: %v", err)
	}
	if _, err := db.Exec(`
		INSERT INTO experiment_results (
			experiment_id, case_id, run_id, score, label, metadata_json, created_at
		) VALUES (
			'exp_read_1', 'case_exp_read', NULL, 0.95, 'pass',
			'{"latency_ms":42}', '2026-09-30T10:03:00Z'
		)
	`); err != nil {
		t.Fatalf("seed result: %v", err)
	}

	result, err := s.handleGetExperiment(map[string]interface{}{"experiment_id": "exp_read_1"})
	if err != nil {
		t.Fatalf("handleGetExperiment error: %v", err)
	}
	for _, want := range []string{"baseline", "case_exp_read", "0.95", "pass", "latency_ms"} {
		if !strings.Contains(result, want) {
			t.Fatalf("expected %q in experiment output, got:\n%s", want, result)
		}
	}
}

func TestSanitizeFilename(t *testing.T) {
	cases := []struct {
		input    string
		expected string
	}{
		{"Hello World", "hello-world"},
		{"Special!@#Chars", "specialchars"},
		{"  spaces  ", "spaces"},
		{"a--b---c", "a-b-c"},
		{"", "untitled"},
		{"UPPERCASE", "uppercase"},
		{"Mixed123-Text", "mixed123-text"},
	}

	for _, c := range cases {
		result := sanitizeFilename(c.input)
		if result != c.expected {
			t.Errorf("sanitizeFilename(%q) = %q, want %q", c.input, result, c.expected)
		}
	}
}

func TestSchemaHelpers(t *testing.T) {
	// Test schemaString
	s := schemaString("A description")
	if s["type"] != "string" {
		t.Errorf("expected type=string, got %v", s["type"])
	}
	if s["description"] != "A description" {
		t.Errorf("expected description, got %v", s["description"])
	}

	// Test schemaStringEnum
	senum := schemaStringEnum("Type", []string{"a", "b"})
	if _, ok := senum["enum"]; !ok {
		t.Error("expected enum key")
	}

	// Test schemaInt
	si := schemaInt("Limit", 10)
	if si["type"] != "integer" {
		t.Errorf("expected type=integer, got %v", si["type"])
	}
	if si["default"] != 10 {
		t.Errorf("expected default=10, got %v", si["default"])
	}

	// Test schemaStringArray
	sa := schemaStringArray("Tags")
	if sa["type"] != "array" {
		t.Errorf("expected type=array, got %v", sa["type"])
	}

	// Test makeSchema
	schema := makeSchema(map[string]interface{}{
		"name": schemaString("Name"),
	}, []string{"name"})
	if schema["type"] != "object" {
		t.Errorf("expected schema type=object, got %v", schema["type"])
	}
	props, ok := schema["properties"].(map[string]interface{})
	if !ok || props["name"] == nil {
		t.Error("expected properties with name")
	}
	required, ok := schema["required"].([]string)
	if !ok || len(required) != 1 || required[0] != "name" {
		t.Errorf("expected required=[name], got %v", schema["required"])
	}
}

func TestCurrentTimestamp(t *testing.T) {
	ts := currentTimestamp()
	if ts == "" {
		t.Error("expected non-empty timestamp")
	}
	// Should be a valid RFC3339-ish timestamp
	if !strings.Contains(ts, "T") {
		t.Errorf("expected RFC3339 format with 'T', got: %s", ts)
	}
}

func TestMakeSchema_NoRequired(t *testing.T) {
	schema := makeSchema(map[string]interface{}{
		"opt": schemaString("Optional"),
	}, nil)
	if _, hasRequired := schema["required"]; hasRequired {
		t.Error("expected no required key when nil")
	}
}

// Benchmark tool registration
func BenchmarkRegisterTools(b *testing.B) {
	tmpDir := b.TempDir()
	os.MkdirAll(filepath.Join(tmpDir, ".agentvault"), 0755)
	database, err := db.Open(tmpDir)
	if err != nil {
		b.Fatalf("open db: %v", err)
	}
	defer database.Close()

	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		srv := NewServer(tmpDir, database)
		srv.RegisterTools()
	}
}

func TestHandleSearch_TypeFilter(t *testing.T) {
	s, db := setupTestServer(t)
	defer db.Close()

	addTestNote(t, db, "type_note_001", "A Note", "notes/n1.md", "note", "p1", "body", []string{})
	addTestNote(t, db, "type_dec_001", "A Decision", "decisions/d1.md", "decision", "p1", "body", []string{})

	result, err := s.handleSearch(map[string]interface{}{
		"query": "",
		"type":  "decision",
		"limit": float64(10),
	})
	if err != nil {
		t.Fatalf("handleSearch error: %v", err)
	}

	if !strings.Contains(result, "A Decision") {
		t.Errorf("expected 'A Decision' in result, got:\n%s", result)
	}
	if strings.Contains(result, "A Note") {
		t.Errorf("did not expect 'A Note' when filtering by decision type, got:\n%s", result)
	}
}

func TestHandleSearch_Limit(t *testing.T) {
	s, db := setupTestServer(t)
	defer db.Close()

	// Add many notes
	for i := 0; i < 20; i++ {
		id := fmt.Sprintf("limit_%02d", i)
		addTestNote(t, db, id, fmt.Sprintf("Note %d", i), fmt.Sprintf("notes/n%d.md", i), "note", "", fmt.Sprintf("body %d", i), []string{})
	}

	result, err := s.handleSearch(map[string]interface{}{
		"query": "",
		"limit": float64(5),
	})
	if err != nil {
		t.Fatalf("handleSearch error: %v", err)
	}

	// Should be limited to 5 results (non-FTS returns everything with limit)
	// We just check it doesn't crash and returns valid markdown
	if !strings.Contains(result, "Search Results") {
		t.Errorf("expected 'Search Results' header, got:\n%s", result)
	}
}

func TestHandleCapture_OnlyTitle(t *testing.T) {
	s, db := setupTestServer(t)
	defer db.Close()

	result, err := s.handleCapture(map[string]interface{}{
		"title": "Minimal Capture",
	})
	if err != nil {
		t.Fatalf("handleCapture error: %v", err)
	}

	if !strings.Contains(result, "Captured to inbox:") {
		t.Errorf("expected capture result, got:\n%s", result)
	}
}

func TestHandleAsk_MissingQuestion(t *testing.T) {
	s, db := setupTestServer(t)
	defer db.Close()

	_, err := s.handleAsk(map[string]interface{}{})
	if err == nil {
		t.Fatal("expected error for missing question")
	}
	if !strings.Contains(err.Error(), "question is required") {
		t.Errorf("expected 'question is required' in error, got: %v", err)
	}
}

func TestHandleAsk_NoConfig(t *testing.T) {
	s, db := setupTestServer(t)
	defer db.Close()

	_, err := s.handleAsk(map[string]interface{}{"question": "what is this?"})
	if err == nil {
		t.Fatal("expected error when config is missing")
	}
	if !strings.Contains(err.Error(), "failed to load config") {
		t.Errorf("expected 'failed to load config' in error, got: %v", err)
	}
}

func TestHandleAsk_MockProviderNoResults(t *testing.T) {
	s, db := setupTestServer(t)
	defer db.Close()

	cfgPath := filepath.Join(s.vaultPath, ".agentvault", "config.json")
	cfg := map[string]interface{}{
		"ai": map[string]string{"provider": "mock"},
	}
	data, err := json.Marshal(cfg)
	if err != nil {
		t.Fatalf("marshal config: %v", err)
	}
	if err := os.WriteFile(cfgPath, data, 0644); err != nil {
		t.Fatalf("write config: %v", err)
	}

	result, err := s.handleAsk(map[string]interface{}{"question": "what is this?"})
	if err != nil {
		t.Fatalf("handleAsk error: %v", err)
	}
	if !strings.Contains(result, "couldn't find any relevant notes") {
		t.Errorf("expected no-information answer, got:\n%s", result)
	}
	if !strings.Contains(result, "Sources:") {
		t.Errorf("expected sources section, got:\n%s", result)
	}
}

func TestHandleReadNote_ByPath(t *testing.T) {
	s, db := setupTestServer(t)
	defer db.Close()

	noteDir := filepath.Join(s.vaultPath, "10-notes")
	os.MkdirAll(noteDir, 0755)
	noteContent := "---\nid: note_path_001\ntype: note\ntitle: Path Readable\nproject: testproj\n---\n\nBody by path.\n"
	os.WriteFile(filepath.Join(noteDir, "path-readable.md"), []byte(noteContent), 0644)
	addTestNote(t, db, "note_path_001", "Path Readable", "10-notes/path-readable.md", "note", "testproj", "Body by path.", []string{})

	result, err := s.handleReadNote(map[string]interface{}{"id": "10-notes/path-readable.md"})
	if err != nil {
		t.Fatalf("handleReadNote error: %v", err)
	}
	if !strings.Contains(result, "Path Readable") {
		t.Errorf("expected title, got:\n%s", result)
	}
	if !strings.Contains(result, "Body by path") {
		t.Errorf("expected body, got:\n%s", result)
	}
}

func TestHandleReadNote_FallbackDB(t *testing.T) {
	s, db := setupTestServer(t)
	defer db.Close()

	// Seed the database without creating the underlying file.
	addTestNote(t, db, "note_fallback_001", "Fallback Note", "notes/fallback.md", "note", "fbproj", "Fallback body content.", []string{"fb"})

	result, err := s.handleReadNote(map[string]interface{}{"id": "note_fallback_001"})
	if err != nil {
		t.Fatalf("handleReadNote error: %v", err)
	}
	if !strings.Contains(result, "Fallback Note") {
		t.Errorf("expected title, got:\n%s", result)
	}
	if !strings.Contains(result, "Fallback body content") {
		t.Errorf("expected body snippet, got:\n%s", result)
	}
}

func TestHandleCreateNote_MissingFields(t *testing.T) {
	s, db := setupTestServer(t)
	defer db.Close()

	_, err := s.handleCreateNote(map[string]interface{}{"type": "note"})
	if err == nil {
		t.Fatal("expected error for missing title")
	}

	_, err = s.handleCreateNote(map[string]interface{}{"title": "Test"})
	if err == nil {
		t.Fatal("expected error for missing type")
	}
}

func TestHandleCapture_SourceURLNoText(t *testing.T) {
	s, db := setupTestServer(t)
	defer db.Close()

	result, err := s.handleCapture(map[string]interface{}{
		"title":      "URL Capture",
		"source_url": "https://example.org",
	})
	if err != nil {
		t.Fatalf("handleCapture error: %v", err)
	}
	if !strings.Contains(result, "Captured to inbox:") {
		t.Errorf("expected capture result, got:\n%s", result)
	}

	inboxDir := filepath.Join(s.vaultPath, "00-inbox")
	entries, err := os.ReadDir(inboxDir)
	if err != nil || len(entries) == 0 {
		t.Fatalf("expected capture file: %v", err)
	}
	content, err := os.ReadFile(filepath.Join(inboxDir, entries[0].Name()))
	if err != nil {
		t.Fatalf("read capture: %v", err)
	}
	if !strings.Contains(string(content), "Source: <https://example.org>") {
		t.Errorf("expected source URL in content, got:\n%s", string(content))
	}
}

func TestHandleSummarize_FileNotDir(t *testing.T) {
	s, db := setupTestServer(t)
	defer db.Close()

	filePath := filepath.Join(s.vaultPath, "not-a-dir.md")
	if err := os.WriteFile(filePath, []byte("# hello"), 0644); err != nil {
		t.Fatalf("write file: %v", err)
	}

	_, err := s.handleSummarize(map[string]interface{}{"path": "not-a-dir.md"})
	if err == nil {
		t.Fatal("expected error for file path")
	}
	if !strings.Contains(err.Error(), "path is not a directory") {
		t.Errorf("expected 'path is not a directory', got: %v", err)
	}
}

func TestHandleListProjects_Empty(t *testing.T) {
	s, db := setupTestServer(t)
	defer db.Close()

	result, err := s.handleListProjects(map[string]interface{}{})
	if err != nil {
		t.Fatalf("handleListProjects error: %v", err)
	}
	if !strings.Contains(result, "**Total project notes:** 0") {
		t.Errorf("expected total 0, got:\n%s", result)
	}
}

func TestHandleListRecent_NoResults(t *testing.T) {
	s, db := setupTestServer(t)
	defer db.Close()

	result, err := s.handleListRecent(map[string]interface{}{"limit": float64(10)})
	if err != nil {
		t.Fatalf("handleListRecent error: %v", err)
	}
	if !strings.Contains(result, "No notes found") {
		t.Errorf("expected 'No notes found', got:\n%s", result)
	}
}

func TestHandleGitStatus_Repo(t *testing.T) {
	if _, err := exec.LookPath("git"); err != nil {
		t.Skip("git not installed")
	}

	s, db := setupTestServer(t)
	defer db.Close()

	// Ignore the .agentvault directory so the repo can be clean.
	gitIgnore := filepath.Join(s.vaultPath, ".gitignore")
	if err := os.WriteFile(gitIgnore, []byte(".agentvault/\n"), 0644); err != nil {
		t.Fatalf("write gitignore: %v", err)
	}

	runGit := func(args ...string) {
		cmd := exec.Command("git", append([]string{"-C", s.vaultPath}, args...)...)
		if out, err := cmd.CombinedOutput(); err != nil {
			t.Fatalf("git %v failed: %v\n%s", args, err, out)
		}
	}

	runGit("init")
	runGit("config", "user.email", "test@example.com")
	runGit("config", "user.name", "Test User")
	runGit("add", ".")
	runGit("commit", "-m", "initial")

	result, err := s.handleGitStatus(map[string]interface{}{})
	if err != nil {
		t.Fatalf("handleGitStatus error: %v", err)
	}
	if !strings.Contains(result, "Working tree clean") {
		t.Errorf("expected clean status, got:\n%s", result)
	}

	dirtyFile := filepath.Join(s.vaultPath, "dirty.txt")
	if err := os.WriteFile(dirtyFile, []byte("change"), 0644); err != nil {
		t.Fatalf("write dirty file: %v", err)
	}

	result, err = s.handleGitStatus(map[string]interface{}{})
	if err != nil {
		t.Fatalf("handleGitStatus error: %v", err)
	}
	if !strings.Contains(result, "dirty.txt") {
		t.Errorf("expected dirty file in status, got:\n%s", result)
	}
}

func TestLogWrite(t *testing.T) {
	s, db := setupTestServer(t)
	defer db.Close()

	s.logWrite("test_op", "notes/test.md")

	var count int
	if err := db.QueryRow(`SELECT COUNT(*) FROM agent_runs WHERE task = ?`, "test_op").Scan(&count); err != nil {
		t.Fatalf("query agent_runs: %v", err)
	}
	if count != 1 {
		t.Errorf("expected 1 run logged, got %d", count)
	}
}


func TestHandleCompileAndGetContext(t *testing.T) {
	s, database := setupTestServer(t)
	defer database.Close()

	if err := os.MkdirAll(filepath.Join(s.vaultPath, "75-agents"), 0755); err != nil {
		t.Fatal(err)
	}
	if err := os.MkdirAll(filepath.Join(s.vaultPath, "10-notes"), 0755); err != nil {
		t.Fatal(err)
	}

	agentContent := `---
id: agt_mcp_context
type: agent
title: MCP Context Agent
revision: 4
identity_ref: identity_mcp_context
memory_refs: [memory_mcp_context]
knowledge_scopes: []
artifact_scopes: []
conversation_scopes: []
capability_refs: [github]
context_policy_ref: ""
---
Build auditable contexts.
`
	identityContent := `---
id: identity_mcp_context
type: note
title: MCP Identity
---
Prefer explicit provenance.
`
	memoryContent := `---
id: memory_mcp_context
type: note
title: MCP Memory
---
Keep runtime execution outside AgentVault.
`
	if err := os.WriteFile(filepath.Join(s.vaultPath, "75-agents", "mcp-context.md"), []byte(agentContent), 0644); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(s.vaultPath, "10-notes", "mcp-identity.md"), []byte(identityContent), 0644); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(s.vaultPath, "10-notes", "mcp-memory.md"), []byte(memoryContent), 0644); err != nil {
		t.Fatal(err)
	}

	addTestNote(t, database, "agt_mcp_context", "MCP Context Agent", "75-agents/mcp-context.md", "agent", "", "Build auditable contexts.", nil)
	addTestNote(t, database, "identity_mcp_context", "MCP Identity", "10-notes/mcp-identity.md", "note", "", "Prefer explicit provenance.", nil)
	addTestNote(t, database, "memory_mcp_context", "MCP Memory", "10-notes/mcp-memory.md", "note", "", "Keep runtime execution outside AgentVault.", nil)

	result, err := s.handleCompileContext(map[string]interface{}{
		"agent_id": "agt_mcp_context",
		"task":     "Compile a stable context.",
	})
	if err != nil {
		t.Fatalf("handleCompileContext: %v", err)
	}
	if !strings.Contains(result, "sha256:") || !strings.Contains(result, "MCP Identity") || !strings.Contains(result, "MCP Memory") {
		t.Fatalf("unexpected compiled context output:\n%s", result)
	}

	var hash string
	if err := database.QueryRow(`SELECT hash FROM context_snapshots ORDER BY created_at DESC LIMIT 1`).Scan(&hash); err != nil {
		t.Fatalf("query context snapshot: %v", err)
	}
	stored, err := s.handleGetContextSnapshot(map[string]interface{}{"hash": hash})
	if err != nil {
		t.Fatalf("handleGetContextSnapshot: %v", err)
	}
	if !strings.Contains(stored, hash) || !strings.Contains(stored, "Compile a stable context.") {
		t.Fatalf("unexpected stored context output:\n%s", stored)
	}
}


func TestHandleGetRunAudit(t *testing.T) {
	s, database := setupTestServer(t)
	defer database.Close()

	snapshot := `{"hash":"sha256:mcp-run-context","agentId":"agt_mcp_run","agentRevision":2,"agentTitle":"MCP Run Agent","task":"audit","conversationId":"","knowledgeScopes":[],"artifactScopes":[],"conversationScopes":[],"capabilityRefs":[],"contextPolicyRef":"","sections":[{"kind":"identity","sourceId":"identity_mcp_run","sourcePath":"10-notes/id.md","title":"Identity","content":"Be precise."}],"unresolved":[],"text":"compiled"}`
	if _, err := database.Exec(`
		INSERT INTO context_snapshots (hash, agent_id, agent_revision, task, context_json, created_at)
		VALUES ('sha256:mcp-run-context', 'agt_mcp_run', 2, 'audit', ?, datetime('now'))
	`, snapshot); err != nil {
		t.Fatal(err)
	}
	run, err := agentstate.RecordRun(database, agentstate.RunRecord{
		AgentName: "mcp-agent", AgentID: "agt_mcp_run", AgentRevision: 2,
		Task: "audit", Status: agentstate.RunSucceeded, ContextHash: "sha256:mcp-run-context",
	})
	if err != nil {
		t.Fatal(err)
	}

	result, err := s.handleGetRunAudit(map[string]interface{}{"run_id": run.ID})
	if err != nil {
		t.Fatalf("handleGetRunAudit: %v", err)
	}
	if !strings.Contains(result, run.ID) || !strings.Contains(result, "sha256:mcp-run-context") || !strings.Contains(result, "identity_mcp_run") {
		t.Fatalf("unexpected run audit output:\n%s", result)
	}
}

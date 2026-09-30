package contract

import (
	"encoding/json"
	"testing"
	"time"
)

// TestSearchResultJSONTags locks the camelCase JSON keys that the HTTP
// server emits. The CI gate in `make contract-check` greps for the
// matching snake_case names in client code; this test is the server-side
// mirror that ensures the contract struct field tags stay camelCase.
func TestSearchResultJSONTags(t *testing.T) {
	r := SearchResult{
		ID:        "id-1",
		Title:     "T",
		Path:      "p",
		Type:      "note",
		Project:   "prj",
		Status:    "active",
		Tags:      []string{"a"},
		Snippet:   "s",
		Score:     0.1,
		UpdatedAt: "2024-01-01",
	}
	b, err := json.Marshal(r)
	if err != nil {
		t.Fatalf("marshal: %v", err)
	}
	got := string(b)
	for _, want := range []string{
		`"id":"id-1"`,
		`"title":"T"`,
		`"path":"p"`,
		`"type":"note"`,
		`"project":"prj"`,
		`"status":"active"`,
		`"tags":["a"]`,
		`"snippet":"s"`,
		`"score":0.1`,
		`"updatedAt":"2024-01-01"`,
	} {
		if !contains(got, want) {
			t.Errorf("expected JSON to contain %s, got %s", want, got)
		}
	}
}

func TestIndexResultJSONTags(t *testing.T) {
	r := IndexResult{
		Scanned:     1,
		Added:       2,
		Updated:     3,
		Removed:     4,
		Skipped:     5,
		Errors:      []IndexError{{Path: "p", Error: "e"}},
		ChunksAdded: 6,
		EmbedErrors: 7,
		Duration:    time.Duration(12345678),
	}
	b, err := json.Marshal(r)
	if err != nil {
		t.Fatalf("marshal: %v", err)
	}
	got := string(b)
	for _, want := range []string{
		`"scanned":1`,
		`"added":2`,
		`"updated":3`,
		`"removed":4`,
		`"skipped":5`,
		`"errors":[{"path":"p","error":"e"}]`,
		`"chunksAdded":6`,
		`"embedErrors":7`,
		`"duration":12345678`,
	} {
		if !contains(got, want) {
			t.Errorf("expected JSON to contain %s, got %s", want, got)
		}
	}
}

func TestVaultStatusJSONTags(t *testing.T) {
	r := VaultStatus{Path: "p", IsVault: true, NoteCount: 1, Version: "v"}
	b, _ := json.Marshal(r)
	got := string(b)
	for _, want := range []string{`"path":"p"`, `"isVault":true`, `"noteCount":1`, `"version":"v"`} {
		if !contains(got, want) {
			t.Errorf("expected JSON to contain %s, got %s", want, got)
		}
	}
}

func TestGitStatusJSONTags(t *testing.T) {
	r := GitStatus{
		IsGitRepo:      true,
		Branch:         "main",
		Clean:          false,
		AheadBehind:    "ahead 1",
		ModifiedFiles:  []GitModifiedFile{{Path: "p", Status: "modified", Staged: false}},
		UntrackedFiles: []string{"u"},
	}
	b, _ := json.Marshal(r)
	got := string(b)
	for _, want := range []string{
		`"isGitRepo":true`,
		`"branch":"main"`,
		`"clean":false`,
		`"aheadBehind":"ahead 1"`,
		`"modifiedFiles":[{"path":"p","status":"modified","staged":false}]`,
		`"untrackedFiles":["u"]`,
	} {
		if !contains(got, want) {
			t.Errorf("expected JSON to contain %s, got %s", want, got)
		}
	}
}

func TestNoteDetailJSONTags(t *testing.T) {
	r := NoteDetail{ID: "i", Title: "T", Path: "p", Type: "note", Project: "prj", Status: "s", Tags: []string{"a"}, Content: "c"}
	b, _ := json.Marshal(r)
	got := string(b)
	for _, want := range []string{`"id":"i"`, `"title":"T"`, `"path":"p"`, `"type":"note"`, `"project":"prj"`, `"status":"s"`, `"tags":["a"]`, `"content":"c"`} {
		if !contains(got, want) {
			t.Errorf("expected JSON to contain %s, got %s", want, got)
		}
	}
}

func TestAnswerJSONTags(t *testing.T) {
	r := Answer{
		Answer:     "a",
		Sources:    []Source{{ID: "i", Path: "p", Title: "t"}},
		Confidence: "high",
	}
	b, _ := json.Marshal(r)
	got := string(b)
	for _, want := range []string{`"answer":"a"`, `"sources":[{"id":"i","path":"p","title":"t"}]`, `"confidence":"high"`} {
		if !contains(got, want) {
			t.Errorf("expected JSON to contain %s, got %s", want, got)
		}
	}
}

func contains(s, sub string) bool {
	return len(sub) == 0 || (len(s) >= len(sub) && indexOf(s, sub) >= 0)
}

func indexOf(s, sub string) int {
	for i := 0; i+len(sub) <= len(s); i++ {
		if s[i:i+len(sub)] == sub {
			return i
		}
	}
	return -1
}

func TestAgentStateReadContractJSONTags(t *testing.T) {
	p := Promotion{
		ID: "promo_1", AgentID: "agt_1", TargetKind: "memory", Status: "proposed",
		Candidate: "Remember", SourceObservationIDs: []string{"obs_1"}, CreatedAt: "2026-09-30T10:00:00Z",
	}
	b, err := json.Marshal(p)
	if err != nil {
		t.Fatalf("marshal promotion: %v", err)
	}
	got := string(b)
	for _, want := range []string{
		`"agentId":"agt_1"`,
		`"targetKind":"memory"`,
		`"sourceObservationIds":["obs_1"]`,
		`"createdAt":"2026-09-30T10:00:00Z"`,
	} {
		if !contains(got, want) {
			t.Errorf("expected promotion JSON to contain %s, got %s", want, got)
		}
	}

	d := EvaluationDatasetDetail{
		EvaluationDataset: EvaluationDataset{ID: "ds_1", Name: "Golden", CreatedAt: "now"},
		Cases:             []EvaluationCase{{ID: "case_1", DatasetID: "ds_1", Name: "Case", Input: map[string]interface{}{"x": true}, Tags: []string{}}},
	}
	b, err = json.Marshal(d)
	if err != nil {
		t.Fatalf("marshal dataset: %v", err)
	}
	got = string(b)
	if !contains(got, `"cases":[{"id":"case_1"`) {
		t.Errorf("expected dataset cases in JSON, got %s", got)
	}

	e := ExperimentDetail{
		Experiment: Experiment{ID: "exp_1", DatasetID: "ds_1", Name: "baseline", AgentID: "agt_1", AgentRevision: 1, Status: "completed", Config: map[string]interface{}{}, CreatedAt: "now"},
		Results:    []ExperimentResult{{ExperimentID: "exp_1", CaseID: "case_1", Label: "pass", Metadata: map[string]interface{}{}, CreatedAt: "now"}},
	}
	b, err = json.Marshal(e)
	if err != nil {
		t.Fatalf("marshal experiment: %v", err)
	}
	got = string(b)
	if !contains(got, `"agentRevision":1`) || !contains(got, `"results":[`) {
		t.Errorf("expected experiment detail fields in JSON, got %s", got)
	}
}

func TestContextSnapshotJSONTags(t *testing.T) {
	snapshot := ContextSnapshot{
		Hash: "sha256:abc", AgentID: "agt_1", AgentRevision: 2, AgentTitle: "Agent",
		KnowledgeScopes: []string{"project:test"}, ArtifactScopes: []string{},
		ConversationScopes: []string{}, CapabilityRefs: []string{"github"},
		Sections:   []ContextSection{{Kind: "identity", SourceID: "identity_1", SourcePath: "10-notes/id.md", Title: "Identity", Content: "Be precise."}},
		Unresolved: []ContextReferenceIssue{},
		Text:       "## identity: Identity\nBe precise.",
	}
	b, err := json.Marshal(snapshot)
	if err != nil {
		t.Fatal(err)
	}
	got := string(b)
	for _, want := range []string{
		`"agentId":"agt_1"`,
		`"agentRevision":2`,
		`"sourceId":"identity_1"`,
		`"knowledgeScopes":["project:test"]`,
	} {
		if !contains(got, want) {
			t.Errorf("expected context JSON to contain %s, got %s", want, got)
		}
	}
}

func TestRunAuditJSONTags(t *testing.T) {
	audit := RunAudit{
		Run: RunRecord{
			ID: "run_1", AgentName: "coder", AgentID: "agt_1", AgentRevision: 3,
			Task: "audit", Status: "succeeded", ContextHash: "sha256:ctx",
			Input: map[string]interface{}{"issue": 81}, Output: map[string]interface{}{"ok": true},
			CapabilitySnapshot: map[string]interface{}{}, RuntimeMetadata: map[string]interface{}{},
			FilesChanged: []string{"README.md"}, CreatedAt: "now",
		},
		Context: &ContextSnapshot{
			Hash: "sha256:ctx", AgentID: "agt_1", AgentRevision: 3, AgentTitle: "Coder",
			KnowledgeScopes: []string{}, ArtifactScopes: []string{}, ConversationScopes: []string{},
			CapabilityRefs: []string{}, Sections: []ContextSection{
				{Kind: "identity", SourceID: "identity_1", SourcePath: "10-notes/identity.md", Title: "Identity", Content: "Be precise."},
			},
			Unresolved: []ContextReferenceIssue{}, Text: "compiled",
		},
		Observations: []RunObservation{{
			ID: "obs_1", RunID: "run_1", Kind: "retrieval", Name: "search",
			Input: map[string]interface{}{}, Output: map[string]interface{}{}, Evidence: map[string]interface{}{"noteId": "memory_1"},
		}},
		Evaluations: []RunEvaluation{{
			ID: "eval_1", RunID: "run_1", Evaluator: "human:test", Name: "correctness",
			Label: "pass", Metadata: map[string]interface{}{},
		}},
	}
	b, err := json.Marshal(audit)
	if err != nil {
		t.Fatal(err)
	}
	got := string(b)
	for _, want := range []string{
		`"agentId":"agt_1"`,
		`"contextHash":"sha256:ctx"`,
		`"context":{"hash":"sha256:ctx"`,
		`"observations":[{"id":"obs_1"`,
		`"evaluations":[{"id":"eval_1"`,
		`"sourceId":"identity_1"`,
	} {
		if !contains(got, want) {
			t.Errorf("expected run audit JSON to contain %s, got %s", want, got)
		}
	}
}


func TestRunLearningCandidateRequestJSONTags(t *testing.T) {
	req := RunLearningCandidateRequest{
		TargetKind: "memory",
		Candidate: "Run focused tests before broad verification.",
		Rationale: "Regression evidence",
		SourceObservationIDs: []string{"obs_1"},
		SourceEvaluationIDs: []string{"eval_1"},
		SupersedesNoteID: "note_old",
	}
	b, err := json.Marshal(req)
	if err != nil {
		t.Fatal(err)
	}
	got := string(b)
	for _, want := range []string{
		`"targetKind":"memory"`,
		`"candidate":"Run focused tests before broad verification."`,
		`"sourceObservationIds":["obs_1"]`,
		`"sourceEvaluationIds":["eval_1"]`,
		`"supersedesNoteId":"note_old"`,
	} {
		if !contains(got, want) {
			t.Errorf("expected learning candidate JSON to contain %s, got %s", want, got)
		}
	}
	if contains(got, "agentId") {
		t.Fatalf("run learning request must not allow caller-supplied agentId: %s", got)
	}
}

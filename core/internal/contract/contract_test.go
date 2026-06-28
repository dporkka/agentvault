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

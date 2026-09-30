package api

import (
	"encoding/json"
	"net/http"
	"os"
	"path/filepath"
	"testing"

	"github.com/agentvault/core/internal/contract"
)

func TestViewRunEndpointExecutesPortableView(t *testing.T) {
	vaultPath, database := setupTestVault(t)
	defer database.Close()

	dir := filepath.Join(vaultPath, ".agentvault", "views")
	if err := os.MkdirAll(dir, 0755); err != nil {
		t.Fatal(err)
	}
	view := []byte("version: 1\nname: Test notes\nquery:\n  types: [note]\n  projects: [test-project]\nlimit: 10\n")
	if err := os.WriteFile(filepath.Join(dir, "test-notes.yaml"), view, 0644); err != nil {
		t.Fatal(err)
	}

	ts := newTestServer(t, vaultPath, database)
	defer ts.Close()

	req, _ := http.NewRequest(http.MethodPost, ts.URL+"/views/test-notes/run", nil)
	resp, err := http.DefaultClient.Do(req)
	if err != nil {
		t.Fatalf("run view: %v", err)
	}
	defer resp.Body.Close()
	if resp.StatusCode != http.StatusOK {
		t.Fatalf("expected 200, got %d", resp.StatusCode)
	}

	var results []contract.SearchResult
	if err := json.NewDecoder(resp.Body).Decode(&results); err != nil {
		t.Fatalf("decode: %v", err)
	}
	if len(results) != 1 || results[0].ID != "note_2024_01_15_123" {
		t.Fatalf("unexpected results: %#v", results)
	}
}

func TestViewEndpointRejectsTraversal(t *testing.T) {
	vaultPath, database := setupTestVault(t)
	defer database.Close()
	ts := newTestServer(t, vaultPath, database)
	defer ts.Close()

	resp, err := http.Get(ts.URL + "/views/..%2Fsecret")
	if err != nil {
		t.Fatalf("get view: %v", err)
	}
	defer resp.Body.Close()
	if resp.StatusCode != http.StatusBadRequest && resp.StatusCode != http.StatusNotFound {
		t.Fatalf("expected traversal rejection, got %d", resp.StatusCode)
	}
}

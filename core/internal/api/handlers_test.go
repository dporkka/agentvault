package api

import (
	"bytes"
	"encoding/json"
	"errors"
	"fmt"
	"net/http"
	"net/http/httptest"
	"os"
	"path/filepath"
	"regexp"
	"strings"
	"testing"
	"time"

	"github.com/agentvault/core/internal/ai"
	"github.com/agentvault/core/internal/indexer"
)

// TestVaultIndexEndpoint exercises POST /vault/index and the IndexOptions body path.
func TestVaultIndexEndpoint(t *testing.T) {
	vaultPath, database := setupTestVault(t)
	defer database.Close()

	srv := NewServer(vaultPath, database)
	srv.RegisterRoutes()

	var handler http.Handler = srv.mux
	handler = srv.authMiddleware(handler)
	handler = srv.corsMiddleware(handler)
	ts := httptest.NewServer(handler)
	defer ts.Close()

	req, err := http.NewRequest(http.MethodPost, ts.URL+"/vault/index", bytes.NewReader([]byte(`{"path":"10-notes"}`)))
	if err != nil {
		t.Fatalf("failed to create index request: %v", err)
	}
	req.Header.Set("Content-Type", "application/json")
	req.Header.Set("X-AgentVault-Token", srv.AuthToken())

	resp, err := http.DefaultClient.Do(req)
	if err != nil {
		t.Fatalf("failed to index: %v", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		t.Errorf("expected status 200, got %d", resp.StatusCode)
	}

	var result map[string]interface{}
	if err := json.NewDecoder(resp.Body).Decode(&result); err != nil {
		t.Fatalf("failed to decode index result: %v", err)
	}
	if scanned, _ := result["scanned"].(float64); scanned <= 0 {
		t.Errorf("expected scanned > 0, got %v", result["scanned"])
	}
}

// TestVaultIndexEndpoint_InvalidBody proves a malformed JSON body returns 400.
func TestVaultIndexEndpoint_InvalidBody(t *testing.T) {
	vaultPath, database := setupTestVault(t)
	defer database.Close()

	srv := NewServer(vaultPath, database)
	srv.RegisterRoutes()

	var handler http.Handler = srv.mux
	handler = srv.authMiddleware(handler)
	handler = srv.corsMiddleware(handler)
	ts := httptest.NewServer(handler)
	defer ts.Close()

	req, err := http.NewRequest(http.MethodPost, ts.URL+"/vault/index", bytes.NewReader([]byte("not json")))
	if err != nil {
		t.Fatalf("failed to create request: %v", err)
	}
	req.Header.Set("Content-Type", "application/json")
	req.Header.Set("X-AgentVault-Token", srv.AuthToken())

	resp, err := http.DefaultClient.Do(req)
	if err != nil {
		t.Fatalf("failed to send index request: %v", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusBadRequest {
		t.Errorf("expected 400 for invalid JSON, got %d", resp.StatusCode)
	}
}

// TestNoteByPathEndpoint_MissingID covers the /notes/{id} route with no id.
func TestNoteByPathEndpoint_MissingID(t *testing.T) {
	vaultPath, database := setupTestVault(t)
	defer database.Close()

	ts := newTestServer(t, vaultPath, database)
	defer ts.Close()

	resp, err := http.Get(ts.URL + "/notes/")
	if err != nil {
		t.Fatalf("failed to request notes/: %v", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusBadRequest {
		t.Errorf("expected 400 for missing id, got %d", resp.StatusCode)
	}
}

// TestNoteByPathEndpoint_NotFound covers the case where the id does not exist.
func TestNoteByPathEndpoint_NotFound(t *testing.T) {
	vaultPath, database := setupTestVault(t)
	defer database.Close()

	ts := newTestServer(t, vaultPath, database)
	defer ts.Close()

	resp, err := http.Get(ts.URL + "/notes/no-such-id")
	if err != nil {
		t.Fatalf("failed to request note: %v", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusNotFound {
		t.Errorf("expected 404 for unknown note, got %d", resp.StatusCode)
	}
}

// TestNoteByPathEndpoint_PathTraversal verifies that a note whose stored path
// escapes the vault root is rejected with 403.
func TestNoteByPathEndpoint_PathTraversal(t *testing.T) {
	vaultPath, database := setupTestVault(t)
	defer database.Close()

	now := time.Now().UTC().Format(time.RFC3339)
	if _, err := database.Exec(
		`INSERT INTO files (id, path, content_hash, created_at, updated_at, indexed_at) VALUES (?, ?, ?, ?, ?, ?)`,
		"bad_file", "../outside.md", "hash", now, now, now,
	); err != nil {
		t.Fatalf("failed to insert bad file row: %v", err)
	}
	if _, err := database.Exec(
		`INSERT INTO notes (id, file_id, title, type, status, project, created_at, updated_at, source_quality, body) VALUES (?, ?, ?, ?, ?, ?, ?, ?, ?, ?)`,
		"bad_note", "bad_file", "Bad", "note", "", "", now, now, "", "",
	); err != nil {
		t.Fatalf("failed to insert bad note row: %v", err)
	}

	ts := newTestServer(t, vaultPath, database)
	defer ts.Close()

	resp, err := http.Get(ts.URL + "/notes/bad_note")
	if err != nil {
		t.Fatalf("failed to request note: %v", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusForbidden {
		t.Errorf("expected 403 for path traversal, got %d", resp.StatusCode)
	}
}

// TestNoteByPathEndpoint_ReadError covers the file-read failure branch.
func TestNoteByPathEndpoint_ReadError(t *testing.T) {
	vaultPath, database := setupTestVault(t)
	defer database.Close()

	now := time.Now().UTC().Format(time.RFC3339)
	if _, err := database.Exec(
		`INSERT INTO files (id, path, content_hash, created_at, updated_at, indexed_at) VALUES (?, ?, ?, ?, ?, ?)`,
		"ghost_file", "ghost.md", "hash", now, now, now,
	); err != nil {
		t.Fatalf("failed to insert ghost file row: %v", err)
	}
	if _, err := database.Exec(
		`INSERT INTO notes (id, file_id, title, type, status, project, created_at, updated_at, source_quality, body) VALUES (?, ?, ?, ?, ?, ?, ?, ?, ?, ?)`,
		"ghost_note", "ghost_file", "Ghost", "note", "", "", now, now, "", "",
	); err != nil {
		t.Fatalf("failed to insert ghost note row: %v", err)
	}

	ts := newTestServer(t, vaultPath, database)
	defer ts.Close()

	resp, err := http.Get(ts.URL + "/notes/ghost_note")
	if err != nil {
		t.Fatalf("failed to request note: %v", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusInternalServerError {
		t.Errorf("expected 500 for missing file, got %d", resp.StatusCode)
	}
}

// TestCreateNoteEndpoint_MissingTitle verifies title validation.
func TestCreateNoteEndpoint_MissingTitle(t *testing.T) {
	vaultPath, database := setupTestVault(t)
	defer database.Close()

	srv := NewServer(vaultPath, database)
	srv.RegisterRoutes()

	var handler http.Handler = srv.mux
	handler = srv.authMiddleware(handler)
	handler = srv.corsMiddleware(handler)
	ts := httptest.NewServer(handler)
	defer ts.Close()

	body := map[string]interface{}{"type": "note"}
	bodyBytes, _ := json.Marshal(body)

	req, err := http.NewRequest(http.MethodPost, ts.URL+"/notes", bytes.NewReader(bodyBytes))
	if err != nil {
		t.Fatalf("failed to create request: %v", err)
	}
	req.Header.Set("Content-Type", "application/json")
	req.Header.Set("X-AgentVault-Token", srv.AuthToken())

	resp, err := http.DefaultClient.Do(req)
	if err != nil {
		t.Fatalf("failed to send create request: %v", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusBadRequest {
		t.Errorf("expected 400 for missing title, got %d", resp.StatusCode)
	}
}

// TestCreateNoteEndpoint_InvalidType covers the template-render failure branch.
func TestCreateNoteEndpoint_InvalidType(t *testing.T) {
	vaultPath, database := setupTestVault(t)
	defer database.Close()

	srv := NewServer(vaultPath, database)
	srv.RegisterRoutes()

	var handler http.Handler = srv.mux
	handler = srv.authMiddleware(handler)
	handler = srv.corsMiddleware(handler)
	ts := httptest.NewServer(handler)
	defer ts.Close()

	body := map[string]interface{}{"type": "unknown-type", "title": "X"}
	bodyBytes, _ := json.Marshal(body)

	req, err := http.NewRequest(http.MethodPost, ts.URL+"/notes", bytes.NewReader(bodyBytes))
	if err != nil {
		t.Fatalf("failed to create request: %v", err)
	}
	req.Header.Set("Content-Type", "application/json")
	req.Header.Set("X-AgentVault-Token", srv.AuthToken())

	resp, err := http.DefaultClient.Do(req)
	if err != nil {
		t.Fatalf("failed to send create request: %v", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusInternalServerError {
		t.Errorf("expected 500 for invalid type, got %d", resp.StatusCode)
	}
}

// TestCreateNoteEndpoint_DefaultType proves an empty type defaults to "note".
func TestCreateNoteEndpoint_DefaultType(t *testing.T) {
	vaultPath, database := setupTestVault(t)
	defer database.Close()

	srv := NewServer(vaultPath, database)
	srv.RegisterRoutes()

	var handler http.Handler = srv.mux
	handler = srv.authMiddleware(handler)
	handler = srv.corsMiddleware(handler)
	ts := httptest.NewServer(handler)
	defer ts.Close()

	body := map[string]interface{}{"title": "Only Title"}
	bodyBytes, _ := json.Marshal(body)

	req, err := http.NewRequest(http.MethodPost, ts.URL+"/notes", bytes.NewReader(bodyBytes))
	if err != nil {
		t.Fatalf("failed to create request: %v", err)
	}
	req.Header.Set("Content-Type", "application/json")
	req.Header.Set("X-AgentVault-Token", srv.AuthToken())

	resp, err := http.DefaultClient.Do(req)
	if err != nil {
		t.Fatalf("failed to send create request: %v", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		t.Errorf("expected 200, got %d", resp.StatusCode)
	}

	var result map[string]interface{}
	if err := json.NewDecoder(resp.Body).Decode(&result); err != nil {
		t.Fatalf("failed to decode: %v", err)
	}
	path, _ := result["path"].(string)
	if !strings.HasPrefix(path, "10-notes/") {
		t.Errorf("expected default note type to file under 10-notes/, got %v", path)
	}
}

// TestCaptureEndpoint_DefaultTitleAndSequence covers the capture title default
// and the next-available-number loop.
func TestCaptureEndpoint_DefaultTitleAndSequence(t *testing.T) {
	vaultPath, database := setupTestVault(t)
	defer database.Close()

	srv := NewServer(vaultPath, database)
	srv.RegisterRoutes()

	var handler http.Handler = srv.mux
	handler = srv.authMiddleware(handler)
	handler = srv.corsMiddleware(handler)
	ts := httptest.NewServer(handler)
	defer ts.Close()

	capture := func(title string) string {
		body := map[string]interface{}{"text": "body", "title": title}
		bodyBytes, _ := json.Marshal(body)
		req, err := http.NewRequest(http.MethodPost, ts.URL+"/capture", bytes.NewReader(bodyBytes))
		if err != nil {
			t.Fatalf("failed to create capture request: %v", err)
		}
		req.Header.Set("Content-Type", "application/json")
		req.Header.Set("X-AgentVault-Token", srv.AuthToken())
		resp, err := http.DefaultClient.Do(req)
		if err != nil {
			t.Fatalf("failed to capture: %v", err)
		}
		defer resp.Body.Close()
		if resp.StatusCode != http.StatusOK {
			t.Fatalf("expected 200, got %d", resp.StatusCode)
		}
		var result map[string]interface{}
		if err := json.NewDecoder(resp.Body).Decode(&result); err != nil {
			t.Fatalf("failed to decode capture result: %v", err)
		}
		return result["path"].(string)
	}

	first := capture("")
	if !regexp.MustCompile(`00-inbox/\d{4}-\d{2}-\d{2}_capture_001\.md$`).MatchString(first) {
		t.Errorf("expected first capture path to end with _001.md, got %s", first)
	}

	second := capture("")
	if !regexp.MustCompile(`00-inbox/\d{4}-\d{2}-\d{2}_capture_002\.md$`).MatchString(second) {
		t.Errorf("expected second capture path to end with _002.md, got %s", second)
	}

	content, err := os.ReadFile(filepath.Join(vaultPath, first))
	if err != nil {
		t.Fatalf("failed to read capture file: %v", err)
	}
	if !strings.Contains(string(content), `title: "Untitled Capture"`) {
		t.Errorf("expected default title in capture file, got:\n%s", string(content))
	}

	// Give the background auto-index goroutines time to finish before the
	// database is closed and the temporary vault is cleaned up.
	time.Sleep(200 * time.Millisecond)
}

// TestCaptureEndpoint_InvalidBody verifies malformed JSON is rejected.
func TestCaptureEndpoint_InvalidBody(t *testing.T) {
	vaultPath, database := setupTestVault(t)
	defer database.Close()

	srv := NewServer(vaultPath, database)
	srv.RegisterRoutes()

	var handler http.Handler = srv.mux
	handler = srv.authMiddleware(handler)
	handler = srv.corsMiddleware(handler)
	ts := httptest.NewServer(handler)
	defer ts.Close()

	req, err := http.NewRequest(http.MethodPost, ts.URL+"/capture", bytes.NewReader([]byte("not json")))
	if err != nil {
		t.Fatalf("failed to create request: %v", err)
	}
	req.Header.Set("Content-Type", "application/json")
	req.Header.Set("X-AgentVault-Token", srv.AuthToken())

	resp, err := http.DefaultClient.Do(req)
	if err != nil {
		t.Fatalf("failed to send capture request: %v", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusBadRequest {
		t.Errorf("expected 400 for invalid JSON, got %d", resp.StatusCode)
	}
}

// TestSearchEndpoint_FiltersAndOffset exercises type/project/tag/limit/offset params.
func TestSearchEndpoint_FiltersAndOffset(t *testing.T) {
	vaultPath, database := setupTestVault(t)
	defer database.Close()

	meetingDir := filepath.Join(vaultPath, "20-projects", "test-project")
	if err := os.MkdirAll(meetingDir, 0755); err != nil {
		t.Fatalf("failed to create meeting dir: %v", err)
	}
	meeting := `---
id: mtg_test_001
type: meeting
title: Test Meeting
project: test-project
tags: [planning]
status: active
created: 2024-01-15T10:00:00Z
updated: 2024-01-15T12:00:00Z
---
# Meeting
`
	if err := os.WriteFile(filepath.Join(meetingDir, "meeting.md"), []byte(meeting), 0644); err != nil {
		t.Fatalf("failed to write meeting note: %v", err)
	}

	idx := indexer.New(database, vaultPath)
	if _, err := idx.Index(indexer.IndexOptions{}); err != nil {
		t.Fatalf("failed to reindex: %v", err)
	}

	ts := newTestServer(t, vaultPath, database)
	defer ts.Close()

	assertCount := func(url string, want int) {
		t.Helper()
		resp, err := http.Get(url)
		if err != nil {
			t.Fatalf("failed to search: %v", err)
		}
		defer resp.Body.Close()
		if resp.StatusCode != http.StatusOK {
			t.Fatalf("expected 200, got %d", resp.StatusCode)
		}
		var results []map[string]interface{}
		if err := json.NewDecoder(resp.Body).Decode(&results); err != nil {
			t.Fatalf("failed to decode: %v", err)
		}
		if len(results) != want {
			t.Errorf("expected %d results for %s, got %d", want, url, len(results))
		}
	}

	assertCount(ts.URL+"/search?q=Meeting&type=meeting", 1)
	assertCount(ts.URL+"/search?q=Test&project=test-project", 2)
	assertCount(ts.URL+"/search?q=Test&tag=planning", 1)
	assertCount(ts.URL+"/search?q=Test&limit=1", 1)
	assertCount(ts.URL+"/search?q=Test&limit=1&offset=1", 1)
}

// TestSearchEndpoint_DatabaseError verifies the search error branch.
func TestSearchEndpoint_DatabaseError(t *testing.T) {
	vaultPath, database := setupTestVault(t)
	database.Close()

	ts := newTestServer(t, vaultPath, database)
	defer ts.Close()

	resp, err := http.Get(ts.URL + "/search?q=test")
	if err != nil {
		t.Fatalf("failed to search: %v", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusInternalServerError {
		t.Errorf("expected 500 for closed db, got %d", resp.StatusCode)
	}
}

// TestRecentEndpoint_Vector exercises the vector branch of /recent.
func TestRecentEndpoint_Vector(t *testing.T) {
	vaultPath, database := setupTestVault(t)
	defer database.Close()

	ts := newTestServer(t, vaultPath, database)
	defer ts.Close()

	resp, err := http.Get(ts.URL + "/recent?limit=5&vector=true")
	if err != nil {
		t.Fatalf("failed to get recent: %v", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		t.Errorf("expected 200, got %d", resp.StatusCode)
	}

	var results []map[string]interface{}
	if err := json.NewDecoder(resp.Body).Decode(&results); err != nil {
		t.Fatalf("failed to decode: %v", err)
	}
	if len(results) == 0 {
		t.Error("expected at least one recent note")
	}
}

// TestStaleEndpoint_Params exercises the days and limit query params.
func TestStaleEndpoint_Params(t *testing.T) {
	vaultPath, database := setupTestVault(t)
	defer database.Close()

	ts := newTestServer(t, vaultPath, database)
	defer ts.Close()

	resp, err := http.Get(ts.URL + "/stale?days=1&limit=1")
	if err != nil {
		t.Fatalf("failed to get stale: %v", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		t.Errorf("expected 200, got %d", resp.StatusCode)
	}

	var results []map[string]interface{}
	if err := json.NewDecoder(resp.Body).Decode(&results); err != nil {
		t.Fatalf("failed to decode: %v", err)
	}
	if len(results) == 0 {
		t.Error("expected at least one stale note")
	}
	if len(results) > 1 {
		t.Errorf("expected limit=1 to cap results, got %d", len(results))
	}
}

// TestProjectsEndpoint_DatabaseError covers the /projects query-failure branch.
func TestProjectsEndpoint_DatabaseError(t *testing.T) {
	vaultPath, database := setupTestVault(t)
	database.Close()

	ts := newTestServer(t, vaultPath, database)
	defer ts.Close()

	resp, err := http.Get(ts.URL + "/projects")
	if err != nil {
		t.Fatalf("failed to get projects: %v", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusInternalServerError {
		t.Errorf("expected 500 for closed db, got %d", resp.StatusCode)
	}
}

// TestRecentEndpoint_DatabaseError covers the /recent error branch.
func TestRecentEndpoint_DatabaseError(t *testing.T) {
	vaultPath, database := setupTestVault(t)
	database.Close()

	ts := newTestServer(t, vaultPath, database)
	defer ts.Close()

	resp, err := http.Get(ts.URL + "/recent")
	if err != nil {
		t.Fatalf("failed to get recent: %v", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusInternalServerError {
		t.Errorf("expected 500 for closed db, got %d", resp.StatusCode)
	}
}

// TestStaleEndpoint_DatabaseError covers the /stale error branch.
func TestStaleEndpoint_DatabaseError(t *testing.T) {
	vaultPath, database := setupTestVault(t)
	database.Close()

	ts := newTestServer(t, vaultPath, database)
	defer ts.Close()

	resp, err := http.Get(ts.URL + "/stale")
	if err != nil {
		t.Fatalf("failed to get stale: %v", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusInternalServerError {
		t.Errorf("expected 500 for closed db, got %d", resp.StatusCode)
	}
}

// TestVaultStatusEndpoint_NonVault verifies the response when the configured
// path is not a vault.
func TestVaultStatusEndpoint_NonVault(t *testing.T) {
	_, database := setupTestVault(t)
	defer database.Close()

	nonVault := t.TempDir()
	srv := NewServer(nonVault, database)
	srv.RegisterRoutes()

	var handler http.Handler = srv.mux
	handler = srv.authMiddleware(handler)
	handler = srv.corsMiddleware(handler)
	ts := httptest.NewServer(handler)
	defer ts.Close()

	resp, err := http.Get(ts.URL + "/vault/status")
	if err != nil {
		t.Fatalf("failed to get status: %v", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		t.Errorf("expected 200, got %d", resp.StatusCode)
	}

	var body map[string]interface{}
	if err := json.NewDecoder(resp.Body).Decode(&body); err != nil {
		t.Fatalf("failed to decode: %v", err)
	}
	if body["isVault"] != false {
		t.Errorf("expected isVault=false, got %v", body["isVault"])
	}
	if body["noteCount"] != float64(0) {
		t.Errorf("expected noteCount=0 for non-vault, got %v", body["noteCount"])
	}
}

// TestAuthVerifyEndpoint_BearerToken covers the Authorization header branch.
func TestAuthVerifyEndpoint_BearerToken(t *testing.T) {
	vaultPath, database := setupTestVault(t)
	defer database.Close()

	srv := NewServer(vaultPath, database)
	srv.RegisterRoutes()

	var handler http.Handler = srv.mux
	handler = srv.authMiddleware(handler)
	handler = srv.corsMiddleware(handler)
	ts := httptest.NewServer(handler)
	defer ts.Close()

	req, err := http.NewRequest(http.MethodGet, ts.URL+"/auth/verify", nil)
	if err != nil {
		t.Fatalf("failed to create request: %v", err)
	}
	req.Header.Set("Authorization", "Bearer "+srv.AuthToken())

	resp, err := http.DefaultClient.Do(req)
	if err != nil {
		t.Fatalf("failed to verify: %v", err)
	}
	defer resp.Body.Close()

	var body map[string]interface{}
	if err := json.NewDecoder(resp.Body).Decode(&body); err != nil {
		t.Fatalf("failed to decode: %v", err)
	}
	if body["hasToken"] != true {
		t.Errorf("expected hasToken=true, got %v", body["hasToken"])
	}
	if body["tokenValid"] != true {
		t.Errorf("expected tokenValid=true, got %v", body["tokenValid"])
	}

	req2, err := http.NewRequest(http.MethodGet, ts.URL+"/auth/verify", nil)
	if err != nil {
		t.Fatalf("failed to create request: %v", err)
	}
	req2.Header.Set("Authorization", "Bearer wrong-token")

	resp2, err := http.DefaultClient.Do(req2)
	if err != nil {
		t.Fatalf("failed to verify: %v", err)
	}
	defer resp2.Body.Close()

	if err := json.NewDecoder(resp2.Body).Decode(&body); err != nil {
		t.Fatalf("failed to decode: %v", err)
	}
	if body["tokenValid"] != false {
		t.Errorf("expected tokenValid=false for wrong bearer, got %v", body["tokenValid"])
	}
}

// TestAskEndpoint_InvalidBody verifies malformed JSON is rejected.
func TestAskEndpoint_InvalidBody(t *testing.T) {
	vaultPath, database := setupTestVault(t)
	defer database.Close()

	srv := NewServer(vaultPath, database)
	srv.RegisterRoutes()

	var handler http.Handler = srv.mux
	handler = srv.authMiddleware(handler)
	handler = srv.corsMiddleware(handler)
	ts := httptest.NewServer(handler)
	defer ts.Close()

	req, err := http.NewRequest(http.MethodPost, ts.URL+"/ask", bytes.NewReader([]byte("not json")))
	if err != nil {
		t.Fatalf("failed to create request: %v", err)
	}
	req.Header.Set("Content-Type", "application/json")
	req.Header.Set("X-AgentVault-Token", srv.AuthToken())

	resp, err := http.DefaultClient.Do(req)
	if err != nil {
		t.Fatalf("failed to ask: %v", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusBadRequest {
		t.Errorf("expected 400 for invalid JSON, got %d", resp.StatusCode)
	}
}

// TestAskEndpoint_ProviderLoadError covers the getAIProvider error branch.
func TestAskEndpoint_ProviderLoadError(t *testing.T) {
	vaultPath, database := setupTestVault(t)
	defer database.Close()

	// Replace the mock config with an unsupported provider so provider loading fails.
	cfg := fmt.Sprintf(`{"vaultPath": %q, "ai": {"provider": "unknown-provider"}}`, vaultPath)
	if err := os.WriteFile(filepath.Join(vaultPath, ".agentvault", "config.json"), []byte(cfg), 0644); err != nil {
		t.Fatalf("failed to overwrite config: %v", err)
	}

	srv := NewServer(vaultPath, database)
	srv.RegisterRoutes()

	var handler http.Handler = srv.mux
	handler = srv.authMiddleware(handler)
	handler = srv.corsMiddleware(handler)
	ts := httptest.NewServer(handler)
	defer ts.Close()

	body := map[string]string{"question": "What is Go?"}
	bodyBytes, _ := json.Marshal(body)

	req, err := http.NewRequest(http.MethodPost, ts.URL+"/ask", bytes.NewReader(bodyBytes))
	if err != nil {
		t.Fatalf("failed to create request: %v", err)
	}
	req.Header.Set("Content-Type", "application/json")
	req.Header.Set("X-AgentVault-Token", srv.AuthToken())

	resp, err := http.DefaultClient.Do(req)
	if err != nil {
		t.Fatalf("failed to ask: %v", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusInternalServerError {
		t.Errorf("expected 500 when provider load fails, got %d", resp.StatusCode)
	}
}

// TestAskEndpoint_AIProviderError covers the RAG pipeline failure branch.
func TestAskEndpoint_AIProviderError(t *testing.T) {
	vaultPath, database := setupTestVault(t)
	defer database.Close()

	srv := NewServer(vaultPath, database)
	srv.RegisterRoutes()
	srv.aiProvider = &ai.MockProvider{Err: errors.New("ai failed")}

	var handler http.Handler = srv.mux
	handler = srv.authMiddleware(handler)
	handler = srv.corsMiddleware(handler)
	ts := httptest.NewServer(handler)
	defer ts.Close()

	body := map[string]string{"question": "What is Go?"}
	bodyBytes, _ := json.Marshal(body)

	req, err := http.NewRequest(http.MethodPost, ts.URL+"/ask", bytes.NewReader(bodyBytes))
	if err != nil {
		t.Fatalf("failed to create request: %v", err)
	}
	req.Header.Set("Content-Type", "application/json")
	req.Header.Set("X-AgentVault-Token", srv.AuthToken())

	resp, err := http.DefaultClient.Do(req)
	if err != nil {
		t.Fatalf("failed to ask: %v", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusBadGateway {
		t.Errorf("expected 502 when AI fails, got %d", resp.StatusCode)
	}
}

// TestAskEndpoint_NoResults covers the no-sources answer branch.
func TestAskEndpoint_NoResults(t *testing.T) {
	vaultPath, database := setupTestVault(t)
	defer database.Close()

	srv := NewServer(vaultPath, database)
	srv.RegisterRoutes()

	var handler http.Handler = srv.mux
	handler = srv.authMiddleware(handler)
	handler = srv.corsMiddleware(handler)
	ts := httptest.NewServer(handler)
	defer ts.Close()

	body := map[string]string{"question": "xyzabc nonmatching"}
	bodyBytes, _ := json.Marshal(body)

	req, err := http.NewRequest(http.MethodPost, ts.URL+"/ask", bytes.NewReader(bodyBytes))
	if err != nil {
		t.Fatalf("failed to create request: %v", err)
	}
	req.Header.Set("Content-Type", "application/json")
	req.Header.Set("X-AgentVault-Token", srv.AuthToken())

	resp, err := http.DefaultClient.Do(req)
	if err != nil {
		t.Fatalf("failed to ask: %v", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		t.Errorf("expected 200 even with no results, got %d", resp.StatusCode)
	}

	var answer map[string]interface{}
	if err := json.NewDecoder(resp.Body).Decode(&answer); err != nil {
		t.Fatalf("failed to decode: %v", err)
	}
	if answer["missingInfo"] == "" {
		t.Errorf("expected missingInfo for no-results answer, got %v", answer["missingInfo"])
	}
}

package api

import (
	"bytes"
	"encoding/json"
	"fmt"
	"net/http"
	"net/http/httptest"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"testing"
	"time"

	"github.com/agentvault/core/internal/db"
	"github.com/agentvault/core/internal/indexer"
)

// setupTestVault creates a temporary vault directory with a database.
func setupTestVault(t *testing.T) (string, *db.DB) {
	t.Helper()
	tmpDir := t.TempDir()

	// Create .agentvault directory
	if err := os.MkdirAll(filepath.Join(tmpDir, ".agentvault"), 0755); err != nil {
		t.Fatalf("failed to create .agentvault dir: %v", err)
	}

	// Create a config file
	config := fmt.Sprintf(`{"vaultPath": %q, "createdAt": %q, "ai": {"provider": "mock"}}`, tmpDir, time.Now().UTC().Format(time.RFC3339))
	if err := os.WriteFile(filepath.Join(tmpDir, ".agentvault", "config.json"), []byte(config), 0644); err != nil {
		t.Fatalf("failed to write config: %v", err)
	}

	// Create standard folders
	folders := []string{"00-inbox", "10-notes", "20-projects"}
	for _, f := range folders {
		if err := os.MkdirAll(filepath.Join(tmpDir, f), 0755); err != nil {
			t.Fatalf("failed to create folder %s: %v", f, err)
		}
	}

	// Open database
	database, err := db.Open(tmpDir)
	if err != nil {
		t.Fatalf("failed to open database: %v", err)
	}

	// Run migrations
	if err := database.RunMigrations(); err != nil {
		t.Fatalf("failed to run migrations: %v", err)
	}

	// Seed with a test note
	noteContent := `---
id: note_2024_01_15_123
type: note
title: Test Note
project: test-project
tags: [go, api]
created: 2024-01-15T10:00:00Z
updated: 2024-01-15T12:00:00Z
---

# Test Note

This is a test note for the API server.
`
	if err := os.WriteFile(filepath.Join(tmpDir, "10-notes", "test-note.md"), []byte(noteContent), 0644); err != nil {
		t.Fatalf("failed to write test note: %v", err)
	}

	// Index the test note
	idx := indexer.New(database, tmpDir)
	if _, err := idx.Index(indexer.IndexOptions{}); err != nil {
		t.Fatalf("failed to index: %v", err)
	}

	return tmpDir, database
}

// newTestServer creates a test server with the given vault.
func newTestServer(t *testing.T, vaultPath string, database *db.DB) *httptest.Server {
	t.Helper()
	srv := NewServer(vaultPath, database)
	srv.RegisterRoutes()

	// Build handler chain (same as production)
	var handler http.Handler = srv.mux
	handler = srv.authMiddleware(handler)
	handler = srv.corsMiddleware(handler)

	return httptest.NewServer(handler)
}

func TestHealthEndpoint(t *testing.T) {
	vaultPath, database := setupTestVault(t)
	defer database.Close()

	ts := newTestServer(t, vaultPath, database)
	defer ts.Close()

	resp, err := http.Get(ts.URL + "/health")
	if err != nil {
		t.Fatalf("failed to get health: %v", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		t.Errorf("expected status 200, got %d", resp.StatusCode)
	}

	var body map[string]interface{}
	if err := json.NewDecoder(resp.Body).Decode(&body); err != nil {
		t.Fatalf("failed to decode response: %v", err)
	}

	if body["status"] != "ok" {
		t.Errorf("expected status=ok, got %v", body["status"])
	}
	if body["version"] != "0.1.0" {
		t.Errorf("expected version=0.1.0, got %v", body["version"])
	}
	if body["vault"] != vaultPath {
		t.Errorf("expected vault=%s, got %v", vaultPath, body["vault"])
	}

	// Check CORS headers
	if resp.Header.Get("Access-Control-Allow-Origin") == "" {
		t.Error("expected CORS Access-Control-Allow-Origin header")
	}
}

func TestSearchEndpoint(t *testing.T) {
	vaultPath, database := setupTestVault(t)
	defer database.Close()

	ts := newTestServer(t, vaultPath, database)
	defer ts.Close()

	resp, err := http.Get(ts.URL + "/search?q=test")
	if err != nil {
		t.Fatalf("failed to search: %v", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		t.Errorf("expected status 200, got %d", resp.StatusCode)
	}

	var results []map[string]interface{}
	if err := json.NewDecoder(resp.Body).Decode(&results); err != nil {
		t.Fatalf("failed to decode response: %v", err)
	}

	if len(results) == 0 {
		t.Error("expected at least one search result")
	}

	// Check first result has expected fields (camelCase JSON keys)
	found := false
	for _, r := range results {
		title, ok := r["title"].(string)
		if ok && strings.Contains(title, "Test") {
			found = true
			if r["id"] != "note_2024_01_15_123" {
				t.Errorf("expected id=note_2024_01_15_123, got %v", r["id"])
			}
			break
		}
	}
	if !found {
		t.Errorf("expected to find 'Test Note' in results, got: %v", results)
	}
}

func TestSearchEndpoint_VectorParams(t *testing.T) {
	vaultPath, database := setupTestVault(t)
	defer database.Close()

	ts := newTestServer(t, vaultPath, database)
	defer ts.Close()

	resp, err := http.Get(ts.URL + "/search?q=test&vector=true&hybrid_weight=0.5&topk=10")
	if err != nil {
		t.Fatalf("failed to search with vector params: %v", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		t.Errorf("expected status 200, got %d", resp.StatusCode)
	}

	var results []map[string]interface{}
	if err := json.NewDecoder(resp.Body).Decode(&results); err != nil {
		t.Fatalf("failed to decode response: %v", err)
	}

	if len(results) == 0 {
		t.Error("expected at least one search result")
	}

	// The test vault has no embeddings, so the server should gracefully fall
	// back to FTS and still return the expected note shape.
	found := false
	for _, r := range results {
		if r["id"] == "note_2024_01_15_123" {
			found = true
			break
		}
	}
	if !found {
		t.Errorf("expected seeded note in vector-param results, got: %v", results)
	}
}

func TestSearchEndpoint_WeightIgnoredWithoutVector(t *testing.T) {
	vaultPath, database := setupTestVault(t)
	defer database.Close()

	ts := newTestServer(t, vaultPath, database)
	defer ts.Close()

	resp, err := http.Get(ts.URL + "/search?q=test&hybrid_weight=1.0")
	if err != nil {
		t.Fatalf("failed to search: %v", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		t.Errorf("expected status 200, got %d", resp.StatusCode)
	}

	var results []map[string]interface{}
	if err := json.NewDecoder(resp.Body).Decode(&results); err != nil {
		t.Fatalf("failed to decode response: %v", err)
	}

	if len(results) == 0 {
		t.Error("expected FTS results even when vector is not enabled")
	}
}

func TestNoteByIDEndpoint(t *testing.T) {
	vaultPath, database := setupTestVault(t)
	defer database.Close()

	ts := newTestServer(t, vaultPath, database)
	defer ts.Close()

	resp, err := http.Get(ts.URL + "/notes/note_2024_01_15_123")
	if err != nil {
		t.Fatalf("failed to get note: %v", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		t.Errorf("expected status 200, got %d", resp.StatusCode)
	}

	var body map[string]interface{}
	if err := json.NewDecoder(resp.Body).Decode(&body); err != nil {
		t.Fatalf("failed to decode response: %v", err)
	}

	if body["id"] != "note_2024_01_15_123" {
		t.Errorf("expected id=note_2024_01_15_123, got %v", body["id"])
	}
	if body["title"] != "Test Note" {
		t.Errorf("expected title='Test Note', got %v", body["title"])
	}
	if body["content"] == "" {
		t.Error("expected non-empty content field")
	}
}

func TestCreateNoteEndpoint(t *testing.T) {
	vaultPath, database := setupTestVault(t)
	defer database.Close()

	srv := NewServer(vaultPath, database)
	srv.RegisterRoutes()

	var handler http.Handler = srv.mux
	handler = srv.authMiddleware(handler)
	handler = srv.corsMiddleware(handler)
	ts := httptest.NewServer(handler)
	defer ts.Close()

	// Attempt without auth token should fail
	reqBody := map[string]interface{}{
		"type":    "note",
		"title":   "My New Note",
		"project": "test-project",
		"tags":    []string{"api", "test"},
	}
	bodyBytes, _ := json.Marshal(reqBody)

	resp, err := http.Post(ts.URL+"/notes", "application/json", bytes.NewReader(bodyBytes))
	if err != nil {
		t.Fatalf("failed to create note without auth: %v", err)
	}
	resp.Body.Close()
	if resp.StatusCode != http.StatusUnauthorized {
		t.Errorf("expected 401 without auth, got %d", resp.StatusCode)
	}

	// Now with auth token
	req, err := http.NewRequest(http.MethodPost, ts.URL+"/notes", bytes.NewReader(bodyBytes))
	if err != nil {
		t.Fatalf("failed to create request: %v", err)
	}
	req.Header.Set("Content-Type", "application/json")
	req.Header.Set("X-AgentVault-Token", srv.AuthToken())

	resp, err = http.DefaultClient.Do(req)
	if err != nil {
		t.Fatalf("failed to create note with auth: %v", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		t.Errorf("expected status 200, got %d", resp.StatusCode)
	}

	var result map[string]interface{}
	if err := json.NewDecoder(resp.Body).Decode(&result); err != nil {
		t.Fatalf("failed to decode response: %v", err)
	}

	if result["path"] == "" {
		t.Error("expected non-empty path in response")
	}
	if result["id"] == "" {
		t.Error("expected non-empty id in response")
	}

	// Verify file was actually created
	fullPath := filepath.Join(vaultPath, result["path"].(string))
	if _, err := os.Stat(fullPath); os.IsNotExist(err) {
		t.Errorf("expected file to exist at %s", fullPath)
	}
}

func TestCaptureEndpoint(t *testing.T) {
	vaultPath, database := setupTestVault(t)
	defer database.Close()

	srv := NewServer(vaultPath, database)
	srv.RegisterRoutes()

	var handler http.Handler = srv.mux
	handler = srv.authMiddleware(handler)
	handler = srv.corsMiddleware(handler)
	ts := httptest.NewServer(handler)
	defer ts.Close()

	// Without auth should fail
	reqBody := map[string]interface{}{
		"type":  "webpage",
		"title": "Test Capture",
		"url":   "https://example.com",
		"text":  "Some captured text",
	}
	bodyBytes, _ := json.Marshal(reqBody)

	resp, err := http.Post(ts.URL+"/capture", "application/json", bytes.NewReader(bodyBytes))
	if err != nil {
		t.Fatalf("failed to capture without auth: %v", err)
	}
	resp.Body.Close()
	if resp.StatusCode != http.StatusUnauthorized {
		t.Errorf("expected 401 without auth, got %d", resp.StatusCode)
	}

	// With auth should succeed
	req, err := http.NewRequest(http.MethodPost, ts.URL+"/capture", bytes.NewReader(bodyBytes))
	if err != nil {
		t.Fatalf("failed to create request: %v", err)
	}
	req.Header.Set("Content-Type", "application/json")
	req.Header.Set("X-AgentVault-Token", srv.AuthToken())

	resp, err = http.DefaultClient.Do(req)
	if err != nil {
		t.Fatalf("failed to capture with auth: %v", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		t.Errorf("expected status 200, got %d", resp.StatusCode)
	}

	var result map[string]interface{}
	if err := json.NewDecoder(resp.Body).Decode(&result); err != nil {
		t.Fatalf("failed to decode response: %v", err)
	}

	if result["path"] == "" {
		t.Error("expected non-empty path in response")
	}
	if !strings.HasPrefix(result["path"].(string), "00-inbox/") {
		t.Errorf("expected path to start with 00-inbox/, got %v", result["path"])
	}
}

func TestCORSHeaders(t *testing.T) {
	vaultPath, database := setupTestVault(t)
	defer database.Close()

	ts := newTestServer(t, vaultPath, database)
	defer ts.Close()

	// Test preflight request
	req, err := http.NewRequest(http.MethodOptions, ts.URL+"/search", nil)
	if err != nil {
		t.Fatalf("failed to create options request: %v", err)
	}
	req.Header.Set("Origin", "http://localhost:3000")
	req.Header.Set("Access-Control-Request-Method", "POST")

	resp, err := http.DefaultClient.Do(req)
	if err != nil {
		t.Fatalf("failed to send options request: %v", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		t.Errorf("expected 200 for OPTIONS, got %d", resp.StatusCode)
	}
	if resp.Header.Get("Access-Control-Allow-Origin") != "http://localhost:3000" {
		t.Errorf("expected CORS origin header, got %v", resp.Header.Get("Access-Control-Allow-Origin"))
	}
	if resp.Header.Get("Access-Control-Allow-Methods") == "" {
		t.Error("expected CORS methods header")
	}
}

func TestVaultStatusEndpoint(t *testing.T) {
	vaultPath, database := setupTestVault(t)
	defer database.Close()

	ts := newTestServer(t, vaultPath, database)
	defer ts.Close()

	resp, err := http.Get(ts.URL + "/vault/status")
	if err != nil {
		t.Fatalf("failed to get vault status: %v", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		t.Errorf("expected status 200, got %d", resp.StatusCode)
	}

	var body map[string]interface{}
	if err := json.NewDecoder(resp.Body).Decode(&body); err != nil {
		t.Fatalf("failed to decode: %v", err)
	}

	if body["isVault"] != true {
		t.Errorf("expected isVault=true, got %v", body["isVault"])
	}
	if body["path"] != vaultPath {
		t.Errorf("expected path=%s, got %v", vaultPath, body["path"])
	}
}

func TestProjectsEndpoint(t *testing.T) {
	vaultPath, database := setupTestVault(t)
	defer database.Close()

	ts := newTestServer(t, vaultPath, database)
	defer ts.Close()

	resp, err := http.Get(ts.URL + "/projects")
	if err != nil {
		t.Fatalf("failed to get projects: %v", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		t.Errorf("expected status 200, got %d", resp.StatusCode)
	}

	// Clients (web, extension, mobile) consume /projects as a bare string[].
	var projects []string
	if err := json.NewDecoder(resp.Body).Decode(&projects); err != nil {
		t.Fatalf("failed to decode projects array: %v", err)
	}

	found := false
	for _, p := range projects {
		if p == "test-project" {
			found = true
			break
		}
	}
	if !found {
		t.Errorf("expected 'test-project' in projects list, got %v", projects)
	}
}

func TestRecentEndpoint(t *testing.T) {
	vaultPath, database := setupTestVault(t)
	defer database.Close()

	ts := newTestServer(t, vaultPath, database)
	defer ts.Close()

	resp, err := http.Get(ts.URL + "/recent?limit=5")
	if err != nil {
		t.Fatalf("failed to get recent: %v", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		t.Errorf("expected status 200, got %d", resp.StatusCode)
	}

	var results []map[string]interface{}
	if err := json.NewDecoder(resp.Body).Decode(&results); err != nil {
		t.Fatalf("failed to decode: %v", err)
	}

	if len(results) == 0 {
		t.Error("expected at least one recent note")
	}
}

// TestStaleEndpoint proves the /stale response shape: a JSON array of results
// carrying the same camelCase fields as /search and /recent. The
// seeded note's `updated` date is in 2024, so it is stale under the 30-day
// default and must appear.
func TestStaleEndpoint(t *testing.T) {
	vaultPath, database := setupTestVault(t)
	defer database.Close()

	ts := newTestServer(t, vaultPath, database)
	defer ts.Close()

	resp, err := http.Get(ts.URL + "/stale")
	if err != nil {
		t.Fatalf("failed to get stale: %v", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		t.Errorf("expected status 200, got %d", resp.StatusCode)
	}

	var results []map[string]interface{}
	if err := json.NewDecoder(resp.Body).Decode(&results); err != nil {
		t.Fatalf("failed to decode stale array: %v", err)
	}

	if len(results) == 0 {
		t.Fatal("expected at least one stale note")
	}

	// Same shape as /search and /recent (camelCase JSON keys).
	found := false
	for _, r := range results {
		if r["id"] == "note_2024_01_15_123" {
			found = true
			if title, _ := r["title"].(string); title != "Test Note" {
				t.Errorf("expected title='Test Note', got %v", r["title"])
			}
			break
		}
	}
	if !found {
		t.Errorf("expected seeded note in stale results, got: %v", results)
	}
}

// TestGitStatusEndpoint covers the non-versioned vault case: the test vault is
// not a git repo, so the endpoint must truthfully report isGitRepo=false rather
// than the old hard-coded "clean main" payload.
func TestGitStatusEndpoint(t *testing.T) {
	vaultPath, database := setupTestVault(t)
	defer database.Close()

	ts := newTestServer(t, vaultPath, database)
	defer ts.Close()

	resp, err := http.Get(ts.URL + "/git/status")
	if err != nil {
		t.Fatalf("failed to get git status: %v", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		t.Errorf("expected status 200, got %d", resp.StatusCode)
	}

	var body map[string]interface{}
	if err := json.NewDecoder(resp.Body).Decode(&body); err != nil {
		t.Fatalf("failed to decode: %v", err)
	}

	if body["isGitRepo"] != false {
		t.Errorf("expected isGitRepo=false for a non-repo vault, got %v", body["isGitRepo"])
	}
	if _, ok := body["modifiedFiles"].([]interface{}); !ok {
		t.Errorf("expected modifiedFiles array, got %T", body["modifiedFiles"])
	}
	if _, ok := body["untrackedFiles"].([]interface{}); !ok {
		t.Errorf("expected untrackedFiles array, got %T", body["untrackedFiles"])
	}
}

// TestGitStatusEndpoint_WithRepo verifies the endpoint reflects real git state
// from internal/git.Status — a dirty repo with one modified file.
func TestGitStatusEndpoint_WithRepo(t *testing.T) {
	if _, err := exec.LookPath("git"); err != nil {
		t.Skip("git not installed")
	}

	vaultPath, database := setupTestVault(t)
	defer database.Close()

	// Initialize a git repo in the vault and create an initial commit.
	run := func(args ...string) {
		cmd := exec.Command("git", append([]string{"-C", vaultPath}, args...)...)
		if out, err := cmd.CombinedOutput(); err != nil {
			t.Fatalf("git %v failed: %v\n%s", args, err, out)
		}
	}
	run("init")
	run("config", "user.email", "test@test.com")
	run("config", "user.name", "Test User")
	tracked := filepath.Join(vaultPath, "tracked.md")
	if err := os.WriteFile(tracked, []byte("original\n"), 0644); err != nil {
		t.Fatalf("failed to write file: %v", err)
	}
	run("add", "-A")
	run("commit", "-m", "initial")

	// Modify the tracked file so the working tree is dirty.
	if err := os.WriteFile(tracked, []byte("changed\n"), 0644); err != nil {
		t.Fatalf("failed to modify file: %v", err)
	}

	ts := newTestServer(t, vaultPath, database)
	defer ts.Close()

	resp, err := http.Get(ts.URL + "/git/status")
	if err != nil {
		t.Fatalf("failed to get git status: %v", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		t.Errorf("expected status 200, got %d", resp.StatusCode)
	}

	var body map[string]interface{}
	if err := json.NewDecoder(resp.Body).Decode(&body); err != nil {
		t.Fatalf("failed to decode: %v", err)
	}

	if body["isGitRepo"] != true {
		t.Errorf("expected isGitRepo=true, got %v", body["isGitRepo"])
	}
	if body["clean"] != false {
		t.Errorf("expected clean=false for a dirty repo, got %v", body["clean"])
	}
	branch, _ := body["branch"].(string)
	if branch == "" {
		t.Errorf("expected a non-empty branch name, got %v", body["branch"])
	}
	modified, ok := body["modifiedFiles"].([]interface{})
	if !ok || len(modified) != 1 {
		t.Fatalf("expected 1 modified file, got %v", body["modifiedFiles"])
	}
	first, _ := modified[0].(map[string]interface{})
	if first["path"] != "tracked.md" {
		t.Errorf("expected modified path tracked.md, got %v", first["path"])
	}
	if first["status"] != "modified" {
		t.Errorf("expected status=modified, got %v", first["status"])
	}
}

func TestAskEndpoint(t *testing.T) {
	vaultPath, database := setupTestVault(t)
	defer database.Close()

	srv := NewServer(vaultPath, database)
	srv.RegisterRoutes()

	var handler http.Handler = srv.mux
	handler = srv.authMiddleware(handler)
	handler = srv.corsMiddleware(handler)
	ts := httptest.NewServer(handler)
	defer ts.Close()

	reqBody := map[string]string{"question": "What is Go?"}
	bodyBytes, _ := json.Marshal(reqBody)

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
		t.Errorf("expected status 200, got %d", resp.StatusCode)
	}

	var body map[string]interface{}
	if err := json.NewDecoder(resp.Body).Decode(&body); err != nil {
		t.Fatalf("failed to decode: %v", err)
	}

	answer, ok := body["answer"].(string)
	if !ok || answer == "" {
		t.Errorf("expected non-empty answer, got %v", body["answer"])
	}

	if strings.Contains(answer, "not yet implemented") {
		t.Errorf("expected real RAG response, got stub answer: %q", answer)
	}

	sources, ok := body["sources"].([]interface{})
	if !ok {
		t.Fatalf("expected sources array, got %T", body["sources"])
	}
	// When sources are present, each must carry the id and path the web,
	// extension, and mobile clients navigate by (the /note/{id} route).
	for i, s := range sources {
		src, ok := s.(map[string]interface{})
		if !ok {
			t.Fatalf("source %d is not an object: %T", i, s)
		}
		if id, _ := src["id"].(string); id == "" {
			t.Errorf("source %d missing id: %v", i, src)
		}
		if path, _ := src["path"].(string); path == "" {
			t.Errorf("source %d missing path: %v", i, src)
		}
	}
}

func TestAskEndpoint_MissingQuestion(t *testing.T) {
	vaultPath, database := setupTestVault(t)
	defer database.Close()

	srv := NewServer(vaultPath, database)
	srv.RegisterRoutes()

	var handler http.Handler = srv.mux
	handler = srv.authMiddleware(handler)
	handler = srv.corsMiddleware(handler)
	ts := httptest.NewServer(handler)
	defer ts.Close()

	req, err := http.NewRequest(http.MethodPost, ts.URL+"/ask", bytes.NewReader([]byte(`{"question":"  "}`)))
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
		t.Errorf("expected status 400, got %d", resp.StatusCode)
	}
}

func TestAuthVerifyEndpoint(t *testing.T) {
	vaultPath, database := setupTestVault(t)
	defer database.Close()

	srv := NewServer(vaultPath, database)
	srv.RegisterRoutes()

	var handler http.Handler = srv.mux
	handler = srv.authMiddleware(handler)
	handler = srv.corsMiddleware(handler)
	ts := httptest.NewServer(handler)
	defer ts.Close()

	// Without token - should report hasToken=false
	resp, err := http.Get(ts.URL + "/auth/verify")
	if err != nil {
		t.Fatalf("failed to verify auth: %v", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		t.Errorf("expected status 200, got %d", resp.StatusCode)
	}

	var body map[string]interface{}
	if err := json.NewDecoder(resp.Body).Decode(&body); err != nil {
		t.Fatalf("failed to decode: %v", err)
	}

	if body["status"] != "ok" {
		t.Errorf("expected status=ok, got %v", body["status"])
	}
	if body["hasToken"] != false {
		t.Errorf("expected hasToken=false, got %v", body["hasToken"])
	}
	if body["tokenValid"] != false {
		t.Errorf("expected tokenValid=false without token, got %v", body["tokenValid"])
	}

	// With correct token - should report tokenValid=true
	req, err := http.NewRequest(http.MethodGet, ts.URL+"/auth/verify", nil)
	if err != nil {
		t.Fatalf("failed to create request: %v", err)
	}
	req.Header.Set("X-AgentVault-Token", srv.AuthToken())

	resp, err = http.DefaultClient.Do(req)
	if err != nil {
		t.Fatalf("failed to verify auth with token: %v", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		t.Errorf("expected status 200 with token, got %d", resp.StatusCode)
	}

	if err := json.NewDecoder(resp.Body).Decode(&body); err != nil {
		t.Fatalf("failed to decode: %v", err)
	}

	if body["hasToken"] != true {
		t.Errorf("expected hasToken=true, got %v", body["hasToken"])
	}
	if body["tokenValid"] != true {
		t.Errorf("expected tokenValid=true with correct token, got %v", body["tokenValid"])
	}

	// With wrong token - should report tokenValid=false
	req, err = http.NewRequest(http.MethodGet, ts.URL+"/auth/verify", nil)
	if err != nil {
		t.Fatalf("failed to create request: %v", err)
	}
	req.Header.Set("X-AgentVault-Token", "wrong-token")

	resp, err = http.DefaultClient.Do(req)
	if err != nil {
		t.Fatalf("failed to verify auth with wrong token: %v", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		t.Errorf("expected status 200 with wrong token, got %d", resp.StatusCode)
	}

	if err := json.NewDecoder(resp.Body).Decode(&body); err != nil {
		t.Fatalf("failed to decode: %v", err)
	}

	if body["hasToken"] != true {
		t.Errorf("expected hasToken=true, got %v", body["hasToken"])
	}
	if body["tokenValid"] != false {
		t.Errorf("expected tokenValid=false with wrong token, got %v", body["tokenValid"])
	}
}

func TestAgentStateReadEndpoints(t *testing.T) {
	vaultPath, database := setupTestVault(t)
	defer database.Close()

	if _, err := database.Exec(`
		INSERT INTO promotion_records (
			id, agent_id, target_kind, status, candidate, rationale,
			source_run_ids_json, source_observation_ids_json, source_evaluation_ids_json,
			created_at
		) VALUES
		('promo_api_pending', 'agt_1', 'memory', 'proposed', 'Pending memory', 'review me', '["run_1"]', '["obs_1"]', '[]', '2026-09-30T10:00:00Z'),
		('promo_api_done', 'agt_1', 'memory', 'committed', 'Done memory', '', '[]', '["obs_2"]', '[]', '2026-09-30T09:00:00Z')
	`); err != nil {
		t.Fatalf("seed promotions: %v", err)
	}
	if _, err := database.Exec(`
		INSERT INTO evaluation_datasets (id, name, description, agent_id, created_at)
		VALUES ('ds_api_1', 'Golden path', 'Core behavior', 'agt_1', '2026-09-30T10:00:00Z')
	`); err != nil {
		t.Fatalf("seed dataset: %v", err)
	}
	if _, err := database.Exec(`
		INSERT INTO evaluation_cases (id, dataset_id, name, input_json, expected_json, tags_json, created_at)
		VALUES ('case_api_1', 'ds_api_1', 'Create note', '{"prompt":"create"}', '{"type":"note"}', '["golden"]', '2026-09-30T10:01:00Z')
	`); err != nil {
		t.Fatalf("seed case: %v", err)
	}
	if _, err := database.Exec(`
		INSERT INTO experiments (
			id, dataset_id, name, agent_id, agent_revision, status, config_json, created_at, completed_at
		) VALUES (
			'exp_api_1', 'ds_api_1', 'baseline', 'agt_1', 3, 'completed',
			'{"model":"test"}', '2026-09-30T10:02:00Z', '2026-09-30T10:03:00Z'
		)
	`); err != nil {
		t.Fatalf("seed experiment: %v", err)
	}
	if _, err := database.Exec(`
		INSERT INTO experiment_results (
			experiment_id, case_id, score, label, metadata_json, created_at
		) VALUES ('exp_api_1', 'case_api_1', 0.95, 'pass', '{"latency_ms":42}', '2026-09-30T10:03:00Z')
	`); err != nil {
		t.Fatalf("seed result: %v", err)
	}

	ts := newTestServer(t, vaultPath, database)
	defer ts.Close()

	t.Run("promotions defaults to proposed", func(t *testing.T) {
		resp, err := http.Get(ts.URL + "/promotions")
		if err != nil {
			t.Fatalf("GET promotions: %v", err)
		}
		defer resp.Body.Close()
		if resp.StatusCode != http.StatusOK {
			t.Fatalf("expected 200, got %d", resp.StatusCode)
		}
		var body []map[string]interface{}
		if err := json.NewDecoder(resp.Body).Decode(&body); err != nil {
			t.Fatalf("decode promotions: %v", err)
		}
		if len(body) != 1 || body[0]["id"] != "promo_api_pending" {
			t.Fatalf("unexpected promotions: %#v", body)
		}
		sources, ok := body[0]["sourceObservationIds"].([]interface{})
		if !ok || len(sources) != 1 || sources[0] != "obs_1" {
			t.Fatalf("unexpected source observation ids: %#v", body[0]["sourceObservationIds"])
		}
	})

	t.Run("dataset includes cases", func(t *testing.T) {
		resp, err := http.Get(ts.URL + "/evaluation-datasets/ds_api_1")
		if err != nil {
			t.Fatalf("GET dataset: %v", err)
		}
		defer resp.Body.Close()
		if resp.StatusCode != http.StatusOK {
			t.Fatalf("expected 200, got %d", resp.StatusCode)
		}
		var body map[string]interface{}
		if err := json.NewDecoder(resp.Body).Decode(&body); err != nil {
			t.Fatalf("decode dataset: %v", err)
		}
		if body["name"] != "Golden path" {
			t.Fatalf("unexpected dataset: %#v", body)
		}
		cases, ok := body["cases"].([]interface{})
		if !ok || len(cases) != 1 {
			t.Fatalf("unexpected cases: %#v", body["cases"])
		}
	})

	t.Run("experiment includes results", func(t *testing.T) {
		resp, err := http.Get(ts.URL + "/experiments/exp_api_1")
		if err != nil {
			t.Fatalf("GET experiment: %v", err)
		}
		defer resp.Body.Close()
		if resp.StatusCode != http.StatusOK {
			t.Fatalf("expected 200, got %d", resp.StatusCode)
		}
		var body map[string]interface{}
		if err := json.NewDecoder(resp.Body).Decode(&body); err != nil {
			t.Fatalf("decode experiment: %v", err)
		}
		if body["name"] != "baseline" {
			t.Fatalf("unexpected experiment: %#v", body)
		}
		results, ok := body["results"].([]interface{})
		if !ok || len(results) != 1 {
			t.Fatalf("unexpected results: %#v", body["results"])
		}
	})
}

func TestAgentStateWriteEndpoints(t *testing.T) {
	vaultPath, database := setupTestVault(t)
	defer database.Close()

	srv := NewServer(vaultPath, database)
	srv.RegisterRoutes()
	var handler http.Handler = srv.mux
	handler = srv.authMiddleware(handler)
	handler = srv.corsMiddleware(handler)
	ts := httptest.NewServer(handler)
	defer ts.Close()

	post := func(t *testing.T, path string, body interface{}, withAuth bool) (*http.Response, map[string]interface{}) {
		t.Helper()
		payload, err := json.Marshal(body)
		if err != nil {
			t.Fatalf("marshal request: %v", err)
		}
		req, err := http.NewRequest(http.MethodPost, ts.URL+path, bytes.NewReader(payload))
		if err != nil {
			t.Fatalf("create request: %v", err)
		}
		req.Header.Set("Content-Type", "application/json")
		if withAuth {
			req.Header.Set("X-AgentVault-Token", srv.AuthToken())
		}
		resp, err := http.DefaultClient.Do(req)
		if err != nil {
			t.Fatalf("POST %s: %v", path, err)
		}
		var decoded map[string]interface{}
		if err := json.NewDecoder(resp.Body).Decode(&decoded); err != nil {
			resp.Body.Close()
			t.Fatalf("decode %s response: %v", path, err)
		}
		resp.Body.Close()
		return resp, decoded
	}

	t.Run("writes require auth", func(t *testing.T) {
		resp, body := post(t, "/evaluation-datasets", map[string]interface{}{"name": "No auth"}, false)
		if resp.StatusCode != http.StatusUnauthorized {
			t.Fatalf("expected 401, got %d body=%#v", resp.StatusCode, body)
		}
	})

	var promotionID string
	t.Run("propose review and commit promotion", func(t *testing.T) {
		resp, body := post(t, "/promotions", map[string]interface{}{
			"agentId":              "agt_api_1",
			"targetKind":           "memory",
			"candidate":            "This is a test note for the API server.",
			"rationale":            "Observed repeatedly",
			"sourceObservationIds": []string{"obs_api_1"},
		}, true)
		if resp.StatusCode != http.StatusCreated {
			t.Fatalf("expected 201 proposing promotion, got %d body=%#v", resp.StatusCode, body)
		}
		promotionID, _ = body["id"].(string)
		if promotionID == "" || body["status"] != "proposed" {
			t.Fatalf("unexpected promotion response: %#v", body)
		}

		resp, body = post(t, "/promotions/"+promotionID+"/review", map[string]interface{}{
			"decision": "approve",
			"reviewer": "human:test",
			"note":     "Evidence is sufficient.",
		}, true)
		if resp.StatusCode != http.StatusOK || body["status"] != "approved" || body["reviewedBy"] != "human:test" {
			t.Fatalf("unexpected review response: status=%d body=%#v", resp.StatusCode, body)
		}

		resp, body = post(t, "/promotions/"+promotionID+"/review", map[string]interface{}{
			"decision": "reject",
			"reviewer": "human:test",
		}, true)
		if resp.StatusCode != http.StatusConflict {
			t.Fatalf("expected 409 for invalid promotion transition, got %d body=%#v", resp.StatusCode, body)
		}

		resp, body = post(t, "/promotions/"+promotionID+"/commit", map[string]interface{}{
			"targetNoteId": "note_2024_01_15_123",
		}, true)
		if resp.StatusCode != http.StatusOK || body["status"] != "committed" || body["targetNoteId"] != "note_2024_01_15_123" {
			t.Fatalf("unexpected commit response: status=%d body=%#v", resp.StatusCode, body)
		}
	})

	var datasetID, caseID string
	t.Run("create evaluation dataset and case", func(t *testing.T) {
		resp, body := post(t, "/evaluation-datasets", map[string]interface{}{
			"name":        "HTTP golden path",
			"description": "Client-created regression set",
			"agentId":     "agt_api_1",
		}, true)
		if resp.StatusCode != http.StatusCreated {
			t.Fatalf("expected 201 creating dataset, got %d body=%#v", resp.StatusCode, body)
		}
		datasetID, _ = body["id"].(string)
		if datasetID == "" || body["name"] != "HTTP golden path" {
			t.Fatalf("unexpected dataset response: %#v", body)
		}

		resp, body = post(t, "/evaluation-datasets/"+datasetID+"/cases", map[string]interface{}{
			"name":     "Create note",
			"input":    map[string]interface{}{"prompt": "create a note"},
			"expected": map[string]interface{}{"type": "note"},
			"tags":     []string{"golden", "http"},
		}, true)
		if resp.StatusCode != http.StatusCreated {
			t.Fatalf("expected 201 creating case, got %d body=%#v", resp.StatusCode, body)
		}
		caseID, _ = body["id"].(string)
		if caseID == "" || body["datasetId"] != datasetID {
			t.Fatalf("unexpected case response: %#v", body)
		}
	})

	var experimentID string
	t.Run("record experiment and result", func(t *testing.T) {
		resp, body := post(t, "/experiments", map[string]interface{}{
			"datasetId":     datasetID,
			"name":          "HTTP baseline",
			"agentId":       "agt_api_1",
			"agentRevision": 4,
			"status":        "completed",
			"config":        map[string]interface{}{"model": "test"},
		}, true)
		if resp.StatusCode != http.StatusCreated {
			t.Fatalf("expected 201 recording experiment, got %d body=%#v", resp.StatusCode, body)
		}
		experimentID, _ = body["id"].(string)
		if experimentID == "" || body["status"] != "completed" || body["agentRevision"] != float64(4) {
			t.Fatalf("unexpected experiment response: %#v", body)
		}

		resp, body = post(t, "/experiments/"+experimentID+"/results", map[string]interface{}{
			"caseId":   caseID,
			"score":    0.9,
			"label":    "pass",
			"metadata": map[string]interface{}{"latencyMs": 42},
		}, true)
		if resp.StatusCode != http.StatusCreated {
			t.Fatalf("expected 201 recording result, got %d body=%#v", resp.StatusCode, body)
		}
		if body["experimentId"] != experimentID || body["caseId"] != caseID || body["score"] != 0.9 || body["label"] != "pass" {
			t.Fatalf("unexpected experiment result response: %#v", body)
		}
	})
}

func TestContextCompilerEndpoints(t *testing.T) {
	vaultPath, database := setupTestVault(t)
	defer database.Close()

	identity := `---
id: identity_api_1
type: note
title: API Agent Identity
---
Prefer deterministic, evidence-backed changes.
`
	agent := `---
id: agt_api_context
type: agent
title: API Context Agent
revision: 2
identity_ref: identity_api_1
memory_refs: [note_2024_01_15_123]
knowledge_scopes: [project:test-project]
artifact_scopes: []
conversation_scopes: []
capability_refs: [filesystem]
context_policy_ref: ""
---
Compile only explicit retrieved evidence.
`
	if err := os.WriteFile(filepath.Join(vaultPath, "10-notes", "identity-api.md"), []byte(identity), 0644); err != nil {
		t.Fatal(err)
	}
	if err := os.MkdirAll(filepath.Join(vaultPath, "75-agents"), 0755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(vaultPath, "75-agents", "api-context-agent.md"), []byte(agent), 0644); err != nil {
		t.Fatal(err)
	}
	if _, err := indexer.New(database, vaultPath).Index(indexer.IndexOptions{}); err != nil {
		t.Fatal(err)
	}

	srv := NewServer(vaultPath, database)
	srv.RegisterRoutes()
	var handler http.Handler = srv.mux
	handler = srv.authMiddleware(handler)
	handler = srv.corsMiddleware(handler)
	ts := httptest.NewServer(handler)
	defer ts.Close()

	payload := map[string]interface{}{
		"task":             "Prepare the next implementation slice.",
		"retrievedNoteIds": []string{"note_2024_01_15_123"},
	}
	bodyBytes, _ := json.Marshal(payload)

	t.Run("compile requires auth", func(t *testing.T) {
		resp, err := http.Post(ts.URL+"/agents/agt_api_context/context", "application/json", bytes.NewReader(bodyBytes))
		if err != nil {
			t.Fatal(err)
		}
		defer resp.Body.Close()
		if resp.StatusCode != http.StatusUnauthorized {
			t.Fatalf("expected 401, got %d", resp.StatusCode)
		}
	})

	var hash string
	t.Run("compile is deterministic and persisted", func(t *testing.T) {
		compile := func() map[string]interface{} {
			req, err := http.NewRequest(http.MethodPost, ts.URL+"/agents/agt_api_context/context", bytes.NewReader(bodyBytes))
			if err != nil {
				t.Fatal(err)
			}
			req.Header.Set("Content-Type", "application/json")
			req.Header.Set("X-AgentVault-Token", srv.AuthToken())
			resp, err := http.DefaultClient.Do(req)
			if err != nil {
				t.Fatal(err)
			}
			defer resp.Body.Close()
			if resp.StatusCode != http.StatusOK {
				t.Fatalf("expected 200, got %d", resp.StatusCode)
			}
			var out map[string]interface{}
			if err := json.NewDecoder(resp.Body).Decode(&out); err != nil {
				t.Fatal(err)
			}
			return out
		}

		first := compile()
		second := compile()
		hash, _ = first["hash"].(string)
		if !strings.HasPrefix(hash, "sha256:") {
			t.Fatalf("expected sha256 hash, got %#v", first["hash"])
		}
		if second["hash"] != hash {
			t.Fatalf("expected deterministic hash, first=%s second=%v", hash, second["hash"])
		}
		if first["agentRevision"] != float64(2) {
			t.Fatalf("expected revision 2, got %#v", first["agentRevision"])
		}
		sections, ok := first["sections"].([]interface{})
		if !ok || len(sections) < 5 {
			t.Fatalf("expected compiled sections, got %#v", first["sections"])
		}
	})

	t.Run("stored snapshot is retrievable", func(t *testing.T) {
		resp, err := http.Get(ts.URL + "/contexts/" + hash)
		if err != nil {
			t.Fatal(err)
		}
		defer resp.Body.Close()
		if resp.StatusCode != http.StatusOK {
			t.Fatalf("expected 200, got %d", resp.StatusCode)
		}
		var out map[string]interface{}
		if err := json.NewDecoder(resp.Body).Decode(&out); err != nil {
			t.Fatal(err)
		}
		if out["hash"] != hash || out["agentId"] != "agt_api_context" {
			t.Fatalf("unexpected snapshot: %#v", out)
		}
		text, _ := out["text"].(string)
		if !strings.Contains(text, "Prepare the next implementation slice.") {
			t.Fatalf("compiled text missing task: %q", text)
		}
	})
}

func TestRunAuditEndpoints(t *testing.T) {
	vaultPath, database := setupTestVault(t)
	defer database.Close()

	snapshotJSON, err := json.Marshal(map[string]interface{}{
		"hash": "sha256:http-run-context", "agentId": "agt_http_run", "agentRevision": 5,
		"agentTitle": "HTTP Run Agent", "task": "run task", "conversationId": "",
		"knowledgeScopes": []string{"project:test"}, "artifactScopes": []string{},
		"conversationScopes": []string{}, "capabilityRefs": []string{"github"},
		"contextPolicyRef": "", "sections": []map[string]interface{}{
			{"kind": "identity", "sourceId": "identity_http", "sourcePath": "10-notes/identity-http.md", "title": "Identity", "content": "Be precise."},
		},
		"unresolved": []interface{}{}, "text": "compiled HTTP context",
	})
	if err != nil {
		t.Fatal(err)
	}
	if _, err := database.Exec(`
		INSERT INTO context_snapshots (
			hash, agent_id, agent_revision, task, context_json, created_at
		) VALUES (?, ?, ?, ?, ?, datetime('now'))
	`, "sha256:http-run-context", "agt_http_run", 5, "run task", string(snapshotJSON)); err != nil {
		t.Fatal(err)
	}

	srv := NewServer(vaultPath, database)
	srv.RegisterRoutes()
	var handler http.Handler = srv.mux
	handler = srv.authMiddleware(handler)
	handler = srv.corsMiddleware(handler)
	ts := httptest.NewServer(handler)
	defer ts.Close()

	postRun := func(body map[string]interface{}, withAuth bool) (*http.Response, map[string]interface{}) {
		payload, _ := json.Marshal(body)
		req, err := http.NewRequest(http.MethodPost, ts.URL+"/runs", bytes.NewReader(payload))
		if err != nil {
			t.Fatal(err)
		}
		req.Header.Set("Content-Type", "application/json")
		if withAuth {
			req.Header.Set("X-AgentVault-Token", srv.AuthToken())
		}
		resp, err := http.DefaultClient.Do(req)
		if err != nil {
			t.Fatal(err)
		}
		defer resp.Body.Close()
		var out map[string]interface{}
		if err := json.NewDecoder(resp.Body).Decode(&out); err != nil {
			t.Fatalf("decode run response: %v", err)
		}
		return resp, out
	}

	body := map[string]interface{}{
		"agentName": "http-agent", "agentId": "agt_http_run", "agentRevision": 5,
		"task": "run task", "status": "succeeded", "contextHash": "sha256:http-run-context",
		"input":              map[string]interface{}{"issue": 81},
		"output":             map[string]interface{}{"result": "ok"},
		"capabilitySnapshot": map[string]interface{}{"github": "read"},
		"runtimeMetadata":    map[string]interface{}{"runtime": "test"},
		"filesChanged":       []string{"README.md"},
	}

	t.Run("run creation requires auth", func(t *testing.T) {
		resp, _ := postRun(body, false)
		if resp.StatusCode != http.StatusUnauthorized {
			t.Fatalf("expected 401, got %d", resp.StatusCode)
		}
	})

	var runID string
	t.Run("records context-bound run", func(t *testing.T) {
		resp, out := postRun(body, true)
		if resp.StatusCode != http.StatusCreated {
			t.Fatalf("expected 201, got %d body=%#v", resp.StatusCode, out)
		}
		runID, _ = out["id"].(string)
		if runID == "" || out["contextHash"] != "sha256:http-run-context" || out["agentRevision"] != float64(5) {
			t.Fatalf("unexpected run response: %#v", out)
		}
	})

	t.Run("rejects mismatched context revision", func(t *testing.T) {
		bad := map[string]interface{}{}
		for k, v := range body {
			bad[k] = v
		}
		bad["agentRevision"] = 6
		resp, out := postRun(bad, true)
		if resp.StatusCode != http.StatusConflict {
			t.Fatalf("expected 409, got %d body=%#v", resp.StatusCode, out)
		}
	})

	t.Run("audit returns context provenance and evidence", func(t *testing.T) {
		if _, err := database.Exec(`
			INSERT INTO run_observations (
				id, run_id, kind, name, status, input_json, output_json, evidence_json, created_at
			) VALUES ('obs_http_audit', ?, 'retrieval', 'search', 'succeeded', '{}', '{}',
				'{"noteIds":["identity_http"]}', datetime('now'))
		`, runID); err != nil {
			t.Fatal(err)
		}
		if _, err := database.Exec(`
			INSERT INTO evaluations (
				id, run_id, observation_id, evaluator, name, label, metadata_json, created_at
			) VALUES ('eval_http_audit', ?, 'obs_http_audit', 'human:test', 'correctness',
				'pass', '{}', datetime('now'))
		`, runID); err != nil {
			t.Fatal(err)
		}

		resp, err := http.Get(ts.URL + "/runs/" + runID + "/audit")
		if err != nil {
			t.Fatal(err)
		}
		defer resp.Body.Close()
		if resp.StatusCode != http.StatusOK {
			t.Fatalf("expected 200, got %d", resp.StatusCode)
		}
		var out map[string]interface{}
		if err := json.NewDecoder(resp.Body).Decode(&out); err != nil {
			t.Fatal(err)
		}
		run, _ := out["run"].(map[string]interface{})
		context, _ := out["context"].(map[string]interface{})
		sections, _ := context["sections"].([]interface{})
		observations, _ := out["observations"].([]interface{})
		evaluations, _ := out["evaluations"].([]interface{})
		if run["id"] != runID || context["hash"] != "sha256:http-run-context" {
			t.Fatalf("unexpected audit linkage: %#v", out)
		}
		if len(sections) != 1 || len(observations) != 1 || len(evaluations) != 1 {
			t.Fatalf("expected context provenance + evidence, got %#v", out)
		}
	})
}

func TestRunLearningCandidateEndpoint(t *testing.T) {
	vaultPath, database := setupTestVault(t)
	defer database.Close()

	if _, err := database.Exec(`
		INSERT INTO agent_runs (
			id, agent_name, agent_id, agent_revision, task, status, created_at
		) VALUES
		('run_learning_api', 'learning-agent', 'agt_learning_api', 3, 'fix regression', 'failed', datetime('now')),
		('run_other_api', 'learning-agent', 'agt_learning_api', 3, 'other run', 'failed', datetime('now'))
	`); err != nil {
		t.Fatal(err)
	}
	if _, err := database.Exec(`
		INSERT INTO run_observations (
			id, run_id, kind, name, status, input_json, output_json, evidence_json, created_at
		) VALUES
		('obs_learning_api', 'run_learning_api', 'tool', 'go test', 'failed', '{}', '{}', '{}', datetime('now')),
		('obs_other_api', 'run_other_api', 'tool', 'other', 'failed', '{}', '{}', '{}', datetime('now'))
	`); err != nil {
		t.Fatal(err)
	}
	if _, err := database.Exec(`
		INSERT INTO evaluations (
			id, run_id, observation_id, evaluator, name, label, rationale, metadata_json, created_at
		) VALUES
		('eval_learning_api', 'run_learning_api', 'obs_learning_api', 'human:test', 'regression', 'fail', 'Missing focused test.', '{}', datetime('now')),
		('eval_other_api', 'run_other_api', 'obs_other_api', 'human:test', 'regression', 'fail', 'Other run.', '{}', datetime('now'))
	`); err != nil {
		t.Fatal(err)
	}

	srv := NewServer(vaultPath, database)
	srv.RegisterRoutes()
	var handler http.Handler = srv.mux
	handler = srv.authMiddleware(handler)
	handler = srv.corsMiddleware(handler)
	ts := httptest.NewServer(handler)
	defer ts.Close()

	post := func(runID string, body map[string]interface{}, withAuth bool) (*http.Response, map[string]interface{}) {
		payload, _ := json.Marshal(body)
		req, err := http.NewRequest(http.MethodPost, ts.URL+"/runs/"+runID+"/learning-candidates", bytes.NewReader(payload))
		if err != nil {
			t.Fatal(err)
		}
		req.Header.Set("Content-Type", "application/json")
		if withAuth {
			req.Header.Set("X-AgentVault-Token", srv.AuthToken())
		}
		resp, err := http.DefaultClient.Do(req)
		if err != nil {
			t.Fatal(err)
		}
		defer resp.Body.Close()
		var out map[string]interface{}
		if err := json.NewDecoder(resp.Body).Decode(&out); err != nil {
			t.Fatalf("decode learning response: %v", err)
		}
		return resp, out
	}

	body := map[string]interface{}{
		"targetKind":           "memory",
		"candidate":            "Run the focused regression test before broad verification.",
		"rationale":            "The failed evaluation identified a missing verification step.",
		"sourceObservationIds": []string{"obs_learning_api"},
		"sourceEvaluationIds":  []string{"eval_learning_api"},
	}

	t.Run("requires auth", func(t *testing.T) {
		resp, _ := post("run_learning_api", body, false)
		if resp.StatusCode != http.StatusUnauthorized {
			t.Fatalf("expected 401, got %d", resp.StatusCode)
		}
	})

	t.Run("derives agent and records evidence lineage", func(t *testing.T) {
		resp, out := post("run_learning_api", body, true)
		if resp.StatusCode != http.StatusCreated {
			t.Fatalf("expected 201, got %d body=%#v", resp.StatusCode, out)
		}
		if out["agentId"] != "agt_learning_api" || out["status"] != "proposed" {
			t.Fatalf("unexpected promotion identity/state: %#v", out)
		}
		runIDs, _ := out["sourceRunIds"].([]interface{})
		evalIDs, _ := out["sourceEvaluationIds"].([]interface{})
		if len(runIDs) != 1 || runIDs[0] != "run_learning_api" {
			t.Fatalf("expected originating run lineage, got %#v", out["sourceRunIds"])
		}
		if len(evalIDs) != 1 || evalIDs[0] != "eval_learning_api" {
			t.Fatalf("expected evaluation lineage, got %#v", out["sourceEvaluationIds"])
		}
	})

	t.Run("rejects evidence from another run", func(t *testing.T) {
		bad := map[string]interface{}{}
		for k, v := range body {
			bad[k] = v
		}
		bad["sourceEvaluationIds"] = []string{"eval_other_api"}
		resp, out := post("run_learning_api", bad, true)
		if resp.StatusCode != http.StatusConflict {
			t.Fatalf("expected 409, got %d body=%#v", resp.StatusCode, out)
		}
	})
}

func TestLearningRecommendationEndpoint(t *testing.T) {
	vaultPath, database := setupTestVault(t)
	defer database.Close()

	if _, err := database.Exec(`
		INSERT INTO agent_runs (
			id, agent_name, agent_id, agent_revision, task, status, created_at
		) VALUES ('run_recommend_api', 'recommend-agent', 'agt_recommend_api', 2, 'fix regression', 'failed', datetime('now'))
	`); err != nil {
		t.Fatal(err)
	}
	if _, err := database.Exec(`
		INSERT INTO run_observations (
			id, run_id, kind, name, status, input_json, output_json, evidence_json, created_at
		) VALUES ('obs_recommend_api', 'run_recommend_api', 'tool', 'go test', 'failed', '{}', '{}', '{}', datetime('now'))
	`); err != nil {
		t.Fatal(err)
	}
	if _, err := database.Exec(`
		INSERT INTO evaluations (
			id, run_id, observation_id, evaluator, name, label, rationale, metadata_json, created_at
		) VALUES ('eval_recommend_api', 'run_recommend_api', 'obs_recommend_api', 'human:test',
			'regression', 'fail', 'Focused test was skipped.',
			'{"target_kind":"memory","supersedes_note_id":"memory_api_old"}', datetime('now'))
	`); err != nil {
		t.Fatal(err)
	}

	ts := newTestServer(t, vaultPath, database)
	defer ts.Close()

	resp, err := http.Get(ts.URL + "/runs/run_recommend_api/learning-recommendation")
	if err != nil {
		t.Fatal(err)
	}
	defer resp.Body.Close()
	if resp.StatusCode != http.StatusOK {
		t.Fatalf("expected 200, got %d", resp.StatusCode)
	}
	var out map[string]interface{}
	if err := json.NewDecoder(resp.Body).Decode(&out); err != nil {
		t.Fatal(err)
	}
	if out["agentId"] != "agt_recommend_api" || out["eligible"] != true || out["supportLevel"] != "strong" {
		t.Fatalf("unexpected recommendation response: %#v", out)
	}
	if out["suggestedTargetKind"] != "memory" || out["evidenceCount"] != float64(2) {
		t.Fatalf("unexpected recommendation hints: %#v", out)
	}
	evals, _ := out["sourceEvaluationIds"].([]interface{})
	if len(evals) != 1 || evals[0] != "eval_recommend_api" {
		t.Fatalf("unexpected evaluation sources: %#v", out["sourceEvaluationIds"])
	}
}

func TestRunRegressionCaseEndpoints(t *testing.T) {
	vaultPath, database := setupTestVault(t)
	defer database.Close()

	if _, err := database.Exec(`
		INSERT INTO evaluation_datasets (id, name, agent_id, created_at)
		VALUES ('ds_regression_api', 'Regression suite', 'agt_regression_api', datetime('now'))
	`); err != nil {
		t.Fatal(err)
	}
	if _, err := database.Exec(`
		INSERT INTO agent_runs (
			id, agent_name, agent_id, agent_revision, task, status, input_json, created_at
		) VALUES (
			'run_regression_api', 'regression-agent', 'agt_regression_api', 3,
			'fix checkout', 'failed', '{"fixture":"checkout-42"}', datetime('now')
		)
	`); err != nil {
		t.Fatal(err)
	}
	if _, err := database.Exec(`
		INSERT INTO run_observations (
			id, run_id, kind, name, status, input_json, output_json, evidence_json, created_at
		) VALUES (
			'obs_regression_api', 'run_regression_api', 'tool', 'checkout test', 'failed',
			'{}', '{}', '{}', datetime('now')
		)
	`); err != nil {
		t.Fatal(err)
	}
	if _, err := database.Exec(`
		INSERT INTO evaluations (
			id, run_id, observation_id, evaluator, name, label, metadata_json, created_at
		) VALUES (
			'eval_regression_api', 'run_regression_api', 'obs_regression_api',
			'human:test', 'checkout', 'fail', '{"expected":{"status":"pass"}}', datetime('now')
		)
	`); err != nil {
		t.Fatal(err)
	}

	srv := NewServer(vaultPath, database)
	srv.RegisterRoutes()
	var handler http.Handler = srv.mux
	handler = srv.authMiddleware(handler)
	handler = srv.corsMiddleware(handler)
	ts := httptest.NewServer(handler)
	defer ts.Close()

	t.Run("proposal is read only", func(t *testing.T) {
		resp, err := http.Get(ts.URL + "/runs/run_regression_api/regression-case-proposal")
		if err != nil {
			t.Fatal(err)
		}
		defer resp.Body.Close()
		if resp.StatusCode != http.StatusOK {
			t.Fatalf("expected 200, got %d", resp.StatusCode)
		}
		var out map[string]interface{}
		if err := json.NewDecoder(resp.Body).Decode(&out); err != nil {
			t.Fatal(err)
		}
		if out["eligible"] != true || out["agentId"] != "agt_regression_api" || out["name"] != "Regression: fix checkout" {
			t.Fatalf("unexpected proposal: %#v", out)
		}
		if out["supportLevel"] != "strong" {
			t.Fatalf("expected strong support, got %#v", out["supportLevel"])
		}
		var count int
		if err := database.QueryRow("SELECT COUNT(*) FROM evaluation_cases").Scan(&count); err != nil {
			t.Fatal(err)
		}
		if count != 0 {
			t.Fatalf("proposal endpoint must not mutate dataset cases, got %d", count)
		}
	})

	captureBody, _ := json.Marshal(map[string]interface{}{
		"datasetId": "ds_regression_api",
		"tags": []string{"checkout"},
	})

	t.Run("capture requires auth", func(t *testing.T) {
		resp, err := http.Post(
			ts.URL+"/runs/run_regression_api/regression-cases",
			"application/json",
			bytes.NewReader(captureBody),
		)
		if err != nil {
			t.Fatal(err)
		}
		defer resp.Body.Close()
		if resp.StatusCode != http.StatusUnauthorized {
			t.Fatalf("expected 401, got %d", resp.StatusCode)
		}
	})

	t.Run("capture persists provenance and is idempotent", func(t *testing.T) {
		capture := func() map[string]interface{} {
			req, err := http.NewRequest(
				http.MethodPost,
				ts.URL+"/runs/run_regression_api/regression-cases",
				bytes.NewReader(captureBody),
			)
			if err != nil {
				t.Fatal(err)
			}
			req.Header.Set("Content-Type", "application/json")
			req.Header.Set("X-AgentVault-Token", srv.AuthToken())
			resp, err := http.DefaultClient.Do(req)
			if err != nil {
				t.Fatal(err)
			}
			defer resp.Body.Close()
			if resp.StatusCode != http.StatusCreated && resp.StatusCode != http.StatusOK {
				t.Fatalf("expected 200/201, got %d", resp.StatusCode)
			}
			var out map[string]interface{}
			if err := json.NewDecoder(resp.Body).Decode(&out); err != nil {
				t.Fatal(err)
			}
			return out
		}
		first := capture()
		second := capture()
		if first["id"] == "" || second["id"] != first["id"] {
			t.Fatalf("expected idempotent case capture: first=%#v second=%#v", first, second)
		}
		if first["sourceRunId"] != "run_regression_api" || first["agentRevision"] != float64(3) {
			t.Fatalf("missing run provenance: %#v", first)
		}
		evals, _ := first["sourceEvaluationIds"].([]interface{})
		if len(evals) != 1 || evals[0] != "eval_regression_api" {
			t.Fatalf("missing evaluation provenance: %#v", first)
		}
	})
}


func TestExperimentComparisonEndpoint(t *testing.T) {
	vaultPath, database := setupTestVault(t)
	defer database.Close()

	if _, err := database.Exec(`
		INSERT INTO evaluation_datasets (id, name, agent_id, created_at)
		VALUES ('ds_compare_api', 'Regression suite', 'agt_compare_api', datetime('now'))
	`); err != nil {
		t.Fatal(err)
	}
	if _, err := database.Exec(`
		INSERT INTO evaluation_cases (
			id, dataset_id, name, input_json, tags_json, created_at
		) VALUES (
			'case_compare_api', 'ds_compare_api', 'Checkout regression',
			'{"fixture":"checkout"}', '["regression"]', datetime('now')
		)
	`); err != nil {
		t.Fatal(err)
	}
	if _, err := database.Exec(`
		INSERT INTO experiments (
			id, dataset_id, name, agent_id, agent_revision, status, config_json, created_at, completed_at
		) VALUES
		('exp_compare_base', 'ds_compare_api', 'baseline', 'agt_compare_api', 2, 'completed', '{}', datetime('now'), datetime('now')),
		('exp_compare_candidate', 'ds_compare_api', 'candidate', 'agt_compare_api', 3, 'completed', '{}', datetime('now'), datetime('now'))
	`); err != nil {
		t.Fatal(err)
	}
	if _, err := database.Exec(`
		INSERT INTO experiment_results (
			experiment_id, case_id, score, label, metadata_json, created_at
		) VALUES
		('exp_compare_base', 'case_compare_api', 0.2, 'fail', '{}', datetime('now')),
		('exp_compare_candidate', 'case_compare_api', 0.9, 'pass', '{}', datetime('now'))
	`); err != nil {
		t.Fatal(err)
	}

	ts := newTestServer(t, vaultPath, database)
	defer ts.Close()

	resp, err := http.Get(ts.URL + "/experiments/exp_compare_base/compare/exp_compare_candidate")
	if err != nil {
		t.Fatal(err)
	}
	defer resp.Body.Close()
	if resp.StatusCode != http.StatusOK {
		t.Fatalf("expected 200, got %d", resp.StatusCode)
	}

	var out map[string]interface{}
	if err := json.NewDecoder(resp.Body).Decode(&out); err != nil {
		t.Fatal(err)
	}
	if out["baselineExperimentId"] != "exp_compare_base" ||
		out["candidateExperimentId"] != "exp_compare_candidate" ||
		out["agentId"] != "agt_compare_api" {
		t.Fatalf("unexpected comparison identity: %#v", out)
	}
	summary, _ := out["summary"].(map[string]interface{})
	if summary["fixes"] != float64(1) || summary["regressions"] != float64(0) {
		t.Fatalf("unexpected comparison summary: %#v", summary)
	}
	cases, _ := out["cases"].([]interface{})
	if len(cases) != 1 {
		t.Fatalf("expected one case comparison: %#v", out["cases"])
	}
	first, _ := cases[0].(map[string]interface{})
	if first["transition"] != "fixed" || first["scoreDelta"] != 0.7 {
		t.Fatalf("unexpected case comparison: %#v", first)
	}
}

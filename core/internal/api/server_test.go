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
	folders := []string{"00-inbox", "10-notes", "20-projects", "30-decisions", "50-actions"}
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

func TestNoteLinksEndpoint(t *testing.T) {
	vaultPath, database := setupTestVault(t)
	defer database.Close()

	// Seed a second note that links to the test note.
	linkedContent := `---
id: note_linked
type: note
title: Linked Note
---

See [[Test Note]] for context.
`
	if err := os.WriteFile(filepath.Join(vaultPath, "10-notes", "linked.md"), []byte(linkedContent), 0644); err != nil {
		t.Fatalf("failed to write linked note: %v", err)
	}
	idx := indexer.New(database, vaultPath)
	if _, err := idx.Index(indexer.IndexOptions{}); err != nil {
		t.Fatalf("failed to reindex: %v", err)
	}

	ts := newTestServer(t, vaultPath, database)
	defer ts.Close()

	resp, err := http.Get(ts.URL + "/notes/note_2024_01_15_123/links")
	if err != nil {
		t.Fatalf("failed to get note links: %v", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		t.Errorf("expected status 200, got %d", resp.StatusCode)
	}

	var body map[string]interface{}
	if err := json.NewDecoder(resp.Body).Decode(&body); err != nil {
		t.Fatalf("failed to decode response: %v", err)
	}

	backlinks, ok := body["backlinks"].([]interface{})
	if !ok {
		t.Fatalf("expected backlinks array, got %T", body["backlinks"])
	}
	if len(backlinks) != 1 {
		t.Errorf("expected 1 backlink, got %d", len(backlinks))
	}

	outgoing, ok := body["outgoing"].([]interface{})
	if !ok {
		t.Fatalf("expected outgoing array, got %T", body["outgoing"])
	}
	if len(outgoing) != 0 {
		t.Errorf("expected 0 outgoing links, got %d", len(outgoing))
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

func TestDashboardEndpoint(t *testing.T) {
	vaultPath, database := setupTestVault(t)
	defer database.Close()

	// Seed an overdue open task, a future open task, and a pending decision.
	taskOverdue := `---
id: task_overdue
type: task
title: Overdue Task
status: open
priority: high
due_date: 2020-01-01
created: 2024-01-01T00:00:00Z
updated: 2024-01-01T00:00:00Z
---

Do this yesterday.
`
	taskFuture := `---
id: task_future
type: task
title: Future Task
status: open
priority: medium
due_date: 2099-12-31
created: 2024-01-01T00:00:00Z
updated: 2024-01-01T00:00:00Z
---

Do this later.
`
	decisionPending := `---
id: decision_pending
type: decision
title: Pending Decision
status: proposed
created: 2024-01-01T00:00:00Z
updated: 2024-01-01T00:00:00Z
---

Should we do it?
`

	if err := os.WriteFile(filepath.Join(vaultPath, "50-actions", "overdue.md"), []byte(taskOverdue), 0644); err != nil {
		t.Fatalf("failed to write overdue task: %v", err)
	}
	if err := os.WriteFile(filepath.Join(vaultPath, "50-actions", "future.md"), []byte(taskFuture), 0644); err != nil {
		t.Fatalf("failed to write future task: %v", err)
	}
	if err := os.WriteFile(filepath.Join(vaultPath, "30-decisions", "pending.md"), []byte(decisionPending), 0644); err != nil {
		t.Fatalf("failed to write pending decision: %v", err)
	}

	idx := indexer.New(database, vaultPath)
	if _, err := idx.Index(indexer.IndexOptions{}); err != nil {
		t.Fatalf("failed to index: %v", err)
	}

	ts := newTestServer(t, vaultPath, database)
	defer ts.Close()

	resp, err := http.Get(ts.URL + "/dashboard")
	if err != nil {
		t.Fatalf("failed to get dashboard: %v", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		t.Errorf("expected status 200, got %d", resp.StatusCode)
	}

	var body map[string]interface{}
	if err := json.NewDecoder(resp.Body).Decode(&body); err != nil {
		t.Fatalf("failed to decode dashboard: %v", err)
	}

	overdue, ok := body["overdueTasks"].([]interface{})
	if !ok {
		t.Fatalf("expected overdueTasks array, got %T", body["overdueTasks"])
	}
	foundOverdue := false
	for _, item := range overdue {
		task, _ := item.(map[string]interface{})
		if task["id"] == "task_overdue" {
			foundOverdue = true
		}
	}
	if !foundOverdue {
		t.Errorf("expected overdue task in dashboard, got: %v", overdue)
	}

	upcoming, ok := body["upcomingTasks"].([]interface{})
	if !ok {
		t.Fatalf("expected upcomingTasks array, got %T", body["upcomingTasks"])
	}
	foundUpcoming := false
	for _, item := range upcoming {
		task, _ := item.(map[string]interface{})
		if task["id"] == "task_future" {
			foundUpcoming = true
		}
	}
	if !foundUpcoming {
		t.Errorf("expected upcoming task in dashboard, got: %v", upcoming)
	}

	pending, ok := body["pendingDecisions"].([]interface{})
	if !ok {
		t.Fatalf("expected pendingDecisions array, got %T", body["pendingDecisions"])
	}
	foundPending := false
	for _, item := range pending {
		d, _ := item.(map[string]interface{})
		if d["id"] == "decision_pending" {
			foundPending = true
		}
	}
	if !foundPending {
		t.Errorf("expected pending decision in dashboard, got: %v", pending)
	}
}

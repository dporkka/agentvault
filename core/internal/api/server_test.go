package api

import (
	"bytes"
	"encoding/json"
	"fmt"
	"net/http"
	"net/http/httptest"
	"os"
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
	config := fmt.Sprintf(`{"vaultPath": %q, "createdAt": %q}`, tmpDir, time.Now().UTC().Format(time.RFC3339))
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

	// Check first result has expected fields (keys are capitalized = exported)
	found := false
	for _, r := range results {
		title, ok := r["Title"].(string)
		if ok && strings.Contains(title, "Test") {
			found = true
			if r["ID"] != "note_2024_01_15_123" {
				t.Errorf("expected ID=note_2024_01_15_123, got %v", r["ID"])
			}
			break
		}
	}
	if !found {
		t.Errorf("expected to find 'Test Note' in results, got: %v", results)
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

	var body map[string]interface{}
	if err := json.NewDecoder(resp.Body).Decode(&body); err != nil {
		t.Fatalf("failed to decode: %v", err)
	}

	projects, ok := body["projects"].([]interface{})
	if !ok {
		t.Fatalf("expected projects array, got %T", body["projects"])
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

	if body["status"] != "ok" {
		t.Errorf("expected status=ok, got %v", body["status"])
	}
	if body["branch"] != "main" {
		t.Errorf("expected branch=main, got %v", body["branch"])
	}
	if body["clean"] != true {
		t.Errorf("expected clean=true, got %v", body["clean"])
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
}

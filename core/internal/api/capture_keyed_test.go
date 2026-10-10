package api

import (
	"bytes"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"

	"github.com/agentvault/core/internal/capturekey"
)

func TestCaptureEndpointUsesCanonicalKeyedPathAndReplaysAcrossRetries(t *testing.T) {
	vault, database := setupTestVault(t)
	defer database.Close()
	srv := NewServer(vault, database)
	srv.RegisterRoutes()
	var handler http.Handler = srv.mux
	handler = srv.authMiddleware(handler)
	handler = srv.corsMiddleware(handler)
	ts := httptest.NewServer(handler)
	defer ts.Close()
	key := "agent:device/π:event-9"
	submit := func(body string) (int, map[string]interface{}) {
		t.Helper()
		req, err := http.NewRequest(http.MethodPost, ts.URL+"/capture", bytes.NewBufferString(body))
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
		var result map[string]interface{}
		if err := json.NewDecoder(resp.Body).Decode(&result); err != nil {
			t.Fatal(err)
		}
		return resp.StatusCode, result
	}
	firstStatus, first := submit(`{"title":"Keyed","text":"original","external_id":"agent:device/π:event-9"}`)
	if firstStatus != http.StatusOK || first["path"] != capturekey.CanonicalPath(key) || first["idempotent"] != nil {
		t.Fatalf("first keyed response status=%d payload=%v", firstStatus, first)
	}
	replayStatus, replay := submit(`{"title":"Changed","text":"must not replace","external_id":"agent:device/π:event-9"}`)
	if replayStatus != http.StatusOK || replay["path"] != first["path"] || replay["idempotent"] != true {
		t.Fatalf("retry status=%d payload=%v", replayStatus, replay)
	}
	content, err := os.ReadFile(filepath.Join(vault, filepath.FromSlash(first["path"].(string))))
	if err != nil || !strings.Contains(string(content), "original") || strings.Contains(string(content), "must not replace") {
		t.Fatalf("replay replaced canonical file: %q (%v)", content, err)
	}
	// Existing indexing remains asynchronous.
	time.Sleep(200 * time.Millisecond)
}

func TestCaptureEndpointConflictingKeyedDestinationReturns409(t *testing.T) {
	vault, database := setupTestVault(t)
	defer database.Close()
	key := "blocked"
	path := filepath.Join(vault, filepath.FromSlash(capturekey.CanonicalPath(key)))
	if err := os.MkdirAll(filepath.Dir(path), 0755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(path, []byte("unparseable"), 0644); err != nil {
		t.Fatal(err)
	}
	srv := NewServer(vault, database)
	srv.RegisterRoutes()
	var handler http.Handler = srv.mux
	handler = srv.authMiddleware(handler)
	handler = srv.corsMiddleware(handler)
	ts := httptest.NewServer(handler)
	defer ts.Close()
	req, _ := http.NewRequest(http.MethodPost, ts.URL+"/capture", bytes.NewBufferString(`{"title":"Note","external_id":"blocked"}`))
	req.Header.Set("Content-Type", "application/json")
	req.Header.Set("X-AgentVault-Token", srv.AuthToken())
	resp, err := http.DefaultClient.Do(req)
	if err != nil {
		t.Fatal(err)
	}
	defer resp.Body.Close()
	if resp.StatusCode != http.StatusConflict {
		t.Fatalf("status = %d; want 409", resp.StatusCode)
	}
	data, _ := os.ReadFile(path)
	if string(data) != "unparseable" {
		t.Fatal("conflicting canonical file was modified")
	}
}

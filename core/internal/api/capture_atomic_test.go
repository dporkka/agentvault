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
)

func TestCaptureEndpointKeepsOccupiedSlotAndPublishesCompleteNextNote(t *testing.T) {
	vaultPath, database := setupTestVault(t)
	defer database.Close()

	inbox := filepath.Join(vaultPath, "00-inbox")
	if err := os.MkdirAll(inbox, 0755); err != nil {
		t.Fatal(err)
	}
	date := time.Now().Format("2006-01-02")
	occupied := filepath.Join(inbox, date+"_capture_001.md")
	if err := os.WriteFile(occupied, []byte("previous user capture"), 0644); err != nil {
		t.Fatal(err)
	}

	srv := NewServer(vaultPath, database)
	srv.RegisterRoutes()
	var handler http.Handler = srv.mux
	handler = srv.authMiddleware(handler)
	handler = srv.corsMiddleware(handler)
	ts := httptest.NewServer(handler)
	defer ts.Close()

	req, err := http.NewRequest(http.MethodPost, ts.URL+"/capture", bytes.NewBufferString(`{"title":"New Capture","text":"completed body"}`))
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
		t.Fatalf("capture status = %d; want 200", resp.StatusCode)
	}
	var result struct {
		Path string `json:"path"`
	}
	if err := json.NewDecoder(resp.Body).Decode(&result); err != nil {
		t.Fatal(err)
	}
	if !strings.HasSuffix(result.Path, date+"_capture_002.md") {
		t.Fatalf("occupied slot not skipped: %q", result.Path)
	}
	original, err := os.ReadFile(occupied)
	if err != nil || string(original) != "previous user capture" {
		t.Fatalf("original capture changed: %q (%v)", original, err)
	}
	published, err := os.ReadFile(filepath.Join(vaultPath, result.Path))
	if err != nil {
		t.Fatal(err)
	}
	if !strings.Contains(string(published), "completed body") || !strings.Contains(string(published), "id: capture_") || !strings.Contains(string(published), "_002\n") {
		t.Fatalf("capture returned before content was complete: %q", published)
	}
	entries, err := os.ReadDir(inbox)
	if err != nil {
		t.Fatal(err)
	}
	for _, entry := range entries {
		if strings.Contains(entry.Name(), ".tmp-") {
			t.Errorf("temporary staging file left in inbox: %s", entry.Name())
		}
	}

	// The API currently indexes asynchronously; allow its background task to
	// finish before closing the test database, as in the existing API suite.
	time.Sleep(200 * time.Millisecond)
}

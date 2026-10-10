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

	"github.com/agentvault/core/internal/templates"
)

// All possible random three-digit IDs for the current day are occupied.
// A new note must fail rather than replace any existing user-owned note.
// This is deterministic; no RNG monkey-patching or timing assumptions.
func TestCreateNoteEndpointRefusesOccupiedGeneratedIDs(t *testing.T) {
	vaultPath, database := setupTestVault(t)
	defer database.Close()
	folder := filepath.Join(vaultPath, "10-notes")
	if err := os.MkdirAll(folder, 0755); err != nil {
		t.Fatal(err)
	}
	sample := templates.GenerateID("note")
	prefix := sample[:strings.LastIndexByte(sample, '_')+1]
	const existing = "DO NOT OVERWRITE THIS NOTE\n"
	for suffix := 100; suffix <= 999; suffix++ {
		path := filepath.Join(folder, fmt.Sprintf("%s%d.md", prefix, suffix))
		if err := os.WriteFile(path, []byte(existing), 0644); err != nil {
			t.Fatal(err)
		}
	}

	srv := NewServer(vaultPath, database)
	srv.RegisterRoutes()
	var handler http.Handler = srv.mux
	handler = srv.authMiddleware(handler)
	handler = srv.corsMiddleware(handler)
	ts := httptest.NewServer(handler)
	defer ts.Close()

	req, err := http.NewRequest(http.MethodPost, ts.URL+"/notes", bytes.NewBufferString(`{"type":"note","title":"New and different"}`))
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
	if resp.StatusCode != http.StatusConflict {
		var body map[string]interface{}
		_ = json.NewDecoder(resp.Body).Decode(&body)
		t.Fatalf("occupied note IDs should fail closed with HTTP 409, got %d: %v", resp.StatusCode, body)
	}

	for suffix := 100; suffix <= 999; suffix++ {
		path := filepath.Join(folder, fmt.Sprintf("%s%d.md", prefix, suffix))
		actual, err := os.ReadFile(path)
		if err != nil || string(actual) != existing {
			t.Fatalf("note %d was replaced or removed: %q (%v)", suffix, actual, err)
		}
	}
}

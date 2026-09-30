package api

import (
	"encoding/json"
	"net/http"
	"os"
	"path/filepath"
	"testing"

	"github.com/agentvault/core/internal/db"
	"github.com/agentvault/core/internal/indexer"
	"github.com/agentvault/core/internal/memory"
)

func seedMemoryHTTPFixtures(t *testing.T, vaultPath string, database *db.DB) {
	t.Helper()

	fixtures := map[string]string{
		"10-notes/memory-global.md": `---
id: memory-global
type: note
title: Global deployment fact
memory_kind: fact
memory_confidence: 0.70
provenance:
  source_type: user
  source_ref: seed-global
  actor: user
observed_at: 2026-01-01T00:00:00Z
---
Use the global deployment target.
`,
		"10-notes/memory-workspace.md": `---
id: memory-workspace
type: note
title: Workspace deployment fact
workspace_id: acme
memory_kind: fact
memory_confidence: 0.95
provenance:
  source_type: conversation
  source_ref: seed-workspace
  actor: user
observed_at: 2026-02-01T00:00:00Z
supersedes: [memory-global]
supersession_reason: workspace-specific correction
---
Use the ACME deployment target.
`,
	}

	for relPath, content := range fixtures {
		fullPath := filepath.Join(vaultPath, relPath)
		if err := os.WriteFile(fullPath, []byte(content), 0644); err != nil {
			t.Fatalf("write %s: %v", relPath, err)
		}
	}

	idx := indexer.New(database, vaultPath)
	result, err := idx.Index(indexer.IndexOptions{Force: true})
	if err != nil {
		t.Fatalf("index memory fixtures: %v", err)
	}
	if len(result.Errors) != 0 {
		t.Fatalf("index memory fixtures returned errors: %v", result.Errors)
	}
}

func TestMemoriesEndpointScopesAndSupersession(t *testing.T) {
	vaultPath, database := setupTestVault(t)
	defer database.Close()
	seedMemoryHTTPFixtures(t, vaultPath, database)

	ts := newTestServer(t, vaultPath, database)
	defer ts.Close()

	resp, err := http.Get(ts.URL + "/memories?workspace=acme")
	if err != nil {
		t.Fatalf("GET /memories: %v", err)
	}
	defer resp.Body.Close()
	if resp.StatusCode != http.StatusOK {
		t.Fatalf("expected 200, got %d", resp.StatusCode)
	}

	var records []memory.Record
	if err := json.NewDecoder(resp.Body).Decode(&records); err != nil {
		t.Fatalf("decode memories: %v", err)
	}
	if len(records) != 1 || records[0].NoteID != "memory-workspace" {
		t.Fatalf("ACME memories = %#v, want only workspace replacement", records)
	}

	resp, err = http.Get(ts.URL + "/memories?workspace=other")
	if err != nil {
		t.Fatalf("GET /memories other workspace: %v", err)
	}
	defer resp.Body.Close()
	if resp.StatusCode != http.StatusOK {
		t.Fatalf("expected 200, got %d", resp.StatusCode)
	}
	records = nil
	if err := json.NewDecoder(resp.Body).Decode(&records); err != nil {
		t.Fatalf("decode other workspace memories: %v", err)
	}
	if len(records) != 1 || records[0].NoteID != "memory-global" {
		t.Fatalf("other workspace memories = %#v, want global memory", records)
	}
}

func TestMemoriesEndpointCanIncludeContextuallySuperseded(t *testing.T) {
	vaultPath, database := setupTestVault(t)
	defer database.Close()
	seedMemoryHTTPFixtures(t, vaultPath, database)

	ts := newTestServer(t, vaultPath, database)
	defer ts.Close()

	resp, err := http.Get(ts.URL + "/memories?workspace=acme&include_superseded=true")
	if err != nil {
		t.Fatalf("GET /memories: %v", err)
	}
	defer resp.Body.Close()

	var records []memory.Record
	if err := json.NewDecoder(resp.Body).Decode(&records); err != nil {
		t.Fatalf("decode memories: %v", err)
	}
	if len(records) != 2 {
		t.Fatalf("expected 2 memories with superseded included, got %d", len(records))
	}

	foundSuperseded := false
	for _, record := range records {
		if record.NoteID == "memory-global" {
			foundSuperseded = record.Superseded
		}
	}
	if !foundSuperseded {
		t.Fatal("expected global memory to be marked contextually superseded in ACME")
	}
}

func TestMemoriesEndpointFiltersAndValidation(t *testing.T) {
	vaultPath, database := setupTestVault(t)
	defer database.Close()
	seedMemoryHTTPFixtures(t, vaultPath, database)

	ts := newTestServer(t, vaultPath, database)
	defer ts.Close()

	resp, err := http.Get(ts.URL + "/memories?workspace=acme&kind=fact&min_confidence=0.9&limit=5")
	if err != nil {
		t.Fatalf("GET filtered memories: %v", err)
	}
	defer resp.Body.Close()
	if resp.StatusCode != http.StatusOK {
		t.Fatalf("expected 200, got %d", resp.StatusCode)
	}
	var records []memory.Record
	if err := json.NewDecoder(resp.Body).Decode(&records); err != nil {
		t.Fatalf("decode filtered memories: %v", err)
	}
	if len(records) != 1 || records[0].NoteID != "memory-workspace" {
		t.Fatalf("filtered memories = %#v", records)
	}

	for _, path := range []string{
		"/memories?kind=unknown",
		"/memories?min_confidence=1.5",
		"/memories?at=yesterday",
		"/memories?include_superseded=maybe",
		"/memories?limit=0",
	} {
		resp, err := http.Get(ts.URL + path)
		if err != nil {
			t.Fatalf("GET %s: %v", path, err)
		}
		if resp.StatusCode != http.StatusBadRequest {
			resp.Body.Close()
			t.Fatalf("GET %s: expected 400, got %d", path, resp.StatusCode)
		}
		resp.Body.Close()
	}
}

func TestMemoryByIDEndpoint(t *testing.T) {
	vaultPath, database := setupTestVault(t)
	defer database.Close()
	seedMemoryHTTPFixtures(t, vaultPath, database)

	ts := newTestServer(t, vaultPath, database)
	defer ts.Close()

	resp, err := http.Get(ts.URL + "/memories/memory-workspace")
	if err != nil {
		t.Fatalf("GET memory by id: %v", err)
	}
	defer resp.Body.Close()
	if resp.StatusCode != http.StatusOK {
		t.Fatalf("expected 200, got %d", resp.StatusCode)
	}
	var record memory.Record
	if err := json.NewDecoder(resp.Body).Decode(&record); err != nil {
		t.Fatalf("decode memory: %v", err)
	}
	if record.NoteID != "memory-workspace" || record.Kind != memory.KindFact {
		t.Fatalf("unexpected memory record: %#v", record)
	}
	if record.Scope.WorkspaceID != "acme" {
		t.Fatalf("workspace scope = %q", record.Scope.WorkspaceID)
	}

	resp, err = http.Get(ts.URL + "/memories/note_2024_01_15_123")
	if err != nil {
		t.Fatalf("GET unclassified note as memory: %v", err)
	}
	defer resp.Body.Close()
	if resp.StatusCode != http.StatusNotFound {
		t.Fatalf("unclassified note should not be exposed as memory; got %d", resp.StatusCode)
	}
}

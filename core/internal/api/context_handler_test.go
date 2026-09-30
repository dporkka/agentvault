package api

import (
	"bytes"
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/agentvault/core/internal/contract"
)

func TestCompileContextEndpoint(t *testing.T) {
	vaultPath, database := setupTestVault(t)
	defer database.Close()

	server := NewServer(vaultPath, database)
	server.RegisterRoutes()

	confidence := 0.9
	provenance, err := server.knowledge.CreateProvenance(contract.ProvenanceRecord{
		ID:         "prov_api_context",
		SourceType: "human",
		Confidence: confidence,
	})
	if err != nil {
		t.Fatal(err)
	}
	object, err := server.knowledge.UpsertObject(contract.UpsertKnowledgeObjectRequest{
		ID:           "obj_api_context",
		Type:         "decision",
		Title:        "Use Go for the context compiler",
		Project:      "test-project",
		ProvenanceID: provenance.ID,
	})
	if err != nil {
		t.Fatal(err)
	}
	if _, err := server.knowledge.RecordMemory(contract.CreateMemoryRequest{
		ID:           "mem_api_context",
		MemoryClass:  "semantic",
		MemoryKind:   "decision",
		ScopeType:    "project",
		ScopeID:      "test-project",
		Content:      "The context compiler is deterministic and does not require an LLM call.",
		ObjectID:     object.ID,
		ProvenanceID: provenance.ID,
		Confidence:   &confidence,
	}); err != nil {
		t.Fatal(err)
	}

	body, err := json.Marshal(contract.CompileContextRequest{
		Task:        "Implement deterministic context compiler",
		Project:     "test-project",
		ObjectIDs:   []string{object.ID},
		TokenBudget: 1000,
		AsOf:        "2026-09-10T12:00:00Z",
	})
	if err != nil {
		t.Fatal(err)
	}
	req := httptest.NewRequest(http.MethodPost, "/context/compile", bytes.NewReader(body))
	recorder := httptest.NewRecorder()
	server.mux.ServeHTTP(recorder, req)
	if recorder.Code != http.StatusOK {
		t.Fatalf("expected 200, got %d: %s", recorder.Code, recorder.Body.String())
	}

	var bundle contract.ContextBundle
	if err := json.NewDecoder(recorder.Body).Decode(&bundle); err != nil {
		t.Fatal(err)
	}
	if bundle.Task == "" || bundle.EstimatedTokens > bundle.TokenBudget {
		t.Fatalf("invalid context bundle: %+v", bundle)
	}
	foundMemory := false
	foundObject := false
	for _, item := range bundle.Items {
		switch item.ID {
		case "mem_api_context":
			foundMemory = true
			if item.Provenance == nil || item.Provenance.ID != provenance.ID {
				t.Fatalf("expected memory provenance, got %+v", item.Provenance)
			}
			if item.Metadata["memoryClass"] != "semantic" || item.Metadata["memoryKind"] != "decision" {
				t.Fatalf("expected unified memory metadata, got %+v", item.Metadata)
			}
		case "obj_api_context":
			foundObject = true
		}
	}
	if !foundMemory || !foundObject {
		t.Fatalf("expected structured context items, got %+v", bundle.Items)
	}
}

func TestCompileContextEndpointValidatesBudget(t *testing.T) {
	vaultPath, database := setupTestVault(t)
	defer database.Close()

	server := NewServer(vaultPath, database)
	server.RegisterRoutes()
	req := httptest.NewRequest(http.MethodPost, "/context/compile", bytes.NewBufferString(`{"task":"test","tokenBudget":10}`))
	recorder := httptest.NewRecorder()
	server.mux.ServeHTTP(recorder, req)
	if recorder.Code != http.StatusBadRequest {
		t.Fatalf("expected 400, got %d: %s", recorder.Code, recorder.Body.String())
	}
}

func TestCompileContextEndpointAppliesSavedViewScope(t *testing.T) {
	vaultPath, database := setupTestVault(t)
	defer database.Close()

	viewDir := filepath.Join(vaultPath, ".agentvault", "views")
	if err := os.MkdirAll(viewDir, 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(viewDir, "active-test.yaml"), []byte(`version: 1
name: Active test notes
query:
  projects: [test-project]
  statuses: [active]
`), 0o644); err != nil {
		t.Fatal(err)
	}

	server := NewServer(vaultPath, database)
	server.RegisterRoutes()
	body, err := json.Marshal(contract.CompileContextRequest{
		Task:        "test note body",
		Project:     "test-project",
		ViewID:      "active-test",
		TokenBudget: 1000,
		AsOf:        "2026-09-10T12:00:00Z",
	})
	if err != nil {
		t.Fatal(err)
	}
	req := httptest.NewRequest(http.MethodPost, "/context/compile", bytes.NewReader(body))
	recorder := httptest.NewRecorder()
	server.mux.ServeHTTP(recorder, req)
	if recorder.Code != http.StatusOK {
		t.Fatalf("expected 200, got %d: %s", recorder.Code, recorder.Body.String())
	}

	var bundle contract.ContextBundle
	if err := json.NewDecoder(recorder.Body).Decode(&bundle); err != nil {
		t.Fatal(err)
	}
	if bundle.ViewID != "active-test" {
		t.Fatalf("viewId = %q, want active-test", bundle.ViewID)
	}
}

func TestCompileContextEndpointRejectsMissingSavedView(t *testing.T) {
	vaultPath, database := setupTestVault(t)
	defer database.Close()

	server := NewServer(vaultPath, database)
	server.RegisterRoutes()
	req := httptest.NewRequest(http.MethodPost, "/context/compile", bytes.NewBufferString(`{"task":"test","viewId":"missing-view"}`))
	recorder := httptest.NewRecorder()
	server.mux.ServeHTTP(recorder, req)
	if recorder.Code != http.StatusNotFound {
		t.Fatalf("expected 404, got %d: %s", recorder.Code, recorder.Body.String())
	}
	if !strings.Contains(recorder.Body.String(), "missing-view") {
		t.Fatalf("expected missing view detail, got %s", recorder.Body.String())
	}
}

func TestCompileContextEndpointReturnsConflictForChangedPinnedView(t *testing.T) {
	vaultPath, database := setupTestVault(t)
	defer database.Close()

	viewDir := filepath.Join(vaultPath, ".agentvault", "views")
	if err := os.MkdirAll(viewDir, 0o755); err != nil {
		t.Fatal(err)
	}
	original := []byte("version: 1\nname: Pinned\nquery:\n  projects: [test-project]\n")
	viewPath := filepath.Join(viewDir, "pinned.yaml")
	if err := os.WriteFile(viewPath, original, 0o644); err != nil {
		t.Fatal(err)
	}
	sum := sha256.Sum256(original)
	expectedHash := hex.EncodeToString(sum[:])
	if err := os.WriteFile(viewPath, append(original, []byte("# changed\n")...), 0o644); err != nil {
		t.Fatal(err)
	}

	server := NewServer(vaultPath, database)
	server.RegisterRoutes()
	body, err := json.Marshal(contract.CompileContextRequest{
		Task:                    "test",
		Project:                 "test-project",
		ViewID:                  "pinned",
		ExpectedViewContentHash: expectedHash,
	})
	if err != nil {
		t.Fatal(err)
	}
	req := httptest.NewRequest(http.MethodPost, "/context/compile", bytes.NewReader(body))
	recorder := httptest.NewRecorder()
	server.mux.ServeHTTP(recorder, req)
	if recorder.Code != http.StatusConflict {
		t.Fatalf("expected 409, got %d: %s", recorder.Code, recorder.Body.String())
	}
}

package api

import (
	"bytes"
	"encoding/json"
	"net/http"
	"net/http/httptest"
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

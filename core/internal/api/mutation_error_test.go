package api

import (
	"net/http"
	"net/http/httptest"
	"testing"
)

func TestMutationMissingIDReturnsNotFound(t *testing.T) {
	vaultPath, database := setupTestVault(t)
	defer database.Close()

	server := NewServer(vaultPath, database)
	server.RegisterRoutes()
	recorder := httptest.NewRecorder()
	request := httptest.NewRequest(http.MethodGet, "/mutations/mut_missing", nil)
	request.Header.Set("X-AgentVault-Token", server.AuthToken())
	server.mux.ServeHTTP(recorder, request)

	if recorder.Code != http.StatusNotFound {
		t.Fatalf("GET missing mutation status=%d want=%d body=%s", recorder.Code, http.StatusNotFound, recorder.Body.String())
	}
}

func TestMutationRecoveryFailureDoesNotDisableKnowledgeAPI(t *testing.T) {
	vaultPath, database := setupTestVault(t)
	defer database.Close()

	// Simulate a projected interrupted mutation whose path is no longer valid.
	// Recovery must fail closed for mutation operations without poisoning the
	// independently healthy universal knowledge projection.
	_, err := database.Exec(`
		INSERT INTO mutation_proposals (
			id, mutation_kind, path, reason, status, before_exists, after_exists,
			diff, created_at, updated_at
		) VALUES (?, ?, ?, ?, ?, ?, ?, ?, ?, ?)`,
		"mut_invalid_recovery", "replace", ".git/config", "simulate invalid recovery",
		"committing", 1, 1, "", "2026-09-12T12:00:00Z", "2026-09-12T12:00:00Z",
	)
	if err != nil {
		t.Fatalf("seed interrupted mutation: %v", err)
	}

	server := NewServer(vaultPath, database)
	if server.knowledgeInitErr != nil {
		t.Fatalf("mutation recovery failure poisoned knowledge readiness: %v", server.knowledgeInitErr)
	}
	if server.mutationInitErr == nil {
		t.Fatal("expected mutation subsystem to fail closed")
	}
	server.RegisterRoutes()

	knowledgeRecorder := httptest.NewRecorder()
	knowledgeRequest := httptest.NewRequest(http.MethodGet, "/objects", nil)
	server.mux.ServeHTTP(knowledgeRecorder, knowledgeRequest)
	if knowledgeRecorder.Code != http.StatusOK {
		t.Fatalf("knowledge endpoint status=%d want=%d body=%s", knowledgeRecorder.Code, http.StatusOK, knowledgeRecorder.Body.String())
	}

	mutationRecorder := httptest.NewRecorder()
	mutationRequest := httptest.NewRequest(http.MethodGet, "/mutations", nil)
	mutationRequest.Header.Set("X-AgentVault-Token", server.AuthToken())
	server.mux.ServeHTTP(mutationRecorder, mutationRequest)
	if mutationRecorder.Code != http.StatusInternalServerError {
		t.Fatalf("mutation endpoint status=%d want=%d body=%s", mutationRecorder.Code, http.StatusInternalServerError, mutationRecorder.Body.String())
	}
}

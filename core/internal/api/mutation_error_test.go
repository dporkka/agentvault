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
	server.mux.ServeHTTP(recorder, request)

	if recorder.Code != http.StatusNotFound {
		t.Fatalf("GET missing mutation status=%d want=%d body=%s", recorder.Code, http.StatusNotFound, recorder.Body.String())
	}
}

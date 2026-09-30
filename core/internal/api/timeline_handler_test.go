package api

import (
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/agentvault/core/internal/contract"
)

func TestTimelineEndpointFiltersNormalizedActivity(t *testing.T) {
	vaultPath, database := setupTestVault(t)
	defer database.Close()

	server := NewServer(vaultPath, database)
	session, err := server.knowledge.StartSession(contract.StartAgentSessionRequest{
		ID:        "session_http_timeline",
		AgentID:   "backend-engineer",
		Project:   "agentvault",
		Objective: "Verify timeline HTTP contract",
	})
	if err != nil {
		t.Fatal(err)
	}
	if _, err := server.knowledge.AppendSessionEvent(session.ID, contract.AppendSessionEventRequest{
		ID:        "event_http_timeline",
		EventType: "verification",
		Payload:   map[string]interface{}{"summary": "Timeline endpoint verified"},
	}); err != nil {
		t.Fatal(err)
	}
	if _, err := server.knowledge.RecordEpisode(contract.CreateEpisodeRequest{
		ID:        "episode_http_timeline",
		ScopeType: "project",
		ScopeID:   "agentvault",
		EventType: "milestone",
		Summary:   "Timeline API milestone",
	}); err != nil {
		t.Fatal(err)
	}

	server.RegisterRoutes()
	req := httptest.NewRequest(http.MethodGet, "/timeline?project=agentvault&kind=episode&limit=5", nil)
	rec := httptest.NewRecorder()
	server.mux.ServeHTTP(rec, req)

	if rec.Code != http.StatusOK {
		t.Fatalf("timeline status=%d body=%s", rec.Code, rec.Body.String())
	}
	var items []contract.TimelineItem
	if err := json.NewDecoder(rec.Body).Decode(&items); err != nil {
		t.Fatal(err)
	}
	if len(items) != 1 {
		t.Fatalf("expected one episode, got %+v", items)
	}
	if items[0].ID != "episode_http_timeline" || items[0].Kind != "episode" || items[0].Project != "agentvault" {
		t.Fatalf("unexpected timeline item: %+v", items[0])
	}
}

func TestTimelineEndpointRejectsInvalidTimeRange(t *testing.T) {
	vaultPath, database := setupTestVault(t)
	defer database.Close()

	server := NewServer(vaultPath, database)
	server.RegisterRoutes()
	req := httptest.NewRequest(http.MethodGet, "/timeline?since=2026-09-30T12:00:00Z&until=2026-09-30T11:00:00Z", nil)
	rec := httptest.NewRecorder()
	server.mux.ServeHTTP(rec, req)

	if rec.Code != http.StatusBadRequest {
		t.Fatalf("expected 400, got %d body=%s", rec.Code, rec.Body.String())
	}
}

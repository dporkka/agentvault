package api

import (
	"bytes"
	"encoding/json"

	"github.com/agentvault/core/internal/contract"
	"net/http"
	"net/http/httptest"
	"os"
	"path/filepath"
	"testing"
)

func TestKnowledgeHTTPAPIEndToEnd(t *testing.T) {
	vaultPath, database := setupTestVault(t)
	defer database.Close()

	server := NewServer(vaultPath, database)
	server.RegisterRoutes()
	ts := httptest.NewServer(server.mux)
	defer ts.Close()

	request := func(method, path string, body interface{}, target interface{}, wantStatus int) {
		t.Helper()
		var payload []byte
		var err error
		if body != nil {
			payload, err = json.Marshal(body)
			if err != nil {
				t.Fatalf("marshal %s %s body: %v", method, path, err)
			}
		}
		req, err := http.NewRequest(method, ts.URL+path, bytes.NewReader(payload))
		if err != nil {
			t.Fatalf("new request: %v", err)
		}
		if body != nil {
			req.Header.Set("Content-Type", "application/json")
		}
		resp, err := http.DefaultClient.Do(req)
		if err != nil {
			t.Fatalf("%s %s: %v", method, path, err)
		}
		defer resp.Body.Close()
		if resp.StatusCode != wantStatus {
			var failure map[string]interface{}
			_ = json.NewDecoder(resp.Body).Decode(&failure)
			t.Fatalf("%s %s status=%d want=%d body=%v", method, path, resp.StatusCode, wantStatus, failure)
		}
		if target != nil {
			if err := json.NewDecoder(resp.Body).Decode(target); err != nil {
				t.Fatalf("decode %s %s response: %v", method, path, err)
			}
		}
	}

	var provenance struct {
		ID string `json:"id"`
	}
	request(http.MethodPost, "/provenance", map[string]interface{}{
		"id":         "prov_http",
		"sourceType": "agent-session",
		"agentId":    "architect",
		"confidence": 0.94,
		"evidence": []map[string]interface{}{
			{"source": "file", "path": "README.md"},
		},
	}, &provenance, http.StatusCreated)

	var object struct {
		ID            string `json:"id"`
		CanonicalPath string `json:"canonicalPath"`
	}
	request(http.MethodPost, "/objects", map[string]interface{}{
		"id":            "obj_http",
		"type":          "decision",
		"title":         "Use AgentVault as the shared agent context layer",
		"project":       "agentvault",
		"canonicalPath": "30-decisions/shared-context.md",
		"provenanceId":  provenance.ID,
	}, &object, http.StatusCreated)
	if object.ID != "obj_http" || object.CanonicalPath == "" {
		t.Fatalf("unexpected object response: %+v", object)
	}

	request(http.MethodPost, "/memory", map[string]interface{}{
		"id":           "mem_http",
		"memoryClass":  "semantic",
		"memoryKind":   "decision",
		"scopeType":    "project",
		"scopeId":      "agentvault",
		"content":      "Other products consume AgentVault through stable integration boundaries.",
		"objectId":     object.ID,
		"provenanceId": provenance.ID,
	}, nil, http.StatusCreated)

	var memories []struct {
		ID          string `json:"id"`
		MemoryClass string `json:"memoryClass"`
		MemoryKind  string `json:"memoryKind"`
	}
	request(http.MethodGet, "/memory?scopeType=project&scopeId=agentvault&memoryClass=semantic", nil, &memories, http.StatusOK)
	if len(memories) != 1 || memories[0].ID != "mem_http" || memories[0].MemoryClass != "semantic" || memories[0].MemoryKind != "decision" {
		t.Fatalf("unexpected memories: %+v", memories)
	}

	var session struct {
		ID     string `json:"id"`
		Status string `json:"status"`
	}
	request(http.MethodPost, "/sessions", map[string]interface{}{
		"id":        "session_http",
		"agentId":   "backend-engineer",
		"project":   "agentvault",
		"objective": "Exercise the universal knowledge API",
	}, &session, http.StatusCreated)
	if session.Status != "active" {
		t.Fatalf("unexpected session: %+v", session)
	}

	request(http.MethodPost, "/sessions/"+session.ID+"/events", map[string]interface{}{
		"id":        "event_http",
		"eventType": "verification",
		"payload":   map[string]interface{}{"passed": true},
	}, nil, http.StatusCreated)

	var loadedSession struct {
		Events []struct {
			ID string `json:"id"`
		} `json:"events"`
	}
	request(http.MethodGet, "/sessions/"+session.ID, nil, &loadedSession, http.StatusOK)
	if len(loadedSession.Events) != 1 || loadedSession.Events[0].ID != "event_http" {
		t.Fatalf("unexpected session history: %+v", loadedSession)
	}

	request(http.MethodPost, "/sessions/"+session.ID+"/close", map[string]interface{}{
		"status": "completed",
	}, nil, http.StatusOK)

	journalPath := filepath.Join(vaultPath, "80-agent-runs", "knowledge.journal.jsonl")
	if info, err := os.Stat(journalPath); err != nil || info.Size() == 0 {
		t.Fatalf("expected HTTP writes to persist canonical journal %s: info=%v err=%v", journalPath, info, err)
	}
}

func TestKnowledgeHTTPAPIRejectsUnknownFields(t *testing.T) {
	vaultPath, database := setupTestVault(t)
	defer database.Close()

	server := NewServer(vaultPath, database)
	server.RegisterRoutes()
	req := httptest.NewRequest(http.MethodPost, "/objects", bytes.NewBufferString(`{"type":"project","title":"AgentVault","unexpected":true}`))
	recorder := httptest.NewRecorder()
	server.mux.ServeHTTP(recorder, req)
	if recorder.Code != http.StatusBadRequest {
		t.Fatalf("expected 400 for unknown field, got %d: %s", recorder.Code, recorder.Body.String())
	}
}

func TestMemoryCandidateHTTPReviewLifecycle(t *testing.T) {
	vaultPath, database := setupTestVault(t)
	defer database.Close()

	server := NewServer(vaultPath, database)
	provenance, err := server.knowledge.CreateProvenance(contract.ProvenanceRecord{
		ID:         "prov_candidate_http",
		SourceType: "agent-session",
		Confidence: 0.92,
	})
	if err != nil {
		t.Fatal(err)
	}
	episode, err := server.knowledge.RecordEpisode(contract.CreateEpisodeRequest{
		ID:           "episode_candidate_http",
		ScopeType:    "project",
		ScopeID:      "agentvault",
		EventType:    "decision.observed",
		Summary:      "Observed reviewable decision",
		ProvenanceID: provenance.ID,
	})
	if err != nil {
		t.Fatal(err)
	}

	server.RegisterRoutes()
	ts := httptest.NewServer(server.mux)
	defer ts.Close()

	request := func(method, path string, body interface{}, target interface{}, wantStatus int) {
		t.Helper()
		var payload []byte
		if body != nil {
			payload, err = json.Marshal(body)
			if err != nil {
				t.Fatal(err)
			}
		}
		req, err := http.NewRequest(method, ts.URL+path, bytes.NewReader(payload))
		if err != nil {
			t.Fatal(err)
		}
		if body != nil {
			req.Header.Set("Content-Type", "application/json")
		}
		resp, err := http.DefaultClient.Do(req)
		if err != nil {
			t.Fatal(err)
		}
		defer resp.Body.Close()
		if resp.StatusCode != wantStatus {
			var failure map[string]interface{}
			_ = json.NewDecoder(resp.Body).Decode(&failure)
			t.Fatalf("%s %s status=%d want=%d body=%v", method, path, resp.StatusCode, wantStatus, failure)
		}
		if target != nil {
			if err := json.NewDecoder(resp.Body).Decode(target); err != nil {
				t.Fatal(err)
			}
		}
	}

	var candidate contract.MemoryCandidate
	request(http.MethodPost, "/memory-candidates", map[string]interface{}{
		"episodeId":  episode.ID,
		"memoryKind": "decision",
		"content":    "Semantic memory requires explicit review.",
		"proposedBy": "extractor",
	}, &candidate, http.StatusCreated)
	if candidate.Status != contract.MemoryCandidatePending || candidate.ProvenanceID != provenance.ID {
		t.Fatalf("unexpected candidate: %+v", candidate)
	}

	var pending []contract.MemoryCandidate
	request(http.MethodGet, "/memory-candidates?status=pending&scopeType=project&scopeId=agentvault", nil, &pending, http.StatusOK)
	if len(pending) != 1 || pending[0].ID != candidate.ID {
		t.Fatalf("unexpected candidate queue: %+v", pending)
	}

	var accepted contract.MemoryCandidate
	request(http.MethodPost, "/memory-candidates/"+candidate.ID+"/accept", map[string]interface{}{
		"reviewedBy": "david",
		"reason":     "Evidence is sufficient.",
	}, &accepted, http.StatusOK)
	if accepted.Status != contract.MemoryCandidateAccepted || accepted.ResultMemoryID == "" {
		t.Fatalf("unexpected accepted candidate: %+v", accepted)
	}

	var loaded contract.MemoryCandidate
	request(http.MethodGet, "/memory-candidates/"+candidate.ID, nil, &loaded, http.StatusOK)
	if loaded.ResultMemoryID != accepted.ResultMemoryID || loaded.ReviewedBy != "david" {
		t.Fatalf("candidate review not persisted: %+v", loaded)
	}
}


func TestMemoryCandidateHTTPDeterministicExtraction(t *testing.T) {
	vaultPath, database := setupTestVault(t)
	defer database.Close()

	server := NewServer(vaultPath, database)
	provenance, err := server.knowledge.CreateProvenance(contract.ProvenanceRecord{
		ID:         "prov_extract_http",
		SourceType: "agent-session",
		Confidence: 0.91,
	})
	if err != nil {
		t.Fatal(err)
	}
	semantic, err := server.knowledge.RecordEpisode(contract.CreateEpisodeRequest{
		ID:           "episode_extract_http",
		ScopeType:    "project",
		ScopeID:      "agentvault",
		EventType:    "decision.observed",
		Summary:      "Keep extraction deterministic before model assistance.",
		ProvenanceID: provenance.ID,
	})
	if err != nil {
		t.Fatal(err)
	}
	raw, err := server.knowledge.RecordEpisode(contract.CreateEpisodeRequest{
		ID:           "episode_raw_http",
		ScopeType:    "project",
		ScopeID:      "agentvault",
		EventType:    "capture.recorded",
		Summary:      "Raw research capture.",
		ProvenanceID: provenance.ID,
	})
	if err != nil {
		t.Fatal(err)
	}

	server.RegisterRoutes()
	ts := httptest.NewServer(server.mux)
	defer ts.Close()

	post := func(episodeID string) []contract.MemoryCandidate {
		t.Helper()
		payload, err := json.Marshal(map[string]interface{}{"episodeId": episodeID})
		if err != nil {
			t.Fatal(err)
		}
		req, err := http.NewRequest(http.MethodPost, ts.URL+"/memory-candidates/extract", bytes.NewReader(payload))
		if err != nil {
			t.Fatal(err)
		}
		req.Header.Set("Content-Type", "application/json")
		resp, err := http.DefaultClient.Do(req)
		if err != nil {
			t.Fatal(err)
		}
		defer resp.Body.Close()
		if resp.StatusCode != http.StatusOK {
			t.Fatalf("extract status=%d", resp.StatusCode)
		}
		var candidates []contract.MemoryCandidate
		if err := json.NewDecoder(resp.Body).Decode(&candidates); err != nil {
			t.Fatal(err)
		}
		return candidates
	}

	candidates := post(semantic.ID)
	if len(candidates) != 1 || candidates[0].MemoryKind != "decision" {
		t.Fatalf("unexpected semantic extraction: %+v", candidates)
	}
	if rawCandidates := post(raw.ID); len(rawCandidates) != 0 {
		t.Fatalf("raw episode extracted candidates: %+v", rawCandidates)
	}
}


func TestNewServerReconcilesMissedSemanticSessionEventPromotion(t *testing.T) {
	vaultPath, database := setupTestVault(t)
	defer database.Close()

	if _, err := database.Exec(`
		INSERT INTO agent_sessions (
			id, agent_id, project, objective, status, context_json, started_at, updated_at
		) VALUES (?, ?, ?, ?, ?, ?, ?, ?)`,
		"session_startup_reconcile",
		"architect",
		"agentvault",
		"Recover semantic enrichment",
		"active",
		"{}",
		"2026-09-30T15:00:00Z",
		"2026-09-30T15:01:00Z",
	); err != nil {
		t.Fatal(err)
	}
	payload, err := json.Marshal(map[string]interface{}{
		"summary": "Startup should repair missed semantic session-event enrichment.",
	})
	if err != nil {
		t.Fatal(err)
	}
	if _, err := database.Exec(`
		INSERT INTO session_events (
			id, session_id, event_type, payload_json, created_at
		) VALUES (?, ?, ?, ?, ?)`,
		"event_startup_reconcile",
		"session_startup_reconcile",
		"decision",
		string(payload),
		"2026-09-30T15:01:00Z",
	); err != nil {
		t.Fatal(err)
	}

	server := NewServer(vaultPath, database)
	if server.knowledgeInitErr != nil {
		t.Fatalf("NewServer knowledge init: %v", server.knowledgeInitErr)
	}
	candidates, err := server.knowledge.ListMemoryCandidates(contract.MemoryCandidateFilter{
		Status:    contract.MemoryCandidatePending,
		ScopeType: "session",
		ScopeID:   "session_startup_reconcile",
		Limit:     20,
	})
	if err != nil {
		t.Fatal(err)
	}
	if len(candidates) != 1 || candidates[0].MemoryKind != "decision" {
		t.Fatalf("startup reconciliation did not backfill candidate: %+v", candidates)
	}
}

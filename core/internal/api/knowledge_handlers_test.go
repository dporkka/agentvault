package api

import (
	"bytes"
	"encoding/json"
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
		"memoryType":   "semantic",
		"scopeType":    "project",
		"scopeId":      "agentvault",
		"content":      "Other products consume AgentVault through stable integration boundaries.",
		"objectId":     object.ID,
		"provenanceId": provenance.ID,
	}, nil, http.StatusCreated)

	var memories []struct {
		ID string `json:"id"`
	}
	request(http.MethodGet, "/memory?scopeType=project&scopeId=agentvault&memoryType=semantic", nil, &memories, http.StatusOK)
	if len(memories) != 1 || memories[0].ID != "mem_http" {
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

package api

import (
	"database/sql"
	"encoding/json"
	"errors"
	"io"
	"net/http"
	"strconv"
	"strings"

	"github.com/agentvault/core/internal/contract"
)

const maxKnowledgeRequestBytes = 2 << 20 // 2 MiB

func (s *Server) handleListObjects(w http.ResponseWriter, r *http.Request) {
	limit, _ := strconv.Atoi(r.URL.Query().Get("limit"))
	filter := contract.KnowledgeObjectFilter{
		Type:         r.URL.Query().Get("type"),
		Organization: r.URL.Query().Get("organization"),
		Project:      r.URL.Query().Get("project"),
		Status:       r.URL.Query().Get("status"),
		Limit:        limit,
	}
	objects, err := s.knowledge.ListObjects(filter)
	if err != nil {
		writeKnowledgeError(w, err)
		return
	}
	writeJSON(w, http.StatusOK, objects)
}

func (s *Server) handleGetObject(w http.ResponseWriter, r *http.Request) {
	object, err := s.knowledge.GetObject(r.PathValue("id"))
	if err != nil {
		writeKnowledgeError(w, err)
		return
	}
	writeJSON(w, http.StatusOK, object)
}

func (s *Server) handleUpsertObject(w http.ResponseWriter, r *http.Request) {
	var req contract.UpsertKnowledgeObjectRequest
	if err := decodeKnowledgeJSON(r, &req); err != nil {
		writeJSON(w, http.StatusBadRequest, map[string]interface{}{"error": err.Error()})
		return
	}
	if pathID := r.PathValue("id"); pathID != "" {
		if req.ID != "" && req.ID != pathID {
			writeJSON(w, http.StatusBadRequest, map[string]interface{}{"error": "body id must match path id"})
			return
		}
		req.ID = pathID
	}
	object, err := s.knowledge.UpsertObject(req)
	if err != nil {
		writeKnowledgeError(w, err)
		return
	}
	status := http.StatusCreated
	if r.Method == http.MethodPut {
		status = http.StatusOK
	}
	writeJSON(w, status, object)
}

func (s *Server) handleCreateRelation(w http.ResponseWriter, r *http.Request) {
	var req contract.CreateObjectRelationRequest
	if err := decodeKnowledgeJSON(r, &req); err != nil {
		writeJSON(w, http.StatusBadRequest, map[string]interface{}{"error": err.Error()})
		return
	}
	relation, err := s.knowledge.CreateRelation(req)
	if err != nil {
		writeKnowledgeError(w, err)
		return
	}
	writeJSON(w, http.StatusCreated, relation)
}

func (s *Server) handleObjectRelations(w http.ResponseWriter, r *http.Request) {
	relations, err := s.knowledge.RelationsForObject(r.PathValue("id"))
	if err != nil {
		writeKnowledgeError(w, err)
		return
	}
	writeJSON(w, http.StatusOK, relations)
}

func (s *Server) handleCreateProvenance(w http.ResponseWriter, r *http.Request) {
	var req contract.ProvenanceRecord
	if err := decodeKnowledgeJSON(r, &req); err != nil {
		writeJSON(w, http.StatusBadRequest, map[string]interface{}{"error": err.Error()})
		return
	}
	record, err := s.knowledge.CreateProvenance(req)
	if err != nil {
		writeKnowledgeError(w, err)
		return
	}
	writeJSON(w, http.StatusCreated, record)
}

func (s *Server) handleGetProvenance(w http.ResponseWriter, r *http.Request) {
	record, err := s.knowledge.GetProvenance(r.PathValue("id"))
	if err != nil {
		writeKnowledgeError(w, err)
		return
	}
	writeJSON(w, http.StatusOK, record)
}

func (s *Server) handleCreateMemory(w http.ResponseWriter, r *http.Request) {
	var req contract.CreateMemoryRequest
	if err := decodeKnowledgeJSON(r, &req); err != nil {
		writeJSON(w, http.StatusBadRequest, map[string]interface{}{"error": err.Error()})
		return
	}
	memory, err := s.knowledge.RecordMemory(req)
	if err != nil {
		writeKnowledgeError(w, err)
		return
	}
	writeJSON(w, http.StatusCreated, memory)
}

func (s *Server) handleListMemories(w http.ResponseWriter, r *http.Request) {
	limit, _ := strconv.Atoi(r.URL.Query().Get("limit"))
	memories, err := s.knowledge.ListMemories(
		r.URL.Query().Get("scopeType"),
		r.URL.Query().Get("scopeId"),
		r.URL.Query().Get("memoryType"),
		limit,
	)
	if err != nil {
		writeKnowledgeError(w, err)
		return
	}
	writeJSON(w, http.StatusOK, memories)
}

func (s *Server) handleStartAgentSession(w http.ResponseWriter, r *http.Request) {
	var req contract.StartAgentSessionRequest
	if err := decodeKnowledgeJSON(r, &req); err != nil {
		writeJSON(w, http.StatusBadRequest, map[string]interface{}{"error": err.Error()})
		return
	}
	session, err := s.knowledge.StartSession(req)
	if err != nil {
		writeKnowledgeError(w, err)
		return
	}
	writeJSON(w, http.StatusCreated, session)
}

func (s *Server) handleGetAgentSession(w http.ResponseWriter, r *http.Request) {
	session, err := s.knowledge.GetSession(r.PathValue("id"))
	if err != nil {
		writeKnowledgeError(w, err)
		return
	}
	writeJSON(w, http.StatusOK, session)
}

func (s *Server) handleAppendSessionEvent(w http.ResponseWriter, r *http.Request) {
	var req contract.AppendSessionEventRequest
	if err := decodeKnowledgeJSON(r, &req); err != nil {
		writeJSON(w, http.StatusBadRequest, map[string]interface{}{"error": err.Error()})
		return
	}
	event, err := s.knowledge.AppendSessionEvent(r.PathValue("id"), req)
	if err != nil {
		writeKnowledgeError(w, err)
		return
	}
	writeJSON(w, http.StatusCreated, event)
}

func (s *Server) handleCloseAgentSession(w http.ResponseWriter, r *http.Request) {
	var req contract.CloseAgentSessionRequest
	if r.Body != nil && r.ContentLength != 0 {
		if err := decodeKnowledgeJSON(r, &req); err != nil {
			writeJSON(w, http.StatusBadRequest, map[string]interface{}{"error": err.Error()})
			return
		}
	}
	session, err := s.knowledge.CloseSession(r.PathValue("id"), req.Status)
	if err != nil {
		writeKnowledgeError(w, err)
		return
	}
	writeJSON(w, http.StatusOK, session)
}

func decodeKnowledgeJSON(r *http.Request, dst interface{}) error {
	decoder := json.NewDecoder(io.LimitReader(r.Body, maxKnowledgeRequestBytes))
	decoder.DisallowUnknownFields()
	if err := decoder.Decode(dst); err != nil {
		if errors.Is(err, io.EOF) {
			return errors.New("request body is required")
		}
		return err
	}
	if decoder.Decode(&struct{}{}) != io.EOF {
		return errors.New("request body must contain one JSON object")
	}
	return nil
}

func writeKnowledgeError(w http.ResponseWriter, err error) {
	status := http.StatusBadRequest
	if errors.Is(err, sql.ErrNoRows) || strings.Contains(strings.ToLower(err.Error()), "not found") {
		status = http.StatusNotFound
	}
	writeJSON(w, status, map[string]interface{}{"error": err.Error()})
}

package api

import (
	"net/http"
	"strconv"

	"github.com/agentvault/core/internal/contract"
)

func (s *Server) handleListMemoryCandidates(w http.ResponseWriter, r *http.Request) {
	limit, _ := strconv.Atoi(r.URL.Query().Get("limit"))
	candidates, err := s.knowledge.ListMemoryCandidates(contract.MemoryCandidateFilter{
		Status:     contract.MemoryCandidateStatus(r.URL.Query().Get("status")),
		ScopeType:  r.URL.Query().Get("scopeType"),
		ScopeID:    r.URL.Query().Get("scopeId"),
		MemoryKind: r.URL.Query().Get("memoryKind"),
		Limit:      limit,
	})
	if err != nil {
		writeKnowledgeError(w, err)
		return
	}
	writeJSON(w, http.StatusOK, candidates)
}

func (s *Server) handleGetMemoryCandidate(w http.ResponseWriter, r *http.Request) {
	candidate, err := s.knowledge.GetMemoryCandidate(r.PathValue("id"))
	if err != nil {
		writeKnowledgeError(w, err)
		return
	}
	writeJSON(w, http.StatusOK, candidate)
}

func (s *Server) handleProposeMemoryCandidate(w http.ResponseWriter, r *http.Request) {
	var req contract.CreateMemoryCandidateRequest
	if err := decodeKnowledgeJSON(r, &req); err != nil {
		writeJSON(w, http.StatusBadRequest, map[string]interface{}{"error": err.Error()})
		return
	}
	candidate, err := s.knowledge.ProposeMemoryCandidate(req)
	if err != nil {
		writeKnowledgeError(w, err)
		return
	}
	writeJSON(w, http.StatusCreated, candidate)
}

func (s *Server) handleAcceptMemoryCandidate(w http.ResponseWriter, r *http.Request) {
	var req contract.ReviewMemoryCandidateRequest
	if err := decodeKnowledgeJSON(r, &req); err != nil {
		writeJSON(w, http.StatusBadRequest, map[string]interface{}{"error": err.Error()})
		return
	}
	candidate, err := s.knowledge.AcceptMemoryCandidate(r.PathValue("id"), req)
	if err != nil {
		writeKnowledgeError(w, err)
		return
	}
	writeJSON(w, http.StatusOK, candidate)
}

func (s *Server) handleRejectMemoryCandidate(w http.ResponseWriter, r *http.Request) {
	var req contract.ReviewMemoryCandidateRequest
	if err := decodeKnowledgeJSON(r, &req); err != nil {
		writeJSON(w, http.StatusBadRequest, map[string]interface{}{"error": err.Error()})
		return
	}
	candidate, err := s.knowledge.RejectMemoryCandidate(r.PathValue("id"), req)
	if err != nil {
		writeKnowledgeError(w, err)
		return
	}
	writeJSON(w, http.StatusOK, candidate)
}

func (s *Server) handleSupersedeMemoryCandidate(w http.ResponseWriter, r *http.Request) {
	var req contract.SupersedeMemoryCandidateRequest
	if err := decodeKnowledgeJSON(r, &req); err != nil {
		writeJSON(w, http.StatusBadRequest, map[string]interface{}{"error": err.Error()})
		return
	}
	candidate, err := s.knowledge.SupersedeMemoryWithCandidate(r.PathValue("id"), req)
	if err != nil {
		writeKnowledgeError(w, err)
		return
	}
	writeJSON(w, http.StatusOK, candidate)
}

func (s *Server) handleMergeMemoryCandidate(w http.ResponseWriter, r *http.Request) {
	var req contract.MergeMemoryCandidateRequest
	if err := decodeKnowledgeJSON(r, &req); err != nil {
		writeJSON(w, http.StatusBadRequest, map[string]interface{}{"error": err.Error()})
		return
	}
	candidate, err := s.knowledge.MergeMemoryCandidate(r.PathValue("id"), req)
	if err != nil {
		writeKnowledgeError(w, err)
		return
	}
	writeJSON(w, http.StatusOK, candidate)
}

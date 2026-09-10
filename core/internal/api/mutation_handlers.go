package api

import (
	"net/http"
	"strconv"
	"strings"

	"github.com/agentvault/core/internal/contract"
)

func (s *Server) handleListMutations(w http.ResponseWriter, r *http.Request) {
	limit, _ := strconv.Atoi(r.URL.Query().Get("limit"))
	proposals, err := s.knowledge.ListMutationProposals(contract.MutationProposalFilter{
		Status:    contract.MutationStatus(r.URL.Query().Get("status")),
		AgentID:   r.URL.Query().Get("agentId"),
		SessionID: r.URL.Query().Get("sessionId"),
		Limit:     limit,
	})
	if err != nil {
		writeMutationError(w, err)
		return
	}
	writeJSON(w, http.StatusOK, proposals)
}

func (s *Server) handleGetMutation(w http.ResponseWriter, r *http.Request) {
	proposal, err := s.knowledge.GetMutationProposal(r.PathValue("id"))
	if err != nil {
		writeMutationError(w, err)
		return
	}
	writeJSON(w, http.StatusOK, proposal)
}

func (s *Server) handleProposeMutation(w http.ResponseWriter, r *http.Request) {
	var req contract.CreateMutationProposalRequest
	if err := decodeKnowledgeJSON(r, &req); err != nil {
		writeJSON(w, http.StatusBadRequest, map[string]interface{}{"error": err.Error()})
		return
	}
	proposal, err := s.mutations.Propose(req)
	if err != nil {
		writeMutationError(w, err)
		return
	}
	writeJSON(w, http.StatusCreated, proposal)
}

func (s *Server) handleApproveMutation(w http.ResponseWriter, r *http.Request) {
	var req contract.ApproveMutationRequest
	if err := decodeKnowledgeJSON(r, &req); err != nil {
		writeJSON(w, http.StatusBadRequest, map[string]interface{}{"error": err.Error()})
		return
	}
	proposal, err := s.mutations.Approve(r.PathValue("id"), req.ApprovedBy)
	if err != nil {
		writeMutationError(w, err)
		return
	}
	writeJSON(w, http.StatusOK, proposal)
}

func (s *Server) handleCommitMutation(w http.ResponseWriter, r *http.Request) {
	result, err := s.mutations.Commit(r.PathValue("id"))
	if err != nil {
		writeMutationError(w, err)
		return
	}
	writeJSON(w, http.StatusOK, result)
}

func (s *Server) handleUndoMutation(w http.ResponseWriter, r *http.Request) {
	result, err := s.mutations.Undo(r.PathValue("id"))
	if err != nil {
		writeMutationError(w, err)
		return
	}
	writeJSON(w, http.StatusOK, result)
}

func (s *Server) handleRejectMutation(w http.ResponseWriter, r *http.Request) {
	var req struct {
		Actor  string `json:"actor"`
		Reason string `json:"reason"`
	}
	if err := decodeKnowledgeJSON(r, &req); err != nil {
		writeJSON(w, http.StatusBadRequest, map[string]interface{}{"error": err.Error()})
		return
	}
	if strings.TrimSpace(req.Actor) == "" {
		writeJSON(w, http.StatusBadRequest, map[string]interface{}{"error": "actor is required"})
		return
	}
	proposal, err := s.knowledge.RejectMutation(r.PathValue("id"), req.Actor, req.Reason)
	if err != nil {
		writeMutationError(w, err)
		return
	}
	writeJSON(w, http.StatusOK, proposal)
}

func writeMutationError(w http.ResponseWriter, err error) {
	message := strings.ToLower(err.Error())
	status := http.StatusBadRequest
	switch {
	case strings.Contains(message, "not found"):
		status = http.StatusNotFound
	case strings.Contains(message, " conflict "),
		strings.Contains(message, "expected approved"),
		strings.Contains(message, "expected committed"),
		strings.Contains(message, "cannot be rejected"),
		strings.Contains(message, "cannot become conflicted"):
		status = http.StatusConflict
	}
	writeJSON(w, status, map[string]interface{}{"error": err.Error()})
}

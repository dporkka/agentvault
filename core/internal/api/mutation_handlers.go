package api

import (
	"database/sql"
	"errors"
	"net/http"
	"strconv"
	"strings"

	"github.com/agentvault/core/internal/authz"
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
	identity, _ := identityFromRequest(r)
	if !identity.Root {
		filtered := make([]contract.MutationProposal, 0, len(proposals))
		for _, proposal := range proposals {
			if err := s.authorizeMutationResource(r, authz.MutationRead, proposal.Path, proposal.SessionID); err == nil {
				filtered = append(filtered, proposal)
			}
		}
		proposals = filtered
	}
	writeJSON(w, http.StatusOK, proposals)
}

func (s *Server) handleGetMutation(w http.ResponseWriter, r *http.Request) {
	proposal, err := s.knowledge.GetMutationProposal(r.PathValue("id"))
	if err != nil {
		writeMutationError(w, err)
		return
	}
	if err := s.authorizeMutationResource(r, authz.MutationRead, proposal.Path, proposal.SessionID); err != nil {
		writeCapabilityError(w, err)
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
	agentID, err := bindProposingAgent(r, req.AgentID)
	if err != nil {
		writeCapabilityError(w, err)
		return
	}
	req.AgentID = agentID
	if err := s.authorizeMutationResource(r, authz.MutationPropose, req.Path, req.SessionID); err != nil {
		writeCapabilityError(w, err)
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
	proposal, err := s.knowledge.GetMutationProposal(r.PathValue("id"))
	if err != nil {
		writeMutationError(w, err)
		return
	}
	if err := s.authorizeMutationResource(r, authz.MutationApprove, proposal.Path, proposal.SessionID); err != nil {
		writeCapabilityError(w, err)
		return
	}
	actor, err := scopedAuditActor(r, req.ApprovedBy)
	if err != nil {
		writeCapabilityError(w, err)
		return
	}
	proposal, err = s.mutations.Approve(proposal.ID, actor)
	if err != nil {
		writeMutationError(w, err)
		return
	}
	writeJSON(w, http.StatusOK, proposal)
}

func (s *Server) handleCommitMutation(w http.ResponseWriter, r *http.Request) {
	proposal, err := s.knowledge.GetMutationProposal(r.PathValue("id"))
	if err != nil {
		writeMutationError(w, err)
		return
	}
	if err := s.authorizeMutationResource(r, authz.MutationCommit, proposal.Path, proposal.SessionID); err != nil {
		writeCapabilityError(w, err)
		return
	}
	result, err := s.mutations.Commit(proposal.ID)
	if err != nil {
		writeMutationError(w, err)
		return
	}
	writeJSON(w, http.StatusOK, result)
}

func (s *Server) handleUndoMutation(w http.ResponseWriter, r *http.Request) {
	proposal, err := s.knowledge.GetMutationProposal(r.PathValue("id"))
	if err != nil {
		writeMutationError(w, err)
		return
	}
	if err := s.authorizeMutationResource(r, authz.MutationUndo, proposal.Path, proposal.SessionID); err != nil {
		writeCapabilityError(w, err)
		return
	}
	result, err := s.mutations.Undo(proposal.ID)
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
	proposal, err := s.knowledge.GetMutationProposal(r.PathValue("id"))
	if err != nil {
		writeMutationError(w, err)
		return
	}
	if err := s.authorizeMutationResource(r, authz.MutationReject, proposal.Path, proposal.SessionID); err != nil {
		writeCapabilityError(w, err)
		return
	}
	actor, err := scopedAuditActor(r, req.Actor)
	if err != nil {
		writeCapabilityError(w, err)
		return
	}
	proposal, err = s.knowledge.RejectMutation(proposal.ID, actor, req.Reason)
	if err != nil {
		writeMutationError(w, err)
		return
	}
	writeJSON(w, http.StatusOK, proposal)
}

func writeMutationError(w http.ResponseWriter, err error) {
	status := http.StatusBadRequest
	if errors.Is(err, sql.ErrNoRows) {
		status = http.StatusNotFound
	} else {
		message := strings.ToLower(err.Error())
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
	}
	writeJSON(w, status, map[string]interface{}{"error": err.Error()})
}

package api

import (
	"net/http"

	"github.com/agentvault/core/internal/authz"
)

func (s *Server) capabilityRegistryReady(w http.ResponseWriter) bool {
	if s.capabilityInitErr == nil && s.capabilityRegistry != nil {
		return true
	}
	detail := "capability registry is unavailable"
	if s.capabilityInitErr != nil {
		detail = s.capabilityInitErr.Error()
	}
	writeJSON(w, http.StatusInternalServerError, map[string]interface{}{"error": "capability registry unavailable", "detail": detail})
	return false
}

func (s *Server) handleListCapabilities(w http.ResponseWriter, r *http.Request) {
	if !s.capabilityRegistryReady(w) {
		return
	}
	writeJSON(w, http.StatusOK, s.capabilityRegistry.List())
}

func (s *Server) handleCreateCapability(w http.ResponseWriter, r *http.Request) {
	if !s.capabilityRegistryReady(w) {
		return
	}
	var req authz.MintRequest
	if err := decodeKnowledgeJSON(r, &req); err != nil {
		writeJSON(w, http.StatusBadRequest, map[string]interface{}{"error": "invalid capability request", "detail": err.Error()})
		return
	}
	issued, err := s.capabilityRegistry.Mint(req)
	if err != nil {
		writeJSON(w, http.StatusBadRequest, map[string]interface{}{"error": "capability issuance failed", "detail": err.Error()})
		return
	}
	writeJSON(w, http.StatusCreated, issued)
}

func (s *Server) handleRevokeCapability(w http.ResponseWriter, r *http.Request) {
	if !s.capabilityRegistryReady(w) {
		return
	}
	principal, err := s.capabilityRegistry.Revoke(r.PathValue("id"))
	if err != nil {
		writeJSON(w, http.StatusNotFound, map[string]interface{}{"error": "capability identity not found", "detail": err.Error()})
		return
	}
	writeJSON(w, http.StatusOK, principal)
}

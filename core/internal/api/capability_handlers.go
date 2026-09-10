package api

import (
	"net/http"

	"github.com/agentvault/core/internal/authz"
)

func (s *Server) handleListCapabilities(w http.ResponseWriter, r *http.Request) {
	registry, err := authz.NewRegistry(s.vaultPath)
	if err != nil {
		writeJSON(w, http.StatusInternalServerError, map[string]interface{}{"error": "capability registry unavailable", "detail": err.Error()})
		return
	}
	writeJSON(w, http.StatusOK, registry.List())
}

func (s *Server) handleCreateCapability(w http.ResponseWriter, r *http.Request) {
	var req authz.MintRequest
	if err := readJSON(r, &req); err != nil {
		writeJSON(w, http.StatusBadRequest, map[string]interface{}{"error": "invalid capability request", "detail": err.Error()})
		return
	}
	registry, err := authz.NewRegistry(s.vaultPath)
	if err != nil {
		writeJSON(w, http.StatusInternalServerError, map[string]interface{}{"error": "capability registry unavailable", "detail": err.Error()})
		return
	}
	issued, err := registry.Mint(req)
	if err != nil {
		writeJSON(w, http.StatusBadRequest, map[string]interface{}{"error": "capability issuance failed", "detail": err.Error()})
		return
	}
	writeJSON(w, http.StatusCreated, issued)
}

func (s *Server) handleRevokeCapability(w http.ResponseWriter, r *http.Request) {
	registry, err := authz.NewRegistry(s.vaultPath)
	if err != nil {
		writeJSON(w, http.StatusInternalServerError, map[string]interface{}{"error": "capability registry unavailable", "detail": err.Error()})
		return
	}
	principal, err := registry.Revoke(r.PathValue("id"))
	if err != nil {
		writeJSON(w, http.StatusNotFound, map[string]interface{}{"error": "capability identity not found", "detail": err.Error()})
		return
	}
	writeJSON(w, http.StatusOK, principal)
}

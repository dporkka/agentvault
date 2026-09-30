package api

import (
	"net/http"

	"github.com/agentvault/core/internal/views"
)

func (s *Server) handleListViews(w http.ResponseWriter, r *http.Request) {
	items, err := views.List(s.vaultPath)
	if err != nil {
		writeJSON(w, http.StatusInternalServerError, map[string]interface{}{"error": "failed to list views", "detail": err.Error()})
		return
	}
	writeJSON(w, http.StatusOK, items)
}

func (s *Server) handleGetView(w http.ResponseWriter, r *http.Request) {
	id := r.PathValue("id")
	view, err := views.Load(s.vaultPath, id)
	if err != nil {
		writeJSON(w, http.StatusNotFound, map[string]interface{}{"error": "view not found", "detail": err.Error()})
		return
	}
	writeJSON(w, http.StatusOK, view)
}

func (s *Server) handleRunView(w http.ResponseWriter, r *http.Request) {
	id := r.PathValue("id")
	view, err := views.Load(s.vaultPath, id)
	if err != nil {
		writeJSON(w, http.StatusNotFound, map[string]interface{}{"error": "view not found", "detail": err.Error()})
		return
	}
	results, err := s.searcher.Search(view.SearchQuery())
	if err != nil {
		writeJSON(w, http.StatusInternalServerError, map[string]interface{}{"error": "view query failed", "detail": err.Error()})
		return
	}
	writeJSON(w, http.StatusOK, results)
}

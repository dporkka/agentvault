package api

import (
	"net/http"

	"github.com/agentvault/core/internal/contextcompiler"
	"github.com/agentvault/core/internal/contract"
	"github.com/agentvault/core/internal/memory"
)

func (s *Server) handleCompileContext(w http.ResponseWriter, r *http.Request) {
	var req contract.CompileContextRequest
	if err := decodeKnowledgeJSON(r, &req); err != nil {
		writeJSON(w, http.StatusBadRequest, map[string]interface{}{"error": err.Error()})
		return
	}
	bundle, err := contextcompiler.CompileUnified(
		contextcompiler.New(s.searcher, s.knowledge),
		memory.NewStore(s.db),
		req,
	)
	if err != nil {
		writeKnowledgeError(w, err)
		return
	}
	writeJSON(w, http.StatusOK, bundle)
}

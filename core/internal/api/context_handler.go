package api

import (
	"net/http"

	"github.com/agentvault/core/internal/contextcompiler"
	"github.com/agentvault/core/internal/contract"
)

func (s *Server) handleCompileContext(w http.ResponseWriter, r *http.Request) {
	var req contract.CompileContextRequest
	if err := decodeKnowledgeJSON(r, &req); err != nil {
		writeJSON(w, http.StatusBadRequest, map[string]interface{}{"error": err.Error()})
		return
	}
	bundle, err := contextcompiler.New(s.searcher, s.knowledge).Compile(req)
	if err != nil {
		writeKnowledgeError(w, err)
		return
	}
	writeJSON(w, http.StatusOK, bundle)
}

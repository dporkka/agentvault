package api

import (
	"net/http"
	"strconv"

	"github.com/agentvault/core/internal/contract"
)

func (s *Server) handleTimeline(w http.ResponseWriter, r *http.Request) {
	limit, _ := strconv.Atoi(r.URL.Query().Get("limit"))
	items, err := s.knowledge.ListTimeline(contract.TimelineFilter{
		Project:   r.URL.Query().Get("project"),
		AgentID:   r.URL.Query().Get("agentId"),
		SessionID: r.URL.Query().Get("sessionId"),
		Kind:      r.URL.Query().Get("kind"),
		Since:     r.URL.Query().Get("since"),
		Until:     r.URL.Query().Get("until"),
		Limit:     limit,
	})
	if err != nil {
		writeKnowledgeError(w, err)
		return
	}
	writeJSON(w, http.StatusOK, items)
}

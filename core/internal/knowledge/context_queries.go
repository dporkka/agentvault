package knowledge

import (
	"encoding/json"
	"fmt"

	"github.com/agentvault/core/internal/contract"
)

// ListSessions returns recent sessions matching optional agent/project filters.
// Event histories are included so callers can compile prior execution context.
func (s *Store) ListSessions(agentID, project, excludeID string, limit int) ([]contract.AgentSession, error) {
	if limit <= 0 || limit > 100 {
		limit = 10
	}

	query := `
		SELECT id, agent_id, COALESCE(project, ''), objective, status,
		       COALESCE(branch, ''), COALESCE(worktree, ''), context_json,
		       started_at, updated_at, COALESCE(ended_at, '')
		FROM agent_sessions WHERE 1=1`
	args := make([]interface{}, 0, 4)
	if agentID != "" {
		query += " AND agent_id = ?"
		args = append(args, agentID)
	}
	if project != "" {
		query += " AND project = ?"
		args = append(args, project)
	}
	if excludeID != "" {
		query += " AND id <> ?"
		args = append(args, excludeID)
	}
	query += " ORDER BY updated_at DESC, id LIMIT ?"
	args = append(args, limit)

	rows, err := s.db.Query(query, args...)
	if err != nil {
		return nil, fmt.Errorf("list sessions: %w", err)
	}

	sessions := make([]contract.AgentSession, 0)
	for rows.Next() {
		var session contract.AgentSession
		var contextJSON string
		if err := rows.Scan(
			&session.ID, &session.AgentID, &session.Project, &session.Objective,
			&session.Status, &session.Branch, &session.Worktree, &contextJSON,
			&session.StartedAt, &session.UpdatedAt, &session.EndedAt,
		); err != nil {
			rows.Close()
			return nil, err
		}
		if err := json.Unmarshal([]byte(contextJSON), &session.Context); err != nil {
			rows.Close()
			return nil, fmt.Errorf("decode session context: %w", err)
		}
		sessions = append(sessions, session)
	}
	if err := rows.Err(); err != nil {
		rows.Close()
		return nil, err
	}
	rows.Close()

	for i := range sessions {
		loaded, err := s.GetSession(sessions[i].ID)
		if err != nil {
			return nil, err
		}
		sessions[i].Events = loaded.Events
	}
	return sessions, nil
}

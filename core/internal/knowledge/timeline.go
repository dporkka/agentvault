package knowledge

import (
	"encoding/json"
	"errors"
	"fmt"
	"sort"
	"strings"
	"time"

	"github.com/agentvault/core/internal/contract"
)

const defaultTimelineLimit = 100

var validTimelineKinds = map[string]struct{}{
	"capture":       {},
	"session_event": {},
	"episode":       {},
	"memory":        {},
	"mutation":      {},
}

// ListTimeline returns one deterministic, newest-first activity stream derived
// from existing canonical sources. Timeline items are projections only: this
// method never creates a second event store.
func (s *Store) ListTimeline(filter contract.TimelineFilter) ([]contract.TimelineItem, error) {
	normalized, err := normalizeTimelineFilter(filter)
	if err != nil {
		return nil, err
	}

	items := make([]contract.TimelineItem, 0, normalized.Limit*2)
	appendKind := func(kind string, load func(contract.TimelineFilter) ([]contract.TimelineItem, error)) error {
		if normalized.Kind != "" && normalized.Kind != kind {
			return nil
		}
		loaded, err := load(normalized)
		if err != nil {
			return err
		}
		items = append(items, loaded...)
		return nil
	}

	if err := appendKind("capture", s.listCaptureTimeline); err != nil {
		return nil, err
	}
	if err := appendKind("session_event", s.listSessionEventTimeline); err != nil {
		return nil, err
	}
	if err := appendKind("episode", s.listEpisodeTimeline); err != nil {
		return nil, err
	}
	if err := appendKind("memory", s.listMemoryTimeline); err != nil {
		return nil, err
	}
	if err := appendKind("mutation", s.listMutationTimeline); err != nil {
		return nil, err
	}

	sort.Slice(items, func(i, j int) bool {
		leftOccurred, leftErr := time.Parse(time.RFC3339Nano, items[i].OccurredAt)
		rightOccurred, rightErr := time.Parse(time.RFC3339Nano, items[j].OccurredAt)
		if leftErr == nil && rightErr == nil && !leftOccurred.Equal(rightOccurred) {
			return leftOccurred.After(rightOccurred)
		}
		if items[i].OccurredAt != items[j].OccurredAt && (leftErr != nil || rightErr != nil) {
			return items[i].OccurredAt > items[j].OccurredAt
		}

		leftCreated, leftCreatedErr := time.Parse(time.RFC3339Nano, items[i].CreatedAt)
		rightCreated, rightCreatedErr := time.Parse(time.RFC3339Nano, items[j].CreatedAt)
		if leftCreatedErr == nil && rightCreatedErr == nil && !leftCreated.Equal(rightCreated) {
			return leftCreated.After(rightCreated)
		}
		if items[i].CreatedAt != items[j].CreatedAt && (leftCreatedErr != nil || rightCreatedErr != nil) {
			return items[i].CreatedAt > items[j].CreatedAt
		}
		if items[i].Kind != items[j].Kind {
			return items[i].Kind < items[j].Kind
		}
		return items[i].ID < items[j].ID
	})
	if len(items) > normalized.Limit {
		items = items[:normalized.Limit]
	}
	return items, nil
}

func normalizeTimelineFilter(filter contract.TimelineFilter) (contract.TimelineFilter, error) {
	filter.Project = strings.TrimSpace(filter.Project)
	filter.AgentID = strings.TrimSpace(filter.AgentID)
	filter.SessionID = strings.TrimSpace(filter.SessionID)
	filter.Kind = strings.TrimSpace(filter.Kind)

	if filter.Kind != "" {
		if _, ok := validTimelineKinds[filter.Kind]; !ok {
			return contract.TimelineFilter{}, fmt.Errorf("invalid timeline kind %q", filter.Kind)
		}
	}

	var since, until time.Time
	var err error
	if strings.TrimSpace(filter.Since) != "" {
		since, err = time.Parse(time.RFC3339Nano, strings.TrimSpace(filter.Since))
		if err != nil {
			return contract.TimelineFilter{}, errors.New("since must be an RFC3339 timestamp")
		}
		filter.Since = since.UTC().Format(time.RFC3339Nano)
	}
	if strings.TrimSpace(filter.Until) != "" {
		until, err = time.Parse(time.RFC3339Nano, strings.TrimSpace(filter.Until))
		if err != nil {
			return contract.TimelineFilter{}, errors.New("until must be an RFC3339 timestamp")
		}
		filter.Until = until.UTC().Format(time.RFC3339Nano)
	}
	if !since.IsZero() && !until.IsZero() && since.After(until) {
		return contract.TimelineFilter{}, errors.New("since must be earlier than or equal to until")
	}

	if filter.Limit <= 0 {
		filter.Limit = defaultTimelineLimit
	}
	if filter.Limit > 500 {
		filter.Limit = 500
	}
	return filter, nil
}

func (s *Store) listCaptureTimeline(filter contract.TimelineFilter) ([]contract.TimelineItem, error) {
	// Captures do not carry durable agent/session identity, so scoped agent or
	// session requests must not accidentally admit them.
	if filter.AgentID != "" || filter.SessionID != "" {
		return []contract.TimelineItem{}, nil
	}

	query := `
		SELECT id, capture_type, COALESCE(title, ''), COALESCE(source_url, ''),
		       COALESCE(project, ''), COALESCE(tags_json, '[]'),
		       COALESCE(raw_payload_json, '{}'), created_at
		FROM captures WHERE 1=1`
	args := make([]interface{}, 0, 4)
	if filter.Project != "" {
		query += " AND project = ?"
		args = append(args, filter.Project)
	}
	query, args = addTimelineWindow(query, args, "created_at", filter)
	query += " ORDER BY created_at DESC, id LIMIT ?"
	args = append(args, filter.Limit)

	rows, err := s.db.Query(query, args...)
	if err != nil {
		return nil, fmt.Errorf("list capture timeline: %w", err)
	}
	defer rows.Close()

	items := make([]contract.TimelineItem, 0)
	for rows.Next() {
		var id, captureType, title, sourceURL, project, tagsJSON, rawJSON, createdAt string
		if err := rows.Scan(&id, &captureType, &title, &sourceURL, &project, &tagsJSON, &rawJSON, &createdAt); err != nil {
			return nil, err
		}
		metadata := make(map[string]interface{})
		_ = json.Unmarshal([]byte(rawJSON), &metadata)
		var tags []string
		if json.Unmarshal([]byte(tagsJSON), &tags) == nil && len(tags) > 0 {
			metadata["tags"] = tags
		}
		if sourceURL != "" {
			metadata["sourceUrl"] = sourceURL
		}
		items = append(items, contract.TimelineItem{
			Kind:       "capture",
			ID:         id,
			Title:      title,
			Summary:    title,
			Project:    project,
			EventType:  captureType,
			OccurredAt: createdAt,
			CreatedAt:  createdAt,
			Metadata:   metadata,
		})
	}
	return items, rows.Err()
}

func (s *Store) listSessionEventTimeline(filter contract.TimelineFilter) ([]contract.TimelineItem, error) {
	query := `
		SELECT e.id, e.session_id, e.event_type, e.payload_json,
		       COALESCE(e.provenance_id, ''), e.created_at,
		       s.agent_id, COALESCE(s.project, '')
		FROM session_events e
		JOIN agent_sessions s ON s.id = e.session_id
		WHERE 1=1`
	args := make([]interface{}, 0, 6)
	if filter.Project != "" {
		query += " AND s.project = ?"
		args = append(args, filter.Project)
	}
	if filter.AgentID != "" {
		query += " AND s.agent_id = ?"
		args = append(args, filter.AgentID)
	}
	if filter.SessionID != "" {
		query += " AND e.session_id = ?"
		args = append(args, filter.SessionID)
	}
	query, args = addTimelineWindow(query, args, "e.created_at", filter)
	query += " ORDER BY e.created_at DESC, e.id LIMIT ?"
	args = append(args, filter.Limit)

	rows, err := s.db.Query(query, args...)
	if err != nil {
		return nil, fmt.Errorf("list session event timeline: %w", err)
	}
	defer rows.Close()

	items := make([]contract.TimelineItem, 0)
	for rows.Next() {
		var id, sessionID, eventType, payloadJSON, provenanceID, createdAt, agentID, project string
		if err := rows.Scan(&id, &sessionID, &eventType, &payloadJSON, &provenanceID, &createdAt, &agentID, &project); err != nil {
			return nil, err
		}
		payload := make(map[string]interface{})
		if err := json.Unmarshal([]byte(payloadJSON), &payload); err != nil {
			return nil, fmt.Errorf("decode timeline session event payload: %w", err)
		}
		summary := eventType
		if value, ok := payload["summary"].(string); ok && strings.TrimSpace(value) != "" {
			summary = value
		}
		items = append(items, contract.TimelineItem{
			Kind:         "session_event",
			ID:           id,
			Summary:      summary,
			Project:      project,
			AgentID:      agentID,
			SessionID:    sessionID,
			ScopeType:    "session",
			ScopeID:      sessionID,
			EventType:    eventType,
			ProvenanceID: provenanceID,
			OccurredAt:   createdAt,
			CreatedAt:    createdAt,
			Metadata:     payload,
		})
	}
	return items, rows.Err()
}

func (s *Store) listEpisodeTimeline(filter contract.TimelineFilter) ([]contract.TimelineItem, error) {
	query := `
		SELECT e.id, e.scope_type, e.scope_id, e.event_type, e.summary,
		       e.object_ids_json, COALESCE(e.provenance_id, ''), e.occurred_at,
		       e.metadata_json, e.created_at,
		       COALESCE(session.agent_id, ''), COALESCE(session.project, '')
		FROM episodes e
		LEFT JOIN agent_sessions session
		  ON e.scope_type = 'session' AND e.scope_id = session.id
		WHERE 1=1`
	args := make([]interface{}, 0, 9)
	if filter.Project != "" {
		query += " AND ((e.scope_type = 'project' AND e.scope_id = ?) OR (e.scope_type = 'session' AND session.project = ?))"
		args = append(args, filter.Project, filter.Project)
	}
	if filter.AgentID != "" {
		query += " AND ((e.scope_type = 'agent' AND e.scope_id = ?) OR (e.scope_type = 'session' AND session.agent_id = ?))"
		args = append(args, filter.AgentID, filter.AgentID)
	}
	if filter.SessionID != "" {
		query += " AND e.scope_type = 'session' AND e.scope_id = ?"
		args = append(args, filter.SessionID)
	}
	query, args = addTimelineWindow(query, args, "e.occurred_at", filter)
	query += " ORDER BY e.occurred_at DESC, e.created_at DESC, e.id LIMIT ?"
	args = append(args, filter.Limit)

	rows, err := s.db.Query(query, args...)
	if err != nil {
		return nil, fmt.Errorf("list episode timeline: %w", err)
	}
	defer rows.Close()

	items := make([]contract.TimelineItem, 0)
	for rows.Next() {
		var id, scopeType, scopeID, eventType, summary, objectIDsJSON, provenanceID, occurredAt, metadataJSON, createdAt, sessionAgentID, sessionProject string
		if err := rows.Scan(
			&id, &scopeType, &scopeID, &eventType, &summary, &objectIDsJSON,
			&provenanceID, &occurredAt, &metadataJSON, &createdAt, &sessionAgentID, &sessionProject,
		); err != nil {
			return nil, err
		}
		var objectIDs []string
		if err := json.Unmarshal([]byte(objectIDsJSON), &objectIDs); err != nil {
			return nil, fmt.Errorf("decode timeline episode object ids: %w", err)
		}
		metadata := make(map[string]interface{})
		if err := json.Unmarshal([]byte(metadataJSON), &metadata); err != nil {
			return nil, fmt.Errorf("decode timeline episode metadata: %w", err)
		}
		project, agentID, sessionID := timelineScope(scopeType, scopeID, sessionProject, sessionAgentID)
		items = append(items, contract.TimelineItem{
			Kind:         "episode",
			ID:           id,
			Summary:      summary,
			Project:      project,
			AgentID:      agentID,
			SessionID:    sessionID,
			ScopeType:    scopeType,
			ScopeID:      scopeID,
			EventType:    eventType,
			ObjectIDs:    objectIDs,
			ProvenanceID: provenanceID,
			OccurredAt:   occurredAt,
			CreatedAt:    createdAt,
			Metadata:     metadata,
		})
	}
	return items, rows.Err()
}

func (s *Store) listMemoryTimeline(filter contract.TimelineFilter) ([]contract.TimelineItem, error) {
	query := `
		SELECT m.id, m.memory_class, COALESCE(m.memory_kind, ''), m.scope_type,
		       m.scope_id, m.content, COALESCE(m.object_id, ''),
		       COALESCE(m.provenance_id, ''), m.metadata_json, m.created_at,
		       COALESCE(session.agent_id, ''), COALESCE(session.project, '')
		FROM memory_records m
		LEFT JOIN agent_sessions session
		  ON m.scope_type = 'session' AND m.scope_id = session.id
		WHERE 1=1`
	args := make([]interface{}, 0, 9)
	if filter.Project != "" {
		query += " AND ((m.scope_type = 'project' AND m.scope_id = ?) OR (m.scope_type = 'session' AND session.project = ?))"
		args = append(args, filter.Project, filter.Project)
	}
	if filter.AgentID != "" {
		query += " AND ((m.scope_type = 'agent' AND m.scope_id = ?) OR (m.scope_type = 'session' AND session.agent_id = ?))"
		args = append(args, filter.AgentID, filter.AgentID)
	}
	if filter.SessionID != "" {
		query += " AND m.scope_type = 'session' AND m.scope_id = ?"
		args = append(args, filter.SessionID)
	}
	query, args = addTimelineWindow(query, args, "m.created_at", filter)
	query += " ORDER BY m.created_at DESC, m.id LIMIT ?"
	args = append(args, filter.Limit)

	rows, err := s.db.Query(query, args...)
	if err != nil {
		return nil, fmt.Errorf("list memory timeline: %w", err)
	}
	defer rows.Close()

	items := make([]contract.TimelineItem, 0)
	for rows.Next() {
		var id, memoryClass, memoryKind, scopeType, scopeID, content, objectID, provenanceID, metadataJSON, createdAt, sessionAgentID, sessionProject string
		if err := rows.Scan(
			&id, &memoryClass, &memoryKind, &scopeType, &scopeID, &content, &objectID,
			&provenanceID, &metadataJSON, &createdAt, &sessionAgentID, &sessionProject,
		); err != nil {
			return nil, err
		}
		metadata := make(map[string]interface{})
		if err := json.Unmarshal([]byte(metadataJSON), &metadata); err != nil {
			return nil, fmt.Errorf("decode timeline memory metadata: %w", err)
		}
		metadata["memoryClass"] = memoryClass
		project, agentID, sessionID := timelineScope(scopeType, scopeID, sessionProject, sessionAgentID)
		objectIDs := []string{}
		if objectID != "" {
			objectIDs = []string{objectID}
		}
		eventType := memoryKind
		if eventType == "" {
			eventType = memoryClass
		}
		items = append(items, contract.TimelineItem{
			Kind:         "memory",
			ID:           id,
			Summary:      content,
			Project:      project,
			AgentID:      agentID,
			SessionID:    sessionID,
			ScopeType:    scopeType,
			ScopeID:      scopeID,
			EventType:    eventType,
			ObjectIDs:    objectIDs,
			ProvenanceID: provenanceID,
			OccurredAt:   createdAt,
			CreatedAt:    createdAt,
			Metadata:     metadata,
		})
	}
	return items, rows.Err()
}

func (s *Store) listMutationTimeline(filter contract.TimelineFilter) ([]contract.TimelineItem, error) {
	query := `
		SELECT m.id, m.mutation_kind, m.path, m.reason,
		       COALESCE(m.agent_id, ''), COALESCE(m.session_id, ''), m.status,
		       m.created_at, m.updated_at, COALESCE(session.project, '')
		FROM mutation_proposals m
		LEFT JOIN agent_sessions session ON session.id = m.session_id
		WHERE 1=1`
	args := make([]interface{}, 0, 7)
	if filter.Project != "" {
		query += " AND session.project = ?"
		args = append(args, filter.Project)
	}
	if filter.AgentID != "" {
		query += " AND m.agent_id = ?"
		args = append(args, filter.AgentID)
	}
	if filter.SessionID != "" {
		query += " AND m.session_id = ?"
		args = append(args, filter.SessionID)
	}
	query, args = addTimelineWindow(query, args, "m.updated_at", filter)
	query += " ORDER BY m.updated_at DESC, m.id LIMIT ?"
	args = append(args, filter.Limit)

	rows, err := s.db.Query(query, args...)
	if err != nil {
		return nil, fmt.Errorf("list mutation timeline: %w", err)
	}
	defer rows.Close()

	items := make([]contract.TimelineItem, 0)
	for rows.Next() {
		var id, mutationKind, path, reason, agentID, sessionID, status, createdAt, updatedAt, project string
		if err := rows.Scan(
			&id, &mutationKind, &path, &reason, &agentID, &sessionID, &status,
			&createdAt, &updatedAt, &project,
		); err != nil {
			return nil, err
		}
		items = append(items, contract.TimelineItem{
			Kind:       "mutation",
			ID:         id,
			Title:      path,
			Summary:    reason,
			Project:    project,
			AgentID:    agentID,
			SessionID:  sessionID,
			ScopeType:  "session",
			ScopeID:    sessionID,
			EventType:  mutationKind,
			OccurredAt: updatedAt,
			CreatedAt:  createdAt,
			Metadata: map[string]interface{}{
				"status": status,
				"path":   path,
			},
		})
	}
	return items, rows.Err()
}

func addTimelineWindow(query string, args []interface{}, column string, filter contract.TimelineFilter) (string, []interface{}) {
	if filter.Since != "" {
		query += " AND julianday(" + column + ") >= julianday(?)"
		args = append(args, filter.Since)
	}
	if filter.Until != "" {
		query += " AND julianday(" + column + ") <= julianday(?)"
		args = append(args, filter.Until)
	}
	return query, args
}

func timelineScope(scopeType, scopeID, sessionProject, sessionAgentID string) (project, agentID, sessionID string) {
	switch scopeType {
	case "project":
		project = scopeID
	case "agent":
		agentID = scopeID
	case "session":
		sessionID = scopeID
		project = sessionProject
		agentID = sessionAgentID
	}
	return project, agentID, sessionID
}

package mcp

import (
	"encoding/json"
	"fmt"

	"github.com/agentvault/core/internal/contract"
	"github.com/agentvault/core/internal/knowledge"
)

// RegisterKnowledgeTools exposes AgentVault's structured knowledge substrate to
// MCP clients. All handlers share one journal-backed store so MCP, HTTP, and
// local clients operate on the same durable object IDs and event history.
func (s *Server) RegisterKnowledgeTools() {
	store := knowledge.New(s.db, s.vaultPath)
	initErr := store.ReplayJournal()
	withStore := func(handler func(*knowledge.Store, map[string]interface{}) (string, error)) func(map[string]interface{}) (string, error) {
		return func(args map[string]interface{}) (string, error) {
			if initErr != nil {
				return "", fmt.Errorf("knowledge projection is unavailable: %w", initErr)
			}
			return handler(store, args)
		}
	}

	s.tools["agentvault.create_provenance"] = Tool{
		Name:        "agentvault.create_provenance",
		Description: "Record append-only provenance for facts, memories, relations, and agent actions before creating the derived knowledge that cites it.",
		InputSchema: makeSchema(map[string]interface{}{
			"id":          schemaString("Optional stable provenance ID"),
			"source_type": schemaString("Source class, for example file, agent-session, email, API, or human"),
			"source_id":   schemaString("Optional source-specific identifier"),
			"agent_id":    schemaString("Optional agent that extracted or inferred the knowledge"),
			"session_id":  schemaString("Optional durable AgentVault session ID"),
			"model":       schemaString("Optional model identifier"),
			"confidence":  schemaNumber("Confidence from 0 to 1", 1),
			"observed_at": schemaString("Optional RFC3339 observation timestamp; defaults to now"),
			"evidence": schemaArray("Supporting evidence records", map[string]interface{}{
				"type": "object",
				"properties": map[string]interface{}{
					"source": schemaString("Evidence source class"),
					"id":     schemaString("Optional source identifier"),
					"path":   schemaString("Optional vault-relative path"),
					"quote":  schemaString("Optional short evidence excerpt"),
				},
			}),
			"metadata": schemaObject("Optional machine-readable provenance metadata"),
		}, []string{"source_type"}),
		Handler: withStore(handleCreateProvenanceTool),
	}

	s.tools["agentvault.upsert_object"] = Tool{
		Name:        "agentvault.upsert_object",
		Description: "Create or update a stable typed knowledge object such as a project, decision, person, repository, task, artifact, agent, or dataset.",
		InputSchema: makeSchema(map[string]interface{}{
			"id":             schemaString("Optional stable object ID; generate one when omitted"),
			"type":           schemaString("Object type, for example project, decision, repository, task, artifact, agent, or dataset"),
			"title":          schemaString("Human-readable object title"),
			"status":         schemaString("Optional lifecycle status"),
			"organization":   schemaString("Optional organization scope"),
			"project":        schemaString("Optional project scope"),
			"canonical_path": schemaString("Optional vault-relative canonical document path"),
			"provenance_id":  schemaString("Optional provenance record supporting this object state"),
			"data":           schemaObject("Type-specific object properties"),
		}, []string{"type", "title"}),
		Handler: withStore(handleUpsertObjectTool),
	}

	s.tools["agentvault.get_object"] = Tool{
		Name:        "agentvault.get_object",
		Description: "Read a typed knowledge object and its incoming/outgoing relations by stable object ID.",
		InputSchema: makeSchema(map[string]interface{}{
			"id": schemaString("Stable knowledge object ID"),
		}, []string{"id"}),
		Handler: withStore(handleGetObjectTool),
	}

	s.tools["agentvault.create_relation"] = Tool{
		Name:        "agentvault.create_relation",
		Description: "Create a typed, optionally temporal relation between two stable knowledge objects.",
		InputSchema: makeSchema(map[string]interface{}{
			"id":             schemaString("Optional stable relation ID"),
			"from_object_id": schemaString("Source object ID"),
			"to_object_id":   schemaString("Target object ID"),
			"relation_type":  schemaString("Typed edge such as affects, depends_on, owns, implements, or supersedes"),
			"valid_from":     schemaString("Optional RFC3339/date validity start"),
			"valid_to":       schemaString("Optional RFC3339/date validity end"),
			"confidence":     schemaNumber("Confidence from 0 to 1", 1),
			"provenance_id":  schemaString("Optional provenance record supporting the relation"),
			"metadata":       schemaObject("Optional relation metadata"),
		}, []string{"from_object_id", "to_object_id", "relation_type"}),
		Handler: withStore(handleCreateRelationTool),
	}

	s.tools["agentvault.record_memory"] = Tool{
		Name:        "agentvault.record_memory",
		Description: "Persist a scoped machine-authored memory with explicit lifecycle class, optional semantic kind, confidence, provenance, temporal validity, and supersession history.",
		InputSchema: makeSchema(map[string]interface{}{
			"id":            schemaString("Optional stable memory ID"),
			"memory_class":  schemaStringEnum("Memory class", []string{"working", "episodic", "semantic", "procedural"}),
			"memory_kind":   schemaStringEnum("Optional semantic kind", []string{"observation", "episode", "fact", "preference", "decision", "procedure", "constraint", "summary"}),
			"scope_type":    schemaString("Scope class such as user, organization, project, agent, or session"),
			"scope_id":      schemaString("Scope identifier"),
			"content":       schemaString("Memory content"),
			"object_id":     schemaString("Optional related knowledge object ID"),
			"provenance_id": schemaString("Optional provenance record"),
			"confidence":    schemaNumber("Confidence from 0 to 1", 1),
			"valid_from":    schemaString("Optional validity start"),
			"valid_to":      schemaString("Optional validity end"),
			"supersedes_id": schemaString("Optional older memory superseded by this one"),
			"metadata":      schemaObject("Optional memory metadata"),
		}, []string{"memory_class", "scope_type", "scope_id", "content"}),
		Handler: withStore(handleRecordMemoryTool),
	}

	s.tools["agentvault.list_memories"] = Tool{
		Name:        "agentvault.list_memories",
		Description: "List durable machine-authored memories for a specific scope, optionally filtered independently by lifecycle class and semantic kind.",
		InputSchema: makeSchema(map[string]interface{}{
			"scope_type":   schemaString("Scope class such as user, organization, project, agent, or session"),
			"scope_id":     schemaString("Scope identifier"),
			"memory_class": schemaStringEnum("Optional memory class", []string{"working", "episodic", "semantic", "procedural"}),
			"memory_kind":  schemaStringEnum("Optional semantic kind", []string{"observation", "episode", "fact", "preference", "decision", "procedure", "constraint", "summary"}),
			"limit":        schemaInt("Maximum memories to return", 100),
		}, []string{"scope_type", "scope_id"}),
		Handler: withStore(handleListMemoriesTool),
	}

	s.tools["agentvault.start_session"] = Tool{
		Name:        "agentvault.start_session",
		Description: "Start a durable agent workspace that records an objective, project, branch/worktree isolation, context snapshot, and append-only event history.",
		InputSchema: makeSchema(map[string]interface{}{
			"id":        schemaString("Optional stable session ID"),
			"agent_id":  schemaString("Stable agent identity"),
			"project":   schemaString("Optional project scope"),
			"objective": schemaString("Objective this agent session must accomplish"),
			"branch":    schemaString("Optional Git branch"),
			"worktree":  schemaString("Optional Git worktree path"),
			"context":   schemaObject("Optional initial context snapshot"),
		}, []string{"agent_id", "objective"}),
		Handler: withStore(handleStartSessionTool),
	}

	s.tools["agentvault.append_session_event"] = Tool{
		Name:        "agentvault.append_session_event",
		Description: "Append an immutable decision, tool result, artifact, verification, failure, or other event to an active durable agent session.",
		InputSchema: makeSchema(map[string]interface{}{
			"session_id":    schemaString("Durable AgentVault session ID"),
			"id":            schemaString("Optional stable event ID"),
			"event_type":    schemaString("Event class such as decision, tool_result, artifact, verification, or failure"),
			"payload":       schemaObject("Machine-readable event payload"),
			"provenance_id": schemaString("Optional provenance record"),
		}, []string{"session_id", "event_type"}),
		Handler: withStore(handleAppendSessionEventTool),
	}

	s.tools["agentvault.get_session"] = Tool{
		Name:        "agentvault.get_session",
		Description: "Read a durable agent session together with its ordered append-only event history.",
		InputSchema: makeSchema(map[string]interface{}{
			"session_id": schemaString("Durable AgentVault session ID"),
		}, []string{"session_id"}),
		Handler: withStore(handleGetSessionTool),
	}

	s.tools["agentvault.close_session"] = Tool{
		Name:        "agentvault.close_session",
		Description: "Close an active durable agent session while preserving its full history. Defaults to completed status.",
		InputSchema: makeSchema(map[string]interface{}{
			"session_id": schemaString("Durable AgentVault session ID"),
			"status":     schemaString("Terminal status such as completed, failed, cancelled, or blocked"),
		}, []string{"session_id"}),
		Handler: withStore(handleCloseSessionTool),
	}
}

func handleCreateProvenanceTool(store *knowledge.Store, args map[string]interface{}) (string, error) {
	confidence, err := optionalConfidence(args, "confidence")
	if err != nil {
		return "", err
	}
	record := contract.ProvenanceRecord{
		ID:         stringArg(args, "id"),
		SourceType: stringArg(args, "source_type"),
		SourceID:   stringArg(args, "source_id"),
		AgentID:    stringArg(args, "agent_id"),
		SessionID:  stringArg(args, "session_id"),
		Model:      stringArg(args, "model"),
		Confidence: 1,
		ObservedAt: stringArg(args, "observed_at"),
		Metadata:   mapArg(args, "metadata"),
	}
	if confidence != nil {
		record.Confidence = *confidence
	}
	if evidenceRaw, ok := args["evidence"]; ok {
		data, err := json.Marshal(evidenceRaw)
		if err != nil {
			return "", fmt.Errorf("encode evidence: %w", err)
		}
		if err := json.Unmarshal(data, &record.Evidence); err != nil {
			return "", fmt.Errorf("evidence must be an array of evidence objects: %w", err)
		}
	}
	created, err := store.CreateProvenance(record)
	if err != nil {
		return "", err
	}
	return prettyJSON(created)
}

func handleUpsertObjectTool(store *knowledge.Store, args map[string]interface{}) (string, error) {
	object, err := store.UpsertObject(contract.UpsertKnowledgeObjectRequest{
		ID:            stringArg(args, "id"),
		Type:          stringArg(args, "type"),
		Title:         stringArg(args, "title"),
		Status:        stringArg(args, "status"),
		Organization:  stringArg(args, "organization"),
		Project:       stringArg(args, "project"),
		CanonicalPath: stringArg(args, "canonical_path"),
		Data:          mapArg(args, "data"),
		ProvenanceID:  stringArg(args, "provenance_id"),
	})
	if err != nil {
		return "", err
	}
	return prettyJSON(object)
}

func handleGetObjectTool(store *knowledge.Store, args map[string]interface{}) (string, error) {
	id := stringArg(args, "id")
	if id == "" {
		return "", fmt.Errorf("id is required")
	}
	object, err := store.GetObject(id)
	if err != nil {
		return "", err
	}
	relations, err := store.RelationsForObject(id)
	if err != nil {
		return "", err
	}
	return prettyJSON(map[string]interface{}{"object": object, "relations": relations})
}

func handleCreateRelationTool(store *knowledge.Store, args map[string]interface{}) (string, error) {
	confidence, err := optionalConfidence(args, "confidence")
	if err != nil {
		return "", err
	}
	relation, err := store.CreateRelation(contract.CreateObjectRelationRequest{
		ID:           stringArg(args, "id"),
		FromObjectID: stringArg(args, "from_object_id"),
		ToObjectID:   stringArg(args, "to_object_id"),
		RelationType: stringArg(args, "relation_type"),
		ValidFrom:    stringArg(args, "valid_from"),
		ValidTo:      stringArg(args, "valid_to"),
		Confidence:   confidence,
		ProvenanceID: stringArg(args, "provenance_id"),
		Metadata:     mapArg(args, "metadata"),
	})
	if err != nil {
		return "", err
	}
	return prettyJSON(relation)
}

func handleRecordMemoryTool(store *knowledge.Store, args map[string]interface{}) (string, error) {
	confidence, err := optionalConfidence(args, "confidence")
	if err != nil {
		return "", err
	}
	memory, err := store.RecordMemory(contract.CreateMemoryRequest{
		ID:           stringArg(args, "id"),
		MemoryClass:  stringArg(args, "memory_class"),
		MemoryKind:   stringArg(args, "memory_kind"),
		ScopeType:    stringArg(args, "scope_type"),
		ScopeID:      stringArg(args, "scope_id"),
		Content:      stringArg(args, "content"),
		ObjectID:     stringArg(args, "object_id"),
		ProvenanceID: stringArg(args, "provenance_id"),
		Confidence:   confidence,
		ValidFrom:    stringArg(args, "valid_from"),
		ValidTo:      stringArg(args, "valid_to"),
		SupersedesID: stringArg(args, "supersedes_id"),
		Metadata:     mapArg(args, "metadata"),
	})
	if err != nil {
		return "", err
	}
	return prettyJSON(memory)
}

func handleListMemoriesTool(store *knowledge.Store, args map[string]interface{}) (string, error) {
	memories, err := store.ListMemoriesFiltered(
		stringArg(args, "scope_type"),
		stringArg(args, "scope_id"),
		stringArg(args, "memory_class"),
		stringArg(args, "memory_kind"),
		intArg(args, "limit", 100),
	)
	if err != nil {
		return "", err
	}
	return prettyJSON(memories)
}

func handleStartSessionTool(store *knowledge.Store, args map[string]interface{}) (string, error) {
	session, err := store.StartSession(contract.StartAgentSessionRequest{
		ID:        stringArg(args, "id"),
		AgentID:   stringArg(args, "agent_id"),
		Project:   stringArg(args, "project"),
		Objective: stringArg(args, "objective"),
		Branch:    stringArg(args, "branch"),
		Worktree:  stringArg(args, "worktree"),
		Context:   mapArg(args, "context"),
	})
	if err != nil {
		return "", err
	}
	return prettyJSON(session)
}

func handleAppendSessionEventTool(store *knowledge.Store, args map[string]interface{}) (string, error) {
	event, err := store.AppendSessionEvent(stringArg(args, "session_id"), contract.AppendSessionEventRequest{
		ID:           stringArg(args, "id"),
		EventType:    stringArg(args, "event_type"),
		Payload:      mapArg(args, "payload"),
		ProvenanceID: stringArg(args, "provenance_id"),
	})
	if err != nil {
		return "", err
	}
	return prettyJSON(event)
}

func handleGetSessionTool(store *knowledge.Store, args map[string]interface{}) (string, error) {
	id := stringArg(args, "session_id")
	if id == "" {
		return "", fmt.Errorf("session_id is required")
	}
	session, err := store.GetSession(id)
	if err != nil {
		return "", err
	}
	return prettyJSON(session)
}

func handleCloseSessionTool(store *knowledge.Store, args map[string]interface{}) (string, error) {
	id := stringArg(args, "session_id")
	if id == "" {
		return "", fmt.Errorf("session_id is required")
	}
	session, err := store.CloseSession(id, stringArg(args, "status"))
	if err != nil {
		return "", err
	}
	return prettyJSON(session)
}

func schemaNumber(desc string, defaultVal float64) map[string]interface{} {
	return map[string]interface{}{
		"type":        "number",
		"description": desc,
		"minimum":     0,
		"maximum":     1,
		"default":     defaultVal,
	}
}

func schemaObject(desc string) map[string]interface{} {
	return map[string]interface{}{
		"type":                 "object",
		"description":          desc,
		"additionalProperties": true,
	}
}

func schemaArray(desc string, items map[string]interface{}) map[string]interface{} {
	return map[string]interface{}{
		"type":        "array",
		"description": desc,
		"items":       items,
	}
}

func mapArg(args map[string]interface{}, key string) map[string]interface{} {
	if value, ok := args[key].(map[string]interface{}); ok && value != nil {
		return value
	}
	return map[string]interface{}{}
}

func optionalConfidence(args map[string]interface{}, key string) (*float64, error) {
	value, ok := args[key]
	if !ok || value == nil {
		return nil, nil
	}
	var parsed float64
	switch v := value.(type) {
	case float64:
		parsed = v
	case float32:
		parsed = float64(v)
	case int:
		parsed = float64(v)
	case int64:
		parsed = float64(v)
	default:
		return nil, fmt.Errorf("%s must be a number between 0 and 1", key)
	}
	if parsed < 0 || parsed > 1 {
		return nil, fmt.Errorf("%s must be between 0 and 1", key)
	}
	return &parsed, nil
}

func prettyJSON(value interface{}) (string, error) {
	data, err := json.MarshalIndent(value, "", "  ")
	if err != nil {
		return "", fmt.Errorf("encode MCP result: %w", err)
	}
	return string(data), nil
}

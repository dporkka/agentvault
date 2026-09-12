package mcp

import (
	"database/sql"
	"errors"
	"fmt"
	"strings"

	"github.com/agentvault/core/internal/authz"
	"github.com/agentvault/core/internal/contract"
	"github.com/agentvault/core/internal/knowledge"
)

// RegisterKnowledgeWriteTools registers only durable machine-authored write
// families explicitly granted to a capability-bound MCP identity. File-backed
// user content is never mutated here; those writes remain transactional mutations.
func (s *Server) RegisterKnowledgeWriteTools() {
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

	principal, ok := s.capabilityIdentity()
	if !ok {
		return
	}

	if authz.HasCapability(principal, authz.KnowledgeWrite) {
		s.tools["agentvault.create_provenance"] = Tool{
			Name:        "agentvault.create_provenance",
			Description: "Record append-only provenance within the identity's authorized project/session scope.",
			InputSchema: makeSchema(map[string]interface{}{
				"id":          schemaString("Optional stable provenance ID; resource-scoped identities must omit it"),
				"source_type": schemaString("Source class such as file, agent-session, email, API, or human"),
				"source_id":   schemaString("Optional source-specific identifier"),
				"session_id":  schemaString("Durable session binding; required for resource-scoped provenance writes"),
				"model":       schemaString("Optional model identifier"),
				"confidence":  schemaNumber("Confidence from 0 to 1", 1),
				"observed_at": schemaString("Optional RFC3339 observation timestamp"),
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
			Handler: withStore(func(store *knowledge.Store, args map[string]interface{}) (string, error) {
				current, err := s.requireWritePrincipal(authz.KnowledgeWrite)
				if err != nil {
					return "", err
				}
				confidence, err := optionalConfidence(args, "confidence")
				if err != nil {
					return "", err
				}
				id := strings.TrimSpace(stringArg(args, "id"))
				if authz.HasResourceScope(current) && id != "" {
					return "", fmt.Errorf("%w: resource-scoped provenance writes must use server-generated IDs", authz.ErrForbidden)
				}
				sessionID := strings.TrimSpace(stringArg(args, "session_id"))
				if authz.HasResourceScope(current) {
					if _, err := authorizeWriteSession(store, current, authz.KnowledgeWrite, sessionID); err != nil {
						return "", err
					}
				}
				record := contract.ProvenanceRecord{
					ID:         id,
					SourceType: stringArg(args, "source_type"),
					SourceID:   stringArg(args, "source_id"),
					AgentID:    current.AgentID,
					SessionID:  sessionID,
					Model:      stringArg(args, "model"),
					Confidence: 1,
					ObservedAt: stringArg(args, "observed_at"),
					Metadata:   mapArg(args, "metadata"),
				}
				if confidence != nil {
					record.Confidence = *confidence
				}
				if evidenceRaw, exists := args["evidence"]; exists {
					if err := decodeToolValue(evidenceRaw, &record.Evidence); err != nil {
						return "", fmt.Errorf("evidence must be an array of evidence objects: %w", err)
					}
				}
				created, err := store.CreateProvenance(record)
				if err != nil {
					return "", err
				}
				return prettyJSON(created)
			}),
		}

		s.tools["agentvault.upsert_object"] = Tool{
			Name:        "agentvault.upsert_object",
			Description: "Create or update an authorized typed knowledge object. Scoped creates use server-generated IDs.",
			InputSchema: makeSchema(map[string]interface{}{
				"id":             schemaString("Existing stable object ID; omit for a resource-scoped create"),
				"type":           schemaString("Object type"),
				"title":          schemaString("Human-readable title"),
				"status":         schemaString("Optional lifecycle status"),
				"organization":   schemaString("Optional organization scope"),
				"project":        schemaString("Project scope"),
				"session_id":     schemaString("Authorization session for a session-scoped identity"),
				"canonical_path": schemaString("Optional canonical path; scoped identities may preserve but not introduce/change paths"),
				"provenance_id":  schemaString("Optional provenance record"),
				"data":           schemaObject("Type-specific object properties"),
			}, []string{"type", "title"}),
			Handler: withStore(func(store *knowledge.Store, args map[string]interface{}) (string, error) {
				current, err := s.requireWritePrincipal(authz.KnowledgeWrite)
				if err != nil {
					return "", err
				}
				req := contract.UpsertKnowledgeObjectRequest{
					ID:            strings.TrimSpace(stringArg(args, "id")),
					Type:          stringArg(args, "type"),
					Title:         stringArg(args, "title"),
					Status:        stringArg(args, "status"),
					Organization:  stringArg(args, "organization"),
					Project:       strings.TrimSpace(stringArg(args, "project")),
					CanonicalPath: strings.TrimSpace(stringArg(args, "canonical_path")),
					ProvenanceID:  strings.TrimSpace(stringArg(args, "provenance_id")),
					Data:          mapArg(args, "data"),
				}
				sessionID := strings.TrimSpace(stringArg(args, "session_id"))
				if authz.HasResourceScope(current) {
					if req.ID != "" {
						existing, err := store.GetObject(req.ID)
						if err != nil {
							return "", scopedWriteLookupError(err, "object")
						}
						if req.Project == "" {
							req.Project = existing.Project
						}
						if _, err := authorizeWriteProject(store, current, authz.KnowledgeWrite, existing.Project, sessionID); err != nil {
							return "", err
						}
						if req.CanonicalPath == "" {
							req.CanonicalPath = existing.CanonicalPath
						} else if req.CanonicalPath != existing.CanonicalPath {
							return "", fmt.Errorf("%w: scoped object writes cannot change canonical paths", authz.ErrForbidden)
						}
					} else if req.CanonicalPath != "" {
						return "", fmt.Errorf("%w: scoped object creates cannot introduce canonical paths", authz.ErrForbidden)
					}
					project, err := authorizeWriteProject(store, current, authz.KnowledgeWrite, req.Project, sessionID)
					if err != nil {
						return "", err
					}
					req.Project = project
					if err := authorizeWriteProvenance(store, current, authz.KnowledgeWrite, req.ProvenanceID, project, sessionID); err != nil {
						return "", err
					}
				}
				object, err := store.UpsertObject(req)
				if err != nil {
					return "", err
				}
				return prettyJSON(object)
			}),
		}

		s.tools["agentvault.create_relation"] = Tool{
			Name:        "agentvault.create_relation",
			Description: "Create a relation only when both endpoint objects are authorized for this identity.",
			InputSchema: makeSchema(map[string]interface{}{
				"id":             schemaString("Optional stable relation ID; resource-scoped identities must omit it"),
				"from_object_id": schemaString("Source object ID"),
				"to_object_id":   schemaString("Target object ID"),
				"relation_type":  schemaString("Typed edge"),
				"session_id":     schemaString("Authorization session for a session-scoped identity"),
				"valid_from":     schemaString("Optional validity start"),
				"valid_to":       schemaString("Optional validity end"),
				"confidence":     schemaNumber("Confidence from 0 to 1", 1),
				"provenance_id":  schemaString("Optional provenance record"),
				"metadata":       schemaObject("Optional relation metadata"),
			}, []string{"from_object_id", "to_object_id", "relation_type"}),
			Handler: withStore(func(store *knowledge.Store, args map[string]interface{}) (string, error) {
				current, err := s.requireWritePrincipal(authz.KnowledgeWrite)
				if err != nil {
					return "", err
				}
				confidence, err := optionalConfidence(args, "confidence")
				if err != nil {
					return "", err
				}
				req := contract.CreateObjectRelationRequest{
					ID:           strings.TrimSpace(stringArg(args, "id")),
					FromObjectID: strings.TrimSpace(stringArg(args, "from_object_id")),
					ToObjectID:   strings.TrimSpace(stringArg(args, "to_object_id")),
					RelationType: stringArg(args, "relation_type"),
					ValidFrom:    stringArg(args, "valid_from"),
					ValidTo:      stringArg(args, "valid_to"),
					Confidence:   confidence,
					ProvenanceID: strings.TrimSpace(stringArg(args, "provenance_id")),
					Metadata:     mapArg(args, "metadata"),
				}
				sessionID := strings.TrimSpace(stringArg(args, "session_id"))
				if authz.HasResourceScope(current) {
					if req.ID != "" {
						return "", fmt.Errorf("%w: resource-scoped relation writes must use server-generated IDs", authz.ErrForbidden)
					}
					fromObject, err := store.GetObject(req.FromObjectID)
					if err != nil {
						return "", scopedWriteLookupError(err, "from object")
					}
					toObject, err := store.GetObject(req.ToObjectID)
					if err != nil {
						return "", scopedWriteLookupError(err, "to object")
					}
					fromProject, err := authorizeWriteProject(store, current, authz.KnowledgeWrite, fromObject.Project, sessionID)
					if err != nil {
						return "", err
					}
					toProject, err := authorizeWriteProject(store, current, authz.KnowledgeWrite, toObject.Project, sessionID)
					if err != nil {
						return "", err
					}
					if fromProject == "" || fromProject != toProject {
						return "", fmt.Errorf("%w: relation endpoints must share one authorized project", authz.ErrForbidden)
					}
					if err := authorizeWriteProvenance(store, current, authz.KnowledgeWrite, req.ProvenanceID, fromProject, sessionID); err != nil {
						return "", err
					}
				}
				relation, err := store.CreateRelation(req)
				if err != nil {
					return "", err
				}
				return prettyJSON(relation)
			}),
		}
	}

	if authz.HasCapability(principal, authz.MemoryWrite) {
		s.tools["agentvault.record_memory"] = Tool{
			Name:        "agentvault.record_memory",
			Description: "Record durable machine memory within the identity's authorized project/session scope.",
			InputSchema: makeSchema(map[string]interface{}{
				"id":            schemaString("Optional stable memory ID; resource-scoped identities must omit it"),
				"memory_class":  schemaStringEnum("Memory class", []string{"working", "episodic", "semantic", "procedural"}),
				"memory_kind":   schemaStringEnum("Optional semantic kind", []string{"observation", "episode", "fact", "preference", "decision", "procedure", "constraint", "summary"}),
				"scope_type":    schemaString("Scope class"),
				"scope_id":      schemaString("Scope identifier"),
				"content":       schemaString("Memory content"),
				"object_id":     schemaString("Optional related object"),
				"provenance_id": schemaString("Optional provenance record"),
				"confidence":    schemaNumber("Confidence from 0 to 1", 1),
				"valid_from":    schemaString("Optional validity start"),
				"valid_to":      schemaString("Optional validity end"),
				"supersedes_id": schemaString("Optional older memory superseded by this one"),
				"metadata":      schemaObject("Optional memory metadata"),
			}, []string{"memory_class", "scope_type", "scope_id", "content"}),
			Handler: withStore(func(store *knowledge.Store, args map[string]interface{}) (string, error) {
				current, err := s.requireWritePrincipal(authz.MemoryWrite)
				if err != nil {
					return "", err
				}
				confidence, err := optionalConfidence(args, "confidence")
				if err != nil {
					return "", err
				}
				req := contract.CreateMemoryRequest{
					ID:           strings.TrimSpace(stringArg(args, "id")),
					MemoryClass:  stringArg(args, "memory_class"),
					MemoryKind:   stringArg(args, "memory_kind"),
					ScopeType:    strings.ToLower(strings.TrimSpace(stringArg(args, "scope_type"))),
					ScopeID:      strings.TrimSpace(stringArg(args, "scope_id")),
					Content:      stringArg(args, "content"),
					ObjectID:     strings.TrimSpace(stringArg(args, "object_id")),
					ProvenanceID: strings.TrimSpace(stringArg(args, "provenance_id")),
					Confidence:   confidence,
					ValidFrom:    stringArg(args, "valid_from"),
					ValidTo:      stringArg(args, "valid_to"),
					SupersedesID: strings.TrimSpace(stringArg(args, "supersedes_id")),
					Metadata:     mapArg(args, "metadata"),
				}
				if authz.HasResourceScope(current) {
					if req.ID != "" {
						return "", fmt.Errorf("%w: resource-scoped memory writes must use server-generated IDs", authz.ErrForbidden)
					}
					project, sessionID, err := authorizeMemoryWriteScope(store, current, req.ScopeType, req.ScopeID)
					if err != nil {
						return "", err
					}
					if req.ObjectID != "" {
						object, err := store.GetObject(req.ObjectID)
						if err != nil {
							return "", scopedWriteLookupError(err, "object")
						}
						if object.Project == "" || object.Project != project {
							return "", fmt.Errorf("%w: memory object is outside the memory scope", authz.ErrForbidden)
						}
					}
					if err := authorizeWriteProvenance(store, current, authz.MemoryWrite, req.ProvenanceID, project, sessionID); err != nil {
						return "", err
					}
					if req.SupersedesID != "" {
						previous, err := store.GetMemory(req.SupersedesID)
						if err != nil {
							return "", scopedWriteLookupError(err, "superseded memory")
						}
						if strings.ToLower(previous.ScopeType) != req.ScopeType || previous.ScopeID != req.ScopeID {
							return "", fmt.Errorf("%w: scoped memory may supersede only within the same memory scope", authz.ErrForbidden)
						}
					}
				}
				memory, err := store.RecordMemory(req)
				if err != nil {
					return "", err
				}
				return prettyJSON(memory)
			}),
		}
	}

	if authz.HasCapability(principal, authz.SessionWrite) {
		s.tools["agentvault.start_session"] = Tool{
			Name:        "agentvault.start_session",
			Description: "Start a durable session in an authorized project; the persisted agent identity is bound to the capability principal.",
			InputSchema: makeSchema(map[string]interface{}{
				"id":        schemaString("Optional stable session ID; resource-scoped identities must omit it"),
				"project":   schemaString("Project scope"),
				"objective": schemaString("Session objective"),
				"branch":    schemaString("Optional Git branch"),
				"worktree":  schemaString("Optional Git worktree path"),
				"context":   schemaObject("Optional initial context snapshot"),
			}, []string{"objective"}),
			Handler: withStore(func(store *knowledge.Store, args map[string]interface{}) (string, error) {
				current, err := s.requireWritePrincipal(authz.SessionWrite)
				if err != nil {
					return "", err
				}
				if len(current.Scope.Sessions) > 0 {
					return "", fmt.Errorf("%w: a session-scoped identity cannot mint a new durable session", authz.ErrForbidden)
				}
				req := contract.StartAgentSessionRequest{
					ID:        strings.TrimSpace(stringArg(args, "id")),
					AgentID:   current.AgentID,
					Project:   strings.TrimSpace(stringArg(args, "project")),
					Objective: stringArg(args, "objective"),
					Branch:    stringArg(args, "branch"),
					Worktree:  stringArg(args, "worktree"),
					Context:   mapArg(args, "context"),
				}
				if authz.HasResourceScope(current) {
					if req.ID != "" {
						return "", fmt.Errorf("%w: resource-scoped session starts must use server-generated IDs", authz.ErrForbidden)
					}
					project, err := authorizeWriteProject(store, current, authz.SessionWrite, req.Project, "")
					if err != nil {
						return "", err
					}
					req.Project = project
				}
				session, err := store.StartSession(req)
				if err != nil {
					return "", err
				}
				return prettyJSON(session)
			}),
		}

		s.tools["agentvault.append_session_event"] = Tool{
			Name:        "agentvault.append_session_event",
			Description: "Append an event only to an authorized active durable session.",
			InputSchema: makeSchema(map[string]interface{}{
				"session_id":    schemaString("Durable session ID"),
				"id":            schemaString("Optional stable event ID; resource-scoped identities must omit it"),
				"event_type":    schemaString("Event type"),
				"payload":       schemaObject("Machine-readable event payload"),
				"provenance_id": schemaString("Optional provenance record from the same authorized session"),
			}, []string{"session_id", "event_type"}),
			Handler: withStore(func(store *knowledge.Store, args map[string]interface{}) (string, error) {
				current, err := s.requireWritePrincipal(authz.SessionWrite)
				if err != nil {
					return "", err
				}
				sessionID := strings.TrimSpace(stringArg(args, "session_id"))
				session, err := authorizeWriteSession(store, current, authz.SessionWrite, sessionID)
				if err != nil {
					return "", err
				}
				req := contract.AppendSessionEventRequest{
					ID:           strings.TrimSpace(stringArg(args, "id")),
					EventType:    stringArg(args, "event_type"),
					Payload:      mapArg(args, "payload"),
					ProvenanceID: strings.TrimSpace(stringArg(args, "provenance_id")),
				}
				if authz.HasResourceScope(current) && req.ID != "" {
					return "", fmt.Errorf("%w: resource-scoped session events must use server-generated IDs", authz.ErrForbidden)
				}
				if req.ProvenanceID != "" && authz.HasResourceScope(current) {
					if err := authorizeWriteProvenance(store, current, authz.SessionWrite, req.ProvenanceID, session.Project, session.ID); err != nil {
						return "", err
					}
				}
				event, err := store.AppendSessionEvent(session.ID, req)
				if err != nil {
					return "", err
				}
				return prettyJSON(event)
			}),
		}

		s.tools["agentvault.close_session"] = Tool{
			Name:        "agentvault.close_session",
			Description: "Close an authorized active durable session while preserving its event history.",
			InputSchema: makeSchema(map[string]interface{}{
				"session_id": schemaString("Durable session ID"),
				"status":     schemaString("Terminal status"),
			}, []string{"session_id"}),
			Handler: withStore(func(store *knowledge.Store, args map[string]interface{}) (string, error) {
				current, err := s.requireWritePrincipal(authz.SessionWrite)
				if err != nil {
					return "", err
				}
				session, err := authorizeWriteSession(store, current, authz.SessionWrite, strings.TrimSpace(stringArg(args, "session_id")))
				if err != nil {
					return "", err
				}
				closed, err := store.CloseSession(session.ID, stringArg(args, "status"))
				if err != nil {
					return "", err
				}
				return prettyJSON(closed)
			}),
		}
	}
}

func (s *Server) requireWritePrincipal(capability authz.Capability) (authz.Principal, error) {
	principal, ok := s.capabilityIdentity()
	if !ok {
		return authz.Principal{}, authz.ErrUnauthenticated
	}
	if !authz.HasCapability(principal, capability) {
		return authz.Principal{}, authz.ErrForbidden
	}
	return principal, nil
}

func authorizeWriteSession(store *knowledge.Store, principal authz.Principal, capability authz.Capability, sessionID string) (contract.AgentSession, error) {
	sessionID = strings.TrimSpace(sessionID)
	if sessionID == "" {
		return contract.AgentSession{}, fmt.Errorf("%w: durable session binding is required", authz.ErrForbidden)
	}
	session, err := store.GetSession(sessionID)
	if err != nil {
		if authz.HasResourceScope(principal) {
			return contract.AgentSession{}, fmt.Errorf("%w: session is outside granted scope", authz.ErrForbidden)
		}
		return contract.AgentSession{}, err
	}
	if err := authz.Authorize(principal, capability, authz.Resource{Project: session.Project, SessionID: session.ID}); err != nil {
		return contract.AgentSession{}, err
	}
	return session, nil
}

func authorizeWriteProject(store *knowledge.Store, principal authz.Principal, capability authz.Capability, project, sessionID string) (string, error) {
	project = strings.TrimSpace(project)
	sessionID = strings.TrimSpace(sessionID)
	if len(principal.Scope.Sessions) > 0 {
		session, err := authorizeWriteSession(store, principal, capability, sessionID)
		if err != nil {
			return "", err
		}
		if project == "" {
			project = session.Project
		}
		if project == "" || project != session.Project {
			return "", fmt.Errorf("%w: project does not match the authorized durable session", authz.ErrForbidden)
		}
		return project, nil
	}
	if project == "" && len(principal.Scope.Projects) == 1 {
		project = principal.Scope.Projects[0]
	}
	if len(principal.Scope.Projects) > 0 {
		if project == "" {
			return "", fmt.Errorf("%w: project is required when multiple projects are granted", authz.ErrForbidden)
		}
		if err := authz.Authorize(principal, capability, authz.Resource{Project: project}); err != nil {
			return "", err
		}
	}
	return project, nil
}

func authorizeWriteProvenance(store *knowledge.Store, principal authz.Principal, capability authz.Capability, provenanceID, project, sessionID string) error {
	provenanceID = strings.TrimSpace(provenanceID)
	if provenanceID == "" || !authz.HasResourceScope(principal) {
		return nil
	}
	record, err := store.GetProvenance(provenanceID)
	if err != nil {
		return scopedWriteLookupError(err, "provenance")
	}
	if record.SessionID == "" {
		return fmt.Errorf("%w: scoped writes may reference only session-bound provenance", authz.ErrForbidden)
	}
	session, err := authorizeWriteSession(store, principal, capability, record.SessionID)
	if err != nil {
		return err
	}
	if strings.TrimSpace(project) != "" && session.Project != strings.TrimSpace(project) {
		return fmt.Errorf("%w: provenance session is outside the write project", authz.ErrForbidden)
	}
	if strings.TrimSpace(sessionID) != "" && len(principal.Scope.Sessions) > 0 && record.SessionID != strings.TrimSpace(sessionID) {
		return fmt.Errorf("%w: provenance belongs to a different durable session", authz.ErrForbidden)
	}
	return nil
}

func authorizeMemoryWriteScope(store *knowledge.Store, principal authz.Principal, scopeType, scopeID string) (project string, sessionID string, err error) {
	scopeType = strings.ToLower(strings.TrimSpace(scopeType))
	scopeID = strings.TrimSpace(scopeID)
	if scopeType == "" || scopeID == "" {
		return "", "", fmt.Errorf("scope_type and scope_id are required")
	}
	if len(principal.Scope.Sessions) > 0 && scopeType != "session" {
		return "", "", fmt.Errorf("%w: session-scoped identities may write only session memories", authz.ErrForbidden)
	}
	switch scopeType {
	case "project":
		if err := authz.Authorize(principal, authz.MemoryWrite, authz.Resource{Project: scopeID}); err != nil {
			return "", "", err
		}
		return scopeID, "", nil
	case "session":
		session, err := authorizeWriteSession(store, principal, authz.MemoryWrite, scopeID)
		if err != nil {
			return "", "", err
		}
		return session.Project, session.ID, nil
	default:
		return "", "", fmt.Errorf("%w: resource-scoped memory writes require project or session scope", authz.ErrForbidden)
	}
}

func scopedWriteLookupError(err error, resource string) error {
	if errors.Is(err, sql.ErrNoRows) {
		return fmt.Errorf("%w: %s is outside granted scope", authz.ErrForbidden, resource)
	}
	return err
}

func decodeToolValue(value interface{}, target interface{}) error {
	data, err := jsonMarshal(value)
	if err != nil {
		return err
	}
	return jsonUnmarshal(data, target)
}

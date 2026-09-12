package mcp

import (
	"fmt"
	"strings"

	"github.com/agentvault/core/internal/authz"
	"github.com/agentvault/core/internal/contract"
	"github.com/agentvault/core/internal/knowledge"
)

// RegisterKnowledgeReadTools exposes only the read side of the structured
// machine-authored knowledge substrate. It intentionally excludes provenance,
// object/relation/memory writes and all session lifecycle mutations.
//
// Bound knowledge:read identities authorize every requested resource against
// persisted object/session scope. Project/session restrictions therefore apply
// to the data itself rather than to caller-provided labels alone.
func (s *Server) RegisterKnowledgeReadTools() {
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
	currentPrincipal := func() (authz.Principal, bool, error) {
		if s.capabilityPrincipal == nil {
			return authz.Principal{}, false, nil
		}
		principal, ok := s.capabilityIdentity()
		if !ok {
			return authz.Principal{}, true, authz.ErrUnauthenticated
		}
		if !authz.HasCapability(principal, authz.KnowledgeRead) {
			return authz.Principal{}, true, authz.ErrForbidden
		}
		return principal, true, nil
	}
	authorizeObject := func(principal authz.Principal, object contract.KnowledgeObject) error {
		return authz.Authorize(principal, authz.KnowledgeRead, authz.Resource{
			Path:    object.CanonicalPath,
			Project: object.Project,
		})
	}

	s.tools["agentvault.get_object"] = Tool{
		Name:        "agentvault.get_object",
		Description: "Read an authorized typed knowledge object and the relations whose endpoints are also visible to this identity.",
		InputSchema: makeSchema(map[string]interface{}{
			"id": schemaString("Stable knowledge object ID"),
		}, []string{"id"}),
		Handler: withStore(func(store *knowledge.Store, args map[string]interface{}) (string, error) {
			id := stringArg(args, "id")
			if id == "" {
				return "", fmt.Errorf("id is required")
			}
			object, err := store.GetObject(id)
			if err != nil {
				return "", err
			}
			principal, bound, err := currentPrincipal()
			if err != nil {
				return "", err
			}
			if bound {
				if err := authorizeObject(principal, object); err != nil {
					return "", err
				}
			}

			relations, err := store.RelationsForObject(id)
			if err != nil {
				return "", err
			}
			if bound && len(principal.Scope.Projects) > 0 {
				filtered := make([]contract.ObjectRelation, 0, len(relations))
				for _, relation := range relations {
					fromObject, fromErr := store.GetObject(relation.FromObjectID)
					toObject, toErr := store.GetObject(relation.ToObjectID)
					if fromErr != nil || toErr != nil {
						continue
					}
					if authorizeObject(principal, fromObject) != nil || authorizeObject(principal, toObject) != nil {
						continue
					}
					filtered = append(filtered, relation)
				}
				relations = filtered
			}
			return prettyJSON(map[string]interface{}{"object": object, "relations": relations})
		}),
	}

	s.tools["agentvault.list_memories"] = Tool{
		Name:        "agentvault.list_memories",
		Description: "List authorized durable machine-authored memories for one explicit project/session/global scope.",
		InputSchema: makeSchema(map[string]interface{}{
			"scope_type":   schemaString("Scope class such as user, organization, project, agent, or session"),
			"scope_id":     schemaString("Scope identifier"),
			"memory_class": schemaStringEnum("Optional memory class", []string{"working", "episodic", "semantic", "procedural"}),
			"memory_kind":  schemaStringEnum("Optional semantic kind", []string{"observation", "episode", "fact", "preference", "decision", "procedure", "constraint", "summary"}),
			"limit":        schemaInt("Maximum memories to return", 100),
		}, []string{"scope_type", "scope_id"}),
		Handler: withStore(func(store *knowledge.Store, args map[string]interface{}) (string, error) {
			scopeType := strings.ToLower(strings.TrimSpace(stringArg(args, "scope_type")))
			scopeID := strings.TrimSpace(stringArg(args, "scope_id"))
			if scopeType == "" || scopeID == "" {
				return "", fmt.Errorf("scope_type and scope_id are required")
			}

			principal, bound, err := currentPrincipal()
			if err != nil {
				return "", err
			}
			if bound {
				resource := authz.Resource{}
				switch scopeType {
				case "project":
					resource.Project = scopeID
				case "session":
					session, err := store.GetSession(scopeID)
					if err != nil {
						return "", err
					}
					resource.Project = session.Project
					resource.SessionID = session.ID
				}
				if err := authz.Authorize(principal, authz.KnowledgeRead, resource); err != nil {
					return "", err
				}
			}

			memories, err := store.ListMemoriesFiltered(
				scopeType,
				scopeID,
				stringArg(args, "memory_class"),
				stringArg(args, "memory_kind"),
				intArg(args, "limit", 100),
			)
			if err != nil {
				return "", err
			}
			return prettyJSON(memories)
		}),
	}

	s.tools["agentvault.get_session"] = Tool{
		Name:        "agentvault.get_session",
		Description: "Read an authorized durable agent session together with its ordered append-only event history.",
		InputSchema: makeSchema(map[string]interface{}{
			"session_id": schemaString("Durable AgentVault session ID"),
		}, []string{"session_id"}),
		Handler: withStore(func(store *knowledge.Store, args map[string]interface{}) (string, error) {
			id := stringArg(args, "session_id")
			if id == "" {
				return "", fmt.Errorf("session_id is required")
			}
			session, err := store.GetSession(id)
			if err != nil {
				return "", err
			}
			principal, bound, err := currentPrincipal()
			if err != nil {
				return "", err
			}
			if bound {
				if err := authz.Authorize(principal, authz.KnowledgeRead, authz.Resource{
					Project:   session.Project,
					SessionID: session.ID,
				}); err != nil {
					return "", err
				}
			}
			return prettyJSON(session)
		}),
	}
}

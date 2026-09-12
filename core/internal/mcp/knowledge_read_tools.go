package mcp

import (
	"fmt"

	"github.com/agentvault/core/internal/knowledge"
)

// RegisterKnowledgeReadTools exposes only the read side of the structured
// machine-authored knowledge substrate. It intentionally excludes provenance,
// object/relation/memory writes and all session lifecycle mutations.
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

	s.tools["agentvault.get_object"] = Tool{
		Name:        "agentvault.get_object",
		Description: "Read a typed knowledge object and its incoming/outgoing relations by stable object ID.",
		InputSchema: makeSchema(map[string]interface{}{
			"id": schemaString("Stable knowledge object ID"),
		}, []string{"id"}),
		Handler: withStore(handleGetObjectTool),
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

	s.tools["agentvault.get_session"] = Tool{
		Name:        "agentvault.get_session",
		Description: "Read a durable agent session together with its ordered append-only event history.",
		InputSchema: makeSchema(map[string]interface{}{
			"session_id": schemaString("Durable AgentVault session ID"),
		}, []string{"session_id"}),
		Handler: withStore(handleGetSessionTool),
	}
}

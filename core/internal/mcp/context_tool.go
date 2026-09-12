package mcp

import (
	"encoding/json"
	"fmt"

	"github.com/agentvault/core/internal/contextcompiler"
	"github.com/agentvault/core/internal/contract"
	"github.com/agentvault/core/internal/knowledge"
	"github.com/agentvault/core/internal/memory"
)

// RegisterContextTool exposes deterministic context compilation to MCP clients.
// When the MCP process is capability-bound, project/session scope is resolved
// from durable AgentVault state before retrieval and every returned evidence item
// is authorized/sanitized before it crosses the MCP boundary.
func (s *Server) RegisterContextTool() {
	store := knowledge.New(s.db, s.vaultPath)
	fileMemories := memory.NewStore(s.db)
	initErr := store.ReplayJournal()
	s.tools["agentvault.compile_context"] = Tool{
		Name:        "agentvault.compile_context",
		Description: "Compile evidence-backed, token-budgeted context for an agent task from authorized Markdown and journal memories, notes, typed objects, temporal relations, and durable session history without making an LLM call.",
		InputSchema: makeSchema(map[string]interface{}{
			"task":         schemaString("Task or objective the context should support"),
			"workspace_id": schemaString("Optional memory workspace scope; defaults to project when omitted"),
			"project":      schemaString("Optional project content scope"),
			"agent_id":     schemaString("Optional stable agent identity"),
			"session_id":   schemaString("Optional current durable AgentVault session ID"),
			"object_ids":   schemaStringArray("Optional stable object IDs that must receive priority"),
			"token_budget": schemaInt("Approximate token budget for compiled context items", 8000),
			"max_items":    schemaInt("Maximum number of context items", 40),
			"as_of":        schemaString("Optional RFC3339 time for temporal memory/relation resolution"),
		}, []string{"task"}),
		Handler: func(args map[string]interface{}) (string, error) {
			if initErr != nil {
				return "", fmt.Errorf("knowledge projection is unavailable: %w", initErr)
			}
			request := contract.CompileContextRequest{
				Task:        stringArg(args, "task"),
				WorkspaceID: stringArg(args, "workspace_id"),
				Project:     stringArg(args, "project"),
				AgentID:     stringArg(args, "agent_id"),
				SessionID:   stringArg(args, "session_id"),
				ObjectIDs:   stringSliceArg(args, "object_ids"),
				TokenBudget: intArg(args, "token_budget", 8000),
				MaxItems:    intArg(args, "max_items", 40),
				AsOf:        stringArg(args, "as_of"),
			}
			request, principal, err := s.prepareContextRequest(store, request)
			if err != nil {
				return "", err
			}
			bundle, err := contextcompiler.CompileUnified(
				contextcompiler.New(s.searcher, store),
				fileMemories,
				request,
			)
			if err != nil {
				return "", err
			}
			bundle = filterContextBundle(store, principal, request, bundle)
			data, err := json.MarshalIndent(bundle, "", "  ")
			if err != nil {
				return "", fmt.Errorf("encode context bundle: %w", err)
			}
			return string(data), nil
		},
	}
}

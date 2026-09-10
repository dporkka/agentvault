package mcp

import (
	"encoding/json"
	"fmt"

	"github.com/agentvault/core/internal/contextcompiler"
	"github.com/agentvault/core/internal/contract"
	"github.com/agentvault/core/internal/knowledge"
)

// RegisterContextTool exposes deterministic context compilation to MCP clients.
func (s *Server) RegisterContextTool() {
	store := knowledge.New(s.db, s.vaultPath)
	initErr := store.ReplayJournal()
	s.tools["agentvault.compile_context"] = Tool{
		Name:        "agentvault.compile_context",
		Description: "Compile evidence-backed, token-budgeted context for an agent task from notes, current memories, typed objects, temporal relations, and durable session history without making an LLM call.",
		InputSchema: makeSchema(map[string]interface{}{
			"task":         schemaString("Task or objective the context should support"),
			"project":      schemaString("Optional project scope"),
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
			bundle, err := contextcompiler.New(s.searcher, store).Compile(contract.CompileContextRequest{
				Task:        stringArg(args, "task"),
				Project:     stringArg(args, "project"),
				AgentID:     stringArg(args, "agent_id"),
				SessionID:   stringArg(args, "session_id"),
				ObjectIDs:   stringSliceArg(args, "object_ids"),
				TokenBudget: intArg(args, "token_budget", 8000),
				MaxItems:    intArg(args, "max_items", 40),
				AsOf:        stringArg(args, "as_of"),
			})
			if err != nil {
				return "", err
			}
			data, err := json.MarshalIndent(bundle, "", "  ")
			if err != nil {
				return "", fmt.Errorf("encode context bundle: %w", err)
			}
			return string(data), nil
		},
	}
}

func stringSliceArg(args map[string]interface{}, key string) []string {
	raw, ok := args[key]
	if !ok || raw == nil {
		return nil
	}
	values, ok := raw.([]interface{})
	if !ok {
		if typed, ok := raw.([]string); ok {
			return typed
		}
		return nil
	}
	result := make([]string, 0, len(values))
	for _, value := range values {
		if text, ok := value.(string); ok && text != "" {
			result = append(result, text)
		}
	}
	return result
}

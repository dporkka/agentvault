package mcp

import (
	"context"
	"encoding/json"
	"fmt"
	"strings"
	"time"

	"github.com/agentvault/core/internal/memory"
)

func (s *Server) registerRecallMemories() {
	s.tools["agentvault.recall_memories"] = Tool{
		Name: "agentvault.recall_memories",
		Description: "Recall classified semantic memories visible in a workspace/agent/session context. " +
			"Applies temporal validity, confidence filters, and contextual supersession before returning evidence-backed records.",
		InputSchema: makeSchema(map[string]interface{}{
			"workspace": schemaString("Workspace scope. Broader global memories are inherited."),
			"agent":     schemaString("Agent scope within the current context."),
			"session":   schemaString("Session/run scope within the current context."),
			"kind":      schemaString("Comma-separated memory kinds: observation, episode, fact, preference, decision, procedure, constraint, summary"),
			"min_confidence": map[string]interface{}{
				"type":        "number",
				"minimum":     0,
				"maximum":     1,
				"description": "Minimum numeric memory confidence from 0 to 1",
			},
			"at": schemaString("Optional RFC3339 point in time for temporal recall; defaults to now"),
			"include_superseded": map[string]interface{}{
				"type":        "boolean",
				"description": "Include memories superseded in the supplied context",
				"default":     false,
			},
			"limit": schemaInt("Maximum number of memory records", 10),
		}, nil),
		Handler: s.handleRecallMemories,
	}
}

func (s *Server) handleRecallMemories(args map[string]interface{}) (string, error) {
	query := memory.Query{
		Context: memory.Scope{
			WorkspaceID: strings.TrimSpace(stringArg(args, "workspace")),
			AgentID:     strings.TrimSpace(stringArg(args, "agent")),
			SessionID:   strings.TrimSpace(stringArg(args, "session")),
		},
		Limit: intArg(args, "limit", 10),
	}
	if _, supplied := args["limit"]; supplied && query.Limit <= 0 {
		return "", fmt.Errorf("limit must be a positive integer")
	}

	if rawKinds := strings.TrimSpace(stringArg(args, "kind")); rawKinds != "" {
		for _, raw := range strings.Split(rawKinds, ",") {
			kind := memory.Kind(strings.TrimSpace(raw))
			if kind == "" {
				continue
			}
			if !kind.Valid() {
				return "", fmt.Errorf("unsupported memory kind %q", kind)
			}
			query.Kinds = append(query.Kinds, kind)
		}
	}

	if raw, supplied := args["min_confidence"]; supplied {
		value, ok := numericArgument(raw)
		if !ok || value < 0 || value > 1 {
			return "", fmt.Errorf("min_confidence must be a number between 0 and 1")
		}
		query.MinConfidence = &value
	}

	if rawAt := strings.TrimSpace(stringArg(args, "at")); rawAt != "" {
		at, err := time.Parse(time.RFC3339, rawAt)
		if err != nil {
			return "", fmt.Errorf("at must be RFC3339")
		}
		query.At = &at
	} else if _, supplied := args["at"]; supplied {
		if _, ok := args["at"].(string); !ok {
			return "", fmt.Errorf("at must be RFC3339")
		}
	}

	if raw, supplied := args["include_superseded"]; supplied {
		value, ok := raw.(bool)
		if !ok {
			return "", fmt.Errorf("include_superseded must be a boolean")
		}
		query.IncludeSuperseded = value
	}

	records, err := memory.NewStore(s.db).Query(context.Background(), query)
	if err != nil {
		return "", fmt.Errorf("recall memories: %w", err)
	}
	if records == nil {
		records = []memory.Record{}
	}

	encoded, err := json.MarshalIndent(records, "", "  ")
	if err != nil {
		return "", fmt.Errorf("encode memory records: %w", err)
	}
	return string(encoded), nil
}

func numericArgument(value interface{}) (float64, bool) {
	switch v := value.(type) {
	case float64:
		return v, true
	case float32:
		return float64(v), true
	case int:
		return float64(v), true
	case int64:
		return float64(v), true
	default:
		return 0, false
	}
}

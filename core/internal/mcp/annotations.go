package mcp

import "encoding/json"

// ToolAnnotations are MCP behavior hints that help clients reason about whether
// a tool is read-only, destructive, idempotent, or may interact with systems
// outside the local vault. They are advisory metadata, not an authorization
// boundary.
type ToolAnnotations struct {
	ReadOnlyHint    bool `json:"readOnlyHint"`
	DestructiveHint bool `json:"destructiveHint"`
	IdempotentHint  bool `json:"idempotentHint"`
	OpenWorldHint   bool `json:"openWorldHint"`
}

// MarshalJSON augments the existing tools/list descriptor with MCP annotations
// without forcing every tool registration to duplicate the same metadata.
func (d toolDescription) MarshalJSON() ([]byte, error) {
	type descriptor struct {
		Name        string                 `json:"name"`
		Description string                 `json:"description"`
		InputSchema map[string]interface{} `json:"inputSchema"`
		Annotations ToolAnnotations        `json:"annotations"`
	}

	return json.Marshal(descriptor{
		Name:        d.Name,
		Description: d.Description,
		InputSchema: d.InputSchema,
		Annotations: annotationsForTool(d.Name),
	})
}

func annotationsForTool(name string) ToolAnnotations {
	switch name {
	case "agentvault.search",
		"agentvault.read_note",
		"agentvault.get_links",
		"agentvault.list_projects",
		"agentvault.list_recent",
		"agentvault.git_status":
		return ToolAnnotations{
			ReadOnlyHint:    true,
			DestructiveHint: false,
			IdempotentHint:  true,
			OpenWorldHint:   false,
		}

	case "agentvault.ask", "agentvault.summarize":
		// These do not mutate the vault, but the configured AI provider may be a
		// remote service and therefore can cross the local trust boundary.
		return ToolAnnotations{
			ReadOnlyHint:    true,
			DestructiveHint: false,
			IdempotentHint:  false,
			OpenWorldHint:   true,
		}

	case "agentvault.set_status", "agentvault.toggle_pin":
		return ToolAnnotations{
			ReadOnlyHint:    false,
			DestructiveHint: false,
			IdempotentHint:  true,
			OpenWorldHint:   false,
		}

	case "agentvault.create_note",
		"agentvault.create_decision",
		"agentvault.create_task",
		"agentvault.capture",
		"agentvault.log_agent_run",
		"agentvault.annotate",
		"agentvault.open_daily":
		return ToolAnnotations{
			ReadOnlyHint:    false,
			DestructiveHint: false,
			IdempotentHint:  false,
			OpenWorldHint:   false,
		}

	default:
		// Unknown tools get conservative hints: clients should not assume they
		// are read-only or safe to repeat.
		return ToolAnnotations{
			ReadOnlyHint:    false,
			DestructiveHint: true,
			IdempotentHint:  false,
			OpenWorldHint:   true,
		}
	}
}

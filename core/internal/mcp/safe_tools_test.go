package mcp

import "testing"

func TestRegisterSafeToolsExcludesLegacyDirectFileWriters(t *testing.T) {
	server, database, _ := setupKnowledgeMCPServer(t)
	defer database.Close()

	// setupKnowledgeMCPServer already registered structured knowledge tools; add
	// the safe legacy read/ask surface exactly as the CLI does by default.
	server.RegisterSafeTools()

	for _, expected := range []string{
		"agentvault.search",
		"agentvault.read_note",
		"agentvault.get_links",
		"agentvault.list_projects",
		"agentvault.list_recent",
		"agentvault.git_status",
		"agentvault.ask",
	} {
		if _, ok := server.tools[expected]; !ok {
			t.Errorf("safe MCP registry missing %s", expected)
		}
	}

	for _, directWriter := range []string{
		"agentvault.create_note",
		"agentvault.create_decision",
		"agentvault.create_task",
		"agentvault.capture",
		"agentvault.open_daily",
		"agentvault.log_agent_run",
		"agentvault.annotate",
		"agentvault.set_status",
		"agentvault.toggle_pin",
	} {
		if _, ok := server.tools[directWriter]; ok {
			t.Errorf("safe MCP registry unexpectedly exposes direct file writer %s", directWriter)
		}
	}
}

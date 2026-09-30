package mcp

import (
	"fmt"
	"strings"

	"github.com/agentvault/core/internal/views"
)

func (s *Server) registerListViews() {
	s.tools["agentvault.list_views"] = Tool{
		Name:        "agentvault.list_views",
		Description: "List portable AgentVault saved views defined in .agentvault/views.",
		InputSchema: makeSchema(map[string]interface{}{}, []string{}),
		Handler:     s.handleListViews,
	}
}

func (s *Server) handleListViews(args map[string]interface{}) (string, error) {
	items, err := views.List(s.vaultPath)
	if err != nil {
		return "", fmt.Errorf("list views: %w", err)
	}
	if len(items) == 0 {
		return "# Saved Views\n\nNo saved views found.", nil
	}
	var sb strings.Builder
	sb.WriteString("# Saved Views\n\n")
	for _, view := range items {
		sb.WriteString(fmt.Sprintf("- **%s** (`%s`)\n", view.Name, view.ID))
	}
	return sb.String(), nil
}

func (s *Server) registerRunView() {
	s.tools["agentvault.run_view"] = Tool{
		Name:        "agentvault.run_view",
		Description: "Run a named portable AgentVault saved view and return its current matching notes.",
		InputSchema: makeSchema(map[string]interface{}{
			"view_id": schemaString("Saved view ID from .agentvault/views, without .yaml"),
		}, []string{"view_id"}),
		Handler: s.handleRunView,
	}
}

func (s *Server) handleRunView(args map[string]interface{}) (string, error) {
	id := stringArg(args, "view_id")
	view, err := views.Load(s.vaultPath, id)
	if err != nil {
		return "", err
	}
	results, err := s.searcher.Search(view.SearchQuery())
	if err != nil {
		return "", fmt.Errorf("run view: %w", err)
	}

	var sb strings.Builder
	sb.WriteString(fmt.Sprintf("# %s\n\n", view.Name))
	if len(results) == 0 {
		sb.WriteString("No matching notes.\n")
		return sb.String(), nil
	}
	for _, result := range results {
		sb.WriteString(fmt.Sprintf("- **%s** — `%s`", result.Title, result.ID))
		if result.Type != "" {
			sb.WriteString(fmt.Sprintf(" [%s]", result.Type))
		}
		if result.Project != "" {
			sb.WriteString(fmt.Sprintf(" (%s)", result.Project))
		}
		sb.WriteString("\n")
	}
	return sb.String(), nil
}

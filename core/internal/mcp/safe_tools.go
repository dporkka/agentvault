package mcp

// RegisterSafeTools registers the legacy MCP tools that do not directly mutate
// user-authored vault files. Structured knowledge/session writes are registered
// separately by RegisterKnowledgeTools; transactional file changes should enter
// through RegisterMutationTools as reviewable proposals.
func (s *Server) RegisterSafeTools() {
	s.registerSearch()
	s.registerReadNote()
	s.registerGetLinks()
	s.registerListProjects()
	s.registerListRecent()
	s.registerGitStatus()
	s.registerAsk()
}

package mcp

// RegisterVaultReadTools registers the read-only legacy vault surface used by
// the explicit vault:read capability family.
func (s *Server) RegisterVaultReadTools() {
	s.registerSearch()
	s.registerRecallMemories()
	s.registerReadNote()
	s.registerGetLinks()
	s.registerListProjects()
	s.registerListRecent()
	s.registerGitStatus()
}

// RegisterAIInvokeTool registers the existing agentvault.ask tool separately so
// model/provider access is never implied by ordinary vault read authority.
func (s *Server) RegisterAIInvokeTool() {
	s.registerAsk()
}

// RegisterSafeTools registers the legacy MCP tools that do not directly mutate
// user-authored vault files. Structured knowledge/session writes are registered
// separately by RegisterKnowledgeTools; transactional file changes should enter
// through RegisterMutationTools as reviewable proposals.
func (s *Server) RegisterSafeTools() {
	s.RegisterVaultReadTools()
	s.RegisterAIInvokeTool()
}

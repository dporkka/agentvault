package mcp

import (
	"fmt"

	"github.com/agentvault/core/internal/authz"
)

// RegisterRuntimeSurface configures the MCP surface for one process identity.
//
// An unbound local stdio process retains AgentVault's compatibility surface:
// safe read/AI tools, structured knowledge/session tools, Context Compiler,
// proposal/read mutations, and resources. Legacy direct file writers are an
// explicit opt-in.
//
// A bound capability identity is intentionally narrower: until AgentVault has
// capability names for search, knowledge, sessions, context, and AI, only the
// mutation tools represented by that identity's granted mutation capabilities
// are registered. This prevents a narrow mutation token from becoming an
// accidental generic vault credential through MCP.
func (s *Server) RegisterRuntimeSurface(allowDirectWrites bool) error {
	if s.capabilityPrincipal != nil {
		if allowDirectWrites {
			return fmt.Errorf("direct-write tools cannot be enabled for a scoped capability identity")
		}
		if _, ok := s.capabilityIdentity(); !ok {
			return authz.ErrUnauthenticated
		}
		s.RegisterMutationTools()
		return nil
	}

	if allowDirectWrites {
		s.RegisterTools()
	} else {
		s.RegisterSafeTools()
	}
	s.RegisterKnowledgeTools()
	s.RegisterContextTool()
	s.RegisterMutationTools()
	s.RegisterResources()
	return nil
}

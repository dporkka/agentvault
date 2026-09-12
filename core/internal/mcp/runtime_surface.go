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
// Bound capability identities receive only explicitly granted tool families.
// Mutation capabilities may retain path/project/session restrictions because
// their handlers enforce those resources. The first non-mutation capability
// families are intentionally global-only; if a principal combines one with any
// resource scope, registration fails closed rather than silently ignoring it.
func (s *Server) RegisterRuntimeSurface(allowDirectWrites bool) error {
	if s.capabilityPrincipal != nil {
		if allowDirectWrites {
			return fmt.Errorf("direct-write tools cannot be enabled for a scoped capability identity")
		}
		principal, ok := s.capabilityIdentity()
		if !ok {
			return authz.ErrUnauthenticated
		}

		hasBroadReadCapability := authz.HasAnyCapability(
			principal,
			authz.VaultRead,
			authz.KnowledgeRead,
			authz.ContextCompile,
			authz.AIInvoke,
		)
		if hasBroadReadCapability && authz.HasResourceScope(principal) {
			return fmt.Errorf("non-mutation MCP capabilities do not yet support path/project/session scope")
		}

		if authz.HasCapability(principal, authz.VaultRead) {
			s.RegisterVaultReadTools()
			s.RegisterResources()
		}
		if authz.HasCapability(principal, authz.KnowledgeRead) {
			s.RegisterKnowledgeReadTools()
		}
		if authz.HasCapability(principal, authz.ContextCompile) {
			s.RegisterContextTool()
		}
		if authz.HasCapability(principal, authz.AIInvoke) {
			s.RegisterAIInvokeTool()
		}

		// RegisterMutationTools performs its own capability and resource checks;
		// on a read-only principal it registers no mutation tools.
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

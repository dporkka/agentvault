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
// Structured knowledge/context families support project/session scope but not
// path-prefix scope. Scoped AI invocation additionally requires context:compile
// so provider execution cannot acquire vault retrieval authority implicitly.
func (s *Server) RegisterRuntimeSurface(allowDirectWrites bool) error {
	if s.capabilityPrincipal != nil {
		if allowDirectWrites {
			return fmt.Errorf("direct-write tools cannot be enabled for a scoped capability identity")
		}
		principal, ok := s.capabilityIdentity()
		if !ok {
			return authz.ErrUnauthenticated
		}

		if authz.HasCapability(principal, authz.VaultRead) && authz.HasResourceScope(principal) {
			return fmt.Errorf("vault:read does not yet support path/project/session scope")
		}
		if authz.HasAnyCapability(
			principal,
			authz.KnowledgeRead,
			authz.ContextCompile,
			authz.KnowledgeWrite,
			authz.MemoryWrite,
			authz.SessionWrite,
			authz.AIInvoke,
		) && len(principal.Scope.PathPrefixes) > 0 {
			return fmt.Errorf("structured knowledge/context/AI capabilities do not yet support path-prefix scope")
		}
		if authz.HasCapability(principal, authz.AIInvoke) && authz.HasResourceScope(principal) && !authz.HasCapability(principal, authz.ContextCompile) {
			return fmt.Errorf("scoped ai:invoke requires context:compile")
		}

		if authz.HasCapability(principal, authz.VaultRead) {
			s.RegisterVaultReadTools()
			s.RegisterResources()
		}
		if authz.HasCapability(principal, authz.KnowledgeRead) {
			s.RegisterKnowledgeReadTools()
		}
		if authz.HasAnyCapability(principal, authz.KnowledgeWrite, authz.MemoryWrite, authz.SessionWrite) {
			s.RegisterKnowledgeWriteTools()
			s.HardenKnowledgeWriteTools()
		}
		if authz.HasCapability(principal, authz.ContextCompile) {
			s.RegisterContextTool()
		}
		if authz.HasCapability(principal, authz.AIInvoke) {
			if authz.HasResourceScope(principal) {
				s.RegisterScopedAIInvokeTool()
			} else {
				s.RegisterAIInvokeTool()
			}
		}

		if authz.HasAnyCapability(
			principal,
			authz.MutationRead,
			authz.MutationPropose,
			authz.MutationApprove,
			authz.MutationCommit,
			authz.MutationUndo,
			authz.MutationReject,
		) {
			// Mutation registration performs crash recovery, so do it only for a
			// principal that actually carries mutation authority. A principal with
			// only knowledge/context/AI authority must not reconcile file mutation state.
			s.RegisterMutationTools()
		}
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

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
// Mutation capabilities enforce path/project/session restrictions against
// persisted proposal/session resources. knowledge:read and context:compile can
// enforce project/session restrictions, but path-prefix scope remains unsupported
// for those structured families. vault:read and ai:invoke remain global-only.
func (s *Server) RegisterRuntimeSurface(allowDirectWrites bool) error {
	if s.capabilityPrincipal != nil {
		if allowDirectWrites {
			return fmt.Errorf("direct-write tools cannot be enabled for a scoped capability identity")
		}
		principal, ok := s.capabilityIdentity()
		if !ok {
			return authz.ErrUnauthenticated
		}

		if authz.HasAnyCapability(principal, authz.VaultRead, authz.AIInvoke) && authz.HasResourceScope(principal) {
			return fmt.Errorf("vault:read and ai:invoke do not yet support path/project/session scope")
		}
		if authz.HasAnyCapability(principal, authz.KnowledgeRead, authz.ContextCompile) && len(principal.Scope.PathPrefixes) > 0 {
			return fmt.Errorf("knowledge:read and context:compile do not yet support path-prefix scope")
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
			// principal that actually carries mutation authority. A pure read-side
			// identity must not reconcile mutation lifecycle state as a startup side effect.
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

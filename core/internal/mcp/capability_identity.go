package mcp

import (
	"fmt"
	"sync"

	"github.com/agentvault/core/internal/authz"
)

var scopedMCPPrincipals sync.Map // *Server -> authz.Principal

// SetCapabilityToken binds this MCP server process to one persistent capability
// identity. Stdio uses the fixed identity directly; HTTP callers must also send
// the same token as transport authentication when configured by the CLI.
func (s *Server) SetCapabilityToken(token string) error {
	registry, err := authz.NewRegistry(s.vaultPath)
	if err != nil {
		return fmt.Errorf("load capability registry: %w", err)
	}
	principal, err := registry.Authenticate(token)
	if err != nil {
		return err
	}
	scopedMCPPrincipals.Store(s, principal)
	return nil
}

func (s *Server) capabilityPrincipal() (authz.Principal, bool) {
	principal, ok := scopedMCPPrincipals.Load(s)
	if !ok {
		return authz.Principal{}, false
	}
	return principal.(authz.Principal), true
}

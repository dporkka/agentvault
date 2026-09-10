package mcp

import (
	"fmt"

	"github.com/agentvault/core/internal/authz"
)

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
	s.capabilityPrincipal = &principal
	return nil
}

func (s *Server) capabilityIdentity() (authz.Principal, bool) {
	if s.capabilityPrincipal == nil {
		return authz.Principal{}, false
	}
	return *s.capabilityPrincipal, true
}

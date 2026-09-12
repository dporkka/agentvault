package mcp

import (
	"fmt"

	"github.com/agentvault/core/internal/authz"
)

// SetCapabilityToken binds this MCP server process to one persistent capability
// identity. The raw token remains only in process memory and is revalidated
// against the persisted registry on each MCP dispatch so expiry/revocation takes
// effect without restarting the server.
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
	// A bound capability token is also the HTTP transport credential. This is
	// harmless for stdio and prevents separate identity/transport secrets from
	// drifting when HTTP transport is used.
	s.authToken = token
	return nil
}

func (s *Server) capabilityIdentity() (authz.Principal, bool) {
	if s.capabilityPrincipal == nil || s.authToken == "" {
		return authz.Principal{}, false
	}
	registry, err := authz.NewRegistry(s.vaultPath)
	if err != nil {
		return authz.Principal{}, false
	}
	principal, err := registry.Authenticate(s.authToken)
	if err != nil {
		return authz.Principal{}, false
	}
	return principal, true
}

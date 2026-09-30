package authz

// MCP capability families are explicit rather than implied by one generic
// AgentVault credential. Read, machine-knowledge writes, session lifecycle,
// model invocation, and transactional file mutation remain independently grantable.
const (
	VaultRead      Capability = "vault:read"
	KnowledgeRead  Capability = "knowledge:read"
	KnowledgeWrite Capability = "knowledge:write"
	MemoryWrite    Capability = "memory:write"
	SessionWrite   Capability = "session:write"
	ContextCompile Capability = "context:compile"
	AIInvoke       Capability = "ai:invoke"
)

func init() {
	validCapabilities[VaultRead] = struct{}{}
	validCapabilities[KnowledgeRead] = struct{}{}
	validCapabilities[KnowledgeWrite] = struct{}{}
	validCapabilities[MemoryWrite] = struct{}{}
	validCapabilities[SessionWrite] = struct{}{}
	validCapabilities[ContextCompile] = struct{}{}
	validCapabilities[AIInvoke] = struct{}{}
}

// HasResourceScope reports whether a principal carries path/project/session
// restrictions. Individual capability families decide which scope dimensions
// they can enforce; unsupported combinations fail closed at runtime registration
// or at the concrete resource operation.
func HasResourceScope(principal Principal) bool {
	return len(principal.Scope.PathPrefixes) > 0 || len(principal.Scope.Projects) > 0 || len(principal.Scope.Sessions) > 0
}

// HasAnyCapability is a small helper for capability-family registration.
func HasAnyCapability(principal Principal, capabilities ...Capability) bool {
	for _, capability := range capabilities {
		if HasCapability(principal, capability) {
			return true
		}
	}
	return false
}

package authz

// Non-mutation MCP capability families deliberately start read-side only.
// Machine-authored knowledge/session writes need resource-aware scope checks
// before they are safe to grant to persistent agent identities.
const (
	VaultRead      Capability = "vault:read"
	KnowledgeRead  Capability = "knowledge:read"
	ContextCompile Capability = "context:compile"
	AIInvoke       Capability = "ai:invoke"
)

func init() {
	validCapabilities[VaultRead] = struct{}{}
	validCapabilities[KnowledgeRead] = struct{}{}
	validCapabilities[ContextCompile] = struct{}{}
	validCapabilities[AIInvoke] = struct{}{}
}

// HasResourceScope reports whether a principal carries path/project/session
// restrictions. The first non-mutation MCP capability slice is intentionally
// global-only: if any of these restrictions are present, runtime registration
// fails closed rather than silently pretending those broad read tools can honor
// a scope they do not yet filter precisely.
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

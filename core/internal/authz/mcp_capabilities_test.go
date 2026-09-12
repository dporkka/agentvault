package authz

import "testing"

func TestRegistryMintsMCPCapabilityFamilies(t *testing.T) {
	registry, err := NewRegistry(t.TempDir())
	if err != nil {
		t.Fatal(err)
	}
	issued, err := registry.Mint(MintRequest{
		AgentID: "agent",
		Capabilities: []Capability{
			VaultRead,
			KnowledgeRead,
			KnowledgeWrite,
			MemoryWrite,
			SessionWrite,
			ContextCompile,
			AIInvoke,
		},
	})
	if err != nil {
		t.Fatal(err)
	}
	principal, err := registry.Authenticate(issued.Token)
	if err != nil {
		t.Fatal(err)
	}
	for _, capability := range []Capability{
		VaultRead,
		KnowledgeRead,
		KnowledgeWrite,
		MemoryWrite,
		SessionWrite,
		ContextCompile,
		AIInvoke,
	} {
		if !HasCapability(principal, capability) {
			t.Errorf("minted principal missing %s", capability)
		}
	}
	if HasResourceScope(principal) {
		t.Fatal("unscoped principal unexpectedly reports resource scope")
	}
	if !HasAnyCapability(principal, MutationCommit, MemoryWrite) {
		t.Fatal("HasAnyCapability should detect granted durable-write capability")
	}
}

func TestHasResourceScopeRecognizesEveryScopeDimension(t *testing.T) {
	cases := []struct {
		name  string
		scope Scope
	}{
		{name: "path", scope: Scope{PathPrefixes: []string{"10-notes"}}},
		{name: "project", scope: Scope{Projects: []string{"alpha"}}},
		{name: "session", scope: Scope{Sessions: []string{"session-1"}}},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			if !HasResourceScope(Principal{Scope: tc.scope}) {
				t.Fatalf("expected %s scope to be detected", tc.name)
			}
		})
	}
}

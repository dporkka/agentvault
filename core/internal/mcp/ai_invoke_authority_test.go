package mcp

import (
	"strings"
	"testing"

	"github.com/agentvault/core/internal/authz"
)

func TestCapabilityBoundAIInvokeNeverImpliesContextAuthority(t *testing.T) {
	_, database, vault := setupKnowledgeMCPServer(t)
	defer database.Close()
	registry, err := authz.NewRegistry(vault)
	if err != nil {
		t.Fatal(err)
	}
	issued, err := registry.Mint(authz.MintRequest{
		AgentID: "model-only",
		Capabilities: []authz.Capability{authz.AIInvoke},
	})
	if err != nil {
		t.Fatal(err)
	}
	server := NewServer(vault, database)
	if err := server.SetCapabilityToken(issued.Token); err != nil {
		t.Fatal(err)
	}
	if err := server.RegisterRuntimeSurface(false); err == nil || !strings.Contains(err.Error(), "requires context:compile") {
		t.Fatalf("ai:invoke without context authority should fail registration, got %v", err)
	}
	if len(server.tools) != 0 || len(server.resources) != 0 {
		t.Fatalf("failed AI registration leaked tools=%d resources=%d", len(server.tools), len(server.resources))
	}
}

package mcp

import (
	"testing"

	"github.com/agentvault/core/internal/authz"
)

func TestRegisterRuntimeSurfaceUnboundKeepsSafeLocalTools(t *testing.T) {
	_, database, vault := setupKnowledgeMCPServer(t)
	defer database.Close()

	server := NewServer(vault, database)
	if err := server.RegisterRuntimeSurface(false); err != nil {
		t.Fatal(err)
	}
	for _, expected := range []string{
		"agentvault.search",
		"agentvault.recall_memories",
		"agentvault.upsert_object",
		"agentvault.compile_context",
		"agentvault.propose_mutation",
	} {
		if _, ok := server.tools[expected]; !ok {
			t.Errorf("unbound safe surface missing %s", expected)
		}
	}
	if _, ok := server.tools["agentvault.create_note"]; ok {
		t.Fatal("unbound safe surface must not expose legacy direct file writers")
	}
	if len(server.resources) == 0 {
		t.Fatal("unbound local surface should retain read resources")
	}
}

func TestRegisterRuntimeSurfaceScopedRegistersOnlyGrantedMutationTools(t *testing.T) {
	_, database, vault := setupKnowledgeMCPServer(t)
	defer database.Close()

	registry, err := authz.NewRegistry(vault)
	if err != nil {
		t.Fatal(err)
	}
	issued, err := registry.Mint(authz.MintRequest{
		AgentID: "reviewer",
		Capabilities: []authz.Capability{
			authz.MutationRead,
			authz.MutationApprove,
		},
	})
	if err != nil {
		t.Fatal(err)
	}

	server := NewServer(vault, database)
	if err := server.SetCapabilityToken(issued.Token); err != nil {
		t.Fatal(err)
	}
	if err := server.RegisterRuntimeSurface(false); err != nil {
		t.Fatal(err)
	}

	for _, expected := range []string{
		"agentvault.get_mutation",
		"agentvault.list_mutations",
		"agentvault.approve_mutation",
	} {
		if _, ok := server.tools[expected]; !ok {
			t.Errorf("scoped surface missing granted tool %s", expected)
		}
	}
	for _, forbidden := range []string{
		"agentvault.propose_mutation",
		"agentvault.commit_mutation",
		"agentvault.search",
		"agentvault.recall_memories",
		"agentvault.upsert_object",
		"agentvault.start_session",
		"agentvault.compile_context",
		"agentvault.create_note",
	} {
		if _, ok := server.tools[forbidden]; ok {
			t.Errorf("scoped surface unexpectedly exposes %s", forbidden)
		}
	}
	if len(server.resources) != 0 {
		t.Fatalf("scoped surface unexpectedly exposes %d resources", len(server.resources))
	}
	if err := server.RegisterRuntimeSurface(true); err == nil {
		t.Fatal("scoped identity must reject direct-write compatibility mode")
	}
}

func TestRegisterRuntimeSurfaceDirectWriteCompatibilityIsExplicit(t *testing.T) {
	_, database, vault := setupKnowledgeMCPServer(t)
	defer database.Close()

	server := NewServer(vault, database)
	if err := server.RegisterRuntimeSurface(true); err != nil {
		t.Fatal(err)
	}
	if _, ok := server.tools["agentvault.create_note"]; !ok {
		t.Fatal("explicit direct-write compatibility mode should register create_note")
	}
	if _, ok := server.tools["agentvault.recall_memories"]; !ok {
		t.Fatal("direct-write compatibility mode must retain Markdown memory recall")
	}
}

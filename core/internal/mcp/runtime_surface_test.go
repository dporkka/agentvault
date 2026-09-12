package mcp

import (
	"strings"
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
		Scope: authz.Scope{Projects: []string{"alpha"}},
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

func TestRegisterRuntimeSurfaceExplicitReadFamilies(t *testing.T) {
	_, database, vault := setupKnowledgeMCPServer(t)
	defer database.Close()

	registry, err := authz.NewRegistry(vault)
	if err != nil {
		t.Fatal(err)
	}
	issued, err := registry.Mint(authz.MintRequest{
		AgentID: "reader",
		Capabilities: []authz.Capability{
			authz.VaultRead,
			authz.KnowledgeRead,
			authz.ContextCompile,
			authz.AIInvoke,
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
		"agentvault.search",
		"agentvault.recall_memories",
		"agentvault.read_note",
		"agentvault.get_links",
		"agentvault.list_projects",
		"agentvault.list_recent",
		"agentvault.git_status",
		"agentvault.get_object",
		"agentvault.list_memories",
		"agentvault.get_session",
		"agentvault.compile_context",
		"agentvault.ask",
	} {
		if _, ok := server.tools[expected]; !ok {
			t.Errorf("read-capability surface missing %s", expected)
		}
	}
	for _, forbidden := range []string{
		"agentvault.create_note",
		"agentvault.create_provenance",
		"agentvault.upsert_object",
		"agentvault.create_relation",
		"agentvault.record_memory",
		"agentvault.start_session",
		"agentvault.append_session_event",
		"agentvault.close_session",
		"agentvault.propose_mutation",
		"agentvault.approve_mutation",
		"agentvault.commit_mutation",
	} {
		if _, ok := server.tools[forbidden]; ok {
			t.Errorf("read-capability surface unexpectedly exposes %s", forbidden)
		}
	}
	if len(server.resources) == 0 {
		t.Fatal("vault:read should expose read-only MCP resources")
	}
}

func TestRegisterRuntimeSurfaceGlobalOnlyFamiliesFailClosedWithResourceScope(t *testing.T) {
	_, database, vault := setupKnowledgeMCPServer(t)
	defer database.Close()

	for _, capability := range []authz.Capability{
		authz.VaultRead,
		authz.ContextCompile,
		authz.AIInvoke,
	} {
		t.Run(string(capability), func(t *testing.T) {
			registry, err := authz.NewRegistry(vault)
			if err != nil {
				t.Fatal(err)
			}
			issued, err := registry.Mint(authz.MintRequest{
				AgentID: "reader-" + strings.ReplaceAll(string(capability), ":", "-"),
				Capabilities: []authz.Capability{capability},
				Scope: authz.Scope{Projects: []string{"alpha"}},
			})
			if err != nil {
				t.Fatal(err)
			}

			server := NewServer(vault, database)
			if err := server.SetCapabilityToken(issued.Token); err != nil {
				t.Fatal(err)
			}
			err = server.RegisterRuntimeSurface(false)
			if err == nil || !strings.Contains(err.Error(), "do not yet support") {
				t.Fatalf("expected scoped global-only capability to fail closed, got %v", err)
			}
			if len(server.tools) != 0 || len(server.resources) != 0 {
				t.Fatalf("failed registration leaked tools=%d resources=%d", len(server.tools), len(server.resources))
			}
		})
	}
}

func TestRegisterRuntimeSurfaceKnowledgeReadAllowsProjectAndSessionScope(t *testing.T) {
	_, database, vault := setupKnowledgeMCPServer(t)
	defer database.Close()

	registry, err := authz.NewRegistry(vault)
	if err != nil {
		t.Fatal(err)
	}
	issued, err := registry.Mint(authz.MintRequest{
		AgentID:      "knowledge-reader",
		Capabilities: []authz.Capability{authz.KnowledgeRead},
		Scope: authz.Scope{
			Projects: []string{"alpha"},
			Sessions: []string{"session-alpha"},
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
		t.Fatalf("project/session-scoped knowledge:read should register: %v", err)
	}
	for _, expected := range []string{"agentvault.get_object", "agentvault.list_memories", "agentvault.get_session"} {
		if _, ok := server.tools[expected]; !ok {
			t.Errorf("scoped knowledge surface missing %s", expected)
		}
	}
}

func TestRegisterRuntimeSurfaceKnowledgeReadRejectsPathScope(t *testing.T) {
	_, database, vault := setupKnowledgeMCPServer(t)
	defer database.Close()

	registry, err := authz.NewRegistry(vault)
	if err != nil {
		t.Fatal(err)
	}
	issued, err := registry.Mint(authz.MintRequest{
		AgentID:      "knowledge-reader",
		Capabilities: []authz.Capability{authz.KnowledgeRead},
		Scope:        authz.Scope{PathPrefixes: []string{"30-projects/alpha"}},
	})
	if err != nil {
		t.Fatal(err)
	}
	server := NewServer(vault, database)
	if err := server.SetCapabilityToken(issued.Token); err != nil {
		t.Fatal(err)
	}
	if err := server.RegisterRuntimeSurface(false); err == nil || !strings.Contains(err.Error(), "path-prefix") {
		t.Fatalf("expected knowledge:read path scope to fail closed, got %v", err)
	}
	if len(server.tools) != 0 {
		t.Fatalf("failed knowledge registration leaked %d tools", len(server.tools))
	}
}

func TestRegisterRuntimeSurfaceUnscopedReadAndMutationCapabilitiesCompose(t *testing.T) {
	_, database, vault := setupKnowledgeMCPServer(t)
	defer database.Close()

	registry, err := authz.NewRegistry(vault)
	if err != nil {
		t.Fatal(err)
	}
	issued, err := registry.Mint(authz.MintRequest{
		AgentID: "builder",
		Capabilities: []authz.Capability{
			authz.VaultRead,
			authz.ContextCompile,
			authz.MutationRead,
			authz.MutationPropose,
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
		"agentvault.search",
		"agentvault.compile_context",
		"agentvault.get_mutation",
		"agentvault.list_mutations",
		"agentvault.propose_mutation",
	} {
		if _, ok := server.tools[expected]; !ok {
			t.Errorf("composed capability surface missing %s", expected)
		}
	}
	if _, ok := server.tools["agentvault.ask"]; ok {
		t.Fatal("ai:invoke was not granted")
	}
	if _, ok := server.tools["agentvault.get_object"]; ok {
		t.Fatal("knowledge:read was not granted")
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

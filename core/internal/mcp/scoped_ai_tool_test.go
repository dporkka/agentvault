package mcp

import (
	"context"
	"strings"
	"testing"

	"github.com/agentvault/core/internal/ai"
	"github.com/agentvault/core/internal/authz"
	"github.com/agentvault/core/internal/config"
	"github.com/agentvault/core/internal/contract"
	"github.com/agentvault/core/internal/knowledge"
)

type captureAIProvider struct {
	messages []ai.Message
}

func (p *captureAIProvider) Name() string { return "capture" }
func (p *captureAIProvider) Chat(_ context.Context, messages []ai.Message) (string, error) {
	p.messages = append([]ai.Message(nil), messages...)
	return "fallback answer", nil
}
func (p *captureAIProvider) ChatJSON(_ context.Context, messages []ai.Message, result interface{}) error {
	p.messages = append([]ai.Message(nil), messages...)
	answer := result.(*contract.Answer)
	*answer = contract.Answer{
		Answer:     "authorized answer",
		Confidence: "high",
		Sources: []contract.Source{{
			ID: "provider-injected", Path: "secret.md", Title: "Provider supplied source",
		}},
	}
	return nil
}
func (p *captureAIProvider) HealthCheck(context.Context) error { return nil }

func TestAnswerFromAuthorizedBundleControlsPromptAndSources(t *testing.T) {
	provider := &captureAIProvider{}
	bundle := contract.ContextBundle{Items: []contract.ContextItem{{
		Kind: "object", ID: "obj-alpha", Title: "Allowed decision", Content: "ALPHA_ALLOWED_CONTEXT", Path: "30-projects/alpha/decision.md",
	}}}
	answer, err := answerFromAuthorizedBundle(context.Background(), provider, "What is allowed?", bundle)
	if err != nil {
		t.Fatal(err)
	}
	if len(provider.messages) != 2 {
		t.Fatalf("provider got %d messages, want 2", len(provider.messages))
	}
	if provider.messages[0].Role != "system" || !strings.Contains(provider.messages[0].Content, "untrusted evidence") {
		t.Fatalf("missing prompt-injection boundary: %+v", provider.messages[0])
	}
	if !strings.Contains(provider.messages[1].Content, "ALPHA_ALLOWED_CONTEXT") || strings.Contains(provider.messages[1].Content, "BETA_SECRET_CONTEXT") {
		t.Fatalf("unexpected provider context: %s", provider.messages[1].Content)
	}
	if len(answer.Sources) != 1 || answer.Sources[0].ID != "obj-alpha" {
		t.Fatalf("provider source injection was not replaced: %+v", answer.Sources)
	}
	if answer.Sources[0].Path != "30-projects/alpha/decision.md" {
		t.Fatalf("unexpected authoritative source path: %+v", answer.Sources[0])
	}
}

func TestScopedAIInvokeRequiresContextCapabilityAndRejectsPathScope(t *testing.T) {
	for _, tc := range []struct {
		name         string
		capabilities []authz.Capability
		scope        authz.Scope
		wantErr      string
	}{
		{
			name: "missing context capability",
			capabilities: []authz.Capability{authz.AIInvoke},
			scope: authz.Scope{Projects: []string{"alpha"}},
			wantErr: "requires context:compile",
		},
		{
			name: "path scope",
			capabilities: []authz.Capability{authz.AIInvoke, authz.ContextCompile},
			scope: authz.Scope{PathPrefixes: []string{"30-projects/alpha"}},
			wantErr: "path-prefix",
		},
	} {
		t.Run(tc.name, func(t *testing.T) {
			_, database, vault := setupKnowledgeMCPServer(t)
			defer database.Close()
			registry, err := authz.NewRegistry(vault)
			if err != nil {
				t.Fatal(err)
			}
			issued, err := registry.Mint(authz.MintRequest{AgentID: "agent", Capabilities: tc.capabilities, Scope: tc.scope})
			if err != nil {
				t.Fatal(err)
			}
			server := NewServer(vault, database)
			if err := server.SetCapabilityToken(issued.Token); err != nil {
				t.Fatal(err)
			}
			err = server.RegisterRuntimeSurface(false)
			if err == nil || !strings.Contains(err.Error(), tc.wantErr) {
				t.Fatalf("registration error=%v want containing %q", err, tc.wantErr)
			}
			if len(server.tools) != 0 || len(server.resources) != 0 {
				t.Fatalf("failed scoped AI registration leaked tools=%d resources=%d", len(server.tools), len(server.resources))
			}
		})
	}
}

func TestScopedAIInvokeUsesAuthorizedSessionContextEndToEnd(t *testing.T) {
	_, database, vault := setupKnowledgeMCPServer(t)
	defer database.Close()
	store := knowledge.New(database, vault)
	if err := store.ReplayJournal(); err != nil {
		t.Fatal(err)
	}
	alphaSession, err := store.StartSession(contract.StartAgentSessionRequest{AgentID: "agent", Project: "alpha", Objective: "ALPHA_SESSION_OBJECTIVE"})
	if err != nil {
		t.Fatal(err)
	}
	betaSession, err := store.StartSession(contract.StartAgentSessionRequest{AgentID: "agent", Project: "beta", Objective: "BETA_SECRET_OBJECTIVE"})
	if err != nil {
		t.Fatal(err)
	}
	_ = betaSession

	if err := config.Save(vault, &config.VaultConfig{
		VaultPath: vault,
		AI:        &config.AIConfig{Provider: "mock"},
		Templates: map[string]string{},
	}); err != nil {
		t.Fatal(err)
	}
	registry, err := authz.NewRegistry(vault)
	if err != nil {
		t.Fatal(err)
	}
	issued, err := registry.Mint(authz.MintRequest{
		AgentID: "agent",
		Capabilities: []authz.Capability{authz.AIInvoke, authz.ContextCompile},
		Scope: authz.Scope{Projects: []string{"alpha"}, Sessions: []string{alphaSession.ID}},
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
	for _, name := range []string{"agentvault.ask", "agentvault.compile_context"} {
		if _, ok := server.tools[name]; !ok {
			t.Fatalf("missing scoped AI tool %s", name)
		}
	}

	bundle, _, err := server.compileScopedAIContext(store, memory.NewStore(database), map[string]interface{}{
		"question": "What is the session objective?", "session_id": alphaSession.ID,
	})
	if err != nil {
		t.Fatal(err)
	}
	encoded, _ := jsonMarshal(bundle.Items)
	if !strings.Contains(string(encoded), "ALPHA_SESSION_OBJECTIVE") {
		t.Fatalf("authorized context missing alpha session: %s", encoded)
	}
	if strings.Contains(string(encoded), "BETA_SECRET_OBJECTIVE") {
		t.Fatalf("scoped context leaked beta session: %s", encoded)
	}

	result, err := server.tools["agentvault.ask"].Handler(map[string]interface{}{
		"question": "What is the session objective?", "session_id": alphaSession.ID,
	})
	if err != nil {
		t.Fatal(err)
	}
	if !strings.Contains(result, "This is a mock response for testing.") {
		t.Fatalf("unexpected scoped AI response: %s", result)
	}
	if !strings.Contains(result, "Authorized Sources") || strings.Contains(result, "BETA_SECRET_OBJECTIVE") {
		t.Fatalf("unexpected source rendering: %s", result)
	}
}

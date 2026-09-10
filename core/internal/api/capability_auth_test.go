package api

import (
	"bytes"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"os"
	"path/filepath"
	"testing"

	"github.com/agentvault/core/internal/authz"
	"github.com/agentvault/core/internal/contract"
)

func TestCapabilityScopedMutationLifecycle(t *testing.T) {
	vaultPath, database := setupTestVault(t)
	defer database.Close()
	if err := os.MkdirAll(filepath.Join(vaultPath, "30-projects", "alpha"), 0o755); err != nil {
		t.Fatal(err)
	}
	target := filepath.Join(vaultPath, "30-projects", "alpha", "plan.md")
	if err := os.WriteFile(target, []byte("before\n"), 0o644); err != nil {
		t.Fatal(err)
	}

	server := NewServer(vaultPath, database)
	session, err := server.knowledge.StartSession(contract.StartAgentSessionRequest{
		AgentID: "planner", Project: "alpha", Objective: "update plan",
	})
	if err != nil {
		t.Fatal(err)
	}
	registry, err := authz.NewRegistry(vaultPath)
	if err != nil {
		t.Fatal(err)
	}
	proposer, err := registry.Mint(authz.MintRequest{
		ID: "proposer", AgentID: "planner", Capabilities: []authz.Capability{authz.MutationRead, authz.MutationPropose},
		Scope: authz.Scope{PathPrefixes: []string{"30-projects/alpha"}, Projects: []string{"alpha"}, Sessions: []string{session.ID}},
	})
	if err != nil {
		t.Fatal(err)
	}
	reviewer, err := registry.Mint(authz.MintRequest{
		ID: "reviewer", AgentID: "reviewer-agent", Capabilities: []authz.Capability{authz.MutationRead, authz.MutationApprove, authz.MutationCommit},
		Scope: authz.Scope{PathPrefixes: []string{"30-projects/alpha"}, Projects: []string{"alpha"}, Sessions: []string{session.ID}},
	})
	if err != nil {
		t.Fatal(err)
	}

	server.RegisterRoutes()
	handler := server.authMiddleware(server.mux)
	ts := httptest.NewServer(handler)
	defer ts.Close()

	call := func(token, method, route string, body interface{}, target interface{}) int {
		t.Helper()
		var payload []byte
		if body != nil {
			payload, _ = json.Marshal(body)
		}
		req, err := http.NewRequest(method, ts.URL+route, bytes.NewReader(payload))
		if err != nil {
			t.Fatal(err)
		}
		if token != "" {
			req.Header.Set("Authorization", "Bearer "+token)
		}
		if body != nil {
			req.Header.Set("Content-Type", "application/json")
		}
		resp, err := http.DefaultClient.Do(req)
		if err != nil {
			t.Fatal(err)
		}
		defer resp.Body.Close()
		if target != nil && resp.StatusCode >= 200 && resp.StatusCode < 300 {
			if err := json.NewDecoder(resp.Body).Decode(target); err != nil {
				t.Fatal(err)
			}
		return resp.StatusCode
	}

	after := "after\n"
	if status := call(proposer.Token, http.MethodPost, "/mutations", map[string]interface{}{
		"kind": "replace", "path": "30-projects/alpha/plan.md", "content": after, "reason": "scoped test",
		"agentId": "spoofed", "sessionId": session.ID,
	}, nil); status != http.StatusForbidden {
		t.Fatalf("spoofed agent status=%d want 403", status)
	}

	var proposal contract.MutationProposal
	if status := call(proposer.Token, http.MethodPost, "/mutations", map[string]interface{}{
		"kind": "replace", "path": "30-projects/alpha/plan.md", "content": after, "reason": "scoped test", "sessionId": session.ID,
	}, &proposal); status != http.StatusCreated {
		t.Fatalf("proposal status=%d want 201", status)
	}
	if proposal.AgentID != "planner" {
		t.Fatalf("proposal agent=%q want planner", proposal.AgentID)
	}
	if status := call("", http.MethodGet, "/mutations/"+proposal.ID, nil, nil); status != http.StatusUnauthorized {
		t.Fatalf("anonymous mutation read status=%d want 401", status)
	}
	if status := call(proposer.Token, http.MethodPost, "/mutations/"+proposal.ID+"/approve", map[string]interface{}{}, nil); status != http.StatusForbidden {
		t.Fatalf("proposer approval status=%d want 403", status)
	}

	var approved contract.MutationProposal
	if status := call(reviewer.Token, http.MethodPost, "/mutations/"+proposal.ID+"/approve", map[string]interface{}{}, &approved); status != http.StatusOK {
		t.Fatalf("reviewer approval status=%d want 200", status)
	}
	if approved.ApprovedBy != "reviewer" {
		t.Fatalf("approvedBy=%q want reviewer capability identity", approved.ApprovedBy)
	}
	if status := call(reviewer.Token, http.MethodPost, "/mutations/"+proposal.ID+"/commit", nil, nil); status != http.StatusOK {
		t.Fatalf("reviewer commit status=%d want 200", status)
	}
	if got, _ := os.ReadFile(target); string(got) != after {
		t.Fatalf("committed content=%q", got)
	}

	// A scoped mutation token must never become a generic API write token.
	if status := call(reviewer.Token, http.MethodPost, "/notes", map[string]interface{}{"title": "bypass"}, nil); status != http.StatusUnauthorized {
		t.Fatalf("generic write with mutation token status=%d want 401", status)
	}
}

func TestProjectScopeUsesDurableSessionProject(t *testing.T) {
	vaultPath, database := setupTestVault(t)
	defer database.Close()
	server := NewServer(vaultPath, database)
	session, err := server.knowledge.StartSession(contract.StartAgentSessionRequest{AgentID: "agent", Project: "beta", Objective: "test"})
	if err != nil {
		t.Fatal(err)
	}
	registry, _ := authz.NewRegistry(vaultPath)
	issued, err := registry.Mint(authz.MintRequest{
		AgentID: "agent", Capabilities: []authz.Capability{authz.MutationPropose}, Scope: authz.Scope{Projects: []string{"alpha"}},
	})
	if err != nil {
		t.Fatal(err)
	}
	principal, err := registry.Authenticate(issued.Token)
	if err != nil {
		t.Fatal(err)
	}
	r := httptest.NewRequest(http.MethodPost, "/mutations", nil)
	r = r.WithContext(contextWithCapabilityIdentity(r, capabilityIdentity{Principal: principal}))
	if err := server.authorizeMutationResource(r, authz.MutationPropose, "note.md", session.ID); err == nil {
		t.Fatal("expected beta durable session to be rejected by alpha project scope")
	}
}

func contextWithCapabilityIdentity(r *http.Request, identity capabilityIdentity) context.Context {
	return context.WithValue(r.Context(), capabilityIdentityKey{}, identity)
}

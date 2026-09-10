package mcp

import (
	"context"
	"testing"

	"github.com/agentvault/core/internal/authz"
)

func TestBoundCapabilityRevocationStopsMCPDispatch(t *testing.T) {
	server, database, vault := setupKnowledgeMCPServer(t)
	defer database.Close()
	registry, err := authz.NewRegistry(vault)
	if err != nil {
		t.Fatal(err)
	}
	issued, err := registry.Mint(authz.MintRequest{
		ID: "revocable-mcp", AgentID: "agent", Capabilities: []authz.Capability{authz.MutationRead},
	})
	if err != nil {
		t.Fatal(err)
	}
	if err := server.SetCapabilityToken(issued.Token); err != nil {
		t.Fatal(err)
	}

	before := server.Handle(context.Background(), JSONRPCRequest{JSONRPC: "2.0", ID: 1, Method: "initialize"})
	if before.Error != nil {
		t.Fatalf("bound capability should initialize before revocation: %+v", before.Error)
	}
	if _, err := registry.Revoke(issued.Principal.ID); err != nil {
		t.Fatal(err)
	}
	after := server.Handle(context.Background(), JSONRPCRequest{JSONRPC: "2.0", ID: 2, Method: "initialize"})
	if after.Error == nil || after.Error.Code != -32001 {
		t.Fatalf("revoked bound capability should stop MCP dispatch: %+v", after)
	}
}

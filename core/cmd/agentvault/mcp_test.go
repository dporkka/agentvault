package main

import (
	"bytes"
	"encoding/json"
	"io"
	"net/http"
	"testing"
	"time"

	"github.com/agentvault/core/internal/authz"
)

func TestMcpFlagsRegistered(t *testing.T) {
	if mcpServeCmd.Flags().Lookup("http") == nil {
		t.Error("expected 'http' flag to be registered on mcp serve")
	}
	if mcpServeCmd.Flags().Lookup("port") == nil {
		t.Error("expected 'port' flag to be registered on mcp serve")
	}
	if mcpServeCmd.Flags().Lookup("allow-direct-writes") == nil {
		t.Error("expected 'allow-direct-writes' flag to be registered on mcp serve")
	}
	if mcpServeCmd.Flags().Lookup("capability-token") == nil {
		t.Error("expected 'capability-token' flag to be registered on mcp serve")
	}
}

func TestRunMcpServeHTTPStartsAndStops(t *testing.T) {
	vp, database := setupTestVaultWithDB(t)
	defer database.Close()
	vaultPath = vp

	registry, err := authz.NewRegistry(vp)
	if err != nil {
		t.Fatal(err)
	}
	issued, err := registry.Mint(authz.MintRequest{
		AgentID: "mcp-test", Capabilities: []authz.Capability{authz.MutationRead, authz.MutationPropose},
	})
	if err != nil {
		t.Fatal(err)
	}

	mcpHTTP = true
	mcpPort = 47322
	mcpAllowDirectWrites = false
	mcpCapabilityToken = issued.Token
	defer func() {
		mcpHTTP = false
		mcpPort = 7777
		mcpAllowDirectWrites = false
		mcpCapabilityToken = ""
	}()

	stopCh := make(chan struct{})
	original := mcpStopSignal
	mcpStopSignal = func() <-chan struct{} { return stopCh }
	defer func() { mcpStopSignal = original }()

	doneCh := make(chan struct{})
	go func() {
		defer close(doneCh)
		cmd := cobraCommandArgs([]string{"serve"})
		runMcpServe(cmd, []string{"serve"})
	}()

	addr := "http://127.0.0.1:47322"
	deadline := time.Now().Add(3 * time.Second)
	var lastErr error
	for time.Now().Before(deadline) {
		resp, err := http.Get(addr)
		if err == nil {
			resp.Body.Close()
			lastErr = nil
			break
		}
		lastErr = err
		time.Sleep(50 * time.Millisecond)
	}
	if lastErr != nil {
		t.Fatalf("MCP server did not start: %v", lastErr)
	}

	reqBody, _ := json.Marshal(map[string]interface{}{
		"jsonrpc": "2.0",
		"id":      1,
		"method":  "initialize",
	})
	req, err := http.NewRequest(http.MethodPost, addr, bytes.NewReader(reqBody))
	if err != nil {
		t.Fatal(err)
	}
	req.Header.Set("Content-Type", "application/json")
	req.Header.Set("Authorization", "Bearer "+issued.Token)
	resp, err := http.DefaultClient.Do(req)
	if err != nil {
		t.Fatalf("MCP request failed: %v", err)
	}
	body, _ := io.ReadAll(resp.Body)
	resp.Body.Close()
	if resp.StatusCode != http.StatusOK {
		t.Fatalf("expected status 200, got %d: %s", resp.StatusCode, string(body))
	}
	var result map[string]interface{}
	if err := json.Unmarshal(body, &result); err != nil {
		t.Fatalf("failed to decode MCP response: %v", err)
	}
	if result["jsonrpc"] != "2.0" {
		t.Errorf("expected jsonrpc 2.0 in response, got %v", result["jsonrpc"])
	}

	close(stopCh)

	select {
	case <-doneCh:
	case <-time.After(3 * time.Second):
		t.Fatal("runMcpServe did not stop after signal")
	}
}

func TestMcpCommandStructure(t *testing.T) {
	if mcpCmd.Short == "" {
		t.Error("expected mcp command to have a short description")
	}
	if mcpServeCmd == nil {
		t.Error("expected mcp serve subcommand")
	}
}

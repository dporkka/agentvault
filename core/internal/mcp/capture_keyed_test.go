package mcp

import (
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"

	"github.com/agentvault/core/internal/capturekey"
)

func TestMCPCaptureSharesDeterministicAuthorityWithHTTP(t *testing.T) {
	srv, database := setupTestServer(t)
	defer database.Close()
	key := "mobile:sensor/α"
	first, err := srv.handleCapture(map[string]interface{}{
		"title": "Measurement", "text": "first", "external_id": key,
	})
	if err != nil {
		t.Fatal(err)
	}
	path := capturekey.CanonicalPath(key)
	if !strings.Contains(first, path) {
		t.Fatalf("MCP capture uses non-canonical path: %q", first)
	}
	replay, err := srv.handleCapture(map[string]interface{}{
		"title": "Changed title", "text": "must not replace", "external_id": key,
	})
	if err != nil || !strings.Contains(replay, "(idempotent)") || !strings.Contains(replay, path) {
		t.Fatalf("MCP replay result: %q, %v", replay, err)
	}
	data, err := os.ReadFile(filepath.Join(srv.vaultPath, filepath.FromSlash(path)))
	if err != nil || !strings.Contains(string(data), "first") || strings.Contains(string(data), "must not replace") {
		t.Fatalf("MCP collision overwrote existing note: %q, %v", data, err)
	}
	time.Sleep(200 * time.Millisecond)
}

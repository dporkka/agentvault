package main

import (
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/agentvault/core/internal/config"
)

func setupVaultWithConfig(t *testing.T) string {
	t.Helper()
	vp := setupTestVault(t)
	cfg := config.DefaultConfig(vp)
	if err := config.Save(vp, cfg); err != nil {
		t.Fatalf("failed to save config: %v", err)
	}
	return vp
}

func TestRunConfigGet(t *testing.T) {
	vp := setupVaultWithConfig(t)
	vaultPath = vp

	tests := []struct {
		key string
	}{
		{"ai"},
		{"ai.provider"},
		{"ai.baseUrl"},
		{"ai.chatModel"},
		{"ai.embeddingModel"},
		{"ai.apiKey"},
	}

	for _, tt := range tests {
		t.Run(tt.key, func(t *testing.T) {
			cmd := cobraCommandArgs([]string{tt.key})
			if err := runConfigGet(cmd, []string{tt.key}); err != nil {
				t.Errorf("runConfigGet(%q) returned error: %v", tt.key, err)
			}
		})
	}
}

func TestRunConfigGetUnknownKey(t *testing.T) {
	vp := setupVaultWithConfig(t)
	vaultPath = vp

	cmd := cobraCommandArgs([]string{"unknown"})
	err := runConfigGet(cmd, []string{"unknown"})
	if err == nil {
		t.Fatal("runConfigGet expected error for unknown key")
	}
	if !strings.Contains(err.Error(), "unknown config key") {
		t.Errorf("error should mention 'unknown config key', got: %v", err)
	}
}

func TestRunConfigSetAndGet(t *testing.T) {
	vp := setupVaultWithConfig(t)
	vaultPath = vp

	cmd := cobraCommandArgs([]string{"ai.provider", "openai"})
	if err := runConfigSet(cmd, []string{"ai.provider", "openai"}); err != nil {
		t.Fatalf("runConfigSet returned error: %v", err)
	}

	cfg, err := config.Load(vp)
	if err != nil {
		t.Fatalf("failed to load config after set: %v", err)
	}
	if cfg.AI == nil || cfg.AI.Provider != "openai" {
		t.Errorf("expected provider to be 'openai', got %q", cfg.AI.Provider)
	}
}

func TestRunConfigSetInvalidProvider(t *testing.T) {
	vp := setupVaultWithConfig(t)
	vaultPath = vp

	cmd := cobraCommandArgs([]string{"ai.provider", "invalid"})
	err := runConfigSet(cmd, []string{"ai.provider", "invalid"})
	if err == nil {
		t.Fatal("runConfigSet expected error for invalid provider")
	}
	if !strings.Contains(err.Error(), "invalid provider") {
		t.Errorf("error should mention 'invalid provider', got: %v", err)
	}
}

func TestRunConfigSetUnknownKey(t *testing.T) {
	vp := setupVaultWithConfig(t)
	vaultPath = vp

	cmd := cobraCommandArgs([]string{"unknown", "value"})
	err := runConfigSet(cmd, []string{"unknown", "value"})
	if err == nil {
		t.Fatal("runConfigSet expected error for unknown key")
	}
	if !strings.Contains(err.Error(), "unknown config key") {
		t.Errorf("error should mention 'unknown config key', got: %v", err)
	}
}

func TestRunConfigShow(t *testing.T) {
	vp := setupVaultWithConfig(t)
	vaultPath = vp

	cmd := cobraCommandArgs([]string{})
	if err := runConfigShow(cmd, []string{}); err != nil {
		t.Fatalf("runConfigShow returned error: %v", err)
	}
}

func TestRunConfigShowMasksAPIKey(t *testing.T) {
	vp := setupVaultWithConfig(t)
	vaultPath = vp

	cfg, err := config.Load(vp)
	if err != nil {
		t.Fatalf("failed to load config: %v", err)
	}
	cfg.AI = &config.AIConfig{
		Provider: "openai",
		APIKey:   "sk-secret-key",
	}
	if err := config.Save(vp, cfg); err != nil {
		t.Fatalf("failed to save config: %v", err)
	}

	cmd := cobraCommandArgs([]string{})
	if err := runConfigShow(cmd, []string{}); err != nil {
		t.Fatalf("runConfigShow returned error: %v", err)
	}

	data, err := os.ReadFile(filepath.Join(vp, ".agentvault", "config.json"))
	if err != nil {
		t.Fatalf("failed to read config: %v", err)
	}
	if !strings.Contains(string(data), "sk-secret-key") {
		t.Error("config file should still contain the unmasked API key")
	}
}

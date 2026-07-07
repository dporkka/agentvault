package config

import (
	"os"
	"path/filepath"
	"testing"
)

func TestDefaultConfig(t *testing.T) {
	vaultPath := "/tmp/test-vault"
	cfg := DefaultConfig(vaultPath)

	if cfg == nil {
		t.Fatal("DefaultConfig returned nil")
	}
	if cfg.VaultPath != vaultPath {
		t.Errorf("VaultPath = %q, want %q", cfg.VaultPath, vaultPath)
	}
	if cfg.Templates == nil {
		t.Error("Templates map is nil")
	}
	if cfg.CreatedAt == "" {
		t.Error("CreatedAt is empty")
	}
	if cfg.UpdatedAt == "" {
		t.Error("UpdatedAt is empty")
	}
	if cfg.CreatedAt != cfg.UpdatedAt {
		t.Error("CreatedAt and UpdatedAt should be equal for a new config")
	}
}

func TestSaveAndLoad(t *testing.T) {
	tmpDir := t.TempDir()
	cfg := DefaultConfig(tmpDir)
	cfg.DefaultProject = "my-project"
	cfg.AI = &AIConfig{
		Provider:       "openai",
		BaseURL:        "https://api.openai.com/v1",
		ChatModel:      "gpt-4",
		EmbeddingModel: "text-embedding-3-small",
		APIKey:         "secret-key",
	}
	cfg.Templates["note"] = "# {{.Title}}"

	if err := Save(tmpDir, cfg); err != nil {
		t.Fatalf("Save failed: %v", err)
	}

	configPath := filepath.Join(tmpDir, ".agentvault", "config.json")
	if _, err := os.Stat(configPath); err != nil {
		t.Fatalf("config file not written: %v", err)
	}

	loaded, err := Load(tmpDir)
	if err != nil {
		t.Fatalf("Load failed: %v", err)
	}

	if loaded.VaultPath != cfg.VaultPath {
		t.Errorf("VaultPath = %q, want %q", loaded.VaultPath, cfg.VaultPath)
	}
	if loaded.DefaultProject != cfg.DefaultProject {
		t.Errorf("DefaultProject = %q, want %q", loaded.DefaultProject, cfg.DefaultProject)
	}
	if loaded.AI == nil {
		t.Fatal("AI config is nil after load")
	}
	if loaded.AI.Provider != cfg.AI.Provider {
		t.Errorf("AI.Provider = %q, want %q", loaded.AI.Provider, cfg.AI.Provider)
	}
	if loaded.AI.APIKey != cfg.AI.APIKey {
		t.Errorf("AI.APIKey = %q, want %q", loaded.AI.APIKey, cfg.AI.APIKey)
	}
	if got, ok := loaded.Templates["note"]; !ok || got != cfg.Templates["note"] {
		t.Errorf("Templates[note] = %q, want %q", got, cfg.Templates["note"])
	}
}

func TestInit(t *testing.T) {
	tmpDir := t.TempDir()

	cfg, err := Init(tmpDir)
	if err != nil {
		t.Fatalf("Init failed: %v", err)
	}
	if cfg == nil {
		t.Fatal("Init returned nil config")
	}

	loaded, err := Load(tmpDir)
	if err != nil {
		t.Fatalf("Load after Init failed: %v", err)
	}
	if loaded.VaultPath != tmpDir {
		t.Errorf("VaultPath = %q, want %q", loaded.VaultPath, tmpDir)
	}
	if loaded.Templates == nil {
		t.Error("Templates map is nil after Init")
	}
}

func TestLoadMissingFile(t *testing.T) {
	tmpDir := t.TempDir()
	_, err := Load(tmpDir)
	if err == nil {
		t.Fatal("expected error when loading missing config")
	}
}

func TestLoadInvalidJSON(t *testing.T) {
	tmpDir := t.TempDir()
	configDir := filepath.Join(tmpDir, ".agentvault")
	if err := os.MkdirAll(configDir, 0755); err != nil {
		t.Fatalf("failed to create config dir: %v", err)
	}
	if err := os.WriteFile(filepath.Join(configDir, "config.json"), []byte("not json"), 0644); err != nil {
		t.Fatalf("failed to write invalid config: %v", err)
	}

	_, err := Load(tmpDir)
	if err == nil {
		t.Fatal("expected error when loading invalid JSON config")
	}
}

func TestSaveCreatesDirectory(t *testing.T) {
	tmpDir := t.TempDir()
	nestedVault := filepath.Join(tmpDir, "deep", "vault")

	if err := os.MkdirAll(nestedVault, 0755); err != nil {
		t.Fatalf("failed to create vault dir: %v", err)
	}

	cfg := DefaultConfig(nestedVault)
	if err := Save(nestedVault, cfg); err != nil {
		t.Fatalf("Save failed: %v", err)
	}

	configPath := filepath.Join(nestedVault, ".agentvault", "config.json")
	if _, err := os.Stat(configPath); err != nil {
		t.Fatalf("config file not written: %v", err)
	}
}

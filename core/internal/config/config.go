// Package config handles AgentVault configuration.
package config

import (
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"time"
)

// VaultConfig holds the configuration for an AgentVault.
type VaultConfig struct {
	VaultPath      string            `json:"vaultPath"`
	DefaultProject string            `json:"defaultProject,omitempty"`
	AI             *AIConfig         `json:"ai,omitempty"`
	Templates      map[string]string `json:"templates,omitempty"`
	CreatedAt      string            `json:"createdAt"`
	UpdatedAt      string            `json:"updatedAt"`
}

// AIConfig holds AI provider settings.
type AIConfig struct {
	Provider       string `json:"provider"`
	BaseURL        string `json:"baseUrl"`
	ChatModel      string `json:"chatModel"`
	EmbeddingModel string `json:"embeddingModel"`
	APIKey         string `json:"apiKey,omitempty"`
}

// Load reads the config from the vault directory.
func Load(vaultPath string) (*VaultConfig, error) {
	configPath := filepath.Join(vaultPath, ".agentvault", "config.json")
	data, err := os.ReadFile(configPath)
	if err != nil {
		return nil, fmt.Errorf("could not read config: %w", err)
	}
	var cfg VaultConfig
	if err := json.Unmarshal(data, &cfg); err != nil {
		return nil, fmt.Errorf("invalid config JSON: %w", err)
	}
	return &cfg, nil
}

// Save writes the config to the vault directory.
func Save(vaultPath string, cfg *VaultConfig) error {
	configPath := filepath.Join(vaultPath, ".agentvault", "config.json")
	data, err := json.MarshalIndent(cfg, "", "  ")
	if err != nil {
		return fmt.Errorf("could not marshal config: %w", err)
	}
	if err := os.WriteFile(configPath, data, 0644); err != nil {
		return fmt.Errorf("could not write config: %w", err)
	}
	return nil
}

// Init creates a new default config for a vault.
func Init(vaultPath string) (*VaultConfig, error) {
	cfg := DefaultConfig(vaultPath)
	if err := Save(vaultPath, cfg); err != nil {
		return nil, err
	}
	return cfg, nil
}

// DefaultConfig returns a default vault configuration.
func DefaultConfig(vaultPath string) *VaultConfig {
	now := time.Now().UTC().Format(time.RFC3339)
	return &VaultConfig{
		VaultPath: vaultPath,
		CreatedAt: now,
		UpdatedAt: now,
		Templates: make(map[string]string),
	}
}

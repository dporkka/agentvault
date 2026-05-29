// Package vault manages AgentVault directory structure.
package vault

import (
	"fmt"
	"os"
	"path/filepath"
)

// DefaultFolders defines the standard vault folder structure.
var DefaultFolders = []string{
	"00-inbox",
	"10-notes",
	"20-projects",
	"30-decisions",
	"40-research",
	"50-people",
	"60-companies",
	"70-prompts",
	"80-agent-runs",
	"90-archive",
}

// Init creates a new AgentVault at the given path.
func Init(vaultPath string) error {
	// Create .agentvault directory
	agentvaultDir := filepath.Join(vaultPath, ".agentvault")
	if err := os.MkdirAll(agentvaultDir, 0755); err != nil {
		return fmt.Errorf("failed to create .agentvault directory: %w", err)
	}

	// Create standard folders
	for _, folder := range DefaultFolders {
		folderPath := filepath.Join(vaultPath, folder)
		if err := os.MkdirAll(folderPath, 0755); err != nil {
			return fmt.Errorf("failed to create folder %s: %w", folder, err)
		}
	}

	return nil
}

// IsVault checks if the given path is an initialized AgentVault.
func IsVault(path string) bool {
	agentvaultDir := filepath.Join(path, ".agentvault")
	info, err := os.Stat(agentvaultDir)
	if err != nil {
		return false
	}
	return info.IsDir()
}

// VaultDBPath returns the path to the SQLite database.
func VaultDBPath(vaultPath string) string {
	return filepath.Join(vaultPath, ".agentvault", "agentvault.db")
}

// VaultConfigPath returns the path to the config file.
func VaultConfigPath(vaultPath string) string {
	return filepath.Join(vaultPath, ".agentvault", "config.json")
}

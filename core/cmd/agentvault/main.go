package main

import (
	"fmt"
	"os"

	"github.com/agentvault/core/internal/db"
	"github.com/agentvault/core/internal/vault"
	"github.com/spf13/cobra"
)

var (
	vaultPath string
	rootCmd   = &cobra.Command{
		Use:   "agentvault",
		Short: "AgentVault — local-first AI knowledge operating system",
		Long: `AgentVault is a durable, local-first AI knowledge system.

It turns a folder of Markdown/YAML files into an intelligent, searchable,
source-grounded, agent-accessible knowledge base.

The user owns the data. The app is only an interface over durable files.`,
	}
)

func main() {
	if err := rootCmd.Execute(); err != nil {
		fmt.Fprintln(os.Stderr, err)
		os.Exit(1)
	}
}

func init() {
	rootCmd.PersistentFlags().StringVar(&vaultPath, "vault", ".", "Path to AgentVault directory")
}

// getVaultPath returns the vault path, resolving relative paths
func getVaultPath() string {
	if vaultPath == "" {
		return "."
	}
	return vaultPath
}

// requireVault checks that the current directory is an initialized vault
func requireVault() (string, error) {
	vp := getVaultPath()
	if !vault.IsVault(vp) {
		return "", fmt.Errorf("not an AgentVault directory: %s\nRun 'agentvault init' to create one", vp)
	}
	return vp, nil
}

// mustRequireVault exits on error
func mustRequireVault() string {
	vp, err := requireVault()
	if err != nil {
		fmt.Fprintln(os.Stderr, err)
		os.Exit(1)
	}
	return vp
}

// openDB opens the SQLite database for the vault
func openDB(vp string) (*db.DB, error) {
	database, err := db.Open(vp)
	if err != nil {
		return nil, fmt.Errorf("could not open database: %w", err)
	}
	return database, nil
}

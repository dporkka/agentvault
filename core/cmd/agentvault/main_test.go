package main

import (
	"os"
	"path/filepath"
	"testing"

	"github.com/agentvault/core/internal/db"
	"github.com/agentvault/core/internal/indexer"
	"github.com/spf13/cobra"
)

// setupTestVault creates a minimal AgentVault directory structure in a temp dir
// and returns the temp dir path and a cleanup function.
func setupTestVault(t *testing.T) string {
	t.Helper()
	tmpDir := t.TempDir()
	agentvaultDir := filepath.Join(tmpDir, ".agentvault")
	if err := os.MkdirAll(agentvaultDir, 0755); err != nil {
		t.Fatalf("failed to create .agentvault dir: %v", err)
	}
	return tmpDir
}

// setupTestVaultWithDB creates a minimal vault, opens the SQLite database, runs
// migrations, and returns the vault path and database handle. The caller is
// responsible for closing the database.
func setupTestVaultWithDB(t *testing.T) (string, *db.DB) {
	t.Helper()
	vp := setupTestVault(t)
	database, err := db.Open(vp)
	if err != nil {
		t.Fatalf("failed to open db: %v", err)
	}
	if err := database.RunMigrations(); err != nil {
		database.Close()
		t.Fatalf("failed to run migrations: %v", err)
	}
	return vp, database
}

// indexNote writes a markdown note at relPath inside the vault and indexes it
// into the database.
func indexNote(t *testing.T, database *db.DB, vp, relPath, content string) {
	t.Helper()
	fullPath := filepath.Join(vp, relPath)
	if err := os.MkdirAll(filepath.Dir(fullPath), 0755); err != nil {
		t.Fatalf("failed to create note dir: %v", err)
	}
	if err := os.WriteFile(fullPath, []byte(content), 0644); err != nil {
		t.Fatalf("failed to write note: %v", err)
	}
	idx := indexer.New(database, vp)
	if _, err := idx.Index(indexer.IndexOptions{}); err != nil {
		t.Fatalf("failed to index note: %v", err)
	}
}

func TestGetVaultPath(t *testing.T) {
	tests := []struct {
		name      string
		vaultPath string
		want      string
	}{
		{"default empty", "", "."},
		{"explicit path", "/tmp/vault", "/tmp/vault"},
		{"relative path", "./my-vault", "./my-vault"},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			vaultPath = tt.vaultPath
			got := getVaultPath()
			if got != tt.want {
				t.Errorf("getVaultPath() = %q, want %q", got, tt.want)
			}
		})
	}
}

func TestRequireVault(t *testing.T) {
	vp := setupTestVault(t)
	vaultPath = vp

	got, err := requireVault()
	if err != nil {
		t.Fatalf("requireVault() returned error for valid vault: %v", err)
	}
	if got != vp {
		t.Errorf("requireVault() = %q, want %q", got, vp)
	}
}

func TestRequireVaultNotVault(t *testing.T) {
	tmpDir := t.TempDir()
	vaultPath = tmpDir

	_, err := requireVault()
	if err == nil {
		t.Fatal("requireVault() expected error for non-vault directory")
	}
}

func TestRootCommandStructure(t *testing.T) {
	expectedCommands := []string{
		"ask",
		"config",
		"doctor",
		"git",
		"import",
		"index",
		"init",
		"mcp",
		"new",
		"read",
		"search",
		"serve",
	}

	for _, name := range expectedCommands {
		cmd, _, err := rootCmd.Find([]string{name})
		if err != nil {
			t.Errorf("expected command %q to be registered: %v", name, err)
			continue
		}
		if cmd.Name() != name {
			t.Errorf("found command %q, want %q", cmd.Name(), name)
		}
	}
}

func TestRootCommandPersistentFlags(t *testing.T) {
	if rootCmd.PersistentFlags().Lookup("vault") == nil {
		t.Error("expected persistent 'vault' flag to be registered")
	}
}

func TestOpenDBRequiresVault(t *testing.T) {
	// openDB wraps db.Open; verify it returns an error for an invalid vault path.
	_, err := openDB("/nonexistent/path/that/should/not/exist")
	if err == nil {
		t.Fatal("openDB() expected error for invalid vault path")
	}
}

// cobraCommandArgs returns a minimal cobra command suitable for RunE tests.
func cobraCommandArgs(args []string) *cobra.Command {
	cmd := &cobra.Command{Use: "test"}
	cmd.SetArgs(args)
	return cmd
}

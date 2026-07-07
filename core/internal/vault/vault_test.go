package vault

import (
	"os"
	"path/filepath"
	"testing"
)

func TestInit(t *testing.T) {
	t.Run("creates vault structure", func(t *testing.T) {
		tmpDir := t.TempDir()

		if err := Init(tmpDir); err != nil {
			t.Fatalf("Init failed: %v", err)
		}

		// Verify .agentvault directory exists
		info, err := os.Stat(filepath.Join(tmpDir, ".agentvault"))
		if err != nil {
			t.Fatalf(".agentvault directory not created: %v", err)
		}
		if !info.IsDir() {
			t.Fatal(".agentvault is not a directory")
		}

		// Verify default folders exist
		for _, folder := range DefaultFolders {
			folderPath := filepath.Join(tmpDir, folder)
			info, err := os.Stat(folderPath)
			if err != nil {
				t.Errorf("folder %s not created: %v", folder, err)
				continue
			}
			if !info.IsDir() {
				t.Errorf("%s is not a directory", folder)
			}
		}
	})

	t.Run("is idempotent", func(t *testing.T) {
		tmpDir := t.TempDir()

		if err := Init(tmpDir); err != nil {
			t.Fatalf("first Init failed: %v", err)
		}

		// Pre-create a file in one of the folders to ensure it is preserved.
		notePath := filepath.Join(tmpDir, "10-notes", "existing.md")
		if err := os.WriteFile(notePath, []byte("hello"), 0644); err != nil {
			t.Fatalf("failed to write existing file: %v", err)
		}

		if err := Init(tmpDir); err != nil {
			t.Fatalf("second Init failed: %v", err)
		}

		// Verify existing file is preserved
		content, err := os.ReadFile(notePath)
		if err != nil {
			t.Fatalf("existing file was not preserved: %v", err)
		}
		if string(content) != "hello" {
			t.Errorf("existing file content changed: got %q", string(content))
		}
	})
}

func TestIsVault(t *testing.T) {
	t.Run("returns true for initialized vault", func(t *testing.T) {
		tmpDir := t.TempDir()

		if err := Init(tmpDir); err != nil {
			t.Fatalf("Init failed: %v", err)
		}

		if !IsVault(tmpDir) {
			t.Error("Expected IsVault to return true for initialized vault")
		}
	})

	t.Run("returns false for uninitialized directory", func(t *testing.T) {
		tmpDir := t.TempDir()

		if IsVault(tmpDir) {
			t.Error("Expected IsVault to return false for uninitialized directory")
		}
	})

	t.Run("returns false when .agentvault is a file", func(t *testing.T) {
		tmpDir := t.TempDir()
		agentvaultFile := filepath.Join(tmpDir, ".agentvault")

		if err := os.WriteFile(agentvaultFile, []byte("not a directory"), 0644); err != nil {
			t.Fatalf("failed to create .agentvault file: %v", err)
		}

		if IsVault(tmpDir) {
			t.Error("Expected IsVault to return false when .agentvault is a file")
		}
	})

	t.Run("returns false for nonexistent path", func(t *testing.T) {
		if IsVault(filepath.Join(t.TempDir(), "does-not-exist")) {
			t.Error("Expected IsVault to return false for nonexistent path")
		}
	})
}

func TestVaultDBPath(t *testing.T) {
	tmpDir := t.TempDir()
	expected := filepath.Join(tmpDir, ".agentvault", "agentvault.db")
	got := VaultDBPath(tmpDir)

	if got != expected {
		t.Errorf("VaultDBPath: expected %q, got %q", expected, got)
	}
}

func TestVaultConfigPath(t *testing.T) {
	tmpDir := t.TempDir()
	expected := filepath.Join(tmpDir, ".agentvault", "config.json")
	got := VaultConfigPath(tmpDir)

	if got != expected {
		t.Errorf("VaultConfigPath: expected %q, got %q", expected, got)
	}
}

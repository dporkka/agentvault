package plugins

import (
	"os"
	"path/filepath"
	"testing"
)

func TestDiscoverEmpty(t *testing.T) {
	tmpDir := t.TempDir()
	plugins, err := Discover(tmpDir)
	if err != nil {
		t.Fatalf("Discover on empty dir failed: %v", err)
	}
	if len(plugins) != 0 {
		t.Errorf("expected 0 plugins, got %d", len(plugins))
	}
}

func TestDiscoverWithPlugin(t *testing.T) {
	vaultPath := t.TempDir()
	pluginsDir := filepath.Join(vaultPath, ".agentvault", "plugins", "test-plugin")
	if err := os.MkdirAll(pluginsDir, 0755); err != nil {
		t.Fatalf("mkdir: %v", err)
	}

	manifest := `{"name":"test-plugin","version":"1.0.0","description":"A test plugin","command":"echo","enabled":true}`
	if err := os.WriteFile(filepath.Join(pluginsDir, "plugin.json"), []byte(manifest), 0644); err != nil {
		t.Fatalf("write manifest: %v", err)
	}

	plugins, err := Discover(vaultPath)
	if err != nil {
		t.Fatalf("Discover failed: %v", err)
	}
	if len(plugins) != 1 {
		t.Fatalf("expected 1 plugin, got %d", len(plugins))
	}
	p := plugins[0]
	if p.Manifest.Name != "test-plugin" {
		t.Errorf("name = %q, want test-plugin", p.Manifest.Name)
	}
	if p.Manifest.Version != "1.0.0" {
		t.Errorf("version = %q, want 1.0.0", p.Manifest.Version)
	}
	if !p.Manifest.Enabled {
		t.Error("expected plugin to be enabled")
	}
}

func TestEnabledFilters(t *testing.T) {
	vaultPath := t.TempDir()

	// Create enabled plugin
	dir1 := filepath.Join(vaultPath, ".agentvault", "plugins", "enabled-plugin")
	os.MkdirAll(dir1, 0755)
	os.WriteFile(filepath.Join(dir1, "plugin.json"), []byte(`{"name":"enabled-plugin","version":"1.0","command":"echo","enabled":true}`), 0644)

	// Create disabled plugin
	dir2 := filepath.Join(vaultPath, ".agentvault", "plugins", "disabled-plugin")
	os.MkdirAll(dir2, 0755)
	os.WriteFile(filepath.Join(dir2, "plugin.json"), []byte(`{"name":"disabled-plugin","version":"1.0","command":"echo","enabled":false}`), 0644)

	enabled, err := Enabled(vaultPath)
	if err != nil {
		t.Fatalf("Enabled failed: %v", err)
	}
	if len(enabled) != 1 {
		t.Fatalf("expected 1 enabled plugin, got %d", len(enabled))
	}
	if enabled[0].Manifest.Name != "enabled-plugin" {
		t.Errorf("got %q, want enabled-plugin", enabled[0].Manifest.Name)
	}
}

func TestEnableDisable(t *testing.T) {
	vaultPath := t.TempDir()
	dir := filepath.Join(vaultPath, ".agentvault", "plugins", "toggle-plugin")
	os.MkdirAll(dir, 0755)
	os.WriteFile(filepath.Join(dir, "plugin.json"), []byte(`{"name":"toggle-plugin","version":"1.0","command":"echo","enabled":false}`), 0644)

	if err := Enable(vaultPath, "toggle-plugin"); err != nil {
		t.Fatalf("Enable failed: %v", err)
	}
	enabled, _ := Enabled(vaultPath)
	if len(enabled) != 1 {
		t.Fatal("expected plugin to be enabled after Enable")
	}

	if err := Disable(vaultPath, "toggle-plugin"); err != nil {
		t.Fatalf("Disable failed: %v", err)
	}
	enabled, _ = Enabled(vaultPath)
	if len(enabled) != 0 {
		t.Fatal("expected 0 enabled plugins after Disable")
	}
}

func TestEnableNotFound(t *testing.T) {
	vaultPath := t.TempDir()
	if err := Enable(vaultPath, "nonexistent"); err == nil {
		t.Error("expected error for nonexistent plugin")
	}
}

func TestInstall(t *testing.T) {
	vaultPath := t.TempDir()
	srcDir := t.TempDir()
	os.WriteFile(filepath.Join(srcDir, "plugin.json"), []byte(`{"name":"installed","version":"2.0","command":"echo","enabled":true}`), 0644)

	if err := Install(vaultPath, srcDir); err != nil {
		t.Fatalf("Install failed: %v", err)
	}

	manifestPath := filepath.Join(vaultPath, ".agentvault", "plugins", filepath.Base(srcDir), "plugin.json")
	if _, err := os.Stat(manifestPath); os.IsNotExist(err) {
		t.Error("manifest not copied to plugins dir")
	}
}

func TestInstallNoManifest(t *testing.T) {
	vaultPath := t.TempDir()
	srcDir := t.TempDir()
	if err := Install(vaultPath, srcDir); err == nil {
		t.Error("expected error when no plugin.json exists")
	}
}

func TestDiscoverInvalidJSON(t *testing.T) {
	vaultPath := t.TempDir()
	dir := filepath.Join(vaultPath, ".agentvault", "plugins", "bad-plugin")
	os.MkdirAll(dir, 0755)
	os.WriteFile(filepath.Join(dir, "plugin.json"), []byte(`not json`), 0644)

	plugins, err := Discover(vaultPath)
	if err != nil {
		t.Fatalf("Discover should not error on invalid JSON: %v", err)
	}
	if len(plugins) != 0 {
		t.Errorf("expected 0 plugins with invalid JSON, got %d", len(plugins))
	}
}

func TestDiscoverSkipsHiddenDirs(t *testing.T) {
	vaultPath := t.TempDir()
	dir := filepath.Join(vaultPath, ".agentvault", "plugins", ".hidden-plugin")
	os.MkdirAll(dir, 0755)
	os.WriteFile(filepath.Join(dir, "plugin.json"), []byte(`{"name":"hidden","version":"1.0","command":"echo","enabled":true}`), 0644)

	plugins, _ := Discover(vaultPath)
	if len(plugins) != 0 {
		t.Errorf("expected 0 plugins (hidden dir), got %d", len(plugins))
	}
}

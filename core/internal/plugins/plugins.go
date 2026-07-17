// Package plugins provides MCP-based plugin discovery, management, and proxying.
// Plugins are external MCP servers (stdio or HTTP) that register tools, resources,
// and prompts. They are discovered from .agentvault/plugins/<name>/plugin.json and
// proxied by the AgentVault MCP server so they appear alongside built-in tools.
package plugins

import (
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"sort"
	"strings"
)

// Manifest is the plugin.json schema found in .agentvault/plugins/<name>/.
type Manifest struct {
	Name        string   `json:"name"`
	Version     string   `json:"version"`
	Description string   `json:"description,omitempty"`
	Command     string   `json:"command"`     // executable to launch (stdio MCP)
	Args        []string `json:"args,omitempty"`
	URL         string   `json:"url,omitempty"` // HTTP MCP endpoint (alternative to command)
	Tools       []string `json:"tools,omitempty"`
	Prompts     []string `json:"prompts,omitempty"`
	Resources   []string `json:"resources,omitempty"`
	Enabled     bool     `json:"enabled"`
	Schedule    string   `json:"schedule,omitempty"`    // cron expression, e.g. "0 9 * * *"
	Permissions []string `json:"permissions,omitempty"` // read, write, annotate
}

// Plugin represents a discovered plugin with its manifest.
type Plugin struct {
	Manifest Manifest
	Dir      string // plugin directory path
}

// Discover scans .agentvault/plugins/ for valid plugin.json manifests.
func Discover(vaultPath string) ([]Plugin, error) {
	pluginsDir := filepath.Join(vaultPath, ".agentvault", "plugins")
	entries, err := os.ReadDir(pluginsDir)
	if err != nil {
		if os.IsNotExist(err) {
			return nil, nil
		}
		return nil, fmt.Errorf("reading plugins dir: %w", err)
	}

	var plugins []Plugin
	for _, entry := range entries {
		if !entry.IsDir() || strings.HasPrefix(entry.Name(), ".") {
			continue
		}
		manifestPath := filepath.Join(pluginsDir, entry.Name(), "plugin.json")
		data, err := os.ReadFile(manifestPath)
		if err != nil {
			continue // skip directories without plugin.json
		}
		var m Manifest
		if err := json.Unmarshal(data, &m); err != nil {
			continue
		}
		if m.Name == "" {
			m.Name = entry.Name()
		}
		plugins = append(plugins, Plugin{
			Manifest: m,
			Dir:      filepath.Join(pluginsDir, entry.Name()),
		})
	}

	sort.Slice(plugins, func(i, j int) bool {
		return plugins[i].Manifest.Name < plugins[j].Manifest.Name
	})
	return plugins, nil
}

// Enabled returns only enabled plugins.
func Enabled(vaultPath string) ([]Plugin, error) {
	all, err := Discover(vaultPath)
	if err != nil {
		return nil, err
	}
	var enabled []Plugin
	for _, p := range all {
		if p.Manifest.Enabled {
			enabled = append(enabled, p)
		}
	}
	return enabled, nil
}


// Scheduled returns enabled plugins that have a cron schedule.
func Scheduled(vaultPath string) ([]Plugin, error) {
	enabled, err := Enabled(vaultPath)
	if err != nil {
		return nil, err
	}
	var scheduled []Plugin
	for _, p := range enabled {
		if p.Manifest.Schedule != "" {
			scheduled = append(scheduled, p)
		}
	}
	return scheduled, nil
}

// HasPermission checks if a plugin has a specific permission.
func HasPermission(p Plugin, perm string) bool {
	for _, have := range p.Manifest.Permissions {
		if have == perm || have == "write" && perm == "annotate" {
			return true
		}
	}
	// No permissions listed = full access (backward compat)
	return len(p.Manifest.Permissions) == 0
}
func Enable(vaultPath, name string) error {
	return setEnabled(vaultPath, name, true)
}

// Disable sets the enabled flag to false in the plugin manifest.
func Disable(vaultPath, name string) error {
	return setEnabled(vaultPath, name, false)
}

func setEnabled(vaultPath, name string, enabled bool) error {
	plugins, err := Discover(vaultPath)
	if err != nil {
		return err
	}
	for _, p := range plugins {
		if p.Manifest.Name == name {
			p.Manifest.Enabled = enabled
			data, _ := json.MarshalIndent(p.Manifest, "", "  ")
			return os.WriteFile(filepath.Join(p.Dir, "plugin.json"), append(data, '\n'), 0644)
		}
	}
	return fmt.Errorf("plugin %q not found", name)
}

// Install copies a plugin directory into .agentvault/plugins/.
func Install(vaultPath, srcDir string) error {
	manifestPath := filepath.Join(srcDir, "plugin.json")
	if _, err := os.Stat(manifestPath); os.IsNotExist(err) {
		return fmt.Errorf("no plugin.json found in %s", srcDir)
	}

	name := filepath.Base(srcDir)
	destDir := filepath.Join(vaultPath, ".agentvault", "plugins", name)
	if err := os.MkdirAll(destDir, 0755); err != nil {
		return fmt.Errorf("creating plugin dir: %w", err)
	}

	// Copy manifest
	data, err := os.ReadFile(manifestPath)
	if err != nil {
		return fmt.Errorf("reading manifest: %w", err)
	}
	return os.WriteFile(filepath.Join(destDir, "plugin.json"), data, 0644)
}

// Package importers provides import functionality for external note formats.
package importers

import (
	"fmt"
	"os"
	"path/filepath"
	"strings"
	"sync"
)

// Importer is the interface for all importers.
type Importer interface {
	Name() string
	Description() string
	Import(opts ImportOptions) (*ImportResult, error)
}

// ImportOptions controls import behavior.
type ImportOptions struct {
	SourcePath     string   // Path to source data
	TargetVault    string   // Path to AgentVault
	Mode           string   // "copy" (default), "in-place", "normalize"
	KeepStructure  bool     // Preserve relative folder structure
	DefaultProject string   // Project to assign
	Tags           []string // Tags to add
}

// ImportResult reports what was imported.
type ImportResult struct {
	FilesImported int
	FilesSkipped  int
	Errors        []ImportError
	Warnings      []string
}

// ImportError records a file that failed to import.
type ImportError struct {
	Path  string
	Error string
}

// String returns a human-readable summary of the import result.
func (r *ImportResult) String() string {
	var b strings.Builder
	fmt.Fprintf(&b, "Imported: %d, Skipped: %d", r.FilesImported, r.FilesSkipped)
	if len(r.Errors) > 0 {
		fmt.Fprintf(&b, ", Errors: %d", len(r.Errors))
	}
	if len(r.Warnings) > 0 {
		fmt.Fprintf(&b, ", Warnings: %d", len(r.Warnings))
	}
	return b.String()
}

// Registry of available importers.
var (
	registry     = map[string]Importer{}
	registryLock sync.RWMutex
)

// Register adds an importer to the registry.
func Register(importer Importer) {
	registryLock.Lock()
	defer registryLock.Unlock()
	registry[importer.Name()] = importer
}

// Get retrieves an importer by name.
func Get(name string) (Importer, bool) {
	registryLock.RLock()
	defer registryLock.RUnlock()
	imp, ok := registry[name]
	return imp, ok
}

// List returns all registered importers.
func List() []Importer {
	registryLock.RLock()
	defer registryLock.RUnlock()
	list := make([]Importer, 0, len(registry))
	for _, imp := range registry {
		list = append(list, imp)
	}
	return list
}

// Available returns a comma-separated list of available importer names.
func Available() string {
	names := make([]string, 0, len(registry))
	for name := range registry {
		names = append(names, name)
	}
	return strings.Join(names, ", ")
}

// IsHiddenDir reports whether a directory name should be skipped during import.
func IsHiddenDir(name string) bool {
	return strings.HasPrefix(name, ".") && name != "."
}

// GenerateID creates a unique note ID based on filename and timestamp.
func GenerateID(prefix, filename string) string {
	// Simple ID: prefix_YYYYMMDD_filename
	clean := strings.ReplaceAll(filename, " ", "-")
	clean = strings.ReplaceAll(clean, "_", "-")
	clean = strings.ToLower(clean)
	return fmt.Sprintf("%s_%s", prefix, clean)
}

// CollisionSafePath ensures the target path doesn't collide with existing files.
// If a file exists, appends _1, _2, etc.
func CollisionSafePath(targetPath string) string {
	if _, err := os.Stat(targetPath); os.IsNotExist(err) {
		return targetPath
	}
	ext := filepath.Ext(targetPath)
	base := strings.TrimSuffix(targetPath, ext)
	for i := 1; i < 1000; i++ {
		candidate := fmt.Sprintf("%s_%d%s", base, i, ext)
		if _, err := os.Stat(candidate); os.IsNotExist(err) {
			return candidate
		}
	}
	return targetPath // fallback, shouldn't happen
}

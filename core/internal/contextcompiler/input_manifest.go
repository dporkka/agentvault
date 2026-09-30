package contextcompiler

import (
	"encoding/json"
	"errors"
	"fmt"
	"os"
	"path/filepath"
	"sort"
	"strings"

	"github.com/agentvault/core/internal/contract"
	"github.com/agentvault/core/internal/fileversion"
)

var ErrInputManifestHashMismatch = errors.New("context input manifest hash mismatch")

func (c *Compiler) buildInputManifest(req contract.CompileContextRequest, items []contract.ContextItem, scope *savedViewScope) (*contract.ContextInputManifest, error) {
	if scope == nil {
		if strings.TrimSpace(req.ExpectedInputManifestHash) != "" {
			return nil, fmt.Errorf("expectedInputManifestHash requires viewId")
		}
		return nil, nil
	}

	sources := make([]contract.ContextInputSource, 0)
	seen := make(map[string]struct{})
	for _, item := range items {
		if !isFileBackedContextItem(item) {
			continue
		}
		if strings.TrimSpace(item.Path) == "" {
			return nil, fmt.Errorf("file-backed context item %s:%s has no path", item.Kind, item.ID)
		}
		hash, err := c.hashVaultFile(item.Path)
		if err != nil {
			return nil, fmt.Errorf("hash context input %s:%s: %w", item.Kind, item.ID, err)
		}
		source := contract.ContextInputSource{
			Kind:        item.Kind,
			ID:          item.ID,
			Path:        filepath.ToSlash(filepath.Clean(item.Path)),
			ContentHash: hash,
		}
		key := source.Kind + "\x00" + source.ID + "\x00" + source.Path
		if _, ok := seen[key]; ok {
			continue
		}
		seen[key] = struct{}{}
		sources = append(sources, source)
	}

	sort.Slice(sources, func(i, j int) bool {
		if sources[i].Kind != sources[j].Kind {
			return sources[i].Kind < sources[j].Kind
		}
		if sources[i].ID != sources[j].ID {
			return sources[i].ID < sources[j].ID
		}
		return sources[i].Path < sources[j].Path
	})

	manifest := &contract.ContextInputManifest{
		Version:         "1",
		ViewID:          scope.id,
		ViewContentHash: scope.contentHash,
		Sources:         sources,
	}
	hash, err := hashInputManifest(*manifest)
	if err != nil {
		return nil, err
	}
	manifest.ManifestHash = hash

	expected := strings.TrimSpace(req.ExpectedInputManifestHash)
	if expected != "" && !strings.EqualFold(expected, manifest.ManifestHash) {
		return nil, fmt.Errorf("%w: expected %s but compiled %s", ErrInputManifestHashMismatch, expected, manifest.ManifestHash)
	}
	return manifest, nil
}

func isFileBackedContextItem(item contract.ContextItem) bool {
	switch item.Kind {
	case "note":
		return true
	case "memory":
		source, _ := item.Metadata["memorySource"].(string)
		return source == "markdown"
	default:
		return false
	}
}

func (c *Compiler) hashVaultFile(path string) (string, error) {
	vault := strings.TrimSpace(c.vaultPath)
	if vault == "" {
		return "", fmt.Errorf("vault path is not configured")
	}
	absVault, err := filepath.Abs(vault)
	if err != nil {
		return "", fmt.Errorf("resolve vault path: %w", err)
	}
	absTarget, err := filepath.Abs(filepath.Join(absVault, filepath.FromSlash(path)))
	if err != nil {
		return "", fmt.Errorf("resolve input path: %w", err)
	}
	rel, err := filepath.Rel(absVault, absTarget)
	if err != nil {
		return "", fmt.Errorf("resolve input path relative to vault: %w", err)
	}
	if rel == ".." || strings.HasPrefix(rel, ".."+string(filepath.Separator)) || filepath.IsAbs(rel) {
		return "", fmt.Errorf("context input path escapes vault: %s", path)
	}
	content, err := os.ReadFile(absTarget)
	if err != nil {
		return "", fmt.Errorf("read %s: %w", path, err)
	}
	return fileversion.Hash(content), nil
}

func hashInputManifest(manifest contract.ContextInputManifest) (string, error) {
	payload := struct {
		Version         string                        `json:"version"`
		ViewID          string                        `json:"viewId"`
		ViewContentHash string                        `json:"viewContentHash"`
		Sources         []contract.ContextInputSource `json:"sources"`
	}{
		Version:         manifest.Version,
		ViewID:          manifest.ViewID,
		ViewContentHash: manifest.ViewContentHash,
		Sources:         manifest.Sources,
	}
	data, err := json.Marshal(payload)
	if err != nil {
		return "", fmt.Errorf("encode context input manifest: %w", err)
	}
	return fileversion.Hash(data), nil
}

package capturekey

import (
	"crypto/sha256"
	"encoding/hex"
	"errors"
	"fmt"
	"os"
	"path/filepath"
	"sort"
	"strings"

	"github.com/agentvault/core/internal/markdown"
)

// Duplicate records only a SHA-256 key fingerprint, never a raw external_id.
// Paths are vault-relative and slash-separated for portable CLI output.
type Duplicate struct {
	KeyDigest string   `json:"key_digest"`
	Paths     []string `json:"paths"`
}

type AuditReport struct {
	KeyedCaptures    int         `json:"keyed_captures"`
	Duplicates       []Duplicate `json:"duplicates"`
	InvalidCanonical []string    `json:"invalid_canonical"`
	MalformedFiles   []string    `json:"malformed_files"`
}

// AuditInbox is a read-only reconciliation preflight. It neither chooses a
// winner for duplicates nor alters user-owned Markdown or stale staging files.
// A successful audit describes the inbox at one instant; the caller must
// coordinate writers during migration and cannot infer exclusive access.
func AuditInbox(vaultPath string) (AuditReport, error) {
	report := AuditReport{}
	root, err := filepath.Abs(vaultPath)
	if err != nil {
		return report, fmt.Errorf("resolve vault: %w", err)
	}
	root, err = filepath.EvalSymlinks(root)
	if err != nil {
		return report, fmt.Errorf("resolve vault symlinks: %w", err)
	}
	inbox := filepath.Join(root, "00-inbox")
	info, err := os.Lstat(inbox)
	switch {
	case errors.Is(err, os.ErrNotExist):
		return report, nil
	case err != nil:
		return report, fmt.Errorf("inspect capture inbox: %w", err)
	case !info.IsDir() || info.Mode()&os.ModeSymlink != 0:
		return report, fmt.Errorf("%w: inbox is not a real directory", ErrConflict)
	}

	entries, err := os.ReadDir(inbox)
	if err != nil {
		return report, fmt.Errorf("read capture inbox: %w", err)
	}
	byKey := make(map[string][]string)
	for _, entry := range entries {
		name := entry.Name()
		if !strings.EqualFold(filepath.Ext(name), ".md") {
			continue
		}
		rel := filepath.ToSlash(filepath.Join("00-inbox", name))
		isCanonical := strings.HasPrefix(name, "ext_")
		// Lstat rather than entry.Info: neither the canonical target nor
		// an arbitrary legacy symlink may be followed outside the vault.
		st, err := os.Lstat(filepath.Join(inbox, name))
		if err != nil {
			return report, fmt.Errorf("inspect %s: %w", rel, err)
		}
		if !st.Mode().IsRegular() {
			report.MalformedFiles = append(report.MalformedFiles, rel)
			if isCanonical {
				report.InvalidCanonical = append(report.InvalidCanonical, rel)
			}
			continue
		}
		doc, err := markdown.ParseFile(filepath.Join(inbox, name))
		if err != nil || doc.RawFrontmatter == "" {
			report.MalformedFiles = append(report.MalformedFiles, rel)
			if isCanonical {
				report.InvalidCanonical = append(report.InvalidCanonical, rel)
			}
			continue
		}
		rawKey, present := doc.Frontmatter.Extra["external_id"]
		key, hasKey := rawKey.(string)
		valid := doc.Frontmatter.Type == "capture" && doc.Frontmatter.ID != "" &&
			hasKey && validateKey(key) == nil
		// A legacy YAML number/bool or invalid key has ambiguous string
		// identity. Never emit a green rollout audit that silently ignores it.
		if present && !valid {
			report.MalformedFiles = append(report.MalformedFiles, rel)
		}
		if isCanonical && (!valid || CanonicalPath(key) != rel) {
			report.InvalidCanonical = append(report.InvalidCanonical, rel)
		}
		if !valid {
			continue
		}
		report.KeyedCaptures++
		byKey[key] = append(byKey[key], rel)
	}
	for key, paths := range byKey {
		if len(paths) < 2 {
			continue
		}
		sort.Strings(paths)
		sum := sha256.Sum256([]byte(key))
		report.Duplicates = append(report.Duplicates, Duplicate{
			KeyDigest: hex.EncodeToString(sum[:]), Paths: paths,
		})
	}
	sort.Slice(report.Duplicates, func(i, j int) bool {
		return report.Duplicates[i].KeyDigest < report.Duplicates[j].KeyDigest
	})
	sort.Strings(report.InvalidCanonical)
	sort.Strings(report.MalformedFiles)
	return report, nil
}

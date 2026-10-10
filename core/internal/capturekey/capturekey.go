// Package capturekey provides filesystem-authoritative idempotency for
// external-ID captures. It does not promise eternal deduplication after a
// user moves/deletes a capture, or coordinate with old-version writers.
package capturekey

import (
	"crypto/sha256"
	"encoding/hex"
	"errors"
	"fmt"
	"io/fs"
	"os"
	"path/filepath"
	"strings"
	"unicode"
	"unicode/utf8"

	"github.com/agentvault/core/internal/atomicfile"
	"github.com/agentvault/core/internal/markdown"
)

var (
	ErrConflict   = errors.New("capture idempotency conflict")
	ErrInvalidKey = errors.New("invalid capture external_id")
)

const maxKeyBytes = 512

type Result struct {
	// Path is always a slash-separated vault-relative path.
	Path    string
	Created bool
}

// CanonicalPath is the deterministic, portable vault-relative filename for
// nonempty keyed captures. Callers must validate keys with Publish first.
func CanonicalPath(externalID string) string {
	sum := sha256.Sum256([]byte(externalID))
	return "00-inbox/ext_" + hex.EncodeToString(sum[:]) + ".md"
}

func validateKey(key string) error {
	if key == "" || len(key) > maxKeyBytes || !utf8.ValidString(key) {
		return fmt.Errorf("%w: external_id must be 1–%d valid UTF-8 bytes", ErrInvalidKey, maxKeyBytes)
	}
	for _, r := range key {
		if unicode.IsControl(r) {
			return fmt.Errorf("%w: control characters are not supported", ErrInvalidKey)
		}
	}
	return nil
}

func matchingCapture(path, key string) (bool, error) {
	data, err := os.ReadFile(path)
	if err != nil {
		return false, err
	}
	doc, err := markdown.ParseBytes(data)
	if err != nil || doc.RawFrontmatter == "" || doc.Frontmatter.Type != "capture" || doc.Frontmatter.ID == "" {
		return false, nil
	}
	val, isString := doc.Frontmatter.Extra["external_id"].(string)
	return isString && val == key, nil
}

func scanExisting(inbox, key string) (string, error) {
	entries, err := os.ReadDir(inbox)
	if err != nil {
		return "", fmt.Errorf("scan capture inbox: %w", err)
	}
	var match string
	for _, entry := range entries {
		if entry.IsDir() || entry.Type()&os.ModeSymlink != 0 || !strings.EqualFold(filepath.Ext(entry.Name()), ".md") {
			continue
		}
		info, err := entry.Info()
		if err != nil {
			return "", fmt.Errorf("inspect legacy capture %s: %w", entry.Name(), err)
		}
		if !info.Mode().IsRegular() {
			continue
		}
		found, err := matchingCapture(filepath.Join(inbox, entry.Name()), key)
		if err != nil {
			return "", fmt.Errorf("read legacy capture %s: %w", entry.Name(), err)
		}
		if found {
			if match != "" {
				return "", fmt.Errorf("%w: multiple captures have this external_id (%s, %s)", ErrConflict, match, entry.Name())
			}
			match = entry.Name()
		}
	}
	return match, nil
}

func validateCanonical(path, key string) error {
	info, err := os.Lstat(path)
	if err != nil {
		return fmt.Errorf("inspect canonical capture: %w", err)
	}
	if !info.Mode().IsRegular() {
		return fmt.Errorf("%w: canonical destination is not a regular file", ErrConflict)
	}
	found, err := matchingCapture(path, key)
	if err != nil {
		return fmt.Errorf("%w: cannot read canonical capture: %v", ErrConflict, err)
	}
	if !found {
		return fmt.Errorf("%w: canonical destination does not contain a complete matching capture", ErrConflict)
	}
	return nil
}

// Publish deduplicates a keyed capture against legacy Markdown, then installs
// the completed new note exclusively at a stable key-derived filename.
// Independently running processes cannot replace a winner at that filename.
// The caller must supply valid capture Markdown with the exact external ID.
func Publish(vaultPath, externalID string, content []byte) (Result, error) {
	if err := validateKey(externalID); err != nil {
		return Result{}, err
	}
	doc, err := markdown.ParseBytes(content)
	if err != nil || doc.RawFrontmatter == "" || doc.Frontmatter.Type != "capture" || doc.Frontmatter.ID == "" {
		return Result{}, fmt.Errorf("%w: publish requires complete capture frontmatter", ErrInvalidKey)
	}
	if key, ok := doc.Frontmatter.Extra["external_id"].(string); !ok || key != externalID {
		return Result{}, fmt.Errorf("%w: capture content external_id must exactly match requested key", ErrInvalidKey)
	}

	root, err := filepath.Abs(vaultPath)
	if err != nil {
		return Result{}, fmt.Errorf("resolve vault: %w", err)
	}
	root, err = filepath.EvalSymlinks(root)
	if err != nil {
		return Result{}, fmt.Errorf("resolve vault symlinks: %w", err)
	}
	inbox := filepath.Join(root, "00-inbox")
	info, err := os.Lstat(inbox)
	switch {
	case errors.Is(err, os.ErrNotExist):
		if err := os.Mkdir(inbox, 0755); err != nil && !errors.Is(err, fs.ErrExist) {
			return Result{}, fmt.Errorf("create inbox: %w", err)
		}
	case err != nil:
		return Result{}, fmt.Errorf("inspect inbox: %w", err)
	case !info.IsDir():
		return Result{}, fmt.Errorf("%w: inbox is not a real directory", ErrConflict)
	}
	// Recheck the path following Mkdir and before opening any child. The
	// guarantee assumes no hostile actor swaps directory components after
	// this check; portable dirfd-level path authority is out of scope.
	info, err = os.Lstat(inbox)
	if err != nil || !info.IsDir() || info.Mode()&os.ModeSymlink != 0 {
		return Result{}, fmt.Errorf("%w: inbox is missing, replaced, or symlinked", ErrConflict)
	}

	name, err := scanExisting(inbox, externalID)
	if err != nil {
		return Result{}, err
	}
	if name != "" {
		return Result{Path: filepath.ToSlash(filepath.Join("00-inbox", name))}, nil
	}

	relPath := CanonicalPath(externalID)
	path := filepath.Join(root, filepath.FromSlash(relPath))
	err = atomicfile.WriteNewFile(path, content, 0644)
	switch {
	case err == nil:
		return Result{Path: relPath, Created: true}, nil
	case errors.Is(err, fs.ErrExist):
		if err := validateCanonical(path, externalID); err != nil {
			return Result{}, err
		}
		return Result{Path: relPath}, nil
	default:
		return Result{}, fmt.Errorf("publish canonical capture: %w", err)
	}
}

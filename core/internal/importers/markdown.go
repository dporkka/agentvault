// Package importers provides import functionality for external note formats.
package importers

import (
	"crypto/sha256"
	"fmt"
	"io"
	"os"
	"path/filepath"
	"regexp"
	"strings"
	"time"

	"github.com/agentvault/core/internal/markdown"
)

// MarkdownImporter imports a folder of Markdown files into AgentVault.
type MarkdownImporter struct{}

// Name returns the importer name.
func (m *MarkdownImporter) Name() string { return "markdown" }

// Description returns the importer description.
func (m *MarkdownImporter) Description() string { return "Import a folder of Markdown files" }

// Import walks the source directory for .md files and copies them to the target vault.
func (m *MarkdownImporter) Import(opts ImportOptions) (*ImportResult, error) {
	result := &ImportResult{}

	// Validate source path
	info, err := os.Stat(opts.SourcePath)
	if err != nil {
		if os.IsNotExist(err) {
			return nil, fmt.Errorf("source path does not exist: %s", opts.SourcePath)
		}
		return nil, fmt.Errorf("failed to access source path: %w", err)
	}
	if !info.IsDir() {
		return nil, fmt.Errorf("source path is not a directory: %s", opts.SourcePath)
	}

	// Ensure target vault exists
	if err := os.MkdirAll(opts.TargetVault, 0755); err != nil {
		return nil, fmt.Errorf("failed to create target vault: %w", err)
	}

	// Walk source directory for markdown files
	err = filepath.Walk(opts.SourcePath, func(path string, info os.FileInfo, err error) error {
		if err != nil {
			result.Errors = append(result.Errors, ImportError{Path: path, Error: err.Error()})
			return nil // continue walking
		}

		// Skip directories
		if info.IsDir() {
			// Skip hidden directories
			if IsHiddenDir(info.Name()) {
				return filepath.SkipDir
			}
			return nil
		}

		// Only process .md and .markdown files
		ext := strings.ToLower(filepath.Ext(path))
		if ext != ".md" && ext != ".markdown" {
			return nil
		}

		// Import the file
		if err := m.importFile(path, opts, result); err != nil {
			result.Errors = append(result.Errors, ImportError{Path: path, Error: err.Error()})
		}

		return nil
	})

	if err != nil {
		result.Errors = append(result.Errors, ImportError{Path: opts.SourcePath, Error: err.Error()})
	}

	return result, nil
}

// importFile imports a single markdown file into the vault.
func (m *MarkdownImporter) importFile(sourcePath string, opts ImportOptions, result *ImportResult) error {
	// Parse the markdown file
	doc, err := markdown.ParseFile(sourcePath)
	if err != nil {
		result.FilesSkipped++
		return fmt.Errorf("failed to parse markdown: %w", err)
	}

	// Normalize frontmatter if requested
	if opts.Mode == "normalize" {
		normalizeFrontmatter(doc, opts)
	}

	// Determine base target path (before collision safety)
	baseTargetPath, err := m.determineBaseTargetPath(sourcePath, opts)
	if err != nil {
		return fmt.Errorf("failed to determine target path: %w", err)
	}

	// Check for existing file with same content at the base path (skip if identical)
	if skip, err := shouldSkipIfDuplicate(sourcePath, baseTargetPath); err != nil {
		return err
	} else if skip {
		result.FilesSkipped++
		return nil
	}

	// Apply collision safety for files with different content
	targetPath := CollisionSafePath(baseTargetPath)

	// Ensure target directory exists
	targetDir := filepath.Dir(targetPath)
	if err := os.MkdirAll(targetDir, 0755); err != nil {
		return fmt.Errorf("failed to create target directory: %w", err)
	}

	// Write the (potentially normalized) file
	content := renderDocument(doc)
	if err := os.WriteFile(targetPath, []byte(content), 0644); err != nil {
		return fmt.Errorf("failed to write file: %w", err)
	}

	result.FilesImported++
	return nil
}

// determineBaseTargetPath decides the base target path without collision safety.
func (m *MarkdownImporter) determineBaseTargetPath(sourcePath string, opts ImportOptions) (string, error) {
	filename := filepath.Base(sourcePath)

	if opts.KeepStructure {
		// Preserve relative structure from source root
		relPath, err := filepath.Rel(opts.SourcePath, sourcePath)
		if err != nil {
			return "", err
		}
		return filepath.Join(opts.TargetVault, "10-notes", relPath), nil
	}

	// Flat: put into 10-notes/
	return filepath.Join(opts.TargetVault, "10-notes", filename), nil
}

// normalizeFrontmatter ensures required fields exist in the document frontmatter.
func normalizeFrontmatter(doc *markdown.ParsedDocument, opts ImportOptions) {
	now := time.Now().UTC().Format("2006-01-02T15:04:05Z")

	if doc.Frontmatter.ID == "" {
		doc.Frontmatter.ID = GenerateID("note", doc.Frontmatter.Title)
	}
	if doc.Frontmatter.Type == "" {
		doc.Frontmatter.Type = "note"
	}
	if doc.Frontmatter.Title == "" {
		// Extract title from first heading or use filename
		doc.Frontmatter.Title = extractTitle(doc.Body)
	}
	if doc.Frontmatter.Created == "" {
		doc.Frontmatter.Created = now
	}
	if doc.Frontmatter.Updated == "" {
		doc.Frontmatter.Updated = now
	}
	if opts.DefaultProject != "" && doc.Frontmatter.Project == "" {
		doc.Frontmatter.Project = opts.DefaultProject
	}
	if len(opts.Tags) > 0 {
		// Merge provided tags with existing ones (avoid duplicates)
		doc.Frontmatter.Tags = mergeTags(doc.Frontmatter.Tags, opts.Tags)
	}
}

// extractTitle extracts a title from the document body (first # heading).
var headingRe = regexp.MustCompile(`^#\s+(.+)$`)

func extractTitle(body string) string {
	lines := strings.Split(body, "\n")
	for _, line := range lines {
		line = strings.TrimSpace(line)
		if matches := headingRe.FindStringSubmatch(line); matches != nil {
			return strings.TrimSpace(matches[1])
		}
	}
	return "Untitled"
}

// mergeTags merges two tag slices, avoiding duplicates.
func mergeTags(existing, newTags []string) []string {
	seen := make(map[string]bool)
	for _, t := range existing {
		seen[t] = true
	}
	for _, t := range newTags {
		if !seen[t] {
			seen[t] = true
			existing = append(existing, t)
		}
	}
	return existing
}

// renderDocument serializes a ParsedDocument back to markdown with frontmatter.
func renderDocument(doc *markdown.ParsedDocument) string {
	var b strings.Builder

	b.WriteString("---\n")

	// Write core fields in order
	if doc.Frontmatter.ID != "" {
		fmt.Fprintf(&b, "id: %s\n", doc.Frontmatter.ID)
	}
	if doc.Frontmatter.Type != "" {
		fmt.Fprintf(&b, "type: %s\n", doc.Frontmatter.Type)
	}
	if doc.Frontmatter.Title != "" {
		fmt.Fprintf(&b, "title: %s\n", doc.Frontmatter.Title)
	}
	if doc.Frontmatter.Status != "" {
		fmt.Fprintf(&b, "status: %s\n", doc.Frontmatter.Status)
	}
	if doc.Frontmatter.Project != "" {
		fmt.Fprintf(&b, "project: %s\n", doc.Frontmatter.Project)
	}
	if len(doc.Frontmatter.Tags) > 0 {
		b.WriteString("tags:\n")
		for _, t := range doc.Frontmatter.Tags {
			fmt.Fprintf(&b, "  - %s\n", t)
		}
	}
	if len(doc.Frontmatter.Entities) > 0 {
		b.WriteString("entities:\n")
		for _, e := range doc.Frontmatter.Entities {
			fmt.Fprintf(&b, "  - %s\n", e)
		}
	}
	if doc.Frontmatter.Created != "" {
		fmt.Fprintf(&b, "created: %s\n", doc.Frontmatter.Created)
	}
	if doc.Frontmatter.Updated != "" {
		fmt.Fprintf(&b, "updated: %s\n", doc.Frontmatter.Updated)
	}
	if doc.Frontmatter.SourceQuality != "" {
		fmt.Fprintf(&b, "source_quality: %s\n", doc.Frontmatter.SourceQuality)
	}

	// Write extra fields
	for key, val := range doc.Frontmatter.Extra {
		// Skip fields we've already handled
		switch key {
		case "id", "type", "title", "status", "project", "tags", "entities", "created", "updated", "source_quality":
			continue
		}
		fmt.Fprintf(&b, "%s: %v\n", key, val)
	}

	b.WriteString("---\n\n")
	b.WriteString(doc.Body)

	return b.String()
}

// ComputeHash returns the SHA-256 hex digest of file content.
func ComputeHash(content []byte) string {
	h := sha256.Sum256(content)
	return fmt.Sprintf("%x", h)
}

// shouldSkipIfDuplicate checks if the target file exists with the same content.
func shouldSkipIfDuplicate(sourcePath, targetPath string) (bool, error) {
	srcInfo, err := os.Stat(sourcePath)
	if err != nil {
		return false, err
	}
	tgtInfo, err := os.Stat(targetPath)
	if err != nil {
		if os.IsNotExist(err) {
			return false, nil // target doesn't exist, don't skip
		}
		return false, err
	}

	// Quick size check
	if srcInfo.Size() != tgtInfo.Size() {
		return false, nil // different sizes, don't skip
	}

	// Full hash comparison
	srcContent, err := os.ReadFile(sourcePath)
	if err != nil {
		return false, err
	}
	tgtContent, err := os.ReadFile(targetPath)
	if err != nil {
		return false, err
	}

	if ComputeHash(srcContent) == ComputeHash(tgtContent) {
		return true, nil // identical content, skip
	}
	return false, nil
}

// copyFile copies a file from src to dst.
func copyFile(src, dst string) error {
	in, err := os.Open(src)
	if err != nil {
		return err
	}
	defer in.Close()

	if err := os.MkdirAll(filepath.Dir(dst), 0755); err != nil {
		return err
	}

	out, err := os.Create(dst)
	if err != nil {
		return err
	}
	defer out.Close()

	if _, err := io.Copy(out, in); err != nil {
		return err
	}
	return out.Close()
}

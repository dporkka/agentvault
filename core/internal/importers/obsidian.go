// Package importers provides import functionality for external note formats.
package importers

import (
	"fmt"
	"os"
	"path/filepath"
	"regexp"
	"strings"

	"github.com/agentvault/core/internal/markdown"
)

// ObsidianImporter imports an Obsidian vault into AgentVault.
type ObsidianImporter struct{}

// Name returns the importer name.
func (o *ObsidianImporter) Name() string { return "obsidian" }

// Description returns the importer description.
func (o *ObsidianImporter) Description() string { return "Import an Obsidian vault" }

// Import walks the Obsidian vault and imports all markdown files with Obsidian-specific handling.
func (o *ObsidianImporter) Import(opts ImportOptions) (*ImportResult, error) {
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

	// Create attachments folder
	attachmentsDir := filepath.Join(opts.TargetVault, "10-notes", "attachments")
	if err := os.MkdirAll(attachmentsDir, 0755); err != nil {
		return nil, fmt.Errorf("failed to create attachments directory: %w", err)
	}

	// Walk source directory
	err = filepath.Walk(opts.SourcePath, func(path string, info os.FileInfo, err error) error {
		if err != nil {
			result.Errors = append(result.Errors, ImportError{Path: path, Error: err.Error()})
			return nil // continue walking
		}

		// Skip directories
		if info.IsDir() {
			if isObsidianDir(info.Name()) {
				return filepath.SkipDir
			}
			if IsHiddenDir(info.Name()) {
				return filepath.SkipDir
			}
			return nil
		}

		// Handle attachments (copy to vault)
		ext := strings.ToLower(filepath.Ext(path))
		if isAttachment(ext) {
			targetPath := filepath.Join(attachmentsDir, filepath.Base(path))
			targetPath = CollisionSafePath(targetPath)
			if err := copyFile(path, targetPath); err != nil {
				result.Warnings = append(result.Warnings,
					fmt.Sprintf("failed to copy attachment %s: %v", path, err))
			}
			return nil
		}

		// Only process .md and .markdown files
		if ext != ".md" && ext != ".markdown" {
			return nil
		}

		// Import the markdown file with Obsidian-specific handling
		if err := o.importObsidianFile(path, opts, result); err != nil {
			result.Errors = append(result.Errors, ImportError{Path: path, Error: err.Error()})
		}

		return nil
	})

	if err != nil {
		result.Errors = append(result.Errors, ImportError{Path: opts.SourcePath, Error: err.Error()})
	}

	return result, nil
}

// importObsidianFile imports a single Obsidian markdown file with special handling.
func (o *ObsidianImporter) importObsidianFile(sourcePath string, opts ImportOptions, result *ImportResult) error {
	// Parse the markdown file
	doc, err := markdown.ParseFile(sourcePath)
	if err != nil {
		result.FilesSkipped++
		return fmt.Errorf("failed to parse markdown: %w", err)
	}

	// Extract inline tags from body and add to frontmatter
	inlineTags := extractInlineTags(doc.Body)
	if len(inlineTags) > 0 {
		doc.Frontmatter.Tags = mergeTags(doc.Frontmatter.Tags, inlineTags)
	}

	// Convert Obsidian aliases to entities
	if aliases := extractAliases(doc); len(aliases) > 0 {
		doc.Frontmatter.Entities = mergeTags(doc.Frontmatter.Entities, aliases)
	}

	// Normalize frontmatter
	if opts.Mode == "normalize" {
		normalizeFrontmatter(doc, opts)
	}

	// Convert Obsidian wiki links to AgentVault format (already [[Target]] format, so they work)
	// Just ensure they are preserved - no conversion needed since AgentVault supports [[...]]

	// Determine base target path (before collision safety)
	baseTargetPath, err := o.determineBaseTargetPath(sourcePath, opts)
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

	// Write the file
	content := renderDocument(doc)
	if err := os.WriteFile(targetPath, []byte(content), 0644); err != nil {
		return fmt.Errorf("failed to write file: %w", err)
	}

	result.FilesImported++
	return nil
}

// determineBaseTargetPath decides the base target path without collision safety.
func (o *ObsidianImporter) determineBaseTargetPath(sourcePath string, opts ImportOptions) (string, error) {
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

// isObsidianDir checks if a directory is an Obsidian-specific folder that should be skipped.
func isObsidianDir(name string) bool {
	switch name {
	case ".obsidian", ".trash", ".git", ".svn":
		return true
	default:
		return false
	}
}

// isAttachment checks if a file extension is an attachment type.
func isAttachment(ext string) bool {
	switch ext {
	case ".png", ".jpg", ".jpeg", ".gif", ".bmp", ".svg",
		".pdf", ".doc", ".docx", ".xls", ".xlsx",
		".ppt", ".pptx", ".mp3", ".mp4", ".wav", ".ogg",
		".zip", ".tar", ".gz", ".7z",
		".epub", ".mobi":
		return true
	default:
		return false
	}
}

// inlineTagRe matches #tag-style inline tags in markdown body.
var inlineTagRe = regexp.MustCompile(`(?:^|\s)#([a-zA-Z][a-zA-Z0-9_-]*)`)

// extractInlineTags finds all #inline-tag style tags in the body.
func extractInlineTags(body string) []string {
	matches := inlineTagRe.FindAllStringSubmatch(body, -1)
	tags := make([]string, 0, len(matches))
	seen := make(map[string]bool)
	for _, m := range matches {
		tag := m[1]
		// Skip hex color codes
		if isHexColor(tag) {
			continue
		}
		if !seen[tag] {
			seen[tag] = true
			tags = append(tags, tag)
		}
	}
	return tags
}

// isHexColor checks if a string looks like a hex color code.
func isHexColor(s string) bool {
	if len(s) != 3 && len(s) != 6 {
		return false
	}
	for _, c := range s {
		if !((c >= '0' && c <= '9') || (c >= 'a' && c <= 'f') || (c >= 'A' && c <= 'F')) {
			return false
		}
	}
	return true
}

// extractAliases extracts aliases from Obsidian frontmatter.
func extractAliases(doc *markdown.ParsedDocument) []string {
	var aliases []string

	// Check Extra map for "aliases" key
	if val, ok := doc.Frontmatter.Extra["aliases"]; ok {
		switch v := val.(type) {
		case string:
			if v != "" {
				aliases = append(aliases, v)
			}
		case []interface{}:
			for _, a := range v {
				if s, ok := a.(string); ok && s != "" {
					aliases = append(aliases, s)
				}
			}
		}
	}

	return aliases
}



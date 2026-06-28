package main

import (
	"fmt"
	"os"
	"path/filepath"
	"strings"
	"time"

	"github.com/agentvault/core/internal/git"
	"github.com/agentvault/core/internal/templates"
	"github.com/agentvault/core/internal/vault"
	"github.com/spf13/cobra"
)

var (
	newTitle   string
	newProject string
	newTags    string
	newURL     string
	newCommit  bool
)

// timeNowFunc is overridable for testing.
var timeNowFunc = func() time.Time {
	return time.Now().UTC()
}

// newCmd is the parent command for creating new notes.
var newCmd = &cobra.Command{
	Use:   "new <type>",
	Short: "Create a new structured note",
	Long: `Create a new structured note from a template.

Supported types:
  note      General note
  decision  Decision record
  task      Task with acceptance criteria
  meeting   Meeting notes
  source    Source capture (article, book, etc.)

Examples:
  agentvault new note --title "My Idea"
  agentvault new decision --project myproject --title "Use Postgres"
  agentvault new task --project myproject --title "Build API"
  agentvault new meeting --project myproject --title "Sprint Planning"
  agentvault new source --title "Article" --url "https://example.com"`,
	Args: cobra.ExactArgs(1),
	RunE: runNew,
}

func init() {
	rootCmd.AddCommand(newCmd)

	newCmd.Flags().StringVar(&newTitle, "title", "", "Note title (required)")
	newCmd.Flags().StringVar(&newProject, "project", "", "Project name")
	newCmd.Flags().StringVar(&newTags, "tags", "", "Comma-separated tags")
	newCmd.Flags().StringVar(&newURL, "url", "", "Source URL (for source type)")

	_ = newCmd.MarkFlagRequired("title")

	newCmd.Flags().BoolVar(&newCommit, "commit", false, "Automatically git-commit the new note")
}

// sanitizeFilename creates a safe filename from a title.
func sanitizeFilename(title string) string {
	// Convert to lowercase and replace spaces/special chars with hyphens
	safe := strings.ToLower(title)
	safe = strings.ReplaceAll(safe, " ", "-")

	// Remove non-alphanumeric characters except hyphens
	var result strings.Builder
	for _, r := range safe {
		if (r >= 'a' && r <= 'z') || (r >= '0' && r <= '9') || r == '-' {
			result.WriteRune(r)
		}
	}

	// Collapse multiple hyphens
	safe = result.String()
	for strings.Contains(safe, "--") {
		safe = strings.ReplaceAll(safe, "--", "-")
	}

	// Trim hyphens from ends
	safe = strings.Trim(safe, "-")

	if safe == "" {
		safe = "untitled"
	}

	return safe
}

func runNew(cmd *cobra.Command, args []string) error {
	noteType := args[0]

	// Validate type
	validType := false
	for _, name := range templates.Available() {
		if name == noteType {
			validType = true
			break
		}
	}
	if !validType {
		return fmt.Errorf("unknown note type: %q\nAvailable types: %s",
			noteType, strings.Join(templates.Available(), ", "))
	}

	// Determine vault path
	vp := getVaultPath()
	if !vault.IsVault(vp) {
		return fmt.Errorf("not an AgentVault directory: %s\nRun 'agentvault init' to create one", vp)
	}

	// Parse tags
	var tags []string
	if newTags != "" {
		for _, t := range strings.Split(newTags, ",") {
			t = strings.TrimSpace(t)
			if t != "" {
				tags = append(tags, t)
			}
		}
	}

	// Generate ID and determine output path. Folder resolution is shared
	// with the HTTP API and MCP server via templates.FolderPathForType so a
	// note written from any surface lands in the same place.
	id := templates.GenerateID(noteType)
	folder := templates.FolderPathForType(noteType, newProject, vp)

	// Create folder (and subfolder for project if needed)
	if err := os.MkdirAll(folder, 0755); err != nil {
		return fmt.Errorf("create folder %s: %w", folder, err)
	}

	// Build filename: {sanitized-title}_{id-abbrev}.md
	safeTitle := sanitizeFilename(newTitle)
	idParts := strings.Split(id, "_")
	var shortID string
	if len(idParts) > 0 {
		shortID = idParts[len(idParts)-1]
	}
	filename := fmt.Sprintf("%s_%s.md", safeTitle, shortID)
	outPath := filepath.Join(folder, filename)

	// Build template data
	data := templates.TemplateData{
		ID:      id,
		Title:   newTitle,
		Project: newProject,
		Tags:    tags,
		Created: timeNowFunc().Format("2006-01-02T15:04:05Z"),
		URL:     newURL,
	}

	// Render template
	content, err := templates.Render(noteType, data)
	if err != nil {
		return fmt.Errorf("render template: %w", err)
	}

	// Write file
	if err := os.WriteFile(outPath, []byte(content), 0644); err != nil {
		return fmt.Errorf("write file %s: %w", outPath, err)
	}

	// Print results
	relPath, _ := filepath.Rel(vp, outPath)
	fmt.Printf("Created: %s\n", relPath)

	if newCommit {
		if git.IsGitRepo(vp) {
			if err := git.CommitFiles(vp, []string{relPath}, fmt.Sprintf("Add %s: %s", noteType, newTitle)); err != nil {
				fmt.Fprintf(os.Stderr, "Warning: failed to commit: %v\n", err)
				fmt.Printf("Run 'git add %s' to track this file\n", relPath)
			} else {
				hash, _ := git.LastCommitHash(vp)
				fmt.Printf("Created: %s and committed (%s)\n", relPath, hash)
			}
		} else {
			fmt.Println("Note: vault is not a git repository. Run 'agentvault git init' to enable versioning.")
			fmt.Printf("Run 'git add %s' to track this file\n", relPath)
		}
	} else {
		fmt.Printf("Run 'git add %s' to track this file\n", relPath)
	}

	return nil
}

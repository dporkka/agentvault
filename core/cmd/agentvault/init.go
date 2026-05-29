package main

import (
	"fmt"
	"os"
	"path/filepath"
	"strings"

	"github.com/agentvault/core/internal/config"
	"github.com/agentvault/core/internal/db"
	"github.com/agentvault/core/internal/templates"
	"github.com/agentvault/core/internal/vault"
	"github.com/spf13/cobra"
)

var initTemplate string

var initCmd = &cobra.Command{
	Use:   "init [path]",
	Short: "Initialize a new AgentVault",
	Long:  "Creates folder structure and config for a new AgentVault at the specified path (default: current directory).",
	Args:  cobra.MaximumNArgs(1),
	RunE: func(cmd *cobra.Command, args []string) error {
		path := "."
		if len(args) > 0 {
			path = args[0]
		}

		// Check if already a vault
		if vault.IsVault(path) {
			return fmt.Errorf("already an AgentVault: %s\nTip: use a different path or delete the existing .agentvault directory", path)
		}

		// Create vault structure
		if err := vault.Init(path); err != nil {
			return fmt.Errorf("failed to initialize vault: %w", err)
		}

		// Create config
		cfg, err := config.Init(path)
		if err != nil {
			return fmt.Errorf("failed to create config: %w", err)
		}

		// Open and initialize database
		database, err := db.Open(path)
		if err != nil {
			return fmt.Errorf("failed to open database: %w", err)
		}
		defer database.Close()

		if err := database.RunMigrations(); err != nil {
			return fmt.Errorf("failed to run database migrations: %w", err)
		}

		// Apply starter template if specified
		if initTemplate != "" {
			tmpl, ok := templates.GetStarterTemplate(initTemplate)
			if !ok {
				available := []string{}
				for _, t := range templates.ListStarterTemplates() {
					available = append(available, t.Name)
				}
				return fmt.Errorf("unknown template %q. Available: %s", initTemplate, strings.Join(available, ", "))
			}
			// Create additional folders
			for _, folder := range tmpl.Folders {
				folderPath := filepath.Join(path, folder)
				if err := os.MkdirAll(folderPath, 0755); err != nil {
					return fmt.Errorf("failed to create folder %s: %w", folder, err)
				}
			}
			// Write template files
			for filePath, content := range tmpl.Files {
				fullPath := filepath.Join(path, filePath)
				if err := os.MkdirAll(filepath.Dir(fullPath), 0755); err != nil {
					return fmt.Errorf("failed to create folder for %s: %w", filePath, err)
				}
				if err := os.WriteFile(fullPath, []byte(content), 0644); err != nil {
					return fmt.Errorf("failed to write %s: %w", filePath, err)
				}
			}
			// Print summary
			fmt.Printf("Template '%s' applied: %d files\n", initTemplate, len(tmpl.Files))
		}

		fmt.Printf("AgentVault initialized at: %s\n", cfg.VaultPath)
		fmt.Println("")
		fmt.Println("Next steps:")
		fmt.Println("  1. Add markdown files to the vault folders")
		fmt.Println("  2. Run 'agentvault index' to build the search index")
		fmt.Println("  3. Run 'agentvault search <query>' to search")

		return nil
	},
}

func init() {
	initCmd.Flags().StringVarP(&initTemplate, "template", "t", "", "Apply a starter template (founder, developer, agent-memory, research)")
	rootCmd.AddCommand(initCmd)
}

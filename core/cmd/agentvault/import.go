package main

import (
	"fmt"
	"strings"

	"github.com/agentvault/core/internal/importers"
	"github.com/agentvault/core/internal/indexer"
	"github.com/spf13/cobra"
)

var (
	importMode          string
	importProject       string
	importTags          string
	importKeepStructure bool
)

var importLong = strings.Join([]string{
	"Import notes from external sources into the current AgentVault.",
	"",
	"Available importers: markdown, obsidian",
	"",
	"Modes:",
	"  copy       Copy files to the vault without modifying them (default)",
	"  in-place   Import by referencing files at their current location",
	"  normalize  Copy and normalize frontmatter (add id, type, title, dates)",
	"",
	"Examples:",
	"  agentvault import markdown ~/Documents/notes",
	"  agentvault import obsidian ~/obsidian-vault --mode normalize --keep-structure",
	"  agentvault import markdown ~/notes --project myproject --tags imported,archive",
}, "\n")

var importCmd = &cobra.Command{
	Use:   "import <importer> <source-path>",
	Short: "Import notes from external sources",
	Long:  importLong,
	Args:  cobra.ExactArgs(2),
	RunE:  runImport,
}

func runImport(cmd *cobra.Command, args []string) error {
	importerName := args[0]
	sourcePath := args[1]

	vp := mustRequireVault()
	database, err := openDB(vp)
	if err != nil {
		return err
	}
	defer database.Close()

	imp, ok := importers.Get(importerName)
	if !ok {
		available := importers.Available()
		if available == "" {
			return fmt.Errorf("no importers registered")
		}
		return fmt.Errorf("unknown importer: %q\nAvailable importers: %s", importerName, available)
	}

	var tags []string
	if importTags != "" {
		for _, t := range strings.Split(importTags, ",") {
			t = strings.TrimSpace(t)
			if t != "" {
				tags = append(tags, t)
			}
		}
	}

	mode := importMode
	if mode == "" {
		mode = "copy"
	}
	if mode != "copy" && mode != "in-place" && mode != "normalize" {
		return fmt.Errorf("invalid mode: %q\nValid modes: copy, in-place, normalize", mode)
	}

	opts := importers.ImportOptions{
		SourcePath:     sourcePath,
		TargetVault:    vp,
		Mode:           mode,
		KeepStructure:  importKeepStructure,
		DefaultProject: importProject,
		Tags:           tags,
	}

	fmt.Printf("Importing from %s using %s importer...\n", sourcePath, importerName)
	if mode != "copy" {
		fmt.Printf("Mode: %s\n", mode)
	}

	result, err := imp.Import(opts)
	if err != nil {
		return fmt.Errorf("import failed: %w", err)
	}

	fmt.Println("")
	fmt.Printf("Imported: %d\n", result.FilesImported)
	fmt.Printf("Skipped:  %d\n", result.FilesSkipped)
	if len(result.Errors) > 0 {
		fmt.Printf("Errors:   %d\n", len(result.Errors))
		for _, e := range result.Errors[:min(len(result.Errors), 10)] {
			fmt.Printf("  - %s: %s\n", e.Path, e.Error)
		}
		if len(result.Errors) > 10 {
			fmt.Printf("  ... and %d more errors\n", len(result.Errors)-10)
		}
	} else {
		fmt.Printf("Errors:   0\n")
	}
	if len(result.Warnings) > 0 {
		fmt.Printf("Warnings: %d\n", len(result.Warnings))
		for _, w := range result.Warnings[:min(len(result.Warnings), 5)] {
			fmt.Printf("  - %s\n", w)
		}
		if len(result.Warnings) > 5 {
			fmt.Printf("  ... and %d more warnings\n", len(result.Warnings)-5)
		}
	}

	if result.FilesImported > 0 {
		fmt.Println("")
		fmt.Println("Indexing imported files...")
		idx := indexer.New(database, vp)
		indexOpts := indexer.IndexOptions{
			Force: false,
		}
		indexResult, err := idx.Index(indexOpts)
		if err != nil {
			fmt.Fprintf(cmd.ErrOrStderr(), "Warning: indexing failed: %v\n", err)
		} else {
			fmt.Printf("Indexed: %d scanned, %d added, %d updated, %d skipped\n",
				indexResult.Scanned, indexResult.Added, indexResult.Updated, indexResult.Skipped)
		}
	}

	return nil
}

func init() {
	importers.Register(&importers.MarkdownImporter{})
	importers.Register(&importers.ObsidianImporter{})

	importCmd.Flags().StringVar(&importMode, "mode", "copy", "Import mode: copy, in-place, normalize")
	importCmd.Flags().StringVar(&importProject, "project", "", "Default project to assign")
	importCmd.Flags().StringVar(&importTags, "tags", "", "Comma-separated tags to add")
	importCmd.Flags().BoolVar(&importKeepStructure, "keep-structure", false, "Preserve source folder structure")

	rootCmd.AddCommand(importCmd)
}

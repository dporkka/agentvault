package main

import (
	"fmt"
	"time"

	"github.com/agentvault/core/internal/indexer"
	"github.com/spf13/cobra"
)

var (
	indexForce   bool
	indexRebuild bool
	indexPath    string
	indexEmbed   bool
)

var indexCmd = &cobra.Command{
	Use:   "index",
	Short: "Index the vault for search",
	Long:  "Scans all markdown files in the vault and updates the SQLite search index.\nUse --embed to generate vector embeddings for semantic search (requires AI provider).",
	RunE: func(cmd *cobra.Command, args []string) error {
		vp := mustRequireVault()
		database, err := openDB(vp)
		if err != nil {
			return err
		}
		defer database.Close()

		opts := indexer.IndexOptions{
			Force:   indexForce,
			Rebuild: indexRebuild,
			Path:    indexPath,
			Embed:   indexEmbed,
		}

		fmt.Println("Indexing vault...")
		if opts.Embed {
			fmt.Println("(Embedding generation enabled - this may take a while)")
		}
		start := time.Now()

		idx := indexer.New(database, vp)
		result, err := idx.Index(opts)
		if err != nil {
			return fmt.Errorf("indexing failed: %w", err)
		}

		duration := time.Since(start)

		fmt.Println("")
		fmt.Printf("Scanned:  %d\n", result.Scanned)
		fmt.Printf("Added:    %d\n", result.Added)
		fmt.Printf("Updated:  %d\n", result.Updated)
		fmt.Printf("Skipped:  %d\n", result.Skipped)
		fmt.Printf("Removed:  %d\n", result.Removed)
		if opts.Embed {
			fmt.Printf("Chunks:   %d\n", result.ChunksAdded)
			if result.EmbedErrors > 0 {
				fmt.Printf("EmbedErr: %d\n", result.EmbedErrors)
			}
		}
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
		fmt.Printf("Duration: %s\n", duration.Round(time.Millisecond))

		return nil
	},
}

func min(a, b int) int {
	if a < b {
		return a
	}
	return b
}

func init() {
	indexCmd.Flags().BoolVar(&indexForce, "force", false, "Reindex even if hash matches")
	indexCmd.Flags().BoolVar(&indexRebuild, "rebuild", false, "Drop and recreate all indexes")
	indexCmd.Flags().StringVar(&indexPath, "path", "", "Index only this subpath")
	indexCmd.Flags().BoolVar(&indexEmbed, "embed", false, "Generate embeddings for semantic search (requires AI provider)")
	rootCmd.AddCommand(indexCmd)
}

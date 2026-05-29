package main

import (
	"fmt"
	"strings"

	"github.com/agentvault/core/internal/search"
	"github.com/fatih/color"
	"github.com/spf13/cobra"
)

var (
	searchType    string
	searchProject string
	searchTag     string
	searchStatus  string
	searchLimit   int
)

var searchCmd = &cobra.Command{
	Use:   "search <query>",
	Short: "Search the vault",
	Long:  "Full-text search across all notes in the AgentVault using SQLite FTS5.",
	Args:  cobra.MinimumNArgs(1),
	RunE: func(cmd *cobra.Command, args []string) error {
		vp := mustRequireVault()
		database, err := openDB(vp)
		if err != nil {
			return err
		}
		defer database.Close()

		query := search.Query{
			Q:       strings.Join(args, " "),
			Type:    searchType,
			Project: searchProject,
			Tag:     searchTag,
			Status:  searchStatus,
			Limit:   searchLimit,
		}

		s := search.New(database)
		results, err := s.Search(query)
		if err != nil {
			return fmt.Errorf("search failed: %w", err)
		}

		if len(results) == 0 {
			fmt.Println("No results found.")
			return nil
		}

		// Print header
		fmt.Printf("\nFound %d result(s)\n\n", len(results))

		// Print results as formatted table
		bold := color.New(color.Bold)
		dim := color.New(color.Faint)
		cyan := color.New(color.FgCyan)
		yellow := color.New(color.FgYellow)

		for i, r := range results {
			// Title line
			bold.Printf("%d. %s", i+1, r.Title)
			if r.Type != "" {
				cyan.Printf(" [%s]", r.Type)
			}
			fmt.Println()

			// Metadata line
			var metaParts []string
			if r.Project != "" {
				metaParts = append(metaParts, fmt.Sprintf("project: %s", r.Project))
			}
			if r.Status != "" {
				metaParts = append(metaParts, fmt.Sprintf("status: %s", r.Status))
			}
			if len(r.Tags) > 0 {
				yellow.Printf("  #%s", strings.Join(r.Tags, " #"))
			}
			if len(metaParts) > 0 {
				dim.Printf("  (%s)", strings.Join(metaParts, ", "))
			}
			if len(metaParts) > 0 || len(r.Tags) > 0 {
				fmt.Println()
			}

			// Snippet
			if r.Snippet != "" {
				// Strip HTML tags for terminal display
				snippet := stripHTMLTags(r.Snippet)
				fmt.Printf("  %s\n", snippet)
			}

			// Path
			dim.Printf("  %s\n", r.Path)

			fmt.Println()
		}

		return nil
	},
}

// stripHTMLTags removes HTML tags from a string.
func stripHTMLTags(s string) string {
	var result strings.Builder
	inTag := false
	for _, c := range s {
		if c == '<' {
			inTag = true
			continue
		}
		if c == '>' {
			inTag = false
			continue
		}
		if !inTag {
			result.WriteRune(c)
		}
	}
	return result.String()
}

func init() {
	searchCmd.Flags().StringVar(&searchType, "type", "", "Filter by note type")
	searchCmd.Flags().StringVar(&searchProject, "project", "", "Filter by project")
	searchCmd.Flags().StringVar(&searchTag, "tag", "", "Filter by tag")
	searchCmd.Flags().StringVar(&searchStatus, "status", "", "Filter by status")
	searchCmd.Flags().IntVar(&searchLimit, "limit", 20, "Maximum number of results")
	rootCmd.AddCommand(searchCmd)
}

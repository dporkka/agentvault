package main

import (
	"context"
	"fmt"
	"strings"

	"github.com/agentvault/core/internal/search"
	"github.com/fatih/color"
	"github.com/spf13/cobra"
)

var (
	searchType         string
	searchProject      string
	searchTag          string
	searchStatus       string
	searchLimit        int
	searchVector       bool
	searchHybridWeight float64
	searchTopK         int
)

var searchCmd = &cobra.Command{
	Use:   "search <query>",
	Short: "Search the vault",
	Long: `Full-text search across all notes in the AgentVault using SQLite FTS5.
Use --vector to enable semantic/hybrid search when embeddings have been generated
(via "agentvault index --embed").`,
	Args: cobra.MinimumNArgs(1),
	RunE: func(cmd *cobra.Command, args []string) error {
		vp := mustRequireVault()
		database, err := openDB(vp)
		if err != nil {
			return err
		}
		defer database.Close()

		queryText := strings.Join(args, " ")
		s := search.New(database)
		s.ConfigureEmbeddings(vp)

		var results []search.Result
		if searchVector {
			vq := search.VectorQuery{
				Query: search.Query{
					Q:       queryText,
					Type:    searchType,
					Project: searchProject,
					Tag:     searchTag,
					Status:  searchStatus,
					Limit:   searchLimit,
				},
				VectorSearch: true,
				QueryText:    queryText,
				TopK:         searchTopK,
				HybridWeight: searchHybridWeight,
			}
			if vq.TopK <= 0 {
				vq.TopK = searchLimit * 3
				if vq.TopK < 10 {
					vq.TopK = 10
				}
			}
			results, err = s.HybridSearch(context.Background(), vq)
		} else {
			query := search.Query{
				Q:       queryText,
				Type:    searchType,
				Project: searchProject,
				Tag:     searchTag,
				Status:  searchStatus,
				Limit:   searchLimit,
			}
			results, err = s.Search(query)
		}
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
	searchCmd.Flags().BoolVar(&searchVector, "vector", false, "Enable semantic/hybrid search (requires embeddings)")
	searchCmd.Flags().Float64Var(&searchHybridWeight, "hybrid-weight", 0.5, "Weight for vector vs FTS (0=FTS only, 1=vector only)")
	searchCmd.Flags().IntVar(&searchTopK, "topk", 0, "Number of vector candidates to fetch (default limit*3, min 10)")
	rootCmd.AddCommand(searchCmd)
}

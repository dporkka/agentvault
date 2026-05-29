package main

import (
	"fmt"
	"strings"

	"github.com/agentvault/core/internal/markdown"
	"github.com/agentvault/core/internal/search"
	"github.com/fatih/color"
	"github.com/spf13/cobra"
)

var readCmd = &cobra.Command{
	Use:   "read <id-or-path>",
	Short: "Read a note by ID or path",
	Long:  "Displays the full content of a note including frontmatter and body.",
	Args:  cobra.ExactArgs(1),
	RunE: func(cmd *cobra.Command, args []string) error {
		vp := mustRequireVault()
		database, err := openDB(vp)
		if err != nil {
			return err
		}
		defer database.Close()

		lookup := strings.TrimSpace(args[0])

		// Try GetByID first, then fall back to GetByPath
		var result *search.Result
		s := search.New(database)

		result, err = s.GetByID(lookup)
		if err != nil {
			// Try as path
			result, err = s.GetByPath(lookup)
			if err != nil {
				// Try prepending common extensions/paths
				if !strings.HasSuffix(lookup, ".md") {
					result, err = s.GetByPath(lookup + ".md")
				}
				if err != nil {
					return fmt.Errorf("note not found: %s\nRun 'agentvault search <term>' to find notes.", lookup)
				}
			}
		}

		// Try to read and display the actual file
		fullPath := vp
		if result.Path != "" {
			fullPath = vp + "/" + result.Path
		}

		doc, err := markdown.ParseFile(fullPath)
		if err != nil {
			// Fallback: display from database
			printFromDB(result)
			return nil
		}

		printDocument(result, doc)
		return nil
	},
}

func printFromDB(result *search.Result) {
	bold := color.New(color.Bold)
	cyan := color.New(color.FgCyan)
	dim := color.New(color.Faint)

	bold.Printf("%s\n", result.Title)
	fmt.Println(strings.Repeat("=", len(result.Title)))
	fmt.Println()

	cyan.Println("Metadata:")
	fmt.Printf("  ID:      %s\n", result.ID)
	fmt.Printf("  Path:    %s\n", result.Path)
	if result.Type != "" {
		fmt.Printf("  Type:    %s\n", result.Type)
	}
	if result.Project != "" {
		fmt.Printf("  Project: %s\n", result.Project)
	}
	if result.Status != "" {
		fmt.Printf("  Status:  %s\n", result.Status)
	}
	if len(result.Tags) > 0 {
		fmt.Printf("  Tags:    %s\n", strings.Join(result.Tags, ", "))
	}
	fmt.Println()

	dim.Println("(Note: could not read original file, showing database content)")
	if result.Snippet != "" {
		fmt.Println()
		fmt.Println(result.Snippet)
	}
}

func printDocument(result *search.Result, doc *markdown.ParsedDocument) {
	bold := color.New(color.Bold)
	cyan := color.New(color.FgCyan)
	yellow := color.New(color.FgYellow)
	green := color.New(color.FgGreen)

	bold.Printf("%s\n", doc.Frontmatter.Title)
	fmt.Println(strings.Repeat("=", len(doc.Frontmatter.Title)))
	fmt.Println()

	cyan.Println("---")
	green.Printf("id: %s\n", doc.Frontmatter.ID)
	green.Printf("type: %s\n", doc.Frontmatter.Type)
	if doc.Frontmatter.Status != "" {
		green.Printf("status: %s\n", doc.Frontmatter.Status)
	}
	if doc.Frontmatter.Project != "" {
		green.Printf("project: %s\n", doc.Frontmatter.Project)
	}
	if len(doc.Frontmatter.Tags) > 0 {
		yellow.Printf("tags: [%s]\n", strings.Join(doc.Frontmatter.Tags, ", "))
	}
	if doc.Frontmatter.Created != "" {
		green.Printf("created: %s\n", doc.Frontmatter.Created)
	}
	if doc.Frontmatter.Updated != "" {
		green.Printf("updated: %s\n", doc.Frontmatter.Updated)
	}
	cyan.Println("---")
	fmt.Println()

	fmt.Println(doc.Body)
}

func init() {
	rootCmd.AddCommand(readCmd)
}

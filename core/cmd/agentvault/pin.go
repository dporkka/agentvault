package main

import (
	"fmt"
	"os"
	"path/filepath"
	"time"

	"github.com/agentvault/core/internal/indexer"
	"github.com/agentvault/core/internal/markdown"
	"github.com/agentvault/core/internal/search"
	"github.com/spf13/cobra"
	"gopkg.in/yaml.v3"
)

var pinCmd = &cobra.Command{
	Use:   "pin <note-id>",
	Short: "Pin a note",
	Long:  `Adds pinned: true to a note's frontmatter and reindexes it.`,
	Args:  cobra.ExactArgs(1),
	RunE:  runPin,
}

var unpinCmd = &cobra.Command{
	Use:   "unpin <note-id>",
	Short: "Unpin a note",
	Long:  `Removes the pinned field from a note's frontmatter and reindexes it.`,
	Args:  cobra.ExactArgs(1),
	RunE:  runUnpin,
}

func init() {
	rootCmd.AddCommand(pinCmd)
	rootCmd.AddCommand(unpinCmd)
}

func togglePin(id string, pin bool) error {
	vp := mustRequireVault()
	database, err := openDB(vp)
	if err != nil {
		return err
	}
	defer database.Close()

	s := search.New(database)
	result, err := s.GetByID(id)
	if err != nil {
		return fmt.Errorf("note not found: %w", err)
	}

	fullPath := filepath.Join(vp, result.Path)
	doc, err := markdown.ParseFile(fullPath)
	if err != nil {
		return fmt.Errorf("failed to parse note: %w", err)
	}

	extra := doc.Frontmatter.Extra
	if extra == nil {
		extra = make(map[string]interface{})
	}

	if pin {
		extra["pinned"] = true
	} else {
		delete(extra, "pinned")
	}
	doc.Frontmatter.Extra = extra
	doc.Frontmatter.Updated = time.Now().UTC().Format(time.RFC3339)

	fm := doc.Frontmatter
	fmMap := map[string]interface{}{
		"id": fm.ID, "type": fm.Type, "title": fm.Title,
		"status": fm.Status, "project": fm.Project,
		"tags": fm.Tags, "created": fm.Created, "updated": fm.Updated,
	}
	for k, v := range fm.Extra {
		fmMap[k] = v
	}
	yamlBytes, _ := yaml.Marshal(fmMap)
	if err := os.WriteFile(fullPath, []byte("---\n"+string(yamlBytes)+"---\n\n"+doc.Body), 0644); err != nil {
		return fmt.Errorf("failed to write note: %w", err)
	}

	idx := indexer.New(database, vp)
	_, ierr := idx.Index(indexer.IndexOptions{Path: result.Path})
	if ierr != nil {
		return fmt.Errorf("reindex failed: %w", ierr)
	}

	action := "unpinned"
	if pin {
		action = "pinned"
	}
	fmt.Printf("Note %q %s\n", result.Path, action)
	return nil
}

func runPin(cmd *cobra.Command, args []string) error {
	return togglePin(args[0], true)
}

func runUnpin(cmd *cobra.Command, args []string) error {
	return togglePin(args[0], false)
}

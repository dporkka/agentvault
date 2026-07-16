package main

import (
	"fmt"
	"os"
	"path/filepath"
	"time"

	"github.com/agentvault/core/internal/templates"
	"github.com/spf13/cobra"
)

var dailyDate string

var dailyCmd = &cobra.Command{
	Use:   "daily",
	Short: "Open or create today's daily note",
	Long: `Creates or opens the daily note for today (or a specific date).
Daily notes are stored in 05-daily/YYYY/MM/YYYY-MM-DD.md.`,
	RunE: runDaily,
}

func init() {
	rootCmd.AddCommand(dailyCmd)
	dailyCmd.Flags().StringVar(&dailyDate, "date", "", "Date for the daily note (YYYY-MM-DD, default: today)")
}

func runDaily(cmd *cobra.Command, args []string) error {
	vp := mustRequireVault()

	// Parse the target date
	var target time.Time
	if dailyDate != "" {
		var err error
		target, err = time.Parse("2006-01-02", dailyDate)
		if err != nil {
			return fmt.Errorf("invalid date %q: use YYYY-MM-DD format", dailyDate)
		}
	} else {
		target = time.Now().UTC()
	}

	dateStr := target.Format("2006-01-02")
	title := target.Format("Monday, January 2, 2006")
	dayOfWeek := target.Format("Monday")

	// Folder structure: 05-daily/YYYY/MM/YYYY-MM-DD.md
	year := target.Format("2006")
	month := target.Format("01")
	folder := filepath.Join("05-daily", year, month)
	filename := dateStr + ".md"
	relPath := filepath.Join(folder, filename)
	fullPath := filepath.Join(vp, relPath)

	// If the file already exists, just report its path
	if _, err := os.Stat(fullPath); err == nil {
		fmt.Println(fullPath)
		return nil
	}

	// Create the daily note
	id := fmt.Sprintf("day_%s", dateStr)
	now := time.Now().UTC().Format(time.RFC3339)

	data := templates.TemplateData{
		ID:        id,
		Title:     title,
		Created:   now,
		DayOfWeek: dayOfWeek,
	}

	rendered, err := templates.Render("daily", data)
	if err != nil {
		return fmt.Errorf("failed to render daily template: %w", err)
	}

	if err := os.MkdirAll(filepath.Dir(fullPath), 0755); err != nil {
		return fmt.Errorf("failed to create directory: %w", err)
	}

	if err := os.WriteFile(fullPath, []byte(rendered), 0644); err != nil {
		return fmt.Errorf("failed to write daily note: %w", err)
	}

	fmt.Println(fullPath)
	return nil
}

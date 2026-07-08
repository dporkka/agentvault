package main

import (
	"fmt"
	"os"
	"strings"

	"github.com/agentvault/core/internal/doctor"
	"github.com/fatih/color"
	"github.com/spf13/cobra"
)

var doctorToken string
var doctorAPIURL string

var doctorCmd = &cobra.Command{
	Use:   "doctor",
	Short: "Validate and diagnose vault issues",
	Long:  "Runs comprehensive checks on the vault configuration, database, and markdown files.",
	RunE: func(cmd *cobra.Command, args []string) error {
		vp := mustRequireVault()
		database, err := openDB(vp)
		if err != nil {
			return err
		}
		defer database.Close()

		token := doctorToken
		if token == "" {
			token = os.Getenv("AGENTVAULT_TOKEN")
		}

		d := doctor.New(database, vp)
		d.SetAPI(doctorAPIURL, token)
		results := d.RunAll()

		// Print results with color-coded status
		okColor := color.New(color.FgGreen)
		warnColor := color.New(color.FgYellow)
		errColor := color.New(color.FgRed)
		bold := color.New(color.Bold)

		fmt.Println()
		bold.Println("AgentVault Diagnostic Report")
		fmt.Println(strings.Repeat("-", 40))
		fmt.Println()

		var okCount, warnCount, errCount int

		for _, r := range results {
			// Status indicator
			switch r.Status {
			case "ok":
				okColor.Printf("  [OK]  ")
				okCount++
			case "warn":
				warnColor.Printf(" [WARN] ")
				warnCount++
			case "error":
				errColor.Printf("[FAIL] ")
				errCount++
			}

			// Check name and message
			fmt.Printf(" %-20s %s\n", r.Name, r.Message)

			// Details
			for _, detail := range r.Details {
				fmt.Printf("        -> %s\n", detail)
			}
		}

		fmt.Println()
		fmt.Println(strings.Repeat("-", 40))

		// Summary
		bold.Print("Summary: ")
		okColor.Printf("%d passed", okCount)
		fmt.Print(", ")
		if warnCount > 0 {
			warnColor.Printf("%d warnings", warnCount)
		} else {
			fmt.Printf("0 warnings")
		}
		fmt.Print(", ")
		if errCount > 0 {
			errColor.Printf("%d errors", errCount)
		} else {
			fmt.Printf("0 errors")
		}
		fmt.Println()
		fmt.Println()

		if errCount > 0 {
			fmt.Println("Suggestion: Run 'agentvault init' to set up a new vault,")
			fmt.Println("            or check the errors above and fix them manually.")
			fmt.Println()
		}

		return nil
	},
}

func init() {
	rootCmd.AddCommand(doctorCmd)
	doctorCmd.Flags().StringVar(&doctorToken, "token", "", "API token for auth verification (defaults to AGENTVAULT_TOKEN env var)")
	doctorCmd.Flags().StringVar(&doctorAPIURL, "api-url", "http://127.0.0.1:47321", "Base URL of the local AgentVault API")
}

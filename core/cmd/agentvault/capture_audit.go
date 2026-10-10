package main

import (
	"encoding/json"
	"fmt"

	"github.com/agentvault/core/internal/capturekey"
	"github.com/spf13/cobra"
)

var captureCmd = &cobra.Command{
	Use:   "capture",
	Short: "Inspect capture readiness",
}

var captureAuditCmd = &cobra.Command{
	Use:   "audit",
	Short: "Read-only keyed-capture duplicate and canonical-file audit",
	Long:  "Inspect 00-inbox Markdown before a keyed-capture migration. Prints JSON with SHA-256 key fingerprints, never raw external IDs. Does not repair files or stop writers.",
	RunE: func(cmd *cobra.Command, args []string) error {
		return runCaptureAudit(cmd)
	},
}

func runCaptureAudit(cmd *cobra.Command) error {
	vp, err := requireVault()
	if err != nil {
		return err
	}
	report, err := capturekey.AuditInbox(vp)
	if err != nil {
		return err
	}
	enc := json.NewEncoder(cmd.OutOrStdout())
	enc.SetIndent("", "  ")
	if err := enc.Encode(report); err != nil {
		return fmt.Errorf("write capture audit: %w", err)
	}
	if n := len(report.Duplicates); n > 0 {
		return fmt.Errorf("capture audit found %d duplicate external_id group(s); reconcile manually before deployment", n)
	}
	if n := len(report.InvalidCanonical) + len(report.MalformedFiles); n > 0 {
		return fmt.Errorf("capture audit found %d invalid or malformed capture file report(s); inspect before deployment", n)
	}
	return nil
}

func init() {
	rootCmd.AddCommand(captureCmd)
	captureCmd.AddCommand(captureAuditCmd)
}

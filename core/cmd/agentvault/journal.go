package main

import (
	"encoding/json"
	"fmt"
	"io"
	"os"

	"github.com/agentvault/core/internal/knowledge"
	"github.com/spf13/cobra"
)

var (
	journalVerifyCheckpoint string
	journalVerifyJSON       bool
	journalCheckpointOutput string
)

var journalCmd = &cobra.Command{
	Use:   "journal",
	Short: "Inspect and verify the durable knowledge journal",
}

var journalVerifyCmd = &cobra.Command{
	Use:   "verify",
	Short: "Verify journal integrity and optional external checkpoint",
	RunE: func(cmd *cobra.Command, args []string) error {
		vp, err := requireVault()
		if err != nil {
			return err
		}
		return runJournalVerify(vp, journalVerifyCheckpoint, journalVerifyJSON, cmd.OutOrStdout())
	},
}

var journalCheckpointCmd = &cobra.Command{
	Use:   "checkpoint",
	Short: "Write a portable external journal checkpoint",
	RunE: func(cmd *cobra.Command, args []string) error {
		vp, err := requireVault()
		if err != nil {
			return err
		}
		if journalCheckpointOutput == "" {
			return fmt.Errorf("--output is required")
		}
		return runJournalCheckpoint(vp, journalCheckpointOutput, cmd.OutOrStdout())
	},
}

func runJournalVerify(vaultPath, checkpointPath string, asJSON bool, out io.Writer) error {
	journal := knowledge.NewJournal(vaultPath)
	report, err := journal.Verify()
	if err != nil {
		return fmt.Errorf("verify journal: %w", err)
	}

	checkpointVerified := false
	if checkpointPath != "" {
		if err := journal.VerifyCheckpoint(checkpointPath); err != nil {
			return fmt.Errorf("verify journal checkpoint: %w", err)
		}
		checkpointVerified = true
	}

	if asJSON {
		result := struct {
			Integrity          knowledge.JournalIntegrityReport `json:"integrity"`
			Checkpoint         string                           `json:"checkpoint,omitempty"`
			CheckpointVerified bool                             `json:"checkpointVerified"`
		}{
			Integrity:          report,
			Checkpoint:         checkpointPath,
			CheckpointVerified: checkpointVerified,
		}
		if err := json.NewEncoder(out).Encode(result); err != nil {
			return fmt.Errorf("write journal verification result: %w", err)
		}
		return nil
	}

	fmt.Fprintln(out, "Journal integrity OK")
	fmt.Fprintf(out, "Events: %d\n", report.Events)
	fmt.Fprintf(out, "Legacy events: %d\n", report.LegacyEvents)
	fmt.Fprintf(out, "Chained events: %d\n", report.ChainedEvents)
	if report.LegacyAnchor != "" {
		fmt.Fprintf(out, "Legacy anchor: %s\n", report.LegacyAnchor)
	}
	if report.HeadHash != "" {
		fmt.Fprintf(out, "Head hash: %s\n", report.HeadHash)
	}
	if checkpointVerified {
		fmt.Fprintln(out, "Checkpoint: OK")
	}
	return nil
}

func runJournalCheckpoint(vaultPath, outputPath string, out io.Writer) error {
	journal := knowledge.NewJournal(vaultPath)
	checkpoint, err := journal.WriteCheckpoint(outputPath)
	if err != nil {
		return fmt.Errorf("write journal checkpoint: %w", err)
	}
	fmt.Fprintf(out, "Journal checkpoint written: %s\n", outputPath)
	fmt.Fprintf(out, "Events: %d\n", checkpoint.Events)
	if checkpoint.HeadHash != "" {
		fmt.Fprintf(out, "Head hash: %s\n", checkpoint.HeadHash)
	}
	fmt.Fprintf(out, "Journal SHA-256: %s\n", checkpoint.JournalSHA256)
	return nil
}

func init() {
	rootCmd.AddCommand(journalCmd)
	journalCmd.AddCommand(journalVerifyCmd)
	journalCmd.AddCommand(journalCheckpointCmd)

	journalVerifyCmd.Flags().StringVar(&journalVerifyCheckpoint, "checkpoint", "", "External checkpoint file to verify against")
	journalVerifyCmd.Flags().BoolVar(&journalVerifyJSON, "json", false, "Emit machine-readable JSON")
	journalCheckpointCmd.Flags().StringVarP(&journalCheckpointOutput, "output", "o", "", "Checkpoint file to create (must not already exist)")

	// Cobra defaults command output to os.Stdout only when executed through the
	// root command. Set explicit defaults so direct command invocation remains
	// predictable for embedding/tests.
	journalVerifyCmd.SetOut(os.Stdout)
	journalCheckpointCmd.SetOut(os.Stdout)
}

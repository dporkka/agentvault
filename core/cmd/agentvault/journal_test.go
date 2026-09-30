package main

import (
	"bytes"
	"path/filepath"
	"strings"
	"testing"

	"github.com/agentvault/core/internal/knowledge"
)

func TestRunJournalVerifyReportsIntegrity(t *testing.T) {
	vp := setupTestVault(t)
	journal := knowledge.NewJournal(vp)
	if _, err := journal.Append("test.event", map[string]string{"value": "one"}); err != nil {
		t.Fatal(err)
	}

	var out bytes.Buffer
	if err := runJournalVerify(vp, "", false, &out); err != nil {
		t.Fatalf("runJournalVerify: %v", err)
	}
	text := out.String()
	if !strings.Contains(text, "Journal integrity OK") || !strings.Contains(text, "Events: 1") {
		t.Fatalf("unexpected verify output: %q", text)
	}
}

func TestRunJournalCheckpointThenVerifyAgainstCheckpoint(t *testing.T) {
	vp := setupTestVault(t)
	journal := knowledge.NewJournal(vp)
	if _, err := journal.Append("test.event", map[string]string{"value": "one"}); err != nil {
		t.Fatal(err)
	}

	checkpointPath := filepath.Join(t.TempDir(), "checkpoint.json")
	var checkpointOut bytes.Buffer
	if err := runJournalCheckpoint(vp, checkpointPath, &checkpointOut); err != nil {
		t.Fatalf("runJournalCheckpoint: %v", err)
	}
	if !strings.Contains(checkpointOut.String(), checkpointPath) {
		t.Fatalf("checkpoint output missing path: %q", checkpointOut.String())
	}

	var verifyOut bytes.Buffer
	if err := runJournalVerify(vp, checkpointPath, false, &verifyOut); err != nil {
		t.Fatalf("verify against checkpoint: %v", err)
	}
	if !strings.Contains(verifyOut.String(), "Checkpoint: OK") {
		t.Fatalf("checkpoint verify output = %q", verifyOut.String())
	}
}

func TestJournalCommandRegistered(t *testing.T) {
	cmd, _, err := rootCmd.Find([]string{"journal", "verify"})
	if err != nil {
		t.Fatalf("journal verify command not registered: %v", err)
	}
	if cmd.Name() != "verify" {
		t.Fatalf("found command %q, want verify", cmd.Name())
	}
}

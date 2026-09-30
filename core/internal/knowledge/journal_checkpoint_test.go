package knowledge

import (
	"os"
	"path/filepath"
	"strings"
	"testing"
)

func TestJournalCheckpointRoundTripAndDetectsWholeJournalRewrite(t *testing.T) {
	vault := t.TempDir()
	journal := NewJournal(vault)

	if _, err := journal.Append("test.event", map[string]string{"value": "one"}); err != nil {
		t.Fatal(err)
	}
	if _, err := journal.Append("test.event", map[string]string{"value": "two"}); err != nil {
		t.Fatal(err)
	}

	checkpointPath := filepath.Join(t.TempDir(), "journal-checkpoint.json")
	checkpoint, err := journal.WriteCheckpoint(checkpointPath)
	if err != nil {
		t.Fatalf("WriteCheckpoint: %v", err)
	}
	if checkpoint.HeadHash == "" || checkpoint.JournalSHA256 == "" {
		t.Fatalf("checkpoint missing integrity fields: %+v", checkpoint)
	}
	if checkpoint.Events != 2 {
		t.Fatalf("checkpoint events = %d, want 2", checkpoint.Events)
	}
	if err := journal.VerifyCheckpoint(checkpointPath); err != nil {
		t.Fatalf("VerifyCheckpoint: %v", err)
	}

	// Rebuild a different, internally valid journal from scratch. Its own hash
	// chain verifies, but it must no longer match the external checkpoint.
	if err := os.Remove(journal.Path()); err != nil {
		t.Fatal(err)
	}
	replacement := NewJournal(vault)
	if _, err := replacement.Append("test.event", map[string]string{"value": "rewritten"}); err != nil {
		t.Fatal(err)
	}
	if _, err := replacement.Verify(); err != nil {
		t.Fatalf("replacement journal should be internally valid: %v", err)
	}
	if err := replacement.VerifyCheckpoint(checkpointPath); err == nil {
		t.Fatal("expected checkpoint verification to detect whole-journal rewrite")
	}
}

func TestJournalCheckpointRefusesOverwrite(t *testing.T) {
	journal := NewJournal(t.TempDir())
	if _, err := journal.Append("test.event", map[string]string{"value": "one"}); err != nil {
		t.Fatal(err)
	}

	checkpointPath := filepath.Join(t.TempDir(), "checkpoint.json")
	if _, err := journal.WriteCheckpoint(checkpointPath); err != nil {
		t.Fatal(err)
	}
	if _, err := journal.WriteCheckpoint(checkpointPath); err == nil {
		t.Fatal("expected checkpoint overwrite to fail")
	}
}

func TestJournalVerifyCheckpointRejectsInvalidCheckpoint(t *testing.T) {
	journal := NewJournal(t.TempDir())
	checkpointPath := filepath.Join(t.TempDir(), "checkpoint.json")
	if err := os.WriteFile(checkpointPath, []byte(`{"version":1}`), 0o600); err != nil {
		t.Fatal(err)
	}
	err := journal.VerifyCheckpoint(checkpointPath)
	if err == nil || !strings.Contains(err.Error(), "invalid journal checkpoint") {
		t.Fatalf("expected invalid checkpoint error, got %v", err)
	}
}

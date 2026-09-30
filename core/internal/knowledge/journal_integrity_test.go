package knowledge

import (
	"encoding/json"
	"os"
	"path/filepath"
	"strings"
	"testing"
)

func TestJournalAppendBuildsVerifiableHashChain(t *testing.T) {
	journal := NewJournal(t.TempDir())

	first, err := journal.Append("test.event", map[string]string{"value": "one"})
	if err != nil {
		t.Fatal(err)
	}
	second, err := journal.Append("test.event", map[string]string{"value": "two"})
	if err != nil {
		t.Fatal(err)
	}

	if first.Hash == "" {
		t.Fatal("first chained event is missing hash")
	}
	if first.PrevHash != "" {
		t.Fatalf("first event prevHash = %q, want empty", first.PrevHash)
	}
	if second.Hash == "" {
		t.Fatal("second chained event is missing hash")
	}
	if second.PrevHash != first.Hash {
		t.Fatalf("second prevHash = %q, want %q", second.PrevHash, first.Hash)
	}

	report, err := journal.Verify()
	if err != nil {
		t.Fatalf("Verify: %v", err)
	}
	if report.Events != 2 || report.ChainedEvents != 2 || report.LegacyEvents != 0 {
		t.Fatalf("unexpected integrity report: %+v", report)
	}
	if report.HeadHash != second.Hash {
		t.Fatalf("head hash = %q, want %q", report.HeadHash, second.Hash)
	}
}

func TestJournalVerifyRejectsTamperingBeforeReplaySideEffects(t *testing.T) {
	journal := NewJournal(t.TempDir())

	if _, err := journal.Append("test.event", map[string]string{"value": "one"}); err != nil {
		t.Fatal(err)
	}
	if _, err := journal.Append("test.event", map[string]string{"value": "two"}); err != nil {
		t.Fatal(err)
	}

	data, err := os.ReadFile(journal.Path())
	if err != nil {
		t.Fatal(err)
	}
	tampered := strings.Replace(string(data), `"value":"one"`, `"value":"tampered"`, 1)
	if tampered == string(data) {
		t.Fatal("failed to tamper journal fixture")
	}
	if err := os.WriteFile(journal.Path(), []byte(tampered), 0o600); err != nil {
		t.Fatal(err)
	}

	if _, err := journal.Verify(); err == nil {
		t.Fatal("expected Verify to reject tampered journal")
	}

	replayed := 0
	err = journal.Replay(func(JournalEvent) error {
		replayed++
		return nil
	})
	if err == nil {
		t.Fatal("expected Replay to reject tampered journal")
	}
	if replayed != 0 {
		t.Fatalf("replay applied %d events before integrity failure; want 0", replayed)
	}
}

func TestJournalFirstChainedEventAnchorsLegacyPrefix(t *testing.T) {
	vault := t.TempDir()
	journal := NewJournal(vault)

	legacy := JournalEvent{
		Version:   journalVersion,
		ID:        "evt_legacy",
		Type:      "test.event",
		Timestamp: "2026-09-30T12:00:00Z",
		Payload:   json.RawMessage(`{"value":"legacy"}`),
	}
	line, err := json.Marshal(legacy)
	if err != nil {
		t.Fatal(err)
	}
	if err := os.MkdirAll(filepath.Dir(journal.Path()), 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(journal.Path(), append(line, '\n'), 0o600); err != nil {
		t.Fatal(err)
	}

	chained, err := journal.Append("test.event", map[string]string{"value": "new"})
	if err != nil {
		t.Fatal(err)
	}
	if chained.PrevHash == "" {
		t.Fatal("first chained event did not anchor the legacy prefix")
	}

	report, err := journal.Verify()
	if err != nil {
		t.Fatalf("Verify: %v", err)
	}
	if report.LegacyEvents != 1 || report.ChainedEvents != 1 {
		t.Fatalf("unexpected legacy/chained counts: %+v", report)
	}
	if report.LegacyAnchor == "" || report.LegacyAnchor != chained.PrevHash {
		t.Fatalf("legacy anchor = %q, chained prevHash = %q", report.LegacyAnchor, chained.PrevHash)
	}

	data, err := os.ReadFile(journal.Path())
	if err != nil {
		t.Fatal(err)
	}
	tampered := strings.Replace(string(data), `"value":"legacy"`, `"value":"tamper"`, 1)
	if tampered == string(data) {
		t.Fatal("failed to tamper legacy fixture")
	}
	if err := os.WriteFile(journal.Path(), []byte(tampered), 0o600); err != nil {
		t.Fatal(err)
	}
	if _, err := journal.Verify(); err == nil {
		t.Fatal("expected legacy-prefix tampering to break the anchored chain")
	}
}

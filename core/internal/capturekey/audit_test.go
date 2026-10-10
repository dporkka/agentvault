package capturekey

import (
	"crypto/sha256"
	"encoding/hex"
	"os"
	"path/filepath"
	"strings"
	"testing"
)

// An audit is intentionally advisory and read-only. Never select a legacy
// duplicate winner or mutate Markdown as part of readiness assessment.
func TestAuditInboxReportsLegacyDuplicatesWithoutMutation(t *testing.T) {
	vault := t.TempDir()
	inbox := filepath.Join(vault, "00-inbox")
	if err := os.MkdirAll(inbox, 0755); err != nil {
		t.Fatal(err)
	}
	key := "sensitive:client-reference"
	snapshots := map[string]string{
		"old-a.md":   string(captureBody(key, "first")),
		"old-b.md":   string(captureBody(key, "second")),
		"unkeyed.md": "---\nid: ordinary\ntype: capture\n---\n\nno key",
	}
	for name, data := range snapshots {
		if err := os.WriteFile(filepath.Join(inbox, name), []byte(data), 0644); err != nil {
			t.Fatal(err)
		}
	}
	report, err := AuditInbox(vault)
	if err != nil {
		t.Fatal(err)
	}
	if report.KeyedCaptures != 2 || len(report.Duplicates) != 1 {
		t.Fatalf("audit counts: %+v", report)
	}
	d := report.Duplicates[0]
	sum := sha256.Sum256([]byte(key))
	if d.KeyDigest != hex.EncodeToString(sum[:]) ||
		len(d.Paths) != 2 || d.Paths[0] != "00-inbox/old-a.md" || d.Paths[1] != "00-inbox/old-b.md" {
		t.Fatalf("audit duplicate report: %+v", d)
	}
	if strings.Contains(d.KeyDigest, key) {
		t.Fatal("audit must not expose raw external ID")
	}
	for name, original := range snapshots {
		data, err := os.ReadFile(filepath.Join(inbox, name))
		if err != nil || string(data) != original {
			t.Fatalf("read-only audit mutated %s: %v", name, err)
		}
	}
	entries, _ := os.ReadDir(inbox)
	if len(entries) != len(snapshots) {
		t.Fatalf("audit wrote files: %d entries", len(entries))
	}
}

func TestAuditInboxReportsInvalidCanonicalWithoutWriting(t *testing.T) {
	vault := t.TempDir()
	inbox := filepath.Join(vault, "00-inbox")
	if err := os.MkdirAll(inbox, 0755); err != nil {
		t.Fatal(err)
	}
	key := "malformed"
	target := filepath.Join(vault, filepath.FromSlash(CanonicalPath(key)))
	if err := os.WriteFile(target, []byte("broken canonical content"), 0644); err != nil {
		t.Fatal(err)
	}
	report, err := AuditInbox(vault)
	if err != nil {
		t.Fatal(err)
	}
	if len(report.InvalidCanonical) != 1 || report.InvalidCanonical[0] != CanonicalPath(key) {
		t.Fatalf("missing invalid canonical diagnostic: %+v", report)
	}
	actual, _ := os.ReadFile(target)
	if string(actual) != "broken canonical content" {
		t.Fatalf("audit changed malformed file: %q", actual)
	}
}

func TestAuditInboxRejectsSymlinkInbox(t *testing.T) {
	vault := t.TempDir()
	if err := os.Symlink(t.TempDir(), filepath.Join(vault, "00-inbox")); err != nil {
		t.Skipf("symlinks unsupported: %v", err)
	}
	if _, err := AuditInbox(vault); err == nil {
		t.Fatal("audit must not follow an inbox symlink")
	}
}

func TestAuditInboxEmptyMissingInbox(t *testing.T) {
	report, err := AuditInbox(t.TempDir())
	if err != nil || report.KeyedCaptures != 0 || len(report.Duplicates) != 0 {
		t.Fatalf("missing inbox should be harmless: %+v, %v", report, err)
	}
}

func TestAuditInboxFlagsNonStringLegacyExternalIDs(t *testing.T) {
	vault := t.TempDir()
	inbox := filepath.Join(vault, "00-inbox")
	if err := os.MkdirAll(inbox, 0755); err != nil {
		t.Fatal(err)
	}
	name := filepath.Join(inbox, "numeric-key.md")
	body := "---\nid: cap_123\ntype: capture\nexternal_id: 123\n---\n\nOriginal\n"
	if err := os.WriteFile(name, []byte(body), 0644); err != nil {
		t.Fatal(err)
	}
	report, err := AuditInbox(vault)
	if err != nil {
		t.Fatal(err)
	}
	if len(report.MalformedFiles) != 1 || report.MalformedFiles[0] != "00-inbox/numeric-key.md" {
		t.Fatalf("non-string legacy external ID must prevent green migration audit: %+v", report)
	}
	got, err := os.ReadFile(name)
	if err != nil || string(got) != body {
		t.Fatalf("migration audit modified legacy note: %v", err)
	}
}

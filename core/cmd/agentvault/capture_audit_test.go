package main

import (
	"bytes"
	"encoding/json"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/agentvault/core/internal/capturekey"
)

func TestCaptureAuditCommandListsDuplicatesWithoutMutatingVault(t *testing.T) {
	vp := setupTestVault(t)
	oldVault := vaultPath
	vaultPath = vp
	defer func() { vaultPath = oldVault }()
	inbox := filepath.Join(vp, "00-inbox")
	if err := os.MkdirAll(inbox, 0755); err != nil {
		t.Fatal(err)
	}
	key := "private-event-credential"
	body := "---\nid: cap_123\ntype: capture\ntitle: Test\nexternal_id: " + `"` + key + `"` + "\n---\n\nComplete\n"
	for _, name := range []string{"legacy-a.md", "legacy-b.md"} {
		if err := os.WriteFile(filepath.Join(inbox, name), []byte(body), 0644); err != nil {
			t.Fatal(err)
		}
	}
	var buf bytes.Buffer
	cmd := cobraCommandArgs(nil)
	cmd.SetOut(&buf)
	err := runCaptureAudit(cmd)
	if err == nil || !strings.Contains(err.Error(), "duplicate") {
		t.Fatalf("expected failing duplicate preflight: %v", err)
	}
	var result capturekey.AuditReport
	if err := json.Unmarshal(buf.Bytes(), &result); err != nil {
		t.Fatal(err)
	}
	if len(result.Duplicates) != 1 || result.KeyedCaptures != 2 {
		t.Fatalf("report: %+v", result)
	}
	if strings.Contains(buf.String(), key) {
		t.Fatal("audit response revealed external_id instead of SHA-256 fingerprint")
	}
	for _, name := range []string{"legacy-a.md", "legacy-b.md"} {
		content, err := os.ReadFile(filepath.Join(inbox, name))
		if err != nil || string(content) != body {
			t.Fatal("audit modified user capture")
		}
	}
}

func TestCaptureAuditCommandCleanVault(t *testing.T) {
	vp := setupTestVault(t)
	oldVault := vaultPath
	vaultPath = vp
	defer func() { vaultPath = oldVault }()
	var buf bytes.Buffer
	cmd := cobraCommandArgs(nil)
	cmd.SetOut(&buf)
	if err := runCaptureAudit(cmd); err != nil {
		t.Fatalf("clean audit: %v", err)
	}
	var report capturekey.AuditReport
	if err := json.Unmarshal(buf.Bytes(), &report); err != nil {
		t.Fatal(err)
	}
	if report.KeyedCaptures != 0 || len(report.InvalidCanonical) != 0 {
		t.Fatalf("unexpected issues: %+v", report)
	}
}

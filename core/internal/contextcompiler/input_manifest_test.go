package contextcompiler

import (
	"crypto/sha256"
	"encoding/hex"
	"errors"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/agentvault/core/internal/contract"
	"github.com/agentvault/core/internal/memory"
)

func TestCompileUnifiedBuildsCandidateInputManifestBeforeBudgeting(t *testing.T) {
	compiler, _, database, vault := setupCompiler(t)
	defer database.Close()

	noteBody := `---
id: manifest_note
type: note
title: Production runbook
project: manifest
status: active
tags: [production]
created: 2026-09-10T10:00:00Z
updated: 2026-09-10T10:00:00Z
---
Production deployment rollback runbook.
`
	memoryBody := `---
id: manifest_memory
type: note
title: Production preference
project: manifest
status: active
tags: [production]
workspace_id: manifest
memory_class: semantic
memory_kind: preference
memory_confidence: 0.95
created: 2026-09-10T10:00:00Z
updated: 2026-09-10T10:00:00Z
---
Prefer immutable production deployments.
`
	writeAndIndexNote(t, database, vault, "manifest-note.md", noteBody)
	writeAndIndexNote(t, database, vault, "manifest-memory.md", memoryBody)

	viewDir := filepath.Join(vault, ".agentvault", "views")
	if err := os.MkdirAll(viewDir, 0o755); err != nil {
		t.Fatal(err)
	}
	viewBytes := []byte("version: 1\nname: Production manifest\nquery:\n  projects: [manifest]\n  statuses: [active]\n  tags: [production]\n")
	if err := os.WriteFile(filepath.Join(viewDir, "production-manifest.yaml"), viewBytes, 0o644); err != nil {
		t.Fatal(err)
	}

	req := contract.CompileContextRequest{
		Task:        "production deployment",
		Project:     "manifest",
		ViewID:      "production-manifest",
		TokenBudget: 256,
		MaxItems:    1,
		AsOf:        "2026-09-10T12:00:00Z",
	}
	bundle, err := CompileUnified(compiler.WithVaultPath(vault), memory.NewStore(database), req)
	if err != nil {
		t.Fatalf("CompileUnified: %v", err)
	}
	if len(bundle.Items) != 1 {
		t.Fatalf("expected token/item budget to include one item, got %d", len(bundle.Items))
	}
	if bundle.InputManifest == nil {
		t.Fatal("expected input manifest for saved-view compilation")
	}
	manifest := bundle.InputManifest
	if manifest.Version != "1" || manifest.ViewID != "production-manifest" {
		t.Fatalf("unexpected manifest identity: %+v", manifest)
	}
	if manifest.ViewContentHash != bundle.ViewContentHash {
		t.Fatalf("manifest view hash = %q, bundle view hash = %q", manifest.ViewContentHash, bundle.ViewContentHash)
	}
	if len(manifest.Sources) != 2 {
		t.Fatalf("expected both eligible file-backed candidates before budgeting, got %+v", manifest.Sources)
	}
	if len(manifest.ManifestHash) != 64 {
		t.Fatalf("expected SHA-256 manifest hash, got %q", manifest.ManifestHash)
	}

	wantHashes := map[string]string{}
	for name, body := range map[string]string{
		"10-notes/manifest-note.md":   noteBody,
		"10-notes/manifest-memory.md": memoryBody,
	} {
		sum := sha256.Sum256([]byte(body))
		wantHashes[name] = hex.EncodeToString(sum[:])
	}
	gotKinds := map[string]string{}
	for _, source := range manifest.Sources {
		if source.ContentHash != wantHashes[source.Path] {
			t.Fatalf("source %s hash = %q, want %q", source.Path, source.ContentHash, wantHashes[source.Path])
		}
		gotKinds[source.ID] = source.Kind
	}
	if gotKinds["manifest_note"] != "note" || gotKinds["manifest_memory"] != "memory" {
		t.Fatalf("unexpected source kinds: %+v", gotKinds)
	}

	repeat, err := CompileUnified(compiler.WithVaultPath(vault), memory.NewStore(database), req)
	if err != nil {
		t.Fatalf("repeat CompileUnified: %v", err)
	}
	if repeat.InputManifest == nil || repeat.InputManifest.ManifestHash != manifest.ManifestHash {
		t.Fatalf("manifest hash is not deterministic: first=%q repeat=%+v", manifest.ManifestHash, repeat.InputManifest)
	}
}

func TestCompileUnifiedInputManifestPinDetectsUnreturnedSourceDrift(t *testing.T) {
	compiler, _, database, vault := setupCompiler(t)
	defer database.Close()

	firstBody := `---
id: drift_primary
type: note
title: Primary production runbook
project: drift
status: active
created: 2026-09-10T10:00:00Z
updated: 2026-09-10T11:00:00Z
---
Primary production deployment instructions.
`
	secondBody := `---
id: drift_secondary
type: note
title: Secondary production runbook
project: drift
status: active
created: 2026-09-10T10:00:00Z
updated: 2026-09-10T10:00:00Z
---
Secondary fallback instructions.
`
	writeAndIndexNote(t, database, vault, "drift-primary.md", firstBody)
	writeAndIndexNote(t, database, vault, "drift-secondary.md", secondBody)

	viewDir := filepath.Join(vault, ".agentvault", "views")
	if err := os.MkdirAll(viewDir, 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(viewDir, "drift.yaml"), []byte("version: 1\nname: Drift\nquery:\n  projects: [drift]\n  statuses: [active]\n"), 0o644); err != nil {
		t.Fatal(err)
	}

	req := contract.CompileContextRequest{
		Task:        "production deployment",
		Project:     "drift",
		ViewID:      "drift",
		TokenBudget: 256,
		MaxItems:    1,
		AsOf:        "2026-09-10T12:00:00Z",
	}
	first, err := CompileUnified(compiler.WithVaultPath(vault), memory.NewStore(database), req)
	if err != nil {
		t.Fatal(err)
	}
	if first.InputManifest == nil || len(first.InputManifest.Sources) != 2 {
		t.Fatalf("expected two manifest candidates, got %+v", first.InputManifest)
	}

	returned := map[string]bool{}
	for _, item := range first.Items {
		returned[item.ID] = true
	}
	var unreturned contract.ContextInputSource
	for _, source := range first.InputManifest.Sources {
		if !returned[source.ID] {
			unreturned = source
			break
		}
	}
	if unreturned.ID == "" {
		t.Fatalf("expected at least one source excluded by maxItems: items=%+v manifest=%+v", first.Items, first.InputManifest)
	}

	rawPath := filepath.Join(vault, filepath.FromSlash(unreturned.Path))
	raw, err := os.ReadFile(rawPath)
	if err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(rawPath, append(raw, []byte("\nChanged outside the current returned context.\n")...), 0o644); err != nil {
		t.Fatal(err)
	}

	req.ExpectedInputManifestHash = first.InputManifest.ManifestHash
	_, err = CompileUnified(compiler.WithVaultPath(vault), memory.NewStore(database), req)
	if !errors.Is(err, ErrInputManifestHashMismatch) {
		t.Fatalf("expected ErrInputManifestHashMismatch after unreturned source drift, got %v", err)
	}
}

func TestCompileRejectsExpectedInputManifestHashWithoutViewID(t *testing.T) {
	compiler, _, database, vault := setupCompiler(t)
	defer database.Close()

	_, err := compiler.WithVaultPath(vault).Compile(contract.CompileContextRequest{
		Task:                      "invalid manifest pin",
		ExpectedInputManifestHash: strings.Repeat("a", 64),
	})
	if err == nil || !strings.Contains(err.Error(), "requires viewId") {
		t.Fatalf("expected manifest pin without viewId to fail, got %v", err)
	}
}

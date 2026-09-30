package contextcompiler

import (
	"errors"
	"os"
	"path/filepath"
	"testing"

	"github.com/agentvault/core/internal/contract"
	"github.com/agentvault/core/internal/memory"
)

func TestCompileUnifiedBuildsDeterministicContextSnapshot(t *testing.T) {
	compiler, store, database, vault := setupCompiler(t)
	defer database.Close()

	writeAndIndexNote(t, database, vault, "snapshot.md", `---
id: snapshot_note
type: note
title: Snapshot runbook
project: snapshot
status: active
tags: [production]
created: 2026-09-30T10:00:00Z
updated: 2026-09-30T10:00:00Z
---
Production deployment rollback runbook.
`)

	viewDir := filepath.Join(vault, ".agentvault", "views")
	if err := os.MkdirAll(viewDir, 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(viewDir, "snapshot.yaml"), []byte("version: 1\nname: Snapshot\nquery:\n  projects: [snapshot]\n  statuses: [active]\n"), 0o644); err != nil {
		t.Fatal(err)
	}

	confidence := 0.9
	if _, err := store.RecordMemory(contract.CreateMemoryRequest{
		ID:          "snapshot_memory",
		MemoryClass: "semantic",
		MemoryKind:  "decision",
		ScopeType:   "project",
		ScopeID:     "snapshot",
		Content:     "Use immutable production deployments.",
		Confidence:  &confidence,
	}); err != nil {
		t.Fatal(err)
	}

	req := contract.CompileContextRequest{
		Task:        "prepare production deployment",
		Project:     "snapshot",
		ViewID:      "snapshot",
		TokenBudget: 1000,
		MaxItems:    10,
		AsOf:        "2026-09-30T12:00:00Z",
	}

	first, err := CompileUnified(compiler.WithVaultPath(vault), memory.NewStore(database), req)
	if err != nil {
		t.Fatalf("CompileUnified: %v", err)
	}
	if first.StructuredInputManifest == nil {
		t.Fatal("expected structured input manifest")
	}
	if len(first.StructuredInputManifest.ManifestHash) != 64 {
		t.Fatalf("expected structured manifest SHA-256, got %q", first.StructuredInputManifest.ManifestHash)
	}
	if len(first.ContextSnapshotHash) != 64 {
		t.Fatalf("expected context snapshot SHA-256, got %q", first.ContextSnapshotHash)
	}

	foundStructuredMemory := false
	for _, source := range first.StructuredInputManifest.Sources {
		if source.Kind == "memory" && source.ID == "snapshot_memory" {
			foundStructuredMemory = true
			break
		}
	}
	if !foundStructuredMemory {
		t.Fatalf("structured manifest omitted relevant memory: %+v", first.StructuredInputManifest.Sources)
	}

	repeat, err := CompileUnified(compiler.WithVaultPath(vault), memory.NewStore(database), req)
	if err != nil {
		t.Fatalf("repeat CompileUnified: %v", err)
	}
	if repeat.ContextSnapshotHash != first.ContextSnapshotHash {
		t.Fatalf("snapshot hash is not deterministic: first=%q repeat=%q", first.ContextSnapshotHash, repeat.ContextSnapshotHash)
	}
	if repeat.StructuredInputManifest == nil || repeat.StructuredInputManifest.ManifestHash != first.StructuredInputManifest.ManifestHash {
		t.Fatalf("structured manifest is not deterministic: first=%+v repeat=%+v", first.StructuredInputManifest, repeat.StructuredInputManifest)
	}
}

func TestCompileUnifiedSnapshotPinDetectsStructuredCandidateDrift(t *testing.T) {
	compiler, store, database, vault := setupCompiler(t)
	defer database.Close()

	writeAndIndexNote(t, database, vault, "snapshot-drift.md", `---
id: snapshot_drift_note
type: note
title: Snapshot drift runbook
project: snapshot-drift
status: active
created: 2026-09-30T10:00:00Z
updated: 2026-09-30T10:00:00Z
---
Production deployment rollback instructions.
`)

	viewDir := filepath.Join(vault, ".agentvault", "views")
	if err := os.MkdirAll(viewDir, 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(viewDir, "snapshot-drift.yaml"), []byte("version: 1\nname: Snapshot drift\nquery:\n  projects: [snapshot-drift]\n  statuses: [active]\n"), 0o644); err != nil {
		t.Fatal(err)
	}

	req := contract.CompileContextRequest{
		Task:        "production deployment",
		Project:     "snapshot-drift",
		ViewID:      "snapshot-drift",
		TokenBudget: 256,
		MaxItems:    1,
		AsOf:        "2026-09-30T12:00:00Z",
	}

	first, err := CompileUnified(compiler.WithVaultPath(vault), memory.NewStore(database), req)
	if err != nil {
		t.Fatal(err)
	}
	if first.InputManifest == nil || first.StructuredInputManifest == nil || first.ContextSnapshotHash == "" {
		t.Fatalf("missing snapshot inputs: %+v", first)
	}
	fileManifestHash := first.InputManifest.ManifestHash

	confidence := 0.1
	if _, err := store.RecordMemory(contract.CreateMemoryRequest{
		ID:          "snapshot_drift_memory",
		MemoryClass: "episodic",
		MemoryKind:  "observation",
		ScopeType:   "project",
		ScopeID:     "snapshot-drift",
		Content:     "Low-priority observation that should remain outside maxItems=1.",
		Confidence:  &confidence,
	}); err != nil {
		t.Fatal(err)
	}

	req.ExpectedContextSnapshotHash = first.ContextSnapshotHash
	_, err = CompileUnified(compiler.WithVaultPath(vault), memory.NewStore(database), req)
	if !errors.Is(err, ErrContextSnapshotHashMismatch) {
		t.Fatalf("expected ErrContextSnapshotHashMismatch, got %v", err)
	}

	withoutPin := req
	withoutPin.ExpectedContextSnapshotHash = ""
	changed, err := CompileUnified(compiler.WithVaultPath(vault), memory.NewStore(database), withoutPin)
	if err != nil {
		t.Fatal(err)
	}
	if changed.InputManifest == nil || changed.InputManifest.ManifestHash != fileManifestHash {
		t.Fatalf("file manifest unexpectedly changed: before=%q after=%+v", fileManifestHash, changed.InputManifest)
	}
	if changed.StructuredInputManifest == nil || changed.StructuredInputManifest.ManifestHash == first.StructuredInputManifest.ManifestHash {
		t.Fatalf("structured manifest did not detect drift: before=%+v after=%+v", first.StructuredInputManifest, changed.StructuredInputManifest)
	}
	if changed.ContextSnapshotHash == first.ContextSnapshotHash {
		t.Fatal("context snapshot hash did not change after structured candidate drift")
	}
}

func TestContextSnapshotHashCommitsCompileParameters(t *testing.T) {
	compiler, _, database, vault := setupCompiler(t)
	defer database.Close()

	writeAndIndexNote(t, database, vault, "params.md", `---
id: snapshot_params_note
type: note
title: Snapshot params note
project: snapshot-params
status: active
created: 2026-09-30T10:00:00Z
updated: 2026-09-30T10:00:00Z
---
Deployment instructions.
`)

	viewDir := filepath.Join(vault, ".agentvault", "views")
	if err := os.MkdirAll(viewDir, 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(viewDir, "snapshot-params.yaml"), []byte("version: 1\nname: Snapshot params\nquery:\n  projects: [snapshot-params]\n"), 0o644); err != nil {
		t.Fatal(err)
	}

	base := contract.CompileContextRequest{
		Task:        "deployment instructions",
		Project:     "snapshot-params",
		ViewID:      "snapshot-params",
		TokenBudget: 1000,
		MaxItems:    10,
		AsOf:        "2026-09-30T12:00:00Z",
	}
	first, err := CompileUnified(compiler.WithVaultPath(vault), memory.NewStore(database), base)
	if err != nil {
		t.Fatal(err)
	}

	changedReq := base
	changedReq.Task = "different deployment objective"
	changed, err := CompileUnified(compiler.WithVaultPath(vault), memory.NewStore(database), changedReq)
	if err != nil {
		t.Fatal(err)
	}
	if changed.ContextSnapshotHash == first.ContextSnapshotHash {
		t.Fatal("snapshot hash did not commit task parameters")
	}
}

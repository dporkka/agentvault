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

func TestCompileUnifiedSavedViewScopesNotesAndMarkdownMemories(t *testing.T) {
	compiler, _, database, vault := setupCompiler(t)
	defer database.Close()

	writeAndIndexNote(t, database, vault, "allowed-note.md", `---
id: note_allowed
type: note
title: Allowed deployment runbook
project: adacavo
status: active
tags: [production]
created: 2026-09-10T10:00:00Z
updated: 2026-09-10T10:00:00Z
---
Deployment workflow rollback verification for production.
`)
	writeAndIndexNote(t, database, vault, "hidden-note.md", `---
id: note_hidden
type: note
title: Hidden deployment runbook
project: adacavo
status: draft
tags: [production]
created: 2026-09-10T10:00:00Z
updated: 2026-09-10T10:00:00Z
---
Deployment workflow rollback verification from a draft.
`)
	writeAndIndexNote(t, database, vault, "allowed-memory.md", `---
id: memory_allowed
type: note
title: Allowed deployment preference
project: adacavo
status: active
tags: [production]
workspace_id: adacavo
memory_class: semantic
memory_kind: preference
memory_confidence: 0.95
created: 2026-09-10T10:00:00Z
updated: 2026-09-10T10:00:00Z
---
Use Cloud Run for production deployment.
`)
	writeAndIndexNote(t, database, vault, "hidden-memory.md", `---
id: memory_hidden
type: note
title: Hidden deployment preference
project: adacavo
status: draft
tags: [production]
workspace_id: adacavo
memory_class: semantic
memory_kind: preference
memory_confidence: 0.99
created: 2026-09-10T10:00:00Z
updated: 2026-09-10T10:00:00Z
---
Use the hidden deployment target.
`)

	viewDir := filepath.Join(vault, ".agentvault", "views")
	if err := os.MkdirAll(viewDir, 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(viewDir, "production.yaml"), []byte(`version: 1
name: Production context
query:
  projects: [adacavo]
  statuses: [active]
  tags: [production]
limit: 100
`), 0o644); err != nil {
		t.Fatal(err)
	}

	bundle, err := CompileUnified(
		compiler.WithVaultPath(vault),
		memory.NewStore(database),
		contract.CompileContextRequest{
			Task:        "deployment workflow",
			Project:     "adacavo",
			ViewID:      "production",
			TokenBudget: 4000,
			MaxItems:    20,
			AsOf:        "2026-09-10T12:00:00Z",
		},
	)
	if err != nil {
		t.Fatalf("CompileUnified: %v", err)
	}
	if bundle.ViewID != "production" {
		t.Fatalf("bundle viewId = %q, want production", bundle.ViewID)
	}

	found := map[string]bool{}
	for _, item := range bundle.Items {
		found[item.ID] = true
	}
	if !found["note_allowed"] {
		t.Fatal("saved view excluded matching ordinary note")
	}
	if !found["memory_allowed"] {
		t.Fatal("saved view excluded matching Markdown memory")
	}
	if found["note_hidden"] || found["memory_hidden"] {
		t.Fatalf("saved view leaked excluded file-backed context: %+v", found)
	}
}

func TestCompileUnifiedSavedViewIntersectsExplicitProject(t *testing.T) {
	compiler, _, database, vault := setupCompiler(t)
	defer database.Close()

	writeAndIndexNote(t, database, vault, "alpha.md", `---
id: note_alpha
type: note
title: Shared architecture alpha
project: alpha
status: active
created: 2026-09-10T10:00:00Z
updated: 2026-09-10T10:00:00Z
---
Shared architecture decision for alpha.
`)
	writeAndIndexNote(t, database, vault, "beta.md", `---
id: note_beta
type: note
title: Shared architecture beta
project: beta
status: active
created: 2026-09-10T10:00:00Z
updated: 2026-09-10T10:00:00Z
---
Shared architecture decision for beta.
`)

	viewDir := filepath.Join(vault, ".agentvault", "views")
	if err := os.MkdirAll(viewDir, 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(viewDir, "cross-project.yaml"), []byte(`version: 1
name: Cross project
query:
  projects: [alpha, beta]
  statuses: [active]
`), 0o644); err != nil {
		t.Fatal(err)
	}

	bundle, err := CompileUnified(
		compiler.WithVaultPath(vault),
		memory.NewStore(database),
		contract.CompileContextRequest{
			Task:    "shared architecture",
			Project: "alpha",
			ViewID:  "cross-project",
			AsOf:    "2026-09-10T12:00:00Z",
		},
	)
	if err != nil {
		t.Fatalf("CompileUnified: %v", err)
	}

	for _, item := range bundle.Items {
		if item.ID == "note_beta" {
			t.Fatalf("saved view bypassed explicit project scope: %+v", item)
		}
	}
}

func TestCompileUnifiedRejectsMissingSavedView(t *testing.T) {
	compiler, _, database, vault := setupCompiler(t)
	defer database.Close()

	_, err := CompileUnified(
		compiler.WithVaultPath(vault),
		memory.NewStore(database),
		contract.CompileContextRequest{
			Task:   "deployment workflow",
			ViewID: "missing-view",
		},
	)
	if err == nil || !strings.Contains(err.Error(), "missing-view") {
		t.Fatalf("expected missing saved view error, got %v", err)
	}
}

func TestCompileSavedViewScopesDirectCompilerNotes(t *testing.T) {
	compiler, _, database, vault := setupCompiler(t)
	defer database.Close()

	writeAndIndexNote(t, database, vault, "included.md", `---
id: direct_included
type: note
title: Included direct note
project: direct
status: active
created: 2026-09-10T10:00:00Z
updated: 2026-09-10T10:00:00Z
---
Direct compiler saved view example.
`)
	writeAndIndexNote(t, database, vault, "excluded.md", `---
id: direct_excluded
type: note
title: Excluded direct note
project: direct
status: draft
created: 2026-09-10T10:00:00Z
updated: 2026-09-10T10:00:00Z
---
Direct compiler saved view example.
`)

	viewDir := filepath.Join(vault, ".agentvault", "views")
	if err := os.MkdirAll(viewDir, 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(viewDir, "direct-active.yaml"), []byte(`version: 1
name: Direct active
query:
  projects: [direct]
  statuses: [active]
`), 0o644); err != nil {
		t.Fatal(err)
	}

	bundle, err := compiler.WithVaultPath(vault).Compile(contract.CompileContextRequest{
		Task:    "saved view example",
		Project: "direct",
		ViewID:  "direct-active",
		AsOf:    "2026-09-10T12:00:00Z",
	})
	if err != nil {
		t.Fatalf("Compile: %v", err)
	}

	foundIncluded := false
	for _, item := range bundle.Items {
		if item.ID == "direct_excluded" {
			t.Fatalf("direct compiler leaked excluded saved-view note: %+v", item)
		}
		if item.ID == "direct_included" {
			foundIncluded = true
		}
	}
	if !foundIncluded {
		t.Fatal("direct compiler did not include saved-view note")
	}
}

func TestCompileUnifiedRecordsAndPinsSavedViewDefinition(t *testing.T) {
	compiler, _, database, vault := setupCompiler(t)
	defer database.Close()

	writeAndIndexNote(t, database, vault, "pin.md", `---
id: pin_note
type: note
title: Pinned scope note
project: pin
status: active
created: 2026-09-10T10:00:00Z
updated: 2026-09-10T10:00:00Z
---
Pinned scope content.
`)

	viewDir := filepath.Join(vault, ".agentvault", "views")
	if err := os.MkdirAll(viewDir, 0o755); err != nil {
		t.Fatal(err)
	}
	viewBytes := []byte("version: 1\nname: Pinned scope\nquery:\n  projects: [pin]\n  statuses: [active]\n")
	viewPath := filepath.Join(viewDir, "pinned.yaml")
	if err := os.WriteFile(viewPath, viewBytes, 0o644); err != nil {
		t.Fatal(err)
	}
	sum := sha256.Sum256(viewBytes)
	expectedHash := hex.EncodeToString(sum[:])

	bundle, err := CompileUnified(
		compiler.WithVaultPath(vault),
		memory.NewStore(database),
		contract.CompileContextRequest{
			Task:                    "pinned scope content",
			Project:                 "pin",
			ViewID:                  "pinned",
			ExpectedViewContentHash: expectedHash,
			AsOf:                    "2026-09-10T12:00:00Z",
		},
	)
	if err != nil {
		t.Fatalf("CompileUnified: %v", err)
	}
	if bundle.ViewVersion != 1 {
		t.Fatalf("viewVersion = %d, want 1", bundle.ViewVersion)
	}
	if bundle.ViewContentHash != expectedHash {
		t.Fatalf("viewContentHash = %q, want %q", bundle.ViewContentHash, expectedHash)
	}

	changed := append(append([]byte{}, viewBytes...), []byte("# changed\n")...)
	if err := os.WriteFile(viewPath, changed, 0o644); err != nil {
		t.Fatal(err)
	}
	_, err = CompileUnified(
		compiler.WithVaultPath(vault),
		memory.NewStore(database),
		contract.CompileContextRequest{
			Task:                    "pinned scope content",
			Project:                 "pin",
			ViewID:                  "pinned",
			ExpectedViewContentHash: expectedHash,
			AsOf:                    "2026-09-10T12:00:00Z",
		},
	)
	if !errors.Is(err, ErrViewContentHashMismatch) {
		t.Fatalf("expected ErrViewContentHashMismatch, got %v", err)
	}
}

func TestCompileRejectsExpectedViewHashWithoutViewID(t *testing.T) {
	compiler, _, database, vault := setupCompiler(t)
	defer database.Close()

	_, err := compiler.WithVaultPath(vault).Compile(contract.CompileContextRequest{
		Task:                    "invalid pin",
		ExpectedViewContentHash: strings.Repeat("a", 64),
	})
	if err == nil || !strings.Contains(err.Error(), "requires viewId") {
		t.Fatalf("expected view hash without viewId to fail, got %v", err)
	}
}

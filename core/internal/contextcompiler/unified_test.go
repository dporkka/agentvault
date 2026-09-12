package contextcompiler

import (
	"testing"

	"github.com/agentvault/core/internal/contract"
	"github.com/agentvault/core/internal/memory"
)

func TestCompileUnifiedIncludesVisibleMarkdownMemoryWithoutScopeLeak(t *testing.T) {
	compiler, _, database, vault := setupCompiler(t)
	defer database.Close()

	writeAndIndexNote(t, database, vault, "allowed-memory.md", `---
id: note_allowed_memory
type: note
title: Preferred deployment target
project: adacavo
workspace_id: adacavo
memory_class: semantic
memory_kind: preference
memory_confidence: 0.95
provenance:
  source_type: human
  source_ref: conversation-allowed
  actor: user
  captured_at: 2026-09-10T10:00:00Z
observed_at: 2026-09-10T10:00:00Z
---
Use Cloud Run as the preferred deployment target for the deployment workflow.
`)

	writeAndIndexNote(t, database, vault, "hidden-memory.md", `---
id: note_hidden_memory
type: note
title: Hidden deployment target
project: adacavo
workspace_id: another-workspace
memory_class: semantic
memory_kind: preference
memory_confidence: 0.99
provenance:
  source_type: human
  source_ref: conversation-hidden
  actor: user
  captured_at: 2026-09-10T10:00:00Z
observed_at: 2026-09-10T10:00:00Z
---
Use the hidden deployment target for the deployment workflow.
`)

	writeAndIndexNote(t, database, vault, "deployment-runbook.md", `---
id: note_deployment_runbook
type: note
title: Deployment Runbook
project: adacavo
created: 2026-09-10T10:00:00Z
updated: 2026-09-10T10:00:00Z
---
The deployment workflow includes health checks and rollback verification.
`)

	bundle, err := CompileUnified(
		compiler,
		memory.NewStore(database),
		contract.CompileContextRequest{
			Task:        "deployment target workflow",
			Project:     "adacavo",
			TokenBudget: 4000,
			MaxItems:    20,
			AsOf:        "2026-09-10T12:00:00Z",
		},
	)
	if err != nil {
		t.Fatalf("CompileUnified: %v", err)
	}
	if bundle.WorkspaceID != "adacavo" {
		t.Fatalf("workspace fallback = %q, want adacavo", bundle.WorkspaceID)
	}

	allowedCount := 0
	genericFound := false
	for _, item := range bundle.Items {
		switch item.ID {
		case "note_allowed_memory":
			allowedCount++
			if item.Kind != "memory" {
				t.Fatalf("visible classified memory re-entered as %q: %+v", item.Kind, item)
			}
			if item.Metadata["memorySource"] != "markdown" || item.Metadata["memoryClass"] != "semantic" || item.Metadata["memoryKind"] != "preference" {
				t.Fatalf("unexpected Markdown memory metadata: %+v", item.Metadata)
			}
			if item.Provenance == nil || item.Provenance.SourceID != "conversation-allowed" {
				t.Fatalf("Markdown memory provenance missing: %+v", item.Provenance)
			}
		case "note_hidden_memory":
			t.Fatalf("workspace-scoped memory leaked into context: %+v", item)
		case "note_deployment_runbook":
			if item.Kind == "note" {
				genericFound = true
			}
		}
	}
	if allowedCount != 1 {
		t.Fatalf("visible Markdown memory count = %d, want exactly 1", allowedCount)
	}
	if !genericFound {
		t.Fatal("expected ordinary matching note to remain available through generic note search")
	}
}

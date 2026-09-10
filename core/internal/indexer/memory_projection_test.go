package indexer

import (
	"context"
	"database/sql"
	"encoding/json"
	"testing"

	"github.com/agentvault/core/internal/memory"
)

func TestIndexerProjectsMemoryFrontmatter(t *testing.T) {
	vaultPath, database, cleanup := setupTestVault(t)
	defer cleanup()
	idx := New(database, vaultPath)

	writeNote(t, vaultPath, "10-notes/old.md", `---
id: fact-old
type: note
title: Old fact
memory_kind: fact
confidence: 0.6
---
Old fact.
`)
	writeNote(t, vaultPath, "10-notes/new.md", `---
id: fact-new
type: note
title: New fact
workspace_id: workspace-a
agent_id: agent-a
session_id: session-a
memory_kind: fact
confidence: 0.93
provenance:
  source_type: conversation
  source_ref: conversation-7
  actor: user
  model: model-x
  captured_at: 2026-09-10T12:00:00+02:00
observed_at: 2026-09-10T12:05:00+02:00
valid_from: 2026-09-10T12:05:00+02:00
valid_to: 2026-10-10T12:05:00+02:00
supersedes: [fact-old]
supersession_reason: newer explicit statement
pinned: true
---
New fact.
`)

	result := mustIndex(t, idx, IndexOptions{})
	if len(result.Errors) != 0 {
		t.Fatalf("index errors: %#v", result.Errors)
	}

	var workspaceID, agentID, sessionID, kind string
	var confidence float64
	var provenanceJSON, observedAt, validFrom, validTo, frontmatterJSON sql.NullString
	if err := database.QueryRow(`
		SELECT workspace_id, agent_id, session_id, memory_kind, confidence,
			provenance_json, observed_at, valid_from, valid_to, frontmatter_json
		FROM notes WHERE id = 'fact-new'
	`).Scan(
		&workspaceID, &agentID, &sessionID, &kind, &confidence,
		&provenanceJSON, &observedAt, &validFrom, &validTo, &frontmatterJSON,
	); err != nil {
		t.Fatalf("read projected metadata: %v", err)
	}

	if workspaceID != "workspace-a" || agentID != "agent-a" || sessionID != "session-a" {
		t.Fatalf("projected scope = %q/%q/%q", workspaceID, agentID, sessionID)
	}
	if kind != "fact" || confidence != 0.93 {
		t.Fatalf("kind/confidence = %q/%v", kind, confidence)
	}
	if observedAt.String != "2026-09-10T10:05:00Z" || validFrom.String != "2026-09-10T10:05:00Z" || validTo.String != "2026-10-10T10:05:00Z" {
		t.Fatalf("timestamps were not normalized to UTC: observed=%q from=%q to=%q", observedAt.String, validFrom.String, validTo.String)
	}

	var provenance memory.Provenance
	if !provenanceJSON.Valid || json.Unmarshal([]byte(provenanceJSON.String), &provenance) != nil {
		t.Fatalf("invalid provenance JSON: %q", provenanceJSON.String)
	}
	if provenance.SourceRef != "conversation-7" || provenance.CapturedAt != "2026-09-10T10:00:00Z" {
		t.Fatalf("projected provenance = %+v", provenance)
	}

	var raw map[string]interface{}
	if !frontmatterJSON.Valid || json.Unmarshal([]byte(frontmatterJSON.String), &raw) != nil {
		t.Fatalf("invalid frontmatter JSON: %q", frontmatterJSON.String)
	}
	if pinned, ok := raw["pinned"].(bool); !ok || !pinned {
		t.Fatalf("frontmatter JSON did not preserve inline pinned field: %#v", raw["pinned"])
	}

	store := memory.NewStore(database)
	record, err := store.Get(context.Background(), "fact-new")
	if err != nil {
		t.Fatalf("memory store cannot read indexed projection: %v", err)
	}
	if len(record.Supersedes) != 1 || record.Supersedes[0] != "fact-old" {
		t.Fatalf("supersession projection = %#v", record.Supersedes)
	}

	old, err := store.Get(context.Background(), "fact-old")
	if err != nil {
		t.Fatal(err)
	}
	if !old.Superseded {
		t.Fatal("fact-old should have an incoming supersession after indexing")
	}
}

func TestIndexerClearsRemovedMemoryMetadata(t *testing.T) {
	vaultPath, database, cleanup := setupTestVault(t)
	defer cleanup()
	idx := New(database, vaultPath)

	path := "10-notes/new.md"
	writeNote(t, vaultPath, path, `---
id: fact-new
type: note
title: New fact
workspace_id: workspace-a
memory_kind: fact
confidence: 0.9
supersedes: [fact-old]
---
New fact.
`)
	mustIndex(t, idx, IndexOptions{})

	writeNote(t, vaultPath, path, `---
id: fact-new
type: note
title: New fact
---
New fact without semantic metadata.
`)
	result := mustIndex(t, idx, IndexOptions{})
	if len(result.Errors) != 0 {
		t.Fatalf("reindex errors: %#v", result.Errors)
	}

	var workspaceID, kind string
	var confidence sql.NullFloat64
	if err := database.QueryRow(`SELECT workspace_id, memory_kind, confidence FROM notes WHERE id = 'fact-new'`).Scan(&workspaceID, &kind, &confidence); err != nil {
		t.Fatal(err)
	}
	if workspaceID != "" || kind != "" || confidence.Valid {
		t.Fatalf("removed metadata remained projected: workspace=%q kind=%q confidence=%v", workspaceID, kind, confidence)
	}

	var count int
	if err := database.QueryRow(`SELECT COUNT(*) FROM memory_supersessions WHERE superseding_note_id = 'fact-new'`).Scan(&count); err != nil {
		t.Fatal(err)
	}
	if count != 0 {
		t.Fatalf("removed supersession relations remained projected: %d", count)
	}
}

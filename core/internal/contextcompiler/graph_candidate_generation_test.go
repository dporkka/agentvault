package contextcompiler

import (
	"testing"

	"github.com/agentvault/core/internal/contract"
)

func TestCompileAddsOneHopGraphCandidateWithinProject(t *testing.T) {
	compiler, _, database, vault := setupCompiler(t)
	defer database.Close()

	writeAndIndexNote(t, database, vault, "graph-seed.md", `---
id: note_graph_seed
type: note
title: Release Deployment Checklist
project: adacavo
created: 2026-09-20T00:00:00Z
updated: 2026-09-29T00:00:00Z
---
Release deployment verification checklist.
`)
	writeAndIndexNote(t, database, vault, "graph-neighbor.md", `---
id: note_graph_neighbor
type: note
title: Recovery Runbook
project: adacavo
created: 2026-09-20T00:00:00Z
updated: 2026-09-29T00:00:00Z
---
Restore the previous stable artifact if validation fails.
`)
	writeAndIndexNote(t, database, vault, "graph-forbidden.md", `---
id: note_graph_forbidden
type: note
title: Other Project Recovery
project: other-project
created: 2026-09-20T00:00:00Z
updated: 2026-09-29T00:00:00Z
---
Private other-project recovery details.
`)

	if _, err := database.Exec(`INSERT INTO links (from_note_id, to_note_id, raw_target, link_type) VALUES ('note_graph_seed', 'note_graph_neighbor', 'Recovery Runbook', 'wiki')`); err != nil {
		t.Fatal(err)
	}
	if _, err := database.Exec(`INSERT INTO links (from_note_id, to_note_id, raw_target, link_type) VALUES ('note_graph_seed', 'note_graph_forbidden', 'Other Project Recovery', 'wiki')`); err != nil {
		t.Fatal(err)
	}

	bundle, err := compiler.Compile(contract.CompileContextRequest{
		Task:        "release deployment verification",
		Project:     "adacavo",
		TokenBudget: 1200,
		MaxItems:    10,
		AsOf:        "2099-01-01T00:00:00Z",
		Explain:     true,
	})
	if err != nil {
		t.Fatalf("Compile: %v", err)
	}

	foundNeighbor := false
	for _, item := range bundle.Items {
		if item.ID == "note_graph_forbidden" {
			t.Fatalf("cross-project graph candidate leaked into context: %+v", item)
		}
		if item.ID == "note_graph_neighbor" {
			foundNeighbor = true
			if item.Ranking == nil {
				t.Fatal("graph candidate missing ranking explanation")
			}
			foundGraph := false
			for _, component := range item.Ranking.Components {
				if component.Signal == "graphDistance" {
					foundGraph = true
					if component.Value != 1 {
						t.Fatalf("one-hop graph value must be 1, got %+v", component)
					}
				}
			}
			if !foundGraph {
				t.Fatalf("graph candidate missing graphDistance explanation: %+v", item.Ranking)
			}
		}
	}
	if !foundNeighbor {
		t.Fatalf("one-hop graph neighbor was not recovered: %+v", bundle.Items)
	}
}

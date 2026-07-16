package graph

import (
	"os"
	"path/filepath"
	"testing"

	"github.com/agentvault/core/internal/contract"
	"github.com/agentvault/core/internal/db"
)

func setupTestDB(t *testing.T) (*db.DB, func()) {
	t.Helper()
	tmpDir := t.TempDir()
	if err := os.MkdirAll(filepath.Join(tmpDir, ".agentvault"), 0755); err != nil {
		t.Fatalf("failed to create .agentvault dir: %v", err)
	}
	database, err := db.Open(tmpDir)
	if err != nil {
		t.Fatalf("failed to open db: %v", err)
	}
	if err := database.RunMigrations(); err != nil {
		t.Fatalf("failed to run migrations: %v", err)
	}

	// Insert 3 notes: A -> B -> C
	_, _ = database.Exec(`INSERT INTO files (id, path, content_hash, indexed_at) VALUES ('f_a', 'a.md', 'h', datetime('now'))`)
	_, _ = database.Exec(`INSERT INTO files (id, path, content_hash, indexed_at) VALUES ('f_b', 'b.md', 'h', datetime('now'))`)
	_, _ = database.Exec(`INSERT INTO files (id, path, content_hash, indexed_at) VALUES ('f_c', 'c.md', 'h', datetime('now'))`)

	_, _ = database.Exec(`INSERT INTO notes (id, file_id, title, type, status, project, updated_at, body) VALUES ('note_a', 'f_a', 'Note A', 'note', 'active', 'proj', datetime('now'), 'links to [[Note B]]')`)
	_, _ = database.Exec(`INSERT INTO notes (id, file_id, title, type, status, project, updated_at, body) VALUES ('note_b', 'f_b', 'Note B', 'note', 'active', 'proj', datetime('now'), 'links to [[Note C]]')`)
	_, _ = database.Exec(`INSERT INTO notes (id, file_id, title, type, status, project, updated_at, body) VALUES ('note_c', 'f_c', 'Note C', 'note', 'draft', '', datetime('now'), 'no outgoing links')`)

	// A -> B
	_, _ = database.Exec(`INSERT INTO links (from_note_id, to_note_id, raw_target, link_type) VALUES ('note_a', 'note_b', 'Note B', 'wiki')`)
	// B -> C
	_, _ = database.Exec(`INSERT INTO links (from_note_id, to_note_id, raw_target, link_type) VALUES ('note_b', 'note_c', 'Note C', 'wiki')`)

	return database, func() { database.Close() }
}

func TestBuildSubgraph_Depth1(t *testing.T) {
	db, cleanup := setupTestDB(t)
	defer cleanup()

	g, err := BuildSubgraph(db, "note_a", 1)
	if err != nil {
		t.Fatalf("BuildSubgraph failed: %v", err)
	}

	if len(g.Nodes) != 2 {
		t.Errorf("expected 2 nodes, got %d", len(g.Nodes))
	}
	if len(g.Edges) != 1 {
		t.Errorf("expected 1 edge, got %d", len(g.Edges))
	}

	// Check edge A -> B
	if g.Edges[0].FromID != "note_a" || g.Edges[0].ToID != "note_b" {
		t.Errorf("expected edge note_a -> note_b, got %s -> %s", g.Edges[0].FromID, g.Edges[0].ToID)
	}

	// Check node titles
	nodeMap := map[string]contract.GraphNode{}
	for _, n := range g.Nodes {
		nodeMap[n.ID] = n
	}
	if nodeMap["note_a"].Title != "Note A" {
		t.Errorf("expected title 'Note A', got %q", nodeMap["note_a"].Title)
	}
	if nodeMap["note_b"].Title != "Note B" {
		t.Errorf("expected title 'Note B', got %q", nodeMap["note_b"].Title)
	}
}

func TestBuildSubgraph_Depth2(t *testing.T) {
	db, cleanup := setupTestDB(t)
	defer cleanup()

	g, err := BuildSubgraph(db, "note_a", 2)
	if err != nil {
		t.Fatalf("BuildSubgraph failed: %v", err)
	}

	if len(g.Nodes) != 3 {
		t.Errorf("expected 3 nodes, got %d", len(g.Nodes))
	}
	if len(g.Edges) != 2 {
		t.Errorf("expected 2 edges, got %d", len(g.Edges))
	}
}

func TestBuildSubgraph_Depth0(t *testing.T) {
	db, cleanup := setupTestDB(t)
	defer cleanup()

	g, err := BuildSubgraph(db, "note_a", 0)
	if err != nil {
		t.Fatalf("BuildSubgraph failed: %v", err)
	}

	if len(g.Nodes) != 1 {
		t.Errorf("expected 1 node, got %d", len(g.Nodes))
	}
	if len(g.Edges) != 0 {
		t.Errorf("expected 0 edges, got %d", len(g.Edges))
	}
}

func TestBuildSubgraph_MissingCenter(t *testing.T) {
	db, cleanup := setupTestDB(t)
	defer cleanup()

	_, err := BuildSubgraph(db, "nonexistent", 1)
	if err == nil {
		t.Error("expected error for missing center note")
	}
}

func TestBuildSubgraph_Backlinks(t *testing.T) {
	db, cleanup := setupTestDB(t)
	defer cleanup()

	// From B's perspective, depth 1 should include A (backlink), C (outgoing), and both edges.
	g, err := BuildSubgraph(db, "note_b", 1)
	if err != nil {
		t.Fatalf("BuildSubgraph failed: %v", err)
	}

	if len(g.Nodes) != 3 {
		t.Errorf("expected 3 nodes, got %d", len(g.Nodes))
	}
	if len(g.Edges) != 2 {
		t.Errorf("expected 2 edges, got %d", len(g.Edges))
	}
}

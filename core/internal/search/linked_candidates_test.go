package search

import "testing"

func TestLinkedCandidatesReturnsOnlyOneHopWithinProject(t *testing.T) {
	database, cleanup := setupTestDB(t)
	defer cleanup()

	s := New(database)

	// Link a same-project note and a cross-project note to note_002.
	if _, err := database.Exec(`INSERT INTO links (from_note_id, to_note_id, raw_target, link_type) VALUES ('note_002', 'note_003', 'Three', 'wiki')`); err != nil {
		t.Fatal(err)
	}
	if _, err := database.Exec(`INSERT INTO links (from_note_id, to_note_id, raw_target, link_type) VALUES ('note_002', 'note_005', 'Five', 'wiki')`); err != nil {
		t.Fatal(err)
	}

	results, err := s.LinkedCandidates([]string{"note_002"}, "webapp", 10)
	if err != nil {
		t.Fatalf("LinkedCandidates: %v", err)
	}

	foundSameProject := false
	for _, result := range results {
		if result.Result.ID == "note_005" {
			t.Fatalf("cross-project link leaked into graph candidates: %+v", results)
		}
		if result.Result.ID == "note_003" {
			foundSameProject = true
			if result.Result.Project != "webapp" {
				t.Fatalf("expected scoped project, got %+v", result)
			}
		}
	}
	if results[0].SeedID != "note_002" || results[0].Distance != 1 {
		t.Fatalf("unexpected graph path: %+v", results[0])
	}
	if !foundSameProject {
		t.Fatalf("expected same-project linked candidate, got %+v", results)
	}
}

func TestLinkedCandidatesRequiresProjectScope(t *testing.T) {
	database, cleanup := setupTestDB(t)
	defer cleanup()

	s := New(database)
	results, err := s.LinkedCandidates([]string{"note_002"}, "", 10)
	if err != nil {
		t.Fatal(err)
	}
	if len(results) != 0 {
		t.Fatalf("unscoped graph expansion must be disabled, got %+v", results)
	}
}

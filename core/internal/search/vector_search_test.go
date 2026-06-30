package search

import (
	"testing"
)

func TestCombineResultsRRF(t *testing.T) {
	s := &Searcher{}

	ftsResults := []Result{
		{ID: "a", Title: "A"},
		{ID: "b", Title: "B"},
		{ID: "c", Title: "C"},
	}
	vecResults := []Result{
		{ID: "b", Title: "B"},
		{ID: "d", Title: "D"},
		{ID: "a", Title: "A"},
	}

	results, err := s.combineResults(ftsResults, vecResults, 0.5, 10)
	if err != nil {
		t.Fatalf("combineResults failed: %v", err)
	}

	if len(results) != 4 {
		t.Fatalf("expected 4 results, got %d", len(results))
	}

	// Item b is present in both lists (rank 2 in FTS, rank 1 in vector),
	// so it should receive the highest RRF score and rank first.
	if results[0].ID != "b" {
		t.Errorf("expected top result to be 'b' (present in both lists), got %s", results[0].ID)
	}

	for _, r := range results {
		if r.Score <= 0 {
			t.Errorf("expected positive RRF score for %s, got %f", r.ID, r.Score)
		}
	}
}

func TestCombineResultsWeightZero(t *testing.T) {
	s := &Searcher{}
	ftsResults := []Result{{ID: "a"}, {ID: "b"}}
	vecResults := []Result{{ID: "c"}, {ID: "d"}}

	results, err := s.combineResults(ftsResults, vecResults, 0, 10)
	if err != nil {
		t.Fatalf("combineResults failed: %v", err)
	}
	if len(results) != 2 {
		t.Fatalf("expected 2 results, got %d", len(results))
	}
	if results[0].ID != "a" || results[1].ID != "b" {
		t.Errorf("expected only FTS results in order, got %v", results)
	}
}

func TestCombineResultsWeightOne(t *testing.T) {
	s := &Searcher{}
	ftsResults := []Result{{ID: "a"}, {ID: "b"}}
	vecResults := []Result{{ID: "c"}, {ID: "d"}}

	results, err := s.combineResults(ftsResults, vecResults, 1, 10)
	if err != nil {
		t.Fatalf("combineResults failed: %v", err)
	}
	if len(results) != 2 {
		t.Fatalf("expected 2 results, got %d", len(results))
	}
	if results[0].ID != "c" || results[1].ID != "d" {
		t.Errorf("expected only vector results in order, got %v", results)
	}
}

func TestCombineResultsLimit(t *testing.T) {
	s := &Searcher{}
	ftsResults := []Result{{ID: "a"}, {ID: "b"}, {ID: "c"}}
	vecResults := []Result{{ID: "d"}, {ID: "e"}, {ID: "f"}}

	results, err := s.combineResults(ftsResults, vecResults, 0.5, 2)
	if err != nil {
		t.Fatalf("combineResults failed: %v", err)
	}
	if len(results) != 2 {
		t.Errorf("expected 2 results, got %d", len(results))
	}
}

package search

import "testing"

func TestCombineResults_UsesRankOrderInsteadOfRawScoreMagnitude(t *testing.T) {
	s := &Searcher{}
	fts := []Result{
		{ID: "best", Score: -1000},
		{ID: "second", Score: -0.001},
	}

	results, err := s.combineResults(fts, nil, 0.5, 10)
	if err != nil {
		t.Fatalf("combineResults() unexpected error: %v", err)
	}
	if len(results) != 2 {
		t.Fatalf("expected 2 results, got %d", len(results))
	}
	if results[0].ID != "best" {
		t.Fatalf("expected first-ranked FTS result to remain first, got %q", results[0].ID)
	}
	if results[0].Score <= results[1].Score {
		t.Fatalf("expected fused score to preserve rank order: %#v", results)
	}
}

func TestCombineResults_RewardsAgreementAcrossRetrievers(t *testing.T) {
	s := &Searcher{}
	fts := []Result{
		{ID: "a", Score: -1},
		{ID: "b", Score: -2},
		{ID: "c", Score: -3},
	}
	vec := []Result{
		{ID: "c", Score: 0.99},
		{ID: "b", Score: 0.95},
		{ID: "d", Score: 0.90},
	}

	results, err := s.combineResults(fts, vec, 0.5, 4)
	if err != nil {
		t.Fatalf("combineResults() unexpected error: %v", err)
	}
	if len(results) != 4 {
		t.Fatalf("expected 4 results, got %d", len(results))
	}
	if results[0].ID != "c" {
		t.Fatalf("expected result present in both ranked lists with strong ranks to win, got %q", results[0].ID)
	}
	if results[1].ID != "b" {
		t.Fatalf("expected second cross-retriever agreement result next, got %q", results[1].ID)
	}
}

func TestCombineResults_WeightEndpointsPreserveRetrieverOrder(t *testing.T) {
	s := &Searcher{}
	fts := []Result{{ID: "fts-1"}, {ID: "fts-2"}}
	vec := []Result{{ID: "vec-1"}, {ID: "vec-2"}}

	ftsOnly, err := s.combineResults(fts, vec, 0, 1)
	if err != nil {
		t.Fatalf("FTS-only combine unexpected error: %v", err)
	}
	if len(ftsOnly) != 1 || ftsOnly[0].ID != "fts-1" {
		t.Fatalf("unexpected FTS-only results: %#v", ftsOnly)
	}

	vecOnly, err := s.combineResults(fts, vec, 1, 1)
	if err != nil {
		t.Fatalf("vector-only combine unexpected error: %v", err)
	}
	if len(vecOnly) != 1 || vecOnly[0].ID != "vec-1" {
		t.Fatalf("unexpected vector-only results: %#v", vecOnly)
	}
}

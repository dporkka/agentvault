package search

import (
	"context"
	"testing"

	"github.com/agentvault/core/internal/embeddings"
)

func TestSemanticCandidatesFiltersProjectBeforeVectorRanking(t *testing.T) {
	database, cleanup := setupTestDB(t)
	defer cleanup()

	s := New(database)
	server := embeddingServer([]float32{1, 0, 0, 0})
	defer server.Close()
	s.SetEmbedClient(embeddings.NewClient(server.URL, "test"))

	// note_002 belongs to webapp; note_005 belongs to ai-team.
	if err := s.StoreChunkEmbedding("scoped_webapp", "note_002", 0, "paraphrase target", "test", []float32{0.9, 0.1, 0, 0}); err != nil {
		t.Fatal(err)
	}
	if err := s.StoreChunkEmbedding("scoped_other", "note_005", 0, "exact but forbidden", "test", []float32{1, 0, 0, 0}); err != nil {
		t.Fatal(err)
	}

	results, err := s.SemanticCandidates(context.Background(), "completely different words", "webapp", 10)
	if err != nil {
		t.Fatalf("SemanticCandidates: %v", err)
	}
	if len(results) != 1 {
		t.Fatalf("expected only scoped candidate, got %+v", results)
	}
	if results[0].ID != "note_002" {
		t.Fatalf("expected webapp note, got %+v", results[0])
	}
}

func TestSemanticCandidatesRequiresProjectScope(t *testing.T) {
	database, cleanup := setupTestDB(t)
	defer cleanup()

	s := New(database)
	server := embeddingServer([]float32{1, 0, 0, 0})
	defer server.Close()
	s.SetEmbedClient(embeddings.NewClient(server.URL, "test"))
	if err := s.StoreChunkEmbedding("unscoped", "note_001", 0, "candidate", "test", []float32{1, 0, 0, 0}); err != nil {
		t.Fatal(err)
	}

	results, err := s.SemanticCandidates(context.Background(), "idea", "", 10)
	if err != nil {
		t.Fatal(err)
	}
	if len(results) != 0 {
		t.Fatalf("unscoped semantic expansion must be disabled, got %+v", results)
	}
}

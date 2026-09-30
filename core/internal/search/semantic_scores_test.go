package search

import (
	"context"
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/agentvault/core/internal/embeddings"
)

func TestSemanticScoresOnlyReturnsAllowedCandidates(t *testing.T) {
	database, cleanup := setupTestDB(t)
	defer cleanup()

	s := New(database)
	server := embeddingServer([]float32{1, 0, 0, 0})
	defer server.Close()
	s.SetEmbedClient(embeddings.NewClient(server.URL, "test"))

	if err := s.StoreChunkEmbedding("chunk_allowed", "note_001", 0, "allowed", "test", []float32{1, 0, 0, 0}); err != nil {
		t.Fatal(err)
	}
	if err := s.StoreChunkEmbedding("chunk_forbidden", "note_005", 0, "forbidden", "test", []float32{1, 0, 0, 0}); err != nil {
		t.Fatal(err)
	}

	scores, err := s.SemanticScores(context.Background(), "idea", []string{"note_001"})
	if err != nil {
		t.Fatalf("SemanticScores: %v", err)
	}
	if len(scores) != 1 {
		t.Fatalf("expected one allowed score, got %#v", scores)
	}
	if scores["note_001"] < 0.99 {
		t.Fatalf("expected strong allowed similarity, got %f", scores["note_001"])
	}
	if _, leaked := scores["note_005"]; leaked {
		t.Fatalf("semantic scorer leaked disallowed candidate: %#v", scores)
	}
}

func TestSemanticScoresFallsBackWhenEmbeddingProviderUnavailable(t *testing.T) {
	database, cleanup := setupTestDB(t)
	defer cleanup()

	s := New(database)
	if err := s.StoreChunkEmbedding("chunk_001", "note_001", 0, "idea", "test", []float32{1, 0, 0, 0}); err != nil {
		t.Fatal(err)
	}

	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		http.Error(w, "offline", http.StatusServiceUnavailable)
	}))
	defer server.Close()
	s.SetEmbedClient(embeddings.NewClient(server.URL, "test"))

	scores, err := s.SemanticScores(context.Background(), "idea", []string{"note_001"})
	if err != nil {
		t.Fatalf("provider failure should be a soft fallback, got %v", err)
	}
	if len(scores) != 0 {
		t.Fatalf("expected empty fallback scores, got %#v", scores)
	}
}

func TestSemanticScoresReturnsEmptyWithoutEmbeddings(t *testing.T) {
	database, cleanup := setupTestDB(t)
	defer cleanup()

	s := New(database)
	scores, err := s.SemanticScores(context.Background(), "idea", []string{"note_001"})
	if err != nil {
		t.Fatal(err)
	}
	if len(scores) != 0 {
		t.Fatalf("expected no scores without stored embeddings, got %#v", scores)
	}
}

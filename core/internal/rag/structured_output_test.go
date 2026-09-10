package rag

import (
	"context"
	"testing"

	"github.com/agentvault/core/internal/ai"
	"github.com/agentvault/core/internal/search"
)

func TestPipeline_Ask_DecodesStructuredJSONWithoutFallbackParsing(t *testing.T) {
	database := setupTestDB(t)
	defer database.Close()
	seedTestData(t, database)

	searcher := search.New(database)
	mock := &ai.MockProvider{
		Response: `{"answer":"Postgres with pgvector was selected.","confidence":"high","caveats":["Re-evaluate at larger scale"],"suggested_actions":["Benchmark retrieval"]}`,
	}

	pipeline := New(searcher, mock)
	answer, err := pipeline.Ask(context.Background(), "What have I decided about vector databases?")
	if err != nil {
		t.Fatalf("Ask() unexpected error: %v", err)
	}

	if answer.Answer != "Postgres with pgvector was selected." {
		t.Fatalf("expected decoded structured answer, got %q", answer.Answer)
	}
	if answer.Confidence != "high" {
		t.Fatalf("expected high confidence, got %q", answer.Confidence)
	}
	if len(answer.Caveats) != 1 || answer.Caveats[0] != "Re-evaluate at larger scale" {
		t.Fatalf("expected structured caveat to be preserved, got %#v", answer.Caveats)
	}
	if len(answer.SuggestedActions) != 1 || answer.SuggestedActions[0] != "Benchmark retrieval" {
		t.Fatalf("expected structured action to be preserved, got %#v", answer.SuggestedActions)
	}
	if len(answer.Sources) == 0 {
		t.Fatal("expected retrieved sources to be attached to structured answer")
	}
}
